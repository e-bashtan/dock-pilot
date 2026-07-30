package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ebash/barn/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgconn"
)

type QRHandler struct {
	qr *auth.QRService
}

func NewQRHandler(qr *auth.QRService) *QRHandler {
	return &QRHandler{qr: qr}
}

type qrCreateResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type qrExchangeRequest struct {
	Code string `json:"code"`
}

type qrExchangeResponse struct {
	Token string `json:"token"`
}

func (h *QRHandler) Create(w http.ResponseWriter, r *http.Request) {
	code, expiresAt, err := h.qr.Create(r.Context())
	if err != nil {
		slog.Error("create qr session", "error", err)
		if isMissingTable(err) {
			writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "qr auth migration required"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "failed to create qr session"})
		return
	}
	writeJSON(w, http.StatusCreated, qrCreateResponse{
		Code:      code,
		ExpiresAt: expiresAt,
	})
}

func (h *QRHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	var body qrExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Code == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "code is required"})
		return
	}

	token, err := h.qr.Exchange(r.Context(), body.Code)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrQRNotFound), errors.Is(err, auth.ErrQRInvalid):
			writeJSON(w, http.StatusNotFound, errorBody{Error: "invalid or expired code"})
		case errors.Is(err, auth.ErrQRUsed):
			writeJSON(w, http.StatusGone, errorBody{Error: "code already used"})
		default:
			writeJSON(w, http.StatusInternalServerError, errorBody{Error: "failed to exchange code"})
		}
		return
	}

	writeJSON(w, http.StatusOK, qrExchangeResponse{Token: token})
}

func isMissingTable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "42P01"
	}
	return strings.Contains(strings.ToLower(err.Error()), "does not exist")
}
