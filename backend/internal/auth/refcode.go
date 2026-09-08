package auth

import (
	"crypto/rand"
	"math/big"
)

// Base58Alphabet contains 58 characters excluding visually ambiguous chars (0, O, o, 1, l, I).
const Base58Alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz"

// RefCodeGenerator defines interface for creating unique referral codes.
type RefCodeGenerator interface {
	GenerateRefCode() string
}

type refCodeGenerator struct {
	length int
}

// NewRefCodeGenerator creates a new RefCodeGenerator generating codes of specified length (default 8).
func NewRefCodeGenerator(length int) RefCodeGenerator {
	if length <= 0 {
		length = 8
	}
	return &refCodeGenerator{
		length: length,
	}
}

// GenerateRefCode generates a cryptographically random referral code.
func (g *refCodeGenerator) GenerateRefCode() string {
	alphabetLen := big.NewInt(int64(len(Base58Alphabet)))
	b := make([]byte, g.length)

	for i := 0; i < g.length; i++ {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			// Fallback in catastrophic failure: standard modulo on random byte
			var randByte [1]byte
			_, _ = rand.Read(randByte[:])
			b[i] = Base58Alphabet[int(randByte[0])%len(Base58Alphabet)]
			continue
		}
		b[i] = Base58Alphabet[idx.Int64()]
	}

	return string(b)
}
