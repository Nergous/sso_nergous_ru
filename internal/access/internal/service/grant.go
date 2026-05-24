package service

import (
	"context"

	access "sso/internal/access/internal/domain"
	appdom "sso/internal/app"
	"sso/internal/audit"
	"sso/internal/identity"
	"sso/internal/kernel/actor"
	"sso/internal/kernel/validation"
	"sso/internal/role"
)

// ----------------------------------------------------------------------------
// GrantRoleToUser
// ----------------------------------------------------------------------------

type GrantRoleToUserInput struct {
	UserID  string
	RoleID  string
	ActorID string // granted_by_user_id; empty until the auth interceptor lands
}

type GrantRoleToUserOutput struct {
	Assignment *access.RoleAssignment
	Created    bool
}

func (s *Service) GrantRoleToUser(ctx context.Context, in GrantRoleToUserInput) (GrantRoleToUserOutput, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return GrantRoleToUserOutput{}, err
	}
	uid, err := access.ParseUserID(in.UserID)
	if err != nil {
		return GrantRoleToUserOutput{}, err
	}
	rid, err := access.ParseRoleID(in.RoleID)
	if err != nil {
		return GrantRoleToUserOutput{}, err
	}
	actorID, err := access.ParseActorID(in.ActorID)
	if err != nil {
		return GrantRoleToUserOutput{}, err
	}

	aud := audit.BaseFromActor(a, audit.EventTypeAccessGrantRoleToUser)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = uid.String()

	// Preconditions: role active, user active. Role lookup also gives us
	// the app_id to denormalise into the assignment row.
	r, err := s.loadActiveRoleInApp(ctx, role.RoleID(rid), nil)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return GrantRoleToUserOutput{}, err
	}
	aud.AppID = r.AppID().String()

	if err := s.requireUserEligible(ctx, identity.UserID(uid)); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return GrantRoleToUserOutput{}, err
	}

	target := access.NewRoleAssignment(access.NewRoleAssignmentParams{
		UserID:          uid,
		RoleID:          rid,
		AppID:           access.AppID(r.AppID().String()),
		GrantedByUserID: actorID,
		Now:             s.now().UTC(),
	})

	created, err := s.repo.Create(ctx, target)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return GrantRoleToUserOutput{}, err
	}
	if !created {
		// Idempotent re-grant: surface the original row so the caller
		// can read the canonical granted_at and granted_by_user_id.
		existing, err := s.repo.Get(ctx, uid, rid)
		if err != nil {
			out, reason := classifyError(err)
			s.auditor.Emit(ctx, withOutcome(aud, out, reason))
			return GrantRoleToUserOutput{}, err
		}
		s.auditor.Success(ctx, aud)
		return GrantRoleToUserOutput{Assignment: existing, Created: false}, nil
	}

	s.auditor.Success(ctx, aud)
	return GrantRoleToUserOutput{Assignment: target, Created: true}, nil
}

// ----------------------------------------------------------------------------
// BulkGrantRoles — atomic, all-or-nothing
// ----------------------------------------------------------------------------

type BulkGrantRolesInput struct {
	UserID  string
	AppID   string
	RoleIDs []string
	ActorID string
}

type BulkGrantRolesOutput struct {
	// Positionally aligned with input.RoleIDs. created[i] is true for
	// fresh inserts, false for idempotent re-grants.
	Assignments []*access.RoleAssignment
	Created     []bool
}

func (s *Service) BulkGrantRoles(ctx context.Context, in BulkGrantRolesInput) (BulkGrantRolesOutput, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return BulkGrantRolesOutput{}, err
	}
	uid, aid, ridsTyped, actorID, err := s.parseBulkInput(in.UserID, in.AppID, in.RoleIDs, in.ActorID)
	if err != nil {
		return BulkGrantRolesOutput{}, err
	}

	aud := audit.BaseFromActor(a, audit.EventTypeAccessBulkGrantRoles)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = uid.String()
	aud.AppID = aid.String()

	// All preconditions (existence/eligibility, role-in-app, role-active)
	// are validated up front so a mid-batch failure cannot leave a
	// half-applied state.
	if err := s.requireAppExists(ctx, appdom.AppID(aid)); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return BulkGrantRolesOutput{}, err
	}
	if err := s.requireUserEligible(ctx, identity.UserID(uid)); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return BulkGrantRolesOutput{}, err
	}
	expectedAppID := role.AppID(aid)
	for _, rid := range ridsTyped {
		if _, err := s.loadActiveRoleInApp(ctx, role.RoleID(rid), &expectedAppID); err != nil {
			out, reason := classifyError(err)
			s.auditor.Emit(ctx, withOutcome(aud, out, reason))
			return BulkGrantRolesOutput{}, err
		}
	}

	now := s.now().UTC()
	assignments := make([]*access.RoleAssignment, len(ridsTyped))
	for i, rid := range ridsTyped {
		assignments[i] = access.NewRoleAssignment(access.NewRoleAssignmentParams{
			UserID:          uid,
			RoleID:          rid,
			AppID:           aid,
			GrantedByUserID: actorID,
			Now:             now,
		})
	}

	createdMask, err := s.repo.BulkCreate(ctx, assignments)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return BulkGrantRolesOutput{}, err
	}

	// For the entries that were "already there" (createdMask[i]=false),
	// fetch the canonical row so the response carries the original
	// granted_at instead of `now`.
	for i, was := range createdMask {
		if was {
			continue
		}
		existing, gerr := s.repo.Get(ctx, uid, ridsTyped[i])
		if gerr != nil {
			// Should not happen — we just observed a duplicate-key for
			// this pair — but propagate anything weird honestly.
			out, reason := classifyError(gerr)
			s.auditor.Emit(ctx, withOutcome(aud, out, reason))
			return BulkGrantRolesOutput{}, gerr
		}
		assignments[i] = existing
	}

	s.auditor.Success(ctx, aud)
	return BulkGrantRolesOutput{Assignments: assignments, Created: createdMask}, nil
}

// parseBulkInput validates and converts the shared (user, app, role_ids,
// actor) tuple used by both BulkGrant / BulkRemove.
func (s *Service) parseBulkInput(rawUser, rawApp string, rawRoles []string, rawActor string) (
	access.UserID, access.AppID, []access.RoleID, access.ActorID, error,
) {
	if len(rawRoles) == 0 {
		return "", "", nil, "", &validation.Error{Field: "role_ids", Reason: "must contain at least one role"}
	}
	if len(rawRoles) > bulkOpsCap {
		return "", "", nil, "", &validation.Error{Field: "role_ids", Reason: "too many roles in one request"}
	}

	uid, err := access.ParseUserID(rawUser)
	if err != nil {
		return "", "", nil, "", err
	}
	aid, err := access.ParseAppID(rawApp)
	if err != nil {
		return "", "", nil, "", err
	}
	actorID, err := access.ParseActorID(rawActor)
	if err != nil {
		return "", "", nil, "", err
	}

	rids := make([]access.RoleID, len(rawRoles))
	seen := make(map[access.RoleID]struct{}, len(rawRoles))
	for i, raw := range rawRoles {
		rid, err := access.ParseRoleID(raw)
		if err != nil {
			return "", "", nil, "", err
		}
		if _, dup := seen[rid]; dup {
			return "", "", nil, "", &validation.Error{
				Field:  "role_ids",
				Reason: "duplicate role_id in request",
			}
		}
		seen[rid] = struct{}{}
		rids[i] = rid
	}
	return uid, aid, rids, actorID, nil
}
