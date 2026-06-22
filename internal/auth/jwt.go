package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

type AppClaims struct {
	jwt.RegisteredClaims
	Roles []string `json:"roles,omitempty"`
	Type  string   `json:"typ"`
}

func newJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating token id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func signToken(secret string, userID int, roles []string, typ string, ttl time.Duration) (token string, jti string, err error) {
	jti, err = newJTI()
	if err != nil {
		return "", "", err
	}

	now := time.Now()
	claims := AppClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Type: typ,
	}
	if typ == tokenTypeAccess {
		claims.Roles = roles
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", "", fmt.Errorf("signing %s token: %w", typ, err)
	}
	return signed, jti, nil
}

func parseToken(secret, tokenString, expectedType string) (*AppClaims, error) {
	claims := &AppClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	if err != nil {
		return nil, fmt.Errorf("parsing %s token: %w", expectedType, err)
	}
	if claims.Type != expectedType {
		return nil, errors.New("auth: unexpected token type")
	}
	return claims, nil
}
