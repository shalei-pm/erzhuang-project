package main

import "testing"

func TestRunRejectsInvalidBackfillArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing tenant", args: nil},
		{name: "negative camera", args: []string{"--tenant-id=10001", "--camera-id=-1"}},
		{name: "all tenants with camera", args: []string{"--all-tenants", "--camera-id=111"}},
		{name: "all tenants with tenant", args: []string{"--all-tenants", "--tenant-id=10001"}},
		{name: "timeout above limit", args: []string{"--tenant-id=10001", "--timeout-per-camera=21s"}},
		{name: "interval below limit", args: []string{"--tenant-id=10001", "--request-interval=1s"}},
		{name: "unexpected positional argument", args: []string{"--tenant-id=10001", "unexpected"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(tt.args); got != 2 {
				t.Fatalf("run(%q) = %d, want 2", tt.args, got)
			}
		})
	}
}

func TestRunFailsClosedWithoutRuntimeSecrets(t *testing.T) {
	t.Setenv("K8S_SECRET_MYSQL_DSN", "")
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("K8S_SECRET_NVR_STREAM_AUTHORIZATION", "")
	t.Setenv("NVR_STREAM_AUTHORIZATION", "")

	if got := run([]string{"--tenant-id=10001", "--camera-id=111"}); got != 1 {
		t.Fatalf("run() = %d, want 1 when runtime secrets are unavailable", got)
	}
}
