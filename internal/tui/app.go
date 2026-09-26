// Package tui implements the terminal user interface for Taillight.
//
// Files in this package:
//
//   - app.go         — root model: NewApp, Init, Update, View, switchTab,
//     Cleanup, contentHeight
//   - app_keys.go    — key routing: handleKey and per-tab focus/update helpers
//   - app_feeds.go   — live log tabs: view + feed per tab, metadata loaders
//   - app_streams.go — drain loop and isConnected
//   - app_toast.go   — per-feed toast rules for alerting events
//   - keys.go        — global key bindings and tab IDs
//   - msg.go         — message types
//   - theme/         — Tokyo Night colors and lipgloss styles
//   - client/        — HTTP/SSE client for the Taillight API
//   - feed/          — SSE feed lifecycle (open, restart, drain, close)
//   - view/          — per-tab views (srvlog, applog, dashboard, ...); the
//     netlog tab reuses the srvlog view since both feeds share the schema
//   - component/     — shared components (statusbar, tabbar, toast)
//   - buffer/        — generic ring buffer for event storage
package tui

import (
	"errors"
	"fmt"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/component"
	"github.com/lasseh/taillight-tui/internal/tui/feed"
	"github.com/lasseh/taillight-tui/internal/tui/theme"
	"github.com/lasseh/taillight-tui/internal/tui/view/dashboard"
)

// Config holds display configuration for the App.
type Config struct {
	BufferSize    int
	BatchInterval time.Duration
	AutoScroll    bool
	TimeFormat    string
}

// DefaultConfig returns the default display configuration shared by the
// taillight-tui and taillight-wish binaries.
func DefaultConfig() Config {
	return Config{
		BufferSize:    10000,
		BatchInterval: 50 * time.Millisecond,
		AutoScroll:    true,
		TimeFormat:    "15:04:05",
	}
}

// App is the root bubbletea model that owns all views and routes messages.
type App struct {
	cfg    Config
	client *client.Client
	keys   KeyMap

	width  int
	height int

	activeTab TabID
	focus     FocusTarget

	// Live log tabs, each a view plus its SSE feed. The netlog tab reuses
	// the srvlog view: netlog events share the srvlog schema.
	feeds map[TabID]liveTab

	dashboard dashboard.Model
	tabBar    component.TabBar
	statusBar component.StatusBar
	helpModel help.Model
	showHelp  bool

	// Track which views have been initialized.
	dashboardInit bool

	// Notification toasts.
	toasts component.ToastQueue

	// Filter popup — nil when closed. Owned at the App level so the same
	// popup machinery serves srvlog/netlog/applog without generic gymnastics.
	popup *component.FilterPopup

	// State.
	lastError   string
	parseErrors int // cumulative JSON unmarshal failures from SSE streams
}

// NewApp creates the root App model. This constructor is wish-compatible: it
// takes config and client, not stdin/stdout.
func NewApp(cfg Config, c *client.Client) *App {
	return newApp(cfg, c, feed.SSEOpener(c))
}

// newApp builds the App with every SSE stream opened through open.
func newApp(cfg Config, c *client.Client, open feed.Opener) *App {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 10000
	}
	if cfg.BatchInterval <= 0 {
		cfg.BatchInterval = 50 * time.Millisecond
	}
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = "15:04:05"
	}

	tabs := []component.Tab{
		// Primary tabs render on the right side of the bar, logo stays left.
		{ID: int(TabDashboard), Label: "DASHBOARD", Color: theme.ColorBlue, Right: true},
		{ID: int(TabNetlog), Label: "NETLOG", Color: theme.ColorFuchsia, Right: true},
		{ID: int(TabSrvlog), Label: "SRVLOG", Color: theme.ColorTeal, Right: true},
		{ID: int(TabApplog), Label: "APPLOG", Color: theme.ColorPink, Right: true},
	}

	return &App{
		cfg:       cfg,
		client:    c,
		keys:      DefaultKeyMap(),
		activeTab: TabDashboard,
		focus:     FocusTable,
		feeds:     newLogTabs(cfg, c, open),
		dashboard: dashboard.New(c),
		toasts:    component.NewToastQueue(),
		tabBar:    component.NewTabBar(tabs),
		statusBar: component.NewStatusBar(),
		helpModel: help.New(),
	}
}

// Init starts background tasks.
func (a *App) Init() tea.Cmd {
	a.dashboardInit = true
	return tea.Batch(
		a.dashboard.Init(),
		a.sseTick(),
	)
}

// Update handles all messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.helpModel.SetWidth(msg.Width)
		ch := a.contentHeight()
		for _, t := range a.feeds {
			t.SetSize(msg.Width, ch)
		}
		a.dashboard.SetSize(msg.Width, ch)
		if a.popup != nil {
			a.popup.SetSize(msg.Width, msg.Height)
		}
		return a, nil

	case tea.KeyPressMsg:
		return a.handleKey(msg)

	case SSETickMsg:
		a.drainAllStreams()
		cmd := a.sseTick()
		return a, cmd

	case MetaLoadedMsg:
		if t, ok := a.feeds[msg.Tab]; ok {
			t.SetMeta(msg.Meta)
		}
		return a, nil

	case dashboard.StatsLoadedMsg:
		a.dashboard, _ = a.dashboard.Update(msg)
		return a, nil

	case dashboard.StreamsStartedMsg:
		var cmd tea.Cmd
		a.dashboard, cmd = a.dashboard.Update(msg)
		return a, cmd

	case dashboard.StreamTickMsg:
		var cmd tea.Cmd
		a.dashboard, cmd = a.dashboard.Update(msg)
		return a, cmd

	case dashboard.RefreshTickMsg:
		if a.activeTab == TabDashboard {
			var cmd tea.Cmd
			a.dashboard, cmd = a.dashboard.Update(msg)
			return a, cmd
		}
		// Keep the refresh chain alive while another tab is visible;
		// skip the load since the dashboard isn't rendered.
		return a, a.dashboard.RefreshTickCmd()

	case component.FilterPopupAppliedMsg:
		a.applyPopupFilter(msg.Values)
		return a, nil

	case ErrorMsg:
		a.lastError = msg.Err.Error()
		// Auth errors are persistent — don't auto-clear. The user needs
		// to see them and fix their API key.
		if errors.Is(msg.Err, client.ErrUnauthorized) {
			a.statusBar.SetError("authentication failed — check API key in config")
			return a, nil
		}
		a.statusBar.SetError(a.lastError)
		return a, tea.Tick(5*time.Second, func(time.Time) tea.Msg {
			return ClearErrorMsg{}
		})

	case ClearErrorMsg:
		a.lastError = ""
		a.statusBar.SetError("")
		return a, nil
	}

	return a, nil
}

// Minimum terminal dimensions for the TUI to render properly.
const (
	minTerminalWidth  = 60
	minTerminalHeight = 10
)

// View renders the full TUI. The layout is pinned: tab bar at top, status bar
// at bottom, content fills the exact space in between.
func (a *App) View() tea.View {
	if a.width == 0 || a.height == 0 {
		return tea.NewView("Initializing...")
	}

	// Refuse to render on terminals too small to be usable.
	if a.width < minTerminalWidth || a.height < minTerminalHeight {
		msg := lipgloss.NewStyle().
			Foreground(theme.ColorYellow).
			Bold(true).
			Render(fmt.Sprintf("Terminal too small\nminimum %dx%d, current %dx%d",
				minTerminalWidth, minTerminalHeight, a.width, a.height))
		centered := lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Center, msg)
		v := tea.NewView(centered)
		v.AltScreen = true
		return v
	}

	// Tab bar (2 lines: tabs + separator).
	tabBar := a.tabBar.View(a.width)

	// Main content — rendered into a fixed-height box.
	var content string
	if t, ok := a.feeds[a.activeTab]; ok {
		content = t.View()
	} else {
		content = a.dashboard.View()
	}

	// Help overlay replaces content.
	if a.showHelp {
		content = theme.Help.Render(a.helpModel.FullHelpView(a.keys.FullHelp()))
	}

	// Force content to exact height so status bar is always at the bottom.
	ch := a.contentHeight()
	contentBox := lipgloss.NewStyle().
		Width(a.width).
		Height(ch).
		MaxHeight(ch).
		Render(content)

	// Status bar (1 line, always at bottom).
	statusBar := a.statusBar.View(a.width)

	screen := lipgloss.JoinVertical(lipgloss.Left, tabBar, contentBox, statusBar)

	// Overlay toast notifications in the top-right corner.
	if a.toasts.HasToasts() {
		toastOverlay := a.toasts.Render(a.width)
		screen = component.OverlayToasts(screen, toastOverlay, a.width, a.height)
	}

	// Filter popup — rendered last so it sits above toasts and help content.
	if a.popup != nil {
		screen = component.OverlayFilterPopup(screen, a.popup.View(), a.width, a.height)
	}

	v := tea.NewView(screen)
	v.AltScreen = true
	return v
}

// contentHeight returns the height available for the main content area
// (terminal height minus tab bar, separator, and status bar).
func (a *App) contentHeight() int {
	// Tab bar (1) + separator (1) + status bar (1) = 3 lines of chrome.
	return max(a.height-3, 1)
}

// Cleanup releases resources held by the App. Must be called after the
// bubbletea program returns from Run() so SSE goroutines and TCP
// connections don't leak. Safe to call multiple times.
func (a *App) Cleanup() {
	for _, t := range a.feeds {
		t.close()
	}
	a.dashboard.Close()
}

// switchTab changes the active tab, starts the stream lazily if needed, and
// loads metadata.
func (a *App) switchTab(tab TabID) tea.Cmd {
	if a.activeTab == tab {
		return nil
	}
	a.activeTab = tab
	a.focus = FocusTable
	a.tabBar.SetActive(int(tab))

	var cmds []tea.Cmd

	// Start stream for the new tab if not already running. Streams stay
	// alive across tab switches so toast notifications fire for events
	// from non-visible tabs too.
	if t, ok := a.feeds[tab]; ok {
		if t.start() {
			cmds = append(cmds, a.loadMeta(tab, t))
		}
	} else if tab == TabDashboard && !a.dashboardInit {
		a.dashboardInit = true
		cmds = append(cmds, a.dashboard.Init())
	}

	// Pills are per-tab: show the params the new tab's stream runs with.
	a.syncFilterPills()

	return tea.Batch(cmds...)
}
