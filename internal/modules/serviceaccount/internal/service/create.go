package service

import (
	"context"
	"fmt"
	"time"

	"sso/internal/modules/audit"
	serviceAccount "sso/internal/modules/serviceaccount/internal/domain"
	"sso/internal/kernel/actor"
)

type CreateServiceAccountInput struct {
	Name        string
	Description string
}

// CreateServiceAccountOutput pairs the persisted account with the
// freshly minted plaintext secret. The plaintext is exposed exactly
// once (see proto: ServiceAccountCredentials.client_secret is
// debug_redact and shown only at Create / Rotate); callers must hand
// it to the consumer immediately.
type CreateServiceAccountOutput struct {
	Account      *serviceAccount.ServiceAccount
	ClientSecret string
	IssuedAt     time.Time
}

func (s *Service) CreateServiceAccount(ctx context.Context, in CreateServiceAccountInput) (CreateServiceAccountOutput, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return CreateServiceAccountOutput{}, fmt.Errorf("create service account: %w", err)
	}

	id, err := serviceAccount.NewServiceAccountID()
	if err != nil {
		return CreateServiceAccountOutput{}, err
	}

	plaintext, hash, err := generateSecret()
	if err != nil {
		return CreateServiceAccountOutput{}, err
	}

	now := s.now().UTC()
	sa := serviceAccount.NewServiceAccount(serviceAccount.NewServiceAccountParams{
		ID:          id,
		Name:        in.Name,
		Description: in.Description,
		SecretHash:  hash,
		Now:         now,
	})

	aud := audit.BaseFromActor(a, audit.EventTypeServiceAccountCreateServiceAccount)
	aud.SubjectType = audit.SubjectTypeServiceAccount
	aud.SubjectID = sa.ID().String()

	if err := s.repo.Create(ctx, sa); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return CreateServiceAccountOutput{}, fmt.Errorf("create service account: %w", err)
	}

	s.auditor.Success(ctx, aud)

	return CreateServiceAccountOutput{
		Account:      sa,
		ClientSecret: plaintext,
		IssuedAt:     now,
	}, nil
}
