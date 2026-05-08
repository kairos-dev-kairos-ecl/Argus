// Package tui provides the `argus tui` Cobra subcommand.
//
// Phase 1 stub: prints an informational message and exits cleanly.
// The real Bubbletea application is delivered in Phase 2.
package tui

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Cmd is the `argus tui` subcommand.
var Cmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the Argus terminal UI (Phase 2)",
	Long:  "Launches the Argus terminal UI. Phase 2 stub — the full TUI is coming soon.",
	RunE:  runTUI,
}

func runTUI(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "Terminal UI launching in Phase 2 — use 'argus web' for now")
	return nil
}
