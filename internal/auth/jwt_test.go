package auth_test

import (
	"testing"
	"time"

	"github.com/Sanjit10/HTTPServer/internal/auth"
	"github.com/google/uuid"
)

func TestMakeAndValidateJWT(t *testing.T) {
	secret := "supersecret"
	userID := uuid.New()

	// Create token
	token, err := auth.MakeJWT(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("failed to make JWT: %v", err)
	}

	// Validate token
	gotUserID, err := auth.ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("failed to validate JWT: %v", err)
	}

	if gotUserID != userID {
		t.Errorf("expected userID %v, got %v", userID, gotUserID)
	}
}

func TestExpiredJWT(t *testing.T) {
	secret := "supersecret"
	userID := uuid.New()

	// Create token that expired 1 second ago
	token, err := auth.MakeJWT(userID, secret, -1*time.Second)
	if err != nil {
		t.Fatalf("failed to make JWT: %v", err)
	}

	// Validate should fail
	_, err = auth.ValidateJWT(token, secret)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestWrongSecretJWT(t *testing.T) {
	secret := "supersecret"
	wrongSecret := "wrongsecret"
	userID := uuid.New()

	// Create token with correct secret
	token, err := auth.MakeJWT(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("failed to make JWT: %v", err)
	}

	// Validate with wrong secret should fail
	_, err = auth.ValidateJWT(token, wrongSecret)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}