package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTokenManagerGenerateAndParse(t *testing.T) {
	userID := uuid.New()
	manager := NewTokenManager("unit-test-secret", time.Hour)

	token, err := manager.Generate(userID)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	got, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got != userID {
		t.Fatalf("Parse() user id = %s, want %s", got, userID)
	}
}

func TestTokenManagerRejectsTamperedToken(t *testing.T) {
	manager := NewTokenManager("unit-test-secret", time.Hour)
	token, err := manager.Generate(uuid.New())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	replacement := "x"
	if token[len(token)-1:] == replacement {
		replacement = "y"
	}
	tampered := token[:len(token)-1] + replacement
	_, err = manager.Parse(tampered)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse() error = %v, want ErrInvalidToken", err)
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	manager := NewTokenManager("unit-test-secret", -time.Second)
	token, err := manager.Generate(uuid.New())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	_, err = manager.Parse(token)
	if !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("Parse() error = %v, want ErrExpiredToken", err)
	}
}
