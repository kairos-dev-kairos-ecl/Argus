package tui

import (
	"fmt"
	"strings"

	"github.com/argusxdr/argus/cmd/argus/tui/components"
	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/bubbles/key"
)

// renderHelpOverlay builds and renders the full help modal overlaid on background.
// Layout: Global section (all 13 global bindings) + one section for the current screen.
// Reads global bindings from m.keys.Sections() and screen-local bindings from
// activeScreenKeyHelp() — single source of truth, zero duplication.
func renderHelpOverlay(m *AppModel, background string) string {
	body := buildHelpBody(m)

	// Modal sizing: width = min(80, m.width-4), height computed from line count up to m.height-4.
	width := 80
	if m.width > 0 && m.width-4 < width {
		width = m.width - 4
	}
	if width < 20 {
		width = 20
	}

	lineCount := strings.Count(body, "\n") + 4 // +4 for title + padding
	height := lineCount
	maxHeight := m.height - 4
	if maxHeight < 6 {
		maxHeight = 6
	}
	if height > maxHeight {
		height = maxHeight
	}

	modal := components.NewModal(width, height, "Keyboard Shortcuts", body)
	return modal.Render(background)
}

// buildHelpBody builds the text body of the help overlay (without modal wrapper).
// Used directly by test helpers and by renderHelpOverlay.
func buildHelpBody(m *AppModel) string {
	var sb strings.Builder

	// Render Global section.
	sections := m.keys.Sections()
	if len(sections) > 0 {
		global := sections[0]
		sb.WriteString(theme.Emphasis.Render(global.Name))
		sb.WriteString("\n")
		for _, b := range global.Bindings {
			if !b.Enabled() {
				continue
			}
			sb.WriteString(formatBinding(b))
			sb.WriteString("\n")
		}
	}

	// Render current screen section.
	sectionName, localBindings := activeScreenKeyHelp(m)
	if len(localBindings) > 0 {
		sb.WriteString("\n")
		sb.WriteString(theme.Emphasis.Render(sectionName))
		sb.WriteString("\n")
		for _, b := range localBindings {
			if !b.Enabled() {
				continue
			}
			sb.WriteString(formatBinding(b))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// formatBinding formats a single key binding as a fixed-width "keys       desc" row.
func formatBinding(b key.Binding) string {
	keyStr := strings.Join(b.Keys(), ", ")
	desc := b.Help().Desc
	line := fmt.Sprintf("%-16s %s", keyStr, desc)
	return theme.Subtle.Render(line)
}

// activeScreenKeyHelp returns the section name and local bindings for the current screen.
// This is the single source of truth for per-screen bindings in the help overlay.
func activeScreenKeyHelp(m *AppModel) (string, []key.Binding) {
	switch m.current {
	case ScreenSignals:
		return "Signals", m.signalsScreen.KeyHelp()
	case ScreenTrace:
		return "Trace", m.traceScreen.KeyHelp()
	case ScreenAlerts:
		return "Alerts", m.alertsScreen.KeyHelp()
	case ScreenRules:
		return "Rules", m.rulesScreen.KeyHelp()
	case ScreenUsers:
		return "Users", m.usersScreen.KeyHelp()
	case ScreenAudit:
		return "Audit", m.auditScreen.KeyHelp()
	}
	return "", nil
}

// --- Test helpers (only used from _test.go files in the tui package) ---

// RenderHelpOverlayForTest is an exported test helper that renders the help overlay body.
// It is exported so that tui_test package can call it without internal access.
func RenderHelpOverlayForTest(m *AppModel) string {
	return buildHelpBody(m)
}

// SetScreenForTest is an exported test helper that sets the active screen for testing.
func SetScreenForTest(m *AppModel, s Screen) {
	m.current = s
}
