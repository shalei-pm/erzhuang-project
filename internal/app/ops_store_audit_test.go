package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPGMySQLStoreAuditEndpointReturnsMissingExternalOrgStores(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setPGMySQLStoreAuditRunnerForTest(func(ctx context.Context) (*pgMySQLStoreAuditResponse, error) {
		return &pgMySQLStoreAuditResponse{
			Summary: pgMySQLStoreAuditSummary{
				PostgresStoreCount:              55,
				PostgresExternalOrgStoreCount:   54,
				PostgresMissingExternalOrgCount: 1,
				MySQLStoreCount:                 54,
			},
			PostgresMissingExternalOrgStores: []storeAuditRow{
				{ID: 99, City: "上海", Name: "未绑定机构门店", ExternalOrgID: "", RecorderCount: 0, ChannelCount: 0},
			},
		}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ops/pg-mysql-store-audit", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response pgMySQLStoreAuditResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || response.Summary.PostgresMissingExternalOrgCount != 1 {
		t.Fatalf("unexpected response: %#v", response)
	}
	if len(response.PostgresMissingExternalOrgStores) != 1 || response.PostgresMissingExternalOrgStores[0].Name != "未绑定机构门店" {
		t.Fatalf("unexpected missing stores: %#v", response.PostgresMissingExternalOrgStores)
	}
}

func setPGMySQLStoreAuditRunnerForTest(runner pgMySQLStoreAuditRunner) func() {
	previous := currentPGMySQLStoreAuditRunner
	currentPGMySQLStoreAuditRunner = runner
	return func() {
		currentPGMySQLStoreAuditRunner = previous
	}
}
