package component

import (
	"strings"
	"testing"
)

// TestStatusBarView guards the status bar segments: filter pills must render
// when set, be truncated when long, and be dropped (not wrapped) when they
// would collide with the right-aligned help segment; the parse-error counter
// must appear only when failures occurred.
func TestStatusBarView(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(s *StatusBar)
		width       int
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "filter pills render when they fit",
			setup:       func(s *StatusBar) { s.SetFilters([]string{"host=core1", "/error"}) },
			width:       80,
			wantContain: []string{"host=core1", "/error"},
		},
		{
			name:        "pill colliding with help segment is dropped",
			setup:       func(s *StatusBar) { s.SetFilters([]string{"host=core1", "/error"}) },
			width:       55,
			wantContain: []string{"host=core1"},
			wantAbsent:  []string{"/error"},
		},
		{
			name:        "long pill is truncated to maxPillWidth",
			setup:       func(s *StatusBar) { s.SetFilters([]string{"/" + strings.Repeat("a", 30)}) },
			width:       120,
			wantContain: []string{"/" + strings.Repeat("a", 20) + "..."},
			wantAbsent:  []string{strings.Repeat("a", 30)},
		},
		{
			name:        "parse error counter visible when non-zero",
			setup:       func(s *StatusBar) { s.SetParseErrors(3) },
			width:       80,
			wantContain: []string{"⚠ 3 parse errors"},
		},
		{
			name:       "no parse error counter at zero",
			setup:      func(s *StatusBar) {},
			width:      80,
			wantAbsent: []string{"parse errors"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStatusBar()
			tt.setup(&s)
			got := s.View(tt.width)
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("View(%d) missing %q\nrendered: %q", tt.width, want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("View(%d) unexpectedly contains %q\nrendered: %q", tt.width, absent, got)
				}
			}
		})
	}
}
