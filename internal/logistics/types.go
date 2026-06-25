package logistics

import (
	"database/sql"
	"errors"
	"time"
)

var (
	ErrProcessTypeNotFound   = errors.New("logistics: process type not found")
	ErrProcessTypeNameExists = errors.New("logistics: process type name already exists")
	ErrProcessTypeInUse      = errors.New("logistics: process type is in use")
	ErrEmptyProcessTypeName  = errors.New("logistics: process type name must not be empty")

	ErrWorkOrderNotFound    = errors.New("logistics: work order not found")
	ErrWorkOrderWrongStatus = errors.New("logistics: work order has wrong status for this operation")
	ErrNoInputs             = errors.New("logistics: at least one input is required")
	ErrNoOutputs            = errors.New("logistics: at least one output is required")

	ErrDeliveryNoteNotFound = errors.New("logistics: delivery note not found")
	ErrNoDeliveryItems      = errors.New("logistics: at least one delivery item is required")

	ErrInventoryItemNotFound = errors.New("logistics: inventory item not found")
)

type ProcessType struct {
	ID   int
	Name string
}

type WorkOrder struct {
	ID                int
	ProcessTypeID     int
	AssignedPartnerID sql.NullInt64
	Status            string
	CreatedBy         sql.NullInt64
	UpdatedAt         time.Time
	LineItems         []WorkOrderLineItem
}

type WorkOrderLineItem struct {
	ID          int
	WorkOrderID int
	InventoryID int
	Quantity    float64
	Direction   string
}

type DeliveryNote struct {
	ID                 int
	DeliveryNoteNumber string
	RecipientPartnerID sql.NullInt64
	CreatedBy          sql.NullInt64
	CreatedAt          time.Time
	Items              []DeliveryNoteItem
}

type DeliveryNoteItem struct {
	ID             int
	DeliveryNoteID int
	InventoryID    int
	Quantity       float64
}

// InputItem is one element of an assign-inputs request.
type InputItem struct {
	InventoryID int
	Quantity    float64
}

// OutputSpec describes a new inventory row to produce when receiving work order outputs.
type OutputSpec struct {
	ProductID   int
	BatchNumber string
	Quantity    float64
	UoMID       int
}

// DeliveryItem is one item on a delivery note.
type DeliveryItem struct {
	InventoryID int
	Quantity    float64
}

// LineageNode is one node in the backward traceability tree.
type LineageNode struct {
	InventoryID int           `json:"inventory_id"`
	Status      string        `json:"status"`
	WorkOrderID *int          `json:"work_order_id,omitempty"`
	Inputs      []LineageNode `json:"inputs,omitempty"`
}
