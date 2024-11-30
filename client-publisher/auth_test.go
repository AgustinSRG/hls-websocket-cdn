// Tests for authentication

package clientpublisher

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

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

func TestSignFunctions(t *testing.T) {
	secret := "test-secret"
	streamId := "stream1"

	tokenPull, _ := signAuthToken(secret, "PULL", streamId)
	if !validateAuthToken(tokenPull, secret, "PULL", streamId) {
		t.Errorf("Token does not pass validation: %v", tokenPull)
	}

	tokenPush, _ := signAuthToken(secret, "PUSH", streamId)
	if !validateAuthToken(tokenPush, secret, "PUSH", streamId) {
		t.Errorf("Token does not pass validation: %v", tokenPush)
	}

	// Invalid tokens should not pass validation

	if validateAuthToken("invalid-token", secret, "PUSH", streamId) {
		t.Errorf("Invalid token passed validation: %v", "invalid-token")
	}

	if validateAuthToken(tokenPull, secret, "PUSH", streamId) {
		t.Errorf("Invalid token passed validation: %v", tokenPull)
	}

	invalidTokenOther, _ := signAuthToken("other-secret", "PULL", streamId)
	if validateAuthToken(invalidTokenOther, secret, "PULL", streamId) {
		t.Errorf("Invalid token passed validation: %v", invalidTokenOther)
	}
}
