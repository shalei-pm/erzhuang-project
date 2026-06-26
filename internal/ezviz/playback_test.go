package ezviz

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPlaybackAddressRequestsTypeTwoFLVAndRefreshesToken(t *testing.T) {
	var tokenRequests int
	var playbackRequests int

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		switch r.URL.Path {
		case "/api/lapp/v2/live/address/get":
			playbackRequests++
			if r.Form.Get("accessToken") == "stale-token" {
				return jsonResponse(`{"code":"10014","msg":"token expired","data":null}`), nil
			}
			if r.Form.Get("type") != "2" {
				t.Fatalf("unexpected type %q", r.Form.Get("type"))
			}
			if r.Form.Get("protocol") != "4" {
				t.Fatalf("unexpected protocol %q", r.Form.Get("protocol"))
			}
			if r.Form.Get("supportH265") != "1" {
				t.Fatalf("unexpected supportH265 %q", r.Form.Get("supportH265"))
			}
			if r.Form.Get("mute") != "0" {
				t.Fatalf("unexpected mute %q", r.Form.Get("mute"))
			}
			if r.Form.Get("startTime") != "1731945592" || r.Form.Get("stopTime") != "1731949200" {
				t.Fatalf("unexpected playback time range %q-%q", r.Form.Get("startTime"), r.Form.Get("stopTime"))
			}
			return jsonResponse(`{"code":"200","msg":"ok","data":{"id":"play-url-1","url":"https://example.test/play.flv","expireTime":"2026-12-31 23:59:59"}}`), nil
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

	result, err := client.PlaybackAddress(context.Background(), Account{Name: "test", AppKey: "k", AppSecret: "s"}, PlaybackRequest{
		DeviceSerial: "az3988334",
		ChannelNo:    1,
		StartTime:    time.Unix(1731945592, 0),
		StopTime:     time.Unix(1731949200, 0),
	})
	if err != nil {
		t.Fatalf("playback address: %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("expected one token refresh, got %d", tokenRequests)
	}
	if playbackRequests != 2 {
		t.Fatalf("expected playback retry, got %d", playbackRequests)
	}
	if result.ID != "play-url-1" || result.URL == "" {
		t.Fatalf("unexpected playback result: %#v", result)
	}
}

func TestPlaybackAddressRejectsInvalidTimeRange(t *testing.T) {
	client := NewClient(ClientOptions{})
	client.mu.Lock()
	client.tokens["test"] = tokenCache{accessToken: "tok", expiresAt: farFuture()}
	client.mu.Unlock()

	now := time.Now()
	_, err := client.PlaybackAddress(context.Background(), Account{Name: "test", AppKey: "k", AppSecret: "s"}, PlaybackRequest{
		DeviceSerial: "AZ3988334",
		ChannelNo:    1,
		StartTime:    now,
		StopTime:     now,
	})
	if err == nil || !strings.Contains(err.Error(), "stopTime") {
		t.Fatalf("expected stopTime validation error, got %v", err)
	}
}

func TestDisableLiveAddressSendsID(t *testing.T) {
	var capturedID string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		capturedID = r.Form.Get("id")
		return jsonResponse(`{"code":"200","msg":"ok","data":null}`), nil
	})

	client := NewClient(ClientOptions{BaseURL: "https://ezviz.test", HTTPClient: &http.Client{Transport: transport}})
	client.mu.Lock()
	client.tokens["test"] = tokenCache{accessToken: "tok", expiresAt: farFuture()}
	client.mu.Unlock()

	if err := client.DisableLiveAddress(context.Background(), Account{Name: "test", AppKey: "k", AppSecret: "s"}, "url-id-123"); err != nil {
		t.Fatalf("disable live address: %v", err)
	}
	if capturedID != "url-id-123" {
		t.Fatalf("unexpected id %q", capturedID)
	}
}
