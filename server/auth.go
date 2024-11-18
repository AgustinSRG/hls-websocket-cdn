// Authentication

package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Signs auth token
func signAuthToken(secret string, action string, streamId string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": action + ":" + streamId,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))

	if err != nil {
		LogError(err, "Error signing token")
	}

	return tokenString
}

// Validates authentication token
func validateAuthToken(tokenString string, secret string, action string, streamId string) bool {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})

	if err != nil {
		return false
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		// Validate expiration
		d, err := claims.GetExpirationTime()

		if err != nil || d == nil || d.UnixMilli() < time.Now().UnixMilli() {
			return false
		}

		// Validate subject
		sub, err := claims.GetSubject()

		if err != nil {
			return false
		}

		expectedSubject := action + ":" + streamId

		return sub == expectedSubject
	} else {
		return false
	}
}

// Auth configuration
type AuthConfiguration struct {
	// Secret for pull tokens
	PullSecret string

	// Secret for push tokens
	PushSecret string

	// True to allow push
	AllowPush bool
}

// Creates new instance of AuthController
func NewAuthController(config AuthConfiguration) *AuthController {
	return &AuthController{
		config: config,
	}
}

// Auth controller
type AuthController struct {
	config AuthConfiguration
}

// Checks if PUSH is allowed
func (ac *AuthController) IsPushAllowed() bool {
	return ac.config.AllowPush
}

// Validates PULL token
func (ac *AuthController) ValidatePullToken(token string, streamId string) bool {
	return validateAuthToken(token, ac.config.PullSecret, "PULL", streamId)
}

// Creates a PULL token
func (ac *AuthController) CreatePullToken(streamId string) string {
	return signAuthToken(ac.config.PullSecret, "PULL", streamId)
}

// Validates PUSH token
func (ac *AuthController) ValidatePushToken(token string, streamId string) bool {
	return validateAuthToken(token, ac.config.PushSecret, "PUSH", streamId)
}
