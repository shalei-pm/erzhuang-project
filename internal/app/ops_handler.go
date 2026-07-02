package app

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
	"github.com/shalei-pm/erzhuang-project/internal/osssmoke"
)

var currentOSSSmokeRunner ossSmokeRunner = runOSSSmokeFromEnv

type ossSmokeResponse struct {
	OK          bool   `json:"ok"`
	Key         string `json:"key,omitempty"`
	Bytes       int    `json:"bytes,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Error       string `json:"error,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

func (h *Handler) ossSmokeHandler(w http.ResponseWriter, r *http.Request) {
	if !opsEnabled() {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := h.ossSmokeRunner(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ossSmokeResponse{
			OK:     false,
			Error:  "oss smoke failed",
			Detail: sanitizeOpsError(err.Error()),
		})
		return
	}
	writeJSON(w, http.StatusOK, ossSmokeResponse{
		OK:          true,
		Key:         result.Key,
		Bytes:       result.Bytes,
		ContentType: result.ContentType,
	})
}

func opsEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("OPS_ENABLED")), "true")
}

func runOSSSmokeFromEnv(ctx context.Context) (*ossSmokeResult, error) {
	store, err := assets.NewStoreFromEnv()
	if err != nil {
		return nil, err
	}
	return osssmoke.Run(ctx, store, osssmoke.Options{Apply: true})
}

var opsSensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)authorization(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)signature(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)stringtosign(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)oss_access_key_secret(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)oss_access_key_id(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)accesskeysecret(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)accesskeyid(?:[=:][^\s,;]+)?`),
}

func sanitizeOpsError(value string) string {
	value = strings.TrimSpace(value)
	for _, pattern := range opsSensitivePatterns {
		value = pattern.ReplaceAllString(value, "[redacted]")
	}
	if len(value) > 320 {
		value = value[:320] + "..."
	}
	if value == "" {
		return "no detail"
	}
	return value
}
