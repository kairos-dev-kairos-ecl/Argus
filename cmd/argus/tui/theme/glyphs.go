package theme

// GlyphMode selects between Unicode and ASCII-only rendering.
type GlyphMode int

const (
	// GlyphModeUnicode uses full Unicode symbols (default).
	GlyphModeUnicode GlyphMode = 0
	// GlyphModeASCII restricts all glyphs to printable ASCII (< 128).
	// Use when TERM=dumb or SSH_TTY is set without UTF-8 LANG.
	GlyphModeASCII GlyphMode = 1
)

// Glyphs holds the Unicode or ASCII symbols used throughout the TUI.
type Glyphs struct {
	// Basic symbols
	Check        string
	Cross        string
	Block        string
	ChevronRight string
	ChevronDown  string
	Bullet       string
	Ellipsis     string

	// Box-drawing characters (for panels and borders).
	// ASCII fallback: + for corners/cross, - for horizontal, | for vertical.
	BoxTopLeft     string
	BoxTopRight    string
	BoxBottomLeft  string
	BoxBottomRight string
	BoxHorizontal  string
	BoxVertical    string
	BoxCross       string

	// LayerBracketL / LayerBracketR wrap the layer number in [L1] notation.
	// In ASCII mode these are empty so label renders as just "L1" (no brackets).
	LayerBracketL string
	LayerBracketR string

	// SeverityBlock is the filled block used in severity indicators.
	// Unicode: █, ASCII: #
	SeverityBlock string
}

// Unicode returns the default Unicode glyph set.
func Unicode() Glyphs {
	return Glyphs{
		Check:          "✓",
		Cross:          "✗",
		Block:          "█",
		ChevronRight:   "▶",
		ChevronDown:    "▼",
		Bullet:         "•",
		Ellipsis:       "…",
		BoxTopLeft:     "┌",
		BoxTopRight:    "┐",
		BoxBottomLeft:  "└",
		BoxBottomRight: "┘",
		BoxHorizontal:  "─",
		BoxVertical:    "│",
		BoxCross:       "┼",
		LayerBracketL:  "[",
		LayerBracketR:  "]",
		SeverityBlock:  "█",
	}
}

// ASCII returns the ASCII-only fallback glyph set for terminals that cannot
// render Unicode correctly (TERM=dumb, SSH without UTF-8, etc.).
// Every rune in this set is guaranteed to be < 128.
func ASCII() Glyphs {
	return Glyphs{
		Check:          "+",
		Cross:          "x",
		Block:          "#",
		ChevronRight:   ">",
		ChevronDown:    "v",
		Bullet:         "*",
		Ellipsis:       "...",
		BoxTopLeft:     "+",
		BoxTopRight:    "+",
		BoxBottomLeft:  "+",
		BoxBottomRight: "+",
		BoxHorizontal:  "-",
		BoxVertical:    "|",
		BoxCross:       "+",
		LayerBracketL:  "",
		LayerBracketR:  "",
		SeverityBlock:  "#",
	}
}

// current is the active glyph set. Defaults to Unicode.
var current = Unicode()

// SetMode switches the active glyph set by GlyphMode constant.
func SetMode(m GlyphMode) {
	if m == GlyphModeASCII {
		current = ASCII()
	} else {
		current = Unicode()
	}
}

// SetASCII switches the active glyph set.
// Pass true to use ASCII-only glyphs; false to restore Unicode.
// Deprecated: prefer SetMode(GlyphModeASCII) / SetMode(GlyphModeUnicode).
// Retained for backward compatibility with Phase 2/3 callers.
func SetASCII(ascii bool) {
	if ascii {
		SetMode(GlyphModeASCII)
	} else {
		SetMode(GlyphModeUnicode)
	}
}

// GetGlyphs returns the currently active glyph set.
func GetGlyphs() Glyphs {
	return current
}
