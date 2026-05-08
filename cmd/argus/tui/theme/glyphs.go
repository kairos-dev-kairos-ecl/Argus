package theme

// Glyphs holds the Unicode or ASCII symbols used throughout the TUI.
type Glyphs struct {
	Check        string
	Cross        string
	Block        string
	ChevronRight string
	ChevronDown  string
	Bullet       string
	Ellipsis     string
}

// Unicode returns the default Unicode glyph set.
func Unicode() Glyphs {
	return Glyphs{
		Check:        "✓",
		Cross:        "✗",
		Block:        "█",
		ChevronRight: "▶",
		ChevronDown:  "▼",
		Bullet:       "•",
		Ellipsis:     "…",
	}
}

// ASCII returns the ASCII-only fallback glyph set for terminals that cannot
// render Unicode correctly.
func ASCII() Glyphs {
	return Glyphs{
		Check:        "+",
		Cross:        "x",
		Block:        "#",
		ChevronRight: ">",
		ChevronDown:  "v",
		Bullet:       "*",
		Ellipsis:     "...",
	}
}

// current is the active glyph set. Defaults to Unicode.
var current = Unicode()

// SetASCII switches the active glyph set.
// Pass true to use ASCII-only glyphs; false to restore Unicode.
func SetASCII(ascii bool) {
	if ascii {
		current = ASCII()
	} else {
		current = Unicode()
	}
}

// GetGlyphs returns the currently active glyph set.
func GetGlyphs() Glyphs {
	return current
}
