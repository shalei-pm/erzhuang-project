package storespace

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCRMDirectoryRejectsLegacyInstitutionMutations(t *testing.T) {
	service := NewService(NewMemoryStore())
	service.UseCRMInstitutionAuthority()
	mux := http.NewServeMux()
	RegisterRoutes(mux, service)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/store-space/stores", `{"city":"南京","name":"旧目录新店","external_org_id":"10086"}`},
		{http.MethodPatch, "/api/store-space/stores/1", `{"city":"南京","name":"旧目录改名","external_org_id":"10086"}`},
		{http.MethodDelete, "/api/store-space/stores/1", ""},
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "CRM") {
			t.Fatalf("%s %s: status=%d body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}
