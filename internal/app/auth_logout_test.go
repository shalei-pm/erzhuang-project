package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAPISIXSSOLogoutGetCanRedirectToGatewayLogoutAfterClearingCookies(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	t.Setenv("SSO_ENABLED", "true")

	gatewayLogout := "https://security-test.sy.soyoung.com/api/g/sso/logouttogether?from_host=lite.sy.soyoung.com&from_uri=https%3A%2F%2Flite.sy.soyoung.com%2Ferzhuang-project%2F"
	request := httptest.NewRequest(http.MethodGet, "https://lite.sy.soyoung.com/erzhuang-project/logout?redirect="+url.QueryEscape(gatewayLogout), nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: "token-value"})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, recorder.Code)
	}
	if recorder.Header().Get("Location") != gatewayLogout {
		t.Fatalf("unexpected redirect location: %s", recorder.Header().Get("Location"))
	}
	cookies := recorder.Result().Cookies()
	if !hasClearedAuthCookie(cookies, "sy_sso_token", "") {
		t.Fatalf("expected logout to clear host-only sso cookie, got %#v", cookies)
	}
	if !hasClearedAuthCookie(cookies, "sy_sso_token", "sy.soyoung.com") {
		t.Fatalf("expected logout to clear parent-domain sso cookie, got %#v", cookies)
	}
}

func TestAPISIXSSOLogoutGetRejectsUnsafeRedirect(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	t.Setenv("SSO_ENABLED", "true")

	request := httptest.NewRequest(http.MethodGet, "https://lite.sy.soyoung.com/erzhuang-project/logout?redirect="+url.QueryEscape("https://example.com/logout"), nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, recorder.Code)
	}
	if recorder.Header().Get("Location") != "/erzhuang-project/" {
		t.Fatalf("unexpected redirect location: %s", recorder.Header().Get("Location"))
	}
}
