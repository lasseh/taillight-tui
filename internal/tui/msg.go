package tui

// API response messages.

// MetaLoadedMsg carries typeahead metadata (hosts, programs, etc.) loaded
// from the API for a log tab, keyed by filter field.
type MetaLoadedMsg struct {
	Tab  TabID
	Meta map[string][]string
}

// ErrorMsg carries an error to display in the status bar.
type ErrorMsg struct {
	Err error
}

// ClearErrorMsg clears the error display.
type ClearErrorMsg struct{}

// SSETickMsg triggers draining the SSE channel for batched event delivery.
type SSETickMsg struct{}
