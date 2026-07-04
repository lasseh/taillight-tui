package highlight

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		maxWidth int
		want     string
	}{
		{name: "shorter unchanged", s: "host", maxWidth: 10, want: "host"},
		{name: "exact width unchanged", s: "hostname", maxWidth: 8, want: "hostname"},
		{name: "ascii cut with ellipsis", s: "verylonghostname", maxWidth: 8, want: "veryl..."},
		{name: "tiny budget hard cut", s: "hostname", maxWidth: 3, want: "hos"},
		{name: "zero budget", s: "hostname", maxWidth: 0, want: ""},
		{name: "multi-byte fits by width not bytes", s: "høst-ø1", maxWidth: 7, want: "høst-ø1"},
		{name: "multi-byte cut on rune boundary", s: "øøøøøøøøøø", maxWidth: 5, want: "øø..."},
		{name: "cjk fits", s: "日本語", maxWidth: 6, want: "日本語"},
		{name: "cjk cut counts double width", s: "日本語テスト", maxWidth: 7, want: "日本..."},
		{name: "emoji cut counts double width", s: "🔥🔥🔥", maxWidth: 5, want: "🔥..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.s, tt.maxWidth)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.maxWidth, got, tt.want)
			}
			if w := ansi.StringWidth(got); w > tt.maxWidth {
				t.Errorf("Truncate(%q, %d) width = %d, exceeds budget", tt.s, tt.maxWidth, w)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{name: "pad ascii", s: "WARN", width: 6, want: "WARN  "},
		{name: "exact width unchanged", s: "ERROR", width: 5, want: "ERROR"},
		{name: "wider truncated without ellipsis", s: "EMERGENCY", width: 5, want: "EMERG"},
		{name: "zero width", s: "x", width: 0, want: ""},
		{name: "multi-byte padded by display width", s: "høst", width: 6, want: "høst  "},
		{name: "cjk padded by display width", s: "日本", width: 6, want: "日本  "},
		{name: "cjk truncated then padded to exact cells", s: "日本語", width: 5, want: "日本 "},
		{name: "emoji padded by display width", s: "🔥", width: 4, want: "🔥  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadRight(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("PadRight(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
			if w := ansi.StringWidth(got); w != tt.width {
				t.Errorf("PadRight(%q, %d) width = %d, want exactly %d", tt.s, tt.width, w, tt.width)
			}
		})
	}
}
