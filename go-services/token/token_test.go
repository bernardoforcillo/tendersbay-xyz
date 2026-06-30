package token_test

import (
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/token"
)

func TestIssueParseJWT(t *testing.T) {
	secret := "test-secret-at-least-32-chars-ok"
	c := token.Claims{UserID: "u1", Email: "a@b.com", DisplayName: "Alice"}

	raw, err := token.IssueJWT(c, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("IssueJWT: %v", err)
	}

	got, err := token.ParseJWT(raw, secret)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}
	if got.UserID != c.UserID || got.Email != c.Email || got.DisplayName != c.DisplayName {
		t.Errorf("claims mismatch: got %+v", got)
	}
}

func TestParseJWT_Expired(t *testing.T) {
	secret := "test-secret-at-least-32-chars-ok"
	c := token.Claims{UserID: "u1", Email: "a@b.com", DisplayName: "Alice"}
	raw, _ := token.IssueJWT(c, secret, -time.Second)
	if _, err := token.ParseJWT(raw, secret); err == nil {
		t.Error("expected error for expired token")
	}
}

func TestGenerateOpaque(t *testing.T) {
	plain, hash, err := token.GenerateOpaque()
	if err != nil {
		t.Fatalf("GenerateOpaque: %v", err)
	}
	if len(plain) != 64 {
		t.Errorf("plain len = %d, want 64", len(plain))
	}
	if len(hash) != 64 {
		t.Errorf("hash len = %d, want 64", len(hash))
	}
	p2, h2, _ := token.GenerateOpaque()
	if plain == p2 || hash == h2 {
		t.Error("GenerateOpaque must produce unique values each call")
	}
}
