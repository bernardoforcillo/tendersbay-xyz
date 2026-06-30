package password

import (
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// Validate returns the names of failed rules; empty slice means the password is valid.
// Rules: min_length (12), uppercase, lowercase, digit, special.
func Validate(plain string) []string {
	var failed []string
	if len([]rune(plain)) < 12 {
		failed = append(failed, "min_length")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range plain {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	if !hasUpper {
		failed = append(failed, "uppercase")
	}
	if !hasLower {
		failed = append(failed, "lowercase")
	}
	if !hasDigit {
		failed = append(failed, "digit")
	}
	if !hasSpecial {
		failed = append(failed, "special")
	}
	return failed
}

// Hash bcrypts plain with DefaultCost.
func Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Verify returns true when plain matches the bcrypt hash.
func Verify(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
