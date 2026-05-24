package service

import (
	"context"
	"time"

	access "sso/internal/access/internal/domain"
	appdom "sso/internal/app"
	"sso/internal/audit/auditx"
	"sso/internal/identity"
	"sso/internal/kernel/cursor"
	"sso/internal/kernel/validation"
	"sso/internal/role"
)

// ----------------------------------------------------------------------------
// ListUserRoles
// ----------------------------------------------------------------------------

type ListUserRolesInput struct {
	UserID    string
	AppID     string
	PageSize  int32
	PageToken string
	OrderBy   access.ListOrderBy
}

type ListUserRolesOutput struct {
	// Roles assigned to the user in the target app. Order matches the
	// requested OrderBy; DISABLED roles are included (proto requires it).
	Roles         []*role.Role
	NextPageToken string
	TotalSize     *int
}

func (s *Service) ListUserRoles(ctx context.Context, in ListUserRolesInput) (ListUserRolesOutput, error) {
	uid, err := access.ParseUserID(in.UserID)
	if err != nil {
		return ListUserRolesOutput{}, err
	}
	aid, err := access.ParseAppID(in.AppID)
	if err != nil {
		return ListUserRolesOutput{}, err
	}

	if err := s.requireUserExists(ctx, identity.UserID(uid)); err != nil {
		return ListUserRolesOutput{}, err
	}
	// app.AppID and access.AppID are both `type X string` — direct conv ok.
	if err := s.requireAppExists(ctx, appdom.AppID(aid)); err != nil {
		return ListUserRolesOutput{}, err
	}

	after, err := decodeCursor(in.PageToken)
	if err != nil {
		return ListUserRolesOutput{}, err
	}

	pageSize, err := auditx.ClampPageSize(in.PageSize)
	if err != nil {
		return ListUserRolesOutput{}, err
	}

	res, err := s.repo.ListUserRoles(ctx, access.ListUserRolesQuery{
		UserID:   uid,
		AppID:    aid,
		PageSize: pageSize,
		After:    after,
		OrderBy:  in.OrderBy,
	})
	if err != nil {
		return ListUserRolesOutput{}, err
	}

	// Hydrate full Role aggregates from role.Repository — keeps the
	// access bounded context unaware of role internals (permissions,
	// status names, etc.).
	roles := make([]*role.Role, 0, len(res.Rows))
	for _, row := range res.Rows {
		r, err := s.roles.GetByID(ctx, role.RoleID(row.RoleID))
		if err != nil {
			// Race window: a role can be PermanentlyDeleted between the
			// list query and this hydrate (FK ON DELETE CASCADE will
			// eventually drop the assignment row, but until it does the
			// SELECT may surface an orphan reference). Skip gracefully.
			continue
		}
		roles = append(roles, r)
	}

	nextToken, err := encodeCursor(res.NextCursor)
	if err != nil {
		return ListUserRolesOutput{}, err
	}
	return ListUserRolesOutput{
		Roles:         roles,
		NextPageToken: nextToken,
		TotalSize:     res.TotalSize,
	}, nil
}

// ----------------------------------------------------------------------------
// Page-cursor codec — JSON over base64url, same shape as identity / role.
// ----------------------------------------------------------------------------

type pageToken struct {
	GrantedAt time.Time `json:"g,omitempty"`
	RoleID    string    `json:"i,omitempty"`
}

func encodeCursor(c *access.PageCursor) (string, error) {
	if c == nil {
		return "", nil
	}
	return cursor.Encode(&pageToken{
		GrantedAt: c.GrantedAt,
		RoleID:    c.RoleID.String(),
	})
}

func decodeCursor(s string) (*access.PageCursor, error) {
	t, err := cursor.Decode[pageToken](s)
	if err != nil {
		return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
	}
	if t == nil {
		return nil, nil
	}
	pc := &access.PageCursor{GrantedAt: t.GrantedAt}
	if t.RoleID != "" {
		rid, err := access.ParseRoleID(t.RoleID)
		if err != nil {
			return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
		}
		pc.RoleID = rid
	}
	return pc, nil
}
