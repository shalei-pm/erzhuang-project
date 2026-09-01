package app

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

const (
	defaultAuditLogPageSize                   = 20
	maxAuditLogPageSize                       = 100
	internalAuditActionSnapshotRefreshPrepare = "snapshot.refresh.prepare"
)

// AuditLog remains the app-facing name for the shared persisted audit event.
type AuditLog = auditlog.AuditEvent

type AuditLogFilter struct {
	StartAt  time.Time
	EndAt    time.Time
	UserID   *int64
	Action   string
	Page     int
	PageSize int
}

type AuditLogPage struct {
	Items    []AuditLog `json:"items"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Total    int        `json:"total"`
}

// AuditLogStore separates audit persistence from the Handler's broader store contract.
type AuditLogStore interface {
	CreateAuditLog(ctx context.Context, log AuditLog) error
	ListAuditLogs(ctx context.Context, filter AuditLogFilter) (AuditLogPage, error)
}

func normalizeAuditLogFilter(filter AuditLogFilter) (AuditLogFilter, int) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = defaultAuditLogPageSize
	}
	if filter.PageSize > maxAuditLogPageSize {
		filter.PageSize = maxAuditLogPageSize
	}
	filter.Action = strings.TrimSpace(filter.Action)

	offset := 0
	if filter.Page > 1 {
		maxInt := int(^uint(0) >> 1)
		if filter.Page-1 > maxInt/filter.PageSize {
			offset = maxInt
		} else {
			offset = (filter.Page - 1) * filter.PageSize
		}
	}
	return filter, offset
}

var safeAuditDetailKeys = map[string]bool{
	"summary":      true,
	"source":       true,
	"reason":       true,
	"error_code":   true,
	"target_name":  true,
	"store_name":   true,
	"channel_name": true,
	"role":         true,
	"scope_count":  true,
	"start_time":   true,
	"end_time":     true,
	"date_range":   true,
}

// sanitizeAuditDetail retains only primitive values for a narrow set of business summary keys.
func sanitizeAuditDetail(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}

	var detail map[string]json.RawMessage
	if err := json.Unmarshal(raw, &detail); err != nil {
		return nil
	}

	sanitized := make(map[string]json.RawMessage, len(detail))
	for key, value := range detail {
		if !safeAuditDetailKeys[key] || !isSafeAuditDetailValue(value) {
			continue
		}
		sanitized[key] = append(json.RawMessage(nil), value...)
	}
	if len(sanitized) == 0 {
		return nil
	}

	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return nil
	}
	return json.RawMessage(encoded)
}

func sanitizeAuditMetadata(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" || containsSensitiveAuditValue(value) {
		return ""
	}
	runes := []rune(value)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return value
}

func isSafeAuditDetailValue(raw json.RawMessage) bool {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	switch value := value.(type) {
	case string:
		return !containsSensitiveAuditValue(value)
	case float64, bool:
		return true
	default:
		return false
	}
}

var sensitiveAuditValueAssignmentPattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:access[ _-]?token|client[ _-]?secret|user[ _-]?password|password|passcode|token|authorization|api[ _-]?key|secret|signature|signed[ _-]?url|wss[ _-]?url|media)\s*(?:["']\s*)?[:=]`)

var bearerCredentialPattern = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]{12,}`)

var dataURIBase64Pattern = regexp.MustCompile(`(?i)\bdata:?(?:[^;,]+)?;base64`)

func containsSensitiveAuditValue(value string) bool {
	lower := strings.ToLower(value)
	if sensitiveAuditValueAssignmentPattern.MatchString(value) ||
		bearerCredentialPattern.MatchString(value) ||
		dataURIBase64Pattern.MatchString(value) ||
		looksLikeBase64Payload(value) {
		return true
	}
	for _, marker := range []string{
		"signed_url",
		"signature=",
		"wss://",
		"ws://",
		"x-oss-signature",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func looksLikeBase64Payload(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 128 || len(trimmed)%4 != 0 {
		return false
	}
	padding := 0
	for index, character := range trimmed {
		switch {
		case character >= 'A' && character <= 'Z', character >= 'a' && character <= 'z', character >= '0' && character <= '9', character == '+', character == '/':
			if padding > 0 {
				return false
			}
		case character == '=':
			if index < len(trimmed)-2 {
				return false
			}
			padding++
		default:
			return false
		}
	}
	return padding <= 2
}

func cloneAuditLog(log AuditLog) AuditLog {
	clone := log
	clone.UserID = cloneAuditLogID(log.UserID)
	clone.EntityID = cloneAuditLogID(log.EntityID)
	clone.StoreID = cloneAuditLogID(log.StoreID)
	clone.ChannelID = cloneAuditLogID(log.ChannelID)
	clone.DetailJSON = append(json.RawMessage(nil), log.DetailJSON...)
	return clone
}

func cloneAuditLogID(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
