package main

import (
	"bytes"
	"testing"

	"github.com/argusxdr/argus/cmd/argus/selector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetUI_ClearsExistingPref(t *testing.T) {
	dir := t.TempDir()
	restore := selector.SetHomeDirForTest(func() (string, error) { return dir, nil })
	t.Cleanup(restore)

	// Write a pref first.
	require.NoError(t, selector.SaveUIPref("web"))

	// Sanity: confirm it's there.
	got, err := selector.LoadUIPref()
	require.NoError(t, err)
	require.Equal(t, "web", got)

	// Run the reset-ui command.
	cmd := resetUICmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = runResetUI(cmd, nil)
	require.NoError(t, err)

	// Pref must be gone.
	got, err = selector.LoadUIPref()
	require.NoError(t, err)
	assert.Equal(t, "", got, "expected empty pref after reset-ui")

	// Success message must be present.
	assert.Contains(t, buf.String(), "Cleared UI preference")
}

func TestResetUI_NoExistingPref_NoError(t *testing.T) {
	dir := t.TempDir()
	restore := selector.SetHomeDirForTest(func() (string, error) { return dir, nil })
	t.Cleanup(restore)

	// No pref written — reset-ui must return nil.
	cmd := resetUICmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runResetUI(cmd, nil)
	require.NoError(t, err)

	// Success message still printed.
	assert.Contains(t, buf.String(), "Cleared UI preference")
}
