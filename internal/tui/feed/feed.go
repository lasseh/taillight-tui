// Package feed owns the lifecycle of one live SSE event feed: when its
// stream opens, when a filter change reopens it, draining decoded events,
// and closing it. Views decide what to do with the events; the feed decides
// when a stream exists.
package feed

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/lasseh/taillight-tui/internal/tui/client"
)

// Stream is the live event source a Feed drains. *client.SSEStream is the
// production adapter; feedtest.Stream is the in-memory adapter for tests.
type Stream interface {
	// Events delivers parsed events; it is closed when the stream stops
	// for good.
	Events() <-chan client.SSEEvent
	Connected() bool
	Close()
	// Err reports why a stopped stream ended (e.g. client.ErrUnauthorized).
	Err() error
}

// Opener starts a stream for an SSE path with filter query params. Opening
// must not block: the stream connects in the background.
type Opener func(path string, params url.Values) Stream

// SSEOpener returns the production Opener backed by c.
func SSEOpener(c *client.Client) Opener {
	return func(path string, params url.Values) Stream {
		return client.NewSSEStream(c, path, params, 0)
	}
}

// Feed is one SSE stream of events of type T. The zero value is not usable;
// construct with New.
//
// Invariants: at most one stream is open; a replaced stream is always closed;
// Params reports the params the open stream was started with.
type Feed[T any] struct {
	path   string
	open   Opener
	stream Stream
	params string // encoded params of the open stream
}

// New returns a Feed for the SSE endpoint at path. No stream is opened until
// Start or Restart.
func New[T any](path string, open Opener) *Feed[T] {
	return &Feed[T]{path: path, open: open}
}

// Start opens a stream with params unless one is already open. It reports
// whether a stream was opened; a new stream backfills recent events, so the
// caller should drop what it has buffered.
func (f *Feed[T]) Start(params url.Values) (opened bool) {
	if f.stream != nil {
		return false
	}
	f.openStream(params)
	return true
}

// Restart makes the open stream match params: a no-op when a stream is
// already open with identical params, otherwise it closes any open stream
// and opens a fresh one. It reports whether a stream was opened.
func (f *Feed[T]) Restart(params url.Values) (opened bool) {
	if f.stream != nil && f.params == params.Encode() {
		return false
	}
	f.Close()
	f.openStream(params)
	return true
}

func (f *Feed[T]) openStream(params url.Values) {
	f.stream = f.open(f.path, params)
	f.params = params.Encode()
}

// Drain reads up to maxEvents pending events without blocking, decoding each
// payload into T. It returns the events in arrival order plus the count of
// payloads that failed to decode. Draining a feed with no stream is a no-op.
//
// When the stream has stopped for good, Drain drops it — so the next Start
// opens a fresh one — and returns why it stopped, wrapped with the feed's
// path. Events delivered before the stop are still returned.
func (f *Feed[T]) Drain(maxEvents int) (events []T, parseErrors int, err error) {
	if f.stream == nil {
		return nil, 0, nil
	}
	for range maxEvents {
		select {
		case evt, ok := <-f.stream.Events():
			if !ok {
				if serr := f.stream.Err(); serr != nil {
					err = fmt.Errorf("%s: %w", f.path, serr)
				}
				f.Close()
				return events, parseErrors, err
			}
			var e T
			if uerr := json.Unmarshal(evt.Data, &e); uerr != nil {
				parseErrors++
				continue
			}
			events = append(events, e)
		default:
			return events, parseErrors, nil
		}
	}
	return events, parseErrors, nil
}

// Connected reports whether the feed's stream is currently connected.
func (f *Feed[T]) Connected() bool {
	return f.stream != nil && f.stream.Connected()
}

// Params returns the encoded query params the open stream was started with,
// or "" when no stream has been opened.
func (f *Feed[T]) Params() string {
	return f.params
}

// Close closes the open stream, if any. Safe to call repeatedly. Params are
// kept so the last filter in effect stays visible.
func (f *Feed[T]) Close() {
	if f.stream != nil {
		f.stream.Close()
		f.stream = nil
	}
}
