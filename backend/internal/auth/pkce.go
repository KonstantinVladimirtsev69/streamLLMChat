package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// GeneratePKCE creates a high-entropy cryptographically secure code_verifier
// and calculates its corresponding code_challenge using the S256 method (RFC 7636).
func GeneratePKCE() (verifier string, challenge string, err error) {
	var randomBytes [32]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes for pkce: %w", err)
	}

	verifier = base64.RawURLEncoding.EncodeToString(randomBytes[:])
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}
