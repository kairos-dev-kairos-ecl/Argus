// Package selector provides the UI preference storage layer and the Bubbletea
// interface-selector application for the Argus XDR CLI.
//
// Security requirements enforced here:
//   - Config directory: 0700
//   - Config file: 0600
//   - Atomic write: os.CreateTemp → write → Chmod → Sync → Close → Rename
//   - Home directory: os.UserHomeDir() via homeDirFn seam (never $HOME directly)
//   - No credential logging at any level
package selector

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// homeDirFn is a test seam. Override in tests via SetHomeDirForTest.
var homeDirFn = os.UserHomeDir

// SetHomeDirForTest overrides the home directory function for the duration of a
// test. Call the returned cleanup function (e.g. via t.Cleanup) to restore the
// original.
func SetHomeDirForTest(fn func() (string, error)) func() {
	orig := homeDirFn
	homeDirFn = fn
	return func() { homeDirFn = orig }
}

// configDir returns the path to ~/.argus/.
func configDir() (string, error) {
	home, err := homeDirFn()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".argus"), nil
}

// configPath returns the path to ~/.argus/config.yaml.
func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// uiConfig is the YAML structure persisted to disk.
type uiConfig struct {
	UI string `yaml:"ui"`
}

// LoadUIPref reads the saved UI preference from ~/.argus/config.yaml.
// Returns ("", nil) when the file does not exist — that is not an error.
func LoadUIPref() (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading ui preference file: %w", err)
	}

	var cfg uiConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parsing ui preference file: %w", err)
	}

	return cfg.UI, nil
}

// SaveUIPref atomically writes the UI preference to ~/.argus/config.yaml with
// file mode 0600. Only "web" and "tui" are valid choices.
func SaveUIPref(choice string) error {
	if choice != "web" && choice != "tui" {
		return fmt.Errorf("invalid ui choice %q: must be \"web\" or \"tui\"", choice)
	}

	dir, err := configDir()
	if err != nil {
		return err
	}

	// Ensure ~/.argus/ exists with mode 0700.
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	finalPath := filepath.Join(dir, "config.yaml")

	// Marshal the config struct to YAML bytes.
	cfg := uiConfig{UI: choice}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshalling ui preference: %w", err)
	}

	// Atomic write: temp file → chmod → sync → close → rename.
	tmp, err := os.CreateTemp(dir, "config-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file for ui preference: %w", err)
	}
	tmpName := tmp.Name()

	// If anything goes wrong before rename, clean up the temp file.
	success := false
	defer func() {
		if !success {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("writing ui preference: %w", err)
	}

	if err := tmp.Chmod(0600); err != nil {
		return fmt.Errorf("setting permissions on ui preference temp file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("syncing ui preference temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing ui preference temp file: %w", err)
	}

	if err := os.Rename(tmpName, finalPath); err != nil {
		return fmt.Errorf("renaming ui preference file into place: %w", err)
	}

	// Defensive chmod after rename — some platforms reset permissions.
	if err := os.Chmod(finalPath, 0600); err != nil {
		return fmt.Errorf("setting final permissions on ui preference: %w", err)
	}

	success = true
	return nil
}

// ClearUIPref removes the saved UI preference so the next `argus` invocation
// shows the selector. If the file does not exist, this is a no-op.
func ClearUIPref() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing ui preference file: %w", err)
	}

	return nil
}
