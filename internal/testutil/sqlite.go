// Package testutil provides test-only helpers. Zero external dependencies —
// tests must run in any environment (no Docker, no network).
package testutil

import (
	"testing"

	"sso/internal/models"
	"sso/internal/storage/mariadb"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestStorage returns a fresh in-memory SQLite Storage with all tables
// migrated via GORM AutoMigrate. Uses pure-Go glebarez/sqlite — no CGO, no
// Docker. Each call yields an isolated DB.
func NewTestStorage(t *testing.T) *mariadb.Storage {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.App{}, &models.RefreshToken{}, &models.Admin{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return &mariadb.Storage{DB: db}
}
