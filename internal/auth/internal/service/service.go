package service

import (
	"log/slog"
	"time"

	"sso/internal/app"
	"sso/internal/audit"
	"sso/internal/audit/auditx"
	"sso/internal/identity"
	"sso/internal/platform/crypto/jwt"
	"sso/internal/platform/crypto/randtoken"
	recoverygen "sso/internal/platform/crypto/recoverycode"
	"sso/internal/recoverycode"
	"sso/internal/serviceaccount"
	"sso/internal/session"
)

type Service struct {
	log             *slog.Logger
	users           identity.Repository
	sessions        session.Repository
	serviceAccounts serviceaccount.Repository
	apps            app.Repository
	recoveryCodes   recoverycode.Repository
	signer          jwt.Signer
	verifier        jwt.Verifier
	tokenGen        randtoken.Generator
	recoveryGen     recoverygen.Generator
	now             func() time.Time

	// accessTTL duplicates what the signer already knows; kept here so
	// Login/Refresh can return AccessExpiresAt to the client without
	// reaching into the signer.
	accessTTL          time.Duration
	refreshTTL         time.Duration // absolute hard-cap
	refreshRotationTTL time.Duration // sliding window

	bcryptCost int

	auditor auditx.Auditor
}

func NewService(
	log *slog.Logger,
	users identity.Repository,
	sessions session.Repository,
	serviceAccounts serviceaccount.Repository,
	apps app.Repository,
	recoveryCodes recoverycode.Repository,
	signer jwt.Signer,
	verifier jwt.Verifier,
	tokenGen randtoken.Generator,
	recoveryGen recoverygen.Generator,
	now func() time.Time,
	accessTTL, refreshTTL, refreshRotationTTL time.Duration,
	bcryptCost int,
	emitter audit.Emitter,
) *Service {
	return &Service{
		log:                log,
		users:              users,
		sessions:           sessions,
		serviceAccounts:    serviceAccounts,
		apps:               apps,
		recoveryCodes:      recoveryCodes,
		signer:             signer,
		verifier:           verifier,
		tokenGen:           tokenGen,
		recoveryGen:        recoveryGen,
		now:                now,
		accessTTL:          accessTTL,
		refreshTTL:         refreshTTL,
		refreshRotationTTL: refreshRotationTTL,
		bcryptCost:         bcryptCost,
		auditor:            auditx.New(log, emitter),
	}
}
