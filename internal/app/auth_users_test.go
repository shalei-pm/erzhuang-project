package app

import (
	"reflect"
	"testing"
)

func TestAuthUserPermissionsIncludeAuditViewForAdminOnly(t *testing.T) {
	tests := []struct {
		role          string
		want          []string
		wantAuditView bool
	}{
		{role: RoleAdmin, want: []string{RoleAdmin, PermissionStoreRead, PermissionStoreWrite, PermissionUserManage, PermissionAuditView}, wantAuditView: true},
		{role: RoleEditor, want: []string{RoleEditor, PermissionStoreRead, PermissionStoreWrite}},
		{role: RoleViewer, want: []string{RoleViewer, PermissionStoreRead}},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := (AuthUserRecord{Role: tt.role}).permissions()
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("permissions()=%v, want %v", got, tt.want)
			}
			if hasPermission(got, PermissionAuditView) != tt.wantAuditView {
				t.Fatalf("permissions() contains %q = %v, want %v", PermissionAuditView, hasPermission(got, PermissionAuditView), tt.wantAuditView)
			}
		})
	}
}
