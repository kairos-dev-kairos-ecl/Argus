package main

import (
	"github.com/argusxdr/argus/cmd/argus/selector"
	"github.com/spf13/cobra"
)

var resetUICmd = &cobra.Command{
	Use:   "reset-ui",
	Short: "Clear saved UI preference (web/tui)",
	Long:  "Removes the saved interface preference so the next `argus` invocation re-shows the selector.",
	RunE:  runResetUI,
}

func runResetUI(cmd *cobra.Command, args []string) error {
	if err := selector.ClearUIPref(); err != nil {
		return err
	}
	// Success message must not echo the previous choice (defence in depth).
	cmd.OutOrStdout().Write([]byte("Cleared UI preference. Next `argus` invocation will show the selector.\n"))
	return nil
}

func init() {
	rootCmd.AddCommand(resetUICmd)
}
