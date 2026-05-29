package service

import (
	"context"

	"sso/internal/modules/audit"
	"sso/internal/modules/audit/auditx"
	"sso/internal/kernel/actor"
	"sso/internal/kernel/validation"
	"sso/internal/modules/role/internal/domain"
)

// ----------------------------------------------------------------------------
// UpdateRole
// ----------------------------------------------------------------------------

// UpdateRoleInput is the parsed UpdateRoleRequest.
//
// Allowed mask paths (per proto): name, description, permissions
// Forbidden: role_id, app_id, status, etag, created_at, updated_at —
// rejected by buildPatch as "unknown".
type UpdateRoleInput struct {
	RoleID       string
	MaskPaths    []string
	ExpectedEtag string

	Name        string
	Description string
	Permissions []string
}

func (s *Service) UpdateRole(ctx context.Context, in UpdateRoleInput) (*domain.Role, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.ParseRoleID(in.RoleID)
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

	aud := audit.BaseFromActor(a, audit.EventTypeRoleUpdateRole)
	aud.SubjectType = audit.SubjectTypeRole
	aud.SubjectID = id.String()

	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}

	r.ApplyPatch(patch, s.now().UTC())
	if err := s.repo.Update(ctx, r, expectedEtag); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}

	s.auditor.Success(ctx, aud)
	return r, nil
}

func buildPatch(in UpdateRoleInput) (domain.RolePatch, error) {
	var p domain.RolePatch
	for _, path := range in.MaskPaths {
		switch path {
		case "name":
			v := in.Name
			p.Name = &v
		case "description":
			v := in.Description
			p.Description = &v
		case "permissions":
			v := in.Permissions
			p.Permissions = &v
		default:
			return domain.RolePatch{}, &validation.Error{
				Field:  "update_mask",
				Reason: "unknown field path: " + path,
			}
		}
	}
	return p, nil
}

// ----------------------------------------------------------------------------
// DisableRole / EnableRole — status transitions
// ----------------------------------------------------------------------------

type DisableRoleInput struct {
	RoleID       string
	AllowMissing bool
}

func (s *Service) DisableRole(ctx context.Context, in DisableRoleInput) error {
	return s.lifecycleTransition(ctx, in.RoleID, in.AllowMissing, audit.EventTypeRoleDisableRole, func(r *domain.Role) {
		r.Disable(s.now().UTC())
	})
}

type EnableRoleInput struct {
	RoleID       string
	AllowMissing bool
}

func (s *Service) EnableRole(ctx context.Context, in EnableRoleInput) error {
	return s.lifecycleTransition(ctx, in.RoleID, in.AllowMissing, audit.EventTypeRoleEnableRole, func(r *domain.Role) {
		r.Enable(s.now().UTC())
	})
}
