package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload stored in every access token.
type Claims struct {
	UserID      string `json:"sub"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	jwt.RegisteredClaims
}

// IssueJWT signs a JWT with HS256 and the given expiry.
func IssueJWT(c Claims, secret string, expiry time.Duration) (string, error) {
	c.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
}

// ParseJWT validates the token and returns the claims.
func ParseJWT(raw, secret string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}
	return c, nil
}
