// Package audit hosts the use-cases for sso.audit.v1.AuditService:
// read-only access to the append-only event log.
//
// File layout:
//
//	service.go — Service struct + constants.
//	authz.go   — AuditAuthorizer interface + default impls.
//	errors.go  — ErrPermissionDenied (other errors come from domain).
//	get.go     — GetAuditEvent.
//	list.go    — ListAuditEvents + page-cursor codec.
//
// The aggregate is append-only; write operations are surfaced through
// the auditbus.Emitter, not this service.
package service

import (
	domain "sso/internal/modules/audit/internal/domain"
	"sync/atomic"
)

// Service exposes the audit read use-cases.
type Service struct {
	repo  domain.Repository
	authz atomic.Pointer[AuditAuthorizer]
}

// NewService wires the read-side service. Both dependencies are
// required; nil panics at first use rather than at construction so
// wiring bugs surface in tests.
func NewService(repo domain.Repository, authz AuditAuthorizer) *Service {
	s := &Service{repo: repo}
	s.authz.Store(&authz)
	return s
}

func (s *Service) SetAuthorizer(a AuditAuthorizer) { s.authz.Store(&a) }
