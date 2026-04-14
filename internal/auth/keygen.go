package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadOrGenerateRSAKey loads an RSA private key from the PEM string in ARGUS_JWT_PRIVATE_KEY_PEM
// or generates an ephemeral 2048-bit key if the env var is absent.
// Returns the private key and matching public key.
func LoadOrGenerateRSAKey() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	pemStr := os.Getenv("ARGUS_JWT_PRIVATE_KEY_PEM")
	if pemStr != "" {
		block, _ := pem.Decode([]byte(pemStr))
		if block == nil {
			return nil, nil, fmt.Errorf("ARGUS_JWT_PRIVATE_KEY_PEM: not a valid PEM block")
		}
		priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			// Try PKCS1
			pk1, err2 := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err2 != nil {
				return nil, nil, fmt.Errorf("failed to parse RSA private key: %w", err)
			}
			return pk1, &pk1.PublicKey, nil
		}
		rsaPriv, ok := priv.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, fmt.Errorf("ARGUS_JWT_PRIVATE_KEY_PEM: not an RSA key")
		}
		return rsaPriv, &rsaPriv.PublicKey, nil
	}

	// Generate ephemeral key (dev/test mode)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}
	return priv, &priv.PublicKey, nil
}
