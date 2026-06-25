package logistics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const listProcessTypesQuery = `SELECT id, name FROM process_types ORDER BY id`

func (r *Repository) ListProcessTypes(ctx context.Context) ([]ProcessType, error) {
	rows, err := r.db.QueryContext(ctx, listProcessTypesQuery)
	if err != nil {
		return nil, fmt.Errorf("listing process types: %w", err)
	}
	defer rows.Close()

	pts := make([]ProcessType, 0)
	for rows.Next() {
		var pt ProcessType
		if err := rows.Scan(&pt.ID, &pt.Name); err != nil {
			return nil, fmt.Errorf("scanning process type: %w", err)
		}
		pts = append(pts, pt)
	}
	return pts, rows.Err()
}

const getProcessTypeByIDQuery = `SELECT id, name FROM process_types WHERE id = $1`

func (r *Repository) GetProcessTypeByID(ctx context.Context, id int) (*ProcessType, error) {
	var pt ProcessType
	err := r.db.QueryRowContext(ctx, getProcessTypeByIDQuery, id).Scan(&pt.ID, &pt.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProcessTypeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting process type: %w", err)
	}
	return &pt, nil
}

const processTypeNameExistsQuery = `
	SELECT EXISTS(SELECT 1 FROM process_types WHERE LOWER(name) = LOWER($1) AND id != $2)
`

func (r *Repository) processTypeNameExists(ctx context.Context, tx *sql.Tx, name string, excludeID int) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, processTypeNameExistsQuery, name, excludeID).Scan(&exists)
	return exists, err
}

const insertProcessTypeQuery = `INSERT INTO process_types (name) VALUES ($1) RETURNING id`

func (r *Repository) CreateProcessType(ctx context.Context, name string) (*ProcessType, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback()

	exists, err := r.processTypeNameExists(ctx, tx, name, 0)
	if err != nil {
		return nil, fmt.Errorf("checking name: %w", err)
	}
	if exists {
		return nil, ErrProcessTypeNameExists
	}

	var id int
	if err := tx.QueryRowContext(ctx, insertProcessTypeQuery, name).Scan(&id); err != nil {
		return nil, fmt.Errorf("inserting process type: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing tx: %w", err)
	}
	return &ProcessType{ID: id, Name: name}, nil
}

const updateProcessTypeQuery = `UPDATE process_types SET name = $1 WHERE id = $2`

func (r *Repository) UpdateProcessType(ctx context.Context, id int, name string) (*ProcessType, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback()

	exists, err := r.processTypeNameExists(ctx, tx, name, id)
	if err != nil {
		return nil, fmt.Errorf("checking name: %w", err)
	}
	if exists {
		return nil, ErrProcessTypeNameExists
	}

	result, err := tx.ExecContext(ctx, updateProcessTypeQuery, name, id)
	if err != nil {
		return nil, fmt.Errorf("updating process type: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return nil, ErrProcessTypeNotFound
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing tx: %w", err)
	}
	return &ProcessType{ID: id, Name: name}, nil
}

const processTypeInUseQuery = `SELECT EXISTS(SELECT 1 FROM work_orders WHERE process_type_id = $1)`

func (r *Repository) IsProcessTypeInUse(ctx context.Context, id int) (bool, error) {
	var inUse bool
	err := r.db.QueryRowContext(ctx, processTypeInUseQuery, id).Scan(&inUse)
	return inUse, err
}

const deleteProcessTypeQuery = `DELETE FROM process_types WHERE id = $1`

func (r *Repository) DeleteProcessType(ctx context.Context, id int) error {
	result, err := r.db.ExecContext(ctx, deleteProcessTypeQuery, id)
	if err != nil {
		return fmt.Errorf("deleting process type: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrProcessTypeNotFound
	}
	return nil
}
