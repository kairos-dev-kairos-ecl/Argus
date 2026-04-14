package engine_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/stretchr/testify/require"
)

func TestLoadBuiltInRules(t *testing.T) {
	_, f, _, _ := runtime.Caller(0)
	dir := filepath.Clean(filepath.Join(filepath.Dir(f), "..", "..", "rules", "built-in"))

	rules, err := engine.LoadRulesFromDirectory(dir)
	require.NoError(t, err)
	require.Len(t, rules, 15)
}
