package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/argusxdr/argus/cmd/argus/selector"
	"github.com/argusxdr/argus/cmd/argus/tui"
	"github.com/argusxdr/argus/cmd/argus/web"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Build-time injected variables
var (
	Version   = "dev"
	BuildDate = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "argus",
	Short: "Argus XDR platform CLI",
	Long:  `Argus is an open-source, production-grade Extended Detection & Response (XDR) platform for LLM-integrated systems.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return dispatchRoot(cmd, args)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Argus XDR %s (built %s)\n", Version, BuildDate)
		return nil
	},
}

// selectorRunner is a test seam so unit tests can stub the Bubbletea program
// without needing a real terminal.
var selectorRunner = runSelectorInteractive

func runSelectorInteractive() (selector.Choice, error) {
	p := tea.NewProgram(selector.New(), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return selector.ChoiceNone, err
	}
	sm, ok := m.(selector.Model)
	if !ok {
		return selector.ChoiceNone, fmt.Errorf("selector returned unexpected model type")
	}
	return sm.Choice(), nil
}

// dispatchRoot is the root command's RunE logic. It:
//  1. Reads the saved UI preference from ~/.argus/config.yaml.
//  2. If no valid pref, runs the interactive interface selector.
//  3. Saves the user's choice.
//  4. Dispatches to web.Cmd.RunE or tui.Cmd.RunE accordingly.
func dispatchRoot(cmd *cobra.Command, args []string) error {
	pref, err := selector.LoadUIPref()
	if err != nil {
		return fmt.Errorf("loading ui preference: %w", err)
	}

	// Treat invalid stored values (e.g. corruption) the same as "no pref".
	if pref != string(selector.ChoiceWeb) && pref != string(selector.ChoiceTUI) {
		pref = ""
	}

	if pref == "" {
		choice, err := selectorRunner()
		if err != nil {
			return err
		}
		if choice == selector.ChoiceNone {
			// User cancelled (q / esc / ctrl+c) — exit cleanly without saving.
			return nil
		}
		if err := selector.SaveUIPref(string(choice)); err != nil {
			return fmt.Errorf("saving ui preference: %w", err)
		}
		pref = string(choice)
	}

	switch pref {
	case string(selector.ChoiceWeb):
		return web.Cmd.RunE(cmd, args)
	case string(selector.ChoiceTUI):
		return tui.Cmd.RunE(cmd, args)
	default:
		return fmt.Errorf("unexpected ui pref: %q", pref)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("config", "", "path to config file")
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))

	// Wire `argus web` to share runAPI (thin wrapper, zero logic duplication).
	// apiCmd is defined at package scope in api.go; its RunE is set before any
	// init() function runs, so this assignment is safe here.
	web.Cmd.RunE = apiCmd.RunE

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(web.Cmd)
	rootCmd.AddCommand(tui.Cmd)
	// resetUICmd registers itself in reset_ui.go's init().
}

func initConfig() {
	if configFile := viper.GetString("config"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("argus")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("/etc/argus")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
	}

	viper.SetEnvPrefix("ARGUS")
	viper.AutomaticEnv()

	// Optional: ignore errors if config file doesn't exist
	viper.ReadInConfig()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
