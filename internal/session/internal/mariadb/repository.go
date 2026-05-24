package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	domain "sso/internal/session/internal/domain"
	"sso/internal/session/internal/mariadb/dbgen"
)

type Repository struct {
	db *sql.DB
	q  *dbgen.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: dbgen.New(db)}
}

var _ domain.Repository = (*Repository)(nil)

func (r *Repository) Create(ctx context.Context, s *domain.Session) error {
	if err := r.q.CreateSession(ctx, toCreateParams(s)); err != nil {
		return fmt.Errorf("session repo: create: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id domain.SessionID) (*domain.Session, error) {
	row, err := r.q.GetSessionById(ctx, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSessionNotFound
		}
		return nil, fmt.Errorf("session repo: get_by_id: %w", err)
	}
	return dbgenToDomain(row), nil
}

func (r *Repository) GetByRefreshHash(ctx context.Context, hash []byte) (*domain.Session, error) {
	row, err := r.q.GetSessionByRefreshHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSessionNotFound
		}
		return nil, fmt.Errorf("session repo: get_by_refresh_hash: %w", err)
	}
	return dbgenToDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, s *domain.Session) error {
	res, err := r.q.UpdateSession(ctx, toUpdateParams(s))
	if err != nil {
		return fmt.Errorf("session repo: update: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("session repo: update: rows_affected: %w", err)
	}

	if rows == 0 {
		return domain.ErrSessionNotFound
	}

	return nil
}

func (r *Repository) Rotate(ctx context.Context, s *domain.Session, expectedRefreshHash []byte) error {
	res, err := r.q.RotateSessionRefresh(ctx, toRotateParams(s, expectedRefreshHash))
	if err != nil {
		return fmt.Errorf("session repo: rotate: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("session repo: rotate: rows_affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrRefreshTokenReused
	}
	return nil
}

func (r *Repository) ListByUser(ctx context.Context, userID domain.UserID) ([]*domain.Session, error) {
	sessions, err := r.q.ListSessionsByUser(ctx, userID.String())
	if err != nil {
		return make([]*domain.Session, 0, len(sessions)), fmt.Errorf("session repo: list_by_user: %w", err)
	}
	var domainSessions []*domain.Session
	for _, s := range sessions {
		domainSessions = append(domainSessions, dbgenToDomain(s))
	}
	return domainSessions, nil
}

func (r *Repository) RevokeAllForUser(ctx context.Context, userID domain.UserID, now time.Time) error {
	_, err := r.q.RevokeAllSessionsForUser(ctx, toRevokeParams(userID, now))
	if err != nil {
		return fmt.Errorf("session repo: revoke_all_for_user: %w", err)
	}
	return nil
}
