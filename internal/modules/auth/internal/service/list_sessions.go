package service

import (
	"context"
	"fmt"
	"time"

	"sso/internal/modules/identity"
	"sso/internal/modules/session"
	"sso/internal/kernel/cursor"
	"sso/internal/kernel/validation"
)

// Default and ceiling for ListSessions.page_size. The proto validates
// ≤ 100 on the wire (typical users have O(10) sessions, not O(1000) —
// the cap is lower than IdentityService); the default lands well under
// the ceiling so the typical client never paginates.
const (
	defaultListSessionsPageSize = 50
	maxListSessionsPageSize     = 100
)

// ListSessionsInput is what the handler hands over. UserID is the
// caller's own subject id from the verified-claims actor; the proto
// contract scopes ListSessions to the caller's own sessions.
type ListSessionsInput struct {
	UserID    string
	PageSize  int32
	PageToken string
}

// ListSessionsOutput is the use-case view of one page. Sessions are
// ordered by IssuedAt DESC (most recent first — Repository.ListByUser
// contract). NextPageToken is "" on the last page. TotalSize is the
// number of currently-active sessions for the user across all pages —
// the count is cheap to compute since the repo returns everything.
type ListSessionsOutput struct {
	Sessions      []*session.Session
	NextPageToken string
	TotalSize     int
}

// ListSessions returns one page of the caller's currently-active
// sessions. Revoked or expired rows are filtered out (the proto says
// "currently active"). Sort order matches Repository.ListByUser:
// IssuedAt DESC.
//
// Pagination is in-memory keyset over the full per-user result. The
// repository does not yet expose a paged list; for O(10) sessions per
// user that is fine. When ListByUser grows a query-paged variant, the
// keyset logic here moves to the repository unchanged on the wire
// (the page_token shape is stable).
func (s *Service) ListSessions(ctx context.Context, in ListSessionsInput) (ListSessionsOutput, error) {
	userID, err := identity.ParseUserID(in.UserID)
	if err != nil {
		return ListSessionsOutput{}, err
	}

	pageSize := int(in.PageSize)
	switch {
	case pageSize < 0:
		return ListSessionsOutput{}, &validation.Error{Field: "page_size", Reason: "must be ≥ 0"}
	case pageSize == 0:
		pageSize = defaultListSessionsPageSize
	case pageSize > maxListSessionsPageSize:
		pageSize = maxListSessionsPageSize
	}

	after, err := decodeSessionsCursor(in.PageToken)
	if err != nil {
		return ListSessionsOutput{}, err
	}

	all, err := s.sessions.ListByUser(ctx, session.UserID(userID.String()))
	if err != nil {
		return ListSessionsOutput{}, fmt.Errorf("list sessions: list by user: %w", err)
	}

	now := s.now().UTC()
	active := make([]*session.Session, 0, len(all))
	for _, sess := range all {
		if sess.IsActive(now) {
			active = append(active, sess)
		}
	}

	page, next := selectSessionsPage(active, after, pageSize)

	nextToken, err := encodeSessionsCursor(next)
	if err != nil {
		return ListSessionsOutput{}, fmt.Errorf("list sessions: encode cursor: %w", err)
	}

	return ListSessionsOutput{
		Sessions:      page,
		NextPageToken: nextToken,
		TotalSize:     len(active),
	}, nil
}

// selectSessionsPage applies the keyset cursor (in DESC IssuedAt order)
// and slices off one page. active MUST be sorted IssuedAt DESC by the
// repository.
//
// "after" means: skip every row at or before the cursor in DESC order
// — i.e. rows with strictly later IssuedAt, or identical IssuedAt and
// id ≥ cursor.id, are the previous page. We resume at the first row
// strictly older than the cursor (or with the same IssuedAt and a
// smaller id, which acts as the tie-breaker).
func selectSessionsPage(
	active []*session.Session, after *sessionsPageToken, pageSize int,
) ([]*session.Session, *sessionsPageToken) {
	start := len(active) // exhausted by default
	if after == nil {
		start = 0
	} else {
		for i, sess := range active {
			if cursorBefore(sess, *after) {
				start = i
				break
			}
		}
	}
	remaining := active[start:]

	hasMore := len(remaining) > pageSize
	if hasMore {
		remaining = remaining[:pageSize]
	}
	var next *sessionsPageToken
	if hasMore {
		last := remaining[len(remaining)-1]
		next = &sessionsPageToken{
			IssuedAt:  last.IssuedAt(),
			SessionID: last.ID().String(),
		}
	}
	return remaining, next
}

// cursorBefore reports whether sess sits strictly past the cursor in
// the DESC-by-IssuedAt order — i.e. it is part of a later page than
// the one the client just consumed.
func cursorBefore(sess *session.Session, c sessionsPageToken) bool {
	if sess.IssuedAt().Before(c.IssuedAt) {
		return true
	}
	if sess.IssuedAt().Equal(c.IssuedAt) && sess.ID().String() < c.SessionID {
		return true
	}
	return false
}

// ----------------------------------------------------------------------------
// Page-cursor codec
// ----------------------------------------------------------------------------
//
// sessionsPageToken is the JSON shape encoded into the opaque
// page_token strings exchanged with clients. Field names are short to
// keep the wire payload compact; the schema is stable across releases
// — new fields must be additive and tolerated as zero by older
// servers.

type sessionsPageToken struct {
	IssuedAt  time.Time `json:"t,omitempty"`
	SessionID string    `json:"i,omitempty"`
}

func encodeSessionsCursor(c *sessionsPageToken) (string, error) {
	return cursor.Encode(c)
}

func decodeSessionsCursor(s string) (*sessionsPageToken, error) {
	t, err := cursor.Decode[sessionsPageToken](s)
	if err != nil {
		return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
	}
	if t == nil {
		return nil, nil
	}
	if _, err := session.ParseSessionID(t.SessionID); err != nil {
		return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
	}
	return t, nil
}
