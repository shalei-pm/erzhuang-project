package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// loginTestSession explicitly exchanges the request's SSO cookie for a local
// session. Callers reuse the returned cookie with this same handler.
func loginTestSession(t *testing.T, handler http.Handler, request *http.Request) *http.Cookie {
	t.Helper()
	callbackURL := *request.URL
	callbackURL.Path = normalizeBasePath(os.Getenv("APP_BASE_PATH")) + "/_/auth/callback"
	callbackURL.RawPath = ""
	callbackURL.RawQuery = ""
	callback := httptest.NewRequest(http.MethodGet, callbackURL.String(), nil)
	callback.Host = request.Host
	callback.Header = request.Header.Clone()
	callback.RemoteAddr = request.RemoteAddr
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, callback)
	if response.Code != http.StatusFound {
		t.Fatalf("login callback status = %d, want 302; body=%s", response.Code, response.Body.String())
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == authSessionCookieName && cookie.Value != "" && cookie.HttpOnly && cookie.MaxAge >= 0 {
			return cookie
		}
	}
	t.Fatal("login callback did not issue an HttpOnly local session cookie")
	return nil
}
