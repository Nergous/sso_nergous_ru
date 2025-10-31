package repositories

import (
	"context"

	"sso/internal/models"
	"sso/internal/storage/mariadb"
)

type TokenRepo struct {
	storage *mariadb.Storage
}

func NewTokenRepo(storage *mariadb.Storage) *TokenRepo {
	return &TokenRepo{storage: storage}
}

func (r *TokenRepo) CreateRefreshToken(
	ctx *context.Context,
	token *models.RefreshToken,
) (*models.RefreshToken, error) {
	rows := r.storage.DB.WithContext(*ctx).Create(&token)
	if rows.Error != nil {
		return nil, rows.Error
	}

	return token, nil
}

func (r *TokenRepo) GetRefreshToken(
	ctx *context.Context,
	token string,
) (models.RefreshToken, error) {
	var refreshToken models.RefreshToken
	rows := r.storage.DB.WithContext(*ctx).Where("token = ?", token).First(&refreshToken)
	if rows.Error != nil {
		return models.RefreshToken{}, rows.Error
	}

	return refreshToken, nil
}

func (r *TokenRepo) DeleteRefreshToken(
	ctx *context.Context,
	token string,
) error {
	rows := r.storage.DB.WithContext(*ctx).Where("token = ?", token).Delete(&models.RefreshToken{})
	if rows.Error != nil {
		return rows.Error
	}

	return nil
}

func (r *TokenRepo) GetUserByRefreshToken(
	ctx *context.Context,
	token string,
) (models.User, error) {
	var rfrTkn models.RefreshToken
	rows := r.storage.DB.WithContext(*ctx).Where("token = ?", token).First(&rfrTkn)
	if rows.Error != nil {
		return models.User{}, rows.Error
	}

	return rfrTkn.User, nil
}
