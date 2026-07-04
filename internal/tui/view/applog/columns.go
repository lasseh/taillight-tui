package applog

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/highlight"
	"github.com/lasseh/taillight-tui/internal/tui/theme"
)

const (
	fixedBarWidth   = 1  // level bar ▎
	fixedTimeWidth  = 10 // " HH:MM:SS " with padding
	fixedLevelWidth = 7  // "FATAL  " with trailing space
)

// columns returns the table column definitions for the given terminal width.
// Matches web GUI: Time, Level, Service, Component, Message.
func columns(width int) []table.Column {
	fixed := fixedBarWidth + fixedTimeWidth + fixedLevelWidth
	remaining := max(width-fixed-8, 20)

	serviceWidth := max(8, remaining*18/100)
	compWidth := max(8, remaining*14/100)
	messageWidth := max(10, remaining-serviceWidth-compWidth)

	return []table.Column{
		{Title: "▎", Width: fixedBarWidth},
		{Title: "  TIME    ", Width: fixedTimeWidth},
		{Title: "LEVEL  ", Width: fixedLevelWidth},
		{Title: "SERVICE", Width: serviceWidth},
		{Title: "COMPONENT", Width: compWidth},
		{Title: "MESSAGE", Width: messageWidth},
	}
}

// eventToRow converts an AppLogEvent to a table row with colored cells.
// Matches web GUI: bar, time, level, service, component, message + attrs.
func eventToRow(e client.AppLogEvent, timeFormat string) table.Row {
	bar := lipglossBar(e.Level)
	ts := theme.Timestamp.Render(" " + e.Timestamp.Local().Format(timeFormat) + " ")
	lvl := theme.AppLogLevelStyle(e.Level).Render(highlight.PadRight(e.Level, fixedLevelWidth-1) + " ")
	svc := theme.Program.Render(highlight.Truncate(e.Service, 20))
	comp := lipgloss.NewStyle().Foreground(theme.ColorYellow).Render(highlight.Truncate(e.Component, 16))

	// Message + inline attrs (like the web GUI: "msg - key=val key=val").
	msg := highlight.Message(e.Msg)
	if attrs := highlight.AttrsInline(e.Attrs); attrs != "" {
		msg += " " + lipgloss.NewStyle().Foreground(theme.ColorOrange).Render("-") +
			" " + theme.Comment.Render(attrs)
	}

	return table.Row{bar, ts, lvl, svc, comp, msg}
}

func lipglossBar(level string) string {
	return theme.AppLogLevelStyle(level).Render("▎")
}

// renderDetailPanel renders the expanded detail view for an applog event.
func renderDetailPanel(e client.AppLogEvent, width int) string {
	var b strings.Builder

	kv := func(key, value string) {
		k := theme.DetailKey.Render(key)
		v := theme.DetailValue.Render(value)
		fmt.Fprintf(&b, "%s %s\n", k, v)
	}

	lvlStyle := theme.AppLogLevelStyle(e.Level)

	kv("ID", fmt.Sprintf("%d", e.ID))
	kv("Received", e.ReceivedAt.Local().Format("2006-01-02 15:04:05.000"))
	kv("Timestamp", e.Timestamp.Local().Format("2006-01-02 15:04:05.000"))
	fmt.Fprintf(&b, "%s %s\n", theme.DetailKey.Render("Level"), lvlStyle.Render(e.Level))
	kv("Service", theme.Program.Render(e.Service))
	if e.Component != "" {
		kv("Component", lipgloss.NewStyle().Foreground(theme.ColorYellow).Render(e.Component))
	}
	kv("Host", theme.Hostname.Render(e.Host))
	if e.Source != "" {
		kv("Source", e.Source)
	}

	b.WriteString("\n")
	kv("Message", "")
	b.WriteString(theme.DetailValue.Width(max(20, width-4)).Render(e.Msg))
	b.WriteString("\n")

	if len(e.Attrs) > 0 && string(e.Attrs) != "{}" && string(e.Attrs) != "null" {
		b.WriteString("\n")
		kv("Attributes", "")
		var pretty json.RawMessage
		if err := json.Unmarshal(e.Attrs, &pretty); err == nil {
			formatted, fErr := json.MarshalIndent(pretty, "", "  ")
			if fErr == nil {
				b.WriteString(theme.Comment.Width(max(20, width-4)).Render(string(formatted)))
			} else {
				b.WriteString(theme.Comment.Render(string(e.Attrs)))
			}
		}
		b.WriteString("\n")
	} else if e.AttrsTruncated {
		// The server omits oversized attrs from list/SSE payloads and sets
		// attrs_truncated — without this hint they would look absent.
		b.WriteString("\n")
		kv("Attributes", "")
		b.WriteString(theme.Comment.Render("(truncated — attrs exceeded the stream size cap and were omitted)"))
		b.WriteString("\n")
	}

	return b.String()
}
