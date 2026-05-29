package service

import (
	"context"

	"sso/internal/modules/app/internal/domain"
	"sso/internal/modules/audit"
	"sso/internal/modules/audit/auditx"
	"sso/internal/kernel/actor"
	"sso/internal/kernel/etag"
	"sso/internal/kernel/validation"
)

// PermanentlyDeleteAppInput is the parsed PermanentlyDeleteAppRequest.
// ExpectedEtag is required (proto: min_len = 1) and the wildcard "*" is
// rejected by the wire-level CEL rule before we reach this code; we still
// re-check defensively.
type PermanentlyDeleteAppInput struct {
	AppID        string
	ExpectedEtag string
}

// PermanentlyDeleteApp hard-deletes the app. Requires a matching etag
// (concurrency-safe confirmation that the caller observed the current
// state). Unlike identity, app has no DELETED state — there is no
// soft-delete gate to clear.
//
// Cascade (per proto): roles for this app and any role assignments are
// deleted with the app. That cross-aggregate cleanup is a future concern;
// for now we delete only the apps row. A FOREIGN KEY ... ON DELETE CASCADE
// in the roles/assignments tables would handle it naturally once those
// tables exist.
//
// Errors:
//
//	ErrAppNotFound  — no row
//	ErrEtagMismatch — etag does not match
//	ValidationError — missing or wildcard etag
func (s *Service) PermanentlyDeleteApp(ctx context.Context, in PermanentlyDeleteAppInput) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	id, err := domain.ParseAppID(in.AppID)
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

	aud := audit.BaseFromActor(a, audit.EventTypeAppPermanentlyDeleteApp)
	aud.SubjectType = audit.SubjectTypeApp
	aud.SubjectID = id.String()
	aud.AppID = id.String()

	target, err := s.repo.GetByID(ctx, id)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}
	if target.Etag() != expectedEtag {
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
