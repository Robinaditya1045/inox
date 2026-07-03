package auth_test

import (
	"strings"
	"testing"

	"github.com/inox/inox/backend/internal/auth"
)

func TestHashAndVerifyPassword(t *testing.T) {
	password := "SecretWatchParty123!"

	// 1. Test HashPassword produces valid format
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("expected no error from HashPassword, got %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("expected hash to start with $argon2id$, got %s", hash)
	}

	// 2. Test VerifyPassword with correct password
	valid, err := auth.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("expected no error from VerifyPassword, got %v", err)
	}
	if !valid {
		t.Errorf("expected password verification to succeed for correct password")
	}

	// 3. Test VerifyPassword with wrong password
	valid, err = auth.VerifyPassword("WrongPassword!", hash)
	if err != nil {
		t.Fatalf("expected no error when verifying wrong password, got %v", err)
	}
	if valid {
		t.Errorf("expected password verification to fail for wrong password")
	}
}
