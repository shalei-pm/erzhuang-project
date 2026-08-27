package resourceview

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type serviceRepositoryStub struct {
	records StoreRecords
}

func (r serviceRepositoryStub) ListStores(context.Context, StoreFilters) ([]StoreRecords, error) {
	return []StoreRecords{r.records}, nil
}

func (r serviceRepositoryStub) ListNVRMonitorStores(context.Context) ([]StoreRecords, error) {
	return []StoreRecords{r.records}, nil
}

func (r serviceRepositoryStub) GetStoreRecords(context.Context, int64) (StoreRecords, error) {
	return r.records, nil
}

func (r serviceRepositoryStub) GetNVRMonitorStoreRecords(context.Context, int64) (StoreRecords, error) {
	return r.records, nil
}

func TestStoreListItemRequiresEveryCameraToHaveAValidBinding(t *testing.T) {
	updatedAt := time.Date(2026, time.August, 19, 14, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	fullyBound := storeListItem(
		StoreDetail{TenantID: 10019, Summary: StoreSummary{CameraCount: 2, BoundCameraCount: 2, UnboundCameraCount: 0}},
		StoreRecords{Relations: []BusinessAreaDeviceRelation{{ID: 1, CreatedAt: updatedAt}}},
		MonitorAccess{},
	)
	if !fullyBound.CamerasFullyBound {
		t.Fatalf("fully bound cameras should be confirmed: %#v", fullyBound)
	}
	if fullyBound.UpdatedAt != "2026-08-19T14:30:00+08:00" {
		t.Fatalf("updated_at = %q", fullyBound.UpdatedAt)
	}

	partiallyBound := storeListItem(
		StoreDetail{Summary: StoreSummary{CameraCount: 2, BoundCameraCount: 1, UnboundCameraCount: 1}},
		StoreRecords{},
		MonitorAccess{},
	)
	if partiallyBound.CamerasFullyBound {
		t.Fatalf("a store with an unbound camera must not be confirmed: %#v", partiallyBound)
	}
	if partiallyBound.UpdatedAt != "" {
		t.Fatalf("store without relations should not have an update time: %#v", partiallyBound)
	}

	noCameras := storeListItem(StoreDetail{Summary: StoreSummary{}}, StoreRecords{}, MonitorAccess{})
	if noCameras.CamerasFullyBound {
		t.Fatalf("a store without cameras must not be confirmed: %#v", noCameras)
	}
}

func TestCityNameUsesStaticDistrictAliases(t *testing.T) {
	tests := []struct {
		cityID int64
		want   string
	}{
		{cityID: 1, want: "北京"},
		{cityID: 9, want: "上海"},
		{cityID: 175, want: "杭州"},
		{cityID: 275, want: "长沙"},
		{cityID: 385, want: "成都"},
		{cityID: 999999, want: "城市 999999"},
		{cityID: 0, want: ""},
	}
	for _, test := range tests {
		if got := CityName(test.cityID); got != test.want {
			t.Errorf("CityName(%d) = %q, want %q", test.cityID, got, test.want)
		}
	}
}

func TestBuildStoreDetailCreatesThreeLevelSpaceTreeAndDeviceTree(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10019, Name: "上海陆家嘴店", HospitalName: "新氧青春诊所(上海陆家嘴店)", Status: 1, CityID: 9},
		Devices: []BusinessDevice{
			{ID: 1, TenantID: 10019, Name: "edge-1", HardwareID: "60beb422a54f", Category: "edge", Status: 1, OnlineStatus: 1},
			{ID: 22, TenantID: 10019, Name: "nvr-1", HardwareID: "NVR001", Category: "nvr", Status: 1, OnlineStatus: 1},
			{ID: 68, TenantID: 10019, ParentID: 22, Name: "治疗室1", HardwareID: "NVRCHANNEL:22-1", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1, OnlineStatus: 1},
		},
		Spaces: []BusinessSpace{
			{ID: 10, TenantID: 10019, Name: "治疗区域", Level: 1, Status: 1, SortOrder: 2},
			{ID: 11, TenantID: 10019, ParentID: 10, Name: "治疗室1", Level: 2, Status: 1, SortOrder: 1},
			{ID: 12, TenantID: 10019, ParentID: 11, Name: "床位1", Level: 3, Status: 1, SortOrder: 1},
		},
		Relations: []BusinessAreaDeviceRelation{{ID: 99, AreaID: 12, DeviceID: 68, FunctionType: "camera"}},
	}

	detail := BuildStoreDetail(records, MonitorAccess{CanViewMonitor: true, MonitorURL: "/monitor/10019"})

	if detail.TenantID != 10019 {
		t.Fatalf("tenant id = %d, want 10019", detail.TenantID)
	}
	if !detail.CanViewMonitor || detail.MonitorURL != "/monitor/10019" {
		t.Fatalf("monitor access = %#v, url %q", detail.CanViewMonitor, detail.MonitorURL)
	}
	if len(detail.SpaceTree) != 1 || len(detail.SpaceTree[0].Children) != 1 || len(detail.SpaceTree[0].Children[0].Children) != 1 {
		t.Fatalf("space tree was not three levels: %#v", detail.SpaceTree)
	}
	leaf := detail.SpaceTree[0].Children[0].Children[0]
	if leaf.ID != 12 || leaf.BoundCameraCount != 1 || !reflect.DeepEqual(leaf.BoundCameraIDs, []int64{68}) {
		t.Fatalf("leaf binding = %#v", leaf)
	}
	flatLeaf := findSpace(detail.Spaces, 12)
	if flatLeaf == nil {
		t.Fatalf("flat spaces missing leaf 12: %#v", detail.Spaces)
	}
	if flatLeaf.BoundCameraCount != leaf.BoundCameraCount || !reflect.DeepEqual(flatLeaf.BoundCameraIDs, leaf.BoundCameraIDs) {
		t.Fatalf("flat leaf binding = %#v, tree leaf binding = %#v", flatLeaf, leaf.Space)
	}
	if len(leaf.BoundCameras) != 1 || leaf.BoundCameras[0].ID != 68 {
		t.Fatalf("leaf bound cameras = %#v", leaf.BoundCameras)
	}
	if len(detail.DeviceTree.Edges) != 1 {
		t.Fatalf("edge count = %d, want 1", len(detail.DeviceTree.Edges))
	}
	if len(detail.DeviceTree.NVRs) != 1 || len(detail.DeviceTree.NVRs[0].Cameras) != 1 {
		t.Fatalf("nvr camera tree = %#v", detail.DeviceTree.NVRs)
	}
	camera := detail.DeviceTree.NVRs[0].Cameras[0]
	if camera.ChannelNo == nil || *camera.ChannelNo != 1 {
		t.Fatalf("channel no = %#v, want 1", camera.ChannelNo)
	}
	if camera.NVRName != "nvr-1" {
		t.Fatalf("nvr name = %q, want nvr-1", camera.NVRName)
	}
	if len(camera.SpacePaths) != 1 || camera.SpacePaths[0] != "治疗区域 / 治疗室1 / 床位1" {
		t.Fatalf("space paths = %#v", camera.SpacePaths)
	}
	if detail.Summary.BoundCameraCount != 1 || detail.Summary.UnboundCameraCount != 0 {
		t.Fatalf("summary = %#v", detail.Summary)
	}
	if len(detail.Issues) != 0 {
		t.Fatalf("issues = %#v, want none", detail.Issues)
	}
}

func TestBuildStoreDetailShowsLegacySnapshotOnlyWithMonitorAccess(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10001, Name: "单录像机门店"},
		Devices: []BusinessDevice{
			{ID: 70, TenantID: 10001, HardwareID: "NVRCHANNEL:22-10", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1},
		},
		LegacyCameraSnapshots: map[int]string{10: "/api/store-space/channel-snapshots/0123456789abcdef0123456789abcdef.jpg"},
	}

	withAccess := BuildStoreDetail(records, MonitorAccess{CanViewMonitor: true})
	if got := withAccess.Cameras[0].ThumbnailURL; got != "/api/store-space-resource-view/stores/10001/cameras/70/snapshot" {
		t.Fatalf("thumbnail url = %q", got)
	}

	withoutAccess := BuildStoreDetail(records, MonitorAccess{})
	if got := withoutAccess.Cameras[0].ThumbnailURL; got != "" {
		t.Fatalf("thumbnail must be withheld without monitor access, got %q", got)
	}
}

func TestBuildStoreDetailLeavesMissingLegacySnapshotEmpty(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10001, Name: "单录像机门店"},
		Devices: []BusinessDevice{
			{ID: 70, TenantID: 10001, HardwareID: "NVRCHANNEL:22-10", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1},
		},
		LegacyCameraSnapshots: map[int]string{},
	}

	detail := BuildStoreDetail(records, MonitorAccess{CanViewMonitor: true})
	if got := detail.Cameras[0].ThumbnailURL; got != "" {
		t.Fatalf("thumbnail url = %q, want empty", got)
	}
}

func TestGetStoreReusesNVRSnapshotOnlyForAuthorizedMonitorAccess(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10001, Name: "单录像机门店"},
		Devices: []BusinessDevice{
			{ID: 70, TenantID: 10001, HardwareID: "NVRCHANNEL:22-10", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1},
			{ID: 71, TenantID: 10001, HardwareID: "NVRCHANNEL:22-11", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1},
		},
		LegacyCameraSnapshots: map[int]string{10: "legacy-10.jpg", 11: "legacy-11.jpg"},
	}
	service := NewService(serviceRepositoryStub{records: records})
	resolverCalls := 0
	service.UseCameraSnapshotURLResolver(func(_ context.Context, tenantID int64, cameraID int64) string {
		resolverCalls++
		if tenantID == 10001 && cameraID == 70 {
			return "/api/h5/nvr-monitor/orgs/10001/cameras/70/snapshot"
		}
		return ""
	})

	detail, err := service.GetStore(context.Background(), 10001, MonitorAccess{CanViewMonitor: true})
	if err != nil {
		t.Fatalf("GetStore() error = %v", err)
	}
	if resolverCalls != 2 {
		t.Fatalf("resolver calls = %d, want 2", resolverCalls)
	}
	if got := detail.Cameras[0].ThumbnailURL; got != "/api/h5/nvr-monitor/orgs/10001/cameras/70/snapshot" {
		t.Fatalf("camera 70 thumbnail = %q", got)
	}
	if got := detail.Cameras[1].ThumbnailURL; got != "/api/store-space-resource-view/stores/10001/cameras/71/snapshot" {
		t.Fatalf("camera 71 thumbnail = %q, want legacy fallback", got)
	}

	resolverCalls = 0
	detail, err = service.GetStore(context.Background(), 10001, MonitorAccess{})
	if err != nil {
		t.Fatalf("GetStore() without access error = %v", err)
	}
	if resolverCalls != 0 {
		t.Fatalf("resolver must not run without monitor access, got %d calls", resolverCalls)
	}
	if got := detail.Cameras[0].ThumbnailURL; got != "" {
		t.Fatalf("thumbnail must be withheld without monitor access, got %q", got)
	}
}

func TestBuildStoreDetailReportsMappingIssues(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10030, Name: "北京保利实验室门店", Status: 1, CityID: 1},
		Devices: []BusinessDevice{
			{ID: 7, TenantID: 10030, Name: "edge-1", HardwareID: "EDGE001", Category: "edge", Status: 1, OnlineStatus: 2},
			{ID: 22, TenantID: 10030, Name: "nvr-1", HardwareID: "NVR001", Category: "nvr", Status: 1, OnlineStatus: 2},
			{ID: 68, TenantID: 10030, ParentID: 22, Name: "摄像头1", HardwareID: "NVRCHANNEL:22-1", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1, OnlineStatus: 1},
			{ID: 69, TenantID: 10030, ParentID: 404, Name: "摄像头2", HardwareID: "NVRCHANNEL:404-2", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1, OnlineStatus: 2},
			{ID: 70, TenantID: 10030, ParentID: 22, Name: "摄像头3", HardwareID: "NVRCHANNEL:22-3", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1, OnlineStatus: 1},
		},
		Spaces: []BusinessSpace{
			{ID: 11, TenantID: 10030, Name: "治疗室1", Level: 2, Status: 0},
			{ID: 12, TenantID: 10030, Name: "治疗室2", Level: 2, Status: 1},
		},
		Relations: []BusinessAreaDeviceRelation{
			{ID: 1, AreaID: 11, DeviceID: 68, FunctionType: "camera"},
			{ID: 2, AreaID: 999, DeviceID: 404, FunctionType: "camera"},
			{ID: 3, AreaID: 12, DeviceID: 68, FunctionType: "camera"},
			{ID: 4, AreaID: 12, DeviceID: 70, FunctionType: "camera"},
			{ID: 5, AreaID: 999, DeviceID: 68, FunctionType: "camera"},
		},
	}

	detail := BuildStoreDetail(records, MonitorAccess{})

	assertIssueTypes(t, detail.Issues, []IssueType{
		IssueCameraBoundManySpaces,
		IssueInactiveBoundSpace,
		IssueMissingCamera,
		IssueMissingNVR,
		IssueMissingSpace,
		IssueOfflineCamera,
		IssueOfflineEdge,
		IssueOfflineNVR,
		IssueSpaceBoundManyCameras,
		IssueUnboundCamera,
	})
	assertIssueCount(t, detail.Issues, IssueMissingCamera, 1)
	assertIssueCount(t, detail.Issues, IssueMissingSpace, 1)
	if detail.Summary.WarningCount != len(detail.Issues) {
		t.Fatalf("warning count = %d, want %d", detail.Summary.WarningCount, len(detail.Issues))
	}
	if detail.Summary.OfflineDeviceCount != 3 {
		t.Fatalf("offline device count = %d, want 3", detail.Summary.OfflineDeviceCount)
	}
}

func TestBuildStoreDetailTreatsMissingSpaceBindingAsUnbound(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10035, Name: "缺空间绑定门店", Status: 1},
		Devices: []BusinessDevice{
			{ID: 68, TenantID: 10035, Name: "摄像头1", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1, OnlineStatus: 1},
		},
		Relations: []BusinessAreaDeviceRelation{
			{ID: 1, AreaID: 999, DeviceID: 68, FunctionType: "camera"},
		},
	}

	detail := BuildStoreDetail(records, MonitorAccess{})

	if detail.Summary.BoundCameraCount != 0 || detail.Summary.UnboundCameraCount != 1 {
		t.Fatalf("summary = %#v, want bound 0 and unbound 1", detail.Summary)
	}
	assertIssueCount(t, detail.Issues, IssueMissingSpace, 1)
	assertIssueCount(t, detail.Issues, IssueUnboundCamera, 1)
}

func TestBuildStoreDetailIgnoresConsultingAreaContainerRelation(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10062, Name: "空间容器关联门店", Status: 1},
		Devices: []BusinessDevice{
			{ID: 70, TenantID: 10062, Name: "摄像头 70", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1, OnlineStatus: 1},
		},
		Spaces: []BusinessSpace{
			{ID: 2387, TenantID: 10062, Name: "诊室区域", Status: 1},
			{ID: 2665, TenantID: 10062, ParentID: 2387, Name: "产研中心", Status: 1},
			{ID: 2667, TenantID: 10062, ParentID: 2665, Name: "产研中心1-2", Status: 1},
		},
		Relations: []BusinessAreaDeviceRelation{
			{ID: 1, AreaID: 2665, DeviceID: 70, FunctionType: "camera"},
			{ID: 2, AreaID: 2667, DeviceID: 70, FunctionType: "camera"},
		},
	}

	detail := BuildStoreDetail(records, MonitorAccess{})

	if len(detail.Relations) != 1 || detail.Relations[0].AreaID != 2667 {
		t.Fatalf("relations = %#v, want only area 2667", detail.Relations)
	}
	if detail.Summary.BoundCameraCount != 1 || detail.Summary.UnboundCameraCount != 0 {
		t.Fatalf("summary = %#v, want one bound camera", detail.Summary)
	}
	if len(detail.Cameras) != 1 || !reflect.DeepEqual(detail.Cameras[0].SpacePaths, []string{"诊室区域 / 产研中心 / 产研中心1-2"}) {
		t.Fatalf("camera paths = %#v", detail.Cameras)
	}
}

func TestBuildStoreDetailOnlyIncludesEnabledHikVisionNvrChannelCameras(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10061, Name: "摄像头范围门店", Status: 1},
		Devices: []BusinessDevice{
			{ID: 10, TenantID: 10061, Name: "有效通道", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1, OnlineStatus: 1},
			{ID: 11, TenantID: 10061, Name: "其他厂商", Category: "camera", Provider: "OtherProvider", Status: 1, OnlineStatus: 1},
			{ID: 12, TenantID: 10061, Name: "已停用通道", Category: "camera", Provider: "HikVisionNvrChannel", Status: 0, OnlineStatus: 1},
		},
		Spaces: []BusinessSpace{{ID: 20, TenantID: 10061, Name: "治疗室 1", Status: 1}},
		Relations: []BusinessAreaDeviceRelation{
			{ID: 1, AreaID: 20, DeviceID: 10, FunctionType: "camera"},
			{ID: 2, AreaID: 20, DeviceID: 11, FunctionType: "camera"},
			{ID: 3, AreaID: 20, DeviceID: 12, FunctionType: "camera"},
		},
	}

	detail := BuildStoreDetail(records, MonitorAccess{})

	if len(detail.Cameras) != 1 || detail.Cameras[0].ID != 10 {
		t.Fatalf("eligible cameras = %#v, want only camera 10", detail.Cameras)
	}
	if detail.Summary.CameraCount != 1 || detail.Summary.BoundCameraCount != 1 || detail.Summary.UnboundCameraCount != 0 {
		t.Fatalf("summary = %#v, want one bound eligible camera", detail.Summary)
	}
	if len(detail.Relations) != 1 || detail.Relations[0].DeviceID != 10 {
		t.Fatalf("relations = %#v, want only eligible camera relation", detail.Relations)
	}
}

func TestBuildStoreDetailIgnoresNonCameraRelations(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10060, Name: "真实关系类型门店", Status: 1},
		Devices: []BusinessDevice{
			{ID: 68, TenantID: 10060, Name: "安防摄像头", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1, OnlineStatus: 1},
			{ID: 69, TenantID: 10060, Name: "PAD", Category: "pad", Status: 1, OnlineStatus: 1},
			{ID: 70, TenantID: 10060, Name: "电视", Category: "tv", Status: 1, OnlineStatus: 1},
			{ID: 71, TenantID: 10060, Name: "蓝牙网关", Category: "bt_gateway", Status: 1, OnlineStatus: 1},
		},
		Spaces: []BusinessSpace{{ID: 12, TenantID: 10060, Name: "治疗室1", Level: 2, Status: 1}},
		Relations: []BusinessAreaDeviceRelation{
			{ID: 1, AreaID: 12, DeviceID: 68, FunctionType: "security_camera"},
			{ID: 2, AreaID: 12, DeviceID: 69, FunctionType: "pad"},
			{ID: 3, AreaID: 12, DeviceID: 70, FunctionType: "business_tv"},
			{ID: 4, AreaID: 12, DeviceID: 71, FunctionType: "bt_gateway"},
			{ID: 5, AreaID: 12, DeviceID: 999, FunctionType: "security_camera"},
		},
	}

	detail := BuildStoreDetail(records, MonitorAccess{})

	if len(detail.Relations) != 2 {
		t.Fatalf("camera relations = %#v, want only camera and missing-camera relations", detail.Relations)
	}
	if detail.Summary.BoundCameraCount != 1 || detail.Summary.UnboundCameraCount != 0 {
		t.Fatalf("summary = %#v, want one bound camera", detail.Summary)
	}
	assertIssueCount(t, detail.Issues, IssueMissingCamera, 1)
}

func TestBuildStoreDetailCountsCamerasOncePerDisplayedSpaceType(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10059, Name: "空间类型计数门店", Status: 1},
		Devices: []BusinessDevice{
			{ID: 10, TenantID: 10059, Category: "camera", Provider: "HikVisionNvrChannel", Status: 1},
			{ID: 20, TenantID: 10059, Category: "camera", Provider: "HikVisionNvrChannel", Status: 1},
		},
		Spaces: []BusinessSpace{
			{ID: 1, TenantID: 10059, Name: consultationSpaceType, Status: 1},
			{ID: 2, TenantID: 10059, ParentID: 1, Name: "面诊室 A", Level: 2, Status: 1},
			{ID: 3, TenantID: 10059, ParentID: 1, Name: "面诊室 B", Level: 2, Status: 1},
			{ID: 4, TenantID: 10059, Name: "治疗区域", Status: 1},
			{ID: 5, TenantID: 10059, ParentID: 4, Name: "治疗室 A", Level: 3, Status: 1},
		},
		Relations: []BusinessAreaDeviceRelation{
			{ID: 1, AreaID: 2, DeviceID: 10, FunctionType: "camera"},
			{ID: 2, AreaID: 3, DeviceID: 10, FunctionType: "camera"},
			{ID: 3, AreaID: 5, DeviceID: 20, FunctionType: "camera"},
		},
	}

	detail := BuildStoreDetail(records, MonitorAccess{})
	if detail.Summary.ConsultationCameraCount != 1 || detail.Summary.TreatmentCameraCount != 1 {
		t.Fatalf("space type counts = %#v, want consultation=1 treatment=1", detail.Summary)
	}
}

func TestBuildStoreDetailDeduplicatesIdenticalRelations(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10036, Name: "重复绑定门店", Status: 1},
		Devices: []BusinessDevice{
			{ID: 68, TenantID: 10036, Name: "摄像头1", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1, OnlineStatus: 1},
		},
		Spaces: []BusinessSpace{
			{ID: 12, TenantID: 10036, Name: "治疗室1", Level: 2, Status: 1},
		},
		Relations: []BusinessAreaDeviceRelation{
			{ID: 3, AreaID: 12, DeviceID: 68, FunctionType: "camera"},
			{ID: 1, AreaID: 12, DeviceID: 68, FunctionType: "camera"},
			{ID: 2, AreaID: 12, DeviceID: 68, FunctionType: "camera"},
		},
	}

	detail := BuildStoreDetail(records, MonitorAccess{})

	if len(detail.Relations) != 1 || detail.Relations[0].ID != 1 {
		t.Fatalf("relations = %#v, want one normalized relation with smallest id", detail.Relations)
	}
	space := findSpace(detail.Spaces, 12)
	if space == nil {
		t.Fatalf("flat spaces missing 12: %#v", detail.Spaces)
	}
	if space.BoundCameraCount != 1 || !reflect.DeepEqual(space.BoundCameraIDs, []int64{68}) {
		t.Fatalf("flat space binding = %#v, want camera 68 once", space)
	}
	if len(detail.SpaceTree) != 1 || detail.SpaceTree[0].BoundCameraCount != 1 || len(detail.SpaceTree[0].BoundCameras) != 1 {
		t.Fatalf("space tree binding = %#v, want camera 68 once", detail.SpaceTree)
	}
	assertIssueCount(t, detail.Issues, IssueCameraBoundManySpaces, 0)
	assertIssueCount(t, detail.Issues, IssueSpaceBoundManyCameras, 0)
	if detail.Summary.BoundCameraCount != 1 || detail.Summary.UnboundCameraCount != 0 {
		t.Fatalf("summary = %#v, want bound 1 and unbound 0", detail.Summary)
	}
}

func TestBuildStoreDetailAttachesSpacesWithMissingParentsToRoot(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10040, Name: "缺父级门店", Status: 1},
		Spaces: []BusinessSpace{
			{ID: 30, TenantID: 10040, ParentID: 404, Name: "孤儿床位", Level: 3, Status: 1},
			{ID: 10, TenantID: 10040, Name: "一级空间", Level: 1, Status: 1},
		},
	}

	detail := BuildStoreDetail(records, MonitorAccess{})

	if len(detail.SpaceTree) != 2 {
		t.Fatalf("root space count = %d, want 2: %#v", len(detail.SpaceTree), detail.SpaceTree)
	}
	if detail.SpaceTree[0].ID != 10 || detail.SpaceTree[1].ID != 30 {
		t.Fatalf("space order = %#v, want level 1 before missing-parent level 3", detail.SpaceTree)
	}
}

func TestParseNVRChannelHardwareID(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  *int
	}{
		{name: "valid", value: "NVRCHANNEL:22-12", want: intPtr(12)},
		{name: "trimmed", value: " NVRCHANNEL:22-1 ", want: intPtr(1)},
		{name: "wrong prefix", value: "CAMERA:22-1"},
		{name: "missing dash", value: "NVRCHANNEL:22"},
		{name: "zero", value: "NVRCHANNEL:22-0"},
		{name: "not number", value: "NVRCHANNEL:22-A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNVRChannelHardwareID(tt.value)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got %d, want nil", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("got %#v, want %d", got, *tt.want)
			}
		})
	}
}

func TestBuildStoreDetailKeepsStableSorting(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10050, Name: "排序门店", Status: 1},
		Devices: []BusinessDevice{
			{ID: 30, TenantID: 10050, Name: "camera-30", HardwareID: "NVRCHANNEL:20-2", ParentID: 20, Category: "camera", Provider: "HikVisionNvrChannel", Status: 1, OnlineStatus: 1},
			{ID: 10, TenantID: 10050, Name: "edge-10", Category: "edge", Status: 1, OnlineStatus: 1},
			{ID: 20, TenantID: 10050, Name: "nvr-20", Category: "nvr", Status: 1, OnlineStatus: 1},
			{ID: 31, TenantID: 10050, Name: "camera-31", HardwareID: "NVRCHANNEL:20-1", ParentID: 20, Category: "camera", Provider: "HikVisionNvrChannel", Status: 1, OnlineStatus: 1},
		},
		Spaces: []BusinessSpace{
			{ID: 12, TenantID: 10050, ParentID: 11, Name: "床位2", Level: 3, Status: 1, SortOrder: 2},
			{ID: 11, TenantID: 10050, ParentID: 10, Name: "治疗室1", Level: 2, Status: 1, SortOrder: 1},
			{ID: 10, TenantID: 10050, Name: "治疗区域", Level: 1, Status: 1, SortOrder: 1},
			{ID: 13, TenantID: 10050, ParentID: 11, Name: "床位1", Level: 3, Status: 1, SortOrder: 1},
		},
		Relations: []BusinessAreaDeviceRelation{
			{ID: 2, AreaID: 12, DeviceID: 30, FunctionType: "camera"},
			{ID: 1, AreaID: 13, DeviceID: 31, FunctionType: "camera"},
		},
	}

	detail := BuildStoreDetail(records, MonitorAccess{})

	gotCameras := []int64{detail.Cameras[0].ID, detail.Cameras[1].ID}
	if !reflect.DeepEqual(gotCameras, []int64{30, 31}) {
		t.Fatalf("camera order = %#v, want source normalized id order", gotCameras)
	}
	children := detail.SpaceTree[0].Children[0].Children
	gotSpaces := []int64{children[0].ID, children[1].ID}
	if !reflect.DeepEqual(gotSpaces, []int64{13, 12}) {
		t.Fatalf("space child order = %#v, want sort_order then id", gotSpaces)
	}
	if detail.Relations[0].ID != 1 || detail.Relations[1].ID != 2 {
		t.Fatalf("relation order = %#v", detail.Relations)
	}
}

func assertIssueTypes(t interface{ Fatalf(string, ...any) }, issues []Issue, expected []IssueType) {
	counts := map[IssueType]int{}
	for _, issue := range issues {
		counts[issue.Type]++
	}
	for _, issueType := range expected {
		if counts[issueType] == 0 {
			t.Fatalf("missing issue %s in %#v", issueType, issues)
		}
	}
}

func assertIssueCount(t interface{ Fatalf(string, ...any) }, issues []Issue, issueType IssueType, expected int) {
	count := 0
	for _, issue := range issues {
		if issue.Type == issueType {
			count++
		}
	}
	if count != expected {
		t.Fatalf("issue %s count = %d, want %d in %#v", issueType, count, expected, issues)
	}
}

func findSpace(spaces []Space, id int64) *Space {
	for i := range spaces {
		if spaces[i].ID == id {
			return &spaces[i]
		}
	}
	return nil
}

func intPtr(value int) *int {
	return &value
}
