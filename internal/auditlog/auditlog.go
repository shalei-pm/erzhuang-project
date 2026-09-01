// Package auditlog defines the shared write-side contract for audit events.
package auditlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// AuditEvent describes one security-relevant business action to be recorded.
// DetailJSON must not contain credentials, signed URLs, or other sensitive payloads.
type AuditEvent struct {
	ID               int64           `json:"id"`
	UserID           *int64          `json:"user_id,omitempty"`
	ActorDisplayName string          `json:"actor_display_name"`
	UserEmail        string          `json:"user_email"`
	Action           string          `json:"action"`
	EntityType       string          `json:"entity_type"`
	EntityID         *int64          `json:"entity_id,omitempty"`
	StoreID          *int64          `json:"store_id,omitempty"`
	ExternalOrgID    string          `json:"external_org_id"`
	ChannelID        *int64          `json:"channel_id,omitempty"`
	AssetLogicalKey  string          `json:"-"`
	IPAddress        string          `json:"-"`
	UserAgent        string          `json:"-"`
	RequestID        string          `json:"-"`
	Result           string          `json:"result"`
	DetailJSON       json.RawMessage `json:"-"`
	CreatedAt        time.Time       `json:"created_at"`
}

// AuditRecorder persists an audit event outside a caller-owned SQL transaction.
type AuditRecorder interface {
	RecordAudit(ctx context.Context, event AuditEvent) error
}

// TxRecorder obtains an AuditRecorder that writes through the supplied SQL transaction.
// The caller owns the transaction lifecycle and must commit or roll it back.
type TxRecorder interface {
	AuditRecorder
	RecorderForTx(tx *sql.Tx) AuditRecorder
}
