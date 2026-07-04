package client

import "encoding/json"

// Drain reads up to maxEvents pending events from the stream without
// blocking, unmarshalling each payload into T. It returns the decoded events
// in arrival order plus the count of unmarshal failures, so callers can
// surface parse errors instead of silently dropping them. A nil stream
// drains nothing.
func Drain[T any](stream *SSEStream, maxEvents int) (events []T, parseErrors int) {
	if stream == nil {
		return nil, 0
	}
	for range maxEvents {
		select {
		case evt, ok := <-stream.Events():
			if !ok {
				return events, parseErrors
			}
			var e T
			if err := json.Unmarshal(evt.Data, &e); err != nil {
				parseErrors++
				continue
			}
			events = append(events, e)
		default:
			return events, parseErrors
		}
	}
	return events, parseErrors
}
