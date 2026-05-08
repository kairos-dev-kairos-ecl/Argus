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

// renderHelpContent builds the key bindings list for the help overlay.
func (m *AppModel) renderHelpContent() string {
	var sb strings.Builder

	// Global bindings.
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

	// Screen-local bindings.
	var localBindings []string
	switch m.current {
	case ScreenSignals:
		for _, b := range m.signalsScreen.KeyHelp() {
			keys := strings.Join(b.Keys(), ", ")
			localBindings = append(localBindings, fmt.Sprintf("%-16s %s", keys, b.Help().Desc))
		}
	case ScreenTrace:
		for _, b := range m.traceScreen.KeyHelp() {
			keys := strings.Join(b.Keys(), ", ")
			localBindings = append(localBindings, fmt.Sprintf("%-16s %s", keys, b.Help().Desc))
		}
	case ScreenAlerts:
		for _, b := range m.alertsScreen.KeyHelp() {
			keys := strings.Join(b.Keys(), ", ")
			localBindings = append(localBindings, fmt.Sprintf("%-16s %s", keys, b.Help().Desc))
		}
	case ScreenRules:
		for _, b := range m.rulesScreen.KeyHelp() {
			keys := strings.Join(b.Keys(), ", ")
			localBindings = append(localBindings, fmt.Sprintf("%-16s %s", keys, b.Help().Desc))
		}
	case ScreenUsers:
		for _, b := range m.usersScreen.KeyHelp() {
			keys := strings.Join(b.Keys(), ", ")
			localBindings = append(localBindings, fmt.Sprintf("%-16s %s", keys, b.Help().Desc))
		}
	case ScreenAudit:
		for _, b := range m.auditScreen.KeyHelp() {
			keys := strings.Join(b.Keys(), ", ")
			localBindings = append(localBindings, fmt.Sprintf("%-16s %s", keys, b.Help().Desc))
		}
	}

	if len(localBindings) > 0 {
		sb.WriteString("\n")
		sb.WriteString(theme.Emphasis.Render("Screen bindings:"))
		sb.WriteString("\n")
		for _, line := range localBindings {
			sb.WriteString(theme.Subtle.Render(line))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
