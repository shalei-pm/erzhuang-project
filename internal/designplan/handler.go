package designplan

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(mux *http.ServeMux, service *Service) {
	handler := NewHandler(service)
	mux.HandleFunc("POST /api/design-plan/uploads", handler.uploadPDF)
	mux.HandleFunc("GET /api/design-plan/uploads/{upload_id}/{asset}", handler.getUploadAsset)
	mux.HandleFunc("POST /api/design-plan/uploads/{upload_id}/recognize", handler.recognizeUpload)
	mux.HandleFunc("GET /api/design-plan/stores", handler.listStores)
	mux.HandleFunc("POST /api/design-plan/stores", handler.createStore)
	mux.HandleFunc("POST /api/design-plan/stores/check-duplicate", handler.checkDuplicate)
	mux.HandleFunc("GET /api/design-plan/stores/{id}", handler.getStore)
	mux.HandleFunc("GET /api/design-plan/stores/{id}/preview", handler.getStorePreview)
	mux.HandleFunc("GET /api/design-plan/stores/{id}/thumbnail", handler.getStoreThumbnail)
	mux.HandleFunc("PUT /api/design-plan/stores/{id}", handler.updateStore)
	mux.HandleFunc("DELETE /api/design-plan/stores/{id}", handler.deleteStore)
}

func (h *Handler) uploadPDF(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	r.Body = http.MaxBytesReader(w, r.Body, maxPDFBytes+1<<20)
	if err := r.ParseMultipartForm(maxPDFBytes + 1<<20); err != nil {
		log.Printf("designplan: upload failed stage=parse_form duration_ms=%d error=%q", time.Since(startedAt).Milliseconds(), err)
		writeError(w, http.StatusBadRequest, "invalid multipart form", nil)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("designplan: upload failed stage=form_file duration_ms=%d error=%q", time.Since(startedAt).Milliseconds(), err)
		writeError(w, http.StatusBadRequest, "PDF 文件不能为空", map[string]string{"file": "PDF 文件不能为空"})
		return
	}
	log.Printf("designplan: upload started file=%q size=%d", header.Filename, header.Size)

	head := make([]byte, 512)
	n, readErr := file.Read(head)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		log.Printf("designplan: upload failed stage=read_head file=%q size=%d duration_ms=%d error=%q", header.Filename, header.Size, time.Since(startedAt).Milliseconds(), readErr)
		writeError(w, http.StatusBadRequest, "read PDF failed", nil)
		return
	}

	result, err := h.service.SaveUpload(r.Context(), UploadInput{
		File:      file,
		FileName:  header.Filename,
		Header:    head[:n],
		Size:      header.Size,
		URLPrefix: "",
	})
	if err != nil {
		log.Printf("designplan: upload failed stage=save file=%q size=%d duration_ms=%d error=%q", header.Filename, header.Size, time.Since(startedAt).Milliseconds(), err)
		handleServiceError(w, err)
		return
	}
	log.Printf("designplan: upload completed upload_id=%s file=%q size=%d pages=%d duration_ms=%d", result.UploadID, result.FileName, header.Size, result.PageCount, time.Since(startedAt).Milliseconds())
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) recognizeUpload(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	uploadID := r.PathValue("upload_id")
	log.Printf("designplan: recognize started upload_id=%s", uploadID)
	result, err := h.service.RecognizeUpload(r.Context(), uploadID)
	if err != nil {
		log.Printf("designplan: recognize failed upload_id=%s duration_ms=%d error=%q", uploadID, time.Since(startedAt).Milliseconds(), err)
		handleServiceError(w, err)
		return
	}
	log.Printf("designplan: recognize completed upload_id=%s store_name=%q areas=%d duration_ms=%d", uploadID, result.StoreName, len(result.Areas), time.Since(startedAt).Milliseconds())
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getUploadAsset(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("upload_id")
	asset, ok := parseUploadAssetKind(r.PathValue("asset"))
	if !ok {
		writeError(w, http.StatusNotFound, "asset not found", nil)
		return
	}
	reader, contentType, err := h.service.OpenUploadAsset(uploadID, asset)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	defer reader.Close()
	serveAsset(w, reader, contentType)
}

func (h *Handler) listStores(w http.ResponseWriter, r *http.Request) {
	filters := StoreFilters{
		Query:    r.URL.Query().Get("q"),
		Page:     parsePositiveInt(r.URL.Query().Get("page"), 1),
		PageSize: parsePositiveInt(r.URL.Query().Get("page_size"), 20),
	}

	result, err := h.service.ListStores(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list stores failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getStore(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	store, err := h.service.GetStore(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "store not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get store failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, store)
}

func (h *Handler) getStorePreview(w http.ResponseWriter, r *http.Request) {
	h.serveStoreImage(w, r, UploadAssetPreview)
}

func (h *Handler) getStoreThumbnail(w http.ResponseWriter, r *http.Request) {
	h.serveStoreImage(w, r, UploadAssetThumbnail)
}

func (h *Handler) serveStoreImage(w http.ResponseWriter, r *http.Request, kind UploadAssetKind) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	store, err := h.service.GetStore(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "store not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get store failed", nil)
		return
	}

	value := store.PreviewImagePath
	if kind == UploadAssetThumbnail {
		value = store.ThumbnailPath
	}
	reader, contentType, err := h.service.OpenStoredAsset(value)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	defer reader.Close()
	serveAsset(w, reader, contentType)
}

func serveAsset(w http.ResponseWriter, reader io.Reader, contentType string) {
	if strings.TrimSpace(contentType) != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if _, err := io.Copy(w, reader); err != nil {
		log.Printf("designplan: serve asset failed: %v", err)
	}
}

func (h *Handler) createStore(w http.ResponseWriter, r *http.Request) {
	var input StoreInput
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

func (h *Handler) updateStore(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var input StoreInput
	if !decodeJSON(w, r, &input) {
		return
	}
	store, err := h.service.UpdateStore(r.Context(), id, input)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, store)
}

func (h *Handler) deleteStore(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteStore(r.Context(), id); err != nil {
		handleServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	value := r.PathValue("id")
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid store id", nil)
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

func parseUploadAssetKind(value string) (UploadAssetKind, bool) {
	switch UploadAssetKind(value) {
	case UploadAssetOriginal:
		return UploadAssetOriginal, true
	case UploadAssetPreview:
		return UploadAssetPreview, true
	case UploadAssetThumbnail:
		return UploadAssetThumbnail, true
	default:
		return "", false
	}
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
	writeError(w, http.StatusInternalServerError, "design plan request failed", nil)
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
