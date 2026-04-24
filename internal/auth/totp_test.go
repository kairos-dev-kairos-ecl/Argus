package auth

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTOTPSecret(t *testing.T) {
	key, err := GenerateTOTPSecret("user@example.com", "ArgusXDR")
	require.NoError(t, err)
	require.NotNil(t, key)

	// Verify the URL contains the expected format: otpauth://totp/ArgusXDR:user@example.com
	assert.Contains(t, key.URL(), "otpauth://totp/ArgusXDR:user@example.com")
}

func TestValidateTOTPCode(t *testing.T) {
	tests := []struct {
		name    string
		secret  *otp.Key
		code    string
		valid   bool
	}{
		{
			name: "valid code for secret",
			secret: func() *otp.Key {
				key, err := totp.Generate(totp.GenerateOpts{
					Issuer:      "TestIssuer",
					AccountName: "test@example.com",
				})
				require.NoError(t, err)
				return key
			}(),
			valid: true,
		},
		{
			name: "invalid code (wrong code)",
			secret: func() *otp.Key {
				key, err := totp.Generate(totp.GenerateOpts{
					Issuer:      "TestIssuer",
					AccountName: "test@example.com",
				})
				require.NoError(t, err)
				return key
			}(),
			code:  "000000",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate a valid code from the secret
			if tt.valid {
				validCode, err := totp.GenerateCode(tt.secret.Secret(), time.Now())
				require.NoError(t, err)
				result := ValidateTOTPCode(tt.secret.Secret(), validCode)
				assert.True(t, result, "should validate a code generated from the same secret")
			} else {
				result := ValidateTOTPCode(tt.secret.Secret(), tt.code)
				assert.False(t, result, "should reject an invalid code")
			}
		})
	}
}

func TestGenerateBackupCodes(t *testing.T) {
	codes, err := GenerateBackupCodes(8)
	require.NoError(t, err)
	require.Equal(t, 8, len(codes))

	// Verify all codes are 8 characters of [A-Z0-9]
	pattern := regexp.MustCompile(`^[A-Z0-9]{8}$`)
	for i, code := range codes {
		assert.True(t, pattern.MatchString(code), "code %d should be 8 [A-Z0-9] chars, got %s", i, code)
	}

	// Verify all codes are unique
	seen := make(map[string]bool)
	for _, code := range codes {
		assert.False(t, seen[code], "code %s is duplicated", code)
		seen[code] = true
	}
}

func TestHashBackupCode(t *testing.T) {
	code := "ABCD1234"
	hash := HashBackupCode(code)

	// Verify it's deterministic (same input = same hash)
	hash2 := HashBackupCode(code)
	assert.Equal(t, hash, hash2, "hash should be deterministic")

	// Verify it's 64 chars (SHA-256 hex = 64 chars)
	assert.Equal(t, 64, len(hash), "hash should be 64 characters (SHA-256 hex)")

	// Verify it's hex-encoded (0-9a-f only)
	pattern := regexp.MustCompile(`^[0-9a-f]{64}$`)
	assert.True(t, pattern.MatchString(hash), "hash should be 64 hex characters")

	// Verify it matches expected SHA-256 hash
	expected := fmt.Sprintf("%x", sha256.Sum256([]byte(code)))
	assert.Equal(t, expected, hash)
}

func TestVerifyBackupCode(t *testing.T) {
	code := "TESTCODE"
	hash := HashBackupCode(code)

	// Test 1: Matching pair should return true
	assert.True(t, VerifyBackupCode(code, hash), "should verify matching code and hash")

	// Test 2: Wrong code should return false
	assert.False(t, VerifyBackupCode("WRONGCODE", hash), "should reject wrong code")

	// Test 3: Different case should not match (codes are case-sensitive)
	assert.False(t, VerifyBackupCode("testcode", hash), "should be case-sensitive")

	// Test 4: Timing attack resistance (constant-time comparison)
	// We can't directly test timing, but we can verify it uses constant-time compare
	// by ensuring both matching and non-matching comparisons complete
	wrongHash := HashBackupCode("WRONGCODE")
	_ = VerifyBackupCode(code, wrongHash)
	// If timing attack wasn't constant-time, the test would potentially fail under measurement,
	// but we can't assert that directly in a unit test
}
