package service

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// secretBytes is the entropy of a freshly minted client_secret. 32 bytes
// (256 bits) yields a 43-char base64url string after encoding — plenty
// for OAuth2-style client_credentials grants and short enough to fit in
// a single env var without line-wrapping.
const secretBytes = 32

// bcryptCost balances throughput on AuthenticateServiceAccount against
// brute-force resistance. 12 ≈ ~150 ms per hash on modest hardware,
// which is comfortably above the ~10 ms ceiling for a perceptibly fast
// API call but slow enough to deter offline cracking.
const bcryptCost = 12

// generateSecret returns a freshly minted plaintext secret and its
// bcrypt hash. The plaintext leaves the system exactly once (in the
// CreateServiceAccount or RotateCredentials response) and is never
// persisted in the clear; the hash is what hits the database.
func generateSecret() (plaintext string, hash []byte, err error) {
	buf := make([]byte, secretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate secret: read random: %w", err)
	}
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	hash, err = bcrypt.GenerateFromPassword([]byte(plaintext), bcryptCost)
	if err != nil {
		return "", nil, fmt.Errorf("generate secret: hash: %w", err)
	}
	return plaintext, hash, nil
}
