package grpcadapter

import (
	"sso/internal/modules/access/internal/domain"
	"sso/internal/modules/role"

	ssoaccessv1 "github.com/Nergous/sso_protos/gen/go/sso/access/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// assignmentToProto renders a domain RoleAssignment as the proto message.
func assignmentToProto(a *domain.RoleAssignment) *ssoaccessv1.RoleAssignment {
	return &ssoaccessv1.RoleAssignment{
		UserId:          a.UserID.String(),
		RoleId:          a.RoleID.String(),
		AppId:           a.AppID.String(),
		GrantedByUserId: a.GrantedByUserID.String(),
		GrantedAt:       timestamppb.New(a.GrantedAt),
	}
}

// orderByFromProto translates the typed sort enum.
func orderByFromProto(o ssoaccessv1.ListUserRolesOrderBy) domain.ListOrderBy {
	switch o {
	case ssoaccessv1.ListUserRolesOrderBy_LIST_USER_ROLES_ORDER_BY_GRANTED_AT_DESC:
		return domain.OrderByGrantedAtDesc
	case ssoaccessv1.ListUserRolesOrderBy_LIST_USER_ROLES_ORDER_BY_GRANTED_AT_ASC:
		return domain.OrderByGrantedAtAsc
	case ssoaccessv1.ListUserRolesOrderBy_LIST_USER_ROLES_ORDER_BY_ROLE_ID_DESC:
		return domain.OrderByRoleIDDesc
	case ssoaccessv1.ListUserRolesOrderBy_LIST_USER_ROLES_ORDER_BY_ROLE_ID_ASC:
		return domain.OrderByRoleIDAsc
	}
	return domain.OrderByUnspecified
}

// reuse role's proto mapper so ListUserRolesResponse.roles uses the
// canonical Role wire format. The role module re-exports its grpc
// mapper at the public surface (role.RoleToProto) so sibling adapters
// don't have to import role/internal/grpc.
var roleToProto = role.RoleToProto
