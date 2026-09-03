package nvrlab

import "errors"

const ExperimentTenantID int64 = 10001

var (
	ErrExperimentNotFound  = errors.New("nvr lab experiment store not found")
	ErrCameraNotFound      = errors.New("nvr lab camera not found")
	ErrInvalidStreamMode   = errors.New("nvr lab stream mode is invalid")
	ErrInvalidPlaybackWindow = errors.New("nvr lab playback window is invalid")
	ErrNotConfigured       = errors.New("nvr lab is not configured")
	ErrAuthorizationFailed = errors.New("nvr lab authorization failed")
	ErrAuthorizationTimeout = errors.New("nvr lab authorization timed out")
)

type Mode string

const (
	ModeLive     Mode = "live"
	ModePlayback Mode = "playback"
)

type Camera struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SpaceType string `json:"space_type,omitempty"`
	SpaceName string `json:"space_name,omitempty"`
}

type CameraListResponse struct {
	TenantID  int64    `json:"tenant_id"`
	StoreName string   `json:"store_name"`
	Cameras   []Camera `json:"cameras"`
}

type StreamSessionRequest struct {
	Mode      Mode  `json:"mode"`
	StartTime int64 `json:"start_time,omitempty"`
	EndTime   int64 `json:"end_time,omitempty"`
}

type StreamSessionResponse struct {
	URL  string `json:"url"`
	Mode Mode   `json:"mode"`
}
