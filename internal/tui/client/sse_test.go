package client

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSplitSSEField(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		field string
		value string
		ok    bool
	}{
		{name: "space form", line: "data: hello", field: "data", value: "hello", ok: true},
		{name: "no-space form", line: "data:hello", field: "data", value: "hello", ok: true},
		{name: "only one space stripped", line: "data:  padded", field: "data", value: " padded", ok: true},
		{name: "id space form", line: "id: 42", field: "id", value: "42", ok: true},
		{name: "id no-space form", line: "id:42", field: "id", value: "42", ok: true},
		{name: "empty value", line: "data:", field: "data", value: "", ok: true},
		{name: "empty value with space", line: "data: ", field: "data", value: "", ok: true},
		{name: "value containing colon", line: "data: a:b", field: "data", value: "a:b", ok: true},
		{name: "comment line", line: ":heartbeat", field: "", value: "", ok: false},
		{name: "bare field name", line: "data", field: "", value: "", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, value, ok := splitSSEField(tt.line)
			if field != tt.field || value != tt.value || ok != tt.ok {
				t.Errorf("splitSSEField(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.line, field, value, ok, tt.field, tt.value, tt.ok)
			}
		})
	}
}

func TestReadEventsParsing(t *testing.T) {
	type want struct {
		id    int64
		event string
		data  string
	}
	tests := []struct {
		name   string
		input  string
		want   []want
		lastID int64
	}{
		{
			name:   "single event space form",
			input:  "id: 1\nevent: srvlog\ndata: {\"a\":1}\n\n",
			want:   []want{{id: 1, event: "srvlog", data: `{"a":1}`}},
			lastID: 1,
		},
		{
			name:   "no-space form",
			input:  "id:2\nevent:applog\ndata:{\"b\":2}\n\n",
			want:   []want{{id: 2, event: "applog", data: `{"b":2}`}},
			lastID: 2,
		},
		{
			name:   "comment lines ignored",
			input:  ":keepalive\nid: 3\nevent: srvlog\n:mid-event comment\ndata: x\n\n",
			want:   []want{{id: 3, event: "srvlog", data: "x"}},
			lastID: 3,
		},
		{
			name:   "heartbeat never delivered",
			input:  "event: heartbeat\ndata: \n\n",
			want:   nil,
			lastID: 0,
		},
		{
			name:   "multi-line data joined with newline",
			input:  "id: 4\nevent: srvlog\ndata: line1\ndata: line2\n\n",
			want:   []want{{id: 4, event: "srvlog", data: "line1\nline2"}},
			lastID: 4,
		},
		{
			name:   "event without data not dispatched",
			input:  "id: 5\nevent: srvlog\n\n",
			want:   nil,
			lastID: 0,
		},
		{
			name:   "unknown field ignored",
			input:  "retry: 5000\nid: 6\nevent: srvlog\ndata: y\n\n",
			want:   []want{{id: 6, event: "srvlog", data: "y"}},
			lastID: 6,
		},
		{
			name: "multiple events track last id",
			input: "id: 7\nevent: srvlog\ndata: a\n\n" +
				"id: 8\nevent: srvlog\ndata: b\n\n",
			want: []want{
				{id: 7, event: "srvlog", data: "a"},
				{id: 8, event: "srvlog", data: "b"},
			},
			lastID: 8,
		},
		{
			name:   "unterminated event not dispatched",
			input:  "id: 9\nevent: srvlog\ndata: z",
			want:   nil,
			lastID: 0,
		},
		{
			name:   "malformed id parses as zero",
			input:  "id: abc\nevent: srvlog\ndata: q\n\n",
			want:   []want{{id: 0, event: "srvlog", data: "q"}},
			lastID: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SSEStream{eventCh: make(chan SSEEvent, 16)}
			if err := s.readEvents(context.Background(), strings.NewReader(tt.input)); err != nil {
				t.Fatalf("readEvents: %v", err)
			}
			got := make([]SSEEvent, 0, len(tt.want))
		drain:
			for {
				select {
				case evt := <-s.eventCh:
					got = append(got, evt)
				default:
					break drain
				}
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d events, want %d", len(got), len(tt.want))
			}
			for i, w := range tt.want {
				if got[i].ID != w.id || got[i].Event != w.event || string(got[i].Data) != w.data {
					t.Errorf("event %d = (%d, %q, %q), want (%d, %q, %q)",
						i, got[i].ID, got[i].Event, got[i].Data, w.id, w.event, w.data)
				}
			}
			if s.LastID() != tt.lastID {
				t.Errorf("LastID() = %d, want %d", s.LastID(), tt.lastID)
			}
		})
	}
}

// TestReadEventsLastIDAdvancesOnlyOnDelivery pins the resume semantics: with
// the channel full, the reader must block rather than drop, and lastID must
// not advance past events that were never delivered — otherwise a reconnect's
// Last-Event-ID backfill would silently skip them.
func TestReadEventsLastIDAdvancesOnlyOnDelivery(t *testing.T) {
	input := "id: 1\nevent: srvlog\ndata: a\n\n" +
		"id: 2\nevent: srvlog\ndata: b\n\n" +
		"id: 3\nevent: srvlog\ndata: c\n\n"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := &SSEStream{eventCh: make(chan SSEEvent, 1)}
	done := make(chan error, 1)
	go func() { done <- s.readEvents(ctx, strings.NewReader(input)) }()

	// Wait until event 1 is delivered; the reader is then blocked sending
	// event 2 into the full channel and cannot advance lastID further.
	deadline := time.After(5 * time.Second)
	for s.LastID() != 1 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for first delivery, LastID() = %d", s.LastID())
		case <-time.After(time.Millisecond):
		}
	}

	// Cancel while blocked: the reader must abandon the stream without
	// recording the undelivered event as seen.
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("readEvents = %v, want context.Canceled", err)
	}
	if got := s.LastID(); got != 1 {
		t.Errorf("LastID() = %d, want 1 — undelivered events must not advance it", got)
	}
	if got := len(s.eventCh); got != 1 {
		t.Errorf("len(eventCh) = %d, want 1", got)
	}
}

// TestReadEventsDeliversAllUnderDrain verifies the blocking send makes
// progress once the consumer drains: every event arrives exactly once and
// lastID ends at the final delivered ID.
func TestReadEventsDeliversAllUnderDrain(t *testing.T) {
	input := "id: 1\nevent: srvlog\ndata: a\n\n" +
		"id: 2\nevent: srvlog\ndata: b\n\n" +
		"id: 3\nevent: srvlog\ndata: c\n\n"

	s := &SSEStream{eventCh: make(chan SSEEvent, 1)}
	done := make(chan error, 1)
	go func() { done <- s.readEvents(context.Background(), strings.NewReader(input)) }()

	for i := int64(1); i <= 3; i++ {
		select {
		case evt := <-s.eventCh:
			if evt.ID != i {
				t.Fatalf("event ID = %d, want %d", evt.ID, i)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
	if err := <-done; err != nil {
		t.Fatalf("readEvents: %v", err)
	}
	if got := s.LastID(); got != 3 {
		t.Errorf("LastID() = %d, want 3", got)
	}
}
