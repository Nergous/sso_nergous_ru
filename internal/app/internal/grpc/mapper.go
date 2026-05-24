package app

import (
	"sso/internal/app/internal/domain"

	ssoappv1 "github.com/Nergous/sso_protos/gen/go/sso/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// appToProto renders a domain.App as a sso.app.v1.App.
func appToProto(a *domain.App) *ssoappv1.App {
	return &ssoappv1.App{
		AppId:     a.ID().String(),
		Name:      a.Name,
		Slug:      a.Slug(),
		Link:      a.Link,
		Status:    statusToProto(a.Status()),
		Etag:      a.Etag().String(),
		CreatedAt: timestamppb.New(a.CreatedAt()),
		UpdatedAt: timestamppb.New(a.UpdatedAt()),
	}
}

func statusToProto(s domain.AppStatus) ssoappv1.AppStatus {
	switch s {
	case domain.AppStatusActive:
		return ssoappv1.AppStatus_APP_STATUS_ACTIVE
	case domain.AppStatusDisabled:
		return ssoappv1.AppStatus_APP_STATUS_DISABLED
	case domain.AppStatusMaintenance:
		return ssoappv1.AppStatus_APP_STATUS_MAINTENANCE
	}
	return ssoappv1.AppStatus_APP_STATUS_UNSPECIFIED
}

func statusFromProto(s ssoappv1.AppStatus) (domain.AppStatus, bool) {
	switch s {
	case ssoappv1.AppStatus_APP_STATUS_ACTIVE:
		return domain.AppStatusActive, true
	case ssoappv1.AppStatus_APP_STATUS_DISABLED:
		return domain.AppStatusDisabled, true
	case ssoappv1.AppStatus_APP_STATUS_MAINTENANCE:
		return domain.AppStatusMaintenance, true
	}
	return 0, false
}

func orderByFromProto(o ssoappv1.ListAppsOrderBy) domain.ListOrderBy {
	switch o {
	case ssoappv1.ListAppsOrderBy_LIST_APPS_ORDER_BY_CREATED_AT_DESC:
		return domain.OrderByCreatedAtDesc
	case ssoappv1.ListAppsOrderBy_LIST_APPS_ORDER_BY_CREATED_AT_ASC:
		return domain.OrderByCreatedAtAsc
	case ssoappv1.ListAppsOrderBy_LIST_APPS_ORDER_BY_APP_ID_DESC:
		return domain.OrderByAppIDDesc
	case ssoappv1.ListAppsOrderBy_LIST_APPS_ORDER_BY_APP_ID_ASC:
		return domain.OrderByAppIDAsc
	case ssoappv1.ListAppsOrderBy_LIST_APPS_ORDER_BY_NAME_DESC:
		return domain.OrderByNameDesc
	case ssoappv1.ListAppsOrderBy_LIST_APPS_ORDER_BY_NAME_ASC:
		return domain.OrderByNameAsc
	}
	return domain.OrderByUnspecified
}
