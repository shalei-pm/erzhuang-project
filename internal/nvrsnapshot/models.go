package nvrsnapshot

import "time"

const (
	SnapshotStatusSucceeded ErrorCode = "succeeded"
	ErrorOSSUploadFailed    ErrorCode = "oss_upload_failed"
)

type SelectionMode string

const (
	SelectionMissingOnly  SelectionMode = "missing_only"
	SelectionResumeFailed SelectionMode = "resume_failed"
)

type Selection struct {
	TenantID int64
	CameraID int64
	Mode     SelectionMode
}

type Candidate struct {
	TenantID int64
	CameraID int64
}

type Snapshot struct {
	TenantID    int64
	CameraID    int64
	Status      ErrorCode
	ObjectKey   string
	ContentType string
	Width       int
	Height      int
	ByteSize    int
	CapturedAt  *time.Time
	AttemptedAt time.Time
	ErrorCode   ErrorCode
}

func snapshotObjectKey(tenantID, cameraID int64) string {
	return "nvr-camera-snapshots/" + formatPositiveID(tenantID) + "/" + formatPositiveID(cameraID) + ".jpg"
}
