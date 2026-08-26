package nvrsnapshot

import (
	"os"
	"strings"
	"testing"
)

func TestSnapshotObjectKeyUsesTenantAndGlobalCameraIdentity(t *testing.T) {
	if got, want := snapshotObjectKey(10001, 111), "nvr-camera-snapshots/10001/111.jpg"; got != want {
		t.Fatalf("snapshotObjectKey() = %q, want %q", got, want)
	}
}

func TestMySQLSnapshotRepositoryReadsOnlyEligibleCameraCandidates(t *testing.T) {
	content, err := os.ReadFile("mysql_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(content))
	for _, required := range []string{
		"tb_crm_iot_device", "d.category = 'camera'",
		"d.provider = 'hikvisionnvrchannel'", "d.status = 1", "d.deleted_at is null",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("repository source missing %q", required)
		}
	}
	for _, banned := range []string{"insert into", "update ", "delete from", "create table", "tb_nvr_camera_snapshots", "get_lock"} {
		if strings.Contains(source, banned) {
			t.Fatalf("repository source contains forbidden mutation %q", banned)
		}
	}
}
