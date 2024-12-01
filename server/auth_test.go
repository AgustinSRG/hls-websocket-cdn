// Tests for authentication

package main

import (
	"testing"

	"github.com/AgustinSRG/glog"
)

func TestSignFunctions(t *testing.T) {
	secret := "test-secret"
	streamId := "stream1"

	tokenPull, err := signAuthToken(secret, "PULL", streamId)
	if err != nil {
		t.Error(err)
	}
	if !validateAuthToken(tokenPull, secret, "PULL", streamId) {
		t.Errorf("Token does not pass validation: %v", tokenPull)
	}

	tokenPush, err := signAuthToken(secret, "PUSH", streamId)
	if err != nil {
		t.Error(err)
	}
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

	invalidTokenOther, err := signAuthToken("other-secret", "PULL", streamId)
	if err != nil {
		t.Error(err)
	}
	if validateAuthToken(invalidTokenOther, secret, "PULL", streamId) {
		t.Errorf("Invalid token passed validation: %v", invalidTokenOther)
	}
}

func TestAuthController(t *testing.T) {
	secretPush := "secret-push"
	secretPull := "secret-pull"
	streamId := "stream1"

	authController := NewAuthController(AuthConfiguration{
		PullSecret: secretPull,
		PushSecret: secretPush,
		AllowPush:  true,
	}, glog.CreateRootLogger(glog.CreateLoggerConfigurationFromLevel(glog.TRACE), glog.StandardLogFunction))

	tokenPull := authController.CreatePullToken(streamId)
	if !authController.ValidatePullToken(tokenPull, streamId) {
		t.Errorf("Token does not pass validation: %v", tokenPull)
	}

	tokenPush, err := signAuthToken(secretPush, "PUSH", streamId)
	if err != nil {
		t.Error(err)
	}
	if !authController.ValidatePushToken(tokenPush, streamId) {
		t.Errorf("Token does not pass validation: %v", tokenPush)
	}
}
