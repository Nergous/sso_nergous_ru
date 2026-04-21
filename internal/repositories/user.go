package repositories

import (
	"context"

	"sso/internal/domain"
	"sso/internal/models"
	"sso/internal/storage/mariadb"
)

type UserRepo struct {
	storage *mariadb.Storage
}

func NewUserRepo(storage *mariadb.Storage) *UserRepo {
	return &UserRepo{storage: storage}
}

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	var user models.User
	err := r.storage.DB.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if isNotFound(err) {
		return models.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (r *UserRepo) GetUserByID(ctx context.Context, id uint32) (models.User, error) {
	var user models.User
	err := r.storage.DB.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if isNotFound(err) {
		return models.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (r *UserRepo) GetAllUsers(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.storage.DB.WithContext(ctx).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *UserRepo) CreateUser(ctx context.Context, user *models.User) (uint32, error) {
	err := r.storage.DB.WithContext(ctx).Create(user).Error
	if isDuplicate(err) {
		return 0, domain.ErrUserAlreadyExists
	}
	if err != nil {
		return 0, err
	}
	return user.ID, nil
}

func (r *UserRepo) UpdateUser(ctx context.Context, user models.User) error {
	res := r.storage.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"email":         user.Email,
		"pass_hash":     user.PassHash,
		"steam_url":     user.SteamURL,
		"path_to_photo": user.PathToPhoto,
	})
	if res.Error != nil {
		if isDuplicate(res.Error) {
			return domain.ErrUserAlreadyExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepo) DeleteUser(ctx context.Context, id uint32) error {
	return r.storage.DB.WithContext(ctx).Delete(&models.User{}, id).Error
}
