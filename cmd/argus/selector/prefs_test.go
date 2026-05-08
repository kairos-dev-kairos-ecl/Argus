package selector

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withTempHome overrides the home directory to dir for the duration of the test.
func withTempHome(t *testing.T, dir string) {
	t.Helper()
	restore := SetHomeDirForTest(func() (string, error) { return dir, nil })
	t.Cleanup(restore)
}

func TestSaveAndLoadUIPref_Web(t *testing.T) {
	dir := t.TempDir()
	withTempHome(t, dir)

	err := SaveUIPref("web")
	require.NoError(t, err)

	got, err := LoadUIPref()
	require.NoError(t, err)
	assert.Equal(t, "web", got)
}

func TestSaveAndLoadUIPref_TUI(t *testing.T) {
	dir := t.TempDir()
	withTempHome(t, dir)

	err := SaveUIPref("tui")
	require.NoError(t, err)

	got, err := LoadUIPref()
	require.NoError(t, err)
	assert.Equal(t, "tui", got)
}

func TestLoadUIPref_NoFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	withTempHome(t, dir)

	// No file written — should return ("", nil), not an error.
	got, err := LoadUIPref()
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestClearUIPref_RemovesUIKey(t *testing.T) {
	dir := t.TempDir()
	withTempHome(t, dir)

	require.NoError(t, SaveUIPref("web"))

	// Verify it's set.
	got, err := LoadUIPref()
	require.NoError(t, err)
	require.Equal(t, "web", got)

	// Clear it.
	require.NoError(t, ClearUIPref())

	// Subsequent load should return empty.
	got, err = LoadUIPref()
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestClearUIPref_NoFile_NoError(t *testing.T) {
	dir := t.TempDir()
	withTempHome(t, dir)

	// ClearUIPref on non-existent file must not return an error.
	err := ClearUIPref()
	require.NoError(t, err)
}

func TestSaveUIPref_FileMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permission bits are not enforced on Windows")
	}
	dir := t.TempDir()
	withTempHome(t, dir)

	require.NoError(t, SaveUIPref("tui"))

	path := filepath.Join(dir, ".argus", "config.yaml")
	info, err := os.Stat(path)
	require.NoError(t, err)

	perm := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0600), perm, "expected file mode 0600, got %v", perm)
}

func TestSaveUIPref_InvalidChoice(t *testing.T) {
	dir := t.TempDir()
	withTempHome(t, dir)

	err := SaveUIPref("browser")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ui choice")
}

func TestSaveUIPref_InvalidChoice_Empty(t *testing.T) {
	dir := t.TempDir()
	withTempHome(t, dir)

	err := SaveUIPref("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ui choice")
}

func TestSaveUIPref_DirCreated0700(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permission bits are not enforced on Windows")
	}
	dir := t.TempDir()
	withTempHome(t, dir)

	require.NoError(t, SaveUIPref("web"))

	argusDir := filepath.Join(dir, ".argus")
	info, err := os.Stat(argusDir)
	require.NoError(t, err)

	perm := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0700), perm, "expected dir mode 0700, got %v", perm)
}

func TestSaveUIPref_Overwrite(t *testing.T) {
	dir := t.TempDir()
	withTempHome(t, dir)

	require.NoError(t, SaveUIPref("web"))
	require.NoError(t, SaveUIPref("tui"))

	got, err := LoadUIPref()
	require.NoError(t, err)
	assert.Equal(t, "tui", got)
}
