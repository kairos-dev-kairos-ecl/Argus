// Package tui provides the `argus tui` Cobra subcommand.
//
// Phase 2: Real Bubbletea application with login screen, auth flow,
// API/WS client, and six placeholder operator screens.
// Phase 3 will replace the placeholder screens with live operator views.
// Phase 4: --no-unicode flag, --version flag, auto-detection of dumb terminal.
package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// flagNoUnicode forces ASCII-only rendering when set.
var flagNoUnicode bool

// flagVersion prints the TUI version and exits when set.
var flagVersion bool

// Cmd is the `argus tui` subcommand.
// The variable address is kept stable from Phase 1 so that main.go's init()
// continues to reference the same cobra.Command pointer.
var Cmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the Argus terminal UI",
	Long:  "Launches the Argus terminal UI with keyboard-driven operator screens.",
	RunE:  runTUI,
}

func init() {
	Cmd.Flags().BoolVar(&flagNoUnicode, "no-unicode", false,
		"force ASCII-only rendering for SSH or dumb terminals")
	Cmd.Flags().BoolVar(&flagVersion, "version", false,
		"print version and exit")
}

// autoDetectNoUnicode returns true when the terminal environment suggests that
// Unicode rendering will fail:
//   - TERM=dumb (explicit dumb terminal declaration), OR
//   - SSH_TTY is set AND LANG+LC_ALL contain no UTF-8 indicator
func autoDetectNoUnicode() bool {
	if os.Getenv("TERM") == "dumb" {
		return true
	}
	if os.Getenv("SSH_TTY") != "" {
		langVars := strings.ToUpper(os.Getenv("LANG") + os.Getenv("LC_ALL"))
		if !strings.Contains(langVars, "UTF-8") && !strings.Contains(langVars, "UTF8") {
			return true
		}
	}
	return false
}

func runTUI(cmd *cobra.Command, _ []string) error {
	// --version: print version string and exit immediately.
	if flagVersion {
		fmt.Fprintln(cmd.OutOrStdout(), VersionString())
		return nil
	}

	// --no-unicode or auto-detection: switch to ASCII glyph set before rendering.
	if flagNoUnicode || autoDetectNoUnicode() {
		theme.SetASCII(true)
	}

	baseURL := viper.GetString("api.url")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	app := New(Config{BaseURL: baseURL})
	_, err := tea.NewProgram(app, tea.WithAltScreen()).Run()
	return err
}
