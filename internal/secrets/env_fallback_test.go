package secrets

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSecretFromStore(t *testing.T) {
	// Clean up any existing store
	defer SetStore(nil)

	tmpDir := t.TempDir()
	path := tmpDir + "/test.key"

	mk, _ := GenerateMasterKey()
	keyBytes, _ := decodeBase64Key(mk)
	store, _ := NewStore(path, keyBytes)

	secrets := map[string]string{
		KeyJWTPrivateKey: "stored-jwt-key",
	}
	_ = store.SaveSecrets(secrets)

	SetStore(store)

	val, ok := GetSecret(KeyJWTPrivateKey)
	assert.True(t, ok)
	assert.Equal(t, "stored-jwt-key", val)
}

func TestGetSecretFromEnv(t *testing.T) {
	defer SetStore(nil)

	os.Setenv("ARGUS_JWT_PRIVATE_KEY_PEM", "env-jwt-key")
	defer os.Unsetenv("ARGUS_JWT_PRIVATE_KEY_PEM")

	val, ok := GetSecret(KeyJWTPrivateKey)
	assert.True(t, ok)
	assert.Equal(t, "env-jwt-key", val)
}

func TestGetSecretNotFound(t *testing.T) {
	defer SetStore(nil)

	// Unset env var
	os.Unsetenv("NONEXISTENT_KEY")

	val, ok := GetSecret("NONEXISTENT_KEY")
	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestGetSecretConcurrency(t *testing.T) {
	defer SetStore(nil)

	tmpDir := t.TempDir()
	path := tmpDir + "/test.key"

	mk, _ := GenerateMasterKey()
	keyBytes, _ := decodeBase64Key(mk)
	store, _ := NewStore(path, keyBytes)

	secrets := map[string]string{
		KeyJWTPrivateKey: "stored-value",
	}
	_ = store.SaveSecrets(secrets)
	SetStore(store)

	var wg sync.WaitGroup
	results := make([]string, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			val, _ := GetSecret(KeyJWTPrivateKey)
			results[idx] = val
		}(i)
	}

	wg.Wait()

	for _, val := range results {
		assert.Equal(t, "stored-value", val)
	}
}

func TestSetStoreReplacesPrevious(t *testing.T) {
	defer SetStore(nil)

	tmpDir := t.TempDir()
	path1 := tmpDir + "/test1.key"
	path2 := tmpDir + "/test2.key"

	mk, _ := GenerateMasterKey()
	keyBytes, _ := decodeBase64Key(mk)

	store1, _ := NewStore(path1, keyBytes)
	_ = store1.SaveSecrets(map[string]string{KeyJWTPrivateKey: "value1"})

	store2, _ := NewStore(path2, keyBytes)
	_ = store2.SaveSecrets(map[string]string{KeyJWTPrivateKey: "value2"})

	SetStore(store1)
	val, _ := GetSecret(KeyJWTPrivateKey)
	assert.Equal(t, "value1", val)

	SetStore(store2)
	val, _ = GetSecret(KeyJWTPrivateKey)
	assert.Equal(t, "value2", val)
}

func TestGetSecretMFAEncryptionKeyFallback(t *testing.T) {
	defer SetStore(nil)

	os.Setenv("ARGUS_MFA_ENCRYPTION_KEY", "mfa-secret")
	defer os.Unsetenv("ARGUS_MFA_ENCRYPTION_KEY")

	val, ok := GetSecret(KeyMFAEncryption)
	assert.True(t, ok)
	assert.Equal(t, "mfa-secret", val)
}

func TestGetSecretPreferStoreOverEnv(t *testing.T) {
	defer SetStore(nil)

	tmpDir := t.TempDir()
	path := tmpDir + "/test.key"

	mk, _ := GenerateMasterKey()
	keyBytes, _ := decodeBase64Key(mk)
	store, _ := NewStore(path, keyBytes)

	_ = store.SaveSecrets(map[string]string{KeyJWTPrivateKey: "stored-value"})
	SetStore(store)

	// Set a different value in env
	os.Setenv("ARGUS_JWT_PRIVATE_KEY_PEM", "env-value")
	defer os.Unsetenv("ARGUS_JWT_PRIVATE_KEY_PEM")

	// Store should win
	val, _ := GetSecret(KeyJWTPrivateKey)
	assert.Equal(t, "stored-value", val)
}

func TestGetSecretEmptyStoreKeyFallsback(t *testing.T) {
	defer SetStore(nil)

	tmpDir := t.TempDir()
	path := tmpDir + "/test.key"

	mk, _ := GenerateMasterKey()
	keyBytes, _ := decodeBase64Key(mk)
	store, _ := NewStore(path, keyBytes)

	// Store exists but doesn't have this key
	_ = store.SaveSecrets(map[string]string{"OTHER_KEY": "value"})
	SetStore(store)

	// Should fall back to env var
	os.Setenv("ARGUS_JWT_PRIVATE_KEY_PEM", "env-value")
	defer os.Unsetenv("ARGUS_JWT_PRIVATE_KEY_PEM")

	val, ok := GetSecret(KeyJWTPrivateKey)
	assert.True(t, ok)
	assert.Equal(t, "env-value", val)
}
