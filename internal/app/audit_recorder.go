package app

import (
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

type auditActor struct {
	userID      *int64
	displayName string
	email       string
}

func int64Pointer(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func (h *Handler) recordAuthLogout(r *http.Request) {
	if h.auditRecorder == nil {
		return
	}
	result := "denied"
	actor := auditActor{}
	cookie, err := r.Cookie(h.auth.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		h.recordAuthAuditEvent(r, "auth.logout", result, actor)
		return
	}
	claims, err := h.auth.validateAPISIXSSOToken(cookie.Value, time.Now())
	if err != nil {
		h.recordAuthAuditEvent(r, "auth.logout", "failed", actor)
		return
	}

	claimsUser := claims.authUser()
	actor = auditActorFromClaims(claimsUser)
	result = "success"
	record, err := h.store.GetAuthUserByEmail(r.Context(), claimsUser.Email)
	switch {
	case err == nil:
		actor = actorFromAuthUser(record, claimsUser)
		if !record.Enabled {
			result = "denied"
		}
	case errors.Is(err, errAuthUserNotFound):
		result = "denied"
	default:
		result = "failed"
	}

	h.recordAuthAuditEvent(r, "auth.logout", result, actor)
}

func (h *Handler) recordAuthLogin(r *http.Request) {
	if h.auditRecorder == nil {
		return
	}
	result := "denied"
	actor := auditActor{}
	cookie, err := r.Cookie(h.auth.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		h.recordAuthAuditEvent(r, "auth.login", result, actor)
		return
	}
	claims, err := h.auth.validateAPISIXSSOToken(cookie.Value, time.Now())
	if err != nil {
		h.recordAuthAuditEvent(r, "auth.login", "failed", actor)
		return
	}

	claimsUser := claims.authUser()
	record, err := h.store.GetAuthUserByEmail(r.Context(), claimsUser.Email)
	switch {
	case err == nil && record.Enabled:
		actor = actorFromAuthUser(record, claimsUser)
		result = "success"
	case errors.Is(err, errAuthUserNotFound), err == nil && !record.Enabled:
		result = "denied"
	default:
		result = "failed"
	}
	h.recordAuthAuditEvent(r, "auth.login", result, actor)
}

func (h *Handler) recordAuthAuditEvent(r *http.Request, action string, result string, actor auditActor) {
	event := auditlog.AuditEvent{
		UserID:           actor.userID,
		ActorDisplayName: actor.displayName,
		UserEmail:        actor.email,
		Action:           action,
		IPAddress:        requestIPAddress(r),
		UserAgent:        strings.TrimSpace(r.UserAgent()),
		RequestID:        strings.TrimSpace(r.Header.Get("X-Request-ID")),
		Result:           result,
	}
	if err := h.auditRecorder.RecordAudit(r.Context(), event); err != nil {
		// Do not include recorder errors: a lower layer must not echo credentials.
		log.Printf("auth: audit record failed action=%s result=%s", action, result)
	}
}

func auditActorFromClaims(user AuthUserResponse) auditActor {
	return auditActor{
		displayName: firstNonEmpty(user.DisplayName, user.Username, user.Email),
		email:       strings.TrimSpace(user.Email),
	}
}

func actorFromAuthUser(record AuthUserRecord, claimsUser AuthUserResponse) auditActor {
	actor := auditActorFromClaims(claimsUser)
	if record.ID != 0 {
		userID := record.ID
		actor.userID = &userID
	}
	actor.displayName = firstNonEmpty(record.DisplayName, actor.displayName)
	actor.email = firstNonEmpty(record.Email, actor.email)
	return actor
}

func requestIPAddress(r *http.Request) string {
	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return strings.TrimSpace(remoteAddr)
}
