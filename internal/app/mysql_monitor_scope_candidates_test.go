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
			var crmRow []driver.Value
			if tc.crmName != nil || tc.crmCity != nil {
				crmRow = []driver.Value{int64(10030), tc.crmName, tc.crmCity}
			}
			db := newSessionScriptDB(t,
				sessionSQLStep{
					op: "query", contains: "from tb_stores",
					rejects: []string{"join ", "insert ", "update ", "delete ", "tb_user_resource_scopes", "tb_crm_iot_device"},
					row:     []driver.Value{int64(25), "北京", "北京保利实验室门店", "10030"},
				},
				sessionSQLStep{
					op: "query", contains: "from tb_crm_admin_tenant",
					wants: []string{"where id in (?)"}, args: []any{int64(10030)},
					rejects: []string{"join ", "insert ", "update ", "delete "}, row: crmRow,
				},
			)
			got, err := NewMySQLStore(db).ListMonitorStoreScopeCandidates(context.Background())
			want := []AuthUserResourceScope{{StoreID: 25, City: tc.wantCity, Name: tc.wantName, ExternalOrgID: "10030"}}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("candidates=%+v err=%v, want=%+v", got, err, want)
			}
		})
	}
}

func TestMySQLMonitorCandidatesCRMReadFailure(t *testing.T) {
	want := errors.New("CRM directory unavailable")
	db := newSessionScriptDB(t,
		sessionSQLStep{op: "query", contains: "from tb_stores", row: []driver.Value{int64(19), "南京", "旧名称", "10053"}},
		sessionSQLStep{op: "query", contains: "from tb_crm_admin_tenant", err: want},
	)
	got, err := NewMySQLStore(db).ListMonitorStoreScopeCandidates(context.Background())
	if !errors.Is(err, want) || got != nil {
		t.Fatalf("candidates=%+v err=%v", got, err)
	}
}

func TestMySQLMonitorCandidatesKeepNoncanonicalIdentity(t *testing.T) {
	for _, key := range []string{"legacy-store", "010030"} {
		db := newSessionScriptDB(t, sessionSQLStep{
			op: "query", contains: "from tb_stores",
			row: []driver.Value{int64(25), "北京", "旧名称", key},
		})
		got, err := NewMySQLStore(db).ListMonitorStoreScopeCandidates(context.Background())
		if err != nil || len(got) != 1 || got[0].ExternalOrgID != key || got[0].Name != "旧名称" {
			t.Fatalf("key=%s candidates=%+v err=%v", key, got, err)
		}
	}
}

func TestMySQLMonitorCandidatesEmptyDirectory(t *testing.T) {
	db := newSessionScriptDB(t, sessionSQLStep{op: "query", contains: "from tb_stores"})
	got, err := NewMySQLStore(db).ListMonitorStoreScopeCandidates(context.Background())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("candidates=%+v err=%v", got, err)
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
