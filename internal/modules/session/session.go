// Package session is the public API of the session bounded context
// (refresh-token-backed login sessions issued by AuthService).
//
// External callers interact with the module through these surfaces:
//
//	session.New(Deps)    wires the module (module.go)
//	session.Repository   persistence contract (consumed by auth)
//
// The type aliases below let other modules program against
// session.Session / session.SessionID etc. instead of importing the
// internal domain package directly. The internal package stays
// unreachable thanks to Go's "internal/" protection.
package session

import "sso/internal/modules/session/internal/domain"

type (
	Session              = domain.Session
	SessionID            = domain.SessionID
	UserID               = domain.UserID
	NewSessionParams     = domain.NewSessionParams
	RestoreSessionParams = domain.RestoreSessionParams
	Repository           = domain.Repository
)

// ID constructors / parsers re-exported as package-level variables.
var (
	NewSessionID   = domain.NewSessionID
	ParseSessionID = domain.ParseSessionID
	ParseUserID    = domain.ParseUserID
	NewSession     = domain.NewSession
	RestoreSession = domain.RestoreSession
)

// Sentinel errors. External consumers test for them with errors.Is.
var (
	ErrSessionNotFound    = domain.ErrSessionNotFound
	ErrSessionRevoked     = domain.ErrSessionRevoked
	ErrSessionExpired     = domain.ErrSessionExpired
	ErrSessionNotOwned    = domain.ErrSessionNotOwned
	ErrRefreshTokenReused = domain.ErrRefreshTokenReused
)
