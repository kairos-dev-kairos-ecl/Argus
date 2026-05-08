package components

import (
	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Spinner wraps bubbles/spinner with Argus theme styling.
type Spinner struct {
	inner spinner.Model
}

// NewSpinner creates a themed spinner.
func NewSpinner() Spinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.Subtle
	return Spinner{inner: s}
}

// Tick returns the tea.Cmd that drives the spinner animation.
func (s Spinner) Tick() tea.Cmd {
	return s.inner.Tick
}

// Update advances the spinner animation state.
func (s Spinner) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	inner, cmd := s.inner.Update(msg)
	s.inner = inner
	return s, cmd
}

// View renders the current spinner frame.
func (s Spinner) View() string {
	return s.inner.View()
}
