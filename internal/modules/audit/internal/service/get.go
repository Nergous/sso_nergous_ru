package service

import (
	"context"

	domain "sso/internal/modules/audit/internal/domain"
)

// GetAuditEvent fetches a single event by UUID.
//
// Authorization is checked before the lookup so an unauthorised caller
// cannot probe for existence — a NOT_FOUND vs PERMISSION_DENIED leak
// would let outsiders enumerate event ids.
func (s *Service) GetAuditEvent(ctx context.Context, rawID string) (*domain.Audit, error) {
	if err := s.requireAuditRead(ctx); err != nil {
		return nil, err
	}
	id, err := domain.ParseAuditID(rawID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// requireAuditRead is the shared authz gate. Use-cases call it before
// touching the repo.
func (s *Service) requireAuditRead(ctx context.Context) error {
	ok, err := (*s.authz.Load()).CanReadAudit(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPermissionDenied
	}
	return nil
}
