package logview_test

import (
	"net/url"
	"testing"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/lasseh/taillight-tui/internal/tui/component"
	"github.com/lasseh/taillight-tui/internal/tui/view/logview"
)

// fakeEvent is a minimal event type for testing logview.Model[T].
type fakeEvent struct {
	ID      int64
	Message string
}

// fakeFilter is a logview.Filter with no feed-specific fields.
type fakeFilter struct{}

func newFakeFilter() *fakeFilter { return &fakeFilter{} }

func (f *fakeFilter) Fields() []component.Field   { return nil }
func (f *fakeFilter) Apply(map[string]string)     {}
func (f *fakeFilter) Params() url.Values          { return url.Values{} }
func (f *fakeFilter) Pills(url.Values) []string   { return nil }
func (f *fakeFilter) SetMeta(map[string][]string) {}

var fakeAdapter = logview.Adapter[fakeEvent]{
	Columns: func(_ int) []table.Column {
		return []table.Column{
			{Title: "ID", Width: 5},
			{Title: "MSG", Width: 50},
		}
	},
	Row: func(e fakeEvent, _ string) table.Row {
		return table.Row{string(rune('0' + e.ID%10)), e.Message}
	},
	Detail: func(e fakeEvent, _ int) string { return e.Message },
	ID:     func(e fakeEvent) int64 { return e.ID },
}

func newTestModel(t *testing.T, bufferSize int) logview.Model[fakeEvent] {
	t.Helper()
	m := logview.New(bufferSize, "15:04:05", true, fakeAdapter, newFakeFilter())
	m.SetSize(120, 40)
	return m
}

func TestNew(t *testing.T) {
	m := newTestModel(t, 100)
	if m.EventCount() != 0 {
		t.Errorf("new model EventCount = %d, want 0", m.EventCount())
	}
	if m.DetailOpen() {
		t.Error("new model should not have detail open")
	}
}

func TestPushEvents(t *testing.T) {
	m := newTestModel(t, 100)
	events := []fakeEvent{
		{ID: 1, Message: "first"},
		{ID: 2, Message: "second"},
		{ID: 3, Message: "third"},
	}
	m.PushEvents(events)
	if got := m.EventCount(); got != 3 {
		t.Errorf("EventCount after PushEvents = %d, want 3", got)
	}
}

func TestPushEventsOverflow(t *testing.T) {
	m := newTestModel(t, 3)
	events := []fakeEvent{
		{ID: 1, Message: "a"},
		{ID: 2, Message: "b"},
		{ID: 3, Message: "c"},
		{ID: 4, Message: "d"},
		{ID: 5, Message: "e"},
	}
	m.PushEvents(events)
	if got := m.EventCount(); got != 3 {
		t.Errorf("EventCount after overflow = %d, want 3", got)
	}
}

func TestClearEvents(t *testing.T) {
	m := newTestModel(t, 100)
	m.PushEvents([]fakeEvent{
		{ID: 1, Message: "stale"},
		{ID: 2, Message: "old"},
	})
	if got := m.EventCount(); got != 2 {
		t.Fatalf("EventCount before ClearEvents = %d, want 2", got)
	}
	m.ClearEvents()
	if got := m.EventCount(); got != 0 {
		t.Errorf("EventCount after ClearEvents = %d, want 0", got)
	}
	// New events after a clear must still render.
	m.PushEvents([]fakeEvent{{ID: 3, Message: "fresh"}})
	if got := m.EventCount(); got != 1 {
		t.Errorf("EventCount after push post-clear = %d, want 1", got)
	}
}

func TestClearEventsClosesDetail(t *testing.T) {
	m := newTestModel(t, 100)
	m.PushEvents([]fakeEvent{{ID: 1, Message: "stale"}})
	m.UpdateTable(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.DetailOpen() {
		t.Fatal("detail should be open after Enter")
	}
	m.ClearEvents()
	if m.DetailOpen() {
		t.Error("detail should close on ClearEvents — its event may no longer match")
	}
}

func TestEmptyStateView(t *testing.T) {
	m := newTestModel(t, 100)
	view := m.View()
	if view == "" {
		t.Error("empty model should render a non-empty view (waiting message)")
	}
}

func TestViewAfterPush(t *testing.T) {
	m := newTestModel(t, 100)
	m.PushEvents([]fakeEvent{{ID: 1, Message: "hello"}})
	view := m.View()
	if view == "" {
		t.Error("view with events should not be empty")
	}
}

func TestFilterFocusBlur(t *testing.T) {
	m := newTestModel(t, 100)
	m.FocusFilter()
	// FocusFilter should not panic; the filter is now focused
	m.BlurFilter()
	// BlurFilter should also not panic
}

func TestSetSizeUpdatesDimensions(t *testing.T) {
	m := newTestModel(t, 100)
	m.SetSize(80, 24)
	// Should not panic; verify by rendering
	if m.View() == "" {
		t.Error("view should render after SetSize")
	}
}

func TestDetailLifecycle(t *testing.T) {
	m := newTestModel(t, 100)
	m.PushEvents([]fakeEvent{{ID: 1, Message: "test"}})
	if m.DetailOpen() {
		t.Error("detail should not be open initially")
	}
	// Send Enter to open detail
	m.UpdateTable(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.DetailOpen() {
		t.Error("detail should be open after Enter")
	}
	m.CloseDetail()
	if m.DetailOpen() {
		t.Error("detail should be closed after CloseDetail")
	}
}
