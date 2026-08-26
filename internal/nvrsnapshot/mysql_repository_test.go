package nvrsnapshot

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestSnapshotObjectKeyUsesTenantAndGlobalCameraIdentity(t *testing.T) {
	if got, want := snapshotObjectKey(10001, 111), "nvr-camera-snapshots/10001/111.jpg"; got != want {
		t.Fatalf("snapshotObjectKey() = %q, want %q", got, want)
	}
}

func TestValidateSnapshotRejectsUnsafeOrInconsistentMetadata(t *testing.T) {
	now := time.Now().UTC()
	success := Snapshot{TenantID: 10001, CameraID: 111, Status: SnapshotStatusSucceeded, ObjectKey: snapshotObjectKey(10001, 111), ContentType: "image/jpeg", Width: 640, Height: 360, ByteSize: 100, CapturedAt: &now, AttemptedAt: now}
	if err := validateSnapshot(success); err != nil {
		t.Fatalf("valid success rejected: %v", err)
	}
	failure := Snapshot{TenantID: 10001, CameraID: 111, Status: ErrorMediaTimeout, ErrorCode: ErrorMediaTimeout, AttemptedAt: now}
	if err := validateSnapshot(failure); err != nil {
		t.Fatalf("valid failure rejected: %v", err)
	}
	for _, invalid := range []Snapshot{
		{TenantID: 10001, CameraID: 111, Status: SnapshotStatusSucceeded, ObjectKey: "nvr-camera-snapshots/10001/111.jpg", ContentType: "image/jpeg", Width: 1, Height: 1, ByteSize: 1, AttemptedAt: now},
		{TenantID: 10001, CameraID: 111, Status: ErrorMediaTimeout, ErrorCode: ErrorDecodeFailed, AttemptedAt: now},
		{TenantID: 10001, CameraID: 111, Status: ErrorCode("database_write_failed"), ErrorCode: ErrorCode("database_write_failed"), AttemptedAt: now},
	} {
		if err := validateSnapshot(invalid); err == nil {
			t.Fatalf("unsafe snapshot %#v was accepted", invalid)
		}
	}
}

func TestMySQLSnapshotRepositoryScopesCandidateReadsAndOwnedWrites(t *testing.T) {
	content, err := os.ReadFile("mysql_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(content))
	for _, required := range []string{
		"tb_crm_iot_device", "tb_nvr_camera_snapshots", "d.category = 'camera'",
		"d.provider = 'hikvisionnvrchannel'", "d.status = 1", "d.deleted_at is null",
		"s.camera_id is null", "s.status <> 'succeeded'", "on duplicate key update",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("repository source missing %q", required)
		}
	}
	for _, banned := range []string{"insert into tb_crm_", "update tb_crm_", "delete from tb_crm_", "create table"} {
		if strings.Contains(source, banned) {
			t.Fatalf("repository source contains forbidden mutation %q", banned)
		}
	}
}
