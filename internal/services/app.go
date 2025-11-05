package services

import (
	"context"
	"log/slog"

	"sso/internal/models"
	"sso/internal/repositories"
	serr "sso/lib/serr"
)

type AppService struct {
	log  *slog.Logger
	appR *repositories.AppRepo
}

func NewAppService(
	log *slog.Logger,
	AppR *repositories.AppRepo,
) *AppService {
	return &AppService{
		log:  log,
		appR: AppR,
	}
}

func (s *AppService) GetApp(ctx *context.Context, id uint32) (*models.App, error) {
	const op = "auth.GetApp"

	app, err := s.appR.GetAppByID(ctx, id)
	ok, err := serr.Gerr(op, "app not found", "failed to get app", s.log, err)
	if !ok {
		return &models.App{}, err
	}

	return app, nil
}

func (s *AppService) GetAllApps(ctx *context.Context) ([]models.App, error) {
	const op = "auth.GetAllApps"

	apps, err := s.appR.GetAllApps(ctx)
	ok, err := serr.Gerr(op, "apps not found", "failed to get apps", s.log, err)
	if !ok {
		return []models.App{}, err
	}

	return apps, nil
}

func (s *AppService) CreateApp(ctx *context.Context, name, link, secret string) (uint32, error) {
	const op = "auth.CreateApp"

	app, err := s.appR.CreateApp(ctx, &models.App{Name: name, Link: link, Secret: secret, IsEnabled: true})
	ok, err := serr.Gerr(op, "app not found", "failed to create app", s.log, err)
	if !ok {
		return 0, err
	}

	return app, nil
}

func (s *AppService) UpdateApp(ctx *context.Context, id uint32, name, link string) error {
	const op = "auth.UpdateApp"

	err := s.appR.UpdateApp(ctx, &models.App{ID: id, Name: name, Link: link})
	ok, err := serr.Gerr(op, "app not found", "failed to update app", s.log, err)
	if !ok {
		return err
	}
	return nil
}

func (s *AppService) DeleteApp(ctx *context.Context, id uint32) error {
	const op = "auth.DeleteApp"

	err := s.appR.DeleteApp(ctx, id)
	ok, err := serr.Gerr(op, "app not found", "failed to delete app", s.log, err)
	if !ok {
		return err
	}
	return nil
}

func (s *AppService) ChangeStatusApp(ctx *context.Context, id uint32) error {
	const op = "auth.ChangeStatusApp"

	err := s.appR.ChangeStatusApp(ctx, id)
	ok, err := serr.Gerr(op, "app not found", "failed to change status app", s.log, err)
	if !ok {
		return err
	}
	return nil
}

func (s *AppService) AddAdmin(ctx *context.Context, userID uint32, appID uint32) error {
	const op = "auth.AddAdmin"

	err := s.appR.AddAdmin(ctx, &models.Admin{UserID: userID, AppID: appID, IsAdmin: true})
	ok, err := serr.Gerr(op, "app not found", "failed to add admin", s.log, err)
	if !ok {
		return err
	}
	return nil
}

func (s *AppService) RemoveAdmin(ctx *context.Context, userID, appID uint32) error {
	const op = "auth.RemoveAdmin"

	err := s.appR.RemoveAdmin(ctx, userID, appID)
	ok, err := serr.Gerr(op, "app not found", "failed to remove admin", s.log, err)
	if !ok {
		return err
	}
	return nil
}

func (s *AppService) IsAdmin(
	ctx *context.Context,
	userID uint32,
	appID uint32,
) (isAdmin bool, err error) {
	const op = "auth.IsAdmin"

	isAdmin, err = s.appR.IsAdmin(ctx, userID, appID)
	ok, err := serr.Gerr(op, "user not found", "failed to get admin", s.log, err)

	if !ok {
		return false, err
	}

	return isAdmin, nil
}
