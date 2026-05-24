package service

import (
	"context"

	"sso/internal/audit"
	"sso/internal/audit/auditx"
	"sso/internal/identity/internal/domain"
	"sso/internal/kernel/actor"
	"sso/internal/kernel/validation"
)

// ----------------------------------------------------------------------------
// UpdateUser
// ----------------------------------------------------------------------------

// UpdateUserInput is the parsed UpdateUserRequest. The handler interprets
// update_mask: only the entries present in MaskPaths are read off the User
// payload. Everything else is ignored.
//
// Allowed mask paths (anything else surfaces ValidationError):
//
//	email, username, display_name, avatar_url, locale, timezone
type UpdateUserInput struct {
	UserID       string
	MaskPaths    []string
	ExpectedEtag string // "*" wildcard or a UUID

	Email       string
	Username    string
	DisplayName string
	AvatarURL   string
	Locale      string
	Timezone    string
}

// UpdateUser applies a FieldMask-driven partial update.
//
// Errors:
//
//	ValidationError       — empty mask, unknown mask path
//	ErrUserNotFound       — no row for user_id
//	ErrEtagMismatch       — supplied etag != current
//	ErrUserDeleted        — target is in DELETED status
//	ErrUserAlreadyExists  — patched email/username collides
func (s *Service) UpdateUser(ctx context.Context, in UpdateUserInput) (*domain.User, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.ParseUserID(in.UserID)
	if err != nil {
		return nil, err
	}

	expectedEtag, err := auditx.ParseExpectedEtag(in.ExpectedEtag, true /*required*/)
	if err != nil {
		return nil, err
	}

	if len(in.MaskPaths) == 0 {
		return nil, &validation.Error{Field: "update_mask", Reason: "must list at least one field"}
	}

	patch, err := buildPatch(in)
	if err != nil {
		return nil, err
	}
	if patch.IsEmpty() {
		// Defensive: buildPatch already rejects unknown paths. This branch
		// only fires if MaskPaths somehow becomes non-empty but contributes
		// no fields — currently impossible.
		return nil, &validation.Error{Field: "update_mask", Reason: "no applicable fields"}
	}

	aud := audit.BaseFromActor(a, audit.EventTypeIdentityUpdateUser)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = id.String()

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}

	if err := user.ApplyPatch(patch, s.now().UTC()); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}

	if err := s.repo.Update(ctx, user, expectedEtag); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}

	s.auditor.Success(ctx, aud)
	return user, nil
}

// buildPatch translates (mask, raw values) into a typed UserPatch. Each
// recognised mask entry copies the matching input field into the patch;
// an unknown entry rejects the whole request.
func buildPatch(in UpdateUserInput) (domain.UserPatch, error) {
	var p domain.UserPatch
	for _, path := range in.MaskPaths {
		switch path {
		case "email":
			v := in.Email
			p.Email = &v
		case "username":
			v := in.Username
			p.Username = &v
		case "display_name":
			v := in.DisplayName
			p.DisplayName = &v
		case "avatar_url":
			v := in.AvatarURL
			p.AvatarURL = &v
		case "locale":
			v := in.Locale
			p.Locale = &v
		case "timezone":
			v := in.Timezone
			p.Timezone = &v
		default:
			return domain.UserPatch{}, &validation.Error{
				Field:  "update_mask",
				Reason: "unknown field path: " + path,
			}
		}
	}
	return p, nil
}

// ----------------------------------------------------------------------------
// DisableUser / EnableUser — status transitions
// ----------------------------------------------------------------------------

// DisableUserInput is the parsed DisableUserRequest.
type DisableUserInput struct {
	UserID       string
	AllowMissing bool
}

// DisableUser sets status to BLOCKED. Idempotent. AllowMissing converts
// ErrUserNotFound into a successful no-op (AIP-135). ErrUserDeleted always
// propagates — a deleted user cannot be re-disabled.
func (s *Service) DisableUser(ctx context.Context, in DisableUserInput) error {
	return s.lifecycleEmit(ctx, in.UserID, in.AllowMissing, audit.EventTypeIdentityDisableUser, func(u *domain.User) error {
		return u.Disable(s.now().UTC())
	})
}

// EnableUserInput is the parsed EnableUserRequest.
type EnableUserInput struct {
	UserID       string
	AllowMissing bool
}

// EnableUser sets status to ACTIVE. Idempotent. ErrUserDeleted on a deleted
// target (the proto contract is explicit: a deleted user cannot be revived
// via Enable).
func (s *Service) EnableUser(ctx context.Context, in EnableUserInput) error {
	return s.lifecycleEmit(ctx, in.UserID, in.AllowMissing, audit.EventTypeIdentityEnableUser, func(u *domain.User) error {
		return u.Enable(s.now().UTC())
	})
}
