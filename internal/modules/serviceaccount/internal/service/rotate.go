package service

import (
	"context"
	"fmt"
	"time"

	"sso/internal/modules/audit"
	serviceAccount "sso/internal/modules/serviceaccount/internal/domain"
	"sso/internal/kernel/actor"
	"sso/internal/modules/audit/auditx"
)

type RotateCredentialsInput struct {
	ServiceAccountID string
	ExpectedEtag     string
}

type RotateCredentialsOutput struct {
	Account      *serviceAccount.ServiceAccount
	ClientSecret string
	IssuedAt     time.Time
}

// RotateCredentials issues a fresh client_secret, invalidating the
// previous one. Requires an etag matching the current ServiceAccount
// (per proto contract) — a wildcard "*" is rejected so racing rotations
// can't silently overwrite each other.
func (s *Service) RotateCredentials(ctx context.Context, in RotateCredentialsInput) (RotateCredentialsOutput, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return RotateCredentialsOutput{}, fmt.Errorf("rotate credentials: %w", err)
	}
	id, err := serviceAccount.ParseServiceAccountID(in.ServiceAccountID)
	if err != nil {
		return RotateCredentialsOutput{}, fmt.Errorf("rotate credentials: %w", err)
	}
	expectedEtag, err := auditx.ParseExpectedEtag(in.ExpectedEtag, true /*required*/)
	if err != nil {
		return RotateCredentialsOutput{}, fmt.Errorf("rotate credentials: %w", err)
	}

	aud := audit.BaseFromActor(a, audit.EventTypeServiceAccountRotateCredentials)
	aud.SubjectType = audit.SubjectTypeServiceAccount
	aud.SubjectID = id.String()

	sa, err := s.repo.GetByID(ctx, id)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return RotateCredentialsOutput{}, fmt.Errorf("rotate credentials: %w", err)
	}

	plaintext, hash, err := generateSecret()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return RotateCredentialsOutput{}, err
	}

	now := s.now().UTC()
	sa.RotateSecret(hash, now)

	if err := s.repo.Update(ctx, sa, expectedEtag); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return RotateCredentialsOutput{}, err
	}

	s.auditor.Success(ctx, aud)

	return RotateCredentialsOutput{
		Account:      sa,
		ClientSecret: plaintext,
		IssuedAt:     now,
	}, nil
}
