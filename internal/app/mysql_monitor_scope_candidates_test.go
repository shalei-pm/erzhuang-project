package app

import (
	"context"
	"database/sql/driver"
	"errors"
	"reflect"
	"testing"
)

func TestMySQLMonitorCandidatesUseCRMMetaWithoutChangingIdentity(t *testing.T) {
	for _, tc := range []struct {
		name     string
		crmName  driver.Value
		crmCity  driver.Value
		wantName string
		wantCity string
	}{
		{"CRM renamed and moved", "数字化测试门店", int64(162), "数字化测试门店", "南京"},
		{"legacy without CRM", nil, nil, "北京保利实验室门店", "北京"},
		{"blank CRM metadata", " ", int64(0), "北京保利实验室门店", "北京"},
		{"unknown CRM city keeps legacy city", "数字化测试门店", int64(999999), "数字化测试门店", "北京"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newSessionScriptDB(t, sessionSQLStep{
				op: "query", contains: "from tb_stores s",
				wants:   []string{"left join tb_crm_admin_tenant", "binary trim(s.external_org_id) = binary cast(t.id as char)"},
				rejects: []string{"insert ", "update ", "delete ", "tb_user_resource_scopes", "tb_crm_iot_device"},
				row:     []driver.Value{int64(25), "北京", "北京保利实验室门店", "10030", tc.crmName, tc.crmCity},
			})
			got, err := NewMySQLStore(db).ListMonitorStoreScopeCandidates(context.Background())
			want := []AuthUserResourceScope{{StoreID: 25, City: tc.wantCity, Name: tc.wantName, ExternalOrgID: "10030"}}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("candidates=%+v err=%v, want=%+v", got, err, want)
			}
		})
	}
}

func TestMySQLMonitorCandidatesReadFailure(t *testing.T) {
	want := errors.New("candidate database unavailable")
	db := newSessionScriptDB(t, sessionSQLStep{op: "query", err: want})
	got, err := NewMySQLStore(db).ListMonitorStoreScopeCandidates(context.Background())
	if !errors.Is(err, want) || got != nil {
		t.Fatalf("candidates=%+v err=%v", got, err)
	}
}
