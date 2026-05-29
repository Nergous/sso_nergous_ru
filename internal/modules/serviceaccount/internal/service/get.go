package service

import (
	"context"
	"time"

	serviceAccount "sso/internal/modules/serviceaccount/internal/domain"
	"sso/internal/kernel/cursor"
	"sso/internal/kernel/validation"
	"sso/internal/modules/audit/auditx"
)

// ----------------------------------------------------------------------------
// GetServiceAccount
// ----------------------------------------------------------------------------

func (s *Service) GetServiceAccount(ctx context.Context, rawID string) (*serviceAccount.ServiceAccount, error) {
	id, err := serviceAccount.ParseServiceAccountID(rawID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// ----------------------------------------------------------------------------
// ListServiceAccounts
// ----------------------------------------------------------------------------

type ListServiceAccountsInput struct {
	PageSize  int32
	PageToken string
	Search    string
	Statuses  []serviceAccount.ServiceAccountStatus
	OrderBy   serviceAccount.ListOrderBy
}

type ListServiceAccountsOutput struct {
	ServiceAccounts []*serviceAccount.ServiceAccount
	NextPageToken   string
	TotalSize       *int
}

func (s *Service) ListServiceAccounts(ctx context.Context, in ListServiceAccountsInput) (ListServiceAccountsOutput, error) {
	after, err := decodeCursor(in.PageToken)
	if err != nil {
		return ListServiceAccountsOutput{}, err
	}

	pageSize, err := auditx.ClampPageSize(in.PageSize)
	if err != nil {
		return ListServiceAccountsOutput{}, err
	}

	res, err := s.repo.List(ctx, serviceAccount.ListQuery{
		PageSize: pageSize,
		After:    after,
		Search:   in.Search,
		Statuses: in.Statuses,
		OrderBy:  in.OrderBy,
	})
	if err != nil {
		return ListServiceAccountsOutput{}, err
	}

	nextToken, err := encodeCursor(res.NextCursor)
	if err != nil {
		return ListServiceAccountsOutput{}, err
	}
	return ListServiceAccountsOutput{
		ServiceAccounts: res.ServiceAccounts,
		NextPageToken:   nextToken,
		TotalSize:       res.TotalSize,
	}, nil
}

// ----------------------------------------------------------------------------
// Page-cursor codec — same JSON-over-base64url shape as identity / role.
// ----------------------------------------------------------------------------

type pageToken struct {
	CreatedAt        time.Time `json:"c,omitempty"`
	ServiceAccountID string    `json:"i,omitempty"`
	Name             string    `json:"n,omitempty"`
}

func encodeCursor(c *serviceAccount.PageCursor) (string, error) {
	if c == nil {
		return "", nil
	}
	return cursor.Encode(&pageToken{
		CreatedAt:        c.CreatedAt,
		ServiceAccountID: c.ServiceAccountID.String(),
		Name:             c.Name,
	})
}

func decodeCursor(s string) (*serviceAccount.PageCursor, error) {
	t, err := cursor.Decode[pageToken](s)
	if err != nil {
		return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
	}
	if t == nil {
		return nil, nil
	}
	pc := &serviceAccount.PageCursor{CreatedAt: t.CreatedAt, Name: t.Name}
	if t.ServiceAccountID != "" {
		id, err := serviceAccount.ParseServiceAccountID(t.ServiceAccountID)
		if err != nil {
			return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
		}
		pc.ServiceAccountID = id
	}
	return pc, nil
}
