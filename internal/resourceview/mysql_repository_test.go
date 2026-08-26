package resourceview

import (
	"os"
	"strings"
	"testing"
)

func TestMySQLRepositoryIsReadOnlyAndBusinessTableScoped(t *testing.T) {
	content, err := os.ReadFile("mysql_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(content))
	for _, banned := range []string{
		" insert ", " update ", " delete ", " replace ",
		"securityvideourl", "content_id",
		"recognize", "design_plan",
	} {
		if strings.Contains(source, banned) {
			t.Fatalf("mysql repository contains banned token %q", banned)
		}
	}
	for _, required := range []string{
		"tb_crm_admin_tenant",
		"tb_crm_iot_device",
		"tb_crm_consulting_room",
		"tb_crm_iot_area_device_relation",
		"category = 'edge'",
		"left join tb_crm_iot_device d",
		"d.category = 'camera'",
		"provider = 'hikvisionnvrchannel'",
		"d.status = 1",
		"tb_stores",
		"tb_video_recorders",
		"tb_video_channels",
		"tb_channel_snapshots",
		"only_recorder",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("mysql repository missing required token %q", required)
		}
	}
}

func TestMySQLRepositoryUsesSyncedResourceTableColumnNames(t *testing.T) {
	content, err := os.ReadFile("mysql_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(content)
	for _, required := range []string{
		"ip_addr",
		"last_heartbeat_time",
		"order_num",
		"select distinct parent_id",
		"and parent_id <> 0",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("mysql repository missing synchronized-table column %q", required)
		}
	}
	for _, stale := range []string{
		"name, hardware_id, sn, ip, category",
		"ext_params, heartbeat_at",
		"dict_id, sort_order",
		"sort_order asc",
	} {
		if strings.Contains(source, stale) {
			t.Fatalf("mysql repository uses stale business-table column pattern %q", stale)
		}
	}
}

func TestMySQLRepositoryHasDedicatedNVRMonitorEligibilityQuery(t *testing.T) {
	content, err := os.ReadFile("mysql_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(content))
	for _, required := range []string{
		"listnvrmonitorstores",
		"getnvrmonitorstorerecords",
		"listnvrmonitorstorebasesql",
		"t.status = 1",
		"d.category = 'camera'",
		"d.provider = 'hikvisionnvrchannel'",
		"d.status = 1",
		"d.deleted_at is null",
		"order by t.city_id asc, t.id asc",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("mysql repository missing nvr monitor eligibility token %q", required)
		}
	}
}
