// Package grpcadapter is the gRPC adapter for sso.identity.v1.IdentityService.
// It is the only place where proto types meet domain types; both directions
// of mapping live alongside the handler in this package (mapper.go).
package grpcadapter

import (
	"context"
	"log/slog"

	"sso/internal/modules/identity/internal/domain"
	identityapp "sso/internal/modules/identity/internal/service"
	"sso/internal/kernel/validation"

	ssoidentityv1 "github.com/Nergous/sso_protos/gen/go/sso/identity/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Handler implements ssoidentityv1.IdentityServiceServer.
type Handler struct {
	ssoidentityv1.UnimplementedIdentityServiceServer

	svc *identityapp.Service
	log *slog.Logger
}

// NewHandler wires the gRPC handler to the application-layer service.
func NewHandler(svc *identityapp.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register attaches the handler to a gRPC server. Used by bootstrap as a
// registrar passed to grpcserver.New.
func (h *Handler) RegisterServer(s *grpc.Server) {
	ssoidentityv1.RegisterIdentityServiceServer(s, h)
}

// ----------------------------------------------------------------------------
// CreateUser
// ----------------------------------------------------------------------------

func (h *Handler) CreateUser(ctx context.Context, req *ssoidentityv1.CreateUserRequest) (*ssoidentityv1.User, error) {
	u := req.GetUser()
	in := identityapp.CreateUserInput{
		Email:       u.GetEmail(),
		Username:    u.GetUsername(),
		DisplayName: u.GetDisplayName(),
		AvatarURL:   u.GetAvatarUrl(), // proto3 optional: zero-string when unset
		Locale:      u.GetLocale(),
		Timezone:    u.GetTimezone(),
	}
	user, err := h.svc.CreateUser(ctx, in)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return UserToProto(user), nil
}

// ----------------------------------------------------------------------------
// GetUser
// ----------------------------------------------------------------------------

func (h *Handler) GetUser(ctx context.Context, req *ssoidentityv1.GetUserRequest) (*ssoidentityv1.User, error) {
	user, err := h.svc.GetUser(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return UserToProto(user), nil
}

// ----------------------------------------------------------------------------
// ListUsers
// ----------------------------------------------------------------------------

func (h *Handler) ListUsers(ctx context.Context, req *ssoidentityv1.ListUsersRequest) (*ssoidentityv1.ListUsersResponse, error) {
	statuses, err := protoStatusesToDomain(req.GetFilters().GetStatuses())
	if err != nil {
		return nil, toGRPCError(err)
	}

	out, err := h.svc.ListUsers(ctx, identityapp.ListUsersInput{
		PageSize:     req.GetPageSize(),
		PageToken:    req.GetPageToken(),
		Search:       req.GetFilters().GetSearch(),
		Emails:       req.GetFilters().GetEmails(),
		Usernames:    req.GetFilters().GetUsernames(),
		DisplayNames: req.GetFilters().GetDisplayNames(),
		Statuses:     statuses,
		OrderBy:      orderByFromProto(req.GetOrderBy()),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &ssoidentityv1.ListUsersResponse{
		Users:         make([]*ssoidentityv1.User, len(out.Users)),
		NextPageToken: out.NextPageToken,
	}
	for i, u := range out.Users {
		resp.Users[i] = UserToProto(u)
	}
	if out.TotalSize != nil {
		ts := int32(*out.TotalSize)
		resp.TotalSize = &ts
	}
	return resp, nil
}

// protoStatusesToDomain validates and converts the repeated UserStatus
// filter from the wire. UNSPECIFIED is an invalid filter value and surfaces
// as a ValidationError.
func protoStatusesToDomain(in []ssoidentityv1.UserStatus) ([]domain.UserStatus, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]domain.UserStatus, 0, len(in))
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
// UpdateUser
// ----------------------------------------------------------------------------

func (h *Handler) UpdateUser(ctx context.Context, req *ssoidentityv1.UpdateUserRequest) (*ssoidentityv1.User, error) {
	mask := req.GetUpdateMask()
	user := req.GetUser() // may be nil if client omitted; handled by Get* defaults

	in := identityapp.UpdateUserInput{
		UserID:       req.GetUserId(),
		MaskPaths:    mask.GetPaths(),
		ExpectedEtag: req.GetEtag(),
		Email:        user.GetEmail(),
		Username:     user.GetUsername(),
		DisplayName:  user.GetDisplayName(),
		AvatarURL:    user.GetAvatarUrl(),
		Locale:       user.GetLocale(),
		Timezone:     user.GetTimezone(),
	}

	updated, err := h.svc.UpdateUser(ctx, in)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return UserToProto(updated), nil
}

// ----------------------------------------------------------------------------
// DisableUser / EnableUser / SoftDeleteUser
// ----------------------------------------------------------------------------

func (h *Handler) DisableUser(ctx context.Context, req *ssoidentityv1.DisableUserRequest) (*emptypb.Empty, error) {
	if err := h.svc.DisableUser(ctx, identityapp.DisableUserInput{
		UserID:       req.GetUserId(),
		AllowMissing: req.GetAllowMissing(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) EnableUser(ctx context.Context, req *ssoidentityv1.EnableUserRequest) (*emptypb.Empty, error) {
	if err := h.svc.EnableUser(ctx, identityapp.EnableUserInput{
		UserID:       req.GetUserId(),
		AllowMissing: req.GetAllowMissing(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) SoftDeleteUser(ctx context.Context, req *ssoidentityv1.SoftDeleteUserRequest) (*emptypb.Empty, error) {
	if err := h.svc.SoftDeleteUser(ctx, identityapp.SoftDeleteUserInput{
		UserID:       req.GetUserId(),
		AllowMissing: req.GetAllowMissing(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ----------------------------------------------------------------------------
// PermanentlyDeleteUser
// ----------------------------------------------------------------------------

func (h *Handler) PermanentlyDeleteUser(ctx context.Context, req *ssoidentityv1.PermanentlyDeleteUserRequest) (*emptypb.Empty, error) {
	if err := h.svc.PermanentlyDeleteUser(ctx, identityapp.PermanentlyDeleteUserInput{
		UserID:       req.GetUserId(),
		ExpectedEtag: req.GetEtag(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}
