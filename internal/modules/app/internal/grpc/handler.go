// Package app is the gRPC adapter for sso.app.v1.AppService.
package app

import (
	"context"
	"log/slog"

	"sso/internal/modules/app/internal/domain"
	"sso/internal/modules/app/internal/service"
	"sso/internal/kernel/validation"

	ssoappv1 "github.com/Nergous/sso_protos/gen/go/sso/app/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Handler implements ssoappv1.AppServiceServer.
type Handler struct {
	ssoappv1.UnimplementedAppServiceServer

	svc *service.Service
	log *slog.Logger
}

func NewHandler(svc *service.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register attaches the handler to a gRPC server. Used by bootstrap as a
// registrar passed to grpcserver.New.
func (h *Handler) RegisterServer(s *grpc.Server) {
	ssoappv1.RegisterAppServiceServer(s, h)
}

// ----------------------------------------------------------------------------
// CreateApp
// ----------------------------------------------------------------------------

func (h *Handler) CreateApp(ctx context.Context, req *ssoappv1.CreateAppRequest) (*ssoappv1.App, error) {
	in := req.GetApp()
	out, err := h.svc.CreateApp(ctx, service.CreateAppInput{
		Name: in.GetName(),
		Slug: in.GetSlug(),
		Link: in.GetLink(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return appToProto(out), nil
}

// ----------------------------------------------------------------------------
// GetApp
// ----------------------------------------------------------------------------

func (h *Handler) GetApp(ctx context.Context, req *ssoappv1.GetAppRequest) (*ssoappv1.App, error) {
	a, err := h.svc.GetApp(ctx, req.GetAppId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return appToProto(a), nil
}

// ----------------------------------------------------------------------------
// ListApps
// ----------------------------------------------------------------------------

func (h *Handler) ListApps(ctx context.Context, req *ssoappv1.ListAppsRequest) (*ssoappv1.ListAppsResponse, error) {
	statuses, err := protoStatusesToDomain(req.GetFilters().GetStatuses())
	if err != nil {
		return nil, toGRPCError(err)
	}

	out, err := h.svc.ListApps(ctx, service.ListAppsInput{
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
		Search:    req.GetFilters().GetSearch(),
		Statuses:  statuses,
		OrderBy:   orderByFromProto(req.GetOrderBy()),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &ssoappv1.ListAppsResponse{
		Apps:          make([]*ssoappv1.App, len(out.Apps)),
		NextPageToken: out.NextPageToken,
	}
	for i, a := range out.Apps {
		resp.Apps[i] = appToProto(a)
	}
	if out.TotalSize != nil {
		ts := int32(*out.TotalSize)
		resp.TotalSize = &ts
	}
	return resp, nil
}

func protoStatusesToDomain(in []ssoappv1.AppStatus) ([]domain.AppStatus, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]domain.AppStatus, 0, len(in))
	for _, s := range in {
		dom, ok := statusFromProto(s)
		if !ok {
			return nil, &validation.Error{Field: "filters.statuses", Reason: "unknown status value"}
		}
		out = append(out, dom)
	}
	return out, nil
}

// ----------------------------------------------------------------------------
// UpdateApp
// ----------------------------------------------------------------------------

func (h *Handler) UpdateApp(ctx context.Context, req *ssoappv1.UpdateAppRequest) (*ssoappv1.App, error) {
	mask := req.GetUpdateMask()
	a := req.GetApp()

	in := service.UpdateAppInput{
		AppID:        req.GetAppId(),
		MaskPaths:    mask.GetPaths(),
		ExpectedEtag: req.GetEtag(),
		Name:         a.GetName(),
		Link:         a.GetLink(),
	}
	updated, err := h.svc.UpdateApp(ctx, in)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return appToProto(updated), nil
}

// ----------------------------------------------------------------------------
// Lifecycle: Disable / Enable / EnterMaintenance / ExitMaintenance
// ----------------------------------------------------------------------------

func (h *Handler) DisableApp(ctx context.Context, req *ssoappv1.DisableAppRequest) (*emptypb.Empty, error) {
	if err := h.svc.DisableApp(ctx, service.DisableAppInput{
		AppID:        req.GetAppId(),
		AllowMissing: req.GetAllowMissing(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) EnableApp(ctx context.Context, req *ssoappv1.EnableAppRequest) (*emptypb.Empty, error) {
	if err := h.svc.EnableApp(ctx, service.EnableAppInput{
		AppID:        req.GetAppId(),
		AllowMissing: req.GetAllowMissing(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) EnterMaintenanceMode(ctx context.Context, req *ssoappv1.EnterMaintenanceModeRequest) (*emptypb.Empty, error) {
	if err := h.svc.EnterMaintenanceMode(ctx, service.EnterMaintenanceModeInput{
		AppID:        req.GetAppId(),
		AllowMissing: req.GetAllowMissing(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) ExitMaintenanceMode(ctx context.Context, req *ssoappv1.ExitMaintenanceModeRequest) (*emptypb.Empty, error) {
	if err := h.svc.ExitMaintenanceMode(ctx, service.ExitMaintenanceModeInput{
		AppID:        req.GetAppId(),
		AllowMissing: req.GetAllowMissing(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ----------------------------------------------------------------------------
// PermanentlyDeleteApp
// ----------------------------------------------------------------------------

func (h *Handler) PermanentlyDeleteApp(ctx context.Context, req *ssoappv1.PermanentlyDeleteAppRequest) (*emptypb.Empty, error) {
	if err := h.svc.PermanentlyDeleteApp(ctx, service.PermanentlyDeleteAppInput{
		AppID:        req.GetAppId(),
		ExpectedEtag: req.GetEtag(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}
