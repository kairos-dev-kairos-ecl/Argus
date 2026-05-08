package tui

// Version is the semantic version of the Argus TUI.
const Version = "0.4.0"

// VersionString returns the human-readable version string printed by --version.
func VersionString() string {
	return "Argus TUI v" + Version + " (Phase 4 complete)"
}
