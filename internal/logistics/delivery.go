package logistics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const deliveryNoteCountQuery = `
	SELECT COUNT(*) FROM delivery_notes WHERE delivery_note_number LIKE $1
`

func (r *Repository) generateDeliveryNoteNumber(ctx context.Context, tx *sql.Tx) (string, error) {
	year := time.Now().Year()
	prefix := fmt.Sprintf("DN-%d-", year)
	var count int
	if err := tx.QueryRowContext(ctx, deliveryNoteCountQuery, prefix+"%").Scan(&count); err != nil {
		return "", fmt.Errorf("counting delivery notes: %w", err)
	}
	return fmt.Sprintf("%s%04d", prefix, count+1), nil
}

const insertDeliveryNoteQuery = `
	INSERT INTO delivery_notes (delivery_note_number, recipient_partner_id, created_by)
	VALUES ($1, $2, $3)
	RETURNING id, created_at
`

const insertDeliveryNoteItemQuery = `
	INSERT INTO delivery_note_items (delivery_note_id, inventory_id, quantity)
	VALUES ($1, $2, $3)
	RETURNING id
`

func (r *Repository) CreateDeliveryNote(ctx context.Context, recipientPartnerID, createdBy int, items []DeliveryItem) (*DeliveryNote, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback()

	noteNumber, err := r.generateDeliveryNoteNumber(ctx, tx)
	if err != nil {
		return nil, err
	}

	note := &DeliveryNote{
		DeliveryNoteNumber: noteNumber,
		RecipientPartnerID: nullInt(recipientPartnerID),
		CreatedBy:          nullInt(createdBy),
	}
	if err := tx.QueryRowContext(ctx, insertDeliveryNoteQuery, noteNumber, nullInt(recipientPartnerID), nullInt(createdBy)).
		Scan(&note.ID, &note.CreatedAt); err != nil {
		return nil, fmt.Errorf("inserting delivery note: %w", err)
	}

	noteItems := make([]DeliveryNoteItem, len(items))
	invIDs := make([]int, len(items))
	for i, item := range items {
		var itemID int
		if err := tx.QueryRowContext(ctx, insertDeliveryNoteItemQuery, note.ID, item.InventoryID, item.Quantity).Scan(&itemID); err != nil {
			return nil, fmt.Errorf("inserting delivery note item: %w", err)
		}
		noteItems[i] = DeliveryNoteItem{
			ID:             itemID,
			DeliveryNoteID: note.ID,
			InventoryID:    item.InventoryID,
			Quantity:       item.Quantity,
		}
		invIDs[i] = item.InventoryID
	}

	if err := r.invRepo.MarkShipped(ctx, tx, invIDs); err != nil {
		return nil, fmt.Errorf("marking inventory shipped: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing tx: %w", err)
	}

	note.Items = noteItems
	return note, nil
}

const getDeliveryNoteQuery = `
	SELECT id, delivery_note_number, recipient_partner_id, created_by, created_at
	FROM delivery_notes WHERE id = $1
`

const getDeliveryNoteItemsQuery = `
	SELECT id, delivery_note_id, inventory_id, quantity
	FROM delivery_note_items WHERE delivery_note_id = $1 ORDER BY id
`

func (r *Repository) GetDeliveryNote(ctx context.Context, id int) (*DeliveryNote, error) {
	var note DeliveryNote
	err := r.db.QueryRowContext(ctx, getDeliveryNoteQuery, id).Scan(
		&note.ID, &note.DeliveryNoteNumber, &note.RecipientPartnerID, &note.CreatedBy, &note.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDeliveryNoteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting delivery note: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, getDeliveryNoteItemsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("getting delivery note items: %w", err)
	}
	defer rows.Close()

	items := make([]DeliveryNoteItem, 0)
	for rows.Next() {
		var item DeliveryNoteItem
		if err := rows.Scan(&item.ID, &item.DeliveryNoteID, &item.InventoryID, &item.Quantity); err != nil {
			return nil, fmt.Errorf("scanning delivery note item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating delivery note items: %w", err)
	}
	note.Items = items
	return &note, nil
}

const listDeliveryNotesQuery = `
	SELECT id, delivery_note_number, recipient_partner_id, created_by, created_at
	FROM delivery_notes ORDER BY id DESC
`

func (r *Repository) ListDeliveryNotes(ctx context.Context) ([]DeliveryNote, error) {
	rows, err := r.db.QueryContext(ctx, listDeliveryNotesQuery)
	if err != nil {
		return nil, fmt.Errorf("listing delivery notes: %w", err)
	}
	defer rows.Close()

	notes := make([]DeliveryNote, 0)
	for rows.Next() {
		var note DeliveryNote
		if err := rows.Scan(&note.ID, &note.DeliveryNoteNumber, &note.RecipientPartnerID, &note.CreatedBy, &note.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning delivery note: %w", err)
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}
