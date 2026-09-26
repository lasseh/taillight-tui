package feed_test

import (
	"net/url"
	"testing"

	"github.com/lasseh/taillight-tui/internal/tui/feed"
	"github.com/lasseh/taillight-tui/internal/tui/feed/feedtest"
)

type payload struct {
	ID int `json:"id"`
}

const path = "/api/v1/srvlog/stream"

func newFeed() (*feed.Feed[payload], *feedtest.Opener) {
	open := &feedtest.Opener{}
	return feed.New[payload](path, open.Open), open
}

func TestStartOpensOnce(t *testing.T) {
	f, open := newFeed()
	if !f.Start(url.Values{"search": {"a"}}) {
		t.Fatal("first Start must open a stream")
	}
	if f.Start(url.Values{"search": {"b"}}) {
		t.Error("Start with a stream open must be a no-op, even with new params")
	}
	if len(open.Opened) != 1 {
		t.Fatalf("opened %d streams, want 1", len(open.Opened))
	}
	if got := open.Opened[0].Path; got != path {
		t.Errorf("path = %q, want %q", got, path)
	}
	if got := f.Params(); got != "search=a" {
		t.Errorf("Params() = %q, want the open stream's params", got)
	}
}

func TestRestart(t *testing.T) {
	tests := []struct {
		name       string
		started    bool
		params     url.Values
		wantOpened bool
		wantClosed bool
	}{
		{name: "unchanged params is a no-op", started: true, params: url.Values{"search": {"a"}}, wantOpened: false},
		{name: "changed params replaces the stream", started: true, params: url.Values{"search": {"b"}}, wantOpened: true, wantClosed: true},
		{name: "no stream opens one", started: false, params: url.Values{"search": {"a"}}, wantOpened: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, open := newFeed()
			if tt.started {
				f.Start(url.Values{"search": {"a"}})
			}
			var first *feedtest.Stream
			if len(open.Opened) > 0 {
				first = open.Opened[0]
			}

			if got := f.Restart(tt.params); got != tt.wantOpened {
				t.Errorf("Restart() = %v, want %v", got, tt.wantOpened)
			}
			if first != nil && first.Closed() != tt.wantClosed {
				t.Errorf("previous stream closed = %v, want %v", first.Closed(), tt.wantClosed)
			}
			if got := f.Params(); got != tt.params.Encode() {
				t.Errorf("Params() = %q, want %q", got, tt.params.Encode())
			}
		})
	}
}

func TestDrain(t *testing.T) {
	tests := []struct {
		name        string
		data        []string // raw payloads queued on the stream
		end         bool
		maxEvents   int
		wantIDs     []int
		wantParsErr int
	}{
		{name: "empty stream", maxEvents: 10},
		{name: "events in arrival order", data: []string{`{"id":1}`, `{"id":2}`, `{"id":3}`}, maxEvents: 10, wantIDs: []int{1, 2, 3}},
		{name: "malformed payload counted and skipped", data: []string{`{"id":1}`, `{not json`, `{"id":3}`}, maxEvents: 10, wantIDs: []int{1, 3}, wantParsErr: 1},
		{name: "maxEvents caps the batch", data: []string{`{"id":1}`, `{"id":2}`, `{"id":3}`}, maxEvents: 2, wantIDs: []int{1, 2}},
		{name: "ended stream returns buffered events", data: []string{`{"id":1}`, `{"id":2}`}, end: true, maxEvents: 10, wantIDs: []int{1, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, open := newFeed()
			f.Start(nil)
			s := open.Opened[0]
			for _, d := range tt.data {
				s.SendRaw(d)
			}
			if tt.end {
				s.End()
			}
			events, parseErrs := f.Drain(tt.maxEvents)
			if parseErrs != tt.wantParsErr {
				t.Errorf("parseErrors = %d, want %d", parseErrs, tt.wantParsErr)
			}
			if len(events) != len(tt.wantIDs) {
				t.Fatalf("got %d events, want %d", len(events), len(tt.wantIDs))
			}
			for i, id := range tt.wantIDs {
				if events[i].ID != id {
					t.Errorf("event %d ID = %d, want %d", i, events[i].ID, id)
				}
			}
		})
	}
}

func TestDrainWithoutStream(t *testing.T) {
	f, _ := newFeed()
	if events, parseErrs := f.Drain(10); events != nil || parseErrs != 0 {
		t.Errorf("Drain() = (%v, %d), want (nil, 0)", events, parseErrs)
	}
	if f.Connected() {
		t.Error("feed without a stream must not report connected")
	}
}

func TestCloseKeepsParamsAndAllowsStart(t *testing.T) {
	f, open := newFeed()
	f.Start(url.Values{"search": {"a"}})
	f.Close()
	f.Close() // idempotent

	if !open.Opened[0].Closed() {
		t.Error("Close must close the stream")
	}
	if got := f.Params(); got != "search=a" {
		t.Errorf("Params() after Close = %q, want the last filter kept", got)
	}
	if !f.Start(url.Values{"search": {"a"}}) {
		t.Error("Start after Close must open a new stream")
	}
}
