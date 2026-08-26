package nvrmonitor

import "errors"

var (
	ErrStoreNotFound          = errors.New("nvr monitor store not found")
	ErrCameraNotFound         = errors.New("nvr monitor camera not found")
	ErrInvalidStreamMode      = errors.New("nvr monitor stream mode is invalid")
	ErrInvalidPlaybackWindow  = errors.New("nvr monitor playback window is invalid")
	ErrNotConfigured          = errors.New("nvr monitor is not configured")
	ErrAuthorizationFailed    = errors.New("nvr monitor authorization failed")
	ErrAuthorizationTimeout   = errors.New("nvr monitor authorization timed out")
	ErrUnauthorized           = errors.New("nvr monitor unauthorized")
	ErrForbidden              = errors.New("nvr monitor forbidden")
)

type Mode string

const (
	ModeLive     Mode = "live"
	ModePlayback Mode = "playback"
)

type StoreInfo struct {
	ExternalOrgID         string `json:"external_org_id"`
	StoreName             string `json:"store_name"`
	City                  string `json:"city"`
	AvailableCameraCount  int    `json:"available_camera_count"`
}

type StoreCityGroup struct {
	City   string      `json:"city"`
	Stores []StoreInfo `json:"stores"`
}

type MonitorStoresResponse struct {
	Cities []StoreCityGroup `json:"cities"`
}

type Camera struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	SpaceType    string `json:"space_type,omitempty"`
	SpaceName    string `json:"space_name,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

type CameraListResponse struct {
	ExternalOrgID string   `json:"external_org_id"`
	TenantID      int64    `json:"tenant_id"`
	StoreName     string   `json:"store_name"`
	City          string   `json:"city"`
	Cameras       []Camera `json:"cameras"`
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
