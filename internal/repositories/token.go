package repositories

import (
	"context"

	"sso/internal/domain"
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
	ctx context.Context,
	token *models.RefreshToken,
) (*models.RefreshToken, error) {
	err := r.storage.DB.WithContext(ctx).Create(token).Error
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (r *TokenRepo) GetRefreshToken(
	ctx context.Context,
	token string,
) (models.RefreshToken, error) {
	var refreshToken models.RefreshToken
	err := r.storage.DB.WithContext(ctx).Where("token = ?", token).First(&refreshToken).Error
	if isNotFound(err) {
		return models.RefreshToken{}, domain.ErrInvalidToken
	}
	if err != nil {
		return models.RefreshToken{}, err
	}
	return refreshToken, nil
}

func (r *TokenRepo) DeleteRefreshToken(
	ctx context.Context,
	token string,
) error {
	return r.storage.DB.WithContext(ctx).Where("token = ?", token).Delete(&models.RefreshToken{}).Error
}

func (r *TokenRepo) DeleteRefreshTokenByIDs(
	ctx context.Context,
	userID uint32,
	appID uint32,
) error {
	return r.storage.DB.WithContext(ctx).Where("user_id = ? AND app_id = ?", userID, appID).Delete(&models.RefreshToken{}).Error
}

func (r *TokenRepo) GetUserByRefreshToken(
	ctx context.Context,
	token string,
) (models.User, error) {
	var rfrTkn models.RefreshToken
	err := r.storage.DB.WithContext(ctx).Where("token = ?", token).First(&rfrTkn).Error
	if isNotFound(err) {
		return models.User{}, domain.ErrInvalidToken
	}
	if err != nil {
		return models.User{}, err
	}
	return rfrTkn.User, nil
}
