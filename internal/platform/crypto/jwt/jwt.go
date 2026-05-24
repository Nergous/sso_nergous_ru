package jwt

import "time"

type SubjectType string

const (
	SubjectTypeUser           SubjectType = "user"
	SubjectTypeServiceAccount SubjectType = "service_account"
)

func (s SubjectType) String() string { return string(s) }

type Claims struct {
	Issuer      string
	Subject     string
	SubjectType SubjectType
	SessionID   string
	IssuedAt    time.Time
	ExpiresAt   time.Time
	JTI         string
}

type Signer interface {
	Sign(Claims) (string, error)
}

type Verifier interface {
	Verify(string) (Claims, error)
}

func (c *Claims) IsExpired(now time.Time) bool {
	return !now.Before(c.ExpiresAt)
}
