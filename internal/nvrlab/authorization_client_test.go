package nvrlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestHTTPAuthorizationClientUsesLiveParametersWithoutTimes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "service-secret" {
			t.Fatalf("Authorization = %q", got)
		}
		query := r.URL.Query()
		if query.Get("camera_id") != "111" || query.Get("stream_type") != "2" {
			t.Fatalf("query = %v", query)
		}
		if query.Has("start_time") || query.Has("end_time") {
			t.Fatalf("live query unexpectedly has playback times: %v", query)
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"token":"issued-session"}}`))
	}))
	defer server.Close()

	client := NewHTTPAuthorizationClient(server.Client(), "service-secret")
	client.authorizationEndpoint = server.URL
	client.webSocketEndpoint = "wss://stream.example.test/nvrapi/ws"

	streamURL, err := client.CreateStreamURL(context.Background(), 111, StreamSessionRequest{Mode: ModeLive})
	if err != nil {
		t.Fatalf("CreateStreamURL() error = %v", err)
	}
	parsed, err := url.Parse(streamURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "wss" || parsed.Host != "stream.example.test" || parsed.Query().Get("token") != "issued-session" {
		t.Fatalf("stream URL has unexpected shape")
	}
}

func TestHTTPAuthorizationClientUsesUnixPlaybackRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("start_time") != "100" || query.Get("end_time") != "200" {
			t.Fatalf("query = %v", query)
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"token":"issued-session"}}`))
	}))
	defer server.Close()

	client := NewHTTPAuthorizationClient(server.Client(), "service-secret")
	client.authorizationEndpoint = server.URL
	client.webSocketEndpoint = "wss://stream.example.test/nvrapi/ws"

	if _, err := client.CreateStreamURL(context.Background(), 111, StreamSessionRequest{Mode: ModePlayback, StartTime: 100, EndTime: 200}); err != nil {
		t.Fatalf("CreateStreamURL() error = %v", err)
	}
}

func TestHTTPAuthorizationClientDoesNotReturnAuthorizationResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"detail":"upstream private detail"}`))
	}))
	defer server.Close()

	client := NewHTTPAuthorizationClient(server.Client(), "service-secret")
	client.authorizationEndpoint = server.URL

	_, err := client.CreateStreamURL(context.Background(), 111, StreamSessionRequest{Mode: ModeLive})
	if err != ErrAuthorizationFailed {
		t.Fatalf("CreateStreamURL() error = %v, want ErrAuthorizationFailed", err)
	}
}
