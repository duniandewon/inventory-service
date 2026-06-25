package logistics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/duniandewon/inventory-service/internal/inventory"
)

const insertWorkOrderQuery = `
	INSERT INTO work_orders (process_type_id, assigned_partner_id, created_by)
	VALUES ($1, $2, $3)
	RETURNING id, status, updated_at
`

func (r *Repository) CreateWorkOrder(ctx context.Context, processTypeID, partnerID, createdBy int) (*WorkOrder, error) {
	wo := &WorkOrder{
		ProcessTypeID:     processTypeID,
		AssignedPartnerID: nullInt(partnerID),
		CreatedBy:         nullInt(createdBy),
	}
	err := r.db.QueryRowContext(ctx, insertWorkOrderQuery,
		processTypeID, nullInt(partnerID), nullInt(createdBy),
	).Scan(&wo.ID, &wo.Status, &wo.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting work order: %w", err)
	}
	return wo, nil
}

const getWorkOrderQuery = `
	SELECT id, process_type_id, assigned_partner_id, status, created_by, updated_at
	FROM work_orders WHERE id = $1
`

func (r *Repository) GetWorkOrder(ctx context.Context, id int) (*WorkOrder, error) {
	var wo WorkOrder
	err := r.db.QueryRowContext(ctx, getWorkOrderQuery, id).Scan(
		&wo.ID, &wo.ProcessTypeID, &wo.AssignedPartnerID, &wo.Status, &wo.CreatedBy, &wo.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWorkOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting work order: %w", err)
	}
	return &wo, nil
}

const listWorkOrdersQuery = `
	SELECT id, process_type_id, assigned_partner_id, status, created_by, updated_at
	FROM work_orders ORDER BY id DESC
`

const listWorkOrdersFilteredQuery = `
	SELECT id, process_type_id, assigned_partner_id, status, created_by, updated_at
	FROM work_orders WHERE status = $1::order_status ORDER BY id DESC
`

func (r *Repository) ListWorkOrders(ctx context.Context, statusFilter string) ([]WorkOrder, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if statusFilter != "" {
		rows, err = r.db.QueryContext(ctx, listWorkOrdersFilteredQuery, statusFilter)
	} else {
		rows, err = r.db.QueryContext(ctx, listWorkOrdersQuery)
	}
	if err != nil {
		return nil, fmt.Errorf("listing work orders: %w", err)
	}
	defer rows.Close()

	wos := make([]WorkOrder, 0)
	for rows.Next() {
		var wo WorkOrder
		if err := rows.Scan(&wo.ID, &wo.ProcessTypeID, &wo.AssignedPartnerID, &wo.Status, &wo.CreatedBy, &wo.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning work order: %w", err)
		}
		wos = append(wos, wo)
	}
	return wos, rows.Err()
}

const getWorkOrderLineItemsQuery = `
	SELECT id, work_order_id, inventory_id, quantity, direction
	FROM work_order_line_items WHERE work_order_id = $1 ORDER BY id
`

func (r *Repository) getWorkOrderLineItems(ctx context.Context, woID int) ([]WorkOrderLineItem, error) {
	rows, err := r.db.QueryContext(ctx, getWorkOrderLineItemsQuery, woID)
	if err != nil {
		return nil, fmt.Errorf("getting line items: %w", err)
	}
	defer rows.Close()

	items := make([]WorkOrderLineItem, 0)
	for rows.Next() {
		var item WorkOrderLineItem
		if err := rows.Scan(&item.ID, &item.WorkOrderID, &item.InventoryID, &item.Quantity, &item.Direction); err != nil {
			return nil, fmt.Errorf("scanning line item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetWorkOrderWithLineItems(ctx context.Context, id int) (*WorkOrder, error) {
	wo, err := r.GetWorkOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	items, err := r.getWorkOrderLineItems(ctx, id)
	if err != nil {
		return nil, err
	}
	wo.LineItems = items
	return wo, nil
}

const insertLineItemQuery = `
	INSERT INTO work_order_line_items (work_order_id, inventory_id, quantity, direction)
	VALUES ($1, $2, $3, $4::io_direction)
`

func (r *Repository) insertLineItems(ctx context.Context, tx *sql.Tx, woID int, items []WorkOrderLineItem) error {
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, insertLineItemQuery, woID, item.InventoryID, item.Quantity, item.Direction); err != nil {
			return fmt.Errorf("inserting line item: %w", err)
		}
	}
	return nil
}

const advanceWorkOrderStatusQuery = `
	UPDATE work_orders SET status = $1::order_status, updated_at = NOW()
	WHERE id = $2 AND status = $3::order_status
`

func (r *Repository) advanceWorkOrderStatus(ctx context.Context, tx *sql.Tx, woID int, from, to string) error {
	result, err := tx.ExecContext(ctx, advanceWorkOrderStatusQuery, to, woID, from)
	if err != nil {
		return fmt.Errorf("advancing work order status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrWorkOrderWrongStatus
	}
	return nil
}

const getWorkOrderInputIDsQuery = `
	SELECT inventory_id FROM work_order_line_items
	WHERE work_order_id = $1 AND direction = 'INPUT'
`

func (r *Repository) getWorkOrderInputIDs(ctx context.Context, tx *sql.Tx, woID int) ([]int, error) {
	rows, err := tx.QueryContext(ctx, getWorkOrderInputIDsQuery, woID)
	if err != nil {
		return nil, fmt.Errorf("getting input IDs: %w", err)
	}
	defer rows.Close()

	ids := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning input ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) AssignInputs(ctx context.Context, woID int, inputs []InputItem) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback()

	lineItems := make([]WorkOrderLineItem, len(inputs))
	invIDs := make([]int, len(inputs))
	for i, inp := range inputs {
		lineItems[i] = WorkOrderLineItem{InventoryID: inp.InventoryID, Quantity: inp.Quantity, Direction: "INPUT"}
		invIDs[i] = inp.InventoryID
	}

	if err := r.insertLineItems(ctx, tx, woID, lineItems); err != nil {
		return err
	}
	if err := r.invRepo.ReserveForWorkOrder(ctx, tx, invIDs); err != nil {
		return fmt.Errorf("reserving inventory: %w", err)
	}
	if err := r.advanceWorkOrderStatus(ctx, tx, woID, "PENDING", "PROCESSING"); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ReceiveOutputs(ctx context.Context, woID int, outputs []OutputSpec) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback()

	// Convert local OutputSpec to inventory.ProduceInput (keeps inventory import
	// confined to the repository layer only).
	produceInputs := make([]inventory.ProduceInput, len(outputs))
	for i, o := range outputs {
		produceInputs[i] = inventory.ProduceInput{
			ProductID:   o.ProductID,
			BatchNumber: o.BatchNumber,
			Quantity:    o.Quantity,
			UoMID:       o.UoMID,
		}
	}

	// FK ordering: new inventory rows must exist before line items that reference them.
	outIDs, err := r.invRepo.ProduceFromWorkOrder(ctx, tx, produceInputs)
	if err != nil {
		return fmt.Errorf("producing inventory: %w", err)
	}

	lineItems := make([]WorkOrderLineItem, len(outIDs))
	for i, id := range outIDs {
		lineItems[i] = WorkOrderLineItem{InventoryID: id, Quantity: outputs[i].Quantity, Direction: "OUTPUT"}
	}
	if err := r.insertLineItems(ctx, tx, woID, lineItems); err != nil {
		return err
	}

	inputIDs, err := r.getWorkOrderInputIDs(ctx, tx, woID)
	if err != nil {
		return err
	}
	if err := r.invRepo.ConsumeForWorkOrder(ctx, tx, inputIDs); err != nil {
		return fmt.Errorf("consuming inventory: %w", err)
	}
	if err := r.advanceWorkOrderStatus(ctx, tx, woID, "PROCESSING", "COMPLETED"); err != nil {
		return err
	}
	return tx.Commit()
}
