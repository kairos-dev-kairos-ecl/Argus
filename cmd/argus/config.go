package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Argus configuration (show, set, validate)",
	Long:  `View, update, or validate the Argus configuration file.`,
}

// config show: display current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	Long:  `Show the current Argus configuration (from argus.yaml or env vars).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configFile := viper.ConfigFileUsed()
		if configFile == "" {
			configFile = "argus.yaml"
		}

		fmt.Printf("Configuration file: %s\n\n", configFile)

		// Read and display configuration
		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		fmt.Println(string(data))
		return nil
	},
}

// config set: set a configuration value
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long:  `Set a configuration value in the argus.yaml file.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		viper.Set(key, value)

		configFile := viper.ConfigFileUsed()
		if configFile == "" {
			configFile = "argus.yaml"
		}

		if err := viper.WriteConfigAs(configFile); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Printf("Configuration updated: %s = %s\n", key, value)
		return nil
	},
}

// config validate: validate configuration
var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long:  `Check that the configuration file is valid and all required fields are present.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Required configuration sections
		requiredSections := []string{
			"server",
			"database",
			"storage",
			"logging",
		}

		configFile := viper.ConfigFileUsed()
		if configFile == "" {
			configFile = "argus.yaml"
		}

		fmt.Printf("Validating configuration: %s\n\n", configFile)

		allValid := true
		for _, section := range requiredSections {
			val := viper.Get(section)
			if val == nil {
				fmt.Printf("✗ Missing section: %s\n", section)
				allValid = false
			} else {
				fmt.Printf("✓ Section found: %s\n", section)
			}
		}

		if !allValid {
			return fmt.Errorf("configuration validation failed")
		}

		fmt.Println("\nConfiguration is valid!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configShowCmd)

	configCmd.AddCommand(configSetCmd)

	configCmd.AddCommand(configValidateCmd)
}
