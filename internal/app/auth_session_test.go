package app

import (
	"context"
	"encoding/base64"
	"encoding/hex"
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
	session.lastActivity = now
	session.expiresAt = now.Add(idleTimeout)
	s.sessions[hex.EncodeToString(hash[:])] = session
	return true, nil
}

func (s *fakeAuthSessionStore) RevokeAuthSession(_ context.Context, token string, reason string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash := hashAuthSessionToken(token)
	key := hex.EncodeToString(hash[:])
	session, ok := s.sessions[key]
	if ok && session.revokedAt.IsZero() {
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
		t.Fatalf("tokens must be non-empty and unique: %q %q", first, second)
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

	ok, err := store.TouchAuthSession(context.Background(), token, 7, createdAt.Add(30*time.Minute+time.Second), defaultAuthIdleTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("touch at 30:01 should fail")
	}
}

func TestAuthSessionRevokedCannotBeTouched(t *testing.T) {
	store := newFakeAuthSessionStore()
	createdAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	token, err := store.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 7, Now: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeAuthSession(context.Background(), token, "manual_logout", createdAt.Add(time.Minute)); err != nil {
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
			t.Fatalf("fake storage contains raw token: %q", key)
		}
	}
	if got := len(store.sessions); got != 1 {
		t.Fatalf("stored sessions = %d, want 1", got)
	}
}
