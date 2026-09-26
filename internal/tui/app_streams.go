package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// sseTick returns a command that fires an SSETickMsg after the batch interval.
func (a *App) sseTick() tea.Cmd {
	return tea.Tick(a.cfg.BatchInterval, func(time.Time) tea.Msg {
		return SSETickMsg{}
	})
}

// drainAllStreams moves pending events from every live feed into its view.
// Alerting events also trigger toast notifications. Tracks JSON parse
// failures so the user is alerted via the status bar. Returns a Cmd raising
// an ErrorMsg when a stream stopped for good (e.g. the API key was revoked
// mid-session), so the failure is visible instead of a silently dead tab.
func (a *App) drainAllStreams() tea.Cmd {
	var stopped error
	for _, id := range AllTabs {
		if t, ok := a.feeds[id]; ok {
			n, err := t.drain(&a.toasts)
			a.parseErrors += n
			if err != nil && stopped == nil {
				stopped = err
			}
		}
	}
	a.statusBar.SetParseErrors(a.parseErrors)

	// Prune expired toasts.
	a.toasts.Prune()
	a.statusBar.SetConnected(a.isConnected())

	if stopped == nil {
		return nil
	}
	return func() tea.Msg { return ErrorMsg{Err: stopped} }
}

// isConnected returns true if ANY SSE stream is currently connected.
// Checks all log-view streams AND the dashboard's own streams.
func (a *App) isConnected() bool {
	for _, t := range a.feeds {
		if t.connected() {
			return true
		}
	}
	return a.dashboard.Connected()
}
