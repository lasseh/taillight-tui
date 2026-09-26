package applog_test

import (
	"slices"
	"testing"

	"github.com/lasseh/taillight-tui/internal/tui/view/applog"
)

// TestFilterRoundTrip guards the popup → stream params → status bar pill
// path for the applog param vocabulary.
func TestFilterRoundTrip(t *testing.T) {
	m := applog.New(10, "15:04:05", true)
	m.ApplyPopupValues(map[string]string{
		"service":   "api",
		"level":     "WARN",
		"component": "ingest",
		"host":      "web1",
		"search":    "timeout",
	})

	encoded := m.Params().Encode()
	if want := "component=ingest&host=web1&level=WARN&search=timeout&service=api"; encoded != want {
		t.Errorf("Params() = %q, want %q", encoded, want)
	}
	want := []string{"host=web1", "svc=api", "comp=ingest", "lvl=WARN", "/timeout"}
	if got := m.Pills(encoded); !slices.Equal(got, want) {
		t.Errorf("Pills(%q) = %v, want %v", encoded, got, want)
	}
}
