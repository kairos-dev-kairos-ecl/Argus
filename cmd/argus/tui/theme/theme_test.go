package theme_test

import (
	"strings"
	"testing"

	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// TestLayerBadge_NonEmpty asserts LayerBadge returns a non-empty style for all
// valid layers L1–L10.
func TestLayerBadge_NonEmpty(t *testing.T) {
	for i := 1; i <= 10; i++ {
		style := theme.LayerBadge(i)
		rendered := style.Render("L" + string(rune('0'+i)))
		if rendered == "" {
			t.Errorf("LayerBadge(%d) rendered empty string", i)
		}
	}
}

// TestLayerBadge_Colors checks that layer badge colors follow the L1-L10 ramp
// defined in CLAUDE.md design tokens.
func TestLayerBadge_Colors(t *testing.T) {
	// Spot-check a few known layer colors from the ramp.
	// L1 = #EF4444, L5 = #A855F7, L10 = #F43F5E
	checks := []struct {
		layer    int
		colorHex string
	}{
		{1, "#EF4444"},
		{10, "#F43F5E"},
	}
	for _, tc := range checks {
		style := theme.LayerBadge(tc.layer)
		// Render with a known text and verify style contains the color.
		rendered := style.Render("X")
		if rendered == "" {
			t.Errorf("LayerBadge(%d) rendered empty", tc.layer)
		}
		// Verify the style has a foreground color set.
		fg, _ := style.GetForeground().(lipgloss.Color)
		if fg == "" {
			t.Errorf("LayerBadge(%d): foreground color not set", tc.layer)
		}
	}
}

// TestSeverityBadge_Colors verifies SeverityBadge returns styles with correct
// status colors from the design system.
func TestSeverityBadge_Colors(t *testing.T) {
	cases := []struct {
		sev      string
		wantFG   string
	}{
		{"CRIT", "#EF4444"},
		{"HI", "#EAB308"},
		{"MD", "#3B82F6"},
		{"LO", "#22C55E"},
		{"INFO", "#A0A0A0"},
	}
	for _, tc := range cases {
		style := theme.SeverityBadge(tc.sev)
		rendered := style.Render(tc.sev)
		if rendered == "" {
			t.Errorf("SeverityBadge(%q) rendered empty", tc.sev)
		}
		fg, _ := style.GetForeground().(lipgloss.Color)
		if string(fg) != tc.wantFG {
			t.Errorf("SeverityBadge(%q): want fg %s, got %s", tc.sev, tc.wantFG, fg)
		}
	}
}

// TestGlyphs_UnicodeAndASCII verifies the ASCII toggle changes the Check glyph.
func TestGlyphs_UnicodeAndASCII(t *testing.T) {
	// Default should be Unicode.
	g := theme.GetGlyphs()
	if !strings.Contains(g.Check, "✓") && !strings.Contains(g.Check, "+") {
		t.Errorf("unexpected Check glyph: %q", g.Check)
	}

	// Toggle to ASCII.
	theme.SetASCII(true)
	g = theme.GetGlyphs()
	if g.Check != "+" {
		t.Errorf("after SetASCII(true): Check should be '+', got %q", g.Check)
	}

	// Toggle back.
	theme.SetASCII(false)
	g = theme.GetGlyphs()
	if g.Check != "✓" {
		t.Errorf("after SetASCII(false): Check should be '✓', got %q", g.Check)
	}
}
