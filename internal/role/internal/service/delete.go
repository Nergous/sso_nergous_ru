package service

import (
	"context"

	"sso/internal/audit"
	"sso/internal/audit/auditx"
	"sso/internal/kernel/actor"
	"sso/internal/kernel/etag"
	"sso/internal/kernel/validation"
	"sso/internal/role/internal/domain"
)

// PermanentlyDeleteRoleInput is the parsed PermanentlyDeleteRoleRequest.
// The earlier `allow_cascade` flag was retired from the proto contract;
// when AccessService (where assignments live) lands, the cascade
// behaviour will be unconditional (assignments wiped in the same tx as
// the role row).
type PermanentlyDeleteRoleInput struct {
	RoleID       string
	ExpectedEtag string
}

// PermanentlyDeleteRole hard-deletes the role. role_permissions cascades
// automatically via FOREIGN KEY ON DELETE CASCADE in the schema.
//
// Errors:
//
//	ValidationError       — missing or wildcard etag
//	ErrRoleNotFound       — no row
//	ErrEtagMismatch       — etag does not match
func (s *Service) PermanentlyDeleteRole(ctx context.Context, in PermanentlyDeleteRoleInput) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	id, err := domain.ParseRoleID(in.RoleID)
	if err != nil {
		return err
	}
	if in.ExpectedEtag == "" {
		return &validation.Error{Field: "etag", Reason: "must be provided"}
	}
	if in.ExpectedEtag == auditx.EtagWildcard {
		return &validation.Error{Field: "etag", Reason: "wildcard is not permitted for permanent delete"}
	}
	expectedEtag, err := etag.Parse(in.ExpectedEtag)
	if err != nil {
		return err
	}

	aud := audit.BaseFromActor(a, audit.EventTypeRolePermanentlyDeleteRole)
	aud.SubjectType = audit.SubjectTypeRole
	aud.SubjectID = id.String()

	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}
	if r.Etag() != expectedEtag {
		s.auditor.Fail(ctx, aud, audit.ReasonEtagMismatch)
		return domain.ErrEtagMismatch
	}

	if err := s.repo.Delete(ctx, id, expectedEtag); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	s.auditor.Success(ctx, aud)
	return nil
}
