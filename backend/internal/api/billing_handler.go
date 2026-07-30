package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ebash/barn/backend/internal/billing"
)

type BillingHandler struct {
	svc *billing.Service
}

func NewBillingHandler(svc *billing.Service) *BillingHandler {
	return &BillingHandler{svc: svc}
}

func (h *BillingHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []billing.AccountResponse{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *BillingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req billing.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, billing.ErrInvalidInput)
		return
	}
	row, err := h.svc.Create(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *BillingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, billing.ErrInvalidInput)
		return
	}
	var req billing.UpdateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, billing.ErrInvalidInput)
		return
	}
	row, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *BillingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, billing.ErrInvalidInput)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *BillingHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, billing.ErrInvalidInput)
		return
	}
	row, err := h.svc.RefreshAccount(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}
