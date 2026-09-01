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
	if len(queries) != 3 {
		t.Fatalf("executed queries = %d, want 3", len(queries))
	}
	for _, query := range queries {
		if strings.Contains(query, token) {
			t.Fatalf("raw session token was interpolated into SQL: raw_token_present=true")
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
		"update tb_auth_sessions",
		"last_activity_at = greatest(last_activity_at, ?)",
		"expires_at = greatest(expires_at, ?)",
		"session_token_hash = ?",
		"user_id = ?",
		"revoked_at is null",
		"expires_at > ?",
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
		"create table if not exists tb_auth_sessions",
		"add column last_activity_at datetime(3) null",
		"set last_activity_at = created_at",
		"modify column last_activity_at datetime(3) not null",
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
