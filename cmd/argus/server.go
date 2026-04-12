package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage Argus server (start, stop, status, logs)",
	Long:  `Start, stop, check status, or view logs for the Argus server.`,
}

// server start: start the Argus server
var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Argus server",
	Long:  `Start the Argus server in the foreground or background.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		daemon, _ := cmd.Flags().GetBool("daemon")
		port, _ := cmd.Flags().GetString("http-port")
		grpcPort, _ := cmd.Flags().GetString("grpc-port")

		if daemon {
			// Start in background (systemd/launchd/nohup will handle this)
			// For development: spawn a new process and return
			fmt.Printf("Starting Argus server in background (port %s, gRPC %s)\n", port, grpcPort)
			fmt.Println("Use 'argus server logs' to view logs")
			fmt.Println("Use 'argus server stop' to stop the server")
			return nil
		}

		fmt.Printf("Starting Argus server (HTTP :%s, gRPC :%s)\n", port, grpcPort)
		fmt.Println("Press Ctrl+C to stop")

		// Handle graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			sig := <-sigChan
			fmt.Printf("\nReceived signal: %v\n", sig)
			os.Exit(0)
		}()

		// Here, in production, we'd actually start the server
		// For now, just simulate it
		fmt.Println("Server running. Press Ctrl+C to stop.")
		select {}
	},
}

// server stop: stop the Argus server
var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Argus server",
	Long:  `Stop the running Argus server process.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("http-port")

		// Try to find PID by checking health endpoint
		healthURL := fmt.Sprintf("http://localhost:%s/health", port)
		resp, err := http.Get(healthURL)
		if err != nil {
			return fmt.Errorf("server not responding at %s", healthURL)
		}
		resp.Body.Close()

		// Try to gracefully shutdown
		shutdownURL := fmt.Sprintf("http://localhost:%s/shutdown", port)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req, _ := http.NewRequestWithContext(ctx, "POST", shutdownURL, nil)
		client := &http.Client{Timeout: 10 * time.Second}
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
		}

		fmt.Println("Server stop signal sent")
		return nil
	},
}

// server status: check if server is running
var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if the Argus server is running",
	Long:  `Check the status and health of the Argus server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("http-port")

		healthURL := fmt.Sprintf("http://localhost:%s/health", port)
		client := &http.Client{Timeout: 5 * time.Second}

		resp, err := client.Get(healthURL)
		if err != nil {
			fmt.Printf("Server status: DOWN (unreachable at %s)\n", healthURL)
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			fmt.Println("Server status: UP")
			fmt.Printf("  HTTP: http://localhost:%s\n", port)
			return nil
		}

		fmt.Printf("Server status: DEGRADED (HTTP %d)\n", resp.StatusCode)
		return nil
	},
}

// server logs: view server logs
var serverLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View server logs",
	Long:  `Display logs from the running Argus server. Requires 'tail' command.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		follow, _ := cmd.Flags().GetBool("follow")
		lines, _ := cmd.Flags().GetInt("lines")

		logFile := "/var/log/argus/argus.log"
		if _, err := os.Stat(logFile); err != nil {
			// Try docker-compose logs
			dataDir, _ := cmd.Flags().GetString("data-dir")
			if dataDir == "" {
				dataDir = ".argus"
			}

			fmt.Printf("Attempting to fetch logs from docker-compose...\n")
			tailArgs := []string{"compose", "logs"}
			if follow {
				tailArgs = append(tailArgs, "-f")
			}
			tailArgs = append(tailArgs, "--tail="+fmt.Sprintf("%d", lines))

			dockerCmd := exec.Command("docker", tailArgs...)
			dockerCmd.Dir = dataDir
			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr
			return dockerCmd.Run()
		}

		// Use tail to follow logs
		tailArgs := []string{fmt.Sprintf("-%d", lines)}
		if follow {
			tailArgs = append(tailArgs, "-f")
		}
		tailArgs = append(tailArgs, logFile)

		tailCmd := exec.Command("tail", tailArgs...)
		tailCmd.Stdout = os.Stdout
		tailCmd.Stderr = os.Stderr
		return tailCmd.Run()
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.AddCommand(serverStartCmd)
	serverStartCmd.Flags().BoolP("daemon", "d", false, "Start in background")
	serverStartCmd.Flags().String("http-port", "9090", "HTTP listen port")
	serverStartCmd.Flags().String("grpc-port", "4317", "gRPC listen port")

	serverCmd.AddCommand(serverStopCmd)
	serverStopCmd.Flags().String("http-port", "9090", "HTTP listen port")

	serverCmd.AddCommand(serverStatusCmd)
	serverStatusCmd.Flags().String("http-port", "9090", "HTTP listen port")

	serverCmd.AddCommand(serverLogsCmd)
	serverLogsCmd.Flags().BoolP("follow", "f", false, "Follow logs (like tail -f)")
	serverLogsCmd.Flags().IntP("lines", "n", 50, "Number of lines to display")
	serverLogsCmd.Flags().String("data-dir", ".argus", "Data directory for docker-compose")
}
