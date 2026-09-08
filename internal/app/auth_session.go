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
	authSessionCookieName      = "erzhuang_session"
	defaultAuthIdleTimeout     = 30 * time.Minute
	defaultAuthAbsoluteTimeout = 8 * time.Hour
)

type authSessionStore interface {
	CreateAuthSession(context.Context, AuthSessionCreate) (string, error)
	TouchAuthSession(context.Context, string, int64, time.Time, time.Duration) (bool, error)
	RevokeAuthSession(context.Context, string, int64, string, time.Time) error
}

type AuthSessionStatus struct {
	IdleRemainingMS     int64 `json:"idle_remaining_ms"`
	AbsoluteRemainingMS int64 `json:"absolute_remaining_ms"`
}

type authSessionStatusStore interface {
	GetAuthSessionStatus(context.Context, string, int64, time.Time) (AuthSessionStatus, error)
}

func (s *memoryAuthSessionStore) GetAuthSessionStatus(_ context.Context, token string, userID int64, now time.Time) (AuthSessionStatus, error) {
	hash := hashAuthSessionToken(token)
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.sessions[hex.EncodeToString(hash[:])]
	if !ok || row.userID != userID || !row.revokedAt.IsZero() {
		return AuthSessionStatus{}, errSessionIdleTimeout
	}
	status := AuthSessionStatus{IdleRemainingMS: row.expiresAt.Sub(now).Milliseconds(), AbsoluteRemainingMS: row.createdAt.Add(defaultAuthAbsoluteTimeout).Sub(now).Milliseconds()}
	if idle := row.lastActivity.Add(defaultAuthIdleTimeout).Sub(now).Milliseconds(); idle < status.IdleRemainingMS {
		status.IdleRemainingMS = idle
	}
	if status.AbsoluteRemainingMS <= 0 {
		return AuthSessionStatus{}, errSessionAbsoluteTimeout
	}
	if status.IdleRemainingMS <= 0 {
		return AuthSessionStatus{}, errSessionIdleTimeout
	}
	return status, nil
}

type AuthSessionCreate struct {
	UserID     int64
	SSOSubject string
	IPAddress  string
	UserAgent  string
	Now        time.Time
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
	ssoSubject   string
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
	defer s.mu.Unlock()
	if isSSOCredentialFingerprint(input.SSOSubject) {
		for _, session := range s.sessions {
			if session.userID == input.UserID && session.ssoSubject == input.SSOSubject {
				return "", errSessionReauthenticationRequired
			}
		}
	}
	s.sessions[hex.EncodeToString(hash[:])] = memoryAuthSession{
		userID: input.UserID, createdAt: now,
		ssoSubject:   input.SSOSubject,
		lastActivity: now, expiresAt: now.Add(defaultAuthIdleTimeout),
	}
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
	if !ok || session.userID != userID || !session.revokedAt.IsZero() {
		return false, nil
	}
	if !now.Before(session.createdAt.Add(defaultAuthAbsoluteTimeout)) {
		return false, errSessionAbsoluteTimeout
	}
	if !now.Before(session.expiresAt) || !now.Before(session.lastActivity.Add(defaultAuthIdleTimeout)) {
		return false, nil
	}
	if now.After(session.lastActivity) {
		session.lastActivity = now
		session.expiresAt = now.Add(defaultAuthIdleTimeout)
		if deadline := session.createdAt.Add(defaultAuthAbsoluteTimeout); session.expiresAt.After(deadline) {
			session.expiresAt = deadline
		}
	}
	s.sessions[key] = session
	return true, nil
}

func ssoCredentialFingerprint(token string) string {
	hash := sha256.Sum256([]byte(token))
	return "jwt-sha256:v1:" + hex.EncodeToString(hash[:])
}

func isSSOCredentialFingerprint(value string) bool {
	const prefix = "jwt-sha256:v1:"
	if len(value) != len(prefix)+sha256.Size*2 || value[:len(prefix)] != prefix {
		return false
	}
	_, err := hex.DecodeString(value[len(prefix):])
	return err == nil
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
