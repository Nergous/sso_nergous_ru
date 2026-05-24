package service

import (
	"context"

	"sso/internal/audit"
	"sso/internal/audit/auditx"
	"sso/internal/identity/internal/domain"
	"sso/internal/kernel/actor"
	"sso/internal/kernel/etag"
	"sso/internal/kernel/validation"
)

// ----------------------------------------------------------------------------
// SoftDeleteUser
// ----------------------------------------------------------------------------

// SoftDeleteUserInput is the parsed SoftDeleteUserRequest.
type SoftDeleteUserInput struct {
	UserID       string
	AllowMissing bool
}

// SoftDeleteUser marks the target as DELETED. Idempotent on an
// already-deleted user.
func (s *Service) SoftDeleteUser(ctx context.Context, in SoftDeleteUserInput) error {
	return s.lifecycleEmit(ctx, in.UserID, in.AllowMissing, audit.EventTypeIdentitySoftDeleteUser, func(u *domain.User) error {
		u.SoftDelete(s.now().UTC())
		return nil
	})
}

// ----------------------------------------------------------------------------
// PermanentlyDeleteUser
// ----------------------------------------------------------------------------

// PermanentlyDeleteUserInput is the parsed PermanentlyDeleteUserRequest.
// ExpectedEtag is required (proto: min_len = 1) and the wildcard "*" is
// rejected by the wire-level CEL rule before we reach this code; we still
// re-check defensively.
type PermanentlyDeleteUserInput struct {
	UserID       string
	ExpectedEtag string
}

// PermanentlyDeleteUser hard-deletes the user. Requires the user to be in
// DELETED status (lifecycle gate) and a matching etag (concurrency-safe
// confirmation that the caller observed the current state).
//
// Errors:
//
//	ErrUserNotFound       — no row
//	ErrUserNotDeleted     — status != DELETED
//	ErrEtagMismatch       — etag does not match
//	ValidationError       — missing or wildcard etag
func (s *Service) PermanentlyDeleteUser(ctx context.Context, in PermanentlyDeleteUserInput) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	id, err := domain.ParseUserID(in.UserID)
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

	aud := audit.BaseFromActor(a, audit.EventTypeIdentityPermanentlyDeleteUser)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = id.String()

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}
	if user.Status() != domain.UserStatusDeleted {
		s.auditor.Fail(ctx, aud, audit.ReasonUserNotDeleted)
		return domain.ErrUserNotDeleted
	}
	if user.Etag() != expectedEtag {
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
