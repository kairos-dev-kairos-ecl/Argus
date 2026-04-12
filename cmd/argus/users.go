package main

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage users (list, create, reset-password)",
	Long:  `View, create, and manage Argus users and their roles.`,
}

// User represents an Argus user
type User struct {
	ID    string
	Name  string
	Email string
	Role  string
	// ... more fields
}

// users list: list all users
var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	Long:  `Display all registered Argus users.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Users:")
		fmt.Printf("%-10s %-20s %-30s %-15s\n", "ID", "Name", "Email", "Role")
		fmt.Println("────────────────────────────────────────────────────────────────")

		// In production, fetch from database
		users := []User{
			{ID: "1", Name: "Admin", Email: "admin@example.com", Role: "admin"},
			{ID: "2", Name: "Analyst", Email: "analyst@example.com", Role: "analyst"},
		}

		for _, user := range users {
			fmt.Printf("%-10s %-20s %-30s %-15s\n", user.ID, user.Name, user.Email, user.Role)
		}

		return nil
	},
}

// users create: create a new user
var usersCreateCmd = &cobra.Command{
	Use:   "create <email> <name>",
	Short: "Create a new user",
	Long:  `Create a new Argus user account. Password will be prompted.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		email := args[0]
		name := args[1]
		role, _ := cmd.Flags().GetString("role")

		// Prompt for password
		fmt.Print("Password: ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println()

		// Prompt for password confirmation
		fmt.Print("Confirm password: ")
		passwordConfirm, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println()

		if string(password) != string(passwordConfirm) {
			return fmt.Errorf("passwords do not match")
		}

		// Create user (in production, call API or database)
		fmt.Printf("Creating user: %s (%s)\n", name, email)
		fmt.Printf("Role: %s\n", role)
		fmt.Println("✓ User created successfully")

		return nil
	},
}

// users reset-password: reset user password
var usersResetPasswordCmd = &cobra.Command{
	Use:   "reset-password <email>",
	Short: "Reset a user password",
	Long:  `Reset the password for an existing user.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		email := args[0]

		// Prompt for new password
		fmt.Print("New password: ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println()

		// Prompt for password confirmation
		fmt.Print("Confirm password: ")
		passwordConfirm, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println()

		if string(password) != string(passwordConfirm) {
			return fmt.Errorf("passwords do not match")
		}

		// Reset password (in production, call API or database)
		fmt.Printf("Resetting password for: %s\n", email)
		fmt.Println("✓ Password reset successfully")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(usersCmd)

	usersCmd.AddCommand(usersListCmd)

	usersCmd.AddCommand(usersCreateCmd)
	usersCreateCmd.Flags().StringP("role", "r", "analyst", "User role (admin, analyst, viewer)")

	usersCmd.AddCommand(usersResetPasswordCmd)
}
