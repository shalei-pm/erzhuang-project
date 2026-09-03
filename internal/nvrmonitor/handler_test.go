package nvrmonitor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
)

type fakeAuthorizer struct {
	allow map[string]bool
}

func (a fakeAuthorizer) CanViewStore(_ *http.Request, externalOrgID string) (bool, error) {
	return a.allow[externalOrgID], nil
}

func (a fakeAuthorizer) FilterStores(_ *http.Request, response MonitorStoresResponse) (MonitorStoresResponse, error) {
	filtered := MonitorStoresResponse{}
	for _, city := range response.Cities {
		next := StoreCityGroup{City: city.City}
		for _, store := range city.Stores {
			if a.allow[store.ExternalOrgID] {
				next.Stores = append(next.Stores, store)
			}
		}
		if len(next.Stores) > 0 {
			filtered.Cities = append(filtered.Cities, next)
		}
	}
	return filtered, nil
}

func (a fakeAuthorizer) RecordAudit(_ *http.Request, _ auditlog.AuditEvent) error {
	return nil
}

type fakeSnapshotBackfillAuthorizer struct {
	fakeAuthorizer
	allowBackfill bool
}

type fakeAuditAuthorizer struct {
	fakeAuthorizer
	events []auditlog.AuditEvent
	err    error
	errs   []error
	check  func(auditlog.AuditEvent) error
}

func (a *fakeAuditAuthorizer) RecordAudit(_ *http.Request, event auditlog.AuditEvent) error {
	call := len(a.events)
	a.events = append(a.events, event)
	if call < len(a.errs) {
		return a.errs[call]
	}
	if a.check != nil {
		return a.check(event)
	}
	return a.err
}

type fakeAuditedSnapshotBackfillAuthorizer struct {
	fakeAuditAuthorizer
	allowBackfill bool
}

func (a *fakeAuditedSnapshotBackfillAuthorizer) CanBackfillSnapshot(_ *http.Request, externalOrgID string) (bool, error) {
	return a.allowBackfill && a.allow[externalOrgID], nil
}

func (a fakeSnapshotBackfillAuthorizer) CanBackfillSnapshot(_ *http.Request, externalOrgID string) (bool, error) {
	return a.allowBackfill && a.allow[externalOrgID], nil
}

func newTestHandler(t *testing.T, allow map[string]bool) http.Handler {
	t.Helper()
	service := NewService(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{url: "wss://stream.example.test/session"})
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, fakeAuthorizer{allow: allow})
	return mux
}

func newTestHandlerWithSnapshots(t *testing.T, allow map[string]bool, snapshots SnapshotStore) http.Handler {
	t.Helper()
	service := NewServiceWithSnapshotStore(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{url: "wss://stream.example.test/session"}, snapshots)
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, fakeAuthorizer{allow: allow})
	return mux
}

func newAuditedHandler(t *testing.T, authorizer *fakeAuditAuthorizer, snapshots SnapshotStore) http.Handler {
	t.Helper()
	service := NewServiceWithSnapshotStore(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{url: "wss://stream.example.test/session"}, snapshots)
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, authorizer)
	return mux
}

func TestStoresFiltersToAuthorizedMonitorScope(t *testing.T) {
	handler := newTestHandler(t, map[string]bool{"10001": true})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/stores", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload MonitorStoresResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Cities) != 1 || len(payload.Cities[0].Stores) != 1 || payload.Cities[0].Stores[0].ExternalOrgID != "10001" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestCameraListRejectsUnauthorizedStore(t *testing.T) {
	handler := newTestHandler(t, map[string]bool{"10001": false})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/orgs/10001/cameras", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestStreamSessionSetsNoStoreAndDoesNotExposeServiceCredential(t *testing.T) {
	handler := newTestHandler(t, map[string]bool{"10001": true})
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"live"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if strings.Contains(response.Body.String(), "service-secret") {
		t.Fatalf("response leaked service credential: %s", response.Body.String())
	}
}

func TestStreamSessionRejectsCameraOutsideTenantEligibleSet(t *testing.T) {
	handler := newTestHandler(t, map[string]bool{"10001": true})
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/112/stream-session", strings.NewReader(`{"mode":"live"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestSnapshotServesBackfilledImageOnlyToAuthorizedStore(t *testing.T) {
	handler := newTestHandlerWithSnapshots(t, map[string]bool{"10001": true}, fakeSnapshotStore{
		data: map[int64]string{111: "jpeg-data"},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if response.Body.String() != "jpeg-data" || response.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("unexpected snapshot response: headers=%#v body=%q", response.Header(), response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", response.Header().Get("X-Content-Type-Options"))
	}
}

func TestSnapshotDoesNotLeakToUnauthorizedStore(t *testing.T) {
	handler := newTestHandlerWithSnapshots(t, map[string]bool{"10001": false}, fakeSnapshotStore{
		data: map[int64]string{111: "jpeg-data"},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || response.Body.String() == "jpeg-data" {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
}

func TestSnapshotBackfillWritesOnlyValidatedJPEGForAuthorizedEditor(t *testing.T) {
	snapshots := &fakeWritableSnapshotStore{fakeSnapshotStore: fakeSnapshotStore{data: map[int64]string{}}}
	service := NewServiceWithSnapshotStore(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{}, snapshots)
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, fakeSnapshotBackfillAuthorizer{fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}}, allowBackfill: true})
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00}))
	request.Header.Set("Content-Type", "image/jpeg")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if snapshots.savedCameraID != 111 || snapshots.savedTenantID != 10001 || snapshots.savedContentType != "image/jpeg" {
		t.Fatalf("snapshot write = %#v", snapshots)
	}
}

func TestSnapshotBackfillRejectsViewer(t *testing.T) {
	snapshots := &fakeWritableSnapshotStore{fakeSnapshotStore: fakeSnapshotStore{data: map[int64]string{}}}
	service := NewServiceWithSnapshotStore(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{}, snapshots)
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, fakeSnapshotBackfillAuthorizer{fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}}})
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", strings.NewReader("not-a-jpeg"))
	request.Header.Set("Content-Type", "image/jpeg")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || snapshots.savedCameraID != 0 {
		t.Fatalf("status = %d write = %#v", response.Code, snapshots)
	}
}

func TestSnapshotBackfillRejectsInvalidPayloadBeforeWriting(t *testing.T) {
	snapshots := &fakeWritableSnapshotStore{fakeSnapshotStore: fakeSnapshotStore{data: map[int64]string{}}}
	service := NewServiceWithSnapshotStore(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{}, snapshots)
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, fakeSnapshotBackfillAuthorizer{fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}}, allowBackfill: true})
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", strings.NewReader("not-a-jpeg"))
	request.Header.Set("Content-Type", "image/jpeg")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || snapshots.savedCameraID != 0 {
		t.Fatalf("status = %d write = %#v", response.Code, snapshots)
	}
}

func TestStreamSessionAuditsLiveAndPlaybackBeforeResponding(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		action string
	}{
		{name: "live", body: `{"mode":"live"}`, action: "monitor.live_view"},
		{name: "playback", body: `{"mode":"playback","start_time":100,"end_time":200}`, action: "monitor.playback_view"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorizer := &fakeAuditAuthorizer{fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}}}
			handler := newAuditedHandler(t, authorizer, nil)
			request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/stream-session", strings.NewReader(tt.body))
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK || len(authorizer.events) != 1 {
				t.Fatalf("status = %d events = %#v body = %s", response.Code, authorizer.events, response.Body.String())
			}
			event := authorizer.events[0]
			if event.Action != tt.action || event.EntityType != "camera" || event.ExternalOrgID != "10001" || event.EntityID == nil || *event.EntityID != 111 || event.Result != "success" {
				t.Fatalf("event = %#v", event)
			}
			if !strings.Contains(string(event.DetailJSON), "门店：北京保利总部店（10001）") ||
				!strings.Contains(string(event.DetailJSON), "摄像头区域：治疗室 / 产研中心1-2") ||
				!strings.Contains(string(event.DetailJSON), "摄像头：治疗室4（ID：111）") ||
				strings.Contains(string(event.DetailJSON), "wss://") {
				t.Fatalf("event detail = %s", event.DetailJSON)
			}
		})
	}
}

func TestStreamSessionBlocksStreamWhenAuditFails(t *testing.T) {
	authorizer := &fakeAuditAuthorizer{
		fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}},
		err:            errors.New("audit unavailable"),
	}
	handler := newAuditedHandler(t, authorizer, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"live"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"nvr_monitor_audit_failed"`) || strings.Contains(response.Body.String(), "stream.example.test") {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestSnapshotDownloadDoesNotAuditAutomaticImageLoad(t *testing.T) {
	authorizer := &fakeAuditAuthorizer{
		fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}},
		err:            errors.New("audit unavailable"),
	}
	handler := newAuditedHandler(t, authorizer, fakeSnapshotStore{data: map[int64]string{111: "jpeg-data"}})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "jpeg-data" || len(authorizer.events) != 0 {
		t.Fatalf("status = %d events = %#v body = %q", response.Code, authorizer.events, response.Body.String())
	}
}

func TestSnapshotRefreshAuditsBeforeWritingSnapshot(t *testing.T) {
	snapshots := &fakeWritableSnapshotStore{fakeSnapshotStore: fakeSnapshotStore{data: map[int64]string{}}}
	authorizer := &fakeAuditedSnapshotBackfillAuthorizer{
		fakeAuditAuthorizer: fakeAuditAuthorizer{fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}}},
		allowBackfill:       true,
	}
	service := NewServiceWithSnapshotStore(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{}, snapshots)
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, authorizer)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00}))
	request.Header.Set("Content-Type", "image/jpeg")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || len(authorizer.events) != 2 || snapshots.savedCameraID != 111 {
		t.Fatalf("status = %d events = %#v snapshots = %#v", response.Code, authorizer.events, snapshots)
	}
	if authorizer.events[0].Action != snapshotRefreshPrepareAction || authorizer.events[0].Result != "success" {
		t.Fatalf("prepare audit event = %#v", authorizer.events[0])
	}
	event := authorizer.events[1]
	if event.Action != "snapshot.refresh" || event.EntityType != "camera" || event.EntityID == nil || *event.EntityID != 111 || event.ExternalOrgID != "10001" || event.Result != "success" {
		t.Fatalf("event = %#v", event)
	}
}

func TestSnapshotRefreshBlocksWriteWhenAuditFails(t *testing.T) {
	snapshots := &fakeWritableSnapshotStore{fakeSnapshotStore: fakeSnapshotStore{data: map[int64]string{}}}
	authorizer := &fakeAuditedSnapshotBackfillAuthorizer{
		fakeAuditAuthorizer: fakeAuditAuthorizer{
			fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}},
			err:            errors.New("audit unavailable"),
		},
		allowBackfill: true,
	}
	service := NewServiceWithSnapshotStore(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{}, snapshots)
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, authorizer)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00}))
	request.Header.Set("Content-Type", "image/jpeg")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || snapshots.savedCameraID != 0 || !strings.Contains(response.Body.String(), `"code":"nvr_monitor_audit_failed"`) {
		t.Fatalf("status = %d body = %s snapshots = %#v", response.Code, response.Body.String(), snapshots)
	}
}

func TestPermissionDenialAuditsWithoutChangingForbiddenResponse(t *testing.T) {
	authorizer := &fakeAuditAuthorizer{fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": false}}}
	handler := newAuditedHandler(t, authorizer, fakeSnapshotStore{data: map[int64]string{111: "jpeg-data"}})
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"playback","start_time":100,"end_time":200}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || len(authorizer.events) != 1 {
		t.Fatalf("status = %d events = %#v body = %s", response.Code, authorizer.events, response.Body.String())
	}
	event := authorizer.events[0]
	if event.Action != "monitor.playback_view" || event.Result != "denied" || event.EntityType != "camera" || event.EntityID == nil || *event.EntityID != 111 || event.ExternalOrgID != "10001" {
		t.Fatalf("event = %#v", event)
	}
}

func TestPermissionDenialAuditFailureDoesNotChangeForbiddenResponse(t *testing.T) {
	authorizer := &fakeAuditAuthorizer{
		fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": false}},
		err:            errors.New("audit unavailable"),
	}
	handler := newAuditedHandler(t, authorizer, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"nvr_monitor_forbidden"`) {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestStreamSessionRejectsTrailingJSON(t *testing.T) {
	handler := newTestHandler(t, map[string]bool{"10001": true})
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"live"}{"mode":"playback"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestStreamSessionRejectsOversizedBody(t *testing.T) {
	handler := newTestHandler(t, map[string]bool{"10001": true})
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"live","padding":"`+strings.Repeat("x", maxStreamSessionBodyBytes)+`"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestStreamSessionAuditsSuccessBeforeReturningURL(t *testing.T) {
	authorizer := &fakeAuditAuthorizer{fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}}}
	handler := newAuditedHandler(t, authorizer, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"live"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || len(authorizer.events) != 1 {
		t.Fatalf("status = %d events = %#v body = %s", response.Code, authorizer.events, response.Body.String())
	}
	event := authorizer.events[0]
	if event.Action != "monitor.live_view" || event.Result != "success" || event.EntityType != "camera" || event.EntityID == nil || *event.EntityID != 111 {
		t.Fatalf("audit event = %#v", event)
	}
	if strings.Contains(response.Body.String(), "service-secret") || !strings.Contains(string(event.DetailJSON), "门店：北京保利总部店（10001）") {
		t.Fatalf("sensitive data leaked: body=%s event=%#v", response.Body.String(), event)
	}
}

func TestStreamSessionAuditFailureDoesNotReturnURL(t *testing.T) {
	authorizer := &fakeAuditAuthorizer{
		fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}},
		err:            errors.New("audit recorder unavailable"),
	}
	handler := newAuditedHandler(t, authorizer, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"live"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "wss://") {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestStreamSessionUpstreamFailureAuditsFailedWithoutLeakingError(t *testing.T) {
	authorizer := &fakeAuditAuthorizer{fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}}}
	service := NewService(
		fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}},
		&fakeAuthorizationClient{err: errors.New("upstream token=secret")},
	)
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, authorizer)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"live"}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError || len(authorizer.events) != 1 {
		t.Fatalf("status = %d events = %#v body = %s", response.Code, authorizer.events, response.Body.String())
	}
	if authorizer.events[0].Action != "monitor.live_view" || authorizer.events[0].Result != "failed" || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("audit or response leaked data: event=%#v body=%s", authorizer.events[0], response.Body.String())
	}
}

func TestSnapshotRejectsUnsafeStoredContentType(t *testing.T) {
	handler := newTestHandlerWithSnapshots(t, map[string]bool{"10001": true}, snapshotStoreWithContentType{
		contentType: "text/html",
	})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "jpeg-data") {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
}

func TestSnapshotBackfillRejectsOversizedPayload(t *testing.T) {
	snapshots := &fakeWritableSnapshotStore{fakeSnapshotStore: fakeSnapshotStore{data: map[int64]string{}}}
	service := NewServiceWithSnapshotStore(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{}, snapshots)
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, fakeSnapshotBackfillAuthorizer{fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}}, allowBackfill: true})
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", bytes.NewReader(append([]byte{0xff, 0xd8, 0xff}, bytes.Repeat([]byte("x"), maxSnapshotUploadBytes)...)))
	request.Header.Set("Content-Type", "image/jpeg")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge || snapshots.savedCameraID != 0 {
		t.Fatalf("status = %d write = %#v", response.Code, snapshots)
	}
}

func TestSnapshotRefreshAuditsOnlyAfterSuccessfulWrite(t *testing.T) {
	saved := false
	snapshots := &trackingSnapshotWriter{fakeWritableSnapshotStore: fakeWritableSnapshotStore{fakeSnapshotStore: fakeSnapshotStore{data: map[int64]string{}}}}
	authorizer := &fakeAuditedSnapshotBackfillAuthorizer{
		fakeAuditAuthorizer: fakeAuditAuthorizer{
			fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}},
			check: func(event auditlog.AuditEvent) error {
				if event.Action == "snapshot.refresh" && !saved {
					t.Errorf("snapshot.refresh success was recorded before SaveSnapshot completed")
				}
				return nil
			},
		},
		allowBackfill: true,
	}
	snapshots.onSave = func() { saved = true }
	handler := newAuditedHandlerWithAuthorizer(t, authorizer, snapshots)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00}))
	request.Header.Set("Content-Type", "image/jpeg")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || snapshots.savedCameraID != 111 || len(authorizer.events) != 2 {
		t.Fatalf("status = %d snapshots = %#v events = %#v", response.Code, snapshots, authorizer.events)
	}
	if authorizer.events[0].Action != snapshotRefreshPrepareAction || authorizer.events[0].Result != "success" {
		t.Fatalf("prepare audit event = %#v", authorizer.events[0])
	}
	if authorizer.events[1].Action != "snapshot.refresh" || authorizer.events[1].Result != "success" {
		t.Fatalf("audit event = %#v", authorizer.events[1])
	}
}

func TestSnapshotRefreshWriteFailureAuditsFailedWithoutSuccess(t *testing.T) {
	authorizer := &fakeAuditedSnapshotBackfillAuthorizer{
		fakeAuditAuthorizer: fakeAuditAuthorizer{fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}}},
		allowBackfill:       true,
	}
	service := NewServiceWithSnapshotStore(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{}, failingSnapshotWriter{err: errors.New("oss unavailable")})
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, authorizer)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00}))
	request.Header.Set("Content-Type", "image/jpeg")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError || len(authorizer.events) != 2 {
		t.Fatalf("status = %d events = %#v body = %s", response.Code, authorizer.events, response.Body.String())
	}
	if authorizer.events[0].Action != snapshotRefreshPrepareAction || authorizer.events[0].Result != "success" {
		t.Fatalf("prepare audit event = %#v", authorizer.events[0])
	}
	if authorizer.events[1].Action != "snapshot.refresh" || authorizer.events[1].Result != "failed" {
		t.Fatalf("failed audit event = %#v", authorizer.events[1])
	}
}

func TestSnapshotRefreshFinalAuditFailureDoesNotConfirmSuccess(t *testing.T) {
	snapshots := &fakeWritableSnapshotStore{fakeSnapshotStore: fakeSnapshotStore{data: map[int64]string{}}}
	authorizer := &fakeAuditedSnapshotBackfillAuthorizer{
		fakeAuditAuthorizer: fakeAuditAuthorizer{
			fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}},
			errs:           []error{nil, errors.New("audit unavailable")},
		},
		allowBackfill: true,
	}
	handler := newAuditedHandlerWithAuthorizer(t, authorizer, snapshots)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00}))
	request.Header.Set("Content-Type", "image/jpeg")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || snapshots.savedCameraID != 111 {
		t.Fatalf("status = %d saved camera = %d body = %s", response.Code, snapshots.savedCameraID, response.Body.String())
	}
	if _, exists := snapshots.data[111]; exists {
		t.Fatalf("new snapshot must be deleted after audit failure: %#v", snapshots.data)
	}
	if len(authorizer.events) != 2 || authorizer.events[0].Action != snapshotRefreshPrepareAction || authorizer.events[1].Action != "snapshot.refresh" || authorizer.events[1].Result != "success" {
		t.Fatalf("audit events = %#v", authorizer.events)
	}
}

func TestSnapshotRefreshFinalAuditFailureRestoresExistingSnapshot(t *testing.T) {
	snapshots := &fakeWritableSnapshotStore{fakeSnapshotStore: fakeSnapshotStore{data: map[int64]string{111: "old-jpeg"}}}
	authorizer := &fakeAuditedSnapshotBackfillAuthorizer{
		fakeAuditAuthorizer: fakeAuditAuthorizer{
			fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}},
			errs:           []error{nil, errors.New("audit unavailable")},
		},
		allowBackfill: true,
	}
	handler := newAuditedHandlerWithAuthorizer(t, authorizer, snapshots)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00}))
	request.Header.Set("Content-Type", "image/jpeg")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || snapshots.data[111] != "old-jpeg" {
		t.Fatalf("status = %d snapshots = %#v body = %s", response.Code, snapshots.data, response.Body.String())
	}
}

func TestSnapshotRefreshFinalAuditFailureReportsRollbackFailure(t *testing.T) {
	snapshots := rollbackFailingSnapshotWriter{fakeWritableSnapshotStore: fakeWritableSnapshotStore{fakeSnapshotStore: fakeSnapshotStore{data: map[int64]string{}}}}
	authorizer := &fakeAuditedSnapshotBackfillAuthorizer{
		fakeAuditAuthorizer: fakeAuditAuthorizer{
			fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": true}},
			errs:           []error{nil, errors.New("audit unavailable")},
		},
		allowBackfill: true,
	}
	handler := newAuditedHandlerWithAuthorizer(t, authorizer, snapshots)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00}))
	request.Header.Set("Content-Type", "image/jpeg")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"nvr_snapshot_rollback_failed"`) {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestDeniedCameraRequestAuditsDenied(t *testing.T) {
	authorizer := &fakeAuditAuthorizer{fakeAuthorizer: fakeAuthorizer{allow: map[string]bool{"10001": false}}}
	handler := newAuditedHandler(t, authorizer, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/orgs/10001/cameras", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || len(authorizer.events) != 1 {
		t.Fatalf("status = %d events = %#v", response.Code, authorizer.events)
	}
	if authorizer.events[0].Action != "monitor.camera_list" || authorizer.events[0].Result != "denied" {
		t.Fatalf("audit event = %#v", authorizer.events[0])
	}
}

type snapshotStoreWithContentType struct {
	contentType string
}

type trackingSnapshotWriter struct {
	fakeWritableSnapshotStore
	onSave func()
}

type rollbackFailingSnapshotWriter struct {
	fakeWritableSnapshotStore
}

type rollbackFailure struct{}

func (rollbackFailure) Rollback(context.Context) error {
	return errors.New("rollback unavailable")
}

func (s rollbackFailingSnapshotWriter) SaveSnapshotWithRollback(ctx context.Context, tenantID int64, cameraID int64, body io.Reader) (SnapshotRollback, error) {
	if err := s.fakeWritableSnapshotStore.Save(ctx, tenantID, cameraID, body); err != nil {
		return nil, err
	}
	return rollbackFailure{}, nil
}

func (s *trackingSnapshotWriter) SaveSnapshotWithRollback(ctx context.Context, tenantID int64, cameraID int64, body io.Reader) (SnapshotRollback, error) {
	rollback, err := s.fakeWritableSnapshotStore.SaveSnapshotWithRollback(ctx, tenantID, cameraID, body)
	if err == nil && s.onSave != nil {
		s.onSave()
	}
	return rollback, err
}

func (s *trackingSnapshotWriter) Save(ctx context.Context, tenantID int64, cameraID int64, body io.Reader) error {
	if err := s.fakeWritableSnapshotStore.Save(ctx, tenantID, cameraID, body); err != nil {
		return err
	}
	if s.onSave != nil {
		s.onSave()
	}
	return nil
}

func (s snapshotStoreWithContentType) Open(_ context.Context, _ int64, _ int64) (io.ReadCloser, string, error) {
	return io.NopCloser(strings.NewReader("jpeg-data")), s.contentType, nil
}

func newAuditedHandlerWithAuthorizer(t *testing.T, authorizer Authorizer, snapshots SnapshotStore) http.Handler {
	t.Helper()
	service := NewServiceWithSnapshotStore(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{url: "wss://stream.example.test/session"}, snapshots)
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, authorizer)
	return mux
}

type failingSnapshotWriter struct {
	err error
}

func (s failingSnapshotWriter) Open(context.Context, int64, int64) (io.ReadCloser, string, error) {
	return nil, "", ErrSnapshotNotFound
}

func (s failingSnapshotWriter) Save(context.Context, int64, int64, io.Reader) error {
	return s.err
}

func (s failingSnapshotWriter) SaveSnapshotWithRollback(context.Context, int64, int64, io.Reader) (SnapshotRollback, error) {
	return nil, s.err
}
