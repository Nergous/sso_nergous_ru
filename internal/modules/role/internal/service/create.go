package service

import (
	"context"

	"sso/internal/modules/audit"
	"sso/internal/kernel/actor"
	"sso/internal/modules/role/internal/domain"
)

// CreateRoleInput is the parsed CreateRoleRequest. ParentAppID is the
// proto's parent_app_id field; Name/Description/Permissions come off the
// embedded Role message. Field-level validation (regex on permission
// strings, length caps) is expected upstream from protovalidate.
type CreateRoleInput struct {
	ParentAppID string
	Name        string
	Description string
	Permissions []string
}

// CreateRole provisions a new role inside the parent app. Server
// generates id, etag, timestamps; status defaults to ACTIVE.
//
// Errors:
//
//	ValidationError      — parent_app_id not a valid UUID
//	ErrRoleAlreadyExists — name collides with an existing role in the app
func (s *Service) CreateRole(ctx context.Context, in CreateRoleInput) (*domain.Role, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return nil, err
	}
	appID, err := domain.ParseAppID(in.ParentAppID)
	if err != nil {
		return nil, err
	}
	id, err := domain.NewRoleID()
	if err != nil {
		return nil, err
	}

	r := domain.NewRole(domain.NewRoleParams{
		ID:          id,
		AppID:       appID,
		Name:        in.Name,
		Description: in.Description,
		Permissions: in.Permissions,
		Now:         s.now().UTC(),
	})

	aud := audit.BaseFromActor(a, audit.EventTypeRoleCreateRole)
	aud.SubjectType = audit.SubjectTypeApp
	aud.SubjectID = appID.String()

	if err := s.repo.Create(ctx, r); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}

	s.auditor.Success(ctx, aud)
	return r, nil
}
