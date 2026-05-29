package service

import (
	"context"

	access "sso/internal/modules/access/internal/domain"
	appdom "sso/internal/modules/app"
	"sso/internal/modules/audit"
	"sso/internal/modules/identity"
	"sso/internal/kernel/actor"
)

// ----------------------------------------------------------------------------
// RemoveRoleFromUser
// ----------------------------------------------------------------------------

type RemoveRoleFromUserInput struct {
	UserID string
	RoleID string
}

// RemoveRoleFromUser is idempotent: a missing assignment returns nil,
// not ErrAssignmentNotFound. Per proto: "removing a non-existent
// assignment succeeds with no effect". User / role existence is still
// checked so callers can't silently target garbage UUIDs.
func (s *Service) RemoveRoleFromUser(ctx context.Context, in RemoveRoleFromUserInput) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	uid, err := access.ParseUserID(in.UserID)
	if err != nil {
		return err
	}
	rid, err := access.ParseRoleID(in.RoleID)
	if err != nil {
		return err
	}

	aud := audit.BaseFromActor(a, audit.EventTypeAccessRemoveRoleFromUser)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = uid.String()

	if err := s.requireUserExists(ctx, identity.UserID(uid)); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}
	// Role must exist (any status). Disabled roles are still removable.
	r, err := s.loadAnyRole(ctx, rid)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}
	aud.AppID = r.AppID().String()

	if _, err := s.repo.Delete(ctx, uid, rid); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	s.auditor.Success(ctx, aud)
	return nil
}

// ----------------------------------------------------------------------------
// BulkRemoveRoles
// ----------------------------------------------------------------------------

type BulkRemoveRolesInput struct {
	UserID  string
	AppID   string
	RoleIDs []string
}

func (s *Service) BulkRemoveRoles(ctx context.Context, in BulkRemoveRolesInput) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	uid, aid, ridsTyped, _, err := s.parseBulkInput(in.UserID, in.AppID, in.RoleIDs, "")
	if err != nil {
		return err
	}

	aud := audit.BaseFromActor(a, audit.EventTypeAccessBulkRemoveRoles)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = uid.String()
	aud.AppID = aid.String()

	if err := s.requireAppExists(ctx, appdom.AppID(aid)); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}
	if err := s.requireUserExists(ctx, identity.UserID(uid)); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	// Cross-app safety: every role_id must belong to the requested app.
	// Disabled roles are still removable, so we don't gate on status.
	for _, rid := range ridsTyped {
		r, err := s.loadAnyRole(ctx, rid)
		if err != nil {
			out, reason := classifyError(err)
			s.auditor.Emit(ctx, withOutcome(aud, out, reason))
			return err
		}
		if r.AppID().String() != aid.String() {
			out, reason := classifyError(access.ErrRoleNotInApp)
			s.auditor.Emit(ctx, withOutcome(aud, out, reason))
			return access.ErrRoleNotInApp
		}
	}

	if err := s.repo.BulkDelete(ctx, uid, ridsTyped); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	s.auditor.Success(ctx, aud)
	return nil
}
