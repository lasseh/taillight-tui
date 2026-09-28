package dashboard

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/feed/feedtest"
)

func TestUpdateStatsLoaded(t *testing.T) {
	prior := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	priorStats := &client.StatsSummary{Total: 42}
	newStats := &client.StatsSummary{Total: 99}

	tests := []struct {
		name         string
		model        Model
		msg          StatsLoadedMsg
		wantFailed   bool
		wantAdvanced bool // lastLoad must advance past the model's value
		wantStats    *client.StatsSummary
	}{
		{
			name:       "first load fails keeps lastLoad zero",
			model:      Model{loading: true},
			msg:        StatsLoadedMsg{Err: errors.New("boom")},
			wantFailed: true,
			wantStats:  nil,
		},
		{
			name:       "refresh failure keeps data and timestamp",
			model:      Model{lastLoad: prior, srvlog: priorStats},
			msg:        StatsLoadedMsg{Err: errors.New("boom")},
			wantFailed: true,
			wantStats:  priorStats,
		},
		{
			name:         "successful load clears failure and advances timestamp",
			model:        Model{loadFailed: true, lastLoad: prior, srvlog: priorStats},
			msg:          StatsLoadedMsg{Srvlog: newStats},
			wantFailed:   false,
			wantAdvanced: true,
			wantStats:    newStats,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := tt.model.Update(tt.msg)
			if got.loading {
				t.Error("loading = true after StatsLoadedMsg, want false")
			}
			if got.loadFailed != tt.wantFailed {
				t.Errorf("loadFailed = %v, want %v", got.loadFailed, tt.wantFailed)
			}
			if tt.wantAdvanced {
				if !got.lastLoad.After(tt.model.lastLoad) {
					t.Errorf("lastLoad = %v, want after %v", got.lastLoad, tt.model.lastLoad)
				}
			} else if !got.lastLoad.Equal(tt.model.lastLoad) {
				t.Errorf("lastLoad = %v, want unchanged %v", got.lastLoad, tt.model.lastLoad)
			}
			if got.srvlog != tt.wantStats {
				t.Errorf("srvlog = %v, want %v", got.srvlog, tt.wantStats)
			}
		})
	}
}

// TestDrainReportsEveryStoppedFeed guards against a silently dead panel: when
// both feeds stop in the same tick (revoked key), both errors must surface,
// since a feed drops its stream once it has returned the error.
func TestDrainReportsEveryStoppedFeed(t *testing.T) {
	open := &feedtest.Opener{}
	m := New(nil, open.Open)
	m.Init()
	for _, s := range open.Opened {
		s.Fail(client.ErrUnauthorized)
	}

	_, err := m.Drain()
	if !errors.Is(err, client.ErrUnauthorized) {
		t.Fatalf("Drain() err = %v, want ErrUnauthorized", err)
	}
	for _, path := range []string{"/api/v1/srvlog/stream", "/api/v1/applog/stream"} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("Drain() err = %q, missing stopped feed %s", err, path)
		}
	}
}

func TestViewSurfacesRefreshFailure(t *testing.T) {
	tests := []struct {
		name        string
		model       Model
		wantFailed  bool
		wantUpdated bool
	}{
		{
			name:        "failed refresh shows indicator",
			model:       Model{loadFailed: true},
			wantFailed:  true,
			wantUpdated: false,
		},
		{
			name:        "failed refresh keeps previous timestamp visible",
			model:       Model{loadFailed: true, lastLoad: time.Now()},
			wantFailed:  true,
			wantUpdated: true,
		},
		{
			name:        "healthy refresh shows no indicator",
			model:       Model{lastLoad: time.Now()},
			wantFailed:  false,
			wantUpdated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.model.SetSize(100, 30)
			view := tt.model.View()
			if got := strings.Contains(view, "last refresh failed"); got != tt.wantFailed {
				t.Errorf("View contains %q = %v, want %v", "last refresh failed", got, tt.wantFailed)
			}
			if got := strings.Contains(view, "Updated"); got != tt.wantUpdated {
				t.Errorf("View contains %q = %v, want %v", "Updated", got, tt.wantUpdated)
			}
		})
	}
}
