package auth_test

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"backend/internal/auth"
)

func TestGeneratePKCE(t *testing.T) {
	t.Parallel()

	verifier, challenge, err := auth.GeneratePKCE()
	if err != nil {
		t.Fatalf("unexpected error generating PKCE: %v", err)
	}

	if len(verifier) < 43 || len(verifier) > 128 {
		t.Errorf("expected verifier length between 43 and 128, got %d", len(verifier))
	}

	if len(challenge) == 0 {
		t.Errorf("challenge cannot be empty")
	}

	// Verify S256 derivation: challenge must be base64url(sha256(verifier))
	h := sha256.Sum256([]byte(verifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(h[:])
	if challenge != expectedChallenge {
		t.Errorf("challenge mismatch: expected %s, got %s", expectedChallenge, challenge)
	}

	// Ensure uniqueness across multiple calls
	verifier2, challenge2, err := auth.GeneratePKCE()
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if verifier == verifier2 || challenge == challenge2 {
		t.Errorf("expected unique PKCE pairs, got duplicate")
	}
}
