package srvlog_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/view/srvlog"
)

// TestFilterRoundTrip guards the popup → stream params → status bar pill
// path: popup keys are the query params, pills are compact and in fixed
// display order with search last.
func TestFilterRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		values     map[string]string
		wantParams string
		wantPills  []string
	}{
		{name: "empty", values: map[string]string{}, wantParams: "", wantPills: nil},
		{name: "search only", values: map[string]string{"search": "error"}, wantParams: "search=error", wantPills: []string{"/error"}},
		{
			name:       "fields in display order",
			values:     map[string]string{"severity_max": "4", "hostname": "core1", "search": "bgp", "programname": "rpd", "facility": "23"},
			wantParams: "facility=23&hostname=core1&programname=rpd&search=bgp&severity_max=4",
			wantPills:  []string{"host=core1", "prog=rpd", "sev≤4", "fac=23", "/bgp"},
		},
		{
			name:       "empty dropdown value clears",
			values:     map[string]string{"severity_max": ""},
			wantParams: "",
			wantPills:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := srvlog.New(10, "15:04:05", true)
			m.ApplyPopupValues(map[string]string{"severity_max": "2"})
			m.ApplyPopupValues(map[string]string{"severity_max": ""})
			m.ApplyPopupValues(tt.values)

			encoded := m.Params().Encode()
			if encoded != tt.wantParams {
				t.Errorf("Params() = %q, want %q", encoded, tt.wantParams)
			}
			if got := m.Pills(encoded); !slices.Equal(got, tt.wantPills) {
				t.Errorf("Pills(%q) = %v, want %v", encoded, got, tt.wantPills)
			}
		})
	}
}

func TestPopupFieldsCarryMetaAndValues(t *testing.T) {
	m := srvlog.New(10, "15:04:05", true)
	m.SetMeta(srvlog.Meta([]string{"core1", "core2"}, []string{"rpd"}))
	m.ApplyPopupValues(map[string]string{"hostname": "core1", "search": "bgp"})

	fields := m.PopupFields()
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.Key)
	}
	if want := []string{"hostname", "programname", "severity_max", "facility", "search"}; !slices.Equal(keys, want) {
		t.Fatalf("popup keys = %v, want %v", keys, want)
	}
	if fields[0].Value != "core1" || !slices.Equal(fields[0].Suggestions, []string{"core1", "core2"}) {
		t.Errorf("hostname field = %+v, want value core1 with host suggestions", fields[0])
	}
	if fields[1].Suggestions[0] != "rpd" {
		t.Errorf("programname suggestions = %v, want [rpd]", fields[1].Suggestions)
	}
	if fields[4].Value != "bgp" {
		t.Errorf("search field value = %q, want bgp", fields[4].Value)
	}
}

func TestCellsUseColumnWidth(t *testing.T) {
	const host = "core-sw-01.osl.example.net" // 26 chars
	const prog = "rpd-routing-daemon"         // 18 chars
	event := client.SrvlogEvent{ID: 1, Hostname: host, Programname: prog, ReceivedAt: time.Now()}

	tests := []struct {
		name      string
		width     int
		wantFull  bool
		wantClipd bool
	}{
		{name: "wide terminal shows full values", width: 240, wantFull: true},
		{name: "narrow terminal clips to column", width: 80, wantClipd: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := srvlog.New(10, "15:04:05", true)
			m.SetSize(tt.width, 20)
			m.PushEvents([]client.SrvlogEvent{event})
			view := ansi.Strip(m.View())

			full := strings.Contains(view, host) && strings.Contains(view, prog)
			if full != tt.wantFull {
				t.Errorf("full host/program shown = %v, want %v\n%s", full, tt.wantFull, view)
			}
			if clipped := strings.Contains(view, "…"); clipped != tt.wantClipd {
				t.Errorf("ellipsis shown = %v, want %v\n%s", clipped, tt.wantClipd, view)
			}
		})
	}
}
