package jwt

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func NewAccessToken(
	userID uint32,
	userEmail string,
	isAdmin bool,
	appID uint32,
	appSecret string,
	duration time.Duration,
) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["uid"] = userID
	claims["email"] = userEmail
	claims["exp"] = time.Now().Add(duration).Unix()
	claims["app_id"] = appID
	claims["is_admin"] = isAdmin
	claims["type"] = "access"

	tokenString, err := token.SignedString([]byte(appSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func NewRefreshToken() (string, error) {
	const tokenLength = 64
	bytes := make([]byte, tokenLength)

	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}
