package storespace

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
)

type Handler struct {
	service *Service
}

type RouteMiddleware func(http.HandlerFunc) http.HandlerFunc

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(mux *http.ServeMux, service *Service) {
	RegisterRoutesWithWriteGuard(mux, service, nil)
}

func RegisterRoutesWithWriteGuard(mux *http.ServeMux, service *Service, writeGuard RouteMiddleware) {
	handler := NewHandler(service)
	write := func(next http.HandlerFunc) http.HandlerFunc {
		if writeGuard == nil {
			return next
		}
		return writeGuard(next)
	}
	mux.HandleFunc("GET /api/store-space/ezviz-accounts", handler.listEzvizAccounts)
	mux.HandleFunc("POST /api/store-space/ezviz-accounts", write(handler.createEzvizAccount))
	mux.HandleFunc("POST /api/store-space/diagnostics/ezviz/live-address", write(handler.getEzvizLiveAddress))
	mux.HandleFunc("GET /api/store-space/stores", handler.listStores)
	mux.HandleFunc("POST /api/store-space/stores", write(handler.createStore))
	mux.HandleFunc("POST /api/store-space/stores/check-duplicate", handler.checkDuplicate)
	mux.HandleFunc("GET /api/store-space/stores/{id}", handler.getStore)
	mux.HandleFunc("PATCH /api/store-space/stores/{id}", write(handler.updateStoreBasicInfo))
	mux.HandleFunc("GET /api/store-space/stores/{id}/design-plan-data", handler.getStoreDesignPlanData)
	mux.HandleFunc("GET /api/store-space/stores/{id}/channel-data", handler.getStoreChannelData)
	mux.HandleFunc("GET /api/store-space/stores/{id}/channel-mappings/export.xlsx", handler.exportChannelMappings)
	mux.HandleFunc("PUT /api/store-space/stores/{id}/design-plan", write(handler.saveDesignPlan))
	mux.HandleFunc("POST /api/store-space/stores/{id}/recorders", write(handler.addRecorder))
	mux.HandleFunc("DELETE /api/store-space/stores/{id}", write(handler.deleteStore))
	mux.HandleFunc("DELETE /api/store-space/recorders/{recorder_id}", write(handler.deleteRecorder))
	mux.HandleFunc("POST /api/store-space/recorders/{recorder_id}/scan-channels", write(handler.scanRecorderChannels))
	mux.HandleFunc("POST /api/store-space/recorders/{recorder_id}/probe-recognize-channel", write(handler.probeRecognizeChannel))
	mux.HandleFunc("POST /api/store-space/recorders/{recorder_id}/recognize-channels", write(handler.recognizeRecorderChannels))
	mux.HandleFunc("GET /api/store-space/channel-snapshots/{name}", handler.getChannelSnapshot)
	mux.HandleFunc("GET /api/store-space/channel-snapshots/{name}/diagnostics", handler.getChannelSnapshotDiagnostics)
	mux.HandleFunc("DELETE /api/store-space/channels/{channel_id}", write(handler.deleteChannel))
	mux.HandleFunc("POST /api/store-space/channels/{channel_id}/recognize", write(handler.recognizeChannel))
	mux.HandleFunc("POST /api/store-space/channels/{channel_id}/snapshot", write(handler.refreshChannelSnapshot))
	mux.HandleFunc("POST /api/store-space/channels/{channel_id}/unlock", write(handler.unlockChannelForEdit))
	mux.HandleFunc("PUT /api/store-space/channels/{channel_id}/confirmation", write(handler.confirmChannel))
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

func (h *Handler) getEzvizLiveAddress(w http.ResponseWriter, r *http.Request) {
	var input LiveAddressInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.GetLiveAddress(r.Context(), input)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listStores(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListStores(r.Context(), StoreFilters{
		Query:    r.URL.Query().Get("q"),
		City:     r.URL.Query().Get("city"),
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

func (h *Handler) getStoreDesignPlanData(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	store, err := h.service.GetStoreDesignPlanData(r.Context(), id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, store)
}

func (h *Handler) getStoreChannelData(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	store, err := h.service.GetStoreChannelData(r.Context(), id)
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

func (h *Handler) updateStoreBasicInfo(w http.ResponseWriter, r *http.Request) {
	storeID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input UpdateStoreBasicInfoInput
	if !decodeJSON(w, r, &input) {
		return
	}
	store, err := h.service.UpdateStoreBasicInfo(r.Context(), storeID, input)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, store)
}

func (h *Handler) saveDesignPlan(w http.ResponseWriter, r *http.Request) {
	storeID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input SaveDesignPlanInput
	if !decodeJSON(w, r, &input) {
		return
	}
	store, err := h.service.SaveDesignPlan(r.Context(), storeID, input)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, store)
}

func (h *Handler) exportChannelMappings(w http.ResponseWriter, r *http.Request) {
	storeID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.ExportChannelMappingExcel(r.Context(), storeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Disposition", contentDispositionAttachment(result.FileName))
	w.Header().Set("Content-Length", strconv.Itoa(len(result.Content)))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(result.Content); err != nil {
		log.Printf("storespace: write channel mapping export failed: %v", err)
	}
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

func (h *Handler) deleteChannel(w http.ResponseWriter, r *http.Request) {
	channelID, ok := parseID(w, r, "channel_id")
	if !ok {
		return
	}
	store, err := h.service.DeleteChannel(r.Context(), channelID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, store)
}

func (h *Handler) scanRecorderChannels(w http.ResponseWriter, r *http.Request) {
	recorderID, ok := parseID(w, r, "recorder_id")
	if !ok {
		return
	}
	recorder, err := h.service.ScanRecorderChannels(r.Context(), recorderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, recorder)
}

func (h *Handler) probeRecognizeChannel(w http.ResponseWriter, r *http.Request) {
	recorderID, ok := parseID(w, r, "recorder_id")
	if !ok {
		return
	}
	var input ProbeRecognizeChannelInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.ProbeRecognizeChannel(r.Context(), recorderID, input)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) recognizeRecorderChannels(w http.ResponseWriter, r *http.Request) {
	recorderID, ok := parseID(w, r, "recorder_id")
	if !ok {
		return
	}
	recorder, err := h.service.RecognizeRecorderChannels(r.Context(), recorderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, recorder)
}

func (h *Handler) recognizeChannel(w http.ResponseWriter, r *http.Request) {
	channelID, ok := parseID(w, r, "channel_id")
	if !ok {
		return
	}
	channel, err := h.service.RecognizeChannel(r.Context(), channelID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, channel)
}

func (h *Handler) refreshChannelSnapshot(w http.ResponseWriter, r *http.Request) {
	channelID, ok := parseID(w, r, "channel_id")
	if !ok {
		return
	}
	channel, err := h.service.RefreshChannelSnapshot(r.Context(), channelID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, channel)
}

func (h *Handler) getChannelSnapshot(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	etag := strconv.Quote(name)
	w.Header().Set("Cache-Control", "private, max-age=604800, immutable")
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	reader, contentType, err := h.service.OpenChannelSnapshot(r.Context(), name)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	defer reader.Close()
	if strings.TrimSpace(contentType) != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if _, err := io.Copy(w, reader); err != nil {
		log.Printf("storespace: serve channel snapshot failed: %v", err)
	}
}

func (h *Handler) getChannelSnapshotDiagnostics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.service.DiagnoseChannelSnapshot(r.Context(), r.PathValue("name")))
}

func (h *Handler) unlockChannelForEdit(w http.ResponseWriter, r *http.Request) {
	channelID, ok := parseID(w, r, "channel_id")
	if !ok {
		return
	}
	channel, err := h.service.UnlockChannelForEdit(r.Context(), channelID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, channel)
}

func (h *Handler) confirmChannel(w http.ResponseWriter, r *http.Request) {
	channelID, ok := parseID(w, r, "channel_id")
	if !ok {
		return
	}
	var input ChannelConfirmationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	store, err := h.service.ConfirmChannel(r.Context(), channelID, input)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, store)
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

func contentDispositionAttachment(fileName string) string {
	escaped := url.PathEscape(fileName)
	return `attachment; filename="channel-mappings.xlsx"; filename*=UTF-8''` + escaped
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
	var ezvizError *ezviz.Error
	if errors.As(err, &ezvizError) {
		writeDiagnosticError(w, http.StatusBadGateway, ezvizError.Error(), "ezviz_api_error", "ezviz", ezvizError.Error(), nil)
		return
	}
	writeDiagnosticError(w, http.StatusInternalServerError, "store space request failed", "store_space_request_failed", "store_space", err.Error(), nil)
}

func writeError(w http.ResponseWriter, status int, message string, fields map[string]string) {
	response := map[string]any{"error": message}
	if len(fields) > 0 {
		response["fields"] = fields
	}
	writeJSON(w, status, response)
}

func writeDiagnosticError(w http.ResponseWriter, status int, message string, code string, stage string, detail string, fields map[string]string) {
	response := map[string]any{
		"error": message,
		"code":  code,
		"stage": stage,
	}
	if cleanDetail := sanitizeDiagnosticDetail(detail); cleanDetail != "" {
		response["detail"] = cleanDetail
	}
	if len(fields) > 0 {
		response["fields"] = fields
	}
	writeJSON(w, status, response)
}

func sanitizeDiagnosticDetail(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > 500 {
		value = value[:500]
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || r == '&' || r == '?' || r == ';'
	})
	for _, field := range fields {
		lower := strings.ToLower(field)
		if strings.Contains(lower, "token=") ||
			strings.Contains(lower, "apikey=") ||
			strings.Contains(lower, "api_key=") ||
			strings.Contains(lower, "access_token=") ||
			strings.Contains(lower, "service_role") ||
			strings.Contains(lower, "authorization:") {
			value = strings.ReplaceAll(value, field, "[redacted]")
		}
	}
	return value
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
