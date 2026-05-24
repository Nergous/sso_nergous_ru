// Package audit is the gRPC adapter for sso.audit.v1.AuditService.
//
// The service is strictly read-only: writes happen as a side-effect of
// other RPCs through auditbus.Emitter. No public RPC bypass — every
// method requires an authenticated caller (the authz check itself sits
// in usecase/audit).
package grpcadapter

import (
	"context"
	"log/slog"

	auditsvc "sso/internal/audit/internal/service"

	ssoauditv1 "github.com/Nergous/sso_protos/gen/go/sso/audit/v1"
	"google.golang.org/grpc"
)

type Handler struct {
	ssoauditv1.UnimplementedAuditServiceServer

	svc *auditsvc.Service
	log *slog.Logger
}

func NewHandler(svc *auditsvc.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) RegisterServer(s *grpc.Server) {
	ssoauditv1.RegisterAuditServiceServer(s, h)
}

// ----------------------------------------------------------------------------
// GetAuditEvent
// ----------------------------------------------------------------------------

func (h *Handler) GetAuditEvent(ctx context.Context, req *ssoauditv1.GetAuditEventRequest) (*ssoauditv1.AuditEvent, error) {
	a, err := h.svc.GetAuditEvent(ctx, req.GetEventId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return auditToProto(a), nil
}

// ----------------------------------------------------------------------------
// ListAuditEvents
// ----------------------------------------------------------------------------

func (h *Handler) ListAuditEvents(ctx context.Context, req *ssoauditv1.ListAuditEventsRequest) (*ssoauditv1.ListAuditEventsResponse, error) {
	out, err := h.svc.ListAuditEvents(ctx, auditsvc.ListAuditEventsInput{
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
		Filters:   filtersFromProto(req.GetFilters()),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &ssoauditv1.ListAuditEventsResponse{
		Events:        make([]*ssoauditv1.AuditEvent, len(out.Audits)),
		NextPageToken: out.NextPageToken,
	}
	for i, a := range out.Audits {
		resp.Events[i] = auditToProto(a)
	}
	if out.TotalSize != nil {
		ts := int32(*out.TotalSize)
		resp.TotalSize = &ts
	}
	return resp, nil
}
