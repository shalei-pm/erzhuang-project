package app

import (
	"context"
	"database/sql/driver"
	"errors"
	"reflect"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

func TestCRMScopesReadLegacyExternalIdentityAndRetainUnavailable(t *testing.T) {
	db := newSessionScriptDB(t,
		sessionSQLStep{op: "query", contains: "from tb_user_resource_scopes", rejects: []string{"tb_stores", "join "}, rowset: [][]driver.Value{
			{int64(7), "store", int64(25), "10030"},
			{int64(7), "store", int64(26), "10030"},
			{int64(7), "store", int64(10030), "10086"},
		}},
		sessionSQLStep{op: "query", contains: "from tb_crm_admin_tenant", row: []driver.Value{int64(10030), "CRM名称", int64(162), int64(1)}},
	)
	got, err := NewMySQLStore(db).GetUserMonitorStoreScopes(context.Background(), 7)
	if err != nil || len(got) != 2 {
		t.Fatalf("scopes=%+v err=%v", got, err)
	}
	byID := map[int64]AuthUserResourceScope{}
	for _, scope := range got {
		byID[scope.StoreID] = scope
	}
	if byID[10030].Name != "CRM名称" || byID[10030].Unavailable || !byID[10086].Unavailable {
		t.Fatalf("scopes=%+v", got)
	}
}

func TestCRMScopesRejectInconsistentAndNoncanonicalIdentity(t *testing.T) {
	for _, row := range [][]driver.Value{
		{int64(7), "store", int64(25), "010030"},
		{int64(7), "store", int64(25), " 10030"},
		{int64(7), "crm_institution", int64(25), "10030"},
	} {
		db := newSessionScriptDB(t, sessionSQLStep{op: "query", contains: "from tb_user_resource_scopes", row: row})
		if _, err := NewMySQLStore(db).GetUserMonitorStoreScopes(context.Background(), 7); err == nil {
			t.Fatal("invalid identity accepted")
		}
	}
}

func TestCRMScopesReplaceBothNamespacesAndRetainExistingMissingInstitution(t *testing.T) {
	db := newSessionScriptDB(t,
		sessionSQLStep{op: "begin"},
		sessionSQLStep{op: "query", contains: "from tb_users", wants: []string{"for update"}, row: []driver.Value{int64(7), true, "viewer"}},
		sessionSQLStep{op: "query", contains: "from tb_user_resource_scopes", row: []driver.Value{int64(7), "store", int64(25), "10030"}},
		sessionSQLStep{op: "query", contains: "from tb_crm_admin_tenant", wants: []string{"status = 1"}, row: []driver.Value{int64(10086), "新机构", int64(162), int64(1)}},
		sessionSQLStep{op: "exec", contains: "delete from tb_user_resource_scopes", wants: []string{"resource_type in (?, ?)"}, args: []any{int64(7), "store", "crm_institution", "monitor:view"}},
		sessionSQLStep{op: "exec", contains: "insert into tb_user_resource_scopes", rejects: []string{"tb_stores", "select"}, args: []any{int64(7), "crm_institution", int64(10030), "10030", "monitor:view"}},
		sessionSQLStep{op: "exec", contains: "insert into tb_user_resource_scopes", args: []any{int64(7), "crm_institution", int64(10086), "10086", "monitor:view"}},
		sessionSQLStep{op: "commit"},
	)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	err = setMySQLUserMonitorScopes(context.Background(), tx, 7, RoleViewer, []int64{10030, 10086, 10086})
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestCRMScopesFailureRollsBackWithoutDeletingGrants(t *testing.T) {
	for _, tc := range []struct {
		name     string
		selected []int64
		crmErr   error
	}{
		{"source error", []int64{}, errors.New("CRM unavailable")},
		{"new inactive institution", []int64{10086}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newSessionScriptDB(t,
				sessionSQLStep{op: "begin"},
				sessionSQLStep{op: "query", contains: "from tb_users", row: []driver.Value{int64(7), true, "viewer"}},
				sessionSQLStep{op: "query", contains: "from tb_user_resource_scopes"},
				sessionSQLStep{op: "query", contains: "from tb_crm_admin_tenant", err: tc.crmErr},
				sessionSQLStep{op: "rollback"},
			)
			tx, _ := db.BeginTx(context.Background(), nil)
			if err := setMySQLUserMonitorScopes(context.Background(), tx, 7, RoleViewer, tc.selected); err == nil {
				t.Fatal("unsafe save accepted")
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCRMScopesExplicitEmptyRevokesBothNamespacesForEveryRole(t *testing.T) {
	for _, role := range []string{RoleViewer, RoleAdmin, RoleEditor} {
		db := newSessionScriptDB(t,
			sessionSQLStep{op: "begin"},
			sessionSQLStep{op: "query", contains: "from tb_users", row: []driver.Value{int64(7), true, "viewer"}},
			sessionSQLStep{op: "query", contains: "from tb_user_resource_scopes", row: []driver.Value{int64(7), "store", int64(25), "10030"}},
			sessionSQLStep{op: "query", contains: "from tb_crm_admin_tenant"},
			sessionSQLStep{op: "exec", contains: "delete from tb_user_resource_scopes", wants: []string{"resource_type in (?, ?)"}},
			sessionSQLStep{op: "commit"},
		)
		tx, _ := db.BeginTx(context.Background(), nil)
		if err := setMySQLUserMonitorScopes(context.Background(), tx, 7, role, []int64{}); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCRMScopesOmittedPreservesExistingSelection(t *testing.T) {
	db := newSessionScriptDB(t, sessionSQLStep{op: "begin"}, sessionSQLStep{op: "commit"})
	tx, _ := db.BeginTx(context.Background(), nil)
	if err := setMySQLUserMonitorScopes(context.Background(), tx, 7, RoleViewer, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestCRMScopesPositiveDeduplication(t *testing.T) {
	if got := uniquePositiveInt64s([]int64{10086, 10030, 10086}); !reflect.DeepEqual(got, []int64{10086, 10030}) {
		t.Fatal(got)
	}
}

func TestCRMScopesStaleRevisionCannotRestoreRevokedGrant(t *testing.T) {
	db := newSessionScriptDB(t,
		sessionSQLStep{op: "begin"},
		sessionSQLStep{op: "query", contains: "from tb_users", wants: []string{"for update"}, row: []driver.Value{int64(7), true, "viewer"}},
		sessionSQLStep{op: "query", contains: "from tb_user_resource_scopes", wants: []string{"for update"}},
		sessionSQLStep{op: "rollback"},
	)
	tx, _ := db.BeginTx(context.Background(), nil)
	oldRevision := userMonitorScopeRevision([]string{"store:25:10030"}, RoleViewer, true)
	if err := setMySQLUserMonitorScopes(context.Background(), tx, 7, RoleViewer, []int64{10030}, oldRevision); !errors.Is(err, errMonitorScopeConflict) {
		t.Fatalf("stale edit accepted: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
}

func TestCRMUserUpdateRejectsStaleRevisionEvenWhenScopesOmitted(t *testing.T) {
	for _, selected := range [][]int64{nil, {10030}} {
		db := newSessionScriptDB(t,
			sessionSQLStep{op: "begin"},
			sessionSQLStep{op: "query", contains: "from tb_users", wants: []string{"for update"}, row: []driver.Value{int64(7), false, "viewer"}},
			sessionSQLStep{op: "query", contains: "from tb_user_resource_scopes", wants: []string{"for update"}},
			sessionSQLStep{op: "rollback"},
		)
		_, err := NewMySQLStore(db).UpdateAuthUser(context.Background(), 7, AuthUserMutation{Role: RoleAdmin, Enabled: true, MonitorInstitutionIDs: selected, MonitorScopeRevision: userMonitorScopeRevision(nil, RoleAdmin, true)})
		if !errors.Is(err, errMonitorScopeConflict) {
			t.Fatalf("stale role/enable update accepted: %v", err)
		}
	}
}

func TestCRMScopesInsertFailureRollsBackReplacement(t *testing.T) {
	db := newSessionScriptDB(t,
		sessionSQLStep{op: "begin"},
		sessionSQLStep{op: "query", contains: "from tb_users", row: []driver.Value{int64(7), true, "viewer"}},
		sessionSQLStep{op: "query", contains: "from tb_user_resource_scopes", row: []driver.Value{int64(7), "store", int64(25), "10030"}},
		sessionSQLStep{op: "query", contains: "from tb_crm_admin_tenant", row: []driver.Value{int64(10086), "新机构", int64(162), int64(1)}},
		sessionSQLStep{op: "exec", contains: "delete from tb_user_resource_scopes"},
		sessionSQLStep{op: "exec", contains: "insert into tb_user_resource_scopes"},
		sessionSQLStep{op: "exec", contains: "insert into tb_user_resource_scopes", err: errors.New("insert unavailable")},
		sessionSQLStep{op: "rollback"},
	)
	tx, _ := db.BeginTx(context.Background(), nil)
	if err := setMySQLUserMonitorScopes(context.Background(), tx, 7, RoleViewer, []int64{10030, 10086}); err == nil {
		t.Fatal("partial replacement reported success")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
}

func TestCRMScopesMixedIdentityFailsClosed(t *testing.T) {
	db := newSessionScriptDB(t, sessionSQLStep{op: "query", contains: "from tb_user_resource_scopes", rowset: [][]driver.Value{
		{int64(7), "store", int64(25), "10030"},
		{int64(7), "crm_institution", int64(10086), "10086"},
	}})
	if _, err := NewMySQLStore(db).GetUserMonitorStoreScopes(context.Background(), 7); err == nil {
		t.Fatal("mixed identity accepted")
	}
}

func TestCRMInstitutionIDBoundary(t *testing.T) {
	for _, key := range []string{"0", "-1", "+1", "01", "1 ", "9007199254740992", "9223372036854775808"} {
		if _, valid := canonicalInstitutionID(key); valid {
			t.Fatalf("invalid ID %q accepted", key)
		}
	}
	if _, valid := canonicalInstitutionID("9007199254740991"); !valid {
		t.Fatal("maximum safe ID rejected")
	}
}

func TestCRMUserMutationAuditFailureAndCommitUncertainty(t *testing.T) {
	for _, stage := range []string{"audit", "commit"} {
		t.Run(stage, func(t *testing.T) {
			steps := []sessionSQLStep{
				{op: "begin"},
				{op: "query", contains: "from tb_users", wants: []string{"for update"}, row: []driver.Value{int64(7), true, "viewer"}},
				{op: "query", contains: "from tb_user_resource_scopes", wants: []string{"for update"}},
				{op: "exec", contains: "update tb_users", affected: 1},
				{op: "exec", contains: "insert ignore into tb_roles"},
				{op: "exec", contains: "delete from tb_user_roles"},
				{op: "exec", contains: "insert ignore into tb_user_roles"},
				{op: "query", contains: "from tb_users", row: []driver.Value{int64(7), "viewer@example.test", "viewer", "验收用户", "", "", "viewer", true, nil}},
				{op: "query", contains: "from tb_user_resource_scopes", wants: []string{"for update"}},
			}
			failure := errors.New(stage + " unavailable")
			if stage == "audit" {
				steps = append(steps, sessionSQLStep{op: "exec", contains: "insert into tb_audit_logs", err: failure}, sessionSQLStep{op: "rollback"})
			} else {
				steps = append(steps, sessionSQLStep{op: "exec", contains: "insert into tb_audit_logs"}, sessionSQLStep{op: "commit", err: failure})
			}
			db := newSessionScriptDB(t, steps...)
			_, err := NewMySQLStore(db).UpdateAuthUserWithAudit(context.Background(), 7, AuthUserMutation{Role: RoleViewer, Enabled: true, MonitorScopeRevision: userMonitorScopeRevision(nil, RoleViewer, true)}, auditlog.AuditEvent{Action: "user.update", Result: "success"})
			if stage == "commit" {
				if !errors.Is(err, errAuthUserCommitUnknown) {
					t.Fatalf("commit uncertainty not classified: %v", err)
				}
			} else if !errors.Is(err, failure) {
				t.Fatalf("audit error lost: %v", err)
			}
		})
	}
}
