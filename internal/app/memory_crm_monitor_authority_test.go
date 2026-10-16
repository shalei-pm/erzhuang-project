package app

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/h5monitor"
	"github.com/shalei-pm/erzhuang-project/internal/nvrmonitor"
)

func TestMemoryCRMMonitorAuthorityRejectsStaleNilUpdates(t *testing.T) {
	ctx := context.Background()
	for _, change := range []string{"scope", "role", "enabled"} {
		t.Run(change, func(t *testing.T) {
			s := NewMemoryStore()
			original, err := s.CreateAuthUser(ctx, AuthUserMutation{Email: "revision@example.test", Role: RoleAdmin, Enabled: true, MonitorInstitutionIDs: []int64{10030}})
			if err != nil {
				t.Fatal(err)
			}
			mutation := AuthUserMutation{Role: original.Role, Enabled: original.Enabled, MonitorScopeRevision: original.MonitorScopeRevision}
			switch change {
			case "scope":
				mutation.MonitorInstitutionIDs = []int64{}
			case "role":
				mutation.Role = RoleViewer
			case "enabled":
				mutation.Enabled = false
			}
			current, err := s.UpdateAuthUser(ctx, original.ID, mutation)
			if err != nil {
				t.Fatal(err)
			}
			if current.MonitorScopeRevision == original.MonitorScopeRevision {
				t.Fatal("authority change did not advance revision")
			}
			for _, revision := range []string{original.MonitorScopeRevision, ""} {
				_, err = s.UpdateAuthUser(ctx, original.ID, AuthUserMutation{Username: "stale", Role: original.Role, Enabled: original.Enabled, MonitorScopeRevision: revision})
				if !errors.Is(err, errMonitorScopeConflict) {
					t.Fatalf("nil update with revision %q must conflict, got %v", revision, err)
				}
			}
			stored, err := s.GetAuthUserByID(ctx, original.ID)
			if err != nil || stored.Role != current.Role || stored.Enabled != current.Enabled || stored.Username != current.Username || stored.MonitorScopeRevision != current.MonitorScopeRevision {
				t.Fatalf("rejected update changed authority: %+v, %v", stored, err)
			}
			updated, err := s.UpdateAuthUser(ctx, original.ID, AuthUserMutation{Username: "fresh", Role: current.Role, Enabled: current.Enabled, MonitorScopeRevision: current.MonitorScopeRevision})
			if err != nil || updated.Username != "fresh" || len(updated.MonitorStoreScopes) != len(current.MonitorStoreScopes) {
				t.Fatalf("fresh nil update must preserve scopes: %+v, %v", updated, err)
			}
		})
	}
}

func TestMemoryCRMMonitorAuthoritySeededUserRequiresRevision(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	if _, err := s.UpdateAuthUser(ctx, 1, AuthUserMutation{Role: RoleViewer, Enabled: false}); !errors.Is(err, errMonitorScopeConflict) {
		t.Fatalf("seeded user accepted missing revision: %v", err)
	}
	user, err := s.GetAuthUserByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateAuthUser(ctx, user.ID, AuthUserMutation{Role: user.Role, Enabled: user.Enabled, MonitorScopeRevision: user.MonitorScopeRevision}); err != nil {
		t.Fatalf("fresh seeded revision rejected: %v", err)
	}
}

func TestMemoryCRMMonitorAuthorityRequiresCurrentEnabledCRM(t *testing.T) {
	ctx := context.Background()
	for _, role := range []string{RoleAdmin, RoleEditor, RoleViewer} {
		for _, state := range []string{"enabled", "disabled", "missing"} {
			t.Run(role+"/"+state, func(t *testing.T) {
				s := NewMemoryStore()
				user, err := s.CreateAuthUser(ctx, AuthUserMutation{Email: "gate@example.test", Role: role, Enabled: true, MonitorInstitutionIDs: []int64{10030}})
				if err != nil {
					t.Fatal(err)
				}
				s.mu.Lock()
				if state == "disabled" {
					s.monitorScopeCandidates[0].Unavailable = true
				} else if state == "missing" {
					s.monitorScopeCandidates = s.monitorScopeCandidates[1:]
				}
				s.mu.Unlock()
				allowed, err := s.CanUserViewMonitorStore(ctx, user, "10030")
				if err != nil || allowed != (state == "enabled") {
					t.Fatalf("current CRM gate: allowed=%v err=%v", allowed, err)
				}
				for _, orgID := range []string{"", "99999", "010030", "+10030", "9007199254740992"} {
					if allowed, err := s.CanUserViewMonitorStore(ctx, user, orgID); err != nil || allowed {
						t.Fatalf("invalid/missing CRM %q allowed=%v err=%v", orgID, allowed, err)
					}
				}
				allowed, err = s.CanUserViewMonitorStore(ctx, user, "10001")
				if err != nil || allowed != (role != RoleViewer) {
					t.Fatalf("synthetic CRM still requires viewer scope: allowed=%v err=%v", allowed, err)
				}
			})
		}
	}
}

func TestMemoryCRMMonitorAuthorityUnavailableScopesCanBeRetainedAndRevoked(t *testing.T) {
	ctx := context.Background()
	for _, state := range []string{"disabled", "missing"} {
		t.Run(state, func(t *testing.T) {
			s := NewMemoryStore()
			user, err := s.CreateAuthUser(ctx, AuthUserMutation{Email: "retain@example.test", Role: RoleViewer, Enabled: true, MonitorInstitutionIDs: []int64{10030}})
			if err != nil {
				t.Fatal(err)
			}
			s.mu.Lock()
			if state == "disabled" {
				s.monitorScopeCandidates[0].Unavailable = true
			} else {
				s.monitorScopeCandidates = s.monitorScopeCandidates[1:]
			}
			s.mu.Unlock()
			for _, ids := range [][]int64{nil, {10030}, {}} {
				user, err = s.UpdateAuthUser(ctx, user.ID, AuthUserMutation{Role: user.Role, Enabled: user.Enabled, MonitorInstitutionIDs: ids, MonitorScopeRevision: user.MonitorScopeRevision})
				if err != nil {
					t.Fatal(err)
				}
				wantCount := 1
				if ids != nil && len(ids) == 0 {
					wantCount = 0
				}
				if len(user.MonitorStoreScopes) != wantCount {
					t.Fatalf("scope retention/revocation failed: %+v", user.MonitorStoreScopes)
				}
				if allowed, err := s.CanUserViewMonitorStore(ctx, user, "10030"); err != nil || allowed {
					t.Fatalf("unavailable scope granted access: %v, %v", allowed, err)
				}
			}
			if _, err := s.UpdateAuthUser(ctx, user.ID, AuthUserMutation{Role: user.Role, Enabled: user.Enabled, MonitorInstitutionIDs: []int64{10030}, MonitorScopeRevision: user.MonitorScopeRevision}); err == nil {
				t.Fatal("revoked unavailable institution was regranted")
			}
		})
	}
}

func TestCRMMonitorAuthorityFiltersApplyToEveryRole(t *testing.T) {
	for _, role := range []string{RoleAdmin, RoleEditor, RoleViewer} {
		t.Run(role, func(t *testing.T) {
			s := NewMemoryStore()
			user, err := s.CreateAuthUser(context.Background(), AuthUserMutation{Email: "lists@example.test", Role: role, Enabled: true, MonitorInstitutionIDs: []int64{10030, 10019}})
			if err != nil {
				t.Fatal(err)
			}
			s.mu.Lock()
			s.monitorScopeCandidates[1].Unavailable = true
			s.mu.Unlock()
			h := &Handler{store: s}
			r := httptest.NewRequest("GET", "/", nil)
			r = r.WithContext(context.WithValue(r.Context(), authContextKey{}, authenticatedAuthContext{record: user}))
			nvr, err := (nvrMonitorAuthorizer{handler: h}).FilterStores(r, nvrmonitor.MonitorStoresResponse{Cities: []nvrmonitor.StoreCityGroup{
				{City: "active", Stores: []nvrmonitor.StoreInfo{{ExternalOrgID: "10030"}, {ExternalOrgID: "10019"}}},
				{City: "missing", Stores: []nvrmonitor.StoreInfo{{ExternalOrgID: "99999"}}},
			}})
			if err != nil || len(nvr.Cities) != 1 || len(nvr.Cities[0].Stores) != 1 || nvr.Cities[0].Stores[0].ExternalOrgID != "10030" {
				t.Errorf("NVR list bypassed CRM gate: %+v, %v", nvr, err)
			}
			h5, err := (h5MonitorAuthorizer{handler: h}).FilterMonitorStores(r, h5monitor.MonitorStoresResponse{Cities: []h5monitor.MonitorStoreCityGroup{
				{City: "active", Stores: []h5monitor.MonitorStoreInfo{{ExternalOrgID: "10030"}, {ExternalOrgID: "10019"}}},
				{City: "missing", Stores: []h5monitor.MonitorStoreInfo{{ExternalOrgID: "99999"}}},
			}})
			if err != nil || len(h5.Cities) != 1 || len(h5.Cities[0].Stores) != 1 || h5.Cities[0].Stores[0].ExternalOrgID != "10030" {
				t.Errorf("H5 list bypassed CRM gate: %+v, %v", h5, err)
			}
		})
	}
}
