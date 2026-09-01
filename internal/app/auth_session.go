package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

const (
	authSessionCookieName  = "erzhuang_session"
	defaultAuthIdleTimeout = 30 * time.Minute
)

type authSessionStore interface {
	CreateAuthSession(context.Context, AuthSessionCreate) (string, error)
	TouchAuthSession(context.Context, string, int64, time.Time, time.Duration) (bool, error)
	RevokeAuthSession(context.Context, string, string, time.Time) error
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
