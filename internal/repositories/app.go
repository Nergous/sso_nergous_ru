package repositories

import (
	"context"

	"sso/internal/models"
	"sso/internal/storage/mariadb"
)

type AppRepo struct {
	storage *mariadb.Storage
}

func NewAppRepo(storage *mariadb.Storage) *AppRepo {
	return &AppRepo{
		storage: storage,
	}
}

func (r *AppRepo) GetAppByID(ctx *context.Context, id uint32) (*models.App, error) {
	var app models.App
	rows := r.storage.DB.WithContext(*ctx).Where("id = ?", id).First(&app)
	if rows.Error != nil {
		return nil, rows.Error
	}

	return &app, nil
}

func (r *AppRepo) GetAllApps(ctx *context.Context) ([]models.App, error) {
	var apps []models.App
	rows := r.storage.DB.WithContext(*ctx).Find(&apps)
	if rows.Error != nil {
		return nil, rows.Error
	}

	return apps, nil
}

func (r *AppRepo) CreateApp(ctx *context.Context, app *models.App) (uint32, error) {
	rows := r.storage.DB.WithContext(*ctx).Create(&app)
	if rows.Error != nil {
		return 0, rows.Error
	}

	return app.ID, nil
}

func (r *AppRepo) UpdateApp(ctx *context.Context, app *models.App) error {
	var oldApp models.App
	rows := r.storage.DB.WithContext(*ctx).Where("id = ?", app.ID).First(&oldApp)
	if rows.Error != nil {
		return rows.Error
	}

	oldApp.Name = app.Name
	oldApp.Secret = app.Secret

	rows = r.storage.DB.WithContext(*ctx).Save(&oldApp)
	if rows.Error != nil {
		return rows.Error
	}

	return nil
}

func (r *AppRepo) DeleteApp(ctx *context.Context, id uint32) error {
	return r.storage.DB.WithContext(*ctx).Delete(&models.App{}, id).Error
}

func (r *AppRepo) ChangeStatusApp(ctx *context.Context, id uint32) error {
	app, err := r.GetAppByID(ctx, id)
	if err != nil {
		return err
	}

	app.IsEnabled = !app.IsEnabled
	return r.storage.DB.WithContext(*ctx).Save(&app).Error
}

func (r *AppRepo) IsAdmin(ctx *context.Context, userID uint32, appID uint32) (bool, error) {
	var admin models.Admin
	rows := r.storage.DB.WithContext(*ctx).Where("user_id = ? AND app_id = ?", userID, appID).First(&admin)
	if rows.Error != nil {
		return false, rows.Error
	}

	if admin.ID == 0 {
		return false, nil
	}

	return admin.IsAdmin, nil
}

func (r *AppRepo) AddAdmin(ctx *context.Context, admin *models.Admin) error {
	rows := r.storage.DB.WithContext(*ctx).Create(&admin)
	if rows.Error != nil {
		return rows.Error
	}

	return nil
}

func (r *AppRepo) RemoveAdmin(ctx *context.Context, userID uint32, appID uint32) error {
	return r.storage.DB.WithContext(*ctx).Delete(&models.Admin{}, "user_id = ? AND app_id = ?", userID, appID).Error
}
