package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/argusxdr/argus/internal/secrets"
	"github.com/spf13/cobra"
)

// secretsCmd represents the top-level secrets command
var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage encrypted secrets (init, set, get, list)",
	Long:  `Manage the argus.key encrypted secrets store. Initialize with a master key, set and retrieve secrets, and list secret names.`,
}

// secretsInitCmd generates a new ARGUS_MASTER_KEY and creates an empty encrypted file
var secretsInitCmd = &cobra.Command{
	Use:   "init [--file ./argus.key] [--force]",
	Short: "Initialize a new encrypted secrets file",
	Long:  `Generate a new ARGUS_MASTER_KEY and create an empty encrypted secrets file. Print the master key to stdout.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		force, _ := cmd.Flags().GetBool("force")

		if filePath == "" {
			filePath = "./argus.key"
		}

		// Check if file exists
		if _, err := os.Stat(filePath); err == nil && !force {
			return fmt.Errorf("file already exists: %s (use --force to overwrite)", filePath)
		}

		// Generate master key
		masterKey, err := secrets.GenerateMasterKey()
		if err != nil {
			return fmt.Errorf("failed to generate master key: %w", err)
		}

		// Create store and save empty secrets
		store, err := secrets.NewStore(filePath, nil)
		if err != nil {
			// If we got here, it means ARGUS_MASTER_KEY wasn't set from NewStore
			// We need to create with our generated key
			keyBytes, err := decodeBase64(masterKey)
			if err != nil {
				return fmt.Errorf("failed to decode generated key: %w", err)
			}
			store, err = secrets.NewStore(filePath, keyBytes)
			if err != nil {
				return fmt.Errorf("failed to create store: %w", err)
			}
		}

		if err := store.SaveSecrets(make(map[string]string)); err != nil {
			return fmt.Errorf("failed to save empty secrets file: %w", err)
		}

		// Print warning banner and key
		fmt.Println("")
		fmt.Println("========================================")
		fmt.Println("IMPORTANT: Master Key Generated")
		fmt.Println("========================================")
		fmt.Println("Save this key in a secure location.")
		fmt.Println("It will NOT be shown again.")
		fmt.Println("")
		fmt.Printf("export ARGUS_MASTER_KEY=%s\n", masterKey)
		fmt.Println("")
		fmt.Printf("Secrets file created at: %s\n", filePath)
		fmt.Println("========================================")
		fmt.Println("")

		return nil
	},
}

// secretsSetCmd sets a secret value
var secretsSetCmd = &cobra.Command{
	Use:   "set <KEY> <VALUE> [--file ./argus.key]",
	Short: "Set a secret value",
	Long:  `Load the secrets file, set a key-value pair, and save it back encrypted.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]
		filePath, _ := cmd.Flags().GetString("file")

		if filePath == "" {
			filePath = "./argus.key"
		}

		// Create store
		store, err := secrets.NewStore(filePath, nil)
		if err != nil {
			if strings.Contains(err.Error(), "ARGUS_MASTER_KEY") {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				fmt.Fprintf(os.Stderr, "To set secrets, first initialize with:\n")
				fmt.Fprintf(os.Stderr, "  export ARGUS_MASTER_KEY=$(argus secrets init)\n")
				return err
			}
			return fmt.Errorf("failed to create store: %w", err)
		}

		// Load existing secrets
		secrets_, err := store.LoadSecrets()
		if err != nil {
			return fmt.Errorf("failed to load secrets: %w", err)
		}

		// Set the key
		secrets_[key] = value

		// Save back
		if err := store.SaveSecrets(secrets_); err != nil {
			return fmt.Errorf("failed to save secrets: %w", err)
		}

		fmt.Printf("Secret set: %s\n", key)
		return nil
	},
}

// secretsGetCmd retrieves a secret value
var secretsGetCmd = &cobra.Command{
	Use:   "get <KEY> [--file ./argus.key]",
	Short: "Retrieve a secret value",
	Long:  `Load the secrets file and print the value of a secret (to stdout only, never to logs).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		filePath, _ := cmd.Flags().GetString("file")

		if filePath == "" {
			filePath = "./argus.key"
		}

		// Create store
		store, err := secrets.NewStore(filePath, nil)
		if err != nil {
			if strings.Contains(err.Error(), "ARGUS_MASTER_KEY") {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				fmt.Fprintf(os.Stderr, "To retrieve secrets, first set ARGUS_MASTER_KEY:\n")
				fmt.Fprintf(os.Stderr, "  export ARGUS_MASTER_KEY=<your-base64-key>\n")
				return err
			}
			return fmt.Errorf("failed to create store: %w", err)
		}

		// Load secrets
		secrets_, err := store.LoadSecrets()
		if err != nil {
			return fmt.Errorf("failed to load secrets: %w", err)
		}

		// Get the value
		value, ok := secrets_[key]
		if !ok {
			return fmt.Errorf("secret not found: %s", key)
		}

		// Print to stdout only (no logging)
		fmt.Println(value)
		return nil
	},
}

// secretsListCmd lists all secret keys (no values)
var secretsListCmd = &cobra.Command{
	Use:   "list [--file ./argus.key]",
	Short: "List all secret keys",
	Long:  `Load the secrets file and print all keys (one per line). Values are never printed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")

		if filePath == "" {
			filePath = "./argus.key"
		}

		// Create store
		store, err := secrets.NewStore(filePath, nil)
		if err != nil {
			if strings.Contains(err.Error(), "ARGUS_MASTER_KEY") {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				fmt.Fprintf(os.Stderr, "To list secrets, first set ARGUS_MASTER_KEY:\n")
				fmt.Fprintf(os.Stderr, "  export ARGUS_MASTER_KEY=<your-base64-key>\n")
				return err
			}
			return fmt.Errorf("failed to create store: %w", err)
		}

		// Load secrets
		secrets_, err := store.LoadSecrets()
		if err != nil {
			return fmt.Errorf("failed to load secrets: %w", err)
		}

		// Print keys only
		if len(secrets_) == 0 {
			fmt.Println("(no secrets)")
			return nil
		}

		for key := range secrets_ {
			fmt.Println(key)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(secretsCmd)

	// Add subcommands
	secretsCmd.AddCommand(secretsInitCmd)
	secretsCmd.AddCommand(secretsSetCmd)
	secretsCmd.AddCommand(secretsGetCmd)
	secretsCmd.AddCommand(secretsListCmd)

	// Add flags
	secretsInitCmd.Flags().String("file", "./argus.key", "path to secrets file")
	secretsInitCmd.Flags().Bool("force", false, "overwrite existing file")

	secretsSetCmd.Flags().String("file", "./argus.key", "path to secrets file")

	secretsGetCmd.Flags().String("file", "./argus.key", "path to secrets file")

	secretsListCmd.Flags().String("file", "./argus.key", "path to secrets file")
}

// decodeBase64 is a helper to decode base64 keys
func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
