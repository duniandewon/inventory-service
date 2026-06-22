package auth

import (
	"database/sql"
	"errors"
)

var (
	ErrUserNotFound      = errors.New("auth: user not found")
	ErrOTPNotFound       = errors.New("auth: otp not found or expired")
	ErrOTPMismatch       = errors.New("auth: otp code mismatch")
	ErrLockedOut         = errors.New("auth: locked out")
	ErrTooManyRequests   = errors.New("auth: too many otp requests")
	ErrRefreshNotFound   = errors.New("auth: refresh token not found or revoked")
	ErrRefreshTokenReuse = errors.New("auth: refresh token reuse detected")
)

type User struct {
	ID          int
	Email       string
	PhoneNumber string
	FullName    sql.NullString
	IsActive    bool
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}
