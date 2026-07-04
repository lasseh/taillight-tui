// Package component provides shared TUI components.
package component

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/lasseh/taillight-tui/internal/tui/highlight"
	"github.com/lasseh/taillight-tui/internal/tui/theme"
)

// StatusBar renders a multi-segment status bar pinned to the terminal bottom.
type StatusBar struct {
	connected   bool
	parseErrors int // cumulative SSE JSON parse failures
	errMsg      string
	filters     []string // active filter pills
}

// NewStatusBar creates a new status bar.
func NewStatusBar() StatusBar {
	return StatusBar{}
}

// SetConnected updates the connection state.
func (s *StatusBar) SetConnected(v bool) {
	s.connected = v
}

// SetParseErrors sets the cumulative SSE JSON parse failure count.
func (s *StatusBar) SetParseErrors(n int) {
	s.parseErrors = n
}

// SetError sets or clears the error message.
func (s *StatusBar) SetError(msg string) {
	s.errMsg = msg
}

// SetFilters sets the active filter pill labels.
func (s *StatusBar) SetFilters(filters []string) {
	s.filters = filters
}

// Segment styles.
var (
	segConn = lipgloss.NewStyle().
		Padding(0, 1)

	segFilter = lipgloss.NewStyle().
			Foreground(theme.ColorTeal).
			Background(lipgloss.Color("#1e2030")).
			Padding(0, 1)

	// segParseErr marks SSE decode failures: red family like segErr, but
	// foreground-on-dark so the persistent counter reads as a warning
	// rather than shouting like the full-error segment.
	segParseErr = lipgloss.NewStyle().
			Foreground(theme.ColorRed).
			Background(lipgloss.Color("#1e2030")).
			Bold(true).
			Padding(0, 1)

	segErr = lipgloss.NewStyle().
		Foreground(theme.ColorBGDark).
		Background(theme.ColorRed).
		Bold(true).
		Padding(0, 1)

	segHelp = lipgloss.NewStyle().
		Foreground(theme.ColorComment).
		Background(lipgloss.Color("#16161e")).
		Padding(0, 1)

	segFill = lipgloss.NewStyle().
		Background(lipgloss.Color("#16161e"))
)

// maxPillWidth caps a single filter pill so one long search string cannot
// crowd out the rest of the status bar.
const maxPillWidth = 24

// View renders the multi-segment status bar at the given width.
func (s *StatusBar) View(width int) string {
	var segments []string

	// Connection segment.
	if s.connected {
		segments = append(segments, segConn.
			Foreground(theme.ColorBGDark).
			Background(theme.ColorGreen).
			Render("● LIVE"))
	} else {
		segments = append(segments, segConn.
			Foreground(theme.ColorBGDark).
			Background(theme.ColorRed).
			Render("● OFFLINE"))
	}

	// Error segment.
	if s.errMsg != "" {
		segments = append(segments, segErr.Render(s.errMsg))
	}

	// Parse error counter (visible when non-zero so production issues
	// don't go unnoticed).
	if s.parseErrors > 0 {
		segments = append(segments, segParseErr.Render(
			fmt.Sprintf("⚠ %d parse errors", s.parseErrors)))
	}

	// Right side: help.
	helpSeg := segHelp.Render("/ search  ? help  q quit")
	rightWidth := lipgloss.Width(helpSeg)

	// Active filter pills — width-aware: each pill is capped at
	// maxPillWidth, and pills that would collide with the right-aligned
	// help segment are dropped instead of wrapping the bar.
	leftWidth := 0
	for _, seg := range segments {
		leftWidth += lipgloss.Width(seg)
	}
	avail := width - leftWidth - rightWidth
	for _, f := range s.filters {
		pill := segFilter.Render(highlight.Truncate(f, maxPillWidth))
		w := lipgloss.Width(pill)
		if w > avail {
			break
		}
		segments = append(segments, pill)
		leftWidth += w
		avail -= w
	}

	// Calculate fill.
	fillWidth := max(0, width-leftWidth-rightWidth)

	fill := segFill.Width(fillWidth).Render("")

	segments = append(segments, fill, helpSeg)
	return lipgloss.JoinHorizontal(lipgloss.Bottom, segments...)
}
