package secrets

import (
	"encoding/base64"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreSaveLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.key"

	// Generate a master key
	mk, err := GenerateMasterKey()
	require.NoError(t, err)
	keyBytes, err := decodeBase64Key(mk)
	require.NoError(t, err)

	store, err := NewStore(path, keyBytes)
	require.NoError(t, err)

	// Save secrets
	original := map[string]string{
		"password": "secret123",
		"api_key":  "sk-1234567890",
		"token":    "eyJhbGciOiJIUzI1NiJ9",
	}

	err = store.SaveSecrets(original)
	require.NoError(t, err)

	// Load and verify
	loaded, err := store.LoadSecrets()
	require.NoError(t, err)
	assert.Equal(t, original, loaded)
}

func TestLoadSecretsFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/nonexistent.key"

	mk, _ := GenerateMasterKey()
	keyBytes, _ := decodeBase64Key(mk)
	store, _ := NewStore(path, keyBytes)

	loaded, err := store.LoadSecrets()
	require.NoError(t, err)
	assert.Equal(t, make(map[string]string), loaded)
}

func TestSaveSecretsFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions test skipped on Windows")
	}

	tmpDir := t.TempDir()
	path := tmpDir + "/test.key"

	mk, _ := GenerateMasterKey()
	keyBytes, _ := decodeBase64Key(mk)
	store, _ := NewStore(path, keyBytes)

	err := store.SaveSecrets(map[string]string{"key": "value"})
	require.NoError(t, err)

	// Check permissions
	info, err := os.Stat(path)
	require.NoError(t, err)
	perms := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0600), perms)
}

func TestSaveSecretsRandomNonce(t *testing.T) {
	tmpDir := t.TempDir()
	path1 := tmpDir + "/test1.key"
	path2 := tmpDir + "/test2.key"

	mk, _ := GenerateMasterKey()
	keyBytes, _ := decodeBase64Key(mk)
	store1, _ := NewStore(path1, keyBytes)
	store2, _ := NewStore(path2, keyBytes)

	secrets := map[string]string{"password": "secret123"}

	err := store1.SaveSecrets(secrets)
	require.NoError(t, err)

	err = store2.SaveSecrets(secrets)
	require.NoError(t, err)

	// Read both files
	data1, err := os.ReadFile(path1)
	require.NoError(t, err)
	data2, err := os.ReadFile(path2)
	require.NoError(t, err)

	// Ciphertexts should differ (different nonces)
	assert.NotEqual(t, data1, data2, "same plaintext with different nonces should produce different ciphertexts")
}

func TestLoadSecretsCorruptedCiphertext(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/corrupt.key"

	mk, _ := GenerateMasterKey()
	keyBytes, _ := decodeBase64Key(mk)
	store, _ := NewStore(path, keyBytes)

	// Save valid file
	err := store.SaveSecrets(map[string]string{"key": "value"})
	require.NoError(t, err)

	// Corrupt the ciphertext (skip header, corrupt a byte)
	data, _ := os.ReadFile(path)
	data[20] ^= 0xFF // flip a bit in the ciphertext
	os.WriteFile(path, data, 0600)

	// Load should fail with auth error
	_, err = store.LoadSecrets()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication tag mismatch")
}

func TestLoadSecretsWrongMasterKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.key"

	mk1, _ := GenerateMasterKey()
	keyBytes1, _ := decodeBase64Key(mk1)
	store1, _ := NewStore(path, keyBytes1)

	err := store1.SaveSecrets(map[string]string{"key": "value"})
	require.NoError(t, err)

	// Try to load with wrong key
	mk2, _ := GenerateMasterKey()
	keyBytes2, _ := decodeBase64Key(mk2)
	store2, _ := NewStore(path, keyBytes2)

	_, err = store2.LoadSecrets()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication tag mismatch")
}

func TestGenerateMasterKey(t *testing.T) {
	key1, err := GenerateMasterKey()
	require.NoError(t, err)
	assert.NotEmpty(t, key1)

	// Decode and check length
	keyBytes, err := decodeBase64Key(key1)
	require.NoError(t, err)
	assert.Equal(t, 32, len(keyBytes))

	// Two calls should produce different keys
	key2, _ := GenerateMasterKey()
	assert.NotEqual(t, key1, key2)
}

func TestNewStoreValidatesMasterKeyLength(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.key"

	tests := []struct {
		name    string
		keyLen  int
		wantErr bool
	}{
		{"32-byte key is valid", 32, false},
		{"31-byte key is invalid", 31, true},
		{"33-byte key is invalid", 33, true},
		{"0-byte key is invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)
			_, err := NewStore(path, key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewStoreReadsMasterKeyFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.key"

	mk, _ := GenerateMasterKey()
	os.Setenv("ARGUS_MASTER_KEY", mk)
	defer os.Unsetenv("ARGUS_MASTER_KEY")

	// nil masterKey should read from env
	store, err := NewStore(path, nil)
	require.NoError(t, err)
	assert.NotNil(t, store)
}

func TestNewStoreRequiresMasterKeyOrEnv(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.key"

	os.Unsetenv("ARGUS_MASTER_KEY")

	_, err := NewStore(path, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ARGUS_MASTER_KEY")
}

// Helper: decode base64 key
func decodeBase64Key(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
