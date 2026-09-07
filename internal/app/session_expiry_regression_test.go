package app

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionStatusRouteDoesNotExtendActivity(t *testing.T) {
	key := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &key.PublicKey))
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{ID: 42, Email: "idle@example.com", Role: RoleAdmin, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	h := NewHandlerWithStore(store)
	now := time.Now()
	r := idleSessionRequest(t, key, "/_/auth/callback", now)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == authSessionCookieName {
			r.AddCookie(cookie)
		}
	}
	r.URL.Path = "/api/auth/session-status"
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status endpoint: status=%d", w.Code)
	}
	var first map[string]int64
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first["idle_remaining_ms"] <= 0 || first["absolute_remaining_ms"] <= 0 {
		t.Fatal("missing server deadlines")
	}
	time.Sleep(15 * time.Millisecond)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var second map[string]int64
	if err := json.Unmarshal(w.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if second["idle_remaining_ms"] >= first["idle_remaining_ms"] {
		t.Fatal("status poll renewed idle deadline")
	}
}

func TestSessionMissingCookieCannotMintSessionOnAPI(t *testing.T) {
	now := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	sessions := newMemoryAuthSessionStore()
	h, key := newIdleSessionTestHandler(t, sessions, now)
	w := httptest.NewRecorder()
	h.authGate(http.HandlerFunc(h.authMeHandler)).ServeHTTP(w, idleSessionRequest(t, key, "/api/auth/me", now))
	if w.Code != http.StatusUnauthorized || len(sessions.sessions) != 0 {
		t.Fatalf("missing local session: status=%d created=%d", w.Code, len(sessions.sessions))
	}
}

func TestSessionCallbackCreatesOnceAndCannotReplaySSO(t *testing.T) {
	now := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	sessions := newMemoryAuthSessionStore()
	h, key := newIdleSessionTestHandler(t, sessions, now)
	r := idleSessionRequest(t, key, "/_/auth/callback", now)
	w := httptest.NewRecorder()
	h.authCallbackHandler(w, r)
	if w.Code != http.StatusFound || !hasCookie(w.Result().Cookies(), authSessionCookieName) || len(sessions.sessions) != 1 {
		t.Fatalf("callback must create one session: status=%d count=%d", w.Code, len(sessions.sessions))
	}
	// Losing the browser cookie must not grant a fresh session for the same SSO.
	w = httptest.NewRecorder()
	h.authCallbackHandler(w, r)
	if hasCookie(w.Result().Cookies(), authSessionCookieName) || len(sessions.sessions) != 1 {
		t.Fatal("replayed SSO created a replacement local session")
	}
}

func TestSessionCallbackCannotRenewExpiredCookie(t *testing.T) {
	now := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	sessions := newMemoryAuthSessionStore()
	h, key := newIdleSessionTestHandler(t, sessions, now)
	token, err := sessions.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 42, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	h.now = func() time.Time { return now.Add(30 * time.Minute) }
	r := idleSessionRequest(t, key, "/_/auth/callback", h.now())
	r.AddCookie(&http.Cookie{Name: authSessionCookieName, Value: token})
	w := httptest.NewRecorder()
	h.authCallbackHandler(w, r)
	if hasCookie(w.Result().Cookies(), authSessionCookieName) || w.Header().Get("Location") == "/" {
		t.Fatal("expired callback must not silently return to the application")
	}
}

func TestMemorySessionAbsoluteDeadlineDespiteContinuousActivity(t *testing.T) {
	now := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	s := newMemoryAuthSessionStore()
	token, err := s.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 42, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	for minute := 20; minute < 480; minute += 20 {
		if ok, err := s.TouchAuthSession(context.Background(), token, 42, now.Add(time.Duration(minute)*time.Minute), defaultAuthIdleTimeout); !ok || err != nil {
			t.Fatalf("active session rejected at minute %d", minute)
		}
	}
	if ok, _ := s.TouchAuthSession(context.Background(), token, 42, now.Add(8*time.Hour), defaultAuthIdleTimeout); ok {
		t.Fatal("continuous activity extended session beyond 8 hours")
	}
}

func TestMemorySessionDoesNotTrustInflatedExpiry(t *testing.T) {
	now := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	s := newMemoryAuthSessionStore()
	token, err := s.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 42, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	hash := hashAuthSessionToken(token)
	id := hex.EncodeToString(hash[:])
	row := s.sessions[id]
	row.expiresAt = now.Add(24 * time.Hour)
	s.sessions[id] = row
	if ok, _ := s.TouchAuthSession(context.Background(), token, 42, now.Add(30*time.Minute), defaultAuthIdleTimeout); ok {
		t.Fatal("inflated expires_at bypassed last_activity_at idle limit")
	}
}
