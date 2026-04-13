package engine_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

func TestLoadRuleFromFile(t *testing.T) {
	rule, err := engine.LoadRuleFromFile(filepath.Join(testdataDir(), "valid.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "argus-t1-test", rule.ID)
	assert.Equal(t, "Test Rule", rule.Name)
	assert.Equal(t, 1, rule.Tier)
	assert.True(t, rule.Enabled)
	assert.Equal(t, 3, rule.Severity)
	assert.Equal(t, "L6_SAFETY", rule.Conditions.Layer)
	assert.Equal(t, "safety.", rule.Conditions.CategoryPrefix)
	assert.Equal(t, 2, rule.Conditions.SeverityGte)
	assert.Equal(t, "Test Alert", rule.Action.Title)
	assert.Equal(t, "AML.T0051", rule.Action.MitreTechnique)
}

func TestLoadRuleFromFileNotFound(t *testing.T) {
	_, err := engine.LoadRuleFromFile("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestLoadRulesFromDirectory(t *testing.T) {
	rules, err := engine.LoadRulesFromDirectory(testdataDir())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(rules), 1)

	found := false
	for _, r := range rules {
		if r.ID == "argus-t1-test" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected argus-t1-test in loaded rules")
}
