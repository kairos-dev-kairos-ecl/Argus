package keys_test

import (
	"testing"

	"github.com/argusxdr/argus/cmd/argus/tui/keys"
)

// TestBindings_HelpNonEmpty verifies that keys.New().Help() returns bindings.
func TestBindings_HelpNonEmpty(t *testing.T) {
	b := keys.New()
	h := b.Help()
	if len(h) == 0 {
		t.Error("Bindings.Help() returned empty slice")
	}
}

// TestBindings_AllGlobalKeysPresent verifies that named key fields are
// initialized with at least one key binding.
func TestBindings_AllGlobalKeysPresent(t *testing.T) {
	b := keys.New()
	checks := []struct {
		name string
		keys []string
	}{
		{"Quit", b.Quit.Keys()},
		{"Help", b.Help_.Keys()},
		{"NextScreen", b.NextScreen.Keys()},
		{"PrevScreen", b.PrevScreen.Keys()},
		{"Screen1", b.Screen1.Keys()},
		{"Screen6", b.Screen6.Keys()},
		{"Refresh", b.Refresh.Keys()},
		{"Submit", b.Submit.Keys()},
		{"Back", b.Back.Keys()},
	}
	for _, tc := range checks {
		if len(tc.keys) == 0 {
			t.Errorf("Binding %q has no keys assigned", tc.name)
		}
	}
}
