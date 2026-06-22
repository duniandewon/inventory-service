package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
)

type Handler struct {
	service  *Service
	validate *validator.Validate
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

type otpRequestDTO struct {
	PhoneNumber string `json:"phone_number" validate:"required,e164"`
}

type otpVerifyDTO struct {
	PhoneNumber string `json:"phone_number" validate:"required,e164"`
	Code        string `json:"code" validate:"required,len=6,numeric"`
}

type refreshDTO struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
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

func decodeAndValidate(r *http.Request, v *validator.Validate, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return err
	}
	return v.Struct(dst)
}

func (h *Handler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	var dto otpRequestDTO
	if err := decodeAndValidate(r, h.validate, &dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.RequestOTP(r.Context(), dto.PhoneNumber)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case errors.Is(err, ErrLockedOut), errors.Is(err, ErrTooManyRequests):
		writeError(w, http.StatusTooManyRequests, "too many requests, try again later")
	default:
		writeError(w, http.StatusInternalServerError, "could not process request")
	}
}

func (h *Handler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var dto otpVerifyDTO
	if err := decodeAndValidate(r, h.validate, &dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pair, err := h.service.VerifyOTP(r.Context(), dto.PhoneNumber, dto.Code)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]string{
			"access_token":  pair.AccessToken,
			"refresh_token": pair.RefreshToken,
		})
	case errors.Is(err, ErrLockedOut):
		writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
	case errors.Is(err, ErrOTPNotFound), errors.Is(err, ErrOTPMismatch), errors.Is(err, ErrUserNotFound):
		writeError(w, http.StatusUnauthorized, "code expired or invalid")
	default:
		writeError(w, http.StatusInternalServerError, "could not verify code")
	}
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var dto refreshDTO
	if err := decodeAndValidate(r, h.validate, &dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pair, err := h.service.Refresh(r.Context(), dto.RefreshToken)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]string{
			"access_token":  pair.AccessToken,
			"refresh_token": pair.RefreshToken,
		})
	case errors.Is(err, ErrRefreshNotFound), errors.Is(err, ErrRefreshTokenReuse):
		writeError(w, http.StatusUnauthorized, "refresh token expired or revoked")
	default:
		writeError(w, http.StatusInternalServerError, "could not refresh session")
	}
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var dto refreshDTO
	if err := decodeAndValidate(r, h.validate, &dto); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.Logout(r.Context(), dto.RefreshToken); err != nil {
		writeError(w, http.StatusInternalServerError, "could not log out")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	user, roles, err := h.service.Me(r.Context(), userID)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{
			"id":           user.ID,
			"email":        user.Email,
			"phone_number": user.PhoneNumber,
			"full_name":    user.FullName.String,
			"roles":        roles,
		})
	case errors.Is(err, ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	default:
		writeError(w, http.StatusInternalServerError, "could not load user")
	}
}
