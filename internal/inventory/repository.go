package inventory

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// Repository exposes the transaction-aware inventory mutations that logistics
// calls inside its own transactions. Inventory owns all status-transition rules;
// logistics owns the transaction boundary.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const reserveInventoryQuery = `
	UPDATE inventory SET status = 'IN_PROGRESS'
	WHERE id = ANY($1) AND status = 'AVAILABLE'
`

// ReserveForWorkOrder transitions the given inventory rows from AVAILABLE to
// IN_PROGRESS. invIDs must be non-empty, or ErrNoInventoryIDs is returned;
// duplicate IDs are collapsed internally before the transition is verified.
func (r *Repository) ReserveForWorkOrder(ctx context.Context, tx DBTX, invIDs []int) error {
	return r.execStatusTransition(ctx, tx, "reserving", reserveInventoryQuery, invIDs)
}

// execStatusTransition runs a status-transition UPDATE for the given query
// against the de-duplicated invIDs, and verifies that every unique ID
// transitioned. invIDs must be non-empty, or ErrNoInventoryIDs is returned
// before any query is issued.
func (r *Repository) execStatusTransition(ctx context.Context, tx DBTX, op, query string, invIDs []int) error {
	if len(invIDs) == 0 {
		return ErrNoInventoryIDs
	}
	ids := uniqueInts(invIDs)
	result, err := tx.ExecContext(ctx, query, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("%s inventory: %w", op, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking %s result: %w", op, err)
	}
	if int(n) != len(ids) {
		return ErrInvalidStatusTransition
	}
	return nil
}

// uniqueInts returns in with duplicate values collapsed, preserving the
// order of first occurrence.
func uniqueInts(in []int) []int {
	seen := make(map[int]struct{}, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

const insertInventoryQuery = `
	INSERT INTO inventory (product_id, batch_number, current_quantity, uom_id, status)
	VALUES ($1, $2, $3, $4, 'AVAILABLE')
	RETURNING id
`

func (r *Repository) ProduceFromWorkOrder(ctx context.Context, tx DBTX, outputs []ProduceInput) ([]int, error) {
	ids := make([]int, 0, len(outputs))
	for _, o := range outputs {
		var batchNumber sql.NullString
		if o.BatchNumber != "" {
			batchNumber = sql.NullString{String: o.BatchNumber, Valid: true}
		}
		var id int
		if err := tx.QueryRowContext(ctx, insertInventoryQuery, o.ProductID, batchNumber, o.Quantity, o.UoMID).Scan(&id); err != nil {
			return nil, fmt.Errorf("inserting inventory row: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

const consumeInventoryQuery = `
	UPDATE inventory SET status = 'CONSUMED'
	WHERE id = ANY($1) AND status = 'IN_PROGRESS'
`

// ConsumeForWorkOrder transitions the given inventory rows from IN_PROGRESS
// to CONSUMED. invIDs must be non-empty, or ErrNoInventoryIDs is returned;
// duplicate IDs are collapsed internally before the transition is verified.
func (r *Repository) ConsumeForWorkOrder(ctx context.Context, tx DBTX, invIDs []int) error {
	return r.execStatusTransition(ctx, tx, "consuming", consumeInventoryQuery, invIDs)
}

const markShippedQuery = `
	UPDATE inventory SET status = 'SHIPPED'
	WHERE id = ANY($1) AND status = 'AVAILABLE'
`

// MarkShipped transitions the given inventory rows from AVAILABLE to
// SHIPPED. invIDs must be non-empty, or ErrNoInventoryIDs is returned;
// duplicate IDs are collapsed internally before the transition is verified.
func (r *Repository) MarkShipped(ctx context.Context, tx DBTX, invIDs []int) error {
	return r.execStatusTransition(ctx, tx, "marking shipped", markShippedQuery, invIDs)
}
