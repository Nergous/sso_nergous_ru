// Package access hosts the use-cases for sso.access.v1.AccessService.
//
// Cross-context cooperation: the package imports identity.Repository,
// role.Repository, and app.Repository to enforce preconditions
// (role active, user not blocked/deleted, role belongs to app, etc.).
// The access domain itself stays free of those imports.
//
// File layout (one file per RPC family, mirrors usecase/role):
//
//	service.go     — Service struct + helpers
//	grant.go       — GrantRoleToUser, BulkGrantRoles
//	remove.go      — RemoveRoleFromUser, BulkRemoveRoles
//	check.go       — HasRoleInApp, CheckPermission, BatchCheckPermission
//	list.go        — ListUserRoles
package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	access "sso/internal/access/internal/domain"
	appdom "sso/internal/app"
	"sso/internal/audit"
	"sso/internal/audit/auditx"
	"sso/internal/identity"
	"sso/internal/role"
)

// Service exposes the access use-cases. now is injected for tests.
type Service struct {
	repo    access.Repository
	users   identity.Repository
	roles   role.Repository
	apps    appdom.Repository
	now     func() time.Time
	auditor auditx.Auditor
}

// NewService constructs the service. All four repositories are required;
// nil panics at first use rather than at construction so wiring bugs
// surface in tests.
func NewService(
	log *slog.Logger,
	repo access.Repository,
	users identity.Repository,
	roles role.Repository,
	apps appdom.Repository,
	now func() time.Time,
	emitter audit.Emitter,
) *Service {
	return &Service{
		repo:    repo,
		users:   users,
		roles:   roles,
		apps:    apps,
		now:     now,
		auditor: auditx.New(log, emitter),
	}
}

// withOutcome is a thin alias over auditx.WithOutcome.
func withOutcome(p audit.NewAuditParams, out audit.AuditOutcome, reason string) audit.NewAuditParams {
	return auditx.WithOutcome(p, out, reason)
}

// errReasonMap maps access-domain sentinels to their audit (Outcome,
// Reason) pair. Policy-style rejections (role disabled, user not
// eligible, role not in app) surface as OutcomeDenied; everything else
// as OutcomeFailure. auditx.Classify handles *validation.Error and the
// default fallback.
var errReasonMap = map[error]auditx.OutcomeReason{
	access.ErrRoleDisabled:     auditx.Deny(audit.ReasonRoleDisabled),
	access.ErrUserNotEligible:  auditx.Deny(audit.ReasonUserNotEligible),
	access.ErrRoleNotInApp:     auditx.Deny(audit.ReasonRoleNotInApp),
	access.ErrUserNotFound:     auditx.Fail(audit.ReasonUserNotFound),
	access.ErrRoleNotFound:     auditx.Fail(audit.ReasonRoleNotFound),
	access.ErrAppNotFound:      auditx.Fail(audit.ReasonAppNotFound),
}

// classifyError is the per-package thin wrapper around auditx.Classify.
func classifyError(err error) (audit.AuditOutcome, string) {
	return auditx.Classify(err, errReasonMap)
}

const (
	// bulkOpsCap mirrors the proto-side cap on BulkGrant/BulkRemove and
	// BatchCheckPermission inputs. See access.proto for rationale.
	bulkOpsCap = 32
)

// loadActiveRoleInApp fetches the role and verifies it is ACTIVE. Returns
// translated domain errors so the gRPC mapper can render the right code.
func (s *Service) loadActiveRoleInApp(ctx context.Context, rid role.RoleID, expectedAppID *role.AppID) (*role.Role, error) {
	r, err := s.roles.GetByID(ctx, rid)
	if err != nil {
		if errors.Is(err, role.ErrRoleNotFound) {
			return nil, access.ErrRoleNotFound
		}
		return nil, err
	}
	if r.Status() != role.RoleStatusActive {
		return nil, access.ErrRoleDisabled
	}
	if expectedAppID != nil && r.AppID() != *expectedAppID {
		return nil, access.ErrRoleNotInApp
	}
	return r, nil
}

// requireUserEligible ensures the user exists and is ACTIVE. BLOCKED /
// DELETED users may keep existing assignments (cascade-on-delete is
// only at hard-delete time) but cannot receive new grants and do not
// contribute to CheckPermission.
func (s *Service) requireUserEligible(ctx context.Context, uid identity.UserID) error {
	u, err := s.users.GetByID(ctx, uid)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			return access.ErrUserNotFound
		}
		return err
	}
	if u.Status() != identity.UserStatusActive {
		return access.ErrUserNotEligible
	}
	return nil
}

// requireUserExists checks identity-level existence only. Used by Read /
// List endpoints where BLOCKED / DELETED users are still allowed to be
// queried (their assignments are visible).
func (s *Service) requireUserExists(ctx context.Context, uid identity.UserID) error {
	if _, err := s.users.GetByID(ctx, uid); err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			return access.ErrUserNotFound
		}
		return err
	}
	return nil
}

func (s *Service) requireAppExists(ctx context.Context, aid appdom.AppID) error {
	if _, err := s.apps.GetByID(ctx, aid); err != nil {
		if errors.Is(err, appdom.ErrAppNotFound) {
			return access.ErrAppNotFound
		}
		return err
	}
	return nil
}

// loadAnyRole fetches a role regardless of its status. Used by remove
// flows where DISABLED roles are still valid removal targets.
func (s *Service) loadAnyRole(ctx context.Context, rid access.RoleID) (*role.Role, error) {
	r, err := s.roles.GetByID(ctx, role.RoleID(rid))
	if err != nil {
		if errors.Is(err, role.ErrRoleNotFound) {
			return nil, access.ErrRoleNotFound
		}
		return nil, err
	}
	return r, nil
}
