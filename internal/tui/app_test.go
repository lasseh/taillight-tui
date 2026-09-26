package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/component"
	"github.com/lasseh/taillight-tui/internal/tui/feed/feedtest"
	"github.com/lasseh/taillight-tui/internal/tui/view/dashboard"
)

const (
	srvlogPath = "/api/v1/srvlog/stream"
	netlogPath = "/api/v1/netlog/stream"
	applogPath = "/api/v1/applog/stream"
)

// newTestApp returns an App whose streams are in-memory, so tests drive the
// real Update paths without network connections.
func newTestApp(t *testing.T) (*App, *feedtest.Opener) {
	t.Helper()
	open := &feedtest.Opener{}
	a := newApp(DefaultConfig(), nil, open.Open)
	a.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	return a, open
}

// typeKeys sends s as individual key presses.
func typeKeys(a *App, s string) {
	for _, r := range s {
		a.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func enter(a *App) { a.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) }

func srvlogTab(t *testing.T, a *App) *logTab[client.SrvlogEvent] {
	t.Helper()
	tab, ok := a.feeds[TabSrvlog].(*logTab[client.SrvlogEvent])
	if !ok {
		t.Fatal("srvlog tab has unexpected type")
	}
	return tab
}

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
			a, _ := newTestApp(t)
			a.activeTab = tt.tab
			_, cmd := a.Update(dashboard.RefreshTickMsg{})
			if cmd == nil {
				t.Error("Update(RefreshTickMsg) returned nil cmd; refresh chain dies")
			}
		})
	}
}

// TestTabSwitchOpensStreamOnce guards lazy stream start: entering a log tab
// opens its stream on the right endpoint, and leaving/returning keeps the
// same stream so toasts keep firing for background tabs.
func TestTabSwitchOpensStreamOnce(t *testing.T) {
	tests := []struct {
		name string
		tab  TabID
		path string
	}{
		{name: "srvlog", tab: TabSrvlog, path: srvlogPath},
		{name: "netlog", tab: TabNetlog, path: netlogPath},
		{name: "applog", tab: TabApplog, path: applogPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, open := newTestApp(t)
			if cmd := a.switchTab(tt.tab); cmd == nil {
				t.Error("first entry must return a meta load cmd")
			}
			a.switchTab(TabDashboard)
			if cmd := a.switchTab(tt.tab); cmd != nil {
				t.Error("re-entry with a live stream must not reload meta")
			}
			if len(open.Opened) != 1 || open.Opened[0].Path != tt.path {
				t.Fatalf("opened %d streams (first %+v), want exactly one on %s", len(open.Opened), open.Opened, tt.path)
			}
			if open.Opened[0].Closed() {
				t.Error("switching tabs must not close the stream")
			}
		})
	}
}

// TestFilterChangeReplacesStream guards the restart path for every feed: a
// changed filter closes the old stream (no leaked reconnect goroutine),
// opens exactly one new stream with the new params, and drops buffered
// events that predate the filter.
func TestFilterChangeReplacesStream(t *testing.T) {
	tests := []struct {
		name string
		tab  TabID
		path string
	}{
		{name: "srvlog", tab: TabSrvlog, path: srvlogPath},
		{name: "netlog", tab: TabNetlog, path: netlogPath},
		{name: "applog", tab: TabApplog, path: applogPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, open := newTestApp(t)
			a.switchTab(tt.tab)
			old := open.Last(tt.path)

			a.Update(component.FilterPopupAppliedMsg{Values: map[string]string{"search": "bgp"}})

			if !old.Closed() {
				t.Error("replaced stream must be closed")
			}
			cur := open.Last(tt.path)
			if len(open.Opened) != 2 || cur == old {
				t.Fatalf("opened %d streams, want 2", len(open.Opened))
			}
			if got := cur.Params.Encode(); got != "search=bgp" {
				t.Errorf("new stream params = %q, want search=bgp", got)
			}
		})
	}
}

// TestSearchEnterAppliesToStream guards the inline `/` search: enter must
// restart the tab's stream with the search query param, drop buffered events
// that predate the filter, and show the filter as a status bar pill that
// clears on the (filterless) dashboard.
func TestSearchEnterAppliesToStream(t *testing.T) {
	a, open := newTestApp(t)
	a.switchTab(TabSrvlog)
	first := open.Last(srvlogPath)
	first.Send(t, client.SrvlogEvent{ID: 1, Severity: 6})
	first.Send(t, client.SrvlogEvent{ID: 2, Severity: 6})
	a.Update(SSETickMsg{})
	if got := srvlogTab(t, a).EventCount(); got != 2 {
		t.Fatalf("EventCount before search = %d, want 2", got)
	}

	typeKeys(a, "/error")
	enter(a)

	if a.focus != FocusTable {
		t.Errorf("focus after enter = %v, want FocusTable", a.focus)
	}
	if got := open.Last(srvlogPath).Params.Encode(); got != "search=error" {
		t.Errorf("stream params = %q, want search=error", got)
	}
	if got := srvlogTab(t, a).EventCount(); got != 0 {
		t.Errorf("EventCount after filter change = %d, want 0 (stale events must clear)", got)
	}
	if got := a.statusBar.View(120); !strings.Contains(got, "/error") {
		t.Errorf("status bar after search commit missing pill %q\nrendered: %q", "/error", got)
	}

	a.switchTab(TabDashboard)
	if got := a.statusBar.View(120); strings.Contains(got, "/error") {
		t.Errorf("status bar on dashboard still shows a pill\nrendered: %q", got)
	}
}

// TestSearchEnterUnchangedIsNoop guards against wiping the buffer when the
// filter didn't actually change (`/` then enter) while a stream is running.
func TestSearchEnterUnchangedIsNoop(t *testing.T) {
	a, open := newTestApp(t)
	a.switchTab(TabSrvlog)
	open.Last(srvlogPath).Send(t, client.SrvlogEvent{ID: 1, Severity: 6})
	a.Update(SSETickMsg{})

	typeKeys(a, "/")
	enter(a)

	if len(open.Opened) != 1 {
		t.Errorf("opened %d streams, want 1 (unchanged filter must not restart)", len(open.Opened))
	}
	if got := srvlogTab(t, a).EventCount(); got != 1 {
		t.Errorf("EventCount after no-op search = %d, want 1 (buffer must survive)", got)
	}
}

// TestSearchEscKeepsRunningFilter guards pills against half-typed input:
// Esc abandons the search, so pills keep showing the running filter and a
// later tab re-entry does not restart the stream with the typed text.
func TestSearchEscKeepsRunningFilter(t *testing.T) {
	a, open := newTestApp(t)
	a.switchTab(TabSrvlog)
	typeKeys(a, "/half")
	a.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	a.switchTab(TabDashboard)
	a.switchTab(TabSrvlog)

	if len(open.Opened) != 1 {
		t.Errorf("opened %d streams, want 1", len(open.Opened))
	}
	if got := a.statusBar.View(120); strings.Contains(got, "/half") {
		t.Errorf("status bar shows uncommitted search\nrendered: %q", got)
	}
}

// TestDrainToastsAndParseErrors guards the drain loop: alerting events raise
// toasts, benign ones don't, and undecodable payloads are counted.
func TestDrainToastsAndParseErrors(t *testing.T) {
	a, open := newTestApp(t)
	a.switchTab(TabSrvlog)
	s := open.Last(srvlogPath)
	s.Send(t, client.SrvlogEvent{ID: 1, Severity: 6, ReceivedAt: time.Now()})
	a.Update(SSETickMsg{})
	if a.toasts.HasToasts() {
		t.Error("info event must not raise a toast")
	}

	s.Send(t, client.SrvlogEvent{ID: 2, Severity: 2, ReceivedAt: time.Now()})
	s.SendRaw("{not json")
	a.Update(SSETickMsg{})

	if !a.toasts.HasToasts() {
		t.Error("crit event must raise a toast")
	}
	if a.parseErrors != 1 {
		t.Errorf("parseErrors = %d, want 1", a.parseErrors)
	}
	if got := srvlogTab(t, a).EventCount(); got != 2 {
		t.Errorf("EventCount = %d, want 2", got)
	}
}

// TestCleanupClosesStreams guards against leaking streams when a session
// ends (critical for the wish server).
func TestCleanupClosesStreams(t *testing.T) {
	a, open := newTestApp(t)
	a.switchTab(TabSrvlog)
	a.switchTab(TabApplog)
	a.Cleanup()
	for _, s := range open.Opened {
		if !s.Closed() {
			t.Errorf("stream %s left open after Cleanup", s.Path)
		}
	}
}
