package logistics

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/duniandewon/inventory-service/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

type Handler struct {
	service  *Service
	validate *validator.Validate
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func idFromPath(r *http.Request) (int, error) {
	return strconv.Atoi(chi.URLParam(r, "id"))
}

func processTypeResponse(pt *ProcessType) map[string]any {
	return map[string]any{"id": pt.ID, "name": pt.Name}
}

func workOrderResponse(wo *WorkOrder) map[string]any {
	m := map[string]any{
		"id":              wo.ID,
		"process_type_id": wo.ProcessTypeID,
		"status":          wo.Status,
		"updated_at":      wo.UpdatedAt,
	}
	if wo.AssignedPartnerID.Valid {
		m["assigned_partner_id"] = wo.AssignedPartnerID.Int64
	} else {
		m["assigned_partner_id"] = nil
	}
	if wo.CreatedBy.Valid {
		m["created_by"] = wo.CreatedBy.Int64
	} else {
		m["created_by"] = nil
	}
	return m
}

func workOrderDetailResponse(wo *WorkOrder) map[string]any {
	m := workOrderResponse(wo)
	items := make([]map[string]any, len(wo.LineItems))
	for i, item := range wo.LineItems {
		items[i] = map[string]any{
			"id":           item.ID,
			"inventory_id": item.InventoryID,
			"quantity":     item.Quantity,
			"direction":    item.Direction,
		}
	}
	m["line_items"] = items
	return m
}

func deliveryNoteResponse(note *DeliveryNote) map[string]any {
	m := map[string]any{
		"id":                   note.ID,
		"delivery_note_number": note.DeliveryNoteNumber,
		"created_at":           note.CreatedAt,
	}
	if note.RecipientPartnerID.Valid {
		m["recipient_partner_id"] = note.RecipientPartnerID.Int64
	} else {
		m["recipient_partner_id"] = nil
	}
	if note.CreatedBy.Valid {
		m["created_by"] = note.CreatedBy.Int64
	} else {
		m["created_by"] = nil
	}
	return m
}

func deliveryNoteDetailResponse(note *DeliveryNote) map[string]any {
	m := deliveryNoteResponse(note)
	items := make([]map[string]any, len(note.Items))
	for i, item := range note.Items {
		items[i] = map[string]any{
			"id":           item.ID,
			"inventory_id": item.InventoryID,
			"quantity":     item.Quantity,
		}
	}
	m["items"] = items
	return m
}

// Process type handlers

func (h *Handler) ListProcessTypes(w http.ResponseWriter, r *http.Request) {
	pts, err := h.service.ListProcessTypes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list process types")
		return
	}
	out := make([]map[string]any, len(pts))
	for i, pt := range pts {
		out[i] = processTypeResponse(&pt)
	}
	writeJSON(w, http.StatusOK, out)
}

type processTypeDTO struct {
	Name string `json:"name" validate:"required"`
}

func (h *Handler) CreateProcessType(w http.ResponseWriter, r *http.Request) {
	var dto processTypeDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	pt, err := h.service.CreateProcessType(r.Context(), dto.Name)
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, processTypeResponse(pt))
	case errors.Is(err, ErrProcessTypeNameExists):
		writeError(w, http.StatusConflict, "process type already exists")
	case errors.Is(err, ErrEmptyProcessTypeName):
		writeError(w, http.StatusBadRequest, "process type name must not be empty")
	default:
		writeError(w, http.StatusInternalServerError, "could not create process type")
	}
}

func (h *Handler) UpdateProcessType(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid process type id")
		return
	}
	var dto processTypeDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	pt, err := h.service.UpdateProcessType(r.Context(), id, dto.Name)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, processTypeResponse(pt))
	case errors.Is(err, ErrProcessTypeNotFound):
		writeError(w, http.StatusNotFound, "process type not found")
	case errors.Is(err, ErrProcessTypeNameExists):
		writeError(w, http.StatusConflict, "process type already exists")
	case errors.Is(err, ErrEmptyProcessTypeName):
		writeError(w, http.StatusBadRequest, "process type name must not be empty")
	default:
		writeError(w, http.StatusInternalServerError, "could not update process type")
	}
}

func (h *Handler) DeleteProcessType(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid process type id")
		return
	}
	err = h.service.DeleteProcessType(r.Context(), id)
	switch {
	case err == nil:
		writeJSON(w, http.StatusNoContent, nil)
	case errors.Is(err, ErrProcessTypeNotFound):
		writeError(w, http.StatusNotFound, "process type not found")
	case errors.Is(err, ErrProcessTypeInUse):
		writeError(w, http.StatusConflict, "process type is in use")
	default:
		writeError(w, http.StatusInternalServerError, "could not delete process type")
	}
}

// Work order handlers

type createWorkOrderDTO struct {
	ProcessTypeID     int `json:"process_type_id" validate:"required"`
	AssignedPartnerID int `json:"assigned_partner_id" validate:"required"`
}

func (h *Handler) CreateWorkOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var dto createWorkOrderDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(dto); err != nil {
		writeError(w, http.StatusBadRequest, "process_type_id and assigned_partner_id are required")
		return
	}
	wo, err := h.service.CreateWorkOrder(r.Context(), dto.ProcessTypeID, dto.AssignedPartnerID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create work order")
		return
	}
	writeJSON(w, http.StatusCreated, workOrderResponse(wo))
}

func (h *Handler) ListWorkOrders(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	wos, err := h.service.ListWorkOrders(r.Context(), statusFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list work orders")
		return
	}
	out := make([]map[string]any, len(wos))
	for i, wo := range wos {
		out[i] = workOrderResponse(&wo)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetWorkOrder(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work order id")
		return
	}
	wo, err := h.service.GetWorkOrder(r.Context(), id)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, workOrderDetailResponse(wo))
	case errors.Is(err, ErrWorkOrderNotFound):
		writeError(w, http.StatusNotFound, "work order not found")
	default:
		writeError(w, http.StatusInternalServerError, "could not get work order")
	}
}

type inputItemDTO struct {
	InventoryID int     `json:"inventory_id" validate:"required"`
	Quantity    float64 `json:"quantity" validate:"required,gt=0"`
}

type assignInputsDTO struct {
	Inputs []inputItemDTO `json:"inputs" validate:"required,min=1,dive"`
}

func (h *Handler) AssignInputs(w http.ResponseWriter, r *http.Request) {
	woID, err := idFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work order id")
		return
	}
	var dto assignInputsDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	inputs := make([]InputItem, len(dto.Inputs))
	for i, inp := range dto.Inputs {
		inputs[i] = InputItem{InventoryID: inp.InventoryID, Quantity: inp.Quantity}
	}
	err = h.service.AssignInputs(r.Context(), woID, inputs)
	switch {
	case err == nil:
		writeJSON(w, http.StatusNoContent, nil)
	case errors.Is(err, ErrWorkOrderNotFound):
		writeError(w, http.StatusNotFound, "work order not found")
	case errors.Is(err, ErrWorkOrderWrongStatus):
		writeError(w, http.StatusConflict, "work order is not in PENDING status")
	default:
		writeError(w, http.StatusInternalServerError, "could not assign inputs")
	}
}

type outputSpecDTO struct {
	ProductID   int     `json:"product_id" validate:"required"`
	BatchNumber string  `json:"batch_number"`
	Quantity    float64 `json:"quantity" validate:"required,gt=0"`
	UoMID       int     `json:"uom_id" validate:"required"`
}

type receiveOutputsDTO struct {
	Outputs []outputSpecDTO `json:"outputs" validate:"required,min=1,dive"`
}

func (h *Handler) ReceiveOutputs(w http.ResponseWriter, r *http.Request) {
	woID, err := idFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work order id")
		return
	}
	var dto receiveOutputsDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	outputs := make([]OutputSpec, len(dto.Outputs))
	for i, o := range dto.Outputs {
		outputs[i] = OutputSpec{
			ProductID:   o.ProductID,
			BatchNumber: o.BatchNumber,
			Quantity:    o.Quantity,
			UoMID:       o.UoMID,
		}
	}
	err = h.service.ReceiveOutputs(r.Context(), woID, outputs)
	switch {
	case err == nil:
		writeJSON(w, http.StatusNoContent, nil)
	case errors.Is(err, ErrWorkOrderNotFound):
		writeError(w, http.StatusNotFound, "work order not found")
	case errors.Is(err, ErrWorkOrderWrongStatus):
		writeError(w, http.StatusConflict, "work order is not in PROCESSING status")
	default:
		writeError(w, http.StatusInternalServerError, "could not receive outputs")
	}
}

// Delivery note handlers

type deliveryItemDTO struct {
	InventoryID int     `json:"inventory_id" validate:"required"`
	Quantity    float64 `json:"quantity" validate:"required,gt=0"`
}

type createDeliveryNoteDTO struct {
	RecipientPartnerID int               `json:"recipient_partner_id" validate:"required"`
	Items              []deliveryItemDTO `json:"items" validate:"required,min=1,dive"`
}

func (h *Handler) CreateDeliveryNote(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var dto createDeliveryNoteDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	items := make([]DeliveryItem, len(dto.Items))
	for i, item := range dto.Items {
		items[i] = DeliveryItem{InventoryID: item.InventoryID, Quantity: item.Quantity}
	}
	note, err := h.service.CreateDeliveryNote(r.Context(), dto.RecipientPartnerID, userID, items)
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, deliveryNoteDetailResponse(note))
	default:
		writeError(w, http.StatusInternalServerError, "could not create delivery note")
	}
}

func (h *Handler) ListDeliveryNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := h.service.ListDeliveryNotes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list delivery notes")
		return
	}
	out := make([]map[string]any, len(notes))
	for i, note := range notes {
		out[i] = deliveryNoteResponse(&note)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetDeliveryNote(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid delivery note id")
		return
	}
	note, err := h.service.GetDeliveryNote(r.Context(), id)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, deliveryNoteDetailResponse(note))
	case errors.Is(err, ErrDeliveryNoteNotFound):
		writeError(w, http.StatusNotFound, "delivery note not found")
	default:
		writeError(w, http.StatusInternalServerError, "could not get delivery note")
	}
}

// Lineage handler

func (h *Handler) GetLineage(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid inventory id")
		return
	}
	node, err := h.service.GetLineage(r.Context(), id)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, node)
	case errors.Is(err, ErrInventoryItemNotFound):
		writeError(w, http.StatusNotFound, "inventory item not found")
	default:
		writeError(w, http.StatusInternalServerError, "could not get lineage")
	}
}
