package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret creates a new TOTP secret for the given account email and issuer.
// Returns a *otp.Key which can be converted to a QR code or displayed as a manual entry string.
func GenerateTOTPSecret(accountEmail, issuer string) (*otp.Key, error) {
	opts := totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountEmail,
	}
	return totp.Generate(opts)
}

// ValidateTOTPCode checks if a given TOTP code is valid for the provided base32-encoded secret.
// Uses the standard 30-second time window and 6-digit codes.
func ValidateTOTPCode(base32Secret, code string) bool {
	return totp.Validate(code, base32Secret)
}

// GenerateBackupCodes generates n unique backup codes.
// Each code is 8 characters from the alphabet [A-Z0-9].
// Regenerates any duplicates found within the batch.
func GenerateBackupCodes(n int) ([]string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const codeLen = 8

	codes := make([]string, 0, n)
	seen := make(map[string]bool)

	for len(codes) < n {
		code := make([]byte, codeLen)
		b := make([]byte, codeLen)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("failed to read random bytes: %w", err)
		}
		for i := 0; i < codeLen; i++ {
			// Use modulo to select from alphabet (36 chars)
			code[i] = alphabet[int(b[i])%len(alphabet)]
		}

		codeStr := string(code)
		if !seen[codeStr] {
			codes = append(codes, codeStr)
			seen[codeStr] = true
		}
		// If duplicate, loop will regenerate
	}

	return codes, nil
}

// HashBackupCode returns the SHA-256 hash of a backup code as a lowercase hex string.
// Used for storing backup codes in the database without storing plaintext.
func HashBackupCode(code string) string {
	hash := sha256.Sum256([]byte(code))
	return fmt.Sprintf("%x", hash)
}

// VerifyBackupCode compares a plaintext backup code against its stored hash.
// Uses constant-time comparison to prevent timing attacks.
func VerifyBackupCode(code, hash string) bool {
	computed := HashBackupCode(code)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) == 1
}
