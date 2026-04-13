package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

// DoctorCmd checks system dependencies, connectivity, and Argus health
var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostic checks on Argus dependencies and connectivity",
	Long:  `Verify that all required dependencies are installed, services are reachable, and Argus is healthy.`,
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Print("\n╔══════════════════════════════════════════╗\n")
	fmt.Print("║  ARGUS DOCTOR — DIAGNOSTIC CHECK        ║\n")
	fmt.Print("╚══════════════════════════════════════════╝\n\n")

	checks := []struct {
		name string
		fn   func() bool
	}{
		{"Docker/Podman", checkContainer},
		{"ClickHouse connectivity", checkClickHouse},
		{"PostgreSQL connectivity", checkPostgreSQL},
		{"Redis connectivity", checkRedis},
		{"Argus API health", checkArgusAPI},
		{"Signal ingestion (gRPC)", checkGRPC},
		{"File permissions", checkFilePermissions},
	}

	passed := 0
	failed := 0

	for _, check := range checks {
		fmt.Printf("• %-40s ", check.name)
		if check.fn() {
			fmt.Println("✓")
			passed++
		} else {
			fmt.Println("✗")
			failed++
		}
	}

	fmt.Printf("\n\nResult: %d/%d checks passed\n", passed, len(checks))
	if failed > 0 {
		fmt.Printf("⚠  %d checks failed — review errors above\n", failed)
		os.Exit(1)
	} else {
		fmt.Println("✓ All systems operational")
		os.Exit(0)
	}

	return nil
}

func checkContainer() bool {
	_, err := exec.LookPath("docker")
	if err == nil {
		// Test docker connectivity
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = exec.CommandContext(ctx, "docker", "ps", "-q").Run()
		return err == nil
	}

	_, err = exec.LookPath("podman")
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = exec.CommandContext(ctx, "podman", "ps", "-q").Run()
		return err == nil
	}

	fmt.Println("\n  → Docker or Podman not found. Install Docker: https://docs.docker.com/get-docker/")
	return false
}

func checkClickHouse() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", "localhost:8123")
	if err != nil {
		fmt.Printf("\n  → ClickHouse unreachable at localhost:8123\n")
		return false
	}
	conn.Close()
	return true
}

func checkPostgreSQL() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", "localhost:5432")
	if err != nil {
		fmt.Printf("\n  → PostgreSQL unreachable at localhost:5432\n")
		return false
	}
	conn.Close()
	return true
}

func checkRedis() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", "localhost:6379")
	if err != nil {
		fmt.Printf("\n  → Redis unreachable at localhost:6379\n")
		return false
	}
	conn.Close()
	return true
}

func checkArgusAPI() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", "localhost:9090")
	if err != nil {
		fmt.Printf("\n  → Argus API unreachable at localhost:9090\n")
		return false
	}
	conn.Close()
	return true
}

func checkGRPC() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", "localhost:4317")
	if err != nil {
		fmt.Printf("\n  → gRPC signal ingestion unreachable at localhost:4317\n")
		return false
	}
	conn.Close()
	return true
}

func checkFilePermissions() bool {
	// Check if Argus data directory is writable
	dataDir := "/var/lib/argus"
	_, err := os.Stat(dataDir)
	if os.IsNotExist(err) {
		// If doesn't exist yet, that's OK — will be created on first run
		return true
	}

	// Test write permission
	testFile := dataDir + "/.doctor-test"
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		fmt.Printf("\n  → Cannot write to %s — check permissions\n", dataDir)
		return false
	}
	os.Remove(testFile)
	return true
}

func init() {
	rootCmd.AddCommand(DoctorCmd)
}
