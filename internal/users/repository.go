package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Repository owns all writes to users, roles, and user_roles, plus the reads
// this feature's own service/handlers need (auth reads these tables itself
// under a read-only lease for the login path).
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const recordLoginQuery = `
	UPDATE users SET last_login_at = NOW() WHERE id = $1
`

func (r *Repository) RecordLogin(ctx context.Context, userID int) error {
	_, err := r.db.ExecContext(ctx, recordLoginQuery, userID)
	if err != nil {
		return fmt.Errorf("recording login: %w", err)
	}
	return nil
}

const listRolesQuery = `
	SELECT id, name, description, created_at FROM roles ORDER BY id
`

func (r *Repository) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := r.db.QueryContext(ctx, listRolesQuery)
	if err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}
	defer rows.Close()

	roles := make([]Role, 0)
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning role: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating roles: %w", err)
	}
	return roles, nil
}

const getRoleByIDQuery = `
	SELECT id, name, description, created_at FROM roles WHERE id = $1
`

func (r *Repository) GetRoleByID(ctx context.Context, roleID int) (*Role, error) {
	var role Role
	err := r.db.QueryRowContext(ctx, getRoleByIDQuery, roleID).
		Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting role by id: %w", err)
	}
	return &role, nil
}

const getUserByIDQuery = `
	SELECT id, email, phone_number, full_name, is_active, last_login_at, created_at
	FROM users
	WHERE id = $1
`

func (r *Repository) GetUserByID(ctx context.Context, userID int) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, getUserByIDQuery, userID).
		Scan(&u.ID, &u.Email, &u.PhoneNumber, &u.FullName, &u.IsActive, &u.LastLoginAt, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return &u, nil
}

const listUserRolesQuery = `
	SELECT r.id, r.name, r.description, r.created_at
	FROM roles r
	JOIN user_roles ur ON ur.role_id = r.id
	WHERE ur.user_id = $1
	ORDER BY r.id
`

func (r *Repository) ListUserRoles(ctx context.Context, userID int) ([]Role, error) {
	rows, err := r.db.QueryContext(ctx, listUserRolesQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("listing user roles: %w", err)
	}
	defer rows.Close()

	roles := make([]Role, 0)
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning user role: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user roles: %w", err)
	}
	return roles, nil
}

const userRoleExistsQuery = `
	SELECT EXISTS(SELECT 1 FROM user_roles WHERE user_id = $1 AND role_id = $2)
`

const insertUserRoleQuery = `
	INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)
`

func (r *Repository) AssignRole(ctx context.Context, userID, roleID int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning assign role tx: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, userRoleExistsQuery, userID, roleID).Scan(&exists); err != nil {
		return fmt.Errorf("checking existing role assignment: %w", err)
	}
	if exists {
		return ErrRoleAlreadyAssigned
	}

	if _, err := tx.ExecContext(ctx, insertUserRoleQuery, userID, roleID); err != nil {
		return fmt.Errorf("inserting user role: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing assign role tx: %w", err)
	}
	return nil
}

const deleteUserRoleQuery = `
	DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2
`

func (r *Repository) RevokeRole(ctx context.Context, userID, roleID int) error {
	result, err := r.db.ExecContext(ctx, deleteUserRoleQuery, userID, roleID)
	if err != nil {
		return fmt.Errorf("revoking user role: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking revoke result: %w", err)
	}
	if rowsAffected == 0 {
		return ErrRoleNotFound
	}
	return nil
}
