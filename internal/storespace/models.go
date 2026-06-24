package storespace

import "time"

type AreaType string

const (
	AreaTypeTreatment    AreaType = "treatment"
	AreaTypeConsultation AreaType = "consultation"
	AreaTypeBeauty       AreaType = "beauty"
)

type AreaSource string

const (
	AreaSourceManual       AreaSource = "manual"
	AreaSourceDesignPlan   AreaSource = "design_plan"
	AreaSourceVideoChannel AreaSource = "video_channel"
	AreaSourceMultiple     AreaSource = "multiple"
)

type AreaStatus string

const (
	AreaStatusCandidate AreaStatus = "candidate"
	AreaStatusConfirmed AreaStatus = "confirmed"
)

type DesignPlanStatus string

const (
	DesignPlanStatusNotUploaded        DesignPlanStatus = "not_uploaded"
	DesignPlanStatusPendingRecognition DesignPlanStatus = "pending_recognition"
	DesignPlanStatusPendingAnnotation  DesignPlanStatus = "pending_annotation"
	DesignPlanStatusCompleted          DesignPlanStatus = "completed"
)

type OverallStatus string

const (
	OverallStatusIncomplete OverallStatus = "incomplete"
	OverallStatusPartial    OverallStatus = "partial"
	OverallStatusCompleted  OverallStatus = "completed"
	OverallStatusException  OverallStatus = "exception"
)

type RecognitionStatus string

const (
	RecognitionStatusNotStarted RecognitionStatus = "not_started"
	RecognitionStatusRunning    RecognitionStatus = "running"
	RecognitionStatusFailed     RecognitionStatus = "failed"
	RecognitionStatusCompleted  RecognitionStatus = "completed"
)

type RecorderStatus string

const (
	RecorderStatusOnline  RecorderStatus = "online"
	RecorderStatusOffline RecorderStatus = "offline"
)

type ChannelStatus string

const (
	ChannelStatusPendingRecognition   ChannelStatus = "pending_recognition"
	ChannelStatusPendingConfirmation  ChannelStatus = "pending_confirmation"
	ChannelStatusConfirmedBusiness    ChannelStatus = "confirmed_business"
	ChannelStatusConfirmedNonBusiness ChannelStatus = "confirmed_non_business"
	ChannelStatusRecognitionFailed    ChannelStatus = "recognition_failed"
	ChannelStatusInactive             ChannelStatus = "inactive"
)

type SceneType string

const (
	SceneTypeTreatment    SceneType = "treatment"
	SceneTypeConsultation SceneType = "consultation"
	SceneTypeBeauty       SceneType = "beauty"
	SceneTypeFrontDesk    SceneType = "front_desk"
	SceneTypeCorridor     SceneType = "corridor"
	SceneTypePassage      SceneType = "passage"
	SceneTypeWaitingArea  SceneType = "waiting_area"
	SceneTypeHall         SceneType = "hall"
	SceneTypeEntrance     SceneType = "entrance"
	SceneTypeStorage      SceneType = "storage"
	SceneTypePharmacy     SceneType = "pharmacy"
	SceneTypeMachineRoom  SceneType = "machine_room"
	SceneTypeUnknown      SceneType = "unknown"
)

type StoreFilters struct {
	Query    string
	Page     int
	PageSize int
}

type CreateStoreInput struct {
	City               string          `json:"city"`
	Name               string          `json:"name"`
	ExternalOrgID      string          `json:"external_org_id,omitempty"`
	DesignPlanUploadID string          `json:"design_plan_upload_id,omitempty"`
	Recorders          []RecorderInput `json:"recorders,omitempty"`
}

type UpdateStoreBasicInfoInput struct {
	City          string `json:"city"`
	Name          string `json:"name"`
	ExternalOrgID string `json:"external_org_id,omitempty"`
}

type CreateEzvizAccountInput struct {
	AccountName string `json:"account_name"`
}

type RecorderInput struct {
	EzvizAccountID int64  `json:"ezviz_account_id,omitempty"`
	DeviceCode     string `json:"device_code"`
}

type AddRecorderInput struct {
	EzvizAccountID int64  `json:"ezviz_account_id,omitempty"`
	DeviceCode     string `json:"device_code"`
}

type ScannedChannel struct {
	ChannelNo   int
	ChannelName string
	Active      bool
}

type ChannelInput struct {
	ChannelNo   int
	ChannelName string
	IsActive    bool
}

type ChannelSnapshotInput struct {
	ThumbnailPath      string
	FullImagePath      string
	FullImageExpiresAt *time.Time
	RecognitionResult  string
	Status             ChannelStatus
	SceneType          SceneType
	AreaType           AreaType
	AreaNumberText     string
	AreaNote           string
	CountAttempt       bool
}

type SnapshotDiagnostics struct {
	Code         string `json:"code"`
	Stage        string `json:"stage"`
	AssetStore   string `json:"asset_store"`
	SnapshotName string `json:"snapshot_name"`
	SnapshotKey  string `json:"snapshot_key"`
	Exists       bool   `json:"exists"`
	Detail       string `json:"detail,omitempty"`
}

type ChannelRecognitionResult struct {
	SceneType      string
	AreaType       string
	AreaNumber     string
	CardText       string
	DecisionSource string
	Confidence     string
	NeedsReview    bool
	RawNotes       string
	Provider       string
	RawResult      string
}

type ChannelConfirmationInput struct {
	Kind       string    `json:"kind,omitempty"`
	AreaType   AreaType  `json:"area_type,omitempty"`
	AreaNumber string    `json:"area_number,omitempty"`
	AreaNote   string    `json:"area_note,omitempty"`
	SceneType  SceneType `json:"scene_type,omitempty"`
}

type SaveDesignPlanInput struct {
	UploadID          string            `json:"upload_id,omitempty"`
	PDFFileName       string            `json:"pdf_file_name,omitempty"`
	OriginalPDFPath   string            `json:"original_pdf_path,omitempty"`
	PreviewImagePath  string            `json:"preview_image_path,omitempty"`
	ThumbnailPath     string            `json:"thumbnail_path,omitempty"`
	PageCount         int               `json:"page_count,omitempty"`
	RecognitionResult string            `json:"recognition_result,omitempty"`
	Areas             []DesignAreaInput `json:"areas"`
}

type DesignAreaInput struct {
	ID          int64    `json:"id,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	Type        AreaType `json:"area_type"`
	NumberText  string   `json:"area_number"`
	Confidence  string   `json:"confidence,omitempty"`
	NeedsReview bool     `json:"needs_review,omitempty"`
	Box         *AreaBox `json:"box,omitempty"`
}

type DuplicateCheckRequest struct {
	Name           string `json:"name"`
	ExcludeStoreID int64  `json:"exclude_store_id,omitempty"`
}

type DuplicateMatch struct {
	ID             int64         `json:"id"`
	Name           string        `json:"name"`
	NormalizedName string        `json:"normalized_name,omitempty"`
	Reason         string        `json:"reason"`
	OverallStatus  OverallStatus `json:"overall_status"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type DuplicateCheckResult struct {
	ExactMatch     *DuplicateMatch  `json:"exact_match"`
	SimilarMatches []DuplicateMatch `json:"similar_matches"`
}

type AreaLookup struct {
	StoreID    int64      `json:"store_id"`
	Type       AreaType   `json:"area_type"`
	NumberText string     `json:"area_number"`
	Source     AreaSource `json:"source,omitempty"`
}

type Store struct {
	ID               int64            `json:"id"`
	City             string           `json:"city"`
	Name             string           `json:"name"`
	NormalizedName   string           `json:"normalized_name,omitempty"`
	ExternalOrgID    string           `json:"external_org_id"`
	DesignPlanStatus DesignPlanStatus `json:"design_plan_status"`
	OverallStatus    OverallStatus    `json:"overall_status"`
	Areas            []Area           `json:"areas,omitempty"`
	DesignPlans      []DesignPlan     `json:"design_plans,omitempty"`
	Recorders        []Recorder       `json:"recorders,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

type StoreListItem struct {
	ID                     int64            `json:"id"`
	City                   string           `json:"city"`
	Name                   string           `json:"name"`
	ExternalOrgID          string           `json:"external_org_id"`
	DesignPlanStatus       DesignPlanStatus `json:"design_plan_status"`
	OverallStatus          OverallStatus    `json:"overall_status"`
	RecorderCount          int              `json:"recorder_count"`
	ChannelCount           int              `json:"channel_count"`
	ChannelsFullyConfirmed bool             `json:"channels_fully_confirmed"`
	TreatmentCount         int              `json:"treatment_count"`
	ConsultationCount      int              `json:"consultation_count"`
	BeautyCount            int              `json:"beauty_count"`
	UpdatedAt              time.Time        `json:"updated_at"`
}

type StoreListResult struct {
	Items    []StoreListItem `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int             `json:"total"`
}

type Area struct {
	ID          int64      `json:"id"`
	StoreID     int64      `json:"store_id"`
	Type        AreaType   `json:"area_type"`
	Number      int        `json:"area_number"`
	DisplayName string     `json:"display_name"`
	Source      AreaSource `json:"source"`
	Status      AreaStatus `json:"status"`
	Box         *AreaBox   `json:"box,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type AreaBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type DesignPlan struct {
	ID                int64             `json:"id"`
	StoreID           int64             `json:"store_id"`
	UploadID          string            `json:"upload_id,omitempty"`
	PDFFileName       string            `json:"pdf_file_name"`
	OriginalPDFPath   string            `json:"original_pdf_path"`
	PreviewImagePath  string            `json:"preview_image_path"`
	ThumbnailPath     string            `json:"thumbnail_path"`
	PageCount         int               `json:"page_count"`
	RecognitionStatus RecognitionStatus `json:"recognition_status"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type Recorder struct {
	ID                    int64          `json:"id"`
	StoreID               int64          `json:"store_id"`
	EzvizAccountID        int64          `json:"ezviz_account_id,omitempty"`
	DeviceCode            string         `json:"device_code"`
	Status                RecorderStatus `json:"status"`
	EffectiveChannelCount int            `json:"effective_channel_count"`
	LastScannedAt         *time.Time     `json:"last_scanned_at,omitempty"`
	Channels              []Channel      `json:"channels,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

type Channel struct {
	ID                  int64         `json:"id"`
	RecorderID          int64         `json:"recorder_id"`
	ChannelNo           int           `json:"channel_no"`
	ChannelName         string        `json:"channel_name"`
	Status              ChannelStatus `json:"status"`
	IsActive            bool          `json:"is_active"`
	SceneType           SceneType     `json:"scene_type"`
	AreaType            AreaType      `json:"area_type,omitempty"`
	AreaNumber          int           `json:"area_number,omitempty"`
	AreaNote            string        `json:"area_note,omitempty"`
	AreaID              int64         `json:"area_id,omitempty"`
	RecognitionAttempts int           `json:"recognition_attempts"`
	RecognitionResult   string        `json:"recognition_result,omitempty"`
	ThumbnailURL        string        `json:"thumbnail_url,omitempty"`
	FullImageURL        string        `json:"full_image_url,omitempty"`
	FullImageExpiresAt  *time.Time    `json:"full_image_expires_at,omitempty"`
	ConfirmedAt         *time.Time    `json:"confirmed_at,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
}

type ProbeRecognizeChannelInput struct {
	ChannelNo int `json:"channel_no"`
}

type ProbeRecognizeChannelResult struct {
	Channel *Channel `json:"channel,omitempty"`
	Active  bool     `json:"active"`
	Message string   `json:"message,omitempty"`
}

type ChannelMappingExport struct {
	FileName    string
	Content     []byte
	ContentType string
}

type ChannelMappingExportRow struct {
	Index         int
	City          string
	StoreName     string
	ExternalOrgID string
	RecorderCode  string
	ChannelNo     int
	SnapshotPath  string
	AreaTypeLabel string
	NumberOrNote  string
}

type EzvizAccount struct {
	ID             int64      `json:"id"`
	AccountName    string     `json:"account_name"`
	Status         string     `json:"status"`
	LastVerifiedAt *time.Time `json:"last_verified_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
