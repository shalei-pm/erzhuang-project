package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestSafeAuthReturnPath(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	for _, value := range []string{"", "https://evil.example/", "//evil.example/", "/other", "/erzhuang-project/../other", "/erzhuang-project/%2e%2e/other", "/erzhuang-project/%5cevil", "/erzhuang-project/api/auth/logout", "/erzhuang-project/_/auth/callback", "/erzhuang-project/logout", "/erzhuang-project/%252e%252e/other"} {
		if got := safeAuthReturnPath(value); got != "/erzhuang-project/" {
			t.Errorf("unsafe return %q -> %q", value, got)
		}
	}
	for _, value := range []string{"/erzhuang-project/", "/erzhuang-project/h5/orgs/10001/monitor", "/erzhuang-project/h5/orgs/10001/monitor/cameras/111?tab=playback#time"} {
		if got := safeAuthReturnPath(value); got != value {
			t.Errorf("return %q -> %q", value, got)
		}
	}
}

func TestAuthCallbackReturnsToBusinessPage(t *testing.T) {
	_, handler, key := newAuthAuditTestHandler(t)
	target := "/erzhuang-project/h5/orgs/10001/monitor/cameras/111?tab=playback"
	r := httptest.NewRequest(http.MethodGet, "https://lite.sy.soyoung.com/erzhuang-project/_/auth/callback?return_to="+url.QueryEscape(target), nil)
	r.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, key, map[string]any{
		"data": map[string]string{"mail": "logout@example.com", "display": "Claims Name"},
		"exp":  time.Now().Add(time.Hour).Unix(),
	})})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusFound || w.Header().Get("Location") != target {
		t.Fatalf("callback: %d %s", w.Code, w.Header().Get("Location"))
	}
}
