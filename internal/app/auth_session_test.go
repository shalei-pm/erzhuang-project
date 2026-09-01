package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type fakeAuthSession struct {
	userID       int64
	createdAt    time.Time
	lastActivity time.Time
	expiresAt    time.Time
	revokedAt    time.Time
	revokeReason string
}

type fakeAuthSessionStore struct {
	mu       sync.Mutex
	sessions map[string]fakeAuthSession
}

func newFakeAuthSessionStore() *fakeAuthSessionStore {
	return &fakeAuthSessionStore{sessions: make(map[string]fakeAuthSession)}
}

func (s *fakeAuthSessionStore) CreateAuthSession(_ context.Context, input AuthSessionCreate) (string, error) {
	token, err := newAuthSessionToken()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	hash := hashAuthSessionToken(token)
	s.sessions[hex.EncodeToString(hash[:])] = fakeAuthSession{
		userID:       input.UserID,
		createdAt:    input.Now,
		lastActivity: input.Now,
		expiresAt:    input.Now.Add(defaultAuthIdleTimeout),
	}
	return token, nil
}

func (s *fakeAuthSessionStore) TouchAuthSession(_ context.Context, token string, userID int64, now time.Time, idleTimeout time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash := hashAuthSessionToken(token)
	session, ok := s.sessions[hex.EncodeToString(hash[:])]
	if !ok || session.userID != userID || !now.Before(session.expiresAt) || !session.revokedAt.IsZero() {
		return false, nil
	}
	if !now.After(session.lastActivity) {
		return true, nil
	}
	session.lastActivity = now
	session.expiresAt = now.Add(idleTimeout)
	s.sessions[hex.EncodeToString(hash[:])] = session
	return true, nil
}

func (s *fakeAuthSessionStore) RevokeAuthSession(_ context.Context, token string, userID int64, reason string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash := hashAuthSessionToken(token)
	key := hex.EncodeToString(hash[:])
	session, ok := s.sessions[key]
	if ok && session.userID == userID && session.revokedAt.IsZero() {
		session.revokedAt = now
		session.revokeReason = reason
		s.sessions[key] = session
	}
	return nil
}

func TestAuthSessionTokenHasSufficientEntropy(t *testing.T) {
	first, err := newAuthSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newAuthSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || second == "" || first == second {
		t.Fatalf("tokens must be non-empty and unique: first_len=%d second_len=%d equal=%t", len(first), len(second), first == second)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil {
		t.Fatalf("token is not valid raw base64url: %v", err)
	}
	if got := len(decoded); got < 32 {
		t.Fatalf("decoded token length = %d, want at least 32 bytes", got)
	}
}

func TestAuthSessionTouchBeforeIdleTimeout(t *testing.T) {
	store := newFakeAuthSessionStore()
	createdAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	token, err := store.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 7, Now: createdAt})
	if err != nil {
		t.Fatal(err)
	}

	ok, err := store.TouchAuthSession(context.Background(), token, 7, createdAt.Add(29*time.Minute+59*time.Second), defaultAuthIdleTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("touch at 29:59 should succeed")
	}
}

func TestAuthSessionTouchAfterIdleTimeout(t *testing.T) {
	store := newFakeAuthSessionStore()
	createdAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	token, err := store.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 7, Now: createdAt})
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		now  time.Time
	}{
		{name: "exactly 30 minutes", now: createdAt.Add(30 * time.Minute)},
		{name: "30 minutes and 1 second", now: createdAt.Add(30*time.Minute + time.Second)},
	} {
		t.Run(test.name, func(t *testing.T) {
			ok, err := store.TouchAuthSession(context.Background(), token, 7, test.now, defaultAuthIdleTimeout)
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				t.Fatal("touch at or after 30 minutes should fail")
			}
		})
	}
}

func TestAuthSessionTouchDoesNotMoveActivityBackward(t *testing.T) {
	store := newFakeAuthSessionStore()
	createdAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	token, err := store.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 7, Now: createdAt})
	if err != nil {
		t.Fatal(err)
	}

	later := createdAt.Add(10 * time.Minute)
	ok, err := store.TouchAuthSession(context.Background(), token, 7, later, defaultAuthIdleTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("touch at the later time should succeed")
	}

	earlier := createdAt.Add(5 * time.Minute)
	ok, err = store.TouchAuthSession(context.Background(), token, 7, earlier, defaultAuthIdleTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("touch with an earlier timestamp should still recognize the active session")
	}

	hash := hashAuthSessionToken(token)
	key := hex.EncodeToString(hash[:])
	store.mu.Lock()
	session := store.sessions[key]
	store.mu.Unlock()
	if !session.lastActivity.Equal(later) {
		t.Fatalf("last activity moved backward: got=%s want=%s", session.lastActivity, later)
	}
	if !session.expiresAt.Equal(later.Add(defaultAuthIdleTimeout)) {
		t.Fatalf("expiry moved backward: got=%s want=%s", session.expiresAt, later.Add(defaultAuthIdleTimeout))
	}
}

func TestAuthSessionRevokedCannotBeTouched(t *testing.T) {
	store := newFakeAuthSessionStore()
	createdAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	token, err := store.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 7, Now: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeAuthSession(context.Background(), token, 7, "manual_logout", createdAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	ok, err := store.TouchAuthSession(context.Background(), token, 7, createdAt.Add(2*time.Minute), defaultAuthIdleTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("revoked session should not be touchable")
	}
}

func TestAuthSessionFakeDoesNotStoreRawToken(t *testing.T) {
	store := newFakeAuthSessionStore()
	token, err := store.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 7, Now: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for key := range store.sessions {
		if key == token || strings.Contains(key, token) {
			t.Fatalf("fake storage contains raw token: raw_token_present=true stored_items=%d", len(store.sessions))
		}
	}
	if got := len(store.sessions); got != 1 {
		t.Fatalf("stored sessions = %d, want 1", got)
	}
}

func TestMySQLAuthSessionPersistenceSQLContract(t *testing.T) {
	recorder := newRecordingSQLDriver(t)
	db, err := sql.Open(recorder.driverName, "")
	if err != nil {
		t.Fatalf("open recording db: %v", err)
	}
	defer db.Close()

	store := NewMySQLStore(db)
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	token, err := store.CreateAuthSession(context.Background(), AuthSessionCreate{
		UserID:    7,
		SSOSubject: "subject-7",
		IPAddress: "192.0.2.1",
		UserAgent: "contract-test",
		Now:       now,
	})
	if err != nil {
		t.Fatalf("create auth session: %v", err)
	}
	if _, err := store.TouchAuthSession(context.Background(), token, 7, now.Add(time.Minute), defaultAuthIdleTimeout); err != nil {
		t.Fatalf("touch auth session: %v", err)
	}
	if err := store.RevokeAuthSession(context.Background(), token, 7, "manual_logout", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("revoke auth session: %v", err)
	}

	queries := recorder.queries()
	calls := recorder.calls()
	if len(queries) != 3 || len(calls) != 3 {
		t.Fatalf("executed calls = %d, want 3", len(calls))
	}
	for _, query := range queries {
		if strings.Contains(query, token) {
			t.Fatalf("raw session token was interpolated into SQL: raw_token_present=true")
		}
	}
	expectedHash := hashAuthSessionToken(token)
	expectedHashText := hex.EncodeToString(expectedHash[:])
	if len(calls[0].args) != 8 || calls[0].args[0].Value != expectedHashText || calls[0].args[1].Value != int64(7) || calls[0].args[2].Value != "subject-7" {
		t.Fatalf("create bindings do not contain the expected hash and metadata: arg_count=%d", len(calls[0].args))
	}
	if len(calls[1].args) != 2 || calls[1].args[0].Value != expectedHashText || calls[1].args[1].Value != int64(7) {
		t.Fatalf("touch bindings do not contain the expected hash and user id: arg_count=%d", len(calls[1].args))
	}
	if len(calls[2].args) != 4 || calls[2].args[1].Value != "manual_logout" || calls[2].args[2].Value != expectedHashText || calls[2].args[3].Value != int64(7) {
		t.Fatalf("revoke bindings do not contain the expected hash, user id, and reason: arg_count=%d", len(calls[2].args))
	}
	for _, call := range calls {
		for _, arg := range call.args {
			if value, ok := arg.Value.(string); ok && value == token {
				t.Fatal("raw session token was passed as a SQL binding")
			}
		}
	}
	for _, want := range []string{
		"insert into tb_auth_sessions",
		"session_token_hash",
		"created_at, last_activity_at, expires_at",
	} {
		if !strings.Contains(queries[0], want) {
			t.Fatalf("create query missing %q: %s", want, queries[0])
		}
	}
	for _, want := range []string{
		"select 1",
		"session_token_hash = ?",
		"user_id = ?",
		"revoked_at is null",
		"expires_at > utc_timestamp(3)",
		"for update",
	} {
		if !strings.Contains(mysqlAuthSessionValidSQL, want) {
			t.Fatalf("touch fallback query missing %q: %s", want, mysqlAuthSessionValidSQL)
		}
	}
	for _, want := range []string{
		"update tb_auth_sessions",
		"last_activity_at = greatest(last_activity_at, utc_timestamp(3))",
		"expires_at = greatest(expires_at, date_add(utc_timestamp(3), interval 30 minute))",
		"session_token_hash = ?",
		"user_id = ?",
		"revoked_at is null",
		"expires_at > utc_timestamp(3)",
	} {
		if !strings.Contains(queries[1], want) {
			t.Fatalf("touch query missing %q: %s", want, queries[1])
		}
	}
	for _, want := range []string{
		"revoked_at = ?",
		"revoked_reason = ?",
		"session_token_hash = ?",
		"user_id = ?",
		"revoked_at is null",
	} {
		if !strings.Contains(queries[2], want) {
			t.Fatalf("revoke query missing %q: %s", want, queries[2])
		}
	}
}

func TestMySQLAuthSessionTouchZeroRowsConfirmsActiveSession(t *testing.T) {
	recorder := newRecordingSQLDriver(t)
	db, err := sql.Open(recorder.driverName, "")
	if err != nil {
		t.Fatalf("open recording db: %v", err)
	}
	defer db.Close()

	store := NewMySQLStore(db)
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	token, err := store.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 7, Now: now})
	if err != nil {
		t.Fatalf("create auth session: %v", err)
	}
	recorder.setExecRowsAffected(0)

	for _, touchAt := range []time.Time{now, now.Add(-time.Minute)} {
		ok, err := store.TouchAuthSession(context.Background(), token, 7, touchAt, defaultAuthIdleTimeout)
		if err != nil {
			t.Fatalf("touch auth session at %s: %v", touchAt, err)
		}
		if !ok {
			t.Fatalf("active session should remain valid when conditional update affects zero rows at %s", touchAt)
		}
	}

	calls := recorder.calls()
	if len(calls) != 5 {
		t.Fatalf("executed calls = %d, want create plus two update/select pairs", len(calls))
	}
	if !strings.Contains(calls[2].query, "select 1") || !strings.Contains(calls[4].query, "select 1") {
		t.Fatalf("zero-row touches must confirm with the active-session select: calls=%#v", calls)
	}
}

func TestMySQLAuthSessionTouchIgnoresCallerTimeoutAndClock(t *testing.T) {
	recorder := newRecordingSQLDriver(t)
	db, err := sql.Open(recorder.driverName, "")
	if err != nil {
		t.Fatalf("open recording db: %v", err)
	}
	defer db.Close()

	store := NewMySQLStore(db)
	token, err := store.CreateAuthSession(context.Background(), AuthSessionCreate{
		UserID: 7,
		Now:    time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create auth session: %v", err)
	}

	callerTime := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := store.TouchAuthSession(context.Background(), token, 7, callerTime, 4*time.Hour); err != nil {
		t.Fatalf("touch auth session: %v", err)
	}

	calls := recorder.calls()
	if len(calls) != 2 {
		t.Fatalf("executed calls = %d, want create and touch", len(calls))
	}
	touchQuery := calls[1].query
	for _, want := range []string{
		"utc_timestamp(3)",
		"date_add(utc_timestamp(3), interval 30 minute)",
		"expires_at > utc_timestamp(3)",
	} {
		if !strings.Contains(touchQuery, want) {
			t.Fatalf("touch query missing database-clock fixed window %q: %s", want, touchQuery)
		}
	}
	if strings.Contains(touchQuery, "interval ?") || strings.Contains(touchQuery, "date_add(?,") {
		t.Fatalf("touch query must not accept a caller-controlled timeout or clock: %s", touchQuery)
	}
	if len(calls[1].args) != 2 || calls[1].args[0].Value == token {
		t.Fatalf("touch must bind only the hashed token and user id: arg_count=%d", len(calls[1].args))
	}
}

func TestMySQLAuthSessionTouchZeroRowsRejectsInvalidStates(t *testing.T) {
	for _, state := range []string{"expired", "revoked", "missing"} {
		t.Run(state, func(t *testing.T) {
			recorder := newRecordingSQLDriver(t)
			db, err := sql.Open(recorder.driverName, "")
			if err != nil {
				t.Fatalf("open recording db: %v", err)
			}
			defer db.Close()

			recorder.setExecRowsAffected(0)
			recorder.setQueryExists(false)
			ok, err := NewMySQLStore(db).TouchAuthSession(
				context.Background(), "invalid-state-token", 7,
				time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), defaultAuthIdleTimeout,
			)
			if err != nil {
				t.Fatalf("touch invalid %s session: %v", state, err)
			}
			if ok {
				t.Fatalf("%s session must not be treated as active", state)
			}
			calls := recorder.calls()
			if len(calls) != 2 || !strings.Contains(calls[1].query, "select 1") {
				t.Fatalf("invalid state must execute update plus confirmation select: calls=%#v", calls)
			}
		})
	}
}

func TestMySQLAuthSessionIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN is not set")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MYSQL_TEST_DSN: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping MYSQL_TEST_DSN: %v", err)
	}

	ok, err := NewMySQLStore(db).TouchAuthSession(ctx, "task2-integration-probe", -1, time.Now(), 24*time.Hour)
	if err != nil {
		t.Fatalf("touch auth-session probe: %v", err)
	}
	if ok {
		t.Fatal("nonexistent integration probe session must not be active")
	}
}

func TestMySQLAuthSessionSchemaAndMigrationContract(t *testing.T) {
	schema := readAuthSessionTestFile(t, filepath.Join("..", "..", "db", "mysql_governance_schema_tb.sql"))
	migration := readAuthSessionTestFile(t, filepath.Join("..", "..", "db", "mysql_auth_sessions.sql"))
	for _, want := range []string{
		"create table if not exists tb_auth_sessions",
		"last_activity_at datetime(3) not null",
		"key idx_tb_auth_sessions_user_activity (user_id, last_activity_at)",
		"key idx_tb_auth_sessions_expires_at (expires_at)",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("governance schema missing %q", want)
		}
	}
	for _, want := range []string{
		"from information_schema.tables",
		"from information_schema.columns",
		"from information_schema.statistics",
		"drop procedure if exists erzhuang_migrate_tb_auth_sessions_tmp",
		"if v_table_exists = 0 then",
		"create table tb_auth_sessions",
		"add column last_activity_at datetime(3) null",
		"set last_activity_at = created_at",
		"modify column last_activity_at datetime(3) not null",
		"if v_index_exists = 0 then",
		"call erzhuang_migrate_tb_auth_sessions_tmp()",
		"drop procedure erzhuang_migrate_tb_auth_sessions_tmp",
		"v_missing_columns",
		"v_null_created_at",
		"NULL created_at",
		"duplicate session_token_hash",
		"wrong uniqueness or column order",
		"wrong columns or order",
		"application startup",
	} {
		if !strings.Contains(migration, want) {
			t.Fatalf("auth-session migration missing %q", want)
		}
	}
	for _, secretMarker := range []string{"Authorization:", "password", "MYSQL_DSN", "wss://", "eyJ"} {
		if strings.Contains(migration, secretMarker) {
			t.Fatalf("auth-session migration contains prohibited secret marker %q", secretMarker)
		}
	}

	mysqlSource := readAuthSessionTestFile(t, "mysql_store.go")
	if strings.Contains(mysqlSource, "log.Print") || strings.Contains(mysqlSource, "log.Printf") {
		t.Fatal("MySQL auth-session implementation must not log session tokens")
	}
}

func readAuthSessionTestFile(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}
