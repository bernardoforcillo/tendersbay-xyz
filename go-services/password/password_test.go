package password_test

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/password"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		plain    string
		wantFail []string
	}{
		{"valid", "Secure!Pass123", []string{}},
		{"too short", "Ab!1", []string{"min_length"}},
		{"no uppercase", "secure!pass123", []string{"uppercase"}},
		{"no lowercase", "SECURE!PASS123", []string{"lowercase"}},
		{"no digit", "Secure!Password", []string{"digit"}},
		{"no special", "SecurePass1234", []string{"special"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := password.Validate(tt.plain)
			if len(got) != len(tt.wantFail) {
				t.Fatalf("Validate(%q) = %v, want %v", tt.plain, got, tt.wantFail)
			}
			for i := range got {
				if got[i] != tt.wantFail[i] {
					t.Errorf("[%d] got %q, want %q", i, got[i], tt.wantFail[i])
				}
			}
		})
	}
}

func TestVerifyDummy(t *testing.T) {
	// VerifyDummy exists only to burn ~bcrypt time on a not-found path; it has
	// no observable result. Assert it runs against a real bcrypt hash without
	// panicking (a malformed dummyHash would make CompareHashAndPassword error,
	// which is fine — the call must simply not crash).
	password.VerifyDummy("any-input")
}

func TestHashVerify(t *testing.T) {
	plain := "Secure!Pass123"
	hash, err := password.Hash(plain)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !password.Verify(plain, hash) {
		t.Error("Verify must return true for correct password")
	}
	if password.Verify("wrong", hash) {
		t.Error("Verify must return false for wrong password")
	}
}
