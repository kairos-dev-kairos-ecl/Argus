package tui_test

import (
	"regexp"
	"strings"
	"testing"

	tui "github.com/argusxdr/argus/cmd/argus/tui"
)

// stripANSI removes ANSI color/style escape sequences from s.
func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

// TestHelpOverlay_ContainsAllGlobalBindings verifies the help overlay includes
// all expected global key identifiers.
func TestHelpOverlay_ContainsAllGlobalBindings(t *testing.T) {
	app := tui.New(tui.Config{BaseURL: "http://localhost:9999"})

	// Render help overlay.
	overlay := tui.RenderHelpOverlayForTest(app)
	plain := stripANSI(overlay)

	wantKeys := []string{"ctrl+c", "?", "tab", "1", "2", "3", "4", "5", "6"}
	for _, k := range wantKeys {
		if !strings.Contains(plain, k) {
			t.Errorf("help overlay missing global key %q", k)
		}
	}
}

// TestHelpOverlay_ShowsCurrentScreenSectionOnly verifies that when on the Alerts
// screen, the overlay shows an "Alerts" section heading but NOT a "Trace" heading.
func TestHelpOverlay_ShowsCurrentScreenSectionOnly(t *testing.T) {
	app := tui.New(tui.Config{BaseURL: "http://localhost:9999"})
	tui.SetScreenForTest(app, tui.ScreenAlerts)

	overlay := tui.RenderHelpOverlayForTest(app)
	plain := stripANSI(overlay)

	if !strings.Contains(plain, "Alerts") {
		t.Error("help overlay should contain 'Alerts' section heading when on Alerts screen")
	}
	if strings.Contains(plain, "Trace") {
		t.Error("help overlay should NOT contain 'Trace' section heading when on Alerts screen")
	}
}
