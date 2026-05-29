package mariadb

import (
	"sso/internal/modules/access/internal/domain"
	"sso/internal/modules/access/internal/mariadb/dbgen"
)

func dbgenToDomain(r dbgen.RoleAssignment) *domain.RoleAssignment {
	return &domain.RoleAssignment{
		UserID:          domain.UserID(r.UserID),
		RoleID:          domain.RoleID(r.RoleID),
		AppID:           domain.AppID(r.AppID),
		GrantedByUserID: domain.ActorID(r.GrantedByUserID),
		GrantedAt:       r.GrantedAt,
	}
}

func toCreateParams(a *domain.RoleAssignment) dbgen.CreateRoleAssignmentParams {
	return dbgen.CreateRoleAssignmentParams{
		UserID:          a.UserID.String(),
		RoleID:          a.RoleID.String(),
		AppID:           a.AppID.String(),
		GrantedByUserID: a.GrantedByUserID.String(),
		GrantedAt:       a.GrantedAt,
	}
}
