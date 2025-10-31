package repositories

import (
	"context"
	"fmt"

	"sso/internal/models"
	"sso/internal/storage/mariadb"
)

type UserRepo struct {
	storage *mariadb.Storage
}

func NewUserRepo(storage *mariadb.Storage) *UserRepo {
	return &UserRepo{
		storage: storage,
	}
}

func (r *UserRepo) GetUserByEmail(ctx *context.Context, email string) (models.User, error) {
	var user models.User
	rows := r.storage.DB.WithContext(*ctx).Where("email = ?", email).First(&user)
	if rows.Error != nil {
		return models.User{}, rows.Error
	}

	return user, nil
}

func (r *UserRepo) GetUserByID(ctx *context.Context, id uint32) (models.User, error) {
	var user models.User
	rows := r.storage.DB.WithContext(*ctx).Where("id = ?", id).First(&user)
	if rows.Error != nil {
		return models.User{}, rows.Error
	}

	return user, nil
}

func (r *UserRepo) GetAllUsers(ctx *context.Context) ([]models.User, error) {
	var users []models.User
	rows := r.storage.DB.WithContext(*ctx).Find(&users)
	if rows.Error != nil {
		return nil, rows.Error
	}

	return users, nil
}

func (r *UserRepo) CreateUser(
	ctx *context.Context,
	user *models.User,
) (uint32, error) {
	rows := r.storage.DB.WithContext(*ctx).Create(&user)
	if rows.Error != nil {
		fmt.Println(rows.Error)
		return 0, rows.Error
	}

	return user.ID, nil
}

func (r *UserRepo) UpdateUser(ctx *context.Context, user models.User) error {
	oldUser, err := r.GetUserByID(ctx, user.ID)
	if err != nil {
		return err
	}

	oldUser.Email = user.Email
	oldUser.PassHash = user.PassHash
	oldUser.SteamURL = user.SteamURL
	oldUser.PathToPhoto = user.PathToPhoto

	return r.storage.DB.WithContext(*ctx).Save(&oldUser).Error
}

func (r *UserRepo) DeleteUser(ctx *context.Context, id uint32) error {
	return r.storage.DB.WithContext(*ctx).Delete(&models.User{}, id).Error
}
