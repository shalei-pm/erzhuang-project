package app

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const (
	defaultAPISIXSSOCookieName = "sy_sso_token"
	defaultAPISIXSSOPublicKey  = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAuobWx9Ayq3Z/Kjqmoju7
IuSBf7N3k2GJL1pX4Tb6RmbKM9G8vD4mWqWIrorgQKOZc4LlIftbsQ4opp0cLq2R
8tNNOugng06v02Pvfk/WC+bBdvFOpE3ZPGUQezNO/XL/H7W0ouv0vb5K8+QVSkFw
dR81Nu1pK9uSlSZMLmI2Wz/8vJwI4o9fiFma9ahYu1utK7CcfZQzdkpVp9TOQF4Q
j0KsFcJe35piVw8u+uHl5VPBIG8W3GnKuUMNo88QVqkODbiONGFLjDSLuRkLL6C8
QYlhtF41cEGNtrVwcn9ltKsBq4RGJcUyNvI+gBh6P9L3dy+yqgZyMvNpptg06sRO
swIDAQAB
-----END PUBLIC KEY-----`
)

type AuthConfig struct {
	Enabled      bool
	CookieName   string
	JWTPublicKey string
	ExpectedSub  string
	ClockSkew    time.Duration
	RequireEmail bool
}

type AuthResponse struct {
	Enabled       bool               `json:"enabled"`
	Authenticated bool               `json:"authenticated"`
	Code          string             `json:"code,omitempty"`
	Message       string             `json:"message,omitempty"`
	LoginURL      string             `json:"login_url,omitempty"`
	User          *AuthUserResponse  `json:"user,omitempty"`
	Permissions   []string           `json:"permissions,omitempty"`
	Session       *AuthSessionStatus `json:"session,omitempty"`
}

type AuthUserResponse struct {
	Email        string `json:"email"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	OpenID       string `json:"open_id,omitempty"`
	FeishuUserID string `json:"feishu_user_id,omitempty"`
	Phone        string `json:"phone,omitempty"`
	LoginWay     string `json:"login_way,omitempty"`
	Subject      string `json:"subject,omitempty"`
	Role         string `json:"role"`
}

type apisixSSOTokenHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type apisixSSOTokenClaims struct {
	Data apisixSSOUserData `json:"data"`
	Exp  int64             `json:"exp"`
	Sub  string            `json:"sub"`
}

type apisixSSOUserData struct {
	Display  string `json:"display"`
	Mail     string `json:"mail"`
	OpenID   string `json:"open_id"`
	UserID   string `json:"user_id"`
	Phone    string `json:"phone"`
	Username string `json:"username"`
	LoginWay string `json:"login_way"`
}

func AuthConfigFromEnv() AuthConfig {
	cookieName := strings.TrimSpace(os.Getenv("SSO_COOKIE_NAME"))
	if cookieName == "" {
		cookieName = defaultAPISIXSSOCookieName
	}
	publicKey := strings.TrimSpace(os.Getenv("SSO_JWT_PUBLIC_KEY"))
	if publicKey == "" {
		publicKey = defaultAPISIXSSOPublicKey
	}
	return AuthConfig{
		Enabled:      isTruthy(os.Getenv("SSO_ENABLED")),
		CookieName:   cookieName,
		JWTPublicKey: publicKey,
		ExpectedSub:  strings.TrimSpace(os.Getenv("SSO_EXPECTED_SUB")),
		ClockSkew:    30 * time.Second,
		RequireEmail: true,
	}
}

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (h *Handler) authMeHandler(w http.ResponseWriter, r *http.Request) {
	identity, err := h.currentAuthIdentity(r)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	record := identity.record
	user := identity.user
	if user.Email == "" {
		h.writeLocalAdminAuth(w)
		return
	}
	var session *AuthSessionStatus
	if h.authRequired(r) {
		if _, supported := h.authSessionStore.(authSessionStatusStore); supported {
			status, err := h.readAuthSessionStatus(r, record.ID)
			if err != nil {
				h.writeAuthError(w, r, err)
				return
			}
			session = &status
		}
	}
	record, err = h.store.UpdateAuthUserProfile(r.Context(), AuthUserPatch{
		Email:        user.Email,
		Username:     user.Username,
		DisplayName:  user.DisplayName,
		FeishuUserID: user.FeishuUserID,
		Phone:        user.Phone,
	})
	if err != nil {
		log.Printf("auth: profile sync failed email=%s error=%q", safeAuthLogValue(user.Email), err.Error())
	} else {
		user = record.applyToResponse(user)
	}
	writeJSON(w, http.StatusOK, AuthResponse{
		Enabled:       true,
		Authenticated: true,
		User:          &user,
		Permissions:   record.permissions(),
		Session:       session,
	})
}

func (h *Handler) readAuthSessionStatus(r *http.Request, userID int64) (AuthSessionStatus, error) {
	store, ok := h.authSessionStore.(authSessionStatusStore)
	if !ok {
		return AuthSessionStatus{}, errAuthSessionUnavailable
	}
	cookie, err := r.Cookie(authSessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return AuthSessionStatus{}, errSessionLoginRequired
	}
	status, err := store.GetAuthSessionStatus(r.Context(), cookie.Value, userID, h.authNow())
	if err != nil && !errors.Is(err, errSessionIdleTimeout) && !errors.Is(err, errSessionAbsoluteTimeout) {
		return AuthSessionStatus{}, errAuthSessionUnavailable
	}
	return status, err
}

// This endpoint authenticates independently: passing through authGate would
// turn the expiry check itself into activity and keep idle browsers signed in.
func (h *Handler) authSessionStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	identity, err := h.authenticateSSO(r)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	if !h.authRequired(r) {
		writeJSON(w, http.StatusOK, map[string]bool{"enabled": false})
		return
	}
	status, err := h.readAuthSessionStatus(r, identity.record.ID)
	if err != nil {
		if errors.Is(err, errSessionIdleTimeout) || errors.Is(err, errSessionAbsoluteTimeout) {
			reason := "idle_timeout"
			if errors.Is(err, errSessionAbsoluteTimeout) {
				reason = "absolute_timeout"
			}
			if cookie, cookieErr := r.Cookie(authSessionCookieName); cookieErr == nil {
				_ = h.authSessionStore.RevokeAuthSession(r.Context(), cookie.Value, identity.record.ID, reason, h.authNow())
			}
			h.recordAuthSessionTimeout(r, identity.record, identity.user, reason)
		}
		h.writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func safeAuthLogValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	at := strings.Index(value, "@")
	if at <= 1 {
		return "***"
	}
	return value[:1] + "***" + value[at:]
}

func (h *Handler) writeLocalAdminAuth(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, AuthResponse{
		Enabled:       false,
		Authenticated: true,
		User: &AuthUserResponse{
			Email:       "local-admin@example.com",
			Username:    "local-admin",
			DisplayName: "本地管理员",
			Role:        "admin",
		},
		Permissions: []string{"admin"},
	})
}

func (h *Handler) writeUnauthorizedAuth(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, AuthResponse{
		Enabled:       true,
		Authenticated: false,
		LoginURL:      normalizeBasePath(os.Getenv("APP_BASE_PATH")) + "/_/auth/callback",
	})
}

func (h *Handler) writeSessionIdleTimeoutAuth(w http.ResponseWriter, r *http.Request) {
	h.writeSessionExpiredAuth(w, r, "session_idle_timeout", "登录已因长时间未操作失效，请重新扫码登录")
}

func (h *Handler) writeSessionExpiredAuth(w http.ResponseWriter, r *http.Request, code, message string) {
	h.clearAuthCookie(w, r)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusUnauthorized, AuthResponse{
		Enabled:       true,
		Authenticated: false,
		Code:          code,
		Message:       message,
		LoginURL:      normalizeBasePath(os.Getenv("APP_BASE_PATH")) + "/_/auth/callback",
	})
}

func (h *Handler) writeAuthSessionUnavailable(w http.ResponseWriter, r *http.Request) {
	h.clearAuthCookie(w, r)
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{
		"code":  "auth_session_unavailable",
		"error": "authentication session unavailable",
	})
}

func (h *Handler) writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errSessionIdleTimeout):
		h.writeSessionIdleTimeoutAuth(w, r)
	case errors.Is(err, errSessionAbsoluteTimeout):
		h.writeSessionExpiredAuth(w, r, "session_absolute_timeout", "登录已超过 8 小时，请重新扫码登录")
	case errors.Is(err, errSessionReauthenticationRequired):
		h.writeSessionExpiredAuth(w, r, "session_reauthentication_required", "原登录凭据已使用，请重新扫码登录")
	case errors.Is(err, errSessionLoginRequired):
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusUnauthorized, AuthResponse{
			Enabled: true, Code: "session_login_required",
			LoginURL: normalizeBasePath(os.Getenv("APP_BASE_PATH")) + "/_/auth/callback",
		})
	case errors.Is(err, errAuthSessionUnavailable):
		h.writeAuthSessionUnavailable(w, r)
	case errors.Is(err, errUnauthorizedAuth):
		h.writeUnauthorizedAuth(w)
	case errors.Is(err, errForbiddenAuth):
		h.writeForbiddenAuth(w)
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load auth user failed"})
	}
}

func (h *Handler) writeForbiddenAuth(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, AuthResponse{
		Enabled:       true,
		Authenticated: false,
		LoginURL:      normalizeBasePath(os.Getenv("APP_BASE_PATH")) + "/_/auth/callback",
	})
}

func (h *Handler) authCallbackHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.authRequired(r) {
		identity, err := h.authenticateSSO(r)
		if err != nil {
			h.writeAuthError(w, r, err)
			return
		}
		if h.authSessionStore == nil {
			h.writeAuthSessionUnavailable(w, r)
			return
		}
		if cookie, err := r.Cookie(authSessionCookieName); err == nil && strings.TrimSpace(cookie.Value) != "" {
			if _, err := h.authenticateRequest(r); err != nil {
				h.rejectAuthCallback(w, r, err)
				return
			}
		} else {
			cookie, _ := r.Cookie(h.auth.CookieName) // Validated by authenticateSSO.
			token, err := h.authSessionStore.CreateAuthSession(r.Context(), AuthSessionCreate{
				UserID: identity.record.ID, SSOSubject: ssoCredentialFingerprint(cookie.Value),
				IPAddress: requestIPAddress(r), UserAgent: r.UserAgent(), Now: h.authNow(),
			})
			if err != nil {
				if !errors.Is(err, errSessionReauthenticationRequired) {
					err = errAuthSessionUnavailable
				}
				h.rejectAuthCallback(w, r, err)
				return
			}
			if token == "" {
				h.writeAuthSessionUnavailable(w, r)
				return
			}
			h.setAuthSessionCookie(w, r, token)
		}
		http.SetCookie(w, expiredAuthCookie("erzhuang_reauth_attempt", ""))
	}
	h.recordAuthLogin(r)
	http.Redirect(w, r, safeAuthReturnPath(r.URL.Query().Get("return_to")), http.StatusFound)
}

// Only business pages within this application may be login return targets.
func safeAuthReturnPath(value string) string {
	home := normalizeBasePath(os.Getenv("APP_BASE_PATH")) + "/"
	u, err := url.Parse(value)
	if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(value, home) {
		return home
	}
	clean := path.Clean(u.Path)
	if strings.ContainsAny(u.Path, "\\%\r\n\t ") || (clean != strings.TrimSuffix(home, "/") && !strings.HasPrefix(clean, home)) {
		return home
	}
	relative := strings.TrimPrefix(clean, home)
	if relative == "api" || strings.HasPrefix(relative, "api/") || relative == "_" || strings.HasPrefix(relative, "_/") || relative == "logout" || strings.HasPrefix(relative, "logout/") {
		return home
	}
	query := u.Query()
	query.Del("return_to")
	u.RawQuery = query.Encode()
	return u.String()
}

func (h *Handler) rejectAuthCallback(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errAuthSessionUnavailable) {
		h.writeAuthSessionUnavailable(w, r)
		return
	}
	gateway := logoutGatewayHost(r.Host)
	_, alreadyAttempted := r.Cookie("erzhuang_reauth_attempt")
	if gateway == "" || alreadyAttempted == nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.clearAuthCookie(w, r)
	http.SetCookie(w, &http.Cookie{
		Name: "erzhuang_reauth_attempt", Value: "1", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: 300,
		Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
	})
	// Return to the callback after the gateway has cleared its own SSO session.
	host := strings.Split(strings.ToLower(r.Host), ":")[0]
	origin := companyApplicationOrigin(host)
	if origin == "" {
		h.writeAuthError(w, r, err)
		return
	}
	callback := origin + normalizeBasePath(os.Getenv("APP_BASE_PATH")) + "/_/auth/callback"
	if target := safeAuthReturnPath(r.URL.Query().Get("return_to")); target != normalizeBasePath(os.Getenv("APP_BASE_PATH"))+"/" {
		callback += "?" + url.Values{"return_to": {target}}.Encode()
	}
	params := url.Values{"from_host": {host}, "from_uri": {callback}}
	http.Redirect(w, r, "https://"+gateway+"/api/g/sso/logouttogether?"+params.Encode(), http.StatusFound)
}

func companyApplicationOrigin(host string) string {
	hostname := strings.ToLower(strings.TrimSpace(host))
	if colon := strings.Index(hostname, ":"); colon >= 0 {
		hostname = hostname[:colon]
	}
	switch hostname {
	case "lite.sy.soyoung.com":
		return "https://lite.sy.soyoung.com"
	case "lite.soyoung.com":
		return "http://lite.soyoung.com"
	default:
		return ""
	}
}

func (h *Handler) authLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && isCrossSiteLogoutRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "不允许跨站触发退出登录"})
		return
	}
	h.recordAuthLogout(r)
	h.revokeLocalAuthSession(r)
	h.clearAuthCookie(w, r)
	if r.Method == http.MethodGet {
		redirectTo := safeLogoutRedirect(r.Host, r.URL.Query().Get("redirect"))
		if redirectTo == "" {
			redirectTo = normalizeBasePath(os.Getenv("APP_BASE_PATH")) + "/"
		}
		http.Redirect(w, r, redirectTo, http.StatusFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func isCrossSiteLogoutRequest(r *http.Request) bool {
	// Modern browsers attach Sec-Fetch-Site to navigation requests. An empty
	// value remains allowed for older corporate browsers and server-side probes.
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site")
}

func (h *Handler) revokeLocalAuthSession(r *http.Request) {
	if h.authSessionStore == nil {
		return
	}
	localCookie, err := r.Cookie(authSessionCookieName)
	if err != nil || strings.TrimSpace(localCookie.Value) == "" {
		return
	}
	ssoCookie, err := r.Cookie(h.auth.CookieName)
	if err != nil || strings.TrimSpace(ssoCookie.Value) == "" {
		return
	}
	claims, err := h.auth.validateAPISIXSSOToken(ssoCookie.Value, h.authNow())
	if err != nil {
		return
	}
	claimsUser := claims.authUser()
	record, err := h.store.GetAuthUserByEmail(r.Context(), claimsUser.Email)
	if err != nil || !record.Enabled {
		return
	}
	_ = h.authSessionStore.RevokeAuthSession(r.Context(), localCookie.Value, record.ID, "manual_logout", h.authNow())
}

func safeLogoutRedirect(requestHost, value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host != logoutGatewayHost(requestHost) {
		return ""
	}
	if parsed.Path != "/api/g/sso/logouttogether" {
		return ""
	}
	return parsed.String()
}

func logoutGatewayHost(host string) string {
	hostname := strings.ToLower(strings.TrimSpace(host))
	if colon := strings.Index(hostname, ":"); colon >= 0 {
		hostname = hostname[:colon]
	}
	switch hostname {
	case "lite.sy.soyoung.com":
		return "security-test.sy.soyoung.com"
	case "lite.soyoung.com":
		return "security.soyoung.com"
	default:
		return ""
	}
}

func (h *Handler) clearAuthCookie(w http.ResponseWriter, r *http.Request) {
	for _, domain := range authCookieClearDomains(r.Host) {
		http.SetCookie(w, expiredAuthCookie(h.auth.CookieName, domain))
	}
	http.SetCookie(w, expiredAuthCookie(authSessionCookieName, ""))
}

func expiredAuthCookie(name, domain string) *http.Cookie {
	cookie := &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1}
	if domain != "" {
		cookie.Domain = domain
	}
	return cookie
}

func (h *Handler) setAuthSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     authSessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func authCookieClearDomains(host string) []string {
	hostname := strings.ToLower(strings.TrimSpace(host))
	if colon := strings.Index(hostname, ":"); colon >= 0 {
		hostname = hostname[:colon]
	}
	domains := []string{""}
	if hostname == "" || hostname == "localhost" || strings.Count(hostname, ".") == 0 {
		return domains
	}
	domains = append(domains, hostname)
	if strings.HasSuffix(hostname, ".sy.soyoung.com") {
		domains = append(domains, "sy.soyoung.com")
	} else if strings.HasSuffix(hostname, ".soyoung.com") {
		domains = append(domains, "soyoung.com")
	}
	return uniqueStrings(domains)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func (config AuthConfig) validateAPISIXSSOToken(token string, now time.Time) (*apisixSSOTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid jwt format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid jwt header")
	}
	var header apisixSSOTokenHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, errors.New("invalid jwt header")
	}
	if header.Algorithm != "RS256" {
		return nil, errors.New("unexpected jwt algorithm")
	}

	publicKey, err := parseRSAPublicKey(config.JWTPublicKey)
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("invalid jwt signature")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return nil, errors.New("invalid jwt signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid jwt payload")
	}
	var claims apisixSSOTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, errors.New("invalid jwt payload")
	}
	if claims.Exp == 0 || now.After(time.Unix(claims.Exp, 0).Add(config.ClockSkew)) {
		return nil, errors.New("expired jwt")
	}
	if config.ExpectedSub != "" && claims.Sub != config.ExpectedSub {
		return nil, errors.New("unexpected jwt subject")
	}
	if config.RequireEmail && strings.TrimSpace(claims.Data.Mail) == "" {
		return nil, errors.New("missing jwt mail")
	}
	return &claims, nil
}

func parseRSAPublicKey(value string) (*rsa.PublicKey, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), `\n`, "\n")
	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil, errors.New("invalid sso public key")
	}
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.New("invalid sso public key")
	}
	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("invalid sso public key type")
	}
	return rsaPublicKey, nil
}

func (claims apisixSSOTokenClaims) authUser() AuthUserResponse {
	email := strings.TrimSpace(claims.Data.Mail)
	username := firstNonEmpty(claims.Data.Username, email)
	return AuthUserResponse{
		Email:        email,
		Username:     username,
		DisplayName:  firstNonEmpty(claims.Data.Display, username, email),
		OpenID:       strings.TrimSpace(claims.Data.OpenID),
		FeishuUserID: strings.TrimSpace(claims.Data.UserID),
		Phone:        strings.TrimSpace(claims.Data.Phone),
		LoginWay:     strings.TrimSpace(claims.Data.LoginWay),
		Subject:      strings.TrimSpace(claims.Sub),
		Role:         "admin",
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
