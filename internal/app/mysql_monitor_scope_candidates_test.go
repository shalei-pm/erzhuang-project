package app

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
)

func TestMySQLMonitorCandidatesUseCRMWithoutLegacyDirectory(t *testing.T) {
	for _, tc := range []struct {
		name     string
		crmName  driver.Value
		crmCity  driver.Value
		wantName string
		wantCity string
	}{
		{"CRM renamed and moved", "数字化测试门店", int64(162), "数字化测试门店", "南京"},
		{"unknown city remains identifiable", "数字化测试门店", int64(999999), "数字化测试门店", "城市 999999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newSessionScriptDB(t,
				sessionSQLStep{
					op: "query", contains: "from tb_crm_admin_tenant",
					wants:   []string{"status = 1"},
					rejects: []string{"tb_stores", "join ", "insert ", "update ", "delete ", "tb_crm_iot_device"},
					row:     []driver.Value{int64(10030), tc.crmName, tc.crmCity, int64(1)},
				},
			)
			got, err := NewMySQLStore(db).ListMonitorStoreScopeCandidates(context.Background())
			if err != nil || len(got) != 1 || got[0].StoreID != 10030 || got[0].City != tc.wantCity || got[0].Name != tc.wantName || got[0].ExternalOrgID != "10030" {
				t.Fatalf("candidates=%+v err=%v", got, err)
			}
		})
	}
}

func TestMySQLMonitorCandidatesCRMReadFailure(t *testing.T) {
	want := errors.New("CRM directory unavailable")
	db := newSessionScriptDB(t,
		sessionSQLStep{op: "query", contains: "from tb_crm_admin_tenant", err: want},
	)
	got, err := NewMySQLStore(db).ListMonitorStoreScopeCandidates(context.Background())
	if !errors.Is(err, want) || got != nil {
		t.Fatalf("candidates=%+v err=%v", got, err)
	}
}

func TestMySQLMonitorCandidatesEmptyDirectory(t *testing.T) {
	db := newSessionScriptDB(t, sessionSQLStep{op: "query", contains: "from tb_crm_admin_tenant"})
	got, err := NewMySQLStore(db).ListMonitorStoreScopeCandidates(context.Background())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("candidates=%+v err=%v", got, err)
	}
}

func TestMySQLMonitorAuthorizationUsesTypedCRMIdentity(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		step := sessionSQLStep{op: "query", contains: "from tb_user_resource_scopes", wants: []string{"resource_type", "resource_id", "binary external_key", "not exists", "tb_crm_admin_tenant", "t.status = 1"}, rejects: []string{"tb_stores", "join "}, args: []any{int64(7), "monitor:view", "10086", "store", "crm_institution", int64(10086), int64(10086)}}
		if allowed {
			step.row = []driver.Value{int64(1)}
		}
		db := newSessionScriptDB(t, step)
		got, err := NewMySQLStore(db).CanUserViewMonitorStore(context.Background(), AuthUserRecord{ID: 7, Role: RoleViewer}, "10086")
		if err != nil || got != allowed {
			t.Fatalf("allowed=%v err=%v", got, err)
		}
	}
}

func TestMySQLMonitorPrivilegedRolesRequireEnabledCRMInstitution(t *testing.T) {
	for _, role := range []string{RoleAdmin, RoleEditor} {
		for _, active := range []bool{true, false} {
			step := sessionSQLStep{op: "query", contains: "from tb_crm_admin_tenant", wants: []string{"status = 1"}, args: []any{int64(10086)}}
			if active {
				step.row = []driver.Value{int64(1)}
			}
			db := newSessionScriptDB(t, step)
			allowed, err := NewMySQLStore(db).CanUserViewMonitorStore(context.Background(), AuthUserRecord{ID: 7, Role: role}, "10086")
			if err != nil || allowed != active {
				t.Fatalf("role=%s active=%v allowed=%v err=%v", role, active, allowed, err)
			}
		}
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
