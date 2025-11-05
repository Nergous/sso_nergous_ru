package models

import "time"

type Admin struct {
	ID        uint32 `gorm:"primaryKey;autoIncrement"`
	UserID    uint32 `gorm:"not null;uniqueIndex"`
	AppID     uint32 `gorm:"not null;uniqueIndex"`
	IsAdmin   bool   `gorm:"not null;default:0"`
	User      User   `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	App       App    `gorm:"foreignKey:AppID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	CreatedAt time.Time
}
