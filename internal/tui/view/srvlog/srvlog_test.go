package srvlog_test

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/view/srvlog"
)

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
