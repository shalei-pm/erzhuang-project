package app

import (
	"os"
	"strings"
)

type MonitorPlaybackMode string

const (
	MonitorPlaybackModeLegacy MonitorPlaybackMode = "legacy"
	MonitorPlaybackModeNVR    MonitorPlaybackMode = "nvr"
)

// MonitorPlaybackModeFromEnv defaults to legacy so an incomplete NVR rollout
// cannot replace the established monitor path by accident.
func MonitorPlaybackModeFromEnv() MonitorPlaybackMode {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("MONITOR_PLAYBACK_MODE")), string(MonitorPlaybackModeNVR)) {
		return MonitorPlaybackModeNVR
	}
	return MonitorPlaybackModeLegacy
}
