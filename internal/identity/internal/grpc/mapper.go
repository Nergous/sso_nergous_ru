package grpcadapter

import (
	"sso/internal/identity/internal/domain"

	ssoidentityv1 "github.com/Nergous/sso_protos/gen/go/sso/identity/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UserToProto renders a domain.User as a sso.identity.v1.User. avatar_url
// and last_login_at are omitted (proto3 optional / left nil) when absent.
//
// Exported so sibling gRPC adapters (currently internal/grpc/auth.Register)
// can reuse the canonical wire format without redeclaring the mapping.
func UserToProto(u *domain.User) *ssoidentityv1.User {
	out := &ssoidentityv1.User{
		UserId:      u.ID().String(),
		Email:       u.Email,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Status:      statusToProto(u.Status()),
		Etag:        u.Etag().String(),
		CreatedAt:   timestamppb.New(u.CreatedAt()),
		UpdatedAt:   timestamppb.New(u.UpdatedAt()),
		Locale:      u.Locale,
		Timezone:    u.Timezone,
	}
	if u.AvatarURL != "" {
		out.AvatarUrl = proto.String(u.AvatarURL)
	}
	if !u.LastLoginAt.IsZero() {
		out.LastLoginAt = timestamppb.New(u.LastLoginAt)
	}
	return out
}

// statusToProto maps the domain enum onto the proto enum. UNSPECIFIED on
// the wire is never produced for a stored user.
func statusToProto(s domain.UserStatus) ssoidentityv1.UserStatus {
	switch s {
	case domain.UserStatusActive:
		return ssoidentityv1.UserStatus_USER_STATUS_ACTIVE
	case domain.UserStatusBlocked:
		return ssoidentityv1.UserStatus_USER_STATUS_BLOCKED
	case domain.UserStatusDeleted:
		return ssoidentityv1.UserStatus_USER_STATUS_DELETED
	}
	return ssoidentityv1.UserStatus_USER_STATUS_UNSPECIFIED
}

// statusFromProto maps the proto enum onto the domain enum. Returns
// (zero, false) for UNSPECIFIED or unknown values; callers should reject
// the request with INVALID_ARGUMENT in that case.
func statusFromProto(s ssoidentityv1.UserStatus) (domain.UserStatus, bool) {
	switch s {
	case ssoidentityv1.UserStatus_USER_STATUS_ACTIVE:
		return domain.UserStatusActive, true
	case ssoidentityv1.UserStatus_USER_STATUS_BLOCKED:
		return domain.UserStatusBlocked, true
	case ssoidentityv1.UserStatus_USER_STATUS_DELETED:
		return domain.UserStatusDeleted, true
	}
	return 0, false
}

// orderByFromProto maps the proto enum onto the domain enum. Unknown values
// degrade to OrderByUnspecified — the repository treats that as "default".
func orderByFromProto(o ssoidentityv1.ListUsersOrderBy) domain.ListOrderBy {
	switch o {
	case ssoidentityv1.ListUsersOrderBy_LIST_USERS_ORDER_BY_CREATED_AT_DESC:
		return domain.OrderByCreatedAtDesc
	case ssoidentityv1.ListUsersOrderBy_LIST_USERS_ORDER_BY_CREATED_AT_ASC:
		return domain.OrderByCreatedAtAsc
	case ssoidentityv1.ListUsersOrderBy_LIST_USERS_ORDER_BY_USER_ID_DESC:
		return domain.OrderByUserIDDesc
	case ssoidentityv1.ListUsersOrderBy_LIST_USERS_ORDER_BY_USER_ID_ASC:
		return domain.OrderByUserIDAsc
	case ssoidentityv1.ListUsersOrderBy_LIST_USERS_ORDER_BY_USERNAME_DESC:
		return domain.OrderByUsernameDesc
	case ssoidentityv1.ListUsersOrderBy_LIST_USERS_ORDER_BY_USERNAME_ASC:
		return domain.OrderByUsernameAsc
	}
	return domain.OrderByUnspecified
}
