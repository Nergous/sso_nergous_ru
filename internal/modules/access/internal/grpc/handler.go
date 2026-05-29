// Package grpcadapter is the gRPC adapter for sso.access.v1.AccessService.
// It is the only place where proto types meet domain types; both
// directions of mapping live alongside the handler in this package
// (mapper.go).
package grpcadapter

import (
	"context"
	"log/slog"

	accesssvc "sso/internal/modules/access/internal/service"
	"sso/internal/kernel/actor"

	ssoaccessv1 "github.com/Nergous/sso_protos/gen/go/sso/access/v1"
	ssorolesv1 "github.com/Nergous/sso_protos/gen/go/sso/roles/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// actorID extracts the caller's subject id from ctx (populated by the
// grpcauth interceptor). Returns "" when the request is anonymous or
// the interceptor was not wired — usecase ParseActorID accepts that as
// "unknown actor" so the audit field simply stays empty.
//
// Service-account callers also surface here: granted_by_user_id is
// intentionally a single opaque actor id (see domain/access.ActorID),
// so a backend identity granting a role is recorded just like a human.
func actorID(ctx context.Context) string {
	if a, ok := actor.From(ctx); ok {
		return a.ID
	}
	return ""
}

type Handler struct {
	ssoaccessv1.UnimplementedAccessServiceServer

	svc *accesssvc.Service
	log *slog.Logger
}

func NewHandler(svc *accesssvc.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) RegisterServer(s *grpc.Server) {
	ssoaccessv1.RegisterAccessServiceServer(s, h)
}

// ----------------------------------------------------------------------------
// HasRoleInApp
// ----------------------------------------------------------------------------

func (h *Handler) HasRoleInApp(ctx context.Context, req *ssoaccessv1.HasRoleInAppRequest) (*ssoaccessv1.HasRoleInAppResponse, error) {
	has, err := h.svc.HasRoleInApp(ctx, accesssvc.HasRoleInAppInput{
		UserID: req.GetUserId(),
		RoleID: req.GetRoleId(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssoaccessv1.HasRoleInAppResponse{HasRole: has}, nil
}

// ----------------------------------------------------------------------------
// ListUserRoles
// ----------------------------------------------------------------------------

func (h *Handler) ListUserRoles(ctx context.Context, req *ssoaccessv1.ListUserRolesRequest) (*ssoaccessv1.ListUserRolesResponse, error) {
	out, err := h.svc.ListUserRoles(ctx, accesssvc.ListUserRolesInput{
		UserID:    req.GetUserId(),
		AppID:     req.GetAppId(),
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
		OrderBy:   orderByFromProto(req.GetOrderBy()),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &ssoaccessv1.ListUserRolesResponse{
		Roles:         make([]*ssorolesv1.Role, len(out.Roles)),
		NextPageToken: out.NextPageToken,
	}
	for i, r := range out.Roles {
		resp.Roles[i] = roleToProto(r)
	}
	if out.TotalSize != nil {
		ts := int32(*out.TotalSize)
		resp.TotalSize = &ts
	}
	return resp, nil
}

// ----------------------------------------------------------------------------
// GrantRoleToUser
// ----------------------------------------------------------------------------

func (h *Handler) GrantRoleToUser(ctx context.Context, req *ssoaccessv1.GrantRoleToUserRequest) (*ssoaccessv1.RoleAssignment, error) {
	out, err := h.svc.GrantRoleToUser(ctx, accesssvc.GrantRoleToUserInput{
		UserID:  req.GetUserId(),
		RoleID:  req.GetRoleId(),
		ActorID: actorID(ctx),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return assignmentToProto(out.Assignment), nil
}

// ----------------------------------------------------------------------------
// RemoveRoleFromUser
// ----------------------------------------------------------------------------

func (h *Handler) RemoveRoleFromUser(ctx context.Context, req *ssoaccessv1.RemoveRoleFromUserRequest) (*emptypb.Empty, error) {
	if err := h.svc.RemoveRoleFromUser(ctx, accesssvc.RemoveRoleFromUserInput{
		UserID: req.GetUserId(),
		RoleID: req.GetRoleId(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ----------------------------------------------------------------------------
// BulkGrantRoles
// ----------------------------------------------------------------------------

func (h *Handler) BulkGrantRoles(ctx context.Context, req *ssoaccessv1.BulkGrantRolesRequest) (*ssoaccessv1.BulkGrantRolesResponse, error) {
	out, err := h.svc.BulkGrantRoles(ctx, accesssvc.BulkGrantRolesInput{
		UserID:  req.GetUserId(),
		AppID:   req.GetAppId(),
		RoleIDs: req.GetRoleIds(),
		ActorID: actorID(ctx),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	resp := &ssoaccessv1.BulkGrantRolesResponse{
		RoleAssignments: make([]*ssoaccessv1.RoleAssignment, len(out.Assignments)),
		NewlyCreated:    out.Created,
	}
	for i, a := range out.Assignments {
		resp.RoleAssignments[i] = assignmentToProto(a)
	}
	return resp, nil
}

// ----------------------------------------------------------------------------
// BulkRemoveRoles
// ----------------------------------------------------------------------------

func (h *Handler) BulkRemoveRoles(ctx context.Context, req *ssoaccessv1.BulkRemoveRolesRequest) (*emptypb.Empty, error) {
	if err := h.svc.BulkRemoveRoles(ctx, accesssvc.BulkRemoveRolesInput{
		UserID:  req.GetUserId(),
		AppID:   req.GetAppId(),
		RoleIDs: req.GetRoleIds(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ----------------------------------------------------------------------------
// CheckPermission
// ----------------------------------------------------------------------------

func (h *Handler) CheckPermission(ctx context.Context, req *ssoaccessv1.CheckPermissionRequest) (*ssoaccessv1.CheckPermissionResponse, error) {
	out, err := h.svc.CheckPermission(ctx, accesssvc.CheckPermissionInput{
		UserID:     req.GetUserId(),
		AppID:      req.GetAppId(),
		Permission: req.GetPermission(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssoaccessv1.CheckPermissionResponse{
		Allowed:        out.Allowed,
		MatchedRoleIds: out.MatchedRoleIDs,
	}, nil
}

// ----------------------------------------------------------------------------
// BatchCheckPermission
// ----------------------------------------------------------------------------

func (h *Handler) BatchCheckPermission(ctx context.Context, req *ssoaccessv1.BatchCheckPermissionRequest) (*ssoaccessv1.BatchCheckPermissionResponse, error) {
	out, err := h.svc.BatchCheckPermission(ctx, accesssvc.BatchCheckPermissionInput{
		UserID:      req.GetUserId(),
		AppID:       req.GetAppId(),
		Permissions: req.GetPermissions(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssoaccessv1.BatchCheckPermissionResponse{Allowed: out.Allowed}, nil
}
