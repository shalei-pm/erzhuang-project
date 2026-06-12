package storespace

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(mux *http.ServeMux, service *Service) {
	handler := NewHandler(service)
	mux.HandleFunc("GET /api/store-space/ezviz-accounts", handler.listEzvizAccounts)
	mux.HandleFunc("POST /api/store-space/ezviz-accounts", handler.createEzvizAccount)
	mux.HandleFunc("GET /api/store-space/stores", handler.listStores)
	mux.HandleFunc("POST /api/store-space/stores", handler.createStore)
	mux.HandleFunc("POST /api/store-space/stores/check-duplicate", handler.checkDuplicate)
	mux.HandleFunc("GET /api/store-space/stores/{id}", handler.getStore)
	mux.HandleFunc("POST /api/store-space/stores/{id}/recorders", handler.addRecorder)
	mux.HandleFunc("DELETE /api/store-space/stores/{id}", handler.deleteStore)
	mux.HandleFunc("DELETE /api/store-space/recorders/{recorder_id}", handler.deleteRecorder)
	mux.HandleFunc("POST /api/store-space/recorders/{recorder_id}/scan-channels", handler.scanRecorderChannels)
	mux.HandleFunc("POST /api/store-space/recorders/{recorder_id}/recognize-channels", handler.recognizeRecorderChannels)
}

func (h *Handler) listEzvizAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.service.ListEzvizAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list ezviz accounts failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, accounts)
}

func (h *Handler) createEzvizAccount(w http.ResponseWriter, r *http.Request) {
	var input CreateEzvizAccountInput
	if !decodeJSON(w, r, &input) {
		return
	}
	account, err := h.service.CreateEzvizAccount(r.Context(), input)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (h *Handler) listStores(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListStores(r.Context(), StoreFilters{
		Query:    r.URL.Query().Get("q"),
		Page:     parsePositiveInt(r.URL.Query().Get("page"), 1),
		PageSize: parsePositiveInt(r.URL.Query().Get("page_size"), 20),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list stores failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getStore(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	store, err := h.service.GetStore(r.Context(), id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, store)
}

func (h *Handler) createStore(w http.ResponseWriter, r *http.Request) {
	var input CreateStoreInput
	if !decodeJSON(w, r, &input) {
		return
	}
	store, err := h.service.CreateStore(r.Context(), input)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, store)
}

func (h *Handler) deleteStore(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := h.service.DeleteStore(r.Context(), id); err != nil {
		handleServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) addRecorder(w http.ResponseWriter, r *http.Request) {
	storeID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input AddRecorderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	store, err := h.service.AddRecorder(r.Context(), storeID, input)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, store)
}

func (h *Handler) checkDuplicate(w http.ResponseWriter, r *http.Request) {
	var request DuplicateCheckRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.service.CheckDuplicate(r.Context(), request)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) deleteRecorder(w http.ResponseWriter, r *http.Request) {
	recorderID, ok := parseID(w, r, "recorder_id")
	if !ok {
		return
	}
	if err := h.service.DeleteRecorder(r.Context(), recorderID); err != nil {
		handleServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) scanRecorderChannels(w http.ResponseWriter, r *http.Request) {
	recorderID, ok := parseID(w, r, "recorder_id")
	if !ok {
		return
	}
	err := h.service.ScanRecorderChannels(r.Context(), recorderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (h *Handler) recognizeRecorderChannels(w http.ResponseWriter, r *http.Request) {
	recorderID, ok := parseID(w, r, "recorder_id")
	if !ok {
		return
	}
	err := h.service.RecognizeRecorderChannels(r.Context(), recorderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func parseID(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id", nil)
		return 0, false
	}
	return id, true
}

func parsePositiveInt(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body", nil)
		return false
	}
	return true
}

func handleServiceError(w http.ResponseWriter, err error) {
	var validationError *ValidationError
	if errors.As(err, &validationError) {
		writeError(w, http.StatusBadRequest, validationError.Error(), validationError.Fields)
		return
	}
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "store not found", nil)
		return
	}
	if errors.Is(err, ErrNotImplemented) {
		writeError(w, http.StatusNotImplemented, "not implemented", nil)
		return
	}
	writeError(w, http.StatusInternalServerError, "store space request failed", nil)
}

func writeError(w http.ResponseWriter, status int, message string, fields map[string]string) {
	response := map[string]any{"error": message}
	if len(fields) > 0 {
		response["fields"] = fields
	}
	writeJSON(w, status, response)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
