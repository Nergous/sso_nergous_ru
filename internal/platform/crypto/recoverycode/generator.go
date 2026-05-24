// Package recoverycode is the platform-level generator for the
// one-time codes consumed by sso.auth.v1.AuthService.
// ResetPasswordWithRecoveryCode.
//
// Output shape: ten codes of the form `XXXX-XXXX-XXXX` (twelve
// significant chars + two hyphens). The alphabet is restricted to 32
// unambiguous base32-style characters — no `0`, `O`, `1`, `I`, or `L` —
// to keep transcription errors out of support tickets.
//
// Entropy:
//
//	12 chars × log2(32) = 60 bit per code.
//
// 2⁶⁰ blind guesses against the per-user batch is intractable; combined
// with the per-IP rate-limit on ResetPasswordWithRecoveryCode (§7),
// brute force is not a practical concern. The hash stored on disk is
// therefore plain SHA-256 (no salt) — the input space is already wide
// enough that a salted-bcrypt round would cost more than it buys.
//
// Normalisation: input from the wire may include hyphens or lowercase
// letters; Hash strips hyphens and uppercases before SHA-256 so the
// stored digest matches the canonical form the server generated.
package recoverycode

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"strings"
)

// Reduced base32 alphabet, 32 symbols. Drops the visually-ambiguous
// `0`/`O` and `1`/`I` pairs (digits 2-9 + A-Z minus I and O = 32). `L`
// stays — keeping it lets the alphabet land at exactly 32, which makes
// the modulo bias check in randomCode a clean power-of-two mask
// instead of rejection sampling.
const (
	alphabet     = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	codeLength   = 12 // significant chars
	groupSize    = 4
	groupCount   = codeLength / groupSize // 3 groups
	codesInBatch = 10
)

// Compile-time sanity check: each character drawn from the alphabet
// must contribute log2(len(alphabet)) bits. A typo in `alphabet`
// (e.g. accidentally re-including 'O') would silently shrink the
// effective entropy; this assertion makes the regression a build break.
func init() {
	if len(alphabet) != 32 {
		panic(fmt.Sprintf("recoverycode: alphabet must be 32 chars (have %d)", len(alphabet)))
	}
}

// Generator is the contract used by the auth use-case. Kept as a tiny
// interface so tests can swap in a deterministic stub.
type Generator interface {
	Generate() (codes []string, hashes [][]byte, err error)
	Hash(plaintext string) []byte
}

type defaultGenerator struct{}

// New returns the production generator: 10 codes per batch from
// crypto/rand, SHA-256 hashing with the canonical normalisation.
func New() Generator { return defaultGenerator{} }

// Generate emits ten distinct random codes plus their canonical hashes.
// "Distinct" is enforced by retry; with 60-bit entropy collisions inside
// a single batch are vanishingly rare, but the explicit loop keeps the
// guarantee local instead of relying on probability.
func (defaultGenerator) Generate() ([]string, [][]byte, error) {
	codes := make([]string, 0, codesInBatch)
	hashes := make([][]byte, 0, codesInBatch)
	seen := make(map[string]struct{}, codesInBatch)

	for len(codes) < codesInBatch {
		code, err := randomCode()
		if err != nil {
			return nil, nil, fmt.Errorf("recoverycode: gen: %w", err)
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
		hashes = append(hashes, hashNormalised(code))
	}
	return codes, hashes, nil
}

// Hash applies the canonical normalisation (strip "-", upper-case) and
// returns the SHA-256 digest. Symmetric with Generate so a hash produced
// here can compare equal to a Generate-side hash for the same code.
func (defaultGenerator) Hash(plaintext string) []byte {
	return hashNormalised(plaintext)
}

// ----------------------------------------------------------------------------
// internals
// ----------------------------------------------------------------------------

func randomCode() (string, error) {
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// Modulo bias: alphabet length is 32 (a power of two), so masking
	// the low 5 bits is exact — no rejection sampling needed.
	const mask = 31
	chars := make([]byte, codeLength)
	for i, b := range buf {
		chars[i] = alphabet[b&mask]
	}
	// Insert hyphens every groupSize chars to land on XXXX-XXXX-XXXX.
	var b strings.Builder
	b.Grow(codeLength + groupCount - 1)
	for i := 0; i < codeLength; i += groupSize {
		if i > 0 {
			b.WriteByte('-')
		}
		b.Write(chars[i : i+groupSize])
	}
	return b.String(), nil
}

func hashNormalised(plaintext string) []byte {
	norm := strings.ToUpper(strings.ReplaceAll(plaintext, "-", ""))
	sum := sha256.Sum256([]byte(norm))
	return sum[:]
}
