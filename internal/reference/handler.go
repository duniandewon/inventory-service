package reference

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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

type unitDTO struct {
	Name string `json:"name" validate:"required"`
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

func unitResponse(u *UnitOfMeasure) map[string]any {
	return map[string]any{"id": u.ID, "name": u.Name}
}

func unitsResponse(units []UnitOfMeasure) []map[string]any {
	out := make([]map[string]any, len(units))
	for i, u := range units {
		out[i] = map[string]any{"id": u.ID, "name": u.Name}
	}
	return out
}

func unitIDFromPath(r *http.Request) (int, error) {
	return strconv.Atoi(chi.URLParam(r, "id"))
}

func (h *Handler) ListUnits(w http.ResponseWriter, r *http.Request) {
	units, err := h.service.ListUnits(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list units")
		return
	}
	writeJSON(w, http.StatusOK, unitsResponse(units))
}

func (h *Handler) CreateUnit(w http.ResponseWriter, r *http.Request) {
	var dto unitDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	unit, err := h.service.CreateUnit(r.Context(), dto.Name)
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, unitResponse(unit))
	case errors.Is(err, ErrUnitNameExists):
		writeError(w, http.StatusConflict, "unit already exists")
	case errors.Is(err, ErrEmptyUnitName):
		writeError(w, http.StatusBadRequest, "unit name must not be empty")
	default:
		writeError(w, http.StatusInternalServerError, "could not create unit")
	}
}

func (h *Handler) UpdateUnit(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid unit id")
		return
	}

	var dto unitDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	unit, err := h.service.UpdateUnit(r.Context(), id, dto.Name)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, unitResponse(unit))
	case errors.Is(err, ErrUnitNotFound):
		writeError(w, http.StatusNotFound, "unit not found")
	case errors.Is(err, ErrUnitNameExists):
		writeError(w, http.StatusConflict, "unit already exists")
	case errors.Is(err, ErrEmptyUnitName):
		writeError(w, http.StatusBadRequest, "unit name must not be empty")
	default:
		writeError(w, http.StatusInternalServerError, "could not update unit")
	}
}

func (h *Handler) DeleteUnit(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid unit id")
		return
	}

	err = h.service.DeleteUnit(r.Context(), id)
	switch {
	case err == nil:
		writeJSON(w, http.StatusNoContent, nil)
	case errors.Is(err, ErrUnitNotFound):
		writeError(w, http.StatusNotFound, "unit not found")
	case errors.Is(err, ErrUnitInUse):
		writeError(w, http.StatusConflict, "unit is in use")
	default:
		writeError(w, http.StatusInternalServerError, "could not delete unit")
	}
}
