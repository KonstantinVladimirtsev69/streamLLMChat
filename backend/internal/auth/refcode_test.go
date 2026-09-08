package auth_test

import (
	"strings"
	"testing"

	"backend/internal/auth"
)

func TestRefCodeGenerator(t *testing.T) {
	t.Parallel()

	gen := auth.NewRefCodeGenerator(8)

	t.Run("length and charset verification", func(t *testing.T) {
		t.Parallel()
		code := gen.GenerateRefCode()
		if len(code) != 8 {
			t.Fatalf("expected code length 8, got %d ('%s')", len(code), code)
		}

		for _, ch := range code {
			if !strings.ContainsRune(auth.Base58Alphabet, ch) {
				t.Errorf("character %c is not in Base58Alphabet", ch)
			}
		}
	})

	t.Run("uniqueness over 5000 iterations", func(t *testing.T) {
		t.Parallel()
		seen := make(map[string]struct{}, 5000)
		for i := 0; i < 5000; i++ {
			code := gen.GenerateRefCode()
			if _, exists := seen[code]; exists {
				t.Fatalf("collision detected at iteration %d: code %s", i, code)
			}
			seen[code] = struct{}{}
		}
	})
}
