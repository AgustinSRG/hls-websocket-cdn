// Authentication

package main

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Signs auth token
func signAuthToken(secret string, action string, streamId string) (string, error) {
	if secret == "" {
		return "", nil
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": action + ":" + streamId,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))

	if err != nil {
		return "", err
	}

	return tokenString, nil
}
