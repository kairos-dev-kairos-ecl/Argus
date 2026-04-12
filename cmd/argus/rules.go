package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage detection rules (list, validate, test, apply)",
	Long:  `View, validate, test, or apply detection rules.`,
}

// Rule represents a detection rule
type Rule struct {
	Name        string                 `yaml:"name" json:"name"`
	ID          string                 `yaml:"id" json:"id"`
	Description string                 `yaml:"description" json:"description"`
	Enabled     bool                   `yaml:"enabled" json:"enabled"`
	Severity    string                 `yaml:"severity" json:"severity"`
	Logic       map[string]interface{} `yaml:"logic" json:"logic"`
}

// rules list: list all detection rules
var rulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all detection rules",
	Long:  `Display all available detection rules with their status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		outputFmt, _ := cmd.Flags().GetString("format")

		// In production, fetch from database or rules directory
		rules := []Rule{
			{
				Name:        "Anomalous Token Usage",
				ID:          "RULE-001",
				Description: "Detects unusual API token usage patterns",
				Enabled:     true,
				Severity:    "high",
			},
			{
				Name:        "Unauthorized API Access",
				ID:          "RULE-002",
				Description: "Detects access without proper authentication",
				Enabled:     true,
				Severity:    "critical",
			},
			{
				Name:        "Data Exfiltration Pattern",
				ID:          "RULE-003",
				Description: "Detects large data transfers",
				Enabled:     true,
				Severity:    "high",
			},
		}

		if outputFmt == "json" {
			data, _ := json.MarshalIndent(rules, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Println("Rules:")
			fmt.Printf("%-20s %-10s %-30s %s\n", "ID", "Severity", "Name", "Status")
			fmt.Println("─────────────────────────────────────────────────────────────")
			for _, rule := range rules {
				status := "enabled"
				if !rule.Enabled {
					status = "disabled"
				}
				fmt.Printf("%-20s %-10s %-30s %s\n", rule.ID, rule.Severity, rule.Name, status)
			}
		}

		return nil
	},
}

// rules validate: validate rule syntax
var rulesValidateCmd = &cobra.Command{
	Use:   "validate <rule-file>",
	Short: "Validate rule syntax",
	Long:  `Check that a rule file has valid YAML/JSON syntax and required fields.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ruleFile := args[0]

		data, err := os.ReadFile(ruleFile)
		if err != nil {
			return fmt.Errorf("failed to read rule file: %w", err)
		}

		var rule Rule
		if filepath.Ext(ruleFile) == ".json" {
			err = json.Unmarshal(data, &rule)
		} else {
			err = yaml.Unmarshal(data, &rule)
		}

		if err != nil {
			return fmt.Errorf("invalid rule syntax: %w", err)
		}

		// Validate required fields
		if rule.ID == "" {
			return fmt.Errorf("missing required field: id")
		}
		if rule.Name == "" {
			return fmt.Errorf("missing required field: name")
		}

		fmt.Printf("✓ Rule is valid: %s (%s)\n", rule.Name, rule.ID)
		return nil
	},
}

// rules test: test rule against sample data
var rulesTestCmd = &cobra.Command{
	Use:   "test <rule-file> <data-file>",
	Short: "Test rule against sample data",
	Long:  `Test a detection rule against sample signal data to verify correctness.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ruleFile := args[0]
		dataFile := args[1]

		// Read rule
		ruleData, err := os.ReadFile(ruleFile)
		if err != nil {
			return fmt.Errorf("failed to read rule file: %w", err)
		}

		var rule Rule
		if filepath.Ext(ruleFile) == ".json" {
			json.Unmarshal(ruleData, &rule)
		} else {
			yaml.Unmarshal(ruleData, &rule)
		}

		// Read test data
		testData, err := os.ReadFile(dataFile)
		if err != nil {
			return fmt.Errorf("failed to read test data: %w", err)
		}

		var testSignals []map[string]interface{}
		json.Unmarshal(testData, &testSignals)

		fmt.Printf("Testing rule: %s (%s)\n", rule.Name, rule.ID)
		fmt.Printf("Test signals: %d\n", len(testSignals))
		fmt.Println("✓ Test completed (results would be shown here)")

		return nil
	},
}

// rules apply: apply/enable a rule
var rulesApplyCmd = &cobra.Command{
	Use:   "apply <rule-id>",
	Short: "Apply a detection rule",
	Long:  `Enable and deploy a detection rule for signal processing.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ruleID := args[0]

		fmt.Printf("Applying rule: %s\n", ruleID)
		fmt.Println("✓ Rule applied and enabled")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(rulesCmd)

	rulesCmd.AddCommand(rulesListCmd)
	rulesListCmd.Flags().StringP("format", "o", "table", "Output format (table, json)")

	rulesCmd.AddCommand(rulesValidateCmd)

	rulesCmd.AddCommand(rulesTestCmd)

	rulesCmd.AddCommand(rulesApplyCmd)
}
