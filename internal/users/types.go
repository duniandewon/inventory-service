package users

import (
	"database/sql"
	"errors"
)

var (
	ErrUserNotFound        = errors.New("users: user not found")
	ErrRoleNotFound        = errors.New("users: role not found")
	ErrRoleAlreadyAssigned = errors.New("users: role already assigned")
)

type User struct {
	ID          int
	Email       string
	PhoneNumber string
	FullName    sql.NullString
	IsActive    bool
	LastLoginAt sql.NullTime
	CreatedAt   sql.NullTime
}

type Role struct {
	ID          int
	Name        string
	Description sql.NullString
	CreatedAt   sql.NullTime
}
