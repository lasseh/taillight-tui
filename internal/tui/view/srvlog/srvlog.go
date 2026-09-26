// Package srvlog wraps the generic logview package with srvlog-specific
// columns, row rendering, detail panel, and filter.
//
// The netlog tab reuses this view as-is: netlog events share the srvlog
// schema (client.NetlogEvent is an alias of client.SrvlogEvent), so only the
// SSE endpoint and stream label differ — both managed by the parent app.
package srvlog

import (
	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/view/logview"
)

// Model is a srvlog log view, type-aliased from the generic logview.Model.
type Model = logview.Model[client.SrvlogEvent]

// New creates a new srvlog view model with the appropriate adapter and filter.
func New(bufferSize int, timeFormat string, autoScroll bool) Model {
	filter := newFilter()
	return logview.New(bufferSize, timeFormat, autoScroll, adapter, filter)
}

// adapter is the srvlog-specific Adapter for logview.Model.
var adapter = logview.Adapter[client.SrvlogEvent]{
	Columns: columns,
	Row:     eventToRow,
	Detail:  renderDetailPanel,
	ID:      func(e client.SrvlogEvent) int64 { return e.ID },
}
