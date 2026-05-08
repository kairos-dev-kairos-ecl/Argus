// Package web provides the `argus web` Cobra subcommand.
//
// RunE is intentionally set to nil at declaration time. It is wired in
// cmd/argus/main.go after apiCmd is available:
//
//	web.Cmd.RunE = apiCmd.RunE
//
// This pattern avoids cross-package symbol exposure while keeping logic in a
// single place (api.go's runAPI function).
package web

import "github.com/spf13/cobra"

// Cmd is the `argus web` subcommand. Its RunE is wired by main.go init().
var Cmd = &cobra.Command{
	Use:   "web",
	Short: "Launch the Argus web dashboard server",
	Long:  "Starts the HTTP API + web dashboard server. Equivalent to `argus api` (thin wrapper).",
	// RunE: wired to apiCmd.RunE in cmd/argus/main.go init()
}
