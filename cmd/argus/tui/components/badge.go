package components

import "github.com/charmbracelet/lipgloss"

// Badge renders a short text label with a Lipgloss style applied.
// Used for [L7] layer badges and [CRIT] severity badges.
type Badge struct {
	style lipgloss.Style
}

// NewBadge creates a Badge with the given style.
func NewBadge(style lipgloss.Style) Badge {
	return Badge{style: style}
}

// Render returns the text wrapped in the badge's style.
func (b Badge) Render(text string) string {
	return b.style.Render("[" + text + "]")
}
