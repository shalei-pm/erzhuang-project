package main

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServerHasExplicitResourceLimits(t *testing.T) {
	server := newHTTPServer("127.0.0.1:18080", http.NotFoundHandler())
	if server.ReadHeaderTimeout != 10*time.Second || server.ReadTimeout != 30*time.Second || server.WriteTimeout != 60*time.Second || server.IdleTimeout != 120*time.Second {
		t.Fatalf("unexpected server timeouts: %#v", server)
	}
	if server.MaxHeaderBytes != 32<<10 {
		t.Fatalf("MaxHeaderBytes=%d, want %d", server.MaxHeaderBytes, 32<<10)
	}
}
