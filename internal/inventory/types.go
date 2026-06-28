package inventory

import (
	"context"
	"database/sql"
	"errors"
)

var ErrInvalidStatusTransition = errors.New("inventory: invalid status transition")

// ErrNoInventoryIDs indicates a caller invoked a status-transition method
// with an empty ID slice. This is a caller error, distinct from
// ErrInvalidStatusTransition which signals a domain conflict (a requested
// row failed to transition).
var ErrNoInventoryIDs = errors.New("inventory: no inventory ids provided")

// DBTX is satisfied by both *sql.DB and *sql.Tx, so repository functions
// can be called inside or outside an explicit transaction.
type DBTX interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// ProduceInput carries the data for a new inventory row created as an output of a work order.
type ProduceInput struct {
	ProductID   int
	BatchNumber string
	Quantity    float64
	UoMID       int
}
