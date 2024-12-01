// Authentication

package main

import (
	"fmt"
	"time"

	"github.com/AgustinSRG/glog"
	"github.com/golang-jwt/jwt/v5"
)

// Signs auth token
func signAuthToken(secret string, action string, streamId string) (string, error) {
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
func NewAuthController(config AuthConfiguration, logger *glog.Logger) *AuthController {
	if config.PullSecret == "" {
		logger.Warning("PULL_SECRET is empty. This means authentication is disabled for pulling streams.")
	}

	if config.PushSecret == "" {
		logger.Warning("PUSH_SECRET is empty. This means authentication is disabled for pushing streams.")
	}

	return &AuthController{
		config: config,
		logger: logger,
	}
}

// Auth controller
type AuthController struct {
	// Configuration
	config AuthConfiguration

	// Logger
	logger *glog.Logger
}

// Checks if PUSH is allowed
func (ac *AuthController) IsPushAllowed() bool {
	return ac.config.AllowPush
}

// Validates PULL token
func (ac *AuthController) ValidatePullToken(token string, streamId string) bool {
	if ac.config.PullSecret == "" {
		return true
	}
	return validateAuthToken(token, ac.config.PullSecret, "PULL", streamId)
}

// Creates a PULL token
func (ac *AuthController) CreatePullToken(streamId string) string {
	if ac.config.PullSecret == "" {
		return ""
	}
	token, err := signAuthToken(ac.config.PullSecret, "PULL", streamId)

	if err != nil {
		ac.logger.Errorf("Error signing token: %v", err)
	}

	return token
}

// Validates PUSH token
func (ac *AuthController) ValidatePushToken(token string, streamId string) bool {
	if ac.config.PushSecret == "" {
		return true
	}
	return validateAuthToken(token, ac.config.PushSecret, "PUSH", streamId)
}
