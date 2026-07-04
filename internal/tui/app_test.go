package tui

import (
	"testing"

	"github.com/lasseh/taillight-tui/internal/tui/view/dashboard"
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
