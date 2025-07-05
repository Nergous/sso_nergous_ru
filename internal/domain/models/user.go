package models

type User struct {
	ID          int64
	Email       string
	PassHash    []byte
	IsAdmin     bool
	SteamURL    string
	PathToPhoto string
}
