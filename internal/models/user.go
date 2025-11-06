package models

type User struct {
	ID          uint32 `gorm:"primaryKey;autoIncrement"`
	Email       string `gorm:"uniqueIndex;not null;"`
	PassHash    string `gorm:"not null;"`
	SteamURL    string `gorm:"not null;"`
	PathToPhoto string `gorm:"not null;"`
}

type AppUser struct {
	User
	AppID   uint32
	IsAdmin bool
}
