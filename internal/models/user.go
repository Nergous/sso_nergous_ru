package models

type User struct {
	ID          uint32 `gorm:"primaryKey;autoIncrement"`
	Email       string `gorm:"uniqueIndex;not null;"`
	PassHash    []byte `gorm:"not null;"`
	SteamURL    string `gorm:"not null;"`
	PathToPhoto string `gorm:"not null;"`
}
