package resourceview

import "time"

type StoreFilters struct {
	Query    string
	CityID   int64
	Page     int
	PageSize int
}

type BusinessTenant struct {
	ID           int64
	Name         string
	HospitalName string
	Status       int
	ProvinceID   int64
	CityID       int64
}

type BusinessDevice struct {
	ID           int64
	TenantID     int64
	ParentID     int64
	Name         string
	HardwareID   string
	SN           string
	IP           string
	Category     string
	Provider     string
	Status       int
	OnlineStatus int
	ExtParams    string
	HeartbeatAt  *time.Time
	DeletedAt    *time.Time
}

type BusinessSpace struct {
	ID        int64
	TenantID  int64
	ParentID  int64
	Name      string
	Code      string
	Level     int
	Status    int
	DictID    int64
	SortOrder int
}

type BusinessAreaDeviceRelation struct {
	ID           int64
	DeviceID     int64
	AreaID       int64
	FunctionType string
	CreatedAt    time.Time
}

type StoreRecords struct {
	Tenant                BusinessTenant
	Devices               []BusinessDevice
	Spaces                []BusinessSpace
	Relations             []BusinessAreaDeviceRelation
	LegacyCameraSnapshots map[int]string
}

type MonitorAccess struct {
	CanViewMonitor bool
	MonitorURL     string
}

type StoreListResult struct {
	Items    []StoreListItem `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int             `json:"total"`
	Summary  StoreSummary    `json:"summary"`
	Cities   []CityOption    `json:"cities"`
}

type CityOption struct {
	CityID int64  `json:"city_id"`
	Name   string `json:"name"`
	Count  int    `json:"count"`
}

type StoreSummary struct {
	StoreCount              int `json:"store_count"`
	EdgeCount               int `json:"edge_count"`
	NVRCount                int `json:"nvr_count"`
	CameraCount             int `json:"camera_count"`
	SpaceCount              int `json:"space_count"`
	ConsultationCameraCount int `json:"consultation_camera_count"`
	TreatmentCameraCount    int `json:"treatment_camera_count"`
	BoundCameraCount        int `json:"bound_camera_count"`
	UnboundCameraCount      int `json:"unbound_camera_count"`
	OfflineDeviceCount      int `json:"offline_device_count"`
	WarningCount            int `json:"warning_count"`
}

type StoreListItem struct {
	TenantID                int64  `json:"tenant_id"`
	StoreName               string `json:"store_name"`
	HospitalName            string `json:"hospital_name"`
	CityID                  int64  `json:"city_id"`
	CityName                string `json:"city_name"`
	EdgeCount               int    `json:"edge_count"`
	NVRCount                int    `json:"nvr_count"`
	CameraCount             int    `json:"camera_count"`
	SpaceCount              int    `json:"space_count"`
	ConsultationCameraCount int    `json:"consultation_camera_count"`
	TreatmentCameraCount    int    `json:"treatment_camera_count"`
	BoundCameraCount        int    `json:"bound_camera_count"`
	UnboundCameraCount      int    `json:"unbound_camera_count"`
	OfflineDeviceCount      int    `json:"offline_device_count"`
	WarningCount            int    `json:"warning_count"`
	CamerasFullyBound       bool   `json:"cameras_fully_bound"`
	UpdatedAt               string `json:"updated_at,omitempty"`
	CanViewMonitor          bool   `json:"can_view_monitor"`
	MonitorURL              string `json:"monitor_url,omitempty"`
}

type StoreDetail struct {
	TenantID       int64                `json:"tenant_id"`
	StoreName      string               `json:"store_name"`
	HospitalName   string               `json:"hospital_name"`
	CityID         int64                `json:"city_id"`
	CityName       string               `json:"city_name"`
	Summary        StoreSummary         `json:"summary"`
	Edges          []Device             `json:"edges"`
	NVRs           []Device             `json:"nvrs"`
	Cameras        []Camera             `json:"cameras"`
	Spaces         []Space              `json:"spaces"`
	Relations      []AreaDeviceRelation `json:"relations"`
	SpaceTree      []SpaceNode          `json:"space_tree"`
	DeviceTree     DeviceTree           `json:"device_tree"`
	Issues         []Issue              `json:"issues"`
	CanViewMonitor bool                 `json:"can_view_monitor"`
	MonitorURL     string               `json:"monitor_url,omitempty"`
}

type Device struct {
	ID           int64   `json:"id"`
	ParentID     int64   `json:"parent_id,omitempty"`
	TenantID     int64   `json:"tenant_id"`
	Name         string  `json:"name"`
	HardwareID   string  `json:"hardware_id"`
	SN           string  `json:"sn,omitempty"`
	IP           string  `json:"ip,omitempty"`
	Category     string  `json:"category"`
	Provider     string  `json:"provider,omitempty"`
	Status       int     `json:"status"`
	StatusText   string  `json:"status_text"`
	OnlineStatus int     `json:"online_status"`
	OnlineText   string  `json:"online_text"`
	ExtSummary   string  `json:"ext_summary,omitempty"`
	HeartbeatAt  *string `json:"heartbeat_at,omitempty"`
}

type Camera struct {
	Device
	ChannelNo     *int     `json:"channel_no,omitempty"`
	NVRID         int64    `json:"nvr_id,omitempty"`
	NVRName       string   `json:"nvr_name,omitempty"`
	SpacePaths    []string `json:"space_paths"`
	ThumbnailKind string  `json:"thumbnail_kind,omitempty"`
	ThumbnailURL  string   `json:"thumbnail_url,omitempty"`
}

type Space struct {
	ID               int64   `json:"id"`
	TenantID         int64   `json:"tenant_id"`
	ParentID         int64   `json:"parent_id,omitempty"`
	Name             string  `json:"name"`
	Code             string  `json:"code,omitempty"`
	Level            int     `json:"level"`
	Status           int     `json:"status"`
	StatusText       string  `json:"status_text"`
	DictID           int64   `json:"dict_id,omitempty"`
	SortOrder        int     `json:"sort_order"`
	BoundCameraIDs   []int64 `json:"bound_camera_ids"`
	BoundCameraCount int     `json:"bound_camera_count"`
}

type AreaDeviceRelation struct {
	ID           int64  `json:"id"`
	DeviceID     int64  `json:"device_id"`
	AreaID       int64  `json:"area_id"`
	FunctionType string `json:"function_type"`
}

type SpaceNode struct {
	Space
	BoundCameras []Camera    `json:"bound_cameras"`
	Children     []SpaceNode `json:"children"`
}

type DeviceTree struct {
	Edges []Device  `json:"edges"`
	NVRs  []NVRNode `json:"nvrs"`
}

type NVRNode struct {
	Device
	Cameras []Camera `json:"cameras"`
}

type IssueSeverity string
type IssueType string

const (
	IssueSeverityError IssueSeverity = "error"
	IssueSeverityWarn  IssueSeverity = "warning"
	IssueSeverityInfo  IssueSeverity = "info"

	IssueUnboundCamera         IssueType = "unbound_camera"
	IssueInactiveBoundSpace    IssueType = "inactive_bound_space"
	IssueMissingCamera         IssueType = "missing_camera"
	IssueMissingSpace          IssueType = "missing_space"
	IssueMissingNVR            IssueType = "missing_nvr"
	IssueOfflineEdge           IssueType = "offline_edge"
	IssueOfflineNVR            IssueType = "offline_nvr"
	IssueOfflineCamera         IssueType = "offline_camera"
	IssueCameraBoundManySpaces IssueType = "camera_bound_many_spaces"
	IssueSpaceBoundManyCameras IssueType = "space_bound_many_cameras"
)

type Issue struct {
	Severity   IssueSeverity `json:"severity"`
	Type       IssueType     `json:"type"`
	Message    string        `json:"message"`
	EntityType string        `json:"entity_type"`
	EntityID   int64         `json:"entity_id"`
}
