package randtoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

type Generator struct{}

func (Generator) Generate() (plaintext string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate secret: read random: %w", err)
	}
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(plaintext))
	hash = sum[:]

	return plaintext, hash, nil
}

func (Generator) Hash(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}
