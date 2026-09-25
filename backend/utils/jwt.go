package utils

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims embeds the registered JWT claims plus our custom fields.
type Claims struct {
	UserID  int    `json:"user_id"`
	Email   string `json:"email"`
	RoleKey string `json:"role_key"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a signed token for an authenticated user.
func GenerateJWT(secret string, expiryHours string, userID int, email, roleKey string) (string, error) {
	hours, err := strconv.Atoi(expiryHours)
	if err != nil || hours <= 0 {
		hours = 12
	}

	claims := Claims{
		UserID:  userID,
		Email:   email,
		RoleKey: roleKey,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(hours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "coal-governance-backend",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseJWT validates a token string and returns its claims.
func ParseJWT(secret string, tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
