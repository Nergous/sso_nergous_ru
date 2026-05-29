package service

import (
	"context"
	"errors"
	"strings"

	"sso/internal/modules/access/internal/domain"
	"sso/internal/modules/app"
	"sso/internal/modules/identity"
	"sso/internal/kernel/validation"
)

// ----------------------------------------------------------------------------
// HasRoleInApp
// ----------------------------------------------------------------------------

type HasRoleInAppInput struct {
	UserID string
	RoleID string
}

// HasRoleInApp checks whether the (user, role) assignment exists.
// DISABLED roles still return true — the assignment exists, even if it
// no longer contributes to CheckPermission. The proto explicitly
// requires this distinction.
func (s *Service) HasRoleInApp(ctx context.Context, in HasRoleInAppInput) (bool, error) {
	uid, err := domain.ParseUserID(in.UserID)
	if err != nil {
		return false, err
	}
	rid, err := domain.ParseRoleID(in.RoleID)
	if err != nil {
		return false, err
	}

	// Existence preconditions: user + role must resolve before the
	// "has assignment" question makes sense (proto requires NOT_FOUND).
	if err := s.requireUserExists(ctx, identity.UserID(uid)); err != nil {
		return false, err
	}
	if _, err := s.loadAnyRole(ctx, rid); err != nil {
		return false, err
	}

	if _, err := s.repo.Get(ctx, uid, rid); err != nil {
		if errors.Is(err, domain.ErrAssignmentNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ----------------------------------------------------------------------------
// CheckPermission
// ----------------------------------------------------------------------------

type CheckPermissionInput struct {
	UserID     string
	AppID      string
	Permission string
}

type CheckPermissionOutput struct {
	Allowed        bool
	MatchedRoleIDs []string
}

func (s *Service) CheckPermission(ctx context.Context, in CheckPermissionInput) (CheckPermissionOutput, error) {
	uid, aid, perms, err := s.parseCheckInput(in.UserID, in.AppID, []string{in.Permission})
	if err != nil {
		return CheckPermissionOutput{}, err
	}

	if err := s.requireUserExists(ctx, identity.UserID(uid)); err != nil {
		return CheckPermissionOutput{}, err
	}
	if err := s.requireAppExists(ctx, app.AppID(aid)); err != nil {
		return CheckPermissionOutput{}, err
	}

	rows, err := s.repo.ListActivePermissions(ctx, uid, aid)
	if err != nil {
		return CheckPermissionOutput{}, err
	}

	matchedSet := matchPermissions(rows, perms[0])
	matched := make([]string, 0, len(matchedSet))
	for rid := range matchedSet {
		matched = append(matched, rid.String())
	}
	return CheckPermissionOutput{
		Allowed:        len(matched) > 0,
		MatchedRoleIDs: matched,
	}, nil
}

// ----------------------------------------------------------------------------
// BatchCheckPermission
// ----------------------------------------------------------------------------

type BatchCheckPermissionInput struct {
	UserID      string
	AppID       string
	Permissions []string
}

type BatchCheckPermissionOutput struct {
	// Allowed[i] mirrors Permissions[i] from the request — same length,
	// same order. matched_role_ids is intentionally NOT returned in
	// batch form per the proto contract.
	Allowed []bool
}

func (s *Service) BatchCheckPermission(ctx context.Context, in BatchCheckPermissionInput) (BatchCheckPermissionOutput, error) {
	uid, aid, perms, err := s.parseCheckInput(in.UserID, in.AppID, in.Permissions)
	if err != nil {
		return BatchCheckPermissionOutput{}, err
	}

	if err := s.requireUserExists(ctx, identity.UserID(uid)); err != nil {
		return BatchCheckPermissionOutput{}, err
	}
	if err := s.requireAppExists(ctx, app.AppID(aid)); err != nil {
		return BatchCheckPermissionOutput{}, err
	}

	rows, err := s.repo.ListActivePermissions(ctx, uid, aid)
	if err != nil {
		return BatchCheckPermissionOutput{}, err
	}

	allowed := make([]bool, len(perms))
	for i, p := range perms {
		allowed[i] = len(matchPermissions(rows, p)) > 0
	}
	return BatchCheckPermissionOutput{Allowed: allowed}, nil
}

// ----------------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------------

// parseCheckInput validates UUIDs and the permission strings shared by
// CheckPermission / BatchCheckPermission.
func (s *Service) parseCheckInput(rawUser, rawApp string, rawPerms []string) (
	domain.UserID, domain.AppID, []string, error,
) {
	if len(rawPerms) == 0 {
		return "", "", nil, &validation.Error{Field: "permissions", Reason: "must contain at least one entry"}
	}
	if len(rawPerms) > bulkOpsCap {
		return "", "", nil, &validation.Error{Field: "permissions", Reason: "too many permissions in one request"}
	}

	uid, err := domain.ParseUserID(rawUser)
	if err != nil {
		return "", "", nil, err
	}
	aid, err := domain.ParseAppID(rawApp)
	if err != nil {
		return "", "", nil, err
	}
	for _, p := range rawPerms {
		if err := validatePermissionRequest(p); err != nil {
			return "", "", nil, err
		}
	}
	return uid, aid, rawPerms, nil
}

// validatePermissionRequest rejects malformed or wildcard request-side
// permissions. Wildcards ("users:*") are permitted only on the
// role-definition side; the request itself must be concrete.
func validatePermissionRequest(p string) error {
	if len(p) < 3 || len(p) > 64 {
		return &validation.Error{Field: "permission", Reason: "length must be between 3 and 64"}
	}
	colon := strings.IndexByte(p, ':')
	if colon <= 0 || colon == len(p)-1 {
		return &validation.Error{Field: "permission", Reason: "must be in resource:action form"}
	}
	resource, action := p[:colon], p[colon+1:]
	if !isLowerIdent(resource) || !isLowerIdent(action) {
		return &validation.Error{
			Field:  "permission",
			Reason: "resource and action must match [a-z][a-z0-9_]*",
		}
	}
	return nil
}

// isLowerIdent reports whether s matches `^[a-z][a-z0-9_]*$`. Avoids a
// regexp compile on the hot path (CheckPermission is high-QPS).
func isLowerIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
		case i > 0 && c >= '0' && c <= '9':
		case i > 0 && c == '_':
		default:
			return false
		}
	}
	return true
}

// matchPermissions returns the set of role_ids whose permission set
// satisfies the requested permission. A role permission matches when:
//   - it equals the request exactly (e.g. "users:read" == "users:read"); or
//   - it is "<resource>:*" and the request's resource matches.
//
// The caller already validated the request as concrete (no "*" on the
// request side), so wildcard handling is one-directional.
func matchPermissions(rows []domain.PermissionRow, requested string) map[domain.RoleID]struct{} {
	out := map[domain.RoleID]struct{}{}

	colon := strings.IndexByte(requested, ':')
	resource := requested
	if colon > 0 {
		resource = requested[:colon]
	}

	for _, r := range rows {
		if r.Permission == requested {
			out[r.RoleID] = struct{}{}
			continue
		}
		// Wildcard: "<resource>:*" matches any concrete action under
		// the same resource.
		if strings.HasSuffix(r.Permission, ":*") {
			rolePerm := r.Permission[:len(r.Permission)-2]
			if rolePerm == resource {
				out[r.RoleID] = struct{}{}
			}
		}
	}
	return out
}
