package users

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

type assignRoleDTO struct {
	RoleID int `json:"role_id" validate:"required"`
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

func roleDTO(role Role) map[string]any {
	return map[string]any{
		"id":          role.ID,
		"name":        role.Name,
		"description": role.Description.String,
	}
}

func rolesDTO(roles []Role) []map[string]any {
	out := make([]map[string]any, len(roles))
	for i, role := range roles {
		out[i] = roleDTO(role)
	}
	return out
}

func userIDFromPath(r *http.Request) (int, error) {
	return strconv.Atoi(chi.URLParam(r, "id"))
}

func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.ListRoles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list roles")
		return
	}
	writeJSON(w, http.StatusOK, rolesDTO(roles))
}

func (h *Handler) ListUserRoles(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	roles, err := h.service.ListUserRoles(r.Context(), userID)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, rolesDTO(roles))
	case errors.Is(err, ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	default:
		writeError(w, http.StatusInternalServerError, "could not list user roles")
	}
}

func (h *Handler) AssignRole(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var dto assignRoleDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	roles, err := h.service.AssignRole(r.Context(), userID, dto.RoleID)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, rolesDTO(roles))
	case errors.Is(err, ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, ErrRoleNotFound):
		writeError(w, http.StatusNotFound, "role not found")
	case errors.Is(err, ErrRoleAlreadyAssigned):
		writeError(w, http.StatusConflict, "role already assigned")
	default:
		writeError(w, http.StatusInternalServerError, "could not assign role")
	}
}

func (h *Handler) RevokeRole(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	roleID, err := strconv.Atoi(chi.URLParam(r, "roleID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid role id")
		return
	}

	roles, err := h.service.RevokeRole(r.Context(), userID, roleID)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, rolesDTO(roles))
	case errors.Is(err, ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, ErrRoleNotFound):
		writeError(w, http.StatusNotFound, "role not found")
	default:
		writeError(w, http.StatusInternalServerError, "could not revoke role")
	}
}
