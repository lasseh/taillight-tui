package tui

import (
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/view/dashboard"
	"github.com/lasseh/taillight-tui/internal/tui/view/srvlog"
)

// TestRefreshTickAlwaysReschedules guards the dashboard auto-refresh chain:
// a RefreshTickMsg must always produce a follow-up command, even when
// another tab is active, or the 30s refresh dies for the session.
func TestRefreshTickAlwaysReschedules(t *testing.T) {
	tests := []struct {
		name string
		tab  TabID
	}{
		{name: "dashboard visible", tab: TabDashboard},
		{name: "srvlog tab active", tab: TabSrvlog},
		{name: "applog tab active", tab: TabApplog},
		{name: "netlog tab active", tab: TabNetlog},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewApp(DefaultConfig(), nil)
			a.activeTab = tt.tab
			_, cmd := a.Update(dashboard.RefreshTickMsg{})
			if cmd == nil {
				t.Error("Update(RefreshTickMsg) returned nil cmd; refresh chain dies")
			}
		})
	}
}

// TestSearchEnterAppliesToStream guards the inline `/` search: enter must
// restart the tab's stream with the search query param and drop buffered
// events that predate the filter — otherwise the search is a no-op.
func TestSearchEnterAppliesToStream(t *testing.T) {
	a := NewApp(DefaultConfig(), nil)
	a.activeTab = TabSrvlog
	a.focus = FocusFilter
	a.srvlog.PushEvents([]client.SrvlogEvent{{ID: 1}, {ID: 2}})
	srvlog.Filter(&a.srvlog).ApplyPopupValues(map[string]string{"search": "error"})

	_, cmd := a.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if a.focus != FocusTable {
		t.Errorf("focus after enter = %v, want FocusTable", a.focus)
	}
	if cmd == nil {
		t.Fatal("enter in search must return a stream restart cmd, got nil")
	}
	if got := a.streamParams[TabSrvlog]; got != "search=error" {
		t.Errorf("stream params = %q, want %q", got, "search=error")
	}
	if got := a.srvlog.EventCount(); got != 0 {
		t.Errorf("EventCount after filter change = %d, want 0 (stale events must clear)", got)
	}
}

// TestFilterPills guards the encoded-params → status bar pill translation:
// labels must be compact, in fixed display order, and cover both the
// srvlog/netlog and applog param vocabularies.
func TestFilterPills(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		want    []string
	}{
		{name: "empty params", encoded: "", want: nil},
		{name: "search only", encoded: "search=error", want: []string{"/error"}},
		{
			name: "srvlog fields in display order",
			encoded: url.Values{
				"severity_max": {"4"},
				"hostname":     {"core1"},
				"search":       {"bgp"},
			}.Encode(),
			want: []string{"host=core1", "sev≤4", "/bgp"},
		},
		{
			name: "applog fields",
			encoded: url.Values{
				"service":   {"api"},
				"level":     {"WARN"},
				"component": {"ingest"},
			}.Encode(),
			want: []string{"svc=api", "comp=ingest", "lvl=WARN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterPills(tt.encoded); !slices.Equal(got, tt.want) {
				t.Errorf("filterPills(%q) = %v, want %v", tt.encoded, got, tt.want)
			}
		})
	}
}

// TestSearchEnterShowsFilterPill guards the pill wiring: committing a search
// must surface it as a status bar pill for the active tab, and switching to
// the (filterless) dashboard must clear the pills again.
func TestSearchEnterShowsFilterPill(t *testing.T) {
	a := NewApp(DefaultConfig(), nil)
	a.activeTab = TabSrvlog
	a.focus = FocusFilter
	srvlog.Filter(&a.srvlog).ApplyPopupValues(map[string]string{"search": "error"})

	a.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := a.statusBar.View(120); !strings.Contains(got, "/error") {
		t.Errorf("status bar after search commit missing pill %q\nrendered: %q", "/error", got)
	}

	a.switchTab(TabDashboard)
	if got := a.statusBar.View(120); strings.Contains(got, "/error") {
		t.Errorf("status bar on dashboard still shows a pill\nrendered: %q", got)
	}
}

// TestStreamStartedMsgClosesReplacedStream guards against leaking SSE
// streams: when a StreamStartedMsg arrives for a feed that already has a
// live stream (double-start race during the in-flight window), the old
// stream must be closed before the new one is stored — otherwise its
// reconnect goroutine and connection leak for the life of the process.
func TestStreamStartedMsgClosesReplacedStream(t *testing.T) {
	tests := []struct {
		name string
		feed string
		get  func(a *App) *client.SSEStream
		set  func(a *App, s *client.SSEStream)
	}{
		{
			name: "srvlog",
			feed: "srvlog",
			get:  func(a *App) *client.SSEStream { return a.srvlogStream },
			set:  func(a *App, s *client.SSEStream) { a.srvlogStream = s },
		},
		{
			name: "applog",
			feed: "applog",
			get:  func(a *App) *client.SSEStream { return a.applogStream },
			set:  func(a *App, s *client.SSEStream) { a.applogStream = s },
		},
		{
			name: "netlog",
			feed: "netlog",
			get:  func(a *App) *client.SSEStream { return a.netlogStream },
			set:  func(a *App, s *client.SSEStream) { a.netlogStream = s },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewApp(DefaultConfig(), nil)
			c := client.New(client.Config{BaseURL: "http://127.0.0.1:0"})

			oldStream := client.NewSSEStream(c, "/api/v1/"+tt.feed+"/stream", url.Values{}, 0)
			tt.set(a, oldStream)

			newStream := client.NewSSEStream(c, "/api/v1/"+tt.feed+"/stream", url.Values{}, 0)
			defer newStream.Close()

			a.Update(StreamStartedMsg{Feed: tt.feed, Stream: newStream})

			if got := tt.get(a); got != newStream {
				t.Error("StreamStartedMsg must store the new stream")
			}
			// Close cancels the old stream's context; its run goroutine
			// closes the event channel on exit. A drained-and-closed channel
			// proves the replaced stream was shut down, not leaked.
			select {
			case _, ok := <-oldStream.Events():
				if ok {
					t.Error("replaced stream delivered an event; expected closed channel")
				}
			case <-time.After(2 * time.Second):
				t.Error("replaced stream not closed: event channel still open after 2s")
			}
		})
	}
}

// TestRestartStreamUnchangedParamsIsNoop guards against wiping the buffer
// when the filter didn't actually change (e.g. `/` then enter, or a popup
// apply with no edits) while a stream is already running.
func TestRestartStreamUnchangedParamsIsNoop(t *testing.T) {
	a := NewApp(DefaultConfig(), nil)
	a.activeTab = TabSrvlog
	a.srvlog.PushEvents([]client.SrvlogEvent{{ID: 1}, {ID: 2}})

	// Simulate a running stream started with the current (empty) params.
	c := client.New(client.Config{BaseURL: "http://127.0.0.1:0"})
	stream := client.NewSSEStream(c, "/api/v1/srvlog/stream", url.Values{}, 0)
	defer stream.Close()
	a.srvlogStream = stream
	a.streamParams[TabSrvlog] = a.srvlog.Filter().Params().Encode()

	if cmd := a.restartStream(TabSrvlog); cmd != nil {
		t.Error("restartStream with unchanged params should be a no-op")
	}
	if got := a.srvlog.EventCount(); got != 2 {
		t.Errorf("EventCount after no-op restart = %d, want 2 (buffer must survive)", got)
	}
	if a.srvlogStream != stream {
		t.Error("no-op restart must keep the existing stream")
	}
}
