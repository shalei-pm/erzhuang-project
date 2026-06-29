package h5monitor

import (
	"strings"
	"time"
)

type MonitorChannel struct {
	ID           int64  `json:"id"`
	ChannelNo    int    `json:"channel_no"`
	ChannelName  string `json:"channel_name"`
	Category     string `json:"category"`
	AreaType     string `json:"area_type"`
	SceneType    string `json:"scene_type"`
	AreaNumber   int    `json:"area_number"`
	AreaNote     string `json:"area_note"`
	ThumbnailURL string `json:"thumbnail_url"`
}

type MonitorGroup struct {
	Category string           `json:"category"`
	Label    string           `json:"label"`
	Channels []MonitorChannel `json:"channels"`
}

type MonitorHomeResponse struct {
	ExternalOrgID string         `json:"external_org_id"`
	StoreName     string         `json:"store_name"`
	City          string         `json:"city"`
	Groups        []MonitorGroup `json:"groups"`
}

type LiveURLRequest struct {
	UserID   string `json:"user_id,omitempty"`
	IsAdmin  bool   `json:"is_admin,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}

type LiveURLResponse struct {
	URL        string `json:"url"`
	ExpireTime string `json:"expire_time"`
	URLID      string `json:"url_id"`
	Protocol   string `json:"protocol"`
}

type SnapshotRefreshResponse struct {
	ThumbnailURL string `json:"thumbnail_url"`
}

type RecordSegmentResponse struct {
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	Type      string `json:"type"`
	TypeLabel string `json:"type_label"`
}

type RecordSegmentsResponse struct {
	Date     string                  `json:"date"`
	Segments []RecordSegmentResponse `json:"segments"`
}

type PlaybackURLRequest struct {
	StartTime int64  `json:"start_time"`
	StopTime  int64  `json:"stop_time"`
	UserID    string `json:"user_id,omitempty"`
	IsAdmin   bool   `json:"is_admin,omitempty"`
}

type PlaybackURLResponse struct {
	URL        string `json:"url"`
	ExpireTime string `json:"expire_time"`
	URLID      string `json:"url_id"`
}

type DisableURLRequest struct {
	URLID  string `json:"url_id"`
	UserID string `json:"user_id,omitempty"`
}

type concurrencyState struct {
	ActiveCount int
	MaxCount    int
	AcquiredAt  time.Time
}

type snapshotRefreshState struct {
	thumbnailURL string
	refreshedAt  time.Time
}

func channelCategoryLabel(category string) string {
	switch category {
	case "consultation":
		return "面诊室"
	case "treatment":
		return "治疗室"
	case "beauty":
		return "生美"
	case "front_waiting":
		return "前台/候诊"
	default:
		return "其他"
	}
}

func segmentTypeLabel(segmentType string) string {
	switch strings.ToUpper(segmentType) {
	case "ALARM":
		return "事件录像"
	case "PLAN":
		return "定时录像"
	default:
		return segmentType
	}
}
