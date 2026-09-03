package h5monitor

import (
	"errors"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
)

func TestSafeLogErrorDoesNotExposeErrorMessage(t *testing.T) {
	secret := "wss://example.test/stream?token=secret-token"
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "provider error", err: &ezviz.Error{Code: "401", Msg: secret}, want: "provider_401"},
		{name: "internal error", err: errors.New(secret), want: "internal"},
		{name: "nil error", err: nil, want: "-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeLogError(tt.err)
			if got != tt.want {
				t.Fatalf("safeLogError() = %q, want %q", got, tt.want)
			}
			if got == secret || strings.Contains(got, "token") || strings.Contains(got, "wss://") {
				t.Fatalf("safeLogError() leaked sensitive content: %q", got)
			}
		})
	}
}
