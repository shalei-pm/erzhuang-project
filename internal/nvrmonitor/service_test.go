package nvrmonitor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
)

type fakeRepository struct {
	stores map[int64]resourceview.StoreRecords
}

func (r fakeRepository) ListStores(context.Context, resourceview.StoreFilters) ([]resourceview.StoreRecords, error) {
	return nil, nil
}

func (r fakeRepository) ListNVRMonitorStores(context.Context) ([]resourceview.StoreRecords, error) {
	result := make([]resourceview.StoreRecords, 0, len(r.stores))
	for _, store := range r.stores {
		result = append(result, store)
	}
	return result, nil
}

func (r fakeRepository) GetStoreRecords(_ context.Context, tenantID int64) (resourceview.StoreRecords, error) {
	return r.GetNVRMonitorStoreRecords(context.Background(), tenantID)
}

func (r fakeRepository) GetNVRMonitorStoreRecords(_ context.Context, tenantID int64) (resourceview.StoreRecords, error) {
	store, ok := r.stores[tenantID]
	if !ok {
		return resourceview.StoreRecords{}, resourceview.ErrNotFound
	}
	return store, nil
}

type fakeAuthorizationClient struct {
	lastCameraID int64
	lastRequest  StreamSessionRequest
	url          string
	err          error
}

type fakeSnapshotStore struct {
	data map[int64]string
	err  error
}

type fakeWritableSnapshotStore struct {
	fakeSnapshotStore
	savedTenantID    int64
	savedCameraID    int64
	savedContentType string
}

func (s *fakeWritableSnapshotStore) Save(_ context.Context, tenantID int64, cameraID int64, body io.Reader) error {
	_, _ = io.ReadAll(body)
	s.savedTenantID = tenantID
	s.savedCameraID = cameraID
	s.savedContentType = "image/jpeg"
	return nil
}

func (s fakeSnapshotStore) Open(_ context.Context, _ int64, cameraID int64) (io.ReadCloser, string, error) {
	if s.err != nil {
		return nil, "", s.err
	}
	data, ok := s.data[cameraID]
	if !ok {
		return nil, "", ErrSnapshotNotFound
	}
	return io.NopCloser(bytes.NewBufferString(data)), "image/jpeg", nil
}

func (c *fakeAuthorizationClient) CreateStreamURL(_ context.Context, cameraID int64, request StreamSessionRequest) (string, error) {
	c.lastCameraID = cameraID
	c.lastRequest = request
	if c.err != nil {
		return "", c.err
	}
	return c.url, nil
}

func nvrMonitorRecords() resourceview.StoreRecords {
	return resourceview.StoreRecords{
		Tenant: resourceview.BusinessTenant{ID: 10001, Name: "北京保利总部店", CityID: 1},
		Devices: []resourceview.BusinessDevice{
			{ID: 111, TenantID: 10001, Name: "治疗室4", HardwareID: "NVRCHANNEL:22-10", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1},
			{ID: 112, TenantID: 10001, Name: "错误 provider", HardwareID: "NVRCHANNEL:22-11", Category: "camera", Provider: "Other", Status: 1},
			{ID: 113, TenantID: 10001, Name: "停用", HardwareID: "NVRCHANNEL:22-12", Category: "camera", Provider: "HikVisionNvrChannel", Status: 0},
		},
		Spaces: []resourceview.BusinessSpace{
			{ID: 2387, TenantID: 10001, Name: "诊室区域", Level: 1},
			{ID: 2665, TenantID: 10001, ParentID: 2387, Name: "产研中心", Level: 2},
			{ID: 2667, TenantID: 10001, ParentID: 2665, Name: "产研中心1-2", Level: 3},
		},
		Relations: []resourceview.BusinessAreaDeviceRelation{
			{ID: 1, DeviceID: 111, AreaID: 2665},
			{ID: 2, DeviceID: 111, AreaID: 2667},
		},
		LegacyCameraSnapshots: map[int]string{10: "0123456789abcdef0123456789abcdef.jpg"},
	}
}

func TestListStoresOnlyReturnsStoresWithEligibleCameras(t *testing.T) {
	service := NewService(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{})

	response, err := service.ListStores(context.Background())
	if err != nil {
		t.Fatalf("ListStores() error = %v", err)
	}
	if len(response.Cities) != 1 || len(response.Cities[0].Stores) != 1 {
		t.Fatalf("stores = %#v", response)
	}
	if got := response.Cities[0].Stores[0].AvailableCameraCount; got != 1 {
		t.Fatalf("AvailableCameraCount = %d, want 1", got)
	}
}

func TestGetCamerasOnlyReturnsEligibleCameraAndApplicableSpace(t *testing.T) {
	service := NewService(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{})

	response, err := service.GetCameras(context.Background(), "10001")
	if err != nil {
		t.Fatalf("GetCameras() error = %v", err)
	}
	if len(response.Cameras) != 1 {
		t.Fatalf("cameras = %#v, want one", response.Cameras)
	}
	camera := response.Cameras[0]
	if camera.ID != 111 || camera.SpaceType != "治疗室" || camera.SpaceName != "产研中心1-2" {
		t.Fatalf("camera = %#v", camera)
	}
	if camera.ThumbnailURL == "" {
		t.Fatalf("camera = %#v, want legacy thumbnail url", camera)
	}
}

func TestGetCamerasProvidesPrivateSnapshotEndpointWhenStoreConfigured(t *testing.T) {
	service := NewServiceWithSnapshotStore(
		fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}},
		&fakeAuthorizationClient{},
		fakeSnapshotStore{},
	)

	response, err := service.GetCameras(context.Background(), "10001")
	if err != nil {
		t.Fatalf("GetCameras() error = %v", err)
	}
	if got, want := response.Cameras[0].ThumbnailURL, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot"; got != want {
		t.Fatalf("thumbnail url = %q, want %q", got, want)
	}
}

func TestCreateSessionRejectsCameraOutsideEligibleSet(t *testing.T) {
	service := NewService(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{})

	_, err := service.CreateSession(context.Background(), "10001", 112, StreamSessionRequest{Mode: ModeLive})
	if !errors.Is(err, ErrCameraNotFound) {
		t.Fatalf("CreateSession() error = %v, want ErrCameraNotFound", err)
	}
}

func TestCreateSessionAllowsOneHourPlayback(t *testing.T) {
	client := &fakeAuthorizationClient{url: "wss://stream.example.test/session"}
	service := NewService(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, client)

	response, err := service.CreateSession(context.Background(), "10001", 111, StreamSessionRequest{Mode: ModePlayback, StartTime: 100, EndTime: 3700})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if response.URL != "wss://stream.example.test/session" || client.lastCameraID != 111 {
		t.Fatalf("response = %#v client = %#v", response, client)
	}
}

func TestCreateSessionRejectsPlaybackLongerThanOneHour(t *testing.T) {
	service := NewService(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{})

	_, err := service.CreateSession(context.Background(), "10001", 111, StreamSessionRequest{Mode: ModePlayback, StartTime: 100, EndTime: 3701})
	if !errors.Is(err, ErrInvalidPlaybackWindow) {
		t.Fatalf("CreateSession() error = %v, want ErrInvalidPlaybackWindow", err)
	}
}
