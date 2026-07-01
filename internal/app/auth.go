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
	"net/http"
	"os"
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
	Enabled       bool              `json:"enabled"`
	Authenticated bool              `json:"authenticated"`
	LoginURL      string            `json:"login_url,omitempty"`
	User          *AuthUserResponse `json:"user,omitempty"`
	Permissions   []string          `json:"permissions,omitempty"`
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
	if !h.auth.Enabled {
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
		return
	}

	cookie, err := r.Cookie(h.auth.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		h.writeUnauthorizedAuth(w)
		return
	}

	claims, err := h.auth.validateAPISIXSSOToken(cookie.Value, time.Now())
	if err != nil {
		h.writeUnauthorizedAuth(w)
		return
	}
	user := claims.authUser()
	writeJSON(w, http.StatusOK, AuthResponse{
		Enabled:       true,
		Authenticated: true,
		User:          &user,
		Permissions:   []string{"admin"},
	})
}

func (h *Handler) writeUnauthorizedAuth(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, AuthResponse{
		Enabled:       true,
		Authenticated: false,
		LoginURL:      normalizeBasePath(os.Getenv("APP_BASE_PATH")) + "/_/auth/callback",
	})
}

func (h *Handler) authCallbackHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, normalizeBasePath(os.Getenv("APP_BASE_PATH"))+"/", http.StatusFound)
}

func (h *Handler) authLogoutHandler(w http.ResponseWriter, r *http.Request) {
	h.clearAuthCookie(w)
	if r.Method == http.MethodGet {
		http.Redirect(w, r, normalizeBasePath(os.Getenv("APP_BASE_PATH"))+"/", http.StatusFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.auth.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
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
