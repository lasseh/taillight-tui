package tui

import (
	"net/url"
	"testing"

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
