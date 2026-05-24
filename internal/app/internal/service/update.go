package service

import (
	"context"

	"sso/internal/app/internal/domain"
	"sso/internal/audit"
	"sso/internal/audit/auditx"
	"sso/internal/kernel/actor"
	"sso/internal/kernel/validation"
)

// ----------------------------------------------------------------------------
// UpdateApp
// ----------------------------------------------------------------------------

// UpdateAppInput is the parsed UpdateAppRequest.
//
// Allowed mask paths (anything else surfaces ValidationError):
//
//	name, link
//
// Forbidden mask paths (per proto contract): app_id, slug, status, etag,
// created_at, updated_at — buildPatch's default branch rejects them as
// "unknown".
type UpdateAppInput struct {
	AppID        string
	MaskPaths    []string
	ExpectedEtag string

	Name string
	Link string
}

// UpdateApp applies a FieldMask-driven partial update.
//
// Errors:
//
//	ValidationError      — empty mask, unknown / forbidden mask path
//	ErrAppNotFound       — no row for app_id
//	ErrEtagMismatch      — supplied etag != current
//	ErrAppAlreadyExists  — patched name collides
func (s *Service) UpdateApp(ctx context.Context, in UpdateAppInput) (*domain.App, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.ParseAppID(in.AppID)
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
		return nil, &validation.Error{Field: "update_mask", Reason: "no applicable fields"}
	}

	aud := audit.BaseFromActor(a, audit.EventTypeAppUpdateApp)
	aud.SubjectType = audit.SubjectTypeApp
	aud.SubjectID = id.String()
	aud.AppID = id.String()

	target, err := s.repo.GetByID(ctx, id)
	if err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}

	target.ApplyPatch(patch, s.now().UTC())
	if err := s.repo.Update(ctx, target, expectedEtag); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}

	s.auditor.Success(ctx, aud)
	return target, nil
}

func buildPatch(in UpdateAppInput) (domain.AppPatch, error) {
	var p domain.AppPatch
	for _, path := range in.MaskPaths {
		switch path {
		case "name":
			v := in.Name
			p.Name = &v
		case "link":
			v := in.Link
			p.Link = &v
		default:
			return domain.AppPatch{}, &validation.Error{
				Field:  "update_mask",
				Reason: "unknown field path: " + path,
			}
		}
	}
	return p, nil
}

// ----------------------------------------------------------------------------
// Lifecycle transitions: Disable / Enable / EnterMaintenance / ExitMaintenance
// ----------------------------------------------------------------------------

type DisableAppInput struct {
	AppID        string
	AllowMissing bool
}

// DisableApp sets status to DISABLED. Permissive on starting state.
func (s *Service) DisableApp(ctx context.Context, in DisableAppInput) error {
	return s.lifecycleEmit(ctx, in.AppID, in.AllowMissing, audit.EventTypeAppDisableApp, func(a *domain.App) error {
		a.Disable(s.now().UTC())
		return nil
	})
}

type EnableAppInput struct {
	AppID        string
	AllowMissing bool
}

// EnableApp sets status to ACTIVE from DISABLED. Idempotent on ACTIVE.
// Rejects MAINTENANCE — caller must use ExitMaintenanceMode.
func (s *Service) EnableApp(ctx context.Context, in EnableAppInput) error {
	return s.lifecycleEmit(ctx, in.AppID, in.AllowMissing, audit.EventTypeAppEnableApp, func(a *domain.App) error {
		return a.Enable(s.now().UTC())
	})
}

type EnterMaintenanceModeInput struct {
	AppID        string
	AllowMissing bool
}

// EnterMaintenanceMode sets status to MAINTENANCE from ACTIVE.
// Idempotent on MAINTENANCE. Rejects DISABLED.
func (s *Service) EnterMaintenanceMode(ctx context.Context, in EnterMaintenanceModeInput) error {
	return s.lifecycleEmit(ctx, in.AppID, in.AllowMissing, audit.EventTypeAppEnterMaintenanceMode, func(a *domain.App) error {
		return a.EnterMaintenance(s.now().UTC())
	})
}

type ExitMaintenanceModeInput struct {
	AppID        string
	AllowMissing bool
}

// ExitMaintenanceMode sets status to ACTIVE from MAINTENANCE.
// Idempotent on ACTIVE. Rejects DISABLED.
func (s *Service) ExitMaintenanceMode(ctx context.Context, in ExitMaintenanceModeInput) error {
	return s.lifecycleEmit(ctx, in.AppID, in.AllowMissing, audit.EventTypeAppExitMaintenanceMode, func(a *domain.App) error {
		return a.ExitMaintenance(s.now().UTC())
	})
}
