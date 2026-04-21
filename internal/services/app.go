package services

import (
	"context"
	"fmt"
	"log/slog"

	"sso/internal/models"
	"sso/internal/repositories"
)

type AppService struct {
	log  *slog.Logger
	appR repositories.AppRepository
}

func NewAppService(
	log *slog.Logger,
	AppR repositories.AppRepository,
) *AppService {
	return &AppService{
		log:  log,
		appR: AppR,
	}
}

func (s *AppService) GetApp(ctx context.Context, id uint32) (*models.App, error) {
	const op = "auth.GetApp"

	app, err := s.appR.GetAppByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return app, nil
}

func (s *AppService) GetAllApps(ctx context.Context) ([]models.App, error) {
	const op = "auth.GetAllApps"

	apps, err := s.appR.GetAllApps(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return apps, nil
}

func (s *AppService) CreateApp(ctx context.Context, name, link, secret string) (uint32, error) {
	const op = "auth.CreateApp"

	id, err := s.appR.CreateApp(ctx, &models.App{Name: name, Link: link, Secret: secret, IsEnabled: true})
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	return id, nil
}

func (s *AppService) UpdateApp(ctx context.Context, id uint32, name, link string) error {
	const op = "auth.UpdateApp"

	if err := s.appR.UpdateApp(ctx, &models.App{ID: id, Name: name, Link: link}); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *AppService) DeleteApp(ctx context.Context, id uint32) error {
	const op = "auth.DeleteApp"

	if err := s.appR.DeleteApp(ctx, id); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *AppService) ChangeStatusApp(ctx context.Context, id uint32) error {
	const op = "auth.ChangeStatusApp"

	if err := s.appR.ChangeStatusApp(ctx, id); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *AppService) AddAdmin(ctx context.Context, userID uint32, appID uint32) error {
	const op = "auth.AddAdmin"

	if err := s.appR.AddAdmin(ctx, &models.Admin{UserID: userID, AppID: appID, IsAdmin: true}); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *AppService) RemoveAdmin(ctx context.Context, userID, appID uint32) error {
	const op = "auth.RemoveAdmin"

	if err := s.appR.RemoveAdmin(ctx, userID, appID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *AppService) IsAdmin(
	ctx context.Context,
	userID uint32,
	appID uint32,
) (bool, error) {
	const op = "auth.IsAdmin"

	isAdmin, err := s.appR.IsAdmin(ctx, userID, appID)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}
	return isAdmin, nil
}

func (s *AppService) GetAllUsersForApp(ctx context.Context, appID uint32) ([]models.AppUser, error) {
	const op = "auth.GetAllUsersForApp"

	users, err := s.appR.GetAllUsersForApp(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return users, nil
}
