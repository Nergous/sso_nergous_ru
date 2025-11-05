package mariadb

import (
	"fmt"

	"sso/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Storage struct {
	DB *gorm.DB
}

func NewStorage(dsn string) (*Storage, error) {
	const op = "storage.mariadb.New"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Storage{DB: db}, nil
}

func (s *Storage) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

func (s *Storage) Migrate() error {
	const op = "storage.mariadb.Migrate"
	if err := s.DB.AutoMigrate(&models.User{}, &models.App{}, &models.RefreshToken{}, &models.Admin{}); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}
