package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeAndValidateJWT(t *testing.T) {
	userID := uuid.New()
	secret := "my-secret-key"

	// 1. Valid Token Test
	token, err := MakeJWT(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("Failed to make JWT: %v", err)
	}

	parsedID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("Failed to validate JWT: %v", err)
	}

	if parsedID != userID {
		t.Errorf("Expected user ID %v, got %v", userID, parsedID)
	}

	// 2. Expired Token Test
	expiredToken, err := MakeJWT(userID, secret, -time.Hour)
	if err != nil {
		t.Fatalf("Failed to make expired JWT: %v", err)
	}

	_, err = ValidateJWT(expiredToken, secret)
	if err == nil {
		t.Errorf("Expected error for expired token, but got none")
	}

	// 3. Wrong Secret Test
	_, err = ValidateJWT(token, "wrong-secret")
	if err == nil {
		t.Errorf("Expected error for wrong secret, but got none")
	}
}
