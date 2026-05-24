package service

import (
	"context"
	"time"

	"sso/internal/audit/auditx"
	"sso/internal/identity/internal/domain"
	"sso/internal/kernel/cursor"
	"sso/internal/kernel/validation"
)

// ----------------------------------------------------------------------------
// GetUser
// ----------------------------------------------------------------------------

// GetUser returns the identity record for the supplied user_id.
// Returns ErrUserNotFound if no row matches.
func (s *Service) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	id, err := domain.ParseUserID(userID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// ----------------------------------------------------------------------------
// ListUsers
// ----------------------------------------------------------------------------

// ListUsersInput is the parsed ListUsersRequest. The opaque page_token from
// the wire is forwarded as-is; this layer decodes it.
type ListUsersInput struct {
	PageSize     int32
	PageToken    string
	Search       string
	Emails       []string
	Usernames    []string
	DisplayNames []string
	Statuses     []domain.UserStatus
	OrderBy      domain.ListOrderBy
}

// ListUsersOutput is the use-case return — opaque page token plus typed
// users. Total size is forwarded from the repository (nil = not computed).
type ListUsersOutput struct {
	Users         []*domain.User
	NextPageToken string
	TotalSize     *int
}

// ListUsers paginates the identity directory. The repository receives a
// typed cursor; this layer handles the opaque-token round-trip.
func (s *Service) ListUsers(ctx context.Context, in ListUsersInput) (ListUsersOutput, error) {
	after, err := decodeCursor(in.PageToken)
	if err != nil {
		return ListUsersOutput{}, err
	}

	pageSize, err := auditx.ClampPageSize(in.PageSize)
	if err != nil {
		return ListUsersOutput{}, err
	}

	res, err := s.repo.List(ctx, domain.ListQuery{
		PageSize:     pageSize,
		After:        after,
		Search:       in.Search,
		Emails:       in.Emails,
		Usernames:    in.Usernames,
		DisplayNames: in.DisplayNames,
		Statuses:     in.Statuses,
		OrderBy:      in.OrderBy,
	})
	if err != nil {
		return ListUsersOutput{}, err
	}

	nextToken, err := encodeCursor(res.NextCursor)
	if err != nil {
		return ListUsersOutput{}, err
	}

	return ListUsersOutput{
		Users:         res.Users,
		NextPageToken: nextToken,
		TotalSize:     res.TotalSize,
	}, nil
}

// ----------------------------------------------------------------------------
// Page-cursor codec
// ----------------------------------------------------------------------------
//
// pageToken is the JSON shape encoded into the opaque page_token strings
// exchanged with clients. Stable across releases; new fields must be
// additive and tolerated as zero by older servers.

type pageToken struct {
	CreatedAt time.Time `json:"c,omitempty"`
	UserID    string    `json:"i,omitempty"`
	Username  string    `json:"u,omitempty"`
}

// encodeCursor renders a typed cursor as the opaque base64(JSON) token
// sent to clients. nil cursor → "" (last-page marker).
func encodeCursor(c *domain.PageCursor) (string, error) {
	if c == nil {
		return "", nil
	}
	return cursor.Encode(&pageToken{
		CreatedAt: c.CreatedAt,
		UserID:    c.UserID.String(),
		Username:  c.Username,
	})
}

// decodeCursor parses the opaque token previously issued by encodeCursor.
// "" → (nil, nil) — first-page request. Malformed tokens surface as a
// ValidationError tagged "page_token".
func decodeCursor(s string) (*domain.PageCursor, error) {
	t, err := cursor.Decode[pageToken](s)
	if err != nil {
		return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
	}
	if t == nil {
		return nil, nil
	}
	pc := &domain.PageCursor{CreatedAt: t.CreatedAt, Username: t.Username}
	if t.UserID != "" {
		uid, err := domain.ParseUserID(t.UserID)
		if err != nil {
			return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
		}
		pc.UserID = uid
	}
	return pc, nil
}
