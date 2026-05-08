package selector

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Design tokens — brutalist dark theme from CLAUDE.md + task spec.
var (
	colorBg        = lipgloss.Color("#0A0A0B")
	colorText      = lipgloss.Color("#FFFFFF")
	colorAccent    = lipgloss.Color("#00C896")
	colorBorder    = lipgloss.Color("#2A2A2F")
	colorSecondary = lipgloss.Color("#A0A0A0")
	colorSuccess   = lipgloss.Color("#22C55E")
	colorError     = lipgloss.Color("#EF4444")

	baseStyle = lipgloss.NewStyle().
			Background(colorBg).
			Foreground(colorText)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	headingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText)

	proStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	conStyle = lipgloss.NewStyle().
			Foreground(colorError)

	hintStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	footerStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)
)

// webPros and webCons define the web dashboard's advantages and disadvantages.
var webPros = []string{
	"Visual dashboards",
	"Sankey flow charts",
	"MITRE ATT&CK matrix",
	"Drag & drop rules",
	"Rich incident view",
}

var webCons = []string{
	"Requires browser",
	"No SSH access",
	"API binding overhead",
}

// tuiPros and tuiCons define the terminal UI's advantages and disadvantages.
var tuiPros = []string{
	"Keyboard-driven",
	"Works over SSH",
	"No browser needed",
	"Lower latency",
	"Scriptable/pipeable",
}

var tuiCons = []string{
	"No Sankey diagrams",
	"No MITRE grid view",
	"No drag & drop",
}

// buildPanel renders a single option panel with the given heading, pros, and
// cons, constrained to the specified width.
func buildPanel(heading string, pros, cons []string, width int) string {
	var sb strings.Builder

	sb.WriteString(headingStyle.Render(heading))
	sb.WriteString("\n\n")

	for _, p := range pros {
		sb.WriteString(proStyle.Render("✓ " + p))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	for _, c := range cons {
		sb.WriteString(conStyle.Render("✗ " + c))
		sb.WriteString("\n")
	}

	return panelStyle.Width(width).Render(sb.String())
}

// View implements tea.Model.
func (m Model) View() string {
	title := titleStyle.Render("ARGUS XDR — Choose your interface")

	const minSideBySideWidth = 80

	var panels string

	if m.width >= minSideBySideWidth {
		// Side-by-side layout. Each panel gets half the available space minus
		// the gap and border overhead, capped at 40 columns.
		panelWidth := (m.width - 6) / 2
		if panelWidth > 40 {
			panelWidth = 40
		}
		if panelWidth < 20 {
			panelWidth = 20
		}

		webPanel := buildPanel("[G] WEB DASHBOARD", webPros, webCons, panelWidth)
		tuiPanel := buildPanel("[T] TERMINAL (TUI)", tuiPros, tuiCons, panelWidth)
		panels = lipgloss.JoinHorizontal(lipgloss.Top, webPanel, "  ", tuiPanel)
	} else {
		// Narrow terminal: stack panels vertically.
		availWidth := m.width - 4
		if availWidth < 20 {
			availWidth = 20
		}
		if availWidth > 40 {
			availWidth = 40
		}

		webPanel := buildPanel("[G] WEB DASHBOARD", webPros, webCons, availWidth)
		tuiPanel := buildPanel("[T] TERMINAL (TUI)", tuiPros, tuiCons, availWidth)
		panels = lipgloss.JoinVertical(lipgloss.Left, webPanel, tuiPanel)
	}

	hint := hintStyle.Render("[G] Launch Web Dashboard    [T] Launch Terminal UI")
	footer := footerStyle.Render("Your choice will be remembered. Run 'argus reset-ui' to change it.")

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		panels,
		"",
		hint,
		footer,
	)

	return baseStyle.Render(body)
}
