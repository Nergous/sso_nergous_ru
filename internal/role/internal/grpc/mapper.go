package grpcadapter

import (
	"sso/internal/role/internal/domain"

	ssorolesv1 "github.com/Nergous/sso_protos/gen/go/sso/roles/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RoleToProto renders a domain.Role as a sso.roles.v1.Role. Exported so
// sibling gRPC adapters (e.g. internal/grpc/access for ListUserRoles)
// can reuse the canonical wire mapping without re-implementing it.
func RoleToProto(r *domain.Role) *ssorolesv1.Role {
	return &ssorolesv1.Role{
		RoleId:      r.ID().String(),
		AppId:       r.AppID().String(),
		Name:        r.Name,
		Description: r.Description,
		Permissions: r.Permissions(),
		Status:      statusToProto(r.Status()),
		Etag:        r.Etag().String(),
		CreatedAt:   timestamppb.New(r.CreatedAt()),
		UpdatedAt:   timestamppb.New(r.UpdatedAt()),
	}
}

func statusToProto(s domain.RoleStatus) ssorolesv1.RoleStatus {
	switch s {
	case domain.RoleStatusActive:
		return ssorolesv1.RoleStatus_ROLE_STATUS_ACTIVE
	case domain.RoleStatusDisabled:
		return ssorolesv1.RoleStatus_ROLE_STATUS_DISABLED
	}
	return ssorolesv1.RoleStatus_ROLE_STATUS_UNSPECIFIED
}

// statusFromProto maps the proto enum onto the domain enum. Returns
// (zero, false) for UNSPECIFIED or unknown values; callers should
// reject the request with INVALID_ARGUMENT in that case.
func statusFromProto(s ssorolesv1.RoleStatus) (domain.RoleStatus, bool) {
	switch s {
	case ssorolesv1.RoleStatus_ROLE_STATUS_ACTIVE:
		return domain.RoleStatusActive, true
	case ssorolesv1.RoleStatus_ROLE_STATUS_DISABLED:
		return domain.RoleStatusDisabled, true
	}
	return 0, false
}

func orderByFromProto(o ssorolesv1.ListRolesOrderBy) domain.ListOrderBy {
	switch o {
	case ssorolesv1.ListRolesOrderBy_LIST_ROLES_ORDER_BY_CREATED_AT_DESC:
		return domain.OrderByCreatedAtDesc
	case ssorolesv1.ListRolesOrderBy_LIST_ROLES_ORDER_BY_CREATED_AT_ASC:
		return domain.OrderByCreatedAtAsc
	case ssorolesv1.ListRolesOrderBy_LIST_ROLES_ORDER_BY_ROLE_ID_DESC:
		return domain.OrderByRoleIDDesc
	case ssorolesv1.ListRolesOrderBy_LIST_ROLES_ORDER_BY_ROLE_ID_ASC:
		return domain.OrderByRoleIDAsc
	case ssorolesv1.ListRolesOrderBy_LIST_ROLES_ORDER_BY_NAME_DESC:
		return domain.OrderByNameDesc
	case ssorolesv1.ListRolesOrderBy_LIST_ROLES_ORDER_BY_NAME_ASC:
		return domain.OrderByNameAsc
	}
	return domain.OrderByUnspecified
}
