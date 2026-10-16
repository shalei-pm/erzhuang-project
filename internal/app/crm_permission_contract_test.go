package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

func TestCRMContractRejectsEveryLegacyScopePayload(t *testing.T) {
	for _, value := range []string{"null", "[]", "[25]"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"email":"viewer@example.test","role":"viewer","monitor_store_scope_ids":`+value+`}`))
		var input AuthUserMutation
		if decodeAuthUserMutation(w, r, &input, true) || w.Code != http.StatusConflict {
			t.Fatalf("body=%s status=%d", value, w.Code)
		}
	}
}

type unknownCommitUserStore struct {
	*MemoryStore
	failureAudits int
}

func (s *unknownCommitUserStore) CreateAuthUserWithAudit(context.Context, AuthUserMutation, auditlog.AuditEvent) (AuthUserRecord, error) {
	return AuthUserRecord{}, errAuthUserCommitUnknown
}

func (s *unknownCommitUserStore) UpdateAuthUserWithAudit(context.Context, int64, AuthUserMutation, auditlog.AuditEvent) (AuthUserRecord, error) {
	return AuthUserRecord{}, errAuthUserCommitUnknown
}

func (s *unknownCommitUserStore) RecordAudit(context.Context, auditlog.AuditEvent) error {
	s.failureAudits++
	return nil
}

func TestCRMUserCommitUnknownReturns503WithoutFailureAudit(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		s := &unknownCommitUserStore{MemoryStore: NewMemoryStore()}
		h := &Handler{store: s, auditRecorder: s}
		r := httptest.NewRequest(method, "/api/users/7", strings.NewReader(`{"email":"viewer@example.test","role":"viewer","enabled":true,"monitor_scope_revision":"revision"}`))
		r.SetPathValue("id", "7")
		r = r.WithContext(context.WithValue(r.Context(), authContextKey{}, authenticatedAuthContext{record: AuthUserRecord{ID: 1, Role: RoleAdmin, Enabled: true}}))
		w := httptest.NewRecorder()
		if method == http.MethodPost {
			h.createUserHandler(w, r)
		} else {
			h.updateUserHandler(w, r)
		}
		if w.Code != http.StatusServiceUnavailable || s.failureAudits != 0 || !strings.Contains(w.Body.String(), "勿直接重复保存") {
			t.Fatalf("method=%s status=%d audits=%d body=%s", method, w.Code, s.failureAudits, w.Body.String())
		}
	}
}

func TestMemoryCRMScopesUseCRMIDsAndRejectUnknown(t *testing.T) {
	s := NewMemoryStore()
	user, err := s.CreateAuthUser(context.Background(), AuthUserMutation{Email: "crm@example.test", Role: RoleViewer, Enabled: true, MonitorInstitutionIDs: []int64{10030}})
	if err != nil || len(user.MonitorStoreScopes) != 1 || user.MonitorStoreScopes[0].StoreID != 10030 {
		t.Fatalf("user=%+v err=%v", user, err)
	}
	user, err = s.UpdateAuthUser(context.Background(), user.ID, AuthUserMutation{Role: RoleViewer, Enabled: true, MonitorScopeRevision: user.MonitorScopeRevision})
	if err != nil || len(user.MonitorStoreScopes) != 1 {
		t.Fatalf("omitted scopes lost: %+v %v", user, err)
	}
	if _, err := s.UpdateAuthUser(context.Background(), user.ID, AuthUserMutation{Role: RoleViewer, Enabled: true, MonitorInstitutionIDs: []int64{30}, MonitorScopeRevision: user.MonitorScopeRevision}); err == nil {
		t.Fatal("legacy internal ID accepted")
	}
	previousRevision := user.MonitorScopeRevision
	user, err = s.UpdateAuthUser(context.Background(), user.ID, AuthUserMutation{Role: RoleViewer, Enabled: true, MonitorInstitutionIDs: []int64{}, MonitorScopeRevision: previousRevision})
	if err != nil || len(user.MonitorStoreScopes) != 0 {
		t.Fatalf("explicit revoke failed: %+v %v", user, err)
	}
	allowed, err := s.CanUserViewMonitorStore(context.Background(), user, "10030")
	if err != nil || allowed {
		t.Fatalf("revoked permission still allowed: %v %v", allowed, err)
	}
	if _, err := s.UpdateAuthUser(context.Background(), user.ID, AuthUserMutation{Role: RoleViewer, Enabled: true, MonitorInstitutionIDs: []int64{10030}, MonitorScopeRevision: previousRevision}); !errors.Is(err, errMonitorScopeConflict) {
		t.Fatalf("stale edit revived permissions: %v", err)
	}
}
