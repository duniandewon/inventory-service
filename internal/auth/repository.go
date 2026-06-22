package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Repository provides read-only access to users and roles. Auth never
// writes to these tables — user/role management lives in another feature.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const findActiveUserByPhoneQuery = `
	SELECT id, email, phone_number, full_name, is_active
	FROM users
	WHERE phone_number = $1 AND is_active = TRUE
`

func (r *Repository) FindActiveUserByPhone(ctx context.Context, phone string) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, findActiveUserByPhoneQuery, phone).
		Scan(&u.ID, &u.Email, &u.PhoneNumber, &u.FullName, &u.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("finding user by phone: %w", err)
	}
	return &u, nil
}

const getUserByIDQuery = `
	SELECT id, email, phone_number, full_name, is_active
	FROM users
	WHERE id = $1
`

func (r *Repository) GetUserByID(ctx context.Context, userID int) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, getUserByIDQuery, userID).
		Scan(&u.ID, &u.Email, &u.PhoneNumber, &u.FullName, &u.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return &u, nil
}

const getUserRolesQuery = `
	SELECT r.name
	FROM roles r
	JOIN user_roles ur ON ur.role_id = r.id
	WHERE ur.user_id = $1
`

func (r *Repository) GetUserRoles(ctx context.Context, userID int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, getUserRolesQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("getting user roles: %w", err)
	}
	defer rows.Close()

	roles := make([]string, 0)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("scanning user role: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user roles: %w", err)
	}
	return roles, nil
}
