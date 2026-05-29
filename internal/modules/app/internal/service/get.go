package service

import (
	"context"
	"time"

	"sso/internal/modules/app/internal/domain"
	"sso/internal/modules/audit/auditx"
	"sso/internal/kernel/cursor"
	"sso/internal/kernel/validation"
)

// ----------------------------------------------------------------------------
// GetApp
// ----------------------------------------------------------------------------

func (s *Service) GetApp(ctx context.Context, appID string) (*domain.App, error) {
	id, err := domain.ParseAppID(appID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// ----------------------------------------------------------------------------
// ListApps
// ----------------------------------------------------------------------------

type ListAppsInput struct {
	PageSize  int32
	PageToken string
	Search    string
	Statuses  []domain.AppStatus
	OrderBy   domain.ListOrderBy
}

type ListAppsOutput struct {
	Apps          []*domain.App
	NextPageToken string
	TotalSize     *int
}

func (s *Service) ListApps(ctx context.Context, in ListAppsInput) (ListAppsOutput, error) {
	cursor, err := decodeCursor(in.PageToken)
	if err != nil {
		return ListAppsOutput{}, err
	}

	pageSize, err := auditx.ClampPageSize(in.PageSize)
	if err != nil {
		return ListAppsOutput{}, err
	}

	res, err := s.repo.List(ctx, domain.ListQuery{
		PageSize: pageSize,
		After:    cursor,
		Search:   in.Search,
		Statuses: in.Statuses,
		OrderBy:  in.OrderBy,
	})
	if err != nil {
		return ListAppsOutput{}, err
	}

	nextToken, err := encodeCursor(res.NextCursor)
	if err != nil {
		return ListAppsOutput{}, err
	}

	return ListAppsOutput{
		Apps:          res.Apps,
		NextPageToken: nextToken,
		TotalSize:     res.TotalSize,
	}, nil
}

// ----------------------------------------------------------------------------
// Page-cursor codec — same JSON-over-base64url shape as the identity codec.
// ----------------------------------------------------------------------------

type pageToken struct {
	CreatedAt time.Time `json:"c,omitempty"`
	AppID     string    `json:"i,omitempty"`
	Name      string    `json:"n,omitempty"`
}

func encodeCursor(c *domain.PageCursor) (string, error) {
	if c == nil {
		return "", nil
	}
	return cursor.Encode(&pageToken{
		CreatedAt: c.CreatedAt,
		AppID:     c.AppID.String(),
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
	if t.AppID != "" {
		id, err := domain.ParseAppID(t.AppID)
		if err != nil {
			return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
		}
		pc.AppID = id
	}
	return pc, nil
}
