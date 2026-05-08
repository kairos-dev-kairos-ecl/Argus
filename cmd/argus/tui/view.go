package tui

import (
	"fmt"

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

	// Hint line — shown on all operator screens (not login).
	// Provides a consistent reminder of the most common global keys.
	var hintLine string
	if m.current != ScreenLogin {
		hintLine = theme.Subtle.Render("[?] help  [q] quit  [1-6] screens")
	}

	// Compose: header + screen + hint line (operator only) + status bar.
	parts := []string{header, activeView}
	if hintLine != "" {
		parts = append(parts, hintLine)
	}
	parts = append(parts, statusBar)

	full := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Help overlay — rendered by help.go using sectioned bindings from keys/bindings.go.
	// Phase 4: replaces the flat renderHelpContent from Phase 2/3.
	if m.helpVisible {
		return renderHelpOverlay(m, full)
	}

	// Quit confirmation overlay.
	if m.quitConfirm {
		quitContent := "Quit Argus TUI? [y/N]"
		modal := components.NewModal(50, 6, "Confirm Quit", quitContent)
		return modal.Render(full)
	}

	return full
}

// renderHeader renders the one-line title header.
func (m *AppModel) renderHeader() string {
	title := theme.Title.Render("ARGUS XDR")
	ver := theme.Muted.Render("TUI v" + Version)
	return theme.Header.Width(m.width).Render(title + "  " + ver)
}

// renderActiveScreen delegates to the currently active screen's View().
func (m *AppModel) renderActiveScreen() string {
	switch m.current {
	case ScreenLogin:
		return m.loginScreen.View()
	case ScreenSignals:
		return m.signalsScreen.View()
	case ScreenTrace:
		return m.traceScreen.View()
	case ScreenAlerts:
		return m.alertsScreen.View()
	case ScreenRules:
		return m.rulesScreen.View()
	case ScreenUsers:
		return m.usersScreen.View()
	case ScreenAudit:
		return m.auditScreen.View()
	}
	return theme.ErrorText.Render("unknown screen")
}
