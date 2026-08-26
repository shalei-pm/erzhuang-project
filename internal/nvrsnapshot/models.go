package nvrsnapshot

import "strconv"

const (
	SnapshotStatusSucceeded ErrorCode = "succeeded"
	ErrorOSSUploadFailed    ErrorCode = "oss_upload_failed"
)

type Selection struct {
	TenantID int64
	CameraID int64
}

type Candidate struct {
	TenantID int64
	CameraID int64
}

func snapshotObjectKey(tenantID, cameraID int64) string {
	return "nvr-camera-snapshots/" + formatPositiveID(tenantID) + "/" + formatPositiveID(cameraID) + ".jpg"
}

func formatPositiveID(value int64) string {
	if value <= 0 {
		return "0"
	}
	return strconv.FormatInt(value, 10)
}
