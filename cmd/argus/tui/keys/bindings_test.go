package keys_test

import (
	"testing"

	"github.com/argusxdr/argus/cmd/argus/tui/keys"
)

// TestBindings_Sections_OrderAndCounts verifies that Sections() returns sections
// in the correct order and the Global section has exactly 13 bindings.
func TestBindings_Sections_OrderAndCounts(t *testing.T) {
	b := keys.New()
	sections := b.Sections()

	if len(sections) == 0 {
		t.Fatal("Sections() returned empty slice")
	}

	// First section must be "Global".
	if sections[0].Name != "Global" {
		t.Errorf("sections[0].Name = %q, want %q", sections[0].Name, "Global")
	}

	// Global section must have exactly 13 bindings.
	globalCount := len(sections[0].Bindings)
	if globalCount != 13 {
		t.Errorf("Global section binding count = %d, want 13", globalCount)
	}

	// Verify expected section names in order.
	wantNames := []string{"Global", "Navigation", "Signals", "Trace", "Alerts", "Rules", "Users", "Audit"}
	for i, want := range wantNames {
		if i >= len(sections) {
			t.Errorf("missing section at index %d, want %q", i, want)
			continue
		}
		if sections[i].Name != want {
			t.Errorf("sections[%d].Name = %q, want %q", i, sections[i].Name, want)
		}
	}
}

// TestBindings_NoConflicts asserts that no two GLOBAL bindings share the same key string.
func TestBindings_NoConflicts(t *testing.T) {
	b := keys.New()
	sections := b.Sections()

	if len(sections) == 0 {
		t.Fatal("Sections() returned empty slice")
	}

	seen := map[string]string{} // key string -> binding description
	for _, binding := range sections[0].Bindings {
		for _, k := range binding.Keys() {
			if prev, exists := seen[k]; exists {
				t.Errorf("key conflict: key %q used by both %q and %q", k, prev, binding.Help().Desc)
			}
			seen[k] = binding.Help().Desc
		}
	}
}
