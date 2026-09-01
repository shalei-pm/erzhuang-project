package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
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

func (s *fakeAuthSessionStore) TouchAuthSession(_ context.Context, token string, userID int64, now time.Time, _ time.Duration) (bool, error) {
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
	session.expiresAt = now.Add(defaultAuthIdleTimeout)
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

func TestAuthSessionTouchNotFound(t *testing.T) {
	store := newFakeAuthSessionStore()
	ok, err := store.TouchAuthSession(
		context.Background(), "missing-session-token", 7,
		time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), 4*time.Hour,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("missing session should not be touchable")
	}
}

func TestAuthSessionTouchUsesFixedIdleTimeout(t *testing.T) {
	store := newFakeAuthSessionStore()
	createdAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	token, err := store.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 7, Now: createdAt})
	if err != nil {
		t.Fatal(err)
	}

	touchAt := createdAt.Add(time.Minute)
	ok, err := store.TouchAuthSession(context.Background(), token, 7, touchAt, 4*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("active session should be touchable")
	}

	hash := hashAuthSessionToken(token)
	key := hex.EncodeToString(hash[:])
	store.mu.Lock()
	expiresAt := store.sessions[key].expiresAt
	store.mu.Unlock()
	if !expiresAt.Equal(touchAt.Add(defaultAuthIdleTimeout)) {
		t.Fatalf("fake store accepted caller timeout: got=%s want=%s", expiresAt, touchAt.Add(defaultAuthIdleTimeout))
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
	if len(calls[0].args) != 5 || calls[0].args[0].Value != expectedHashText || calls[0].args[1].Value != int64(7) || calls[0].args[2].Value != "subject-7" {
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
		"utc_timestamp(3)",
		"date_add(utc_timestamp(3), interval 30 minute)",
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
		t.Fatalf("zero-row touches must confirm with the active-session select: %s", summarizeSQLCalls(calls))
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
				t.Fatalf("invalid state must execute update plus confirmation select: %s", summarizeSQLCalls(calls))
			}
		})
	}
}

func TestMySQLAuthSessionIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN is not set")
	}
	if !isSafeMySQLTestDSN(dsn) {
		t.Fatal("MYSQL_TEST_DSN must identify an isolated test database and must not look like production")
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

	var tableExists int
	if err := db.QueryRowContext(ctx, `
		select count(*)
		from information_schema.tables
		where table_schema = database() and table_name = 'tb_auth_sessions'
	`).Scan(&tableExists); err != nil {
		t.Fatalf("check tb_auth_sessions: %v", err)
	}
	if tableExists != 1 {
		t.Fatal("tb_auth_sessions is required for the integration test")
	}

	const requiredColumnCount = 11
	var columnCount int
	if err := db.QueryRowContext(ctx, `
		select count(*)
		from information_schema.columns
		where table_schema = database()
		  and table_name = 'tb_auth_sessions'
		  and column_name in (
			'id', 'session_token_hash', 'user_id', 'sso_subject', 'ip_address',
			'user_agent', 'created_at', 'last_activity_at', 'expires_at',
			'revoked_at', 'revoked_reason'
		  )
	`).Scan(&columnCount); err != nil {
		t.Fatalf("check tb_auth_sessions columns: %v", err)
	}
	if columnCount != requiredColumnCount {
		t.Fatalf("tb_auth_sessions required columns = %d, want %d", columnCount, requiredColumnCount)
	}
	var nullCreatedAt, nullTokenHash int
	if err := db.QueryRowContext(ctx, `select count(*) from tb_auth_sessions where created_at is null`).Scan(&nullCreatedAt); err != nil {
		t.Fatalf("check NULL created_at preflight: %v", err)
	}
	if err := db.QueryRowContext(ctx, `select count(*) from tb_auth_sessions where session_token_hash is null`).Scan(&nullTokenHash); err != nil {
		t.Fatalf("check NULL session_token_hash preflight: %v", err)
	}
	if nullCreatedAt != 0 || nullTokenHash != 0 {
		t.Fatalf("migration preflight data checks failed: null_created_at=%d null_token_hash=%d", nullCreatedAt, nullTokenHash)
	}
	var duplicateHashes int
	if err := db.QueryRowContext(ctx, `
		select count(*)
		from (
			select session_token_hash
			from tb_auth_sessions
			group by session_token_hash
			having count(*) > 1
		) duplicate_hashes
	`).Scan(&duplicateHashes); err != nil {
		t.Fatalf("check duplicate session_token_hash preflight: %v", err)
	}
	if duplicateHashes != 0 {
		t.Fatalf("migration preflight found duplicate session_token_hash groups: %d", duplicateHashes)
	}

	var userID int64
	if err := db.QueryRowContext(ctx, `select id from tb_users order by id limit 1`).Scan(&userID); err != nil {
		t.Fatalf("find integration test user: %v", err)
	}

	store := NewMySQLStore(db)
	now := time.Now().UTC()
	token, err := store.CreateAuthSession(ctx, AuthSessionCreate{UserID: userID, Now: now})
	if err != nil {
		t.Fatalf("create auth session: %v", err)
	}
	tokenHash := hashAuthSessionToken(token)
	hashText := hex.EncodeToString(tokenHash[:])
	defer func() {
		_, _ = db.ExecContext(context.Background(), `delete from tb_auth_sessions where session_token_hash = ?`, hashText)
	}()

	ok, err := store.TouchAuthSession(ctx, token, userID, now.Add(24*time.Hour), 24*time.Hour)
	if err != nil {
		t.Fatalf("touch active auth session: %v", err)
	}
	if !ok {
		t.Fatal("new auth session must be active")
	}

	if _, err := db.ExecContext(ctx, `
		update tb_auth_sessions
		set expires_at = utc_timestamp(3) - interval 1 second
		where session_token_hash = ? and user_id = ?
	`, hashText, userID); err != nil {
		t.Fatalf("expire auth session: %v", err)
	}
	if ok, err := store.TouchAuthSession(ctx, token, userID, now, 24*time.Hour); err != nil {
		t.Fatalf("touch expired auth session: %v", err)
	} else if ok {
		t.Fatal("expired auth session must not be active")
	}

	revokedToken, err := store.CreateAuthSession(ctx, AuthSessionCreate{UserID: userID, Now: time.Now().UTC()})
	if err != nil {
		t.Fatalf("create revocation auth session: %v", err)
	}
	revokedHash := hashAuthSessionToken(revokedToken)
	revokedHashText := hex.EncodeToString(revokedHash[:])
	defer func() {
		_, _ = db.ExecContext(context.Background(), `delete from tb_auth_sessions where session_token_hash = ?`, revokedHashText)
	}()
	if err := store.RevokeAuthSession(ctx, revokedToken, userID, "integration_test", time.Now().UTC()); err != nil {
		t.Fatalf("revoke auth session: %v", err)
	}
	if ok, err := store.TouchAuthSession(ctx, revokedToken, userID, now, 24*time.Hour); err != nil {
		t.Fatalf("touch revoked auth session: %v", err)
	} else if ok {
		t.Fatal("revoked auth session must not be active")
	}
}

func TestMySQLAuthSessionMigrationIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	declaredDatabase := strings.TrimSpace(os.Getenv("MYSQL_TEST_MIGRATION_DATABASE"))
	if dsn == "" || os.Getenv("MYSQL_TEST_DSN_ALLOW_MIGRATION") != "1" || declaredDatabase == "" {
		t.Skip("MYSQL_TEST_DSN, MYSQL_TEST_DSN_ALLOW_MIGRATION=1, and MYSQL_TEST_MIGRATION_DATABASE are required")
	}
	if !isSafeMySQLMigrationDSN(dsn, declaredDatabase) {
		t.Skip("migration test requires an isolated *_migration_test database and an exact MYSQL_TEST_MIGRATION_DATABASE match")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("open MYSQL_TEST_DSN failed")
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal("ping MYSQL_TEST_DSN failed")
	}

	defer func() {
		_, _ = db.ExecContext(context.Background(), `drop procedure if exists erzhuang_migrate_tb_auth_sessions_tmp`)
		_, _ = db.ExecContext(context.Background(), `drop table if exists tb_auth_sessions`)
	}()
	if _, err := db.ExecContext(ctx, `drop table if exists tb_auth_sessions`); err != nil {
		t.Fatal("prepare isolated migration database failed")
	}

	if err := runMySQLAuthSessionMigration(ctx, db); err != nil {
		t.Fatal("fresh migration execution failed")
	}
	assertAuthSessionMigrationTable(t, ctx, db, true)

	userID := integrationAuthUserID(t, ctx, db)
	preservedCreatedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	preservedActivityAt := preservedCreatedAt.Add(15 * time.Minute)
	insertAuthSessionRow(t, ctx, db, "a", userID, preservedCreatedAt, preservedActivityAt)
	if err := runMySQLAuthSessionMigration(ctx, db); err != nil {
		t.Fatal("repeat migration with existing last_activity_at failed")
	}
	assertAuthSessionActivity(t, ctx, db, "a", preservedActivityAt)

	if _, err := db.ExecContext(ctx, `drop table tb_auth_sessions`); err != nil {
		t.Fatal("prepare duplicate preflight case failed")
	}
	if err := createLegacyAuthSessionTable(ctx, db); err != nil {
		t.Fatal("create legacy auth-session table failed")
	}
	insertLegacyAuthSessionRow(t, ctx, db, "b", userID, preservedCreatedAt)
	insertLegacyAuthSessionRow(t, ctx, db, "b", userID, preservedCreatedAt.Add(time.Minute))
	if err := runMySQLAuthSessionMigration(ctx, db); err == nil {
		t.Fatal("duplicate session_token_hash preflight must block migration")
	}
	var activityColumnCount int
	if err := db.QueryRowContext(ctx, `
		select count(*) from information_schema.columns
		where table_schema = database()
		  and table_name = 'tb_auth_sessions'
		  and column_name = 'last_activity_at'
	`).Scan(&activityColumnCount); err != nil {
		t.Fatal("check duplicate preflight side effects failed")
	}
	if activityColumnCount != 0 {
		t.Fatal("duplicate preflight must block before adding last_activity_at")
	}

	if _, err := db.ExecContext(ctx, `drop table tb_auth_sessions`); err != nil {
		t.Fatal("prepare legacy backfill case failed")
	}
	if err := createLegacyAuthSessionTable(ctx, db); err != nil {
		t.Fatal("create legacy backfill table failed")
	}
	legacyCreatedAt := preservedCreatedAt.Add(2 * time.Hour)
	insertLegacyAuthSessionRow(t, ctx, db, "c", userID, legacyCreatedAt)
	if err := runMySQLAuthSessionMigration(ctx, db); err != nil {
		t.Fatal("legacy backfill migration failed")
	}
	assertAuthSessionMigrationTable(t, ctx, db, true)
	assertAuthSessionActivity(t, ctx, db, "c", legacyCreatedAt)
}

func isSafeMySQLTestDSN(dsn string) bool {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || strings.TrimSpace(cfg.DBName) == "" {
		return false
	}
	identity := strings.ToLower(cfg.Addr + " " + cfg.DBName)
	for _, productionMarker := range []string{"polar-ops", "production", "prod", "ops"} {
		if strings.Contains(identity, productionMarker) {
			return false
		}
	}
	for _, testMarker := range []string{"test", "dev", "sandbox", "migration", "localhost", "127.0.0.1", "::1"} {
		if strings.Contains(identity, testMarker) {
			return true
		}
	}
	return false
}

func isSafeMySQLMigrationDSN(dsn, declaredDatabase string) bool {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return false
	}
	database := strings.TrimSpace(cfg.DBName)
	declaredDatabase = strings.TrimSpace(declaredDatabase)
	if database == "" || database != declaredDatabase || !strings.HasSuffix(strings.ToLower(database), "_migration_test") {
		return false
	}

	identity := strings.ToLower(database)
	for _, marker := range []string{
		"db_pm_erzhuang",
		"production",
		"prod",
		"ops",
	} {
		if strings.Contains(identity, marker) {
			return false
		}
	}
	return true
}

func TestMySQLAuthSessionMigrationSafetyGate(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		dsn      string
		declared string
		wantSafe bool
	}{
		{
			name:     "isolated database with exact declaration",
			dsn:      "test_user@tcp(polar-dev.rwlb.rds.aliyuncs.com:3306)/erzhuang_migration_test",
			declared: "erzhuang_migration_test",
			wantSafe: true,
		},
		{
			name:     "ordinary shared business database",
			dsn:      "test_user@tcp(polar-dev.rwlb.rds.aliyuncs.com:3306)/db_pm_erzhuang",
			declared: "db_pm_erzhuang",
			wantSafe: false,
		},
		{
			name:     "shared business name disguised with migration suffix",
			dsn:      "test_user@tcp(polar-dev.rwlb.rds.aliyuncs.com:3306)/db_pm_erzhuang_migration_test",
			declared: "db_pm_erzhuang_migration_test",
			wantSafe: false,
		},
		{
			name:     "declared database does not exactly match DSN",
			dsn:      "test_user@tcp(polar-dev.rwlb.rds.aliyuncs.com:3306)/erzhuang_migration_test",
			declared: "other_migration_test",
			wantSafe: false,
		},
		{
			name:     "database without isolation suffix",
			dsn:      "test_user@tcp(polar-dev.rwlb.rds.aliyuncs.com:3306)/erzhuang_test",
			declared: "erzhuang_test",
			wantSafe: false,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := isSafeMySQLMigrationDSN(testCase.dsn, testCase.declared); got != testCase.wantSafe {
				t.Fatalf("migration DSN safety = %t, want %t", got, testCase.wantSafe)
			}
		})
	}
}

func runMySQLAuthSessionMigration(ctx context.Context, db *sql.DB) error {
	script, err := readAuthSessionTestFileForMigration(filepath.Join("..", "..", "db", "mysql_auth_sessions.sql"))
	if err != nil {
		return err
	}
	statements, err := splitMySQLMigrationStatements(script)
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if strings.EqualFold(sqlStatementKeyword(statement), "select") {
			rows, err := db.QueryContext(ctx, statement)
			if err != nil {
				return err
			}
			for rows.Next() {
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func splitMySQLMigrationStatements(script string) ([]string, error) {
	var statements []string
	var current strings.Builder
	delimiter := ";"
	inSingle, inDouble, inBacktick, inLineComment, inBlockComment := false, false, false, false, false
	lineStart := true

	flush := func() {
		if statement := strings.TrimSpace(current.String()); statement != "" {
			statements = append(statements, statement)
		}
		current.Reset()
	}

	for index := 0; index < len(script); {
		if lineStart && !inSingle && !inDouble && !inBacktick && !inLineComment && !inBlockComment {
			lineEnd := strings.IndexByte(script[index:], '\n')
			if lineEnd < 0 {
				lineEnd = len(script) - index
			}
			line := strings.TrimSpace(script[index : index+lineEnd])
			if len(line) >= len("delimiter ") && strings.EqualFold(line[:len("delimiter ")], "delimiter ") {
				delimiter = strings.TrimSpace(line[len("delimiter "):])
				index += lineEnd
				if index < len(script) && script[index] == '\n' {
					index++
				}
				lineStart = true
				continue
			}
		}

		if !inSingle && !inDouble && !inBacktick && !inBlockComment {
			if inLineComment {
				current.WriteByte(script[index])
				if script[index] == '\n' {
					inLineComment = false
					lineStart = true
				}
				index++
				continue
			}
			if script[index] == '#' || (script[index] == '-' && index+2 < len(script) && script[index+1] == '-' && (script[index+2] == ' ' || script[index+2] == '\t')) {
				inLineComment = true
				current.WriteByte(script[index])
				index++
				continue
			}
		}
		if !inSingle && !inDouble && !inBacktick && !inLineComment && script[index] == '/' && index+1 < len(script) && script[index+1] == '*' {
			inBlockComment = true
			current.WriteString("/*")
			index += 2
			continue
		}
		if inBlockComment {
			if script[index] == '*' && index+1 < len(script) && script[index+1] == '/' {
				current.WriteString("*/")
				index += 2
				inBlockComment = false
				continue
			}
			current.WriteByte(script[index])
			index++
			continue
		}
		if !inDouble && !inBacktick && script[index] == '\'' {
			if inSingle && index+1 < len(script) && script[index+1] == '\'' {
				current.WriteString("''")
				index += 2
				continue
			}
			inSingle = !inSingle
		} else if !inSingle && !inBacktick && script[index] == '"' {
			inDouble = !inDouble
		} else if !inSingle && !inDouble && script[index] == '`' {
			inBacktick = !inBacktick
		}
		if !inSingle && !inDouble && !inBacktick && strings.HasPrefix(script[index:], delimiter) {
			flush()
			index += len(delimiter)
			lineStart = false
			continue
		}
		current.WriteByte(script[index])
		if script[index] == '\n' {
			lineStart = true
		} else if lineStart && script[index] != ' ' && script[index] != '\t' && script[index] != '\r' {
			lineStart = false
		}
		index++
	}
	if inSingle || inDouble || inBacktick || inBlockComment {
		return nil, fmt.Errorf("unterminated SQL quote or comment")
	}
	flush()
	return statements, nil
}

func sqlStatementKeyword(statement string) string {
	trimmed := strings.TrimSpace(statement)
	for {
		switch {
		case strings.HasPrefix(trimmed, "--"):
			if newline := strings.IndexByte(trimmed, '\n'); newline >= 0 {
				trimmed = strings.TrimSpace(trimmed[newline+1:])
				continue
			}
			return ""
		case strings.HasPrefix(trimmed, "#"):
			if newline := strings.IndexByte(trimmed, '\n'); newline >= 0 {
				trimmed = strings.TrimSpace(trimmed[newline+1:])
				continue
			}
			return ""
		case strings.HasPrefix(trimmed, "/*"):
			if end := strings.Index(trimmed[2:], "*/"); end >= 0 {
				trimmed = strings.TrimSpace(trimmed[end+4:])
				continue
			}
			return ""
		}
		break
	}
	if space := strings.IndexAny(trimmed, " \t\r\n"); space >= 0 {
		return trimmed[:space]
	}
	return trimmed
}

func readAuthSessionTestFileForMigration(name string) (string, error) {
	content, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func integrationAuthUserID(t *testing.T, ctx context.Context, db *sql.DB) int64 {
	t.Helper()
	var userID int64
	if err := db.QueryRowContext(ctx, `select id from tb_users order by id limit 1`).Scan(&userID); err != nil {
		t.Fatal("find integration test user failed")
	}
	return userID
}

func insertAuthSessionRow(t *testing.T, ctx context.Context, db *sql.DB, hashSeed string, userID int64, createdAt, activityAt time.Time) {
	t.Helper()
	hash := strings.Repeat(hashSeed, 64)
	if _, err := db.ExecContext(ctx, `
		insert into tb_auth_sessions (
			session_token_hash, user_id, sso_subject, ip_address, user_agent,
			created_at, last_activity_at, expires_at
		) values (?, ?, '', '', '', ?, ?, ?)
	`, hash, userID, createdAt, activityAt, activityAt.Add(defaultAuthIdleTimeout)); err != nil {
		t.Fatalf("insert integration auth session row failed: seed=%s", hashSeed)
	}
}

func insertLegacyAuthSessionRow(t *testing.T, ctx context.Context, db *sql.DB, hashSeed string, userID int64, createdAt time.Time) {
	t.Helper()
	hash := strings.Repeat(hashSeed, 64)
	if _, err := db.ExecContext(ctx, `
		insert into tb_auth_sessions (
			session_token_hash, user_id, sso_subject, ip_address, user_agent,
			created_at, expires_at
		) values (?, ?, '', '', '', ?, ?)
	`, hash, userID, createdAt, createdAt.Add(defaultAuthIdleTimeout)); err != nil {
		t.Fatalf("insert legacy auth session row failed: seed=%s", hashSeed)
	}
}

func createLegacyAuthSessionTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		create table tb_auth_sessions (
			id bigint not null auto_increment,
			session_token_hash char(64) not null,
			user_id bigint not null,
			sso_subject varchar(255) not null default '',
			ip_address varchar(64) not null default '',
			user_agent varchar(512) not null default '',
			created_at datetime(3) not null,
			expires_at datetime(3) not null,
			revoked_at datetime(3) null,
			revoked_reason varchar(255) not null default '',
			primary key (id),
			key idx_legacy_auth_sessions_user (user_id, created_at)
		) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci
	`)
	return err
}

func assertAuthSessionMigrationTable(t *testing.T, ctx context.Context, db *sql.DB, wantActivityColumn bool) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `
		select count(*) from information_schema.columns
		where table_schema = database()
		  and table_name = 'tb_auth_sessions'
		  and column_name = 'last_activity_at'
		  and is_nullable = 'NO'
	`).Scan(&count); err != nil {
		t.Fatal("check migrated last_activity_at failed")
	}
	if (count == 1) != wantActivityColumn {
		t.Fatalf("last_activity_at column present=%t, want %t", count == 1, wantActivityColumn)
	}
	assertAuthSessionIndex(t, ctx, db, "uq_tb_auth_sessions_token_hash", "non_unique = 0", "seq_in_index = 1", "column_name = 'session_token_hash'")
	assertAuthSessionIndex(t, ctx, db, "idx_tb_auth_sessions_user_activity", "non_unique = 1", "seq_in_index = 1", "column_name = 'user_id'")
	assertAuthSessionIndex(t, ctx, db, "idx_tb_auth_sessions_user_activity", "non_unique = 1", "seq_in_index = 2", "column_name = 'last_activity_at'")
	assertAuthSessionIndex(t, ctx, db, "idx_tb_auth_sessions_expires_at", "non_unique = 1", "seq_in_index = 1", "column_name = 'expires_at'")
}

func assertAuthSessionIndex(t *testing.T, ctx context.Context, db *sql.DB, name string, predicates ...string) {
	t.Helper()
	where := []string{"table_schema = database()", "table_name = 'tb_auth_sessions'", "index_name = ?", "index_type = 'BTREE'", "(sub_part = 0 or sub_part is null)"}
	where = append(where, predicates...)
	var count int
	if err := db.QueryRowContext(ctx, `select count(*) from information_schema.statistics where `+strings.Join(where, " and ")+
		"", name).Scan(&count); err != nil {
		t.Fatalf("check migrated index failed: name=%s", name)
	}
	if count != 1 {
		t.Fatalf("migrated index definition count = %d, want 1: name=%s", count, name)
	}
}

func assertAuthSessionActivity(t *testing.T, ctx context.Context, db *sql.DB, hashSeed string, want time.Time) {
	t.Helper()
	var got string
	if err := db.QueryRowContext(ctx, `
		select date_format(last_activity_at, '%Y-%m-%d %H:%i:%s.%f')
		from tb_auth_sessions where session_token_hash = ?
	`, strings.Repeat(hashSeed, 64)).Scan(&got); err != nil {
		t.Fatalf("read migrated last_activity_at failed: seed=%s", hashSeed)
	}
	if got != want.Format("2006-01-02 15:04:05.000000") {
		t.Fatalf("last_activity_at = %s, want %s", got, want.Format("2006-01-02 15:04:05.000000"))
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
		"if v_index_entries = 0 then",
		"call erzhuang_migrate_tb_auth_sessions_tmp()",
		"drop procedure erzhuang_migrate_tb_auth_sessions_tmp",
		"v_missing_columns",
		"v_null_created_at",
		"v_nullable_token_hash",
		"v_null_token_hash",
		"v_null_expires_at",
		"NULL created_at",
		"NULL session_token_hash",
		"NULL expires_at",
		"duplicate session_token_hash",
		"tb_auth_sessions PRIMARY index must be a single BTREE on id without a prefix",
		"idx_tb_auth_sessions_user has the wrong uniqueness, BTREE type, prefix, or column order",
		"index_type = 'BTREE'",
		"sub_part = 0 or sub_part is null",
		"uq_tb_auth_sessions_token_hash has the wrong uniqueness, BTREE type, prefix, or column order",
		"idx_tb_auth_sessions_user_activity has the wrong uniqueness, BTREE type, prefix, or column order",
		"idx_tb_auth_sessions_expires_at has the wrong uniqueness, BTREE type, prefix, or column order",
		"application startup",
	} {
		if !strings.Contains(migration, want) {
			t.Fatalf("auth-session migration missing %q", want)
		}
	}
	firstAlter := strings.Index(migration, "alter table tb_auth_sessions")
	if firstAlter < 0 {
		t.Fatal("migration must contain the existing-table patch branch")
	}
	for _, preflightMarker := range []string{
		"tb_auth_sessions is missing required base columns",
		"tb_auth_sessions.session_token_hash must be NOT NULL",
		"tb_auth_sessions contains rows with NULL created_at",
		"tb_auth_sessions contains rows with NULL session_token_hash",
		"tb_auth_sessions contains rows with NULL expires_at",
		"duplicate session_token_hash values must be repaired before migration",
		"tb_auth_sessions PRIMARY index must be a single BTREE on id without a prefix",
		"idx_tb_auth_sessions_user has the wrong uniqueness, BTREE type, prefix, or column order",
		"and index_type = 'BTREE'",
		"and (sub_part = 0 or sub_part is null)",
	} {
		if index := strings.Index(migration, preflightMarker); index < 0 || index > firstAlter {
			t.Fatalf("migration preflight marker %q must appear before first ALTER", preflightMarker)
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

func summarizeSQLCalls(calls []recordingSQLCall) string {
	summaries := make([]string, 0, len(calls))
	for index, call := range calls {
		summaries = append(summaries, fmt.Sprintf("%d:{keyword=%s,args=%d}", index, sqlStatementKeyword(call.query), len(call.args)))
	}
	return strings.Join(summaries, " ")
}
