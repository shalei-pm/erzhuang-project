package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/go-sql-driver/mysql"
	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

const digitalTwinSettingsKey = "digital_twin_store_ids"

var ErrDigitalTwinSettingsConflict = errors.New("digital twin settings conflict")

type DigitalTwinSettings struct {
	StoreIDs []string `json:"store_ids"`
	Version  string   `json:"version"`
}

type DigitalTwinSettingsStore interface {
	GetDigitalTwinSettings(context.Context) (DigitalTwinSettings, error)
	UpdateDigitalTwinSettings(context.Context, string, []string, auditlog.AuditEvent) (DigitalTwinSettings, error)
}

var _ DigitalTwinSettingsStore = (*MySQLStore)(nil)
var _ DigitalTwinSettingsStore = (*MemoryStore)(nil)

// NormalizeDigitalTwinStoreIDs accepts canonical positive decimal IDs, not numbers.
// The 18-digit limit matches frontend validation and fits signed int64 tenant IDs.
func NormalizeDigitalTwinStoreIDs(ids []string) ([]string, error) {
	if len(ids) > 100 {
		return nil, errors.New("at most 100 digital twin store IDs allowed")
	}
	unique := make(map[string]bool, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if len(id) == 0 || len(id) > 18 || id[0] < '1' || id[0] > '9' {
			return nil, errors.New("invalid digital twin store ID")
		}
		for i := 1; i < len(id); i++ {
			if id[i] < '0' || id[i] > '9' {
				return nil, errors.New("invalid digital twin store ID")
			}
		}
		if !unique[id] {
			unique[id] = true
			result = append(result, id)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if len(result[i]) != len(result[j]) {
			return len(result[i]) < len(result[j])
		}
		return result[i] < result[j]
	})
	return result, nil
}

func digitalTwinSettingsValue(ids []string) DigitalTwinSettings {
	value, _ := json.Marshal(ids)
	return DigitalTwinSettings{StoreIDs: ids, Version: fmt.Sprintf("saved:%x", sha256.Sum256(value))}
}

func defaultDigitalTwinSettings() DigitalTwinSettings {
	return DigitalTwinSettings{StoreIDs: []string{"10001"}, Version: "missing:10001"}
}

func parseDigitalTwinSettings(raw string) (DigitalTwinSettings, error) {
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return DigitalTwinSettings{}, fmt.Errorf("invalid digital twin settings JSON: %w", err)
	}
	if ids == nil {
		return DigitalTwinSettings{}, errors.New("digital twin settings must be an array")
	}
	ids, err := NormalizeDigitalTwinStoreIDs(ids)
	if err != nil {
		return DigitalTwinSettings{}, err
	}
	return digitalTwinSettingsValue(ids), nil
}

func digitalTwinSettingsAudit(event auditlog.AuditEvent, previous, next DigitalTwinSettings) auditlog.AuditEvent {
	oldJSON, _ := json.Marshal(previous.StoreIDs)
	newJSON, _ := json.Marshal(next.StoreIDs)
	event.Action = "system.digital_twin_whitelist.update"
	event.EntityType = "system_setting"
	event.EntityID, event.StoreID, event.ChannelID = nil, nil, nil
	event.ExternalOrgID, event.AssetLogicalKey = "", ""
	event.Result = "success"
	summary := fmt.Sprintf("数字孪生白名单：原机构ID=%s；新机构ID=%s", oldJSON, newJSON)
	if previous.Version == defaultDigitalTwinSettings().Version {
		summary += "；原配置来源=系统默认"
	}
	event.DetailJSON, _ = json.Marshal(map[string]string{"summary": summary})
	return event
}

func (s *MySQLStore) GetDigitalTwinSettings(ctx context.Context) (DigitalTwinSettings, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, "select value from tb_app_settings where `key` = ?", digitalTwinSettingsKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultDigitalTwinSettings(), nil
	}
	if err != nil {
		return DigitalTwinSettings{}, err
	}
	return parseDigitalTwinSettings(raw)
}

func (s *MySQLStore) UpdateDigitalTwinSettings(ctx context.Context, expectedVersion string, ids []string, event auditlog.AuditEvent) (DigitalTwinSettings, error) {
	ids, err := NormalizeDigitalTwinStoreIDs(ids)
	if err != nil {
		return DigitalTwinSettings{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DigitalTwinSettings{}, err
	}
	defer tx.Rollback()
	var raw string
	err = tx.QueryRowContext(ctx, "select value from tb_app_settings where `key` = ? for update", digitalTwinSettingsKey).Scan(&raw)
	missing := errors.Is(err, sql.ErrNoRows)
	previous := defaultDigitalTwinSettings()
	if !missing {
		if err != nil {
			return DigitalTwinSettings{}, digitalTwinWriteError(err)
		}
		previous, err = parseDigitalTwinSettings(raw)
		if err != nil {
			return DigitalTwinSettings{}, err
		}
	}
	if previous.Version != expectedVersion {
		return DigitalTwinSettings{}, ErrDigitalTwinSettingsConflict
	}
	next := digitalTwinSettingsValue(ids)
	value, _ := json.Marshal(ids)
	if missing {
		_, err = tx.ExecContext(ctx, "insert into tb_app_settings (`key`, value, updated_at) values (?, ?, current_timestamp(3))", digitalTwinSettingsKey, string(value))
	} else {
		_, err = tx.ExecContext(ctx, "update tb_app_settings set value = ?, updated_at = current_timestamp(3) where `key` = ?", string(value), digitalTwinSettingsKey)
	}
	if err != nil {
		return DigitalTwinSettings{}, digitalTwinWriteError(err)
	}
	if err := s.RecorderForTx(tx).RecordAudit(ctx, digitalTwinSettingsAudit(event, previous, next)); err != nil {
		return DigitalTwinSettings{}, err
	}
	if err := tx.Commit(); err != nil {
		return DigitalTwinSettings{}, err
	}
	return next, nil
}

// Missing-row races can surface as duplicate keys or gap-lock deadlocks.
func digitalTwinWriteError(err error) error {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && (mysqlErr.Number == 1062 || mysqlErr.Number == 1213) {
		return fmt.Errorf("%w: %v", ErrDigitalTwinSettingsConflict, err)
	}
	return err
}

func (s *MemoryStore) GetDigitalTwinSettings(ctx context.Context) (DigitalTwinSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return DigitalTwinSettings{}, err
	}
	if s.digitalTwinSettingsJSON == nil {
		return defaultDigitalTwinSettings(), nil
	}
	return parseDigitalTwinSettings(*s.digitalTwinSettingsJSON)
}

func (s *MemoryStore) UpdateDigitalTwinSettings(ctx context.Context, expectedVersion string, ids []string, event auditlog.AuditEvent) (DigitalTwinSettings, error) {
	ids, err := NormalizeDigitalTwinStoreIDs(ids)
	if err != nil {
		return DigitalTwinSettings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return DigitalTwinSettings{}, err
	}
	previous := defaultDigitalTwinSettings()
	if s.digitalTwinSettingsJSON != nil {
		previous, err = parseDigitalTwinSettings(*s.digitalTwinSettingsJSON)
		if err != nil {
			return DigitalTwinSettings{}, err
		}
	}
	if previous.Version != expectedVersion {
		return DigitalTwinSettings{}, ErrDigitalTwinSettingsConflict
	}
	next := digitalTwinSettingsValue(ids)
	if err := s.createAuditLogLocked(ctx, digitalTwinSettingsAudit(event, previous, next)); err != nil {
		return DigitalTwinSettings{}, err
	}
	value, _ := json.Marshal(ids)
	raw := string(value)
	s.digitalTwinSettingsJSON = &raw
	return next, nil
}
