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
