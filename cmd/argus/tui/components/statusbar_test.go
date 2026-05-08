package components_test

import (
	"testing"

	"github.com/argusxdr/argus/cmd/argus/tui/components"
	"github.com/charmbracelet/lipgloss"
)

// TestStatusBar_ExactWidth verifies that StatusBar.Render always produces output
// whose visible width equals the requested width.
func TestStatusBar_ExactWidth(t *testing.T) {
	cases := []struct {
		width  int
		left   string
		center string
		right  string
	}{
		{80, "argus@localhost", "SIGNALS", "admin@admin"},
		{40, "left", "mid", "right"},
		{120, "very long left side text here", "CENTER", "short"},
		{20, "", "", ""},
	}
	for _, tc := range cases {
		sb := components.NewStatusBar(tc.width)
		out := sb.Render(tc.left, tc.center, tc.right)
		got := lipgloss.Width(out)
		if got != tc.width {
			t.Errorf("StatusBar(%d).Render(%q, %q, %q): want width %d, got %d",
				tc.width, tc.left, tc.center, tc.right, tc.width, got)
		}
	}
}
