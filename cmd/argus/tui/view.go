package tui

import (
	"fmt"
	"strings"

	"github.com/argusxdr/argus/cmd/argus/tui/components"
	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model. Renders the full TUI screen.
func (m *AppModel) View() string {
	// Render the active screen content.
	activeView := m.renderActiveScreen()

	// Build header.
	header := m.renderHeader()

	// Build status bar.
	host := "localhost"
	left := fmt.Sprintf("argus@%s", host)
	center := m.screenTitle()
	right := m.authStatus()
	statusBar := m.statusBar.Render(left, center, right)

	// Compose: header + screen + status bar.
	full := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		activeView,
		statusBar,
	)

	// Help overlay.
	if m.helpVisible {
		helpContent := m.renderHelpContent()
		modal := components.NewModal(60, 20, "Keyboard Shortcuts", helpContent)
		return modal.Render(full)
	}

	// Quit confirmation overlay.
	if m.quitConfirm {
		quitContent := "Press [y] to confirm, any other key to cancel"
		modal := components.NewModal(50, 6, "Quit Argus TUI?", quitContent)
		return modal.Render(full)
	}

	return full
}

// renderHeader renders the one-line title header.
func (m *AppModel) renderHeader() string {
	title := theme.Title.Render("ARGUS XDR")
	version := theme.Muted.Render("TUI")
	return theme.Header.Width(m.width).Render(title + "  " + version)
}

// renderActiveScreen delegates to the currently active screen's View().
func (m *AppModel) renderActiveScreen() string {
	if m.current == ScreenLogin {
		return m.loginScreen.View()
	}

	if p, ok := m.placeholders[m.current]; ok {
		return p.View()
	}

	return theme.ErrorText.Render("unknown screen")
}

// renderHelpContent builds the key bindings list for the help overlay.
func (m *AppModel) renderHelpContent() string {
	var sb strings.Builder
	bindings := m.keys.Help()
	for _, b := range bindings {
		if !b.Enabled() {
			continue
		}
		keys := strings.Join(b.Keys(), ", ")
		help := b.Help().Desc
		line := fmt.Sprintf("%-16s %s", keys, help)
		sb.WriteString(theme.Subtle.Render(line))
		sb.WriteString("\n")
	}
	return sb.String()
}

