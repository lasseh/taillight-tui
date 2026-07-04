package client

import (
	"encoding/json"
	"testing"
)

func TestDrain(t *testing.T) {
	type payload struct {
		ID int `json:"id"`
	}
	tests := []struct {
		name        string
		data        []string // raw payloads buffered on the channel
		closeCh     bool
		maxEvents   int
		wantIDs     []int
		wantParsErr int
	}{
		{
			name:      "empty channel",
			maxEvents: 10,
		},
		{
			name:      "events in arrival order",
			data:      []string{`{"id":1}`, `{"id":2}`, `{"id":3}`},
			maxEvents: 10,
			wantIDs:   []int{1, 2, 3},
		},
		{
			name:        "malformed payload counted and skipped",
			data:        []string{`{"id":1}`, `{not json`, `{"id":3}`},
			maxEvents:   10,
			wantIDs:     []int{1, 3},
			wantParsErr: 1,
		},
		{
			name:      "maxEvents caps the batch",
			data:      []string{`{"id":1}`, `{"id":2}`, `{"id":3}`},
			maxEvents: 2,
			wantIDs:   []int{1, 2},
		},
		{
			name:      "closed channel returns buffered events",
			data:      []string{`{"id":1}`, `{"id":2}`},
			closeCh:   true,
			maxEvents: 10,
			wantIDs:   []int{1, 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SSEStream{eventCh: make(chan SSEEvent, len(tt.data)+1)}
			for _, d := range tt.data {
				s.eventCh <- SSEEvent{Data: json.RawMessage(d)}
			}
			if tt.closeCh {
				close(s.eventCh)
			}
			events, parseErrs := Drain[payload](s, tt.maxEvents)
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

func TestDrainNilStream(t *testing.T) {
	events, parseErrs := Drain[struct{}](nil, 10)
	if events != nil || parseErrs != 0 {
		t.Errorf("Drain(nil) = (%v, %d), want (nil, 0)", events, parseErrs)
	}
}
