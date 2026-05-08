package components

import (
	"fmt"

	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Viewport wraps bubbles/viewport with Argus theme styling.
type Viewport struct {
	inner  viewport.Model
	width  int
	height int
}

// NewViewport creates a themed viewport at the given dimensions.
func NewViewport(width, height int) Viewport {
	vp := viewport.New(width, height)
	return Viewport{inner: vp, width: width, height: height}
}

// SetContent sets the text content to display.
func (v *Viewport) SetContent(content string) {
	v.inner.SetContent(content)
}

// Update handles key and mouse events.
func (v Viewport) Update(msg tea.Msg) (Viewport, tea.Cmd) {
	inner, cmd := v.inner.Update(msg)
	v.inner = inner
	return v, cmd
}

// View renders the viewport with a scroll percentage indicator.
func (v Viewport) View() string {
	scrollPct := fmt.Sprintf("  %3.f%%", v.inner.ScrollPercent()*100)
	pctStyle := theme.Muted.
		Width(v.width).
		Align(lipgloss.Right)

	return theme.Panel.Render(v.inner.View()) + "\n" + pctStyle.Render(scrollPct)
}
