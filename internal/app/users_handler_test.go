package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestNewUserMutationAuditEventIncludesTargetAndPermissionState(t *testing.T) {
	request := httptest.NewRequest("PUT", "/api/users/42", nil)
	event := newUserMutationAuditEvent(request, AuthUserRecord{
		ID:          7,
		Email:       "operator@soyoung.com",
		DisplayName: "操作人",
	}, "user.update", "success", 42, AuthUserMutation{
		Email:                "target@soyoung.com",
		Username:             "target",
		DisplayName:          "目标用户",
		Role:                 RoleViewer,
		Enabled:              true,
		MonitorStoreScopeIDs: []int64{10001, 10030},
	}, 2)

	var detail map[string]any
	if err := json.Unmarshal(event.DetailJSON, &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	wantSummary := "将用户“目标用户（target@soyoung.com）”权限更新为：角色=普通查看，状态=启用，门店范围=2家（门店ID=10001,10030）"
	if detail["summary"] != wantSummary {
		t.Fatalf("summary = %q, want %q", detail["summary"], wantSummary)
	}
	if detail["target_name"] != "目标用户" || detail["target_email"] != "target@soyoung.com" || detail["role"] != RoleViewer || detail["enabled"] != true || detail["scope_ids"] != "10001,10030" {
		t.Fatalf("unexpected target detail: %#v", detail)
	}
}

func TestEnrichUserMutationAuditTargetUsesStoredEmail(t *testing.T) {
	store := NewMemoryStore()
	target, err := store.GetAuthUserByEmail(context.Background(), "maming@soyoung.com")
	if err != nil {
		t.Fatalf("get seeded target: %v", err)
	}
	h := &Handler{store: store}
	input := h.enrichUserMutationAuditTarget(nil, target.ID, AuthUserMutation{
		DisplayName: "更新后的名称",
		Role:        RoleEditor,
		Enabled:     false,
	})
	if input.Email != target.Email {
		t.Fatalf("target email = %q, want stored email %q", input.Email, target.Email)
	}
}

func TestAuditLogSummaryUsesSafeDetailSummary(t *testing.T) {
	log := AuditLog{
		Action:     "user.update",
		DetailJSON: json.RawMessage(`{"summary":"将用户“目标用户（target@soyoung.com）”权限更新为：角色=管理员，状态=启用，门店范围=全部门店"}`),
	}
	want := "将用户“目标用户（target@soyoung.com）”权限更新为：角色=管理员，状态=启用，门店范围=全部门店"
	if got := auditLogSummary(log); got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestAuditLogSummaryFallsBackWhenDetailSummaryIsSensitive(t *testing.T) {
	log := AuditLog{
		Action:     "user.update",
		DetailJSON: json.RawMessage(`{"summary":"token=secret-value"}`),
	}
	if got := auditLogSummary(log); got != "Audit event: user.update" {
		t.Fatalf("summary = %q, want generic fallback", got)
	}
}

func TestAuditLogSummaryFallsBackForLegacyActionSummary(t *testing.T) {
	log := AuditLog{
		Action:     "user.update",
		DetailJSON: json.RawMessage(`{"summary":"user.update"}`),
	}
	if got := auditLogSummary(log); got != "Audit event: user.update" {
		t.Fatalf("summary = %q, want generic fallback", got)
	}
}
