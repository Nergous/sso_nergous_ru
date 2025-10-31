package models

import (
	"time"
)

type App struct {
	ID     uint32 `gorm:"primaryKey;autoIncrement"`
	Name   string `gorm:"uniqueIndex;not null;"`
	Secret string `gorm:"not null;"`
	Link   string `gorm:"not null;"`
	// Signature string `gorm:"not null;"`
	IsEnabled bool `gorm:"not null;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
