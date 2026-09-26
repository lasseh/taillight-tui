// Package feedtest provides an in-memory feed.Stream adapter so feed owners
// can be tested without network connections.
package feedtest

import (
	"encoding/json"
	"net/url"
	"sync"
	"testing"

	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/feed"
)

// Stream is an in-memory feed.Stream. Tests push events with Send and end it
// with End.
type Stream struct {
	Path   string
	Params url.Values

	ch        chan client.SSEEvent
	connected bool
	closed    bool
	err       error
	endOnce   sync.Once
}

// Err implements feed.Stream.
func (s *Stream) Err() error { return s.err }

// Fail ends the stream with err, as a stream does after e.g. a 401.
func (s *Stream) Fail(err error) {
	s.err = err
	s.End()
}

func newStream(path string, params url.Values) *Stream {
	return &Stream{Path: path, Params: params, ch: make(chan client.SSEEvent, 256), connected: true}
}

// Events implements feed.Stream.
func (s *Stream) Events() <-chan client.SSEEvent { return s.ch }

// Connected implements feed.Stream.
func (s *Stream) Connected() bool { return s.connected && !s.closed }

// Close implements feed.Stream.
func (s *Stream) Close() {
	s.closed = true
	s.End()
}

// Closed reports whether the owner closed the stream.
func (s *Stream) Closed() bool { return s.closed }

// Send queues v, JSON-encoded, as the next event.
func (s *Stream) Send(t *testing.T, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("feedtest: marshal event: %v", err)
	}
	s.ch <- client.SSEEvent{Data: data}
}

// SendRaw queues a raw payload, e.g. malformed JSON.
func (s *Stream) SendRaw(data string) {
	s.ch <- client.SSEEvent{Data: json.RawMessage(data)}
}

// End closes the event channel, as a stream does when it stops for good.
func (s *Stream) End() {
	s.endOnce.Do(func() {
		s.connected = false
		close(s.ch)
	})
}

// Opener records every stream it opens.
type Opener struct {
	Opened []*Stream
}

// Open is the feed.Opener.
func (o *Opener) Open(path string, params url.Values) feed.Stream {
	s := newStream(path, params)
	o.Opened = append(o.Opened, s)
	return s
}

// Last returns the most recently opened stream for path, or nil.
func (o *Opener) Last(path string) *Stream {
	for i := len(o.Opened) - 1; i >= 0; i-- {
		if o.Opened[i].Path == path {
			return o.Opened[i]
		}
	}
	return nil
}
