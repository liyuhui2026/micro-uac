package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/liyuhui/micro-uac/internal/domain"
	"github.com/liyuhui/micro-uac/internal/task"
)

type Handler struct {
	manager *task.Manager
}

func NewHandler(manager *task.Manager) *Handler {
	return &Handler{manager: manager}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /calls", h.createCall)
	mux.HandleFunc("GET /calls/", h.getCall)
	return mux
}

func (h *Handler) createCall(w http.ResponseWriter, r *http.Request) {
	var req domain.CallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	res, err := h.manager.Create(r.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, task.ErrBusy) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, res)
}

func (h *Handler) getCall(w http.ResponseWriter, r *http.Request) {
	callID := strings.TrimPrefix(r.URL.Path, "/calls/")
	if callID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "call_id is required"})
		return
	}
	res, ok := h.manager.Get(callID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "call not found"})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
