package logistics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type lineageInventoryItem struct {
	ID     int
	Status string
}

const getLineageInventoryItemQuery = `
	SELECT id, status FROM inventory WHERE id = $1
`

func (r *Repository) getLineageInventoryItem(ctx context.Context, id int) (*lineageInventoryItem, error) {
	var item lineageInventoryItem
	err := r.db.QueryRowContext(ctx, getLineageInventoryItemQuery, id).Scan(&item.ID, &item.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInventoryItemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting inventory item: %w", err)
	}
	return &item, nil
}

// getProducingWorkOrderID returns the work_order_id of the work order that produced
// invID as an OUTPUT, or nil if invID is a raw item with no producing work order.
const getProducingWorkOrderIDQuery = `
	SELECT work_order_id FROM work_order_line_items
	WHERE inventory_id = $1 AND direction = 'OUTPUT'
	LIMIT 1
`

func (r *Repository) getProducingWorkOrderID(ctx context.Context, invID int) (*int, error) {
	var woID int
	err := r.db.QueryRowContext(ctx, getProducingWorkOrderIDQuery, invID).Scan(&woID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting producing work order: %w", err)
	}
	return &woID, nil
}

const getWorkOrderInputItemsQuery = `
	SELECT i.id, i.status
	FROM inventory i
	JOIN work_order_line_items woli ON woli.inventory_id = i.id
	WHERE woli.work_order_id = $1 AND woli.direction = 'INPUT'
`

func (r *Repository) getWorkOrderInputItems(ctx context.Context, woID int) ([]lineageInventoryItem, error) {
	rows, err := r.db.QueryContext(ctx, getWorkOrderInputItemsQuery, woID)
	if err != nil {
		return nil, fmt.Errorf("getting work order input items: %w", err)
	}
	defer rows.Close()

	items := make([]lineageInventoryItem, 0)
	for rows.Next() {
		var item lineageInventoryItem
		if err := rows.Scan(&item.ID, &item.Status); err != nil {
			return nil, fmt.Errorf("scanning input item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
