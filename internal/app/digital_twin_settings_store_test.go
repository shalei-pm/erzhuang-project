package app

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

func TestDigitalTwinAuditFullLists(t *testing.T) {
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = strings.Repeat("9", 15) + fmt.Sprintf("%03d", i)
	}
	normalized, err := NormalizeDigitalTwinStoreIDs(ids)
	if err != nil {
		t.Fatal(err)
	}
	previous := digitalTwinSettingsValue(normalized)
	next := digitalTwinSettingsValue([]string{})
	event := digitalTwinSettingsAudit(auditlog.AuditEvent{DetailJSON: json.RawMessage(`{"summary":"forged"}`)}, previous, next)
	if event.Action != "system.digital_twin_whitelist.update" {
		t.Fatalf("unexpected action: %s", event.Action)
	}
	if !reflect.DeepEqual(event.DetailJSON, sanitizeAuditDetail(event.DetailJSON)) {
		t.Fatal("summary was filtered")
	}
	var detail map[string]string
	if err := json.Unmarshal(event.DetailJSON, &detail); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if !strings.Contains(detail["summary"], id) {
			t.Fatal("audit lost an ID")
		}
	}
	if !strings.HasPrefix(detail["summary"], "数字孪生白名单：原机构ID=[") || !strings.HasSuffix(detail["summary"], "；新机构ID=[]") {
		t.Fatal("empty list missing from audit")
	}
	if strings.Contains(detail["summary"], previous.Version) {
		t.Fatal("audit must not expose hash version")
	}
}

func TestDigitalTwinAuditDefaultSource(t *testing.T) {
	event := digitalTwinSettingsAudit(auditlog.AuditEvent{}, defaultDigitalTwinSettings(), digitalTwinSettingsValue([]string{}))
	var detail map[string]string
	if err := json.Unmarshal(event.DetailJSON, &detail); err != nil {
		t.Fatal(err)
	}
	if detail["summary"] != `数字孪生白名单：原机构ID=["10001"]；新机构ID=[]；原配置来源=系统默认` {
		t.Fatalf("unexpected default audit: %s", detail["summary"])
	}
}

func TestMySQLDigitalTwinUpdateExistingAndFailures(t *testing.T) {
	ctx := context.Background()
	old := digitalTwinSettingsValue([]string{"10001"})
	s := NewMySQLStore(newSessionScriptDB(t,
		sessionSQLStep{op: "begin"},
		sessionSQLStep{op: "query", contains: "for update", rows: []driver.Value{`["10001"]`}},
		sessionSQLStep{op: "exec", contains: "update tb_app_settings", args: []any{`["2"]`, digitalTwinSettingsKey}, affected: 1},
		sessionSQLStep{op: "exec", contains: "insert into tb_audit_logs", affected: 1},
		sessionSQLStep{op: "commit"},
	))
	got, err := s.UpdateDigitalTwinSettings(ctx, old.Version, []string{"2"}, auditlog.AuditEvent{})
	if err != nil || !reflect.DeepEqual(got.StoreIDs, []string{"2"}) {
		t.Fatalf("%+v %v", got, err)
	}
	for _, number := range []uint16{1062, 1213} {
		s := NewMySQLStore(newSessionScriptDB(t,
			sessionSQLStep{op: "begin"}, sessionSQLStep{op: "query", contains: "for update"},
			sessionSQLStep{op: "exec", contains: "insert into tb_app_settings", err: &mysql.MySQLError{Number: number}}, sessionSQLStep{op: "rollback"},
		))
		if _, err := s.UpdateDigitalTwinSettings(ctx, defaultDigitalTwinSettings().Version, nil, auditlog.AuditEvent{}); !errors.Is(err, ErrDigitalTwinSettingsConflict) {
			t.Fatalf("race %d: %v", number, err)
		}
	}
	s = NewMySQLStore(newSessionScriptDB(t, sessionSQLStep{op: "query", contains: "tb_app_settings", err: errors.New("database unavailable")}))
	if got, err := s.GetDigitalTwinSettings(ctx); err == nil || len(got.StoreIDs) != 0 {
		t.Fatalf("read failed open: %+v %v", got, err)
	}
	if defaultDigitalTwinSettings().Version == old.Version {
		t.Fatal("missing and saved versions must differ")
	}
}

func TestNormalizeDigitalTwinStoreIDs(t *testing.T) {
	limitIDs := make([]string, 100)
	for i := range limitIDs {
		limitIDs[i] = fmt.Sprint(i + 1)
	}
	if got, err := NormalizeDigitalTwinStoreIDs(limitIDs); err != nil || len(got) != 100 {
		t.Fatalf("100 IDs must be accepted: count=%d err=%v", len(got), err)
	}
	if _, err := NormalizeDigitalTwinStoreIDs(append(limitIDs, "101")); err == nil {
		t.Fatal("101 valid IDs must be rejected")
	}
	if _, err := NormalizeDigitalTwinStoreIDs([]string{strings.Repeat("9", 18)}); err != nil {
		t.Fatalf("18-digit ID must be accepted: %v", err)
	}
	for _, ids := range [][]string{{"0"}, {"01"}, {"-1"}, {"+1"}, {" 1"}, {"1 "}, {"1.0"}, {"１"}, {""}, {strings.Repeat("1", 19)}, make([]string, 101)} {
		if _, err := NormalizeDigitalTwinStoreIDs(ids); err == nil {
			t.Fatalf("accepted invalid IDs: %v", ids)
		}
	}
	got, err := NormalizeDigitalTwinStoreIDs([]string{"10", "2", "2", "1"})
	if err != nil || !reflect.DeepEqual(got, []string{"1", "2", "10"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	got, err = NormalizeDigitalTwinStoreIDs(nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty = %#v, %v", got, err)
	}
}

func TestMemoryDigitalTwinSettingsAtomic(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	initial, err := s.GetDigitalTwinSettings(ctx)
	if err != nil || !reflect.DeepEqual(initial.StoreIDs, []string{"10001"}) {
		t.Fatalf("default: %+v %v", initial, err)
	}
	initial.StoreIDs[0] = "999"
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.UpdateDigitalTwinSettings(ctx, initial.Version, []string{}, auditlog.AuditEvent{DetailJSON: json.RawMessage(`{"summary":"forged old value"}`)})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrDigitalTwinSettingsConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 || len(s.auditLogs) != 1 {
		t.Fatalf("success=%d conflict=%d audits=%d", success, conflict, len(s.auditLogs))
	}
	current, err := s.GetDigitalTwinSettings(ctx)
	if err != nil || current.StoreIDs == nil || len(current.StoreIDs) != 0 || current.Version == initial.Version {
		t.Fatalf("saved empty: %+v %v", current, err)
	}
	if !strings.Contains(string(s.auditLogs[0].DetailJSON), "10001") || strings.Contains(string(s.auditLogs[0].DetailJSON), "forged") {
		t.Fatalf("audit: %s", s.auditLogs[0].DetailJSON)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.UpdateDigitalTwinSettings(canceled, current.Version, []string{"2"}, auditlog.AuditEvent{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(s.auditLogs) != 1 {
		t.Fatal("canceled mutation wrote audit")
	}
}

func TestMySQLDigitalTwinSettingsReads(t *testing.T) {
	for _, raw := range []string{"null", "", `{}`, `[1]`, `["01"]`, `["1"] true`} {
		t.Run(raw, func(t *testing.T) {
			s := NewMySQLStore(newSessionScriptDB(t, sessionSQLStep{op: "query", contains: "tb_app_settings", rows: []driver.Value{raw}}))
			if _, err := s.GetDigitalTwinSettings(context.Background()); err == nil {
				t.Fatal("accepted corrupt setting")
			}
		})
	}
	s := NewMySQLStore(newSessionScriptDB(t, sessionSQLStep{op: "query", contains: "tb_app_settings"}))
	got, err := s.GetDigitalTwinSettings(context.Background())
	if err != nil || !reflect.DeepEqual(got.StoreIDs, []string{"10001"}) {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestMySQLDigitalTwinSettingsTransactions(t *testing.T) {
	ctx := context.Background()
	initial, _ := NewMemoryStore().GetDigitalTwinSettings(ctx)
	for _, auditFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "audit rollback"}[auditFails], func(t *testing.T) {
			steps := []sessionSQLStep{{op: "begin"}, {op: "query", contains: "for update"}, {op: "exec", contains: "insert into tb_app_settings", affected: 1}, {op: "exec", contains: "insert into tb_audit_logs", affected: 1}}
			if auditFails {
				steps[3].err = errors.New("audit unavailable")
				steps = append(steps, sessionSQLStep{op: "rollback"})
			} else {
				steps = append(steps, sessionSQLStep{op: "commit"})
			}
			s := NewMySQLStore(newSessionScriptDB(t, steps...))
			got, err := s.UpdateDigitalTwinSettings(ctx, initial.Version, []string{}, auditlog.AuditEvent{})
			if (err != nil) != auditFails {
				t.Fatalf("result=%+v err=%v", got, err)
			}
		})
	}
	s := NewMySQLStore(newSessionScriptDB(t, sessionSQLStep{op: "begin"}, sessionSQLStep{op: "query", contains: "for update", rows: []driver.Value{`[]`}}, sessionSQLStep{op: "rollback"}))
	if _, err := s.UpdateDigitalTwinSettings(ctx, initial.Version, []string{"2"}, auditlog.AuditEvent{}); !errors.Is(err, ErrDigitalTwinSettingsConflict) {
		t.Fatal(err)
	}
}
