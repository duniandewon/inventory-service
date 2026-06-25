package logistics

import (
	"context"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Process types

func (s *Service) ListProcessTypes(ctx context.Context) ([]ProcessType, error) {
	return s.repo.ListProcessTypes(ctx)
}

func (s *Service) CreateProcessType(ctx context.Context, name string) (*ProcessType, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyProcessTypeName
	}
	return s.repo.CreateProcessType(ctx, name)
}

func (s *Service) UpdateProcessType(ctx context.Context, id int, name string) (*ProcessType, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyProcessTypeName
	}
	return s.repo.UpdateProcessType(ctx, id, name)
}

func (s *Service) DeleteProcessType(ctx context.Context, id int) error {
	if _, err := s.repo.GetProcessTypeByID(ctx, id); err != nil {
		return err
	}
	inUse, err := s.repo.IsProcessTypeInUse(ctx, id)
	if err != nil {
		return err
	}
	if inUse {
		return ErrProcessTypeInUse
	}
	return s.repo.DeleteProcessType(ctx, id)
}

// Work orders

func (s *Service) CreateWorkOrder(ctx context.Context, processTypeID, partnerID, createdBy int) (*WorkOrder, error) {
	return s.repo.CreateWorkOrder(ctx, processTypeID, partnerID, createdBy)
}

func (s *Service) AssignInputs(ctx context.Context, woID int, inputs []InputItem) error {
	if len(inputs) == 0 {
		return ErrNoInputs
	}
	wo, err := s.repo.GetWorkOrder(ctx, woID)
	if err != nil {
		return err
	}
	if wo.Status != "PENDING" {
		return ErrWorkOrderWrongStatus
	}
	return s.repo.AssignInputs(ctx, woID, inputs)
}

func (s *Service) ReceiveOutputs(ctx context.Context, woID int, outputs []OutputSpec) error {
	if len(outputs) == 0 {
		return ErrNoOutputs
	}
	wo, err := s.repo.GetWorkOrder(ctx, woID)
	if err != nil {
		return err
	}
	if wo.Status != "PROCESSING" {
		return ErrWorkOrderWrongStatus
	}
	return s.repo.ReceiveOutputs(ctx, woID, outputs)
}

func (s *Service) ListWorkOrders(ctx context.Context, statusFilter string) ([]WorkOrder, error) {
	return s.repo.ListWorkOrders(ctx, statusFilter)
}

func (s *Service) GetWorkOrder(ctx context.Context, id int) (*WorkOrder, error) {
	return s.repo.GetWorkOrderWithLineItems(ctx, id)
}

// Delivery notes

func (s *Service) CreateDeliveryNote(ctx context.Context, recipientPartnerID, createdBy int, items []DeliveryItem) (*DeliveryNote, error) {
	if len(items) == 0 {
		return nil, ErrNoDeliveryItems
	}
	return s.repo.CreateDeliveryNote(ctx, recipientPartnerID, createdBy, items)
}

func (s *Service) ListDeliveryNotes(ctx context.Context) ([]DeliveryNote, error) {
	return s.repo.ListDeliveryNotes(ctx)
}

func (s *Service) GetDeliveryNote(ctx context.Context, id int) (*DeliveryNote, error) {
	return s.repo.GetDeliveryNote(ctx, id)
}

// Lineage

func (s *Service) GetLineage(ctx context.Context, invID int) (*LineageNode, error) {
	return s.buildLineageNode(ctx, invID, 0)
}

func (s *Service) buildLineageNode(ctx context.Context, invID, depth int) (*LineageNode, error) {
	if depth > 20 {
		return &LineageNode{InventoryID: invID}, nil
	}

	item, err := s.repo.getLineageInventoryItem(ctx, invID)
	if err != nil {
		return nil, err
	}

	node := &LineageNode{
		InventoryID: item.ID,
		Status:      item.Status,
	}

	woID, err := s.repo.getProducingWorkOrderID(ctx, invID)
	if err != nil {
		return nil, err
	}
	if woID == nil {
		return node, nil
	}
	node.WorkOrderID = woID

	inputItems, err := s.repo.getWorkOrderInputItems(ctx, *woID)
	if err != nil {
		return nil, err
	}

	for _, inp := range inputItems {
		child, err := s.buildLineageNode(ctx, inp.ID, depth+1)
		if err != nil {
			return nil, err
		}
		node.Inputs = append(node.Inputs, *child)
	}
	return node, nil
}
