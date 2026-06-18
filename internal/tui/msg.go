package tui

import (
	"github.com/lasseh/taillight-tui/internal/tui/client"
)

// StreamStartedMsg carries a newly created SSE stream back to the Update
// handler for safe assignment. Cmds must not mutate the model directly.
type StreamStartedMsg struct {
	Feed   string // "srvlog", "applog", "netlog"
	Stream *client.SSEStream
}

// API response messages.

// MetaLoadedMsg carries metadata (hosts, programs, etc.) loaded from the API.
type MetaLoadedMsg struct {
	Feed       string
	Hosts      []string
	Programs   []string
	Services   []string // applog only
	Components []string // applog only
}

// ErrorMsg carries an error to display in the status bar.
type ErrorMsg struct {
	Err error
}

// ClearErrorMsg clears the error display.
type ClearErrorMsg struct{}

// SSETickMsg triggers draining the SSE channel for batched event delivery.
type SSETickMsg struct{}
