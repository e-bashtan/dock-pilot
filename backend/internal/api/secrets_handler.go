package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	secretpkg "github.com/ebash/barn/backend/internal/secrets"
)

type SecretsHandler struct {
	secrets *secretpkg.Service
}

func NewSecretsHandler(secrets *secretpkg.Service) *SecretsHandler {
	return &SecretsHandler{secrets: secrets}
}

func (h *SecretsHandler) List(w http.ResponseWriter, r *http.Request) {
	siteID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, secretpkg.ErrInvalidInput)
		return
	}

	rows, err := h.secrets.List(r.Context(), siteID)
	if err != nil {
		writeError(w, err)
		return
	}
	if rows == nil {
		rows = []secretpkg.SecretResponse{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *SecretsHandler) CreateMany(w http.ResponseWriter, r *http.Request) {
	siteID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, secretpkg.ErrInvalidInput)
		return
	}

	var req secretpkg.SetSecretsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, secretpkg.ErrInvalidInput)
		return
	}

	rows, err := h.secrets.SetMany(r.Context(), siteID, req.Secrets)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rows)
}

func (h *SecretsHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	siteID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, secretpkg.ErrInvalidInput)
		return
	}
	key := chi.URLParam(r, "key")

	var req secretpkg.SetSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, secretpkg.ErrInvalidInput)
		return
	}

	row, err := h.secrets.Set(r.Context(), siteID, key, req.Value)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *SecretsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	siteID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, secretpkg.ErrInvalidInput)
		return
	}
	key := chi.URLParam(r, "key")

	if err := h.secrets.Delete(r.Context(), siteID, key); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
