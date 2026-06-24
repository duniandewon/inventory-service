package reference

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Repository owns all writes to units_of_measure. Other domains foreign-key
// straight to the table for reads (see context/features/units-of-measure.md §2).
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const listUnitsQuery = `
	SELECT id, name FROM units_of_measure ORDER BY id
`

func (r *Repository) ListUnits(ctx context.Context) ([]UnitOfMeasure, error) {
	rows, err := r.db.QueryContext(ctx, listUnitsQuery)
	if err != nil {
		return nil, fmt.Errorf("listing units: %w", err)
	}
	defer rows.Close()

	units := make([]UnitOfMeasure, 0)
	for rows.Next() {
		var u UnitOfMeasure
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			return nil, fmt.Errorf("scanning unit: %w", err)
		}
		units = append(units, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating units: %w", err)
	}
	return units, nil
}

const getUnitByIDQuery = `
	SELECT id, name FROM units_of_measure WHERE id = $1
`

func (r *Repository) GetUnitByID(ctx context.Context, id int) (*UnitOfMeasure, error) {
	var u UnitOfMeasure
	err := r.db.QueryRowContext(ctx, getUnitByIDQuery, id).Scan(&u.ID, &u.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnitNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting unit by id: %w", err)
	}
	return &u, nil
}

const unitNameExistsQuery = `
	SELECT EXISTS(SELECT 1 FROM units_of_measure WHERE LOWER(name) = LOWER($1) AND id != $2)
`

const insertUnitQuery = `
	INSERT INTO units_of_measure (name) VALUES ($1) RETURNING id
`

func (r *Repository) CreateUnit(ctx context.Context, name string) (*UnitOfMeasure, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning create unit tx: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, unitNameExistsQuery, name, 0).Scan(&exists); err != nil {
		return nil, fmt.Errorf("checking existing unit name: %w", err)
	}
	if exists {
		return nil, ErrUnitNameExists
	}

	var id int
	if err := tx.QueryRowContext(ctx, insertUnitQuery, name).Scan(&id); err != nil {
		return nil, fmt.Errorf("inserting unit: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing create unit tx: %w", err)
	}
	return &UnitOfMeasure{ID: id, Name: name}, nil
}

const updateUnitQuery = `
	UPDATE units_of_measure SET name = $1 WHERE id = $2
`

func (r *Repository) UpdateUnit(ctx context.Context, id int, name string) (*UnitOfMeasure, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning update unit tx: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, unitNameExistsQuery, name, id).Scan(&exists); err != nil {
		return nil, fmt.Errorf("checking existing unit name: %w", err)
	}
	if exists {
		return nil, ErrUnitNameExists
	}

	result, err := tx.ExecContext(ctx, updateUnitQuery, name, id)
	if err != nil {
		return nil, fmt.Errorf("updating unit: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking update result: %w", err)
	}
	if rowsAffected == 0 {
		return nil, ErrUnitNotFound
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing update unit tx: %w", err)
	}
	return &UnitOfMeasure{ID: id, Name: name}, nil
}

// work_order_line_items has no uom_id column in the current schema (001),
// despite the FK being described in context/features/units-of-measure.md §3 —
// only the two FKs that actually exist are checked here.
const unitInUseQuery = `
	SELECT
		EXISTS(SELECT 1 FROM products WHERE default_uom_id = $1)
		OR EXISTS(SELECT 1 FROM inventory WHERE uom_id = $1)
`

func (r *Repository) IsUnitInUse(ctx context.Context, id int) (bool, error) {
	var inUse bool
	if err := r.db.QueryRowContext(ctx, unitInUseQuery, id).Scan(&inUse); err != nil {
		return false, fmt.Errorf("checking unit usage: %w", err)
	}
	return inUse, nil
}

const deleteUnitQuery = `
	DELETE FROM units_of_measure WHERE id = $1
`

func (r *Repository) DeleteUnit(ctx context.Context, id int) error {
	result, err := r.db.ExecContext(ctx, deleteUnitQuery, id)
	if err != nil {
		return fmt.Errorf("deleting unit: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking delete result: %w", err)
	}
	if rowsAffected == 0 {
		return ErrUnitNotFound
	}
	return nil
}
