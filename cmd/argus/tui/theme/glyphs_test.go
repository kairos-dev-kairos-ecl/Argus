package theme_test

import (
	"testing"
	"unicode/utf8"

	"github.com/argusxdr/argus/cmd/argus/tui/theme"
)

// TestGlyphs_ASCII_HasNoUnicode verifies every field in ASCII() is pure ASCII (rune < 128).
func TestGlyphs_ASCII_HasNoUnicode(t *testing.T) {
	g := theme.ASCII()
	fields := []struct {
		name  string
		value string
	}{
		{"Check", g.Check},
		{"Cross", g.Cross},
		{"Block", g.Block},
		{"ChevronRight", g.ChevronRight},
		{"ChevronDown", g.ChevronDown},
		{"Bullet", g.Bullet},
		{"Ellipsis", g.Ellipsis},
		{"BoxTopLeft", g.BoxTopLeft},
		{"BoxTopRight", g.BoxTopRight},
		{"BoxBottomLeft", g.BoxBottomLeft},
		{"BoxBottomRight", g.BoxBottomRight},
		{"BoxHorizontal", g.BoxHorizontal},
		{"BoxVertical", g.BoxVertical},
		{"BoxCross", g.BoxCross},
		{"LayerBracketL", g.LayerBracketL},
		{"LayerBracketR", g.LayerBracketR},
		{"SeverityBlock", g.SeverityBlock},
	}

	for _, f := range fields {
		for i, r := range f.value {
			if r >= utf8.RuneSelf {
				t.Errorf("ASCII().%s contains non-ASCII rune %q at position %d", f.name, r, i)
			}
		}
	}
}

// TestGlyphs_LayerBracket_ASCII_Empty verifies that ASCII layer bracket glyphs are empty strings.
func TestGlyphs_LayerBracket_ASCII_Empty(t *testing.T) {
	g := theme.ASCII()
	if g.LayerBracketL != "" {
		t.Errorf("ASCII().LayerBracketL = %q, want %q", g.LayerBracketL, "")
	}
	if g.LayerBracketR != "" {
		t.Errorf("ASCII().LayerBracketR = %q, want %q", g.LayerBracketR, "")
	}
}

// TestGlyphs_GlyphMode verifies SetMode toggles the active glyph set correctly.
func TestGlyphs_GlyphMode(t *testing.T) {
	// Start in Unicode mode.
	theme.SetMode(theme.GlyphModeUnicode)
	u := theme.GetGlyphs()
	if u.Check != "✓" {
		t.Errorf("Unicode mode Check = %q, want ✓", u.Check)
	}

	// Switch to ASCII mode.
	theme.SetMode(theme.GlyphModeASCII)
	a := theme.GetGlyphs()
	if a.Check != "+" {
		t.Errorf("ASCII mode Check = %q, want +", a.Check)
	}

	// Restore Unicode mode for other tests.
	theme.SetMode(theme.GlyphModeUnicode)
}
