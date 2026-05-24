package service

import (
	"context"
	"time"

	"sso/internal/audit/auditx"
	"sso/internal/kernel/cursor"
	"sso/internal/kernel/validation"
	"sso/internal/role/internal/domain"
)

// ----------------------------------------------------------------------------
// GetRole
// ----------------------------------------------------------------------------

func (s *Service) GetRole(ctx context.Context, roleID string) (*domain.Role, error) {
	id, err := domain.ParseRoleID(roleID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// ----------------------------------------------------------------------------
// ListRoles
// ----------------------------------------------------------------------------

// ListRolesInput is the parsed ListRolesRequest. AppID is the proto's
// filters.app_id (required — listing is always scoped per-app).
type ListRolesInput struct {
	AppID     string
	PageSize  int32
	PageToken string
	Search    string
	Statuses  []domain.RoleStatus
	OrderBy   domain.ListOrderBy
}

type ListRolesOutput struct {
	Roles         []*domain.Role
	NextPageToken string
	TotalSize     *int
}

func (s *Service) ListRoles(ctx context.Context, in ListRolesInput) (ListRolesOutput, error) {
	appID, err := domain.ParseAppID(in.AppID)
	if err != nil {
		return ListRolesOutput{}, err
	}

	after, err := decodeCursor(in.PageToken)
	if err != nil {
		return ListRolesOutput{}, err
	}

	pageSize, err := auditx.ClampPageSize(in.PageSize)
	if err != nil {
		return ListRolesOutput{}, err
	}

	res, err := s.repo.List(ctx, domain.ListQuery{
		AppID:    appID,
		PageSize: pageSize,
		After:    after,
		Search:   in.Search,
		Statuses: in.Statuses,
		OrderBy:  in.OrderBy,
	})
	if err != nil {
		return ListRolesOutput{}, err
	}

	nextToken, err := encodeCursor(res.NextCursor)
	if err != nil {
		return ListRolesOutput{}, err
	}

	return ListRolesOutput{
		Roles:         res.Roles,
		NextPageToken: nextToken,
		TotalSize:     res.TotalSize,
	}, nil
}

// ----------------------------------------------------------------------------
// Page-cursor codec — same JSON-over-base64url shape as identity / app.
// ----------------------------------------------------------------------------

type pageToken struct {
	CreatedAt time.Time `json:"c,omitempty"`
	RoleID    string    `json:"i,omitempty"`
	Name      string    `json:"n,omitempty"`
}

func encodeCursor(c *domain.PageCursor) (string, error) {
	if c == nil {
		return "", nil
	}
	return cursor.Encode(&pageToken{
		CreatedAt: c.CreatedAt,
		RoleID:    c.RoleID.String(),
		Name:      c.Name,
	})
}

func decodeCursor(s string) (*domain.PageCursor, error) {
	t, err := cursor.Decode[pageToken](s)
	if err != nil {
		return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
	}
	if t == nil {
		return nil, nil
	}
	pc := &domain.PageCursor{CreatedAt: t.CreatedAt, Name: t.Name}
	if t.RoleID != "" {
		id, err := domain.ParseRoleID(t.RoleID)
		if err != nil {
			return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
		}
		pc.RoleID = id
	}
	return pc, nil
}
