package service

import (
	"context"
	"fmt"

	"sso/internal/modules/audit"
	serviceAccount "sso/internal/modules/serviceaccount/internal/domain"
	"sso/internal/kernel/actor"
	"sso/internal/kernel/etag"
	"sso/internal/kernel/validation"
	"sso/internal/modules/audit/auditx"
)

type PermanentlyDeleteServiceAccountInput struct {
	ServiceAccountID string
	ExpectedEtag     string
}

func (s *Service) PermanentlyDeleteServiceAccount(ctx context.Context, in PermanentlyDeleteServiceAccountInput) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return fmt.Errorf("permanently delete service_account: %w", err)
	}
	id, err := serviceAccount.ParseServiceAccountID(in.ServiceAccountID)
	if err != nil {
		return fmt.Errorf("permanently delete service_account: %w", err)
	}
	if in.ExpectedEtag == "" {
		return &validation.Error{Field: "etag", Reason: "must be provided"}
	}
	if in.ExpectedEtag == auditx.EtagWildcard {
		return &validation.Error{Field: "etag", Reason: "wildcard is not permitted for permanent delete"}
	}
	expectedEtag, err := etag.Parse(in.ExpectedEtag)
	if err != nil {
		return fmt.Errorf("permanently delete service_account: %w", err)
	}

	aud := audit.BaseFromActor(a, audit.EventTypeServiceAccountPermanentlyDeleteServiceAccount)
	aud.SubjectType = audit.SubjectTypeServiceAccount
	aud.SubjectID = id.String()

	sa, err := s.repo.GetByID(ctx, id)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}
	if sa.Etag() != expectedEtag {
		s.auditor.Fail(ctx, aud, audit.ReasonEtagMismatch)
		return serviceAccount.ErrEtagMismatch
	}

	if err := s.repo.Delete(ctx, id, expectedEtag); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	s.auditor.Success(ctx, aud)
	return nil
}
