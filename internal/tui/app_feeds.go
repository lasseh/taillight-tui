package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/component"
	"github.com/lasseh/taillight-tui/internal/tui/feed"
	"github.com/lasseh/taillight-tui/internal/tui/view/applog"
	"github.com/lasseh/taillight-tui/internal/tui/view/logview"
	"github.com/lasseh/taillight-tui/internal/tui/view/srvlog"
)

// drainBatch caps how many events one tick drains from a feed.
const drainBatch = 100

// metaLoadTimeout is the budget for fetching meta (hosts/programs/services).
// Bounded so hung API calls don't leak goroutines forever.
const metaLoadTimeout = 10 * time.Second

// liveTab is a log tab backed by a live feed: the view App routes keys and
// renders through, plus the feed lifecycle. App holds one per log tab and
// never needs to know the tab's event type.
type liveTab interface {
	SetSize(width, height int)
	View() string
	FocusFilter() tea.Cmd
	BlurFilter()
	UpdateFilter(msg tea.Msg) tea.Cmd
	UpdateTable(msg tea.Msg) tea.Cmd
	DetailOpen() bool
	CloseDetail()
	PopupFields() []component.Field
	ApplyPopupValues(values map[string]string)
	SetMeta(meta map[string][]string)

	// start opens the feed's stream with the view's filter unless one is
	// already open. It reports whether a stream was opened.
	start() bool
	// restart reopens the stream when the view's filter changed since it
	// was opened. A reopened stream starts on an empty view; its backfill
	// repopulates it with matching events.
	restart()
	// drain moves pending events into the view, pushes toasts for alerting
	// events, and returns the count of undecodable payloads.
	drain(toasts *component.ToastQueue) (parseErrors int)
	connected() bool
	close()
	// pills renders status bar labels for the filter the open stream runs
	// with — not half-typed input.
	pills() []string
	// loadMeta fetches typeahead metadata. It runs inside a tea.Cmd, so it
	// must not touch view state.
	loadMeta(ctx context.Context) (map[string][]string, error)
}

// logTab is the liveTab for events of type T.
type logTab[T any] struct {
	*logview.Model[T]
	feed  *feed.Feed[T]
	meta  func(ctx context.Context) (map[string][]string, error)
	toast func(e T) (component.Toast, bool)
}

func newLogTab[T any](view logview.Model[T], f *feed.Feed[T],
	meta func(context.Context) (map[string][]string, error),
	toast func(T) (component.Toast, bool),
) *logTab[T] {
	return &logTab[T]{Model: &view, feed: f, meta: meta, toast: toast}
}

func (t *logTab[T]) start() bool {
	if !t.feed.Start(t.Params()) {
		return false
	}
	t.ClearEvents()
	return true
}

func (t *logTab[T]) restart() {
	if t.feed.Restart(t.Params()) {
		t.ClearEvents()
	}
}

func (t *logTab[T]) drain(toasts *component.ToastQueue) int {
	events, parseErrors := t.feed.Drain(drainBatch)
	if len(events) > 0 {
		t.PushEvents(events)
		for i := range events {
			if toast, ok := t.toast(events[i]); ok {
				toasts.Push(toast)
			}
		}
	}
	return parseErrors
}

func (t *logTab[T]) connected() bool { return t.feed.Connected() }
func (t *logTab[T]) close()          { t.feed.Close() }
func (t *logTab[T]) pills() []string { return t.Pills(t.feed.Params()) }

func (t *logTab[T]) loadMeta(ctx context.Context) (map[string][]string, error) {
	return t.meta(ctx)
}

// newLogTabs builds the live log tabs. Every stream goes through open, so
// tests can substitute an in-memory adapter.
func newLogTabs(cfg Config, c *client.Client, open feed.Opener) map[TabID]liveTab {
	return map[TabID]liveTab{
		TabSrvlog: newLogTab(
			srvlog.New(cfg.BufferSize, cfg.TimeFormat, cfg.AutoScroll),
			feed.New[client.SrvlogEvent]("/api/v1/srvlog/stream", open),
			syslogMeta("srvlog", c.SrvlogHosts, c.SrvlogPrograms),
			syslogToast("srvlog"),
		),
		TabNetlog: newLogTab(
			srvlog.New(cfg.BufferSize, cfg.TimeFormat, cfg.AutoScroll),
			feed.New[client.SrvlogEvent]("/api/v1/netlog/stream", open),
			syslogMeta("netlog", c.NetlogHosts, c.NetlogPrograms),
			syslogToast("netlog"),
		),
		TabApplog: newLogTab(
			applog.New(cfg.BufferSize, cfg.TimeFormat, cfg.AutoScroll),
			feed.New[client.AppLogEvent]("/api/v1/applog/stream", open),
			applogMeta(c),
			applogToast,
		),
	}
}

type listFunc func(ctx context.Context) ([]string, error)

// syslogMeta loads host and program typeahead for a syslog-shaped feed.
func syslogMeta(name string, hosts, programs listFunc) func(context.Context) (map[string][]string, error) {
	return func(ctx context.Context) (map[string][]string, error) {
		hostList, err := hosts(ctx)
		if err != nil {
			return nil, fmt.Errorf("%s hosts: %w", name, err)
		}
		progList, err := programs(ctx)
		if err != nil {
			return nil, fmt.Errorf("%s programs: %w", name, err)
		}
		return srvlog.Meta(hostList, progList), nil
	}
}

func applogMeta(c *client.Client) func(context.Context) (map[string][]string, error) {
	return func(ctx context.Context) (map[string][]string, error) {
		svcList, err := c.AppLogServices(ctx)
		if err != nil {
			return nil, fmt.Errorf("applog services: %w", err)
		}
		compList, err := c.AppLogComponents(ctx)
		if err != nil {
			return nil, fmt.Errorf("applog components: %w", err)
		}
		hostList, err := c.AppLogHosts(ctx)
		if err != nil {
			return nil, fmt.Errorf("applog hosts: %w", err)
		}
		return applog.Meta(svcList, compList, hostList), nil
	}
}

// loadMeta returns a Cmd that fetches the tab's typeahead metadata.
func (a *App) loadMeta(id TabID, t liveTab) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), metaLoadTimeout)
		defer cancel()
		meta, err := t.loadMeta(ctx)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return MetaLoadedMsg{Tab: id, Meta: meta}
	}
}
