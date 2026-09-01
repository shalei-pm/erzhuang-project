package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"time"
)

const (
	authSessionCookieName  = "erzhuang_session"
	defaultAuthIdleTimeout = 30 * time.Minute
)

type authSessionStore interface {
	CreateAuthSession(context.Context, AuthSessionCreate) (string, error)
	TouchAuthSession(context.Context, string, int64, time.Time, time.Duration) (bool, error)
	RevokeAuthSession(context.Context, string, int64, string, time.Time) error
}

type AuthSessionCreate struct {
	UserID    int64
	SSOSubject string
	IPAddress string
	UserAgent string
	Now       time.Time
}

func newAuthSessionToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashAuthSessionToken(token string) [sha256.Size]byte {
	return sha256.Sum256([]byte(token))
}

// memoryAuthSessionStore keeps local development and in-memory tests aligned
// with the persistent session contract without weakening the SSO gate.
type memoryAuthSessionStore struct {
	mu       sync.Mutex
	sessions map[string]memoryAuthSession
}

type memoryAuthSession struct {
	userID       int64
	lastActivity time.Time
	expiresAt    time.Time
	revokedAt    time.Time
	createdAt    time.Time
	revokeReason string
}

func newMemoryAuthSessionStore() *memoryAuthSessionStore {
	return &memoryAuthSessionStore{sessions: make(map[string]memoryAuthSession)}
}

func (s *memoryAuthSessionStore) CreateAuthSession(_ context.Context, input AuthSessionCreate) (string, error) {
	token, err := newAuthSessionToken()
	if err != nil {
		return "", err
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	hash := hashAuthSessionToken(token)
	s.mu.Lock()
	s.sessions[hex.EncodeToString(hash[:])] = memoryAuthSession{
		userID: input.UserID, createdAt: now,
		lastActivity: now, expiresAt: now.Add(defaultAuthIdleTimeout),
	}
	s.mu.Unlock()
	return token, nil
}

func (s *memoryAuthSessionStore) TouchAuthSession(_ context.Context, token string, userID int64, now time.Time, _ time.Duration) (bool, error) {
	if now.IsZero() {
		now = time.Now()
	}
	hash := hashAuthSessionToken(token)
	key := hex.EncodeToString(hash[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[key]
	if !ok || session.userID != userID || !session.revokedAt.IsZero() || !now.Before(session.expiresAt) {
		return false, nil
	}
	if now.After(session.lastActivity) {
		session.lastActivity = now
		session.expiresAt = now.Add(defaultAuthIdleTimeout)
	}
	s.sessions[key] = session
	return true, nil
}

func (s *memoryAuthSessionStore) RevokeAuthSession(_ context.Context, token string, userID int64, reason string, now time.Time) error {
	if now.IsZero() {
		now = time.Now()
	}
	hash := hashAuthSessionToken(token)
	key := hex.EncodeToString(hash[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[key]
	if ok && session.userID == userID && session.revokedAt.IsZero() {
		session.revokedAt = now
		session.revokeReason = reason
		s.sessions[key] = session
	}
	return nil
}
