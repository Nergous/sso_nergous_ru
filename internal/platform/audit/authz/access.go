package authz

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sso/internal/access"
	"sso/internal/kernel/actor"
	"sync/atomic"
)

const (
	adminAppSlug       = "sso-admin"
	requiredPermission = "audit:read"
)

type AccessBackedAuthorizer struct {
	accessSvc *access.Service
	db        *sql.DB
	log       *slog.Logger

	appID atomic.Pointer[string]
}

func New(accessSvc *access.Service, db *sql.DB, log *slog.Logger) *AccessBackedAuthorizer {
	return &AccessBackedAuthorizer{accessSvc: accessSvc, db: db, log: log}
}

func (a *AccessBackedAuthorizer) CanReadAudit(ctx context.Context) (bool, error) {
	act, ok := actor.From(ctx)
	if !ok || act.Kind != actor.KindUser {
		return false, nil
	}

	resolved, err := a.resolveAdminAppID(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		a.log.Warn("auditauthz: resolve admin app", slog.Any("error", err))
		return false, err
	}

	output, err := a.accessSvc.CheckPermission(ctx, access.CheckPermissionInput{
		UserID:     act.ID,
		AppID:      resolved,
		Permission: requiredPermission,
	})
	if err != nil {
		a.log.Warn("auditauthz: check permission", slog.Any("error", err))
		return false, err
	}

	return output.Allowed, nil
}

// resolveAdminAppID looks up the sso-admin app UUID. Hits are cached;
// misses are not, so seed-admin can be run after sso starts without a
// process restart.
func (a *AccessBackedAuthorizer) resolveAdminAppID(ctx context.Context) (string, error) {
	if p := a.appID.Load(); p != nil {
		return *p, nil
	}
	var id string
	if err := a.db.QueryRowContext(ctx,
		"SELECT id FROM apps WHERE slug = ?", adminAppSlug,
	).Scan(&id); err != nil {
		return "", err
	}
	a.appID.Store(&id)
	return id, nil
}
