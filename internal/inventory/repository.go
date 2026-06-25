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

func (r *Repository) ReserveForWorkOrder(ctx context.Context, tx DBTX, invIDs []int) error {
	result, err := tx.ExecContext(ctx, reserveInventoryQuery, pq.Array(invIDs))
	if err != nil {
		return fmt.Errorf("reserving inventory: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking reserve result: %w", err)
	}
	if int(n) != len(invIDs) {
		return ErrInvalidStatusTransition
	}
	return nil
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

func (r *Repository) ConsumeForWorkOrder(ctx context.Context, tx DBTX, invIDs []int) error {
	result, err := tx.ExecContext(ctx, consumeInventoryQuery, pq.Array(invIDs))
	if err != nil {
		return fmt.Errorf("consuming inventory: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking consume result: %w", err)
	}
	if int(n) != len(invIDs) {
		return ErrInvalidStatusTransition
	}
	return nil
}

const markShippedQuery = `
	UPDATE inventory SET status = 'SHIPPED'
	WHERE id = ANY($1) AND status = 'AVAILABLE'
`

func (r *Repository) MarkShipped(ctx context.Context, tx DBTX, invIDs []int) error {
	result, err := tx.ExecContext(ctx, markShippedQuery, pq.Array(invIDs))
	if err != nil {
		return fmt.Errorf("marking inventory shipped: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking ship result: %w", err)
	}
	if int(n) != len(invIDs) {
		return ErrInvalidStatusTransition
	}
	return nil
}
