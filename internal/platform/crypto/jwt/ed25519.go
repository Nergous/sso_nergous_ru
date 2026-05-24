package jwt

import (
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ed25519Signer struct {
	priv      ed25519.PrivateKey
	issuer    string
	accessTTL time.Duration
}

type ed25519Verifier struct {
	pub            ed25519.PublicKey
	expectedIssuer string
}

type tokenClaims struct {
	jwt.RegisteredClaims
	SubjectType SubjectType `json:"subject_type"`
	SessionID   string      `json:"sid,omitempty"`
}

func NewEd25519Signer(priv ed25519.PrivateKey, issuer string, accessTTL time.Duration) Signer {
	return &ed25519Signer{priv: priv, issuer: issuer, accessTTL: accessTTL}
}

func NewEd25519Verifier(pub ed25519.PublicKey, expectedIssuer string) Verifier {
	return &ed25519Verifier{pub: pub, expectedIssuer: expectedIssuer}
}

func (s *ed25519Signer) Sign(c Claims) (string, error) {
	now := time.Now().UTC()
	tokenClaim := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   c.Subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			ID:        c.JTI,
		},
		SubjectType: c.SubjectType,
		SessionID:   c.SessionID,
	}

	sign := jwt.SigningMethodEdDSA

	return jwt.NewWithClaims(sign, tokenClaim).SignedString(s.priv)
}

func (v *ed25519Verifier) Verify(token string) (Claims, error) {
	tokenClaim := tokenClaims{}
	_, err := jwt.ParseWithClaims(
		token,
		&tokenClaim,
		func(t *jwt.Token) (interface{}, error) {
			// check if method EdDSA
			if t.Method.Alg() != "EdDSA" {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Method.Alg())
			}
			return v.pub, nil
		},
		jwt.WithIssuer(v.expectedIssuer),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("jwt: %w", err)
	}

	return tokenClaimsToClaims(tokenClaim), nil
}

func tokenClaimsToClaims(c tokenClaims) Claims {
	return Claims{
		Issuer:      c.Issuer,
		Subject:     c.Subject,
		SubjectType: c.SubjectType,
		SessionID:   c.SessionID,
		IssuedAt:    c.IssuedAt.Time,
		ExpiresAt:   c.ExpiresAt.Time,
		JTI:         c.ID,
	}
}
