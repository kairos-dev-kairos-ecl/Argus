package behaviour

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	flagAppID string
	flagURL   string
	flagToken string
)

var Cmd = &cobra.Command{
	Use:   "behaviour",
	Short: "Open the behavioural traceability TUI",
	Long:  "Browse recent LLM runs for an app_id, inspect span trees with deviation scores, and compare runs side-by-side.",
	RunE: func(c *cobra.Command, _ []string) error {
		if flagAppID == "" {
			return fmt.Errorf("--app-id required")
		}
		if flagURL == "" {
			flagURL = "http://localhost:8080"
		}
		if flagToken == "" {
			return fmt.Errorf("--token required (JWT bearer)")
		}
		m := New(flagURL, flagToken, flagAppID)
		_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
		return err
	},
}

func init() {
	Cmd.Flags().StringVar(&flagAppID, "app-id", "", "Application ID (required)")
	Cmd.Flags().StringVar(&flagURL, "url", "http://localhost:8080", "Argus API base URL")
	Cmd.Flags().StringVar(&flagToken, "token", "", "JWT bearer token (required)")
}
