package components

import (
	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// Modal renders a centered bordered dialog overlaid on a background string.
type Modal struct {
	width  int
	height int
	title  string
	body   string
}

// NewModal creates a modal with the given dimensions and content.
func NewModal(width, height int, title, body string) Modal {
	return Modal{
		width:  width,
		height: height,
		title:  title,
		body:   body,
	}
}

// Render overlays the modal on the given background string.
// The background is dimmed and the modal is placed in the center.
func (m Modal) Render(background string) string {
	// Build modal content.
	titleLine := theme.Title.Render(m.title)
	content := titleLine + "\n\n" + m.body

	box := theme.PanelFocused.
		Width(m.width).
		Height(m.height).
		Render(content)

	// Place the modal centered on the background.
	// Use WithWhitespaceForeground to dim the surrounding area.
	return lipgloss.Place(
		lipgloss.Width(background),
		lipgloss.Height(background),
		lipgloss.Center,
		lipgloss.Center,
		box,
		lipgloss.WithWhitespaceForeground(lipgloss.Color(theme.ColorSecondary)),
	)
}
