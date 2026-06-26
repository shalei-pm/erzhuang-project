package h5monitor

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
	mux.HandleFunc("GET /api/h5/orgs/{externalOrgId}/monitor", handler.getMonitorHome)
	mux.HandleFunc("POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/live-url", handler.getLiveURL)
	mux.HandleFunc("GET /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/record-segments", handler.getRecordSegments)
	mux.HandleFunc("POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/playback-url", handler.getPlaybackURL)
	mux.HandleFunc("POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/disable-url", handler.disableURL)
}

func (h *Handler) getMonitorHome(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.GetMonitorHome(r.Context(), r.PathValue("externalOrgId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getLiveURL(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseChannelID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id", nil)
		return
	}
	var request LiveURLRequest
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeJSON(w, r, &request) {
			return
		}
	}
	result, err := h.service.GetLiveURL(r.Context(), r.PathValue("externalOrgId"), channelID, request.UserID, request.IsAdmin)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getRecordSegments(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseChannelID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id", nil)
		return
	}
	result, err := h.service.GetRecordSegments(r.Context(), r.PathValue("externalOrgId"), channelID, r.URL.Query().Get("date"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getPlaybackURL(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseChannelID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id", nil)
		return
	}
	var request PlaybackURLRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.service.GetPlaybackURL(r.Context(), r.PathValue("externalOrgId"), channelID, request)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) disableURL(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseChannelID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id", nil)
		return
	}
	var request DisableURLRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	userID := request.UserID
	if userID == "" {
		userID = r.URL.Query().Get("user_id")
	}
	err = h.service.DisableURL(r.Context(), r.PathValue("externalOrgId"), channelID, request.URLID, userID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func parseChannelID(r *http.Request) (int64, error) {
	value := strings.TrimSpace(r.PathValue("channelId"))
	if value == "" {
		return 0, errors.New("channel id is required")
	}
	return strconv.ParseInt(value, 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, message string, fields map[string]string) {
	response := map[string]any{"error": message}
	if len(fields) > 0 {
		response["fields"] = fields
	}
	writeJSON(w, status, response)
}

func writeServiceError(w http.ResponseWriter, err error) {
	var validationError *ValidationError
	if errors.As(err, &validationError) {
		writeError(w, http.StatusBadRequest, validationError.Error(), validationError.Fields)
		return
	}
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found", nil)
		return
	}
	if errors.Is(err, ErrConcurrencyLimit) {
		writeError(w, http.StatusTooManyRequests, err.Error(), nil)
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error(), nil)
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
