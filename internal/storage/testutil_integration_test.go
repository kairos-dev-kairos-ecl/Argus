//go:build integration

package storage_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationsDir returns the absolute path to the migrations directory.
func migrationsDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "migrations")
}

// ApplyMigrations applies migration SQL files in numeric order up to (and including) upTo.
// direction must be "up" or "down".
func ApplyMigrations(t *testing.T, pool *pgxpool.Pool, direction string, upTo int) {
	t.Helper()
	dir := migrationsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	suffix := fmt.Sprintf(".%s.sql", direction)
	type migFile struct {
		num  int
		path string
	}
	var files []migFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		// Extract leading number from filename e.g. "018_create_alerts_v2.up.sql" → 18
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		if n > upTo {
			continue
		}
		files = append(files, migFile{num: n, path: filepath.Join(dir, e.Name())})
	}

	if direction == "down" {
		// Apply down migrations in reverse numeric order
		sort.Slice(files, func(i, j int) bool { return files[i].num > files[j].num })
	} else {
		sort.Slice(files, func(i, j int) bool { return files[i].num < files[j].num })
	}

	for _, f := range files {
		sql, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatalf("read migration %s: %v", f.path, err)
		}
		if _, err := pool.Exec(context.Background(), string(sql)); err != nil {
			t.Fatalf("exec migration %s: %v", f.path, err)
		}
	}
}
