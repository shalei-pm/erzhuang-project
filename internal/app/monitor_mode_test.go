package app

import "testing"

func TestMonitorPlaybackModeFromEnvUsesNVRWhenExplicitlyConfigured(t *testing.T) {
	t.Setenv("MONITOR_PLAYBACK_MODE", "nvr")

	if got := MonitorPlaybackModeFromEnv(); got != MonitorPlaybackModeNVR {
		t.Fatalf("MonitorPlaybackModeFromEnv() = %q, want %q", got, MonitorPlaybackModeNVR)
	}
}

func TestMonitorPlaybackModeFromEnvFallsBackToLegacy(t *testing.T) {
	t.Setenv("MONITOR_PLAYBACK_MODE", "unexpected")

	if got := MonitorPlaybackModeFromEnv(); got != MonitorPlaybackModeLegacy {
		t.Fatalf("MonitorPlaybackModeFromEnv() = %q, want %q", got, MonitorPlaybackModeLegacy)
	}
}
