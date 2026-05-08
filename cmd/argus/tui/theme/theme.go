// Package theme provides Lipgloss style constants for the Argus TUI.
// All colors are sourced from the CLAUDE.md design system (brutalist dark theme).
//
// # Phase 3 $EDITOR Security Constraints
//
// When Phase 3 implements detection-rule editing via $EDITOR, the following
// constraints MUST be followed (locked in security_constraints from Phase 2 plan):
//
//  1. Temporary file directory: os.MkdirTemp(homeDir, "argus-rule-*") where
//     homeDir is determined via os.UserHomeDir(). NEVER use os.TempDir() or
//     /tmp/ directly — these locations are world-readable on multi-user systems.
//
//  2. Directory permissions: 0700 (owner-only rwx).
//
//  3. File permissions: 0600 (owner-only rw). Use os.WriteFile(path, data, 0600).
//
//  4. Editor invocation: exec.Command(editorBin, tmpFile) — fork the editor
//     directly, NEVER via sh -c "editorBin tmpFile" to avoid shell injection.
//     Resolve editorBin from $EDITOR env var; fall back to "vi" if unset.
//
//  5. Cleanup: always defer os.RemoveAll(tmpDir) regardless of editor exit code.
//
//  6. Never log or expose the tmpFile path in user-visible messages beyond a
//     generic "editing rules" message.
package theme

import "github.com/charmbracelet/lipgloss"

// Color constants — design tokens from CLAUDE.md.
const (
	ColorBg        = "#0A0A0B"
	ColorBorder    = "#2A2A2F"
	ColorText      = "#FFFFFF"
	ColorSecondary = "#A0A0A0"
	ColorAccent    = "#00C896"
	ColorSuccess   = "#22C55E"
	ColorWarning   = "#EAB308"
	ColorError     = "#EF4444"
	ColorInfo      = "#3B82F6"
)

// Layer colors L1–L10 ramp from CLAUDE.md design system (red → rose).
// L1=#EF4444 (red), L2=#F97316 (orange), L3=#EAB308 (yellow), L4=#22C55E (green),
// L5=#A855F7 (purple), L6=#3B82F6 (blue), L7=#06B6D4 (cyan), L8=#14B8A6 (teal),
// L9=#EC4899 (pink), L10=#F43F5E (rose).
var layerColors = [11]string{
	"",         // index 0 unused
	"#EF4444",  // L1
	"#F97316",  // L2
	"#EAB308",  // L3
	"#22C55E",  // L4
	"#A855F7",  // L5
	"#3B82F6",  // L6
	"#06B6D4",  // L7
	"#14B8A6",  // L8
	"#EC4899",  // L9
	"#F43F5E",  // L10
}

// Exported Lipgloss style values — initialized once in init().
var (
	Background   lipgloss.Style
	Border       lipgloss.Style
	Title        lipgloss.Style
	Subtitle     lipgloss.Style
	Muted        lipgloss.Style
	Code         lipgloss.Style
	Header       lipgloss.Style
	StatusBar    lipgloss.Style
	Panel        lipgloss.Style
	PanelFocused lipgloss.Style
	PanelError   lipgloss.Style
	Subtle       lipgloss.Style
	Emphasis     lipgloss.Style
	ErrorText    lipgloss.Style
)

func init() {
	Background = lipgloss.NewStyle().
		Background(lipgloss.Color(ColorBg)).
		Foreground(lipgloss.Color(ColorText))

	Border = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorBorder))

	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorAccent))

	Subtitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSecondary))

	Muted = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSecondary))

	Code = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorAccent)).
		Background(lipgloss.Color(ColorBorder))

	Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorText)).
		Background(lipgloss.Color(ColorBorder)).
		Padding(0, 1)

	StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorText)).
		Background(lipgloss.Color(ColorBorder))

	Panel = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Padding(0, 1)

	PanelFocused = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(0, 1)

	PanelError = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorError)).
		Padding(0, 1)

	Subtle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSecondary))

	Emphasis = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorAccent))

	ErrorText = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorError))
}

// LayerBadge returns a Lipgloss style for the given layer number (1–10).
// Out-of-range values fall back to the Subtle style.
func LayerBadge(layer int) lipgloss.Style {
	if layer < 1 || layer > 10 {
		return Subtle
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(layerColors[layer]))
}

// SeverityBadge returns a Lipgloss style for the given severity level.
// Recognized values: "CRIT", "HI", "MD", "LO", "INFO" (case-sensitive).
// Unknown severities fall back to the Subtle style.
func SeverityBadge(sev string) lipgloss.Style {
	switch sev {
	case "CRIT":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorError))
	case "HI":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorWarning))
	case "MD":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorInfo))
	case "LO":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorSuccess))
	case "INFO":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorSecondary))
	default:
		return Subtle
	}
}
