package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// SSEEvent is a parsed Server-Sent Event.
type SSEEvent struct {
	ID    int64
	Event string          // "srvlog", "applog", "netlog" — heartbeats are never delivered
	Data  json.RawMessage // raw JSON payload
}

// SSEStream manages a long-lived SSE connection with automatic reconnection.
type SSEStream struct {
	eventCh   chan SSEEvent
	cancel    context.CancelFunc
	connected atomic.Bool
	lastID    atomic.Int64
	// authFailed is set when the server responds 401/403. The stream stops
	// retrying — retrying auth failures indefinitely is wrong — and Err
	// reports it to the owner. Stored before the event channel closes.
	authFailed atomic.Bool
}

// NewSSEStream starts an SSE stream to the given path with filter parameters.
// Events are delivered on the returned stream's channel. The stream reconnects
// automatically on disconnection with exponential backoff.
func NewSSEStream(c *Client, path string, params url.Values, lastEventID int64) *SSEStream {
	ctx, cancel := context.WithCancel(context.Background())
	s := &SSEStream{
		eventCh: make(chan SSEEvent, 256),
		cancel:  cancel,
	}
	s.lastID.Store(lastEventID)

	go s.run(ctx, c, path, params)
	return s
}

// Events returns the channel on which parsed SSE events are delivered.
func (s *SSEStream) Events() <-chan SSEEvent {
	return s.eventCh
}

// Connected reports whether the stream is currently connected.
func (s *SSEStream) Connected() bool {
	return s.connected.Load()
}

// LastID returns the ID of the most recently delivered event.
func (s *SSEStream) LastID() int64 {
	return s.lastID.Load()
}

// Err reports why the stream stopped for good: ErrUnauthorized after the
// server rejected the credentials, nil while running or after Close. Once the
// event channel is closed, Err is final.
func (s *SSEStream) Err() error {
	if s.authFailed.Load() {
		return ErrUnauthorized
	}
	return nil
}

// Close stops the stream and closes the event channel.
func (s *SSEStream) Close() {
	s.cancel()
}

func (s *SSEStream) run(ctx context.Context, c *Client, path string, params url.Values) {
	defer close(s.eventCh)

	httpClient := c.SSEClient()
	attempt := 0

	// A session must stay connected at least this long to count as healthy
	// and reset the backoff. Without this floor, a server that accepts then
	// instantly drops every connection would spin in a tight reconnect loop
	// (a successful handshake resets attempt, so the next retry has no delay).
	const minHealthy = 5 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if attempt > 0 {
			delay := backoff(attempt)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}

		start := time.Now()
		err := s.connect(ctx, httpClient, c, path, params)
		if ctx.Err() != nil {
			return // cancelled
		}
		// connect only ever sets connected=true (on a successful handshake);
		// run is what resets it. A true reading here means this attempt
		// connected before the stream dropped.
		healthy := s.connected.Load() && time.Since(start) >= minHealthy
		s.connected.Store(false)
		if errors.Is(err, ErrUnauthorized) {
			// Server rejected our credentials — stop retrying.
			s.authFailed.Store(true)
			return
		}
		if healthy {
			// Healthy session that dropped — restart backoff so the next
			// reconnect retries quickly instead of inheriting the saturated
			// delay accumulated from earlier failures.
			attempt = 0
		} else {
			// Never connected, or dropped almost immediately — keep backing
			// off so a flapping server can't cause a hot reconnect loop.
			attempt++
		}
	}
}

func (s *SSEStream) connect(ctx context.Context, httpClient *http.Client, c *Client, path string, params url.Values) error {
	u, err := url.JoinPath(c.BaseURL(), path)
	if err != nil {
		return fmt.Errorf("build URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.URL.RawQuery = params.Encode()
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if c.APIKey() != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey())
	}
	if lastID := s.lastID.Load(); lastID > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatInt(lastID, 10))
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connect: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable.

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE status %d", resp.StatusCode)
	}

	s.connected.Store(true)

	return s.readEvents(ctx, resp.Body)
}

// readEvents scans SSE lines from r, assembles events, and delivers them on
// s.eventCh until r is exhausted or ctx is done.
func (s *SSEStream) readEvents(ctx context.Context, r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	var (
		eventID    int64
		eventType  string
		dataBuffer strings.Builder
	)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		if line == "" {
			// Empty line = event dispatch.
			if dataBuffer.Len() > 0 && eventType != "heartbeat" {
				evt := SSEEvent{
					ID:    eventID,
					Event: eventType,
					Data:  json.RawMessage(dataBuffer.String()),
				}
				// Blocking send: Last-Event-ID resume is only honest if
				// lastID means "delivered to the consumer", so events must
				// not be dropped here. The UI drains the channel in batches
				// every tick, so a full channel is transient backpressure on
				// this read loop; if the consumer stalls entirely, the
				// server's SSE write deadline drops the connection and
				// reconnect+backfill resumes from the last delivered event.
				select {
				case s.eventCh <- evt:
				case <-ctx.Done():
					return ctx.Err()
				}
				// Advance only after a successful send — an undelivered
				// event must be replayed by the next reconnect's backfill.
				if eventID > 0 {
					s.lastID.Store(eventID)
				}
			}
			// Reset for next event.
			eventID = 0
			eventType = ""
			dataBuffer.Reset()
			continue
		}

		field, value, ok := splitSSEField(line)
		if !ok {
			continue // comment line or line without a colon — ignore
		}
		switch field {
		case "id":
			eventID, _ = strconv.ParseInt(value, 10, 64)
		case "event":
			eventType = value
		case "data":
			if dataBuffer.Len() > 0 {
				dataBuffer.WriteByte('\n')
			}
			dataBuffer.WriteString(value)
		default:
			// Unknown field (e.g. "retry") — ignore.
		}
	}

	return scanner.Err()
}

// splitSSEField splits an SSE line into field name and value. Per the SSE
// spec the first colon ends the field name and a single space after it is
// stripped from the value, so "id: 5" and "id:5" are equivalent. Comment
// lines (leading colon) and lines without a colon report ok = false.
func splitSSEField(line string) (field, value string, ok bool) {
	idx := strings.IndexByte(line, ':')
	if idx <= 0 {
		return "", "", false
	}
	return line[:idx], strings.TrimPrefix(line[idx+1:], " "), true
}

// backoff returns a delay with exponential backoff and jitter.
// attempt 1 = ~1s, attempt 2 = ~2s, ..., capped at 30s.
func backoff(attempt int) time.Duration {
	base := math.Min(float64(attempt)*1000, 30000)
	jitter := rand.Float64() * base * 0.3 //nolint:gosec // non-crypto jitter
	return time.Duration(base+jitter) * time.Millisecond
}
