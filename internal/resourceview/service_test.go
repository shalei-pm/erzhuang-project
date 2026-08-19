package resourceview

import (
	"reflect"
	"testing"
	"time"
)

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
