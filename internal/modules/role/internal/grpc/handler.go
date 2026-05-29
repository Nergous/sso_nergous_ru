// Package grpcadapter is the gRPC adapter for sso.roles.v1.RolesService.
// It is the only place where proto types meet domain types; both
// directions of mapping live alongside the handler in this package
// (mapper.go).
package grpcadapter

import (
	"context"
	"log/slog"

	"sso/internal/kernel/validation"
	"sso/internal/modules/role/internal/domain"
	rolesvc "sso/internal/modules/role/internal/service"

	ssorolesv1 "github.com/Nergous/sso_protos/gen/go/sso/roles/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Handler implements ssorolesv1.RolesServiceServer.
type Handler struct {
	ssorolesv1.UnimplementedRolesServiceServer

	svc *rolesvc.Service
	log *slog.Logger
}

func NewHandler(svc *rolesvc.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register attaches the handler to a gRPC server. Used by bootstrap as a
// registrar passed to grpcserver.New.
func (h *Handler) RegisterServer(s *grpc.Server) {
	ssorolesv1.RegisterRolesServiceServer(s, h)
}

// ----------------------------------------------------------------------------
// CreateRole
// ----------------------------------------------------------------------------

func (h *Handler) CreateRole(ctx context.Context, req *ssorolesv1.CreateRoleRequest) (*ssorolesv1.Role, error) {
	in := req.GetRole()
	out, err := h.svc.CreateRole(ctx, rolesvc.CreateRoleInput{
		ParentAppID: req.GetParentAppId(),
		Name:        in.GetName(),
		Description: in.GetDescription(),
		Permissions: in.GetPermissions(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return RoleToProto(out), nil
}

// ----------------------------------------------------------------------------
// GetRole
// ----------------------------------------------------------------------------

func (h *Handler) GetRole(ctx context.Context, req *ssorolesv1.GetRoleRequest) (*ssorolesv1.Role, error) {
	r, err := h.svc.GetRole(ctx, req.GetRoleId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return RoleToProto(r), nil
}

// ----------------------------------------------------------------------------
// ListRoles
// ----------------------------------------------------------------------------

func (h *Handler) ListRoles(ctx context.Context, req *ssorolesv1.ListRolesRequest) (*ssorolesv1.ListRolesResponse, error) {
	statuses, err := protoStatusesToDomain(req.GetFilters().GetStatuses())
	if err != nil {
		return nil, toGRPCError(err)
	}

	out, err := h.svc.ListRoles(ctx, rolesvc.ListRolesInput{
		AppID:     req.GetFilters().GetAppId(),
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
		Search:    req.GetFilters().GetSearch(),
		Statuses:  statuses,
		OrderBy:   orderByFromProto(req.GetOrderBy()),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &ssorolesv1.ListRolesResponse{
		Roles:         make([]*ssorolesv1.Role, len(out.Roles)),
		NextPageToken: out.NextPageToken,
	}
	for i, r := range out.Roles {
		resp.Roles[i] = RoleToProto(r)
	}
	if out.TotalSize != nil {
		ts := int32(*out.TotalSize)
		resp.TotalSize = &ts
	}
	return resp, nil
}

func protoStatusesToDomain(in []ssorolesv1.RoleStatus) ([]domain.RoleStatus, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]domain.RoleStatus, 0, len(in))
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
// UpdateRole
// ----------------------------------------------------------------------------

func (h *Handler) UpdateRole(ctx context.Context, req *ssorolesv1.UpdateRoleRequest) (*ssorolesv1.Role, error) {
	mask := req.GetUpdateMask()
	r := req.GetRole()

	in := rolesvc.UpdateRoleInput{
		RoleID:       req.GetRoleId(),
		MaskPaths:    mask.GetPaths(),
		ExpectedEtag: req.GetEtag(),
		Name:         r.GetName(),
		Description:  r.GetDescription(),
		Permissions:  r.GetPermissions(),
	}
	updated, err := h.svc.UpdateRole(ctx, in)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return RoleToProto(updated), nil
}

// ----------------------------------------------------------------------------
// Lifecycle: Disable / Enable
// ----------------------------------------------------------------------------

func (h *Handler) DisableRole(ctx context.Context, req *ssorolesv1.DisableRoleRequest) (*emptypb.Empty, error) {
	if err := h.svc.DisableRole(ctx, rolesvc.DisableRoleInput{
		RoleID:       req.GetRoleId(),
		AllowMissing: req.GetAllowMissing(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) EnableRole(ctx context.Context, req *ssorolesv1.EnableRoleRequest) (*emptypb.Empty, error) {
	if err := h.svc.EnableRole(ctx, rolesvc.EnableRoleInput{
		RoleID:       req.GetRoleId(),
		AllowMissing: req.GetAllowMissing(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ----------------------------------------------------------------------------
// PermanentlyDeleteRole
// ----------------------------------------------------------------------------

func (h *Handler) PermanentlyDeleteRole(ctx context.Context, req *ssorolesv1.PermanentlyDeleteRoleRequest) (*emptypb.Empty, error) {
	if err := h.svc.PermanentlyDeleteRole(ctx, rolesvc.PermanentlyDeleteRoleInput{
		RoleID:       req.GetRoleId(),
		ExpectedEtag: req.GetEtag(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}
