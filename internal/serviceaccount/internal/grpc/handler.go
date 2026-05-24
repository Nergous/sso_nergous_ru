// Package serviceAccount is the gRPC adapter for sso.serviceaccount.v1.ServiceAccountService.
package grpcadapter

import (
	"context"
	"log/slog"

	domain "sso/internal/serviceaccount/internal/domain"
	"sso/internal/kernel/validation"
	sasvc "sso/internal/serviceaccount/internal/service"

	ssosav1 "github.com/Nergous/sso_protos/gen/go/sso/serviceaccount/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Handler struct {
	ssosav1.UnimplementedServiceAccountServiceServer

	svc *sasvc.Service
	log *slog.Logger
}

func NewHandler(svc *sasvc.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) RegisterServer(s *grpc.Server) {
	ssosav1.RegisterServiceAccountServiceServer(s, h)
}

// ----------------------------------------------------------------------------
// CreateServiceAccount
// ----------------------------------------------------------------------------

func (h *Handler) CreateServiceAccount(ctx context.Context, req *ssosav1.CreateServiceAccountRequest) (*ssosav1.CreateServiceAccountResponse, error) {
	in := req.GetServiceAccount()
	out, err := h.svc.CreateServiceAccount(ctx, sasvc.CreateServiceAccountInput{
		Name:        in.GetName(),
		Description: in.GetDescription(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssosav1.CreateServiceAccountResponse{
		Credentials: credsToProto(out.Account, out.ClientSecret, out.IssuedAt.Unix()),
	}, nil
}

// ----------------------------------------------------------------------------
// GetServiceAccount
// ----------------------------------------------------------------------------

func (h *Handler) GetServiceAccount(ctx context.Context, req *ssosav1.GetServiceAccountRequest) (*ssosav1.ServiceAccount, error) {
	sa, err := h.svc.GetServiceAccount(ctx, req.GetServiceAccountId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return saToProto(sa), nil
}

// ----------------------------------------------------------------------------
// ListServiceAccounts
// ----------------------------------------------------------------------------

func (h *Handler) ListServiceAccounts(ctx context.Context, req *ssosav1.ListServiceAccountsRequest) (*ssosav1.ListServiceAccountsResponse, error) {
	statuses, err := protoStatusesToDomain(req.GetFilters().GetStatuses())
	if err != nil {
		return nil, toGRPCError(err)
	}

	out, err := h.svc.ListServiceAccounts(ctx, sasvc.ListServiceAccountsInput{
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
		Search:    req.GetFilters().GetSearch(),
		Statuses:  statuses,
		OrderBy:   orderByFromProto(req.GetOrderBy()),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &ssosav1.ListServiceAccountsResponse{
		ServiceAccounts: make([]*ssosav1.ServiceAccount, len(out.ServiceAccounts)),
		NextPageToken:   out.NextPageToken,
	}
	for i, sa := range out.ServiceAccounts {
		resp.ServiceAccounts[i] = saToProto(sa)
	}
	if out.TotalSize != nil {
		ts := int32(*out.TotalSize)
		resp.TotalSize = &ts
	}
	return resp, nil
}

func protoStatusesToDomain(in []ssosav1.ServiceAccountStatus) ([]domain.ServiceAccountStatus, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]domain.ServiceAccountStatus, 0, len(in))
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
// UpdateServiceAccount
// ----------------------------------------------------------------------------

func (h *Handler) UpdateServiceAccount(ctx context.Context, req *ssosav1.UpdateServiceAccountRequest) (*ssosav1.ServiceAccount, error) {
	sa := req.GetServiceAccount()
	updated, err := h.svc.UpdateServiceAccount(ctx, sasvc.UpdateServiceAccountInput{
		ServiceAccountID: req.GetServiceAccountId(),
		MaskPaths:        req.GetUpdateMask().GetPaths(),
		ExpectedEtag:     req.GetEtag(),
		Name:             sa.GetName(),
		Description:      sa.GetDescription(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return saToProto(updated), nil
}

// ----------------------------------------------------------------------------
// RotateCredentials
// ----------------------------------------------------------------------------

func (h *Handler) RotateCredentials(ctx context.Context, req *ssosav1.RotateCredentialsRequest) (*ssosav1.ServiceAccountCredentials, error) {
	out, err := h.svc.RotateCredentials(ctx, sasvc.RotateCredentialsInput{
		ServiceAccountID: req.GetServiceAccountId(),
		ExpectedEtag:     req.GetEtag(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return credsToProto(out.Account, out.ClientSecret, out.IssuedAt.Unix()), nil
}

// ----------------------------------------------------------------------------
// Lifecycle
// ----------------------------------------------------------------------------

func (h *Handler) DisableServiceAccount(ctx context.Context, req *ssosav1.DisableServiceAccountRequest) (*emptypb.Empty, error) {
	if err := h.svc.DisableServiceAccount(ctx, sasvc.DisableServiceAccountInput{
		ServiceAccountID: req.GetServiceAccountId(),
		AllowMissing:     req.GetAllowMissing(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) EnableServiceAccount(ctx context.Context, req *ssosav1.EnableServiceAccountRequest) (*emptypb.Empty, error) {
	if err := h.svc.EnableServiceAccount(ctx, sasvc.EnableServiceAccountInput{
		ServiceAccountID: req.GetServiceAccountId(),
		AllowMissing:     req.GetAllowMissing(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ----------------------------------------------------------------------------
// PermanentlyDeleteServiceAccount
// ----------------------------------------------------------------------------

func (h *Handler) PermanentlyDeleteServiceAccount(ctx context.Context, req *ssosav1.PermanentlyDeleteServiceAccountRequest) (*emptypb.Empty, error) {
	if err := h.svc.PermanentlyDeleteServiceAccount(ctx, sasvc.PermanentlyDeleteServiceAccountInput{
		ServiceAccountID: req.GetServiceAccountId(),
		ExpectedEtag:     req.GetEtag(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}
