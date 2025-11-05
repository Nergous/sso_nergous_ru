package models

import (
	"time"
)

type RefreshToken struct {
	ID        uint32    `gorm:"primaryKey;autoIncrement"`
	Token     string    `gorm:"uniqueIndex;not null;"`
	UserID    uint32    `gorm:"not null;"`
	AppID     uint32    `gorm:"not null;"`
	ExpiresAt time.Time `gorm:"not null;"`
	CreatedAt time.Time
	User      User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	App       App  `gorm:"foreignKey:AppID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
