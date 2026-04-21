package repositories

import (
	"context"

	"sso/internal/domain"
	"sso/internal/models"
	"sso/internal/storage/mariadb"
)

type AppRepo struct {
	storage *mariadb.Storage
}

func NewAppRepo(storage *mariadb.Storage) *AppRepo {
	return &AppRepo{storage: storage}
}

func (r *AppRepo) GetAppByID(ctx context.Context, id uint32) (*models.App, error) {
	var app models.App
	err := r.storage.DB.WithContext(ctx).Where("id = ?", id).First(&app).Error
	if isNotFound(err) {
		return nil, domain.ErrAppNotFound
	}
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *AppRepo) GetAllApps(ctx context.Context) ([]models.App, error) {
	var apps []models.App
	err := r.storage.DB.WithContext(ctx).Find(&apps).Error
	if err != nil {
		return nil, err
	}
	return apps, nil
}

func (r *AppRepo) CreateApp(ctx context.Context, app *models.App) (uint32, error) {
	err := r.storage.DB.WithContext(ctx).Create(app).Error
	if isDuplicate(err) {
		return 0, domain.ErrAppAlreadyExists
	}
	if err != nil {
		return 0, err
	}
	return app.ID, nil
}

func (r *AppRepo) UpdateApp(ctx context.Context, app *models.App) error {
	res := r.storage.DB.WithContext(ctx).Model(&models.App{}).Where("id = ?", app.ID).Updates(map[string]any{
		"name":   app.Name,
		"secret": app.Secret,
		"link":   app.Link,
	})
	if res.Error != nil {
		if isDuplicate(res.Error) {
			return domain.ErrAppAlreadyExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrAppNotFound
	}
	return nil
}

func (r *AppRepo) DeleteApp(ctx context.Context, id uint32) error {
	return r.storage.DB.WithContext(ctx).Delete(&models.App{}, id).Error
}

func (r *AppRepo) ChangeStatusApp(ctx context.Context, id uint32) error {
	app, err := r.GetAppByID(ctx, id)
	if err != nil {
		return err
	}

	app.IsEnabled = !app.IsEnabled
	return r.storage.DB.WithContext(ctx).Save(&app).Error
}

func (r *AppRepo) IsAdmin(ctx context.Context, userID uint32, appID uint32) (bool, error) {
	var admin models.Admin
	err := r.storage.DB.WithContext(ctx).Where("user_id = ? AND app_id = ?", userID, appID).First(&admin).Error
	if isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return admin.IsAdmin, nil
}

func (r *AppRepo) AddAdmin(ctx context.Context, admin *models.Admin) error {
	err := r.storage.DB.WithContext(ctx).Create(admin).Error
	if isDuplicate(err) {
		return domain.ErrUserAlreadyExists
	}
	if err != nil {
		return err
	}
	return nil
}

func (r *AppRepo) RemoveAdmin(ctx context.Context, userID uint32, appID uint32) error {
	return r.storage.DB.WithContext(ctx).Delete(&models.Admin{}, "user_id = ? AND app_id = ?", userID, appID).Error
}

func (r *AppRepo) GetAllUsersForApp(ctx context.Context, appID uint32) ([]models.AppUser, error) {
	var users []models.AppUser

	err := r.storage.DB.WithContext(ctx).Select("users.id, users.email, users.steam_url, users.path_to_photo, admins.is_admin, admins.app_id").Where("admins.app_id = ?", appID).Joins("JOIN admins ON users.id = admins.user_id").Find(&users).Error
	if err != nil {
		return nil, err
	}

	return users, nil
}
