package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestNewSessionToken(t *testing.T) {
	randomBytes := bytes.Repeat([]byte{0x42}, 32)
	token, tokenHash, err := newSessionToken(bytes.NewReader(randomBytes))
	if err != nil {
		t.Fatalf("newSessionToken() error = %v", err)
	}
	if token == "" || tokenHash == "" {
		t.Fatal("newSessionToken() returned an empty token or hash")
	}
	if token == tokenHash {
		t.Fatal("stored hash must not equal the cookie token")
	}

	wantHash := sha256.Sum256([]byte(token))
	if got, want := tokenHash, hex.EncodeToString(wantHash[:]); got != want {
		t.Fatalf("token hash = %q, want %q", got, want)
	}
}

func TestTruncateUsesCharacters(t *testing.T) {
	if got, want := truncate("กขคง", 3), "กขค"; got != want {
		t.Fatalf("truncate() = %q, want %q", got, want)
	}
}
