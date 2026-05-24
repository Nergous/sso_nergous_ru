package grpcadapter

import (
	domain "sso/internal/serviceaccount/internal/domain"

	ssosav1 "github.com/Nergous/sso_protos/gen/go/sso/serviceaccount/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func saToProto(s *domain.ServiceAccount) *ssosav1.ServiceAccount {
	out := &ssosav1.ServiceAccount{
		ServiceAccountId: s.ID().String(),
		Name:             s.Name,
		Description:      s.Description,
		Status:           statusToProto(s.Status()),
		Etag:             s.Etag().String(),
		CreatedAt:        timestamppb.New(s.CreatedAt()),
		UpdatedAt:        timestamppb.New(s.UpdatedAt()),
	}
	if !s.LastAuthenticatedAt.IsZero() {
		out.LastAuthenticatedAt = timestamppb.New(s.LastAuthenticatedAt)
	}
	return out
}

func statusToProto(s domain.ServiceAccountStatus) ssosav1.ServiceAccountStatus {
	switch s {
	case domain.ServiceAccountActive:
		return ssosav1.ServiceAccountStatus_SERVICE_ACCOUNT_STATUS_ACTIVE
	case domain.ServiceAccountDisabled:
		return ssosav1.ServiceAccountStatus_SERVICE_ACCOUNT_STATUS_DISABLED
	}
	return ssosav1.ServiceAccountStatus_SERVICE_ACCOUNT_STATUS_UNSPECIFIED
}

func statusFromProto(s ssosav1.ServiceAccountStatus) (domain.ServiceAccountStatus, bool) {
	switch s {
	case ssosav1.ServiceAccountStatus_SERVICE_ACCOUNT_STATUS_ACTIVE:
		return domain.ServiceAccountActive, true
	case ssosav1.ServiceAccountStatus_SERVICE_ACCOUNT_STATUS_DISABLED:
		return domain.ServiceAccountDisabled, true
	}
	return 0, false
}

func orderByFromProto(o ssosav1.ListServiceAccountsOrderBy) domain.ListOrderBy {
	switch o {
	case ssosav1.ListServiceAccountsOrderBy_LIST_SERVICE_ACCOUNTS_ORDER_BY_CREATED_AT_DESC:
		return domain.OrderByCreatedAtDesc
	case ssosav1.ListServiceAccountsOrderBy_LIST_SERVICE_ACCOUNTS_ORDER_BY_CREATED_AT_ASC:
		return domain.OrderByCreatedAtAsc
	case ssosav1.ListServiceAccountsOrderBy_LIST_SERVICE_ACCOUNTS_ORDER_BY_SERVICE_ACCOUNT_ID_DESC:
		return domain.OrderByServiceAccountIDDesc
	case ssosav1.ListServiceAccountsOrderBy_LIST_SERVICE_ACCOUNTS_ORDER_BY_SERVICE_ACCOUNT_ID_ASC:
		return domain.OrderByServiceAccountIDAsc
	case ssosav1.ListServiceAccountsOrderBy_LIST_SERVICE_ACCOUNTS_ORDER_BY_NAME_DESC:
		return domain.OrderByNameDesc
	case ssosav1.ListServiceAccountsOrderBy_LIST_SERVICE_ACCOUNTS_ORDER_BY_NAME_ASC:
		return domain.OrderByNameAsc
	case ssosav1.ListServiceAccountsOrderBy_LIST_SERVICE_ACCOUNTS_ORDER_BY_LAST_AUTHENTICATED_AT_DESC:
		return domain.OrderByLastAuthenticatedAtDesc
	case ssosav1.ListServiceAccountsOrderBy_LIST_SERVICE_ACCOUNTS_ORDER_BY_LAST_AUTHENTICATED_AT_ASC:
		return domain.OrderByLastAuthenticatedAtAsc
	}
	return domain.OrderByUnspecified
}

// credsToProto renders ServiceAccountCredentials for Create / Rotate
// responses. Plaintext secret + issued_at are included (proto: shown
// once, debug_redact); the embedded Account carries the post-write
// state (fresh etag/updated_at after rotation).
func credsToProto(account *domain.ServiceAccount, plaintext string, issuedAt int64) *ssosav1.ServiceAccountCredentials {
	creds := &ssosav1.ServiceAccountCredentials{
		Account:      saToProto(account),
		ClientSecret: plaintext,
	}
	if issuedAt != 0 {
		creds.IssuedAt = timestamppb.New(account.UpdatedAt())
	}
	return creds
}
