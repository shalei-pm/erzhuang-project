package nvrlab

import (
	"context"
	"errors"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
)

type fakeRepository struct {
	records map[int64]resourceview.StoreRecords
}

func (r fakeRepository) ListStores(context.Context, resourceview.StoreFilters) ([]resourceview.StoreRecords, error) {
	return nil, nil
}

func (r fakeRepository) GetStoreRecords(_ context.Context, tenantID int64) (resourceview.StoreRecords, error) {
	record, ok := r.records[tenantID]
	if !ok {
		return resourceview.StoreRecords{}, resourceview.ErrNotFound
	}
	return record, nil
}

type fakeAuthorizationClient struct {
	lastCameraID int64
	lastRequest  StreamSessionRequest
	url          string
	err          error
}

func (c *fakeAuthorizationClient) CreateStreamURL(_ context.Context, cameraID int64, request StreamSessionRequest) (string, error) {
	c.lastCameraID = cameraID
	c.lastRequest = request
	if c.err != nil {
		return "", c.err
	}
	return c.url, nil
}

func sampleRecords() resourceview.StoreRecords {
	return resourceview.StoreRecords{
		Tenant: resourceview.BusinessTenant{ID: ExperimentTenantID, Name: "北京保利总部店"},
		Devices: []resourceview.BusinessDevice{
			{ID: 111, TenantID: ExperimentTenantID, Name: "治疗室4", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1},
			{ID: 112, TenantID: ExperimentTenantID, Name: "非海康", Category: "camera", Provider: "Other", Status: 1},
		},
		Spaces: []resourceview.BusinessSpace{
			{ID: 2665, TenantID: ExperimentTenantID, Name: "产研中心", Level: 2},
			{ID: 2667, TenantID: ExperimentTenantID, ParentID: 2665, Name: "产研中心1-2", Level: 3},
		},
		Relations: []resourceview.BusinessAreaDeviceRelation{{ID: 1, DeviceID: 111, AreaID: 2667}},
	}
}

func TestListCamerasReturnsOnlyEnabledHikVisionCameras(t *testing.T) {
	service := NewService(fakeRepository{records: map[int64]resourceview.StoreRecords{ExperimentTenantID: sampleRecords()}}, &fakeAuthorizationClient{})

	response, err := service.ListCameras(context.Background(), ExperimentTenantID)
	if err != nil {
		t.Fatalf("ListCameras() error = %v", err)
	}
	if len(response.Cameras) != 1 {
		t.Fatalf("cameras = %#v, want one valid camera", response.Cameras)
	}
	if response.Cameras[0].ID != 111 || response.Cameras[0].SpaceType != "治疗室" || response.Cameras[0].SpaceName != "产研中心1-2" {
		t.Fatalf("camera = %#v", response.Cameras[0])
	}
}

func TestCreateSessionRejectsCameraOutsideExperimentStore(t *testing.T) {
	service := NewService(fakeRepository{records: map[int64]resourceview.StoreRecords{ExperimentTenantID: sampleRecords()}}, &fakeAuthorizationClient{})

	_, err := service.CreateSession(context.Background(), ExperimentTenantID+1, 111, StreamSessionRequest{Mode: ModeLive})
	if !errors.Is(err, ErrExperimentNotFound) {
		t.Fatalf("CreateSession() error = %v, want ErrExperimentNotFound", err)
	}
}

func TestCreateSessionRejectsCameraOutsideEligibleSet(t *testing.T) {
	service := NewService(fakeRepository{records: map[int64]resourceview.StoreRecords{ExperimentTenantID: sampleRecords()}}, &fakeAuthorizationClient{})

	_, err := service.CreateSession(context.Background(), ExperimentTenantID, 112, StreamSessionRequest{Mode: ModeLive})
	if !errors.Is(err, ErrCameraNotFound) {
		t.Fatalf("CreateSession() error = %v, want ErrCameraNotFound", err)
	}
}

func TestCreateSessionRejectsPlaybackLongerThanThirtyMinutes(t *testing.T) {
	service := NewService(fakeRepository{records: map[int64]resourceview.StoreRecords{ExperimentTenantID: sampleRecords()}}, &fakeAuthorizationClient{})

	_, err := service.CreateSession(context.Background(), ExperimentTenantID, 111, StreamSessionRequest{Mode: ModePlayback, StartTime: 100, EndTime: 1901})
	if !errors.Is(err, ErrInvalidPlaybackWindow) {
		t.Fatalf("CreateSession() error = %v, want ErrInvalidPlaybackWindow", err)
	}
}

func TestCreateSessionUsesFreshAuthorizationForLiveStream(t *testing.T) {
	client := &fakeAuthorizationClient{url: "wss://example.test/session"}
	service := NewService(fakeRepository{records: map[int64]resourceview.StoreRecords{ExperimentTenantID: sampleRecords()}}, client)

	session, err := service.CreateSession(context.Background(), ExperimentTenantID, 111, StreamSessionRequest{Mode: ModeLive})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if session.URL != "wss://example.test/session" || session.Mode != ModeLive {
		t.Fatalf("session = %#v", session)
	}
	if client.lastCameraID != 111 || client.lastRequest.Mode != ModeLive || client.lastRequest.StartTime != 0 || client.lastRequest.EndTime != 0 {
		t.Fatalf("authorization request = %#v camera=%d", client.lastRequest, client.lastCameraID)
	}
}
