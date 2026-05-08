// Package tui provides the `argus tui` Cobra subcommand.
//
// Phase 2: Real Bubbletea application with login screen, auth flow,
// API/WS client, and six placeholder operator screens.
// Phase 3 will replace the placeholder screens with live operator views.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Cmd is the `argus tui` subcommand.
// The variable address is kept stable from Phase 1 so that main.go's init()
// continues to reference the same cobra.Command pointer.
var Cmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the Argus terminal UI",
	Long:  "Launches the Argus terminal UI with keyboard-driven operator screens.",
	RunE:  runTUI,
}

func runTUI(_ *cobra.Command, _ []string) error {
	baseURL := viper.GetString("api.url")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	app := New(Config{BaseURL: baseURL})
	_, err := tea.NewProgram(app, tea.WithAltScreen()).Run()
	return err
}
