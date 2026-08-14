package ezviz

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func farFuture() time.Time {
	return time.Now().Add(24 * time.Hour)
}

func TestEnsureAACTransferSendsHeadersAndRefreshesToken(t *testing.T) {
	var tokenRequests int
	var transferRequests int

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/service/media/aac/transfer":
			transferRequests++
			if r.URL.Query().Get("enable") != "1" {
				t.Fatalf("unexpected enable query %q", r.URL.Query().Get("enable"))
			}
			if r.Header.Get("accessToken") == "stale-token" {
				return jsonResponse(`{"meta":{"code":10002,"message":"token expired"},"data":null}`), nil
			}
			if r.Header.Get("accessToken") != "fresh-token" {
				t.Fatalf("unexpected accessToken header %q", r.Header.Get("accessToken"))
			}
			if r.Header.Get("deviceSerial") != "AZ3988334" {
				t.Fatalf("unexpected deviceSerial header %q", r.Header.Get("deviceSerial"))
			}
			if r.Header.Get("localIndex") != "3" {
				t.Fatalf("unexpected localIndex header %q", r.Header.Get("localIndex"))
			}
			return jsonResponse(`{"meta":{"code":200,"message":"ok"},"data":null}`), nil
		case "/api/lapp/token/get":
			tokenRequests++
			return jsonResponse(`{"code":"200","msg":"ok","data":{"accessToken":"fresh-token","expireTime":9999999999999}}`), nil
		default:
			return jsonResponse(`{"code":"404","msg":"not found"}`), nil
		}
	})

	client := NewClient(ClientOptions{BaseURL: "https://ezviz.test", HTTPClient: &http.Client{Transport: transport}})
	client.mu.Lock()
	client.tokens["test"] = tokenCache{accessToken: "stale-token", expiresAt: farFuture()}
	client.mu.Unlock()

	err := client.EnsureAACTransfer(context.Background(), Account{Name: "test", AppKey: "k", AppSecret: "s"}, "az3988334", 3)
	if err != nil {
		t.Fatalf("ensure aac transfer: %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("expected one token refresh, got %d", tokenRequests)
	}
	if transferRequests != 2 {
		t.Fatalf("expected transfer retry, got %d", transferRequests)
	}
}
