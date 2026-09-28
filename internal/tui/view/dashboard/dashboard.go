// Package dashboard implements the dashboard view with summary stats,
// severity distribution, top hosts, and recent critical events — matching
// the web GUI's HomeView layout.
package dashboard

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"net/url"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/feed"
	"github.com/lasseh/taillight-tui/internal/tui/highlight"
	"github.com/lasseh/taillight-tui/internal/tui/theme"
)

const refreshInterval = 30 * time.Second

// StatsLoadedMsg carries loaded dashboard statistics.
type StatsLoadedMsg struct {
	Srvlog       *client.StatsSummary
	Applog       *client.AppLogStatsSummary
	CriticalLogs []client.SrvlogEvent
	RecentErrors []client.AppLogEvent
	Err          error
}

const maxRecentEvents = 10

// Model is the dashboard view.
type Model struct {
	client       *client.Client
	srvlog       *client.StatsSummary
	applog       *client.AppLogStatsSummary
	criticalLogs []client.SrvlogEvent
	recentErrors []client.AppLogEvent
	loading      bool
	loadFailed   bool // last stats load returned an error
	lastLoad     time.Time
	width        int
	height       int

	// Live feeds for the recent-events panels.
	criticalFeed *feed.Feed[client.SrvlogEvent]
	errorFeed    *feed.Feed[client.AppLogEvent]
}

// Live feed filters: srvlog severity ≤ 2 (emerg, alert, crit) and applog
// WARN and above, matching the REST queries in loadStats.
var (
	criticalParams = url.Values{"severity_max": {"2"}}
	errorParams    = url.Values{"level": {"WARN"}}
)

// drainBatch caps how many events one tick drains from each feed.
const drainBatch = 50

// New creates a new dashboard model. Its live feeds open through open.
func New(c *client.Client, open feed.Opener) Model {
	return Model{
		client:       c,
		loading:      true,
		criticalFeed: feed.New[client.SrvlogEvent]("/api/v1/srvlog/stream", open),
		errorFeed:    feed.New[client.AppLogEvent]("/api/v1/applog/stream", open),
	}
}

// Connected reports whether any of the dashboard's feeds are connected.
func (m *Model) Connected() bool {
	return m.criticalFeed.Connected() || m.errorFeed.Connected()
}

// Close releases the dashboard's feeds. Called from the app's Cleanup() to
// prevent goroutine and connection leaks.
func (m *Model) Close() {
	m.criticalFeed.Close()
	m.errorFeed.Close()
}

// startFeeds opens any feed that isn't running.
func (m *Model) startFeeds() {
	m.criticalFeed.Start(criticalParams)
	m.errorFeed.Start(errorParams)
}

// Drain moves pending live events into the recent-events panels. The app
// calls it on its SSE tick. It returns the count of undecodable payloads and
// the errors of feeds that stopped for good, joined; a manual refresh reopens
// them. Both are reported: a feed drops its stream when it returns the error,
// so an error not returned here is lost.
func (m *Model) Drain() (parseErrors int, err error) {
	crit, critParse, critErr := m.criticalFeed.Drain(drainBatch)
	m.criticalLogs = prependCapped(crit, m.criticalLogs)
	errs, errParse, errErr := m.errorFeed.Drain(drainBatch)
	m.recentErrors = prependCapped(errs, m.recentErrors)
	return critParse + errParse, errors.Join(critErr, errErr)
}

// SetSize updates dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Init opens the live feeds and returns the initial load command.
func (m *Model) Init() tea.Cmd {
	m.startFeeds()
	return tea.Batch(m.loadStats(), m.refreshTick())
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StatsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			// Keep previous data and the previous Updated timestamp;
			// surface the failure in the footer instead.
			m.loadFailed = true
			return m, nil
		}
		m.loadFailed = false
		m.lastLoad = time.Now()
		m.srvlog = msg.Srvlog
		m.applog = msg.Applog
		m.criticalLogs = msg.CriticalLogs
		m.recentErrors = msg.RecentErrors
		return m, nil

	case RefreshTickMsg:
		return m, tea.Batch(m.loadStats(), m.refreshTick())

	case tea.KeyPressMsg:
		if key.Matches(msg, refreshKey) {
			m.startFeeds()
			m.loading = true
			cmd := m.loadStats()
			return m, cmd
		}
	}
	return m, nil
}

// View renders the dashboard.
func (m *Model) View() string {
	if m.loading && m.srvlog == nil {
		return lipgloss.NewStyle().
			Width(m.width).Height(m.height).
			Foreground(theme.ColorComment).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Loading dashboard...")
	}

	var sections []string

	// Section header.
	sections = append(sections, sectionHeader("SYSLOG", theme.ColorTeal))

	// Summary cards.
	if m.srvlog != nil {
		sections = append(sections, m.renderSummaryCards())
		sections = append(sections, "")
	}

	// Recent high-severity events.
	if len(m.criticalLogs) > 0 {
		sections = append(sections, m.renderRecentCritical())
		sections = append(sections, "")
	}

	// Applog section.
	if m.applog != nil {
		sections = append(sections, sectionHeader("APPLOG", theme.ColorPink))
		sections = append(sections, m.renderApplogCards())
	}

	// Recent applog errors.
	if len(m.recentErrors) > 0 {
		sections = append(sections, "")
		sections = append(sections, m.renderRecentErrors())
	}

	// Footer.
	if !m.lastLoad.IsZero() || m.loadFailed {
		sep := theme.Comment.Render("  ·  ")
		parts := make([]string, 0, 3)
		if m.loadFailed {
			parts = append(parts, lipgloss.NewStyle().Foreground(theme.ColorRed).Render("last refresh failed"))
		}
		if !m.lastLoad.IsZero() {
			parts = append(parts, theme.Comment.Render(fmt.Sprintf("Updated %s", m.lastLoad.Format("15:04:05"))))
		}
		parts = append(parts, theme.Comment.Render("r refresh  ·  24h range"))
		sections = append(sections, "")
		sections = append(sections, "  "+strings.Join(parts, sep))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func sectionHeader(title string, c color.Color) string {
	label := lipgloss.NewStyle().Foreground(c).Bold(true).Render("  " + title)
	return label
}

// --- Summary cards matching the web GUI: Total, Emerg & Alert, Criticals, Errors ---

func (m *Model) renderSummaryCards() string {
	s := m.srvlog
	cardW := max(15, (m.width-10)/4)

	// Find specific severity counts.
	var emergCnt, alertCnt, critCnt, errCnt int64
	for _, sc := range s.SeverityBreakdown {
		switch sc.Severity {
		case 0:
			emergCnt = sc.Count
		case 1:
			alertCnt = sc.Count
		case 2:
			critCnt = sc.Count
		case 3:
			errCnt = sc.Count
		}
	}

	total := miniCard(cardW, "Total", formatCount(s.Total), theme.ColorTeal,
		trendStr(s.Trend))
	emergAlert := miniCard(cardW, "Emerg & Alert",
		fmt.Sprintf("%s / %s", formatCount(emergCnt), formatCount(alertCnt)),
		theme.ColorPink, "")
	crits := miniCard(cardW, "Criticals", formatCount(critCnt), theme.ColorRed,
		pctStr(critCnt, s.Total))
	errs := miniCard(cardW, "Errors", formatCount(errCnt), theme.ColorOrange,
		pctStr(errCnt, s.Total))

	return lipgloss.JoinHorizontal(lipgloss.Top, total, " ", emergAlert, " ", crits, " ", errs)
}

func miniCard(width int, label, value string, valueColor color.Color, subtitle string) string {
	labelLine := theme.Comment.Render("  " + label)
	valueLine := lipgloss.NewStyle().Foreground(valueColor).Bold(true).Render("  " + value)
	subtitleLine := " " // always render a third line for consistent height
	if subtitle != "" {
		subtitleLine = theme.Comment.Render("  " + subtitle)
	}
	content := strings.Join([]string{labelLine, valueLine, subtitleLine}, "\n")
	return theme.Card.Width(width).Render(content)
}

// --- Recent critical events ---

func (m *Model) renderRecentCritical() string {
	lines := make([]string, 0, 2+len(m.criticalLogs))
	lines = append(lines, theme.CardLabel.Render("  RECENT HIGH-SEVERITY"))
	lines = append(lines, "")

	// Iterate newest-last so the rendered panel reads oldest-on-top, newest at
	// the bottom — matches every other log feed in the product. The producer
	// (drainStreams) prepends, so m.criticalLogs is newest-first.
	for i := len(m.criticalLogs) - 1; i >= 0; i-- {
		e := m.criticalLogs[i]
		ts := theme.Timestamp.Render(e.ReceivedAt.Local().Format("15:04"))
		sev := theme.SeverityStyle(e.Severity).Render(highlight.PadRight(strings.ToUpper(e.SeverityLabel), 8))
		host := theme.Hostname.Render(highlight.PadRight(highlight.Truncate(e.Hostname, 18), 18))
		prog := theme.Program.Render(highlight.PadRight(highlight.Truncate(e.Programname, 12), 12))

		msgW := max(20, m.width-55)
		msg := highlight.Message(highlight.Truncate(e.Message, msgW))

		lines = append(lines, fmt.Sprintf("  %s  %s %s %s %s", ts, sev, host, prog, msg))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// --- Recent applog errors ---

func (m *Model) renderRecentErrors() string {
	lines := make([]string, 0, 2+len(m.recentErrors))
	lines = append(lines, theme.CardLabel.Render("  RECENT ERRORS"))
	lines = append(lines, "")

	// Iterate newest-last so the rendered panel reads oldest-on-top, newest at
	// the bottom — matches every other log feed in the product. The producer
	// (drainStreams) prepends, so m.recentErrors is newest-first.
	for i := len(m.recentErrors) - 1; i >= 0; i-- {
		e := m.recentErrors[i]
		ts := theme.Timestamp.Render(e.Timestamp.Local().Format("15:04"))
		lvl := theme.AppLogLevelStyle(e.Level).Render(highlight.PadRight(e.Level, 6))
		svc := theme.Program.Render(highlight.PadRight(highlight.Truncate(e.Service, 18), 18))
		comp := lipgloss.NewStyle().Foreground(theme.ColorYellow).Render(highlight.PadRight(highlight.Truncate(e.Component, 14), 14))

		msgW := max(20, m.width-56)
		msg := highlight.Message(highlight.Truncate(e.Msg, msgW))

		// Inline attrs like the web GUI.
		if attrs := highlight.AttrsInline(e.Attrs); attrs != "" {
			msg += " " + lipgloss.NewStyle().Foreground(theme.ColorOrange).Render("-") +
				" " + theme.Comment.Render(highlight.Truncate(attrs, max(10, msgW/2)))
		}

		lines = append(lines, fmt.Sprintf("  %s  %s %s %s %s", ts, lvl, svc, comp, msg))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// --- Applog summary cards ---

func (m *Model) renderApplogCards() string {
	s := m.applog
	cardW := max(15, (m.width-10)/4)

	var fatalCnt, errCnt, warnCnt, infoCnt int64
	for _, lc := range s.LevelBreakdown {
		switch lc.Level {
		case "FATAL":
			fatalCnt = lc.Count
		case "ERROR":
			errCnt = lc.Count
		case "WARN":
			warnCnt = lc.Count
		case "INFO":
			infoCnt = lc.Count
		}
	}

	total := miniCard(cardW, "Total", formatCount(s.Total), theme.ColorTeal,
		trendStr(s.Trend))
	fatalErr := miniCard(cardW, "Fatal & Errors",
		fmt.Sprintf("%s / %s", formatCount(fatalCnt), formatCount(errCnt)),
		theme.ColorPink, "")
	warns := miniCard(cardW, "Warnings", formatCount(warnCnt), theme.ColorYellow, "")
	infos := miniCard(cardW, "Info", formatCount(infoCnt), theme.ColorTeal, "")

	return lipgloss.JoinHorizontal(lipgloss.Top, total, " ", fatalErr, " ", warns, " ", infos)
}

// prependCapped prepends a drained batch (arrival order, oldest first) to
// current so the result stays newest-first, capped at maxRecentEvents.
func prependCapped[T any](batch, current []T) []T {
	if len(batch) == 0 {
		return current
	}
	out := make([]T, 0, len(batch)+len(current))
	for i := len(batch) - 1; i >= 0; i-- {
		out = append(out, batch[i])
	}
	out = append(out, current...)
	if len(out) > maxRecentEvents {
		out = out[:maxRecentEvents]
	}
	return out
}

// --- Data loading ---

func (m *Model) loadStats() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srvlog, err := c.SrvlogSummary(ctx, "24h")
		if err != nil {
			return StatsLoadedMsg{Err: err}
		}
		applog, err := c.AppLogSummary(ctx, "24h")
		if err != nil {
			return StatsLoadedMsg{Err: err}
		}
		// Fetch recent critical srvlog events (severity ≤ 2).
		critResp, err := c.ListSrvlogs(ctx, client.SrvlogFilter{
			SeverityMax: 2,
			Facility:    -1,
		}, "", 10)
		if err != nil {
			return StatsLoadedMsg{Err: err}
		}
		// Fetch recent applog errors (WARN and above).
		errResp, err := c.ListAppLogs(ctx, client.AppLogFilter{
			Level: "WARN",
		}, "", 10)
		if err != nil {
			return StatsLoadedMsg{Err: err}
		}
		return StatsLoadedMsg{
			Srvlog:       srvlog,
			Applog:       applog,
			CriticalLogs: critResp.Data,
			RecentErrors: errResp.Data,
		}
	}
}

// RefreshTickMsg triggers a dashboard refresh.
type RefreshTickMsg struct{}

func (m *Model) refreshTick() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		return RefreshTickMsg{}
	})
}

// RefreshTickCmd schedules the next RefreshTickMsg. The app calls this to
// keep the refresh chain alive while the dashboard tab is not visible.
func (m *Model) RefreshTickCmd() tea.Cmd {
	return m.refreshTick()
}

var refreshKey = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))

// --- Helpers ---

func formatCount(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func trendStr(trend float64) string {
	if trend > 0 {
		return fmt.Sprintf("▲ +%.0f%%", trend)
	}
	if trend < 0 {
		return fmt.Sprintf("▼ %.0f%%", trend)
	}
	return "─ 0%"
}

func pctStr(count, total int64) string {
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%.1f%% of total", float64(count)/float64(total)*100)
}
