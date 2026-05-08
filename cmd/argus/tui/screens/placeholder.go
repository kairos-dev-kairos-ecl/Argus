// Package screens provides Bubbletea screen models for the Argus TUI.
package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/argusxdr/argus/cmd/argus/tui/theme"
)

// PlaceholderModel is a stub screen rendered for screens not yet implemented
// in Phase 2. Each placeholder shows "{Name} — coming in Phase 3".
type PlaceholderModel struct {
	Name string
}

// NewPlaceholder creates a placeholder screen with the given name.
func NewPlaceholder(name string) *PlaceholderModel {
	return &PlaceholderModel{Name: name}
}

// Init implements tea.Model. No I/O needed.
func (m PlaceholderModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Placeholders do not handle any messages.
func (m PlaceholderModel) Update(_ tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

// View renders a centered bordered panel with the placeholder message.
func (m PlaceholderModel) View() string {
	msg := m.Name + " — coming in Phase 3"
	return lipgloss.NewStyle().
		Inherit(theme.Panel).
		Align(lipgloss.Center, lipgloss.Center).
		Render(msg)
}

// Title returns the screen name for use in the status bar.
func (m PlaceholderModel) Title() string {
	return m.Name
}

// KeyHelp returns an empty slice since placeholder screens have no local bindings.
func (m PlaceholderModel) KeyHelp() []key.Binding {
	return []key.Binding{}
}
