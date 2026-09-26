package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/lasseh/taillight-tui/internal/tui/component"
)

// handleKey routes a key press through help overlay → filter popup → filter
// input → global keys → active view, in that priority order.
func (a *App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Help overlay takes priority — any key dismisses it.
	if a.showHelp {
		if key.Matches(msg, a.keys.Help) || key.Matches(msg, a.keys.Escape) {
			a.showHelp = false
		}
		return a, nil
	}

	// Filter popup absorbs all keys while open. The popup's own Update
	// handles Esc while editing a field (cancel the edit); Esc in navigation
	// mode closes the popup entirely.
	if a.focus == FocusPopup && a.popup != nil {
		editing := a.popup.Editing()
		if key.Matches(msg, a.keys.Escape) && !editing {
			a.closePopup()
			return a, nil
		}
		updated, cmd := a.popup.Update(msg)
		a.popup = &updated
		return a, cmd
	}

	// Filter input takes priority when focused so number keys (1-7) and
	// vim navigation don't get hijacked while the user is typing a search.
	if a.focus == FocusFilter {
		if msg.String() == "enter" {
			// Commit the search: restart the tab's stream so the server
			// applies the new search param (filtering is server-side).
			a.focus = FocusTable
			if t, ok := a.feeds[a.activeTab]; ok {
				t.BlurFilter()
				t.restart()
			}
			a.syncFilterPills()
			return a, nil
		}
		if key.Matches(msg, a.keys.Escape) {
			a.focus = FocusTable
			a.blurActiveFilter()
			return a, nil
		}
		cmd := a.updateActiveFilter(msg)
		return a, cmd
	}

	// Global keys.
	switch {
	case key.Matches(msg, a.keys.Quit):
		// Release SSE goroutines and TCP connections before quitting.
		// Critical for the wish SSH server where each session would
		// otherwise leak streams until the parent process exits.
		a.Cleanup()
		return a, tea.Quit
	case key.Matches(msg, a.keys.Help):
		a.showHelp = true
		return a, nil
	case key.Matches(msg, a.keys.Filter):
		a.openPopup()
		return a, nil
	case key.Matches(msg, a.keys.Search), key.Matches(msg, a.keys.ToggleFocus):
		a.focus = FocusFilter
		// IMPORTANT: forward the textinput's focus cmd (cursor blink)
		// to bubbletea — without it the input won't respond properly.
		cmd := a.focusActiveFilter()
		return a, cmd
	case key.Matches(msg, a.keys.Tab1):
		cmd := a.switchTab(TabDashboard)
		return a, cmd
	case key.Matches(msg, a.keys.Tab2):
		cmd := a.switchTab(TabNetlog)
		return a, cmd
	case key.Matches(msg, a.keys.Tab3):
		cmd := a.switchTab(TabSrvlog)
		return a, cmd
	case key.Matches(msg, a.keys.Tab4):
		cmd := a.switchTab(TabApplog)
		return a, cmd
	case key.Matches(msg, a.keys.TabNext):
		cmd := a.switchTab(a.adjacentTab(1))
		return a, cmd
	case key.Matches(msg, a.keys.TabPrev):
		cmd := a.switchTab(a.adjacentTab(-1))
		return a, cmd
	case key.Matches(msg, a.keys.Escape):
		if a.focus == FocusDetail {
			a.focus = FocusTable
			a.closeActiveDetail()
			return a, nil
		}
		return a, nil
	}

	// Delegate to active view.
	cmd := a.updateActiveTable(msg)
	return a, cmd
}

// adjacentTab returns the tab delta positions away from the active tab in
// display order, wrapping around at both ends.
func (a *App) adjacentTab(delta int) TabID {
	idx := 0
	for i, t := range AllTabs {
		if t == a.activeTab {
			idx = i
			break
		}
	}
	n := len(AllTabs)
	return AllTabs[((idx+delta)%n+n)%n]
}

// openPopup initialises a filter popup for the active view. Dashboard has no
// filter, so the call is a no-op on that tab.
func (a *App) openPopup() {
	fields := a.activeFilterFields()
	if fields == nil {
		return
	}
	p := component.NewFilterPopup(fields)
	p.SetSize(a.width, a.height)
	a.popup = &p
	a.focus = FocusPopup
}

// closePopup dismisses the popup without applying filter values.
func (a *App) closePopup() {
	a.popup = nil
	a.focus = FocusTable
}

// activeFilterFields builds the popup fields for the active tab's filter.
// Returns nil when the tab has no associated filter (dashboard).
func (a *App) activeFilterFields() []component.Field {
	if t, ok := a.feeds[a.activeTab]; ok {
		return t.PopupFields()
	}
	return nil
}

// applyPopupFilter commits values to the active view's filter and restarts
// that tab's SSE stream so the server-side filters take effect immediately.
func (a *App) applyPopupFilter(values map[string]string) {
	a.closePopup()
	t, ok := a.feeds[a.activeTab]
	if !ok {
		return
	}
	t.ApplyPopupValues(values)
	t.restart()
	a.syncFilterPills()
}

// syncFilterPills refreshes the status bar pills from the params the active
// tab's stream is actually running with. Called at every commit point (popup
// apply, search enter, tab switch) so pills always reflect the server-side
// filtering in effect, never half-typed input.
func (a *App) syncFilterPills() {
	var pills []string
	if t, ok := a.feeds[a.activeTab]; ok {
		pills = t.pills()
	}
	a.statusBar.SetFilters(pills)
}

// Active view delegation helpers.

// focusActiveFilter activates the active view's filter input and returns
// any tea.Cmd from the underlying textinput (e.g., cursor blink) which the
// caller must forward to bubbletea.
func (a *App) focusActiveFilter() tea.Cmd {
	if t, ok := a.feeds[a.activeTab]; ok {
		return t.FocusFilter()
	}
	return nil
}

func (a *App) blurActiveFilter() {
	if t, ok := a.feeds[a.activeTab]; ok {
		t.BlurFilter()
	}
}

func (a *App) updateActiveFilter(msg tea.Msg) tea.Cmd {
	if t, ok := a.feeds[a.activeTab]; ok {
		return t.UpdateFilter(msg)
	}
	return nil
}

func (a *App) updateActiveTable(msg tea.Msg) tea.Cmd {
	t, ok := a.feeds[a.activeTab]
	if !ok {
		var cmd tea.Cmd
		a.dashboard, cmd = a.dashboard.Update(msg)
		return cmd
	}
	cmd := t.UpdateTable(msg)
	if t.DetailOpen() {
		a.focus = FocusDetail
	}
	return cmd
}

func (a *App) closeActiveDetail() {
	if t, ok := a.feeds[a.activeTab]; ok {
		t.CloseDetail()
	}
}
