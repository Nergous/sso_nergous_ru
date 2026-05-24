package service

import (
	"context"
	"errors"

	"sso/internal/audit"
	serviceAccount "sso/internal/serviceaccount/internal/domain"
	"sso/internal/kernel/actor"
	"sso/internal/kernel/validation"
	"sso/internal/audit/auditx"
)

type UpdateServiceAccountInput struct {
	ServiceAccountID string
	MaskPaths        []string
	ExpectedEtag     string

	Name        string
	Description string
}

func (s *Service) UpdateServiceAccount(ctx context.Context, in UpdateServiceAccountInput) (*serviceAccount.ServiceAccount, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := serviceAccount.ParseServiceAccountID(in.ServiceAccountID)
	if err != nil {
		return nil, err
	}

	expectedEtag, err := auditx.ParseExpectedEtag(in.ExpectedEtag, true /*required*/)
	if err != nil {
		return nil, err
	}

	if len(in.MaskPaths) == 0 {
		return nil, &validation.Error{Field: "update_mask", Reason: "must list at least one field"}
	}

	patch, err := buildPatch(in)
	if err != nil {
		return nil, err
	}
	if patch.IsEmpty() {
		return nil, &validation.Error{Field: "update_mask", Reason: "no applicable fields"}
	}

	aud := audit.BaseFromActor(a, audit.EventTypeServiceAccountUpdateServiceAccount)
	aud.SubjectType = audit.SubjectTypeServiceAccount
	aud.SubjectID = id.String()

	sa, err := s.repo.GetByID(ctx, id)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}
	sa.ApplyPatch(patch, s.now().UTC())
	if err := s.repo.Update(ctx, sa, expectedEtag); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}

	s.auditor.Success(ctx, aud)
	return sa, nil
}

func buildPatch(in UpdateServiceAccountInput) (serviceAccount.ServiceAccountPatch, error) {
	var p serviceAccount.ServiceAccountPatch
	for _, path := range in.MaskPaths {
		switch path {
		case "name":
			v := in.Name
			p.Name = &v
		case "description":
			v := in.Description
			p.Description = &v
		default:
			return serviceAccount.ServiceAccountPatch{}, &validation.Error{
				Field:  "update_mask",
				Reason: "unknown field path: " + path,
			}
		}
	}
	return p, nil
}

// ----------------------------------------------------------------------------
// Lifecycle
// ----------------------------------------------------------------------------

type DisableServiceAccountInput struct {
	ServiceAccountID string
	AllowMissing     bool
}

func (s *Service) DisableServiceAccount(ctx context.Context, in DisableServiceAccountInput) error {
	return s.lifecycleEmit(ctx,
		in.ServiceAccountID,
		in.AllowMissing,
		audit.EventTypeServiceAccountDisableServiceAccount,
		func(sa *serviceAccount.ServiceAccount) { sa.Disable(s.now().UTC()) },
	)
}

type EnableServiceAccountInput struct {
	ServiceAccountID string
	AllowMissing     bool
}

func (s *Service) EnableServiceAccount(ctx context.Context, in EnableServiceAccountInput) error {
	return s.lifecycleEmit(ctx,
		in.ServiceAccountID,
		in.AllowMissing,
		audit.EventTypeServiceAccountEnableServiceAccount,
		func(sa *serviceAccount.ServiceAccount) { sa.Enable(s.now().UTC()) },
	)
}

// lifecycleEmit is the Disable/Enable scaffold with audit emission:
// load → mutate → save → emit. The transition closure is a pure
// state-machine call on the aggregate (no I/O), so the only failure
// modes we audit come from the repo round-trips.
//
// AllowMissing + NotFound returns nil without an emit: nothing happened,
// nothing to record. All other paths emit exactly once.
func (s *Service) lifecycleEmit(
	ctx context.Context,
	rawID string,
	allowMissing bool,
	eventType audit.EventType,
	mutate func(*serviceAccount.ServiceAccount),
) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	id, err := serviceAccount.ParseServiceAccountID(rawID)
	if err != nil {
		return err
	}

	aud := audit.BaseFromActor(a, eventType)
	aud.SubjectType = audit.SubjectTypeServiceAccount
	aud.SubjectID = id.String()

	sa, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if allowMissing && errors.Is(err, serviceAccount.ErrServiceAccountNotFound) {
			return nil
		}
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	mutate(sa)
	if err := s.repo.Update(ctx, sa, ""); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	s.auditor.Success(ctx, aud)
	return nil
}
