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

	if len(verifier) != 43 {
		t.Errorf("expected verifier length 43, got %d", len(verifier))
	}

	if len(challenge) != 43 {
		t.Errorf("expected challenge length 43, got %d", len(challenge))
	}

	h := sha256.Sum256([]byte(verifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(h[:])
	if challenge != expectedChallenge {
		t.Errorf("expected challenge %s, got %s", expectedChallenge, challenge)
	}
}
