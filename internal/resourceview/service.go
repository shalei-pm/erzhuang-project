package resourceview

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func BuildStoreDetail(records StoreRecords, access MonitorAccess) StoreDetail {
	spaces := normalizedSpaces(records.Spaces)
	devices := normalizedDevices(records.Devices)
	relations := normalizedRelations(records.Relations)
	cameras := buildCameras(devices, relations, spaces)
	issues := buildIssues(devices, spaces, relations, cameras)
	summary := buildSummary(devices, spaces, relations, issues)

	return StoreDetail{
		TenantID:       records.Tenant.ID,
		StoreName:      strings.TrimSpace(records.Tenant.Name),
		HospitalName:   strings.TrimSpace(records.Tenant.HospitalName),
		CityID:         records.Tenant.CityID,
		CityName:       cityName(records.Tenant.CityID),
		Summary:        summary,
		Edges:          devicesByCategory(devices, "edge"),
		NVRs:           devicesByCategory(devices, "nvr"),
		Cameras:        cameras,
		Spaces:         spaces,
		Relations:      relations,
		SpaceTree:      buildSpaceTree(spaces, cameras, relations),
		DeviceTree:     buildDeviceTree(devices, cameras),
		Issues:         issues,
		CanViewMonitor: access.CanViewMonitor,
		MonitorURL:     strings.TrimSpace(access.MonitorURL),
	}
}

func parseNVRChannelHardwareID(value string) *int {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "NVRCHANNEL:") {
		return nil
	}
	no, err := strconv.Atoi(parts[1])
	if err != nil || no <= 0 {
		return nil
	}
	return &no
}

func normalizedSpaces(input []BusinessSpace) []Space {
	out := make([]Space, 0, len(input))
	for _, space := range input {
		out = append(out, Space{
			ID:         space.ID,
			TenantID:   space.TenantID,
			ParentID:   space.ParentID,
			Name:       strings.TrimSpace(space.Name),
			Code:       strings.TrimSpace(space.Code),
			Level:      space.Level,
			Status:     space.Status,
			StatusText: enabledText(space.Status),
			DictID:     space.DictID,
			SortOrder:  space.SortOrder,
		})
	}
	sortSpaces(out)
	return out
}

func normalizedDevices(input []BusinessDevice) []Device {
	out := make([]Device, 0, len(input))
	for _, device := range input {
		if device.DeletedAt != nil {
			continue
		}
		out = append(out, Device{
			ID:           device.ID,
			ParentID:     device.ParentID,
			TenantID:     device.TenantID,
			Name:         strings.TrimSpace(device.Name),
			HardwareID:   strings.TrimSpace(device.HardwareID),
			SN:           strings.TrimSpace(device.SN),
			IP:           strings.TrimSpace(device.IP),
			Category:     strings.TrimSpace(device.Category),
			Provider:     strings.TrimSpace(device.Provider),
			Status:       device.Status,
			StatusText:   enabledText(device.Status),
			OnlineStatus: device.OnlineStatus,
			OnlineText:   onlineText(device.OnlineStatus),
			ExtSummary:   extSummary(device.ExtParams),
			HeartbeatAt:  formatOptionalTime(device.HeartbeatAt),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if categoryRank(out[i].Category) == categoryRank(out[j].Category) {
			return out[i].ID < out[j].ID
		}
		return categoryRank(out[i].Category) < categoryRank(out[j].Category)
	})
	return out
}

func normalizedRelations(input []BusinessAreaDeviceRelation) []AreaDeviceRelation {
	out := make([]AreaDeviceRelation, 0, len(input))
	for _, relation := range input {
		out = append(out, AreaDeviceRelation{
			ID:           relation.ID,
			DeviceID:     relation.DeviceID,
			AreaID:       relation.AreaID,
			FunctionType: strings.TrimSpace(relation.FunctionType),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func buildCameras(devices []Device, relations []AreaDeviceRelation, spaces []Space) []Camera {
	nvrs := map[int64]Device{}
	for _, device := range devices {
		if device.Category == "nvr" {
			nvrs[device.ID] = device
		}
	}
	spaceByID := spacesByID(spaces)
	pathsByCameraID := map[int64][]string{}
	for _, relation := range relations {
		if _, ok := spaceByID[relation.AreaID]; ok {
			pathsByCameraID[relation.DeviceID] = append(pathsByCameraID[relation.DeviceID], spacePath(spaceByID, relation.AreaID))
		}
	}
	for cameraID := range pathsByCameraID {
		sort.Strings(pathsByCameraID[cameraID])
	}

	cameras := []Camera{}
	for _, device := range devices {
		if device.Category != "camera" {
			continue
		}
		camera := Camera{
			Device:     device,
			ChannelNo:  parseNVRChannelHardwareID(device.HardwareID),
			NVRID:      device.ParentID,
			SpacePaths: append([]string{}, pathsByCameraID[device.ID]...),
		}
		if nvr, ok := nvrs[device.ParentID]; ok {
			camera.NVRName = nvr.Name
		}
		cameras = append(cameras, camera)
	}
	sort.Slice(cameras, func(i, j int) bool { return cameras[i].ID < cameras[j].ID })
	return cameras
}

func buildSummary(devices []Device, spaces []Space, relations []AreaDeviceRelation, issues []Issue) StoreSummary {
	cameraIDs := map[int64]struct{}{}
	boundCameraIDs := map[int64]struct{}{}
	summary := StoreSummary{StoreCount: 1, SpaceCount: len(spaces), WarningCount: len(issues)}

	for _, device := range devices {
		switch device.Category {
		case "edge":
			summary.EdgeCount++
		case "nvr":
			summary.NVRCount++
		case "camera":
			summary.CameraCount++
			cameraIDs[device.ID] = struct{}{}
		}
		if device.OnlineStatus != 1 {
			summary.OfflineDeviceCount++
		}
	}
	for _, relation := range relations {
		if _, ok := cameraIDs[relation.DeviceID]; ok {
			boundCameraIDs[relation.DeviceID] = struct{}{}
		}
	}
	summary.BoundCameraCount = len(boundCameraIDs)
	summary.UnboundCameraCount = summary.CameraCount - summary.BoundCameraCount
	return summary
}

func buildIssues(devices []Device, spaces []Space, relations []AreaDeviceRelation, cameras []Camera) []Issue {
	issues := []Issue{}
	deviceByID := devicesByID(devices)
	spaceByID := spacesByID(spaces)
	boundsByCamera := map[int64]int{}
	boundsBySpace := map[int64]int{}

	for _, relation := range relations {
		device, hasDevice := deviceByID[relation.DeviceID]
		space, hasSpace := spaceByID[relation.AreaID]
		if !hasDevice || device.Category != "camera" {
			issues = append(issues, Issue{
				Severity:   IssueSeverityError,
				Type:       IssueMissingCamera,
				Message:    fmt.Sprintf("绑定关系 %d 指向不存在的摄像头", relation.ID),
				EntityType: "relation",
				EntityID:   relation.ID,
			})
			continue
		}
		if !hasSpace {
			issues = append(issues, Issue{
				Severity:   IssueSeverityError,
				Type:       IssueMissingCamera,
				Message:    fmt.Sprintf("绑定关系 %d 指向不存在的空间", relation.ID),
				EntityType: "relation",
				EntityID:   relation.ID,
			})
			continue
		}
		boundsByCamera[relation.DeviceID]++
		boundsBySpace[relation.AreaID]++
		if space.Status != 1 {
			issues = append(issues, Issue{
				Severity:   IssueSeverityWarn,
				Type:       IssueInactiveBoundSpace,
				Message:    fmt.Sprintf("空间 %s 已停用但仍绑定摄像头", space.Name),
				EntityType: "space",
				EntityID:   space.ID,
			})
		}
	}

	for _, camera := range cameras {
		if camera.NVRID > 0 {
			if nvr, ok := deviceByID[camera.NVRID]; !ok || nvr.Category != "nvr" {
				issues = append(issues, Issue{
					Severity:   IssueSeverityError,
					Type:       IssueMissingNVR,
					Message:    fmt.Sprintf("摄像头 %s 的父级 NVR 不存在", camera.Name),
					EntityType: "camera",
					EntityID:   camera.ID,
				})
			}
		}
		if len(camera.SpacePaths) == 0 {
			issues = append(issues, Issue{
				Severity:   IssueSeverityWarn,
				Type:       IssueUnboundCamera,
				Message:    fmt.Sprintf("摄像头 %s 未绑定空间", camera.Name),
				EntityType: "camera",
				EntityID:   camera.ID,
			})
		}
		if camera.OnlineStatus != 1 {
			issues = append(issues, Issue{
				Severity:   IssueSeverityWarn,
				Type:       IssueOfflineCamera,
				Message:    fmt.Sprintf("摄像头 %s 离线", camera.Name),
				EntityType: "camera",
				EntityID:   camera.ID,
			})
		}
	}

	for _, device := range devices {
		switch device.Category {
		case "edge":
			if device.OnlineStatus != 1 {
				issues = append(issues, Issue{
					Severity:   IssueSeverityWarn,
					Type:       IssueOfflineEdge,
					Message:    fmt.Sprintf("工控机 %s 离线", device.Name),
					EntityType: "device",
					EntityID:   device.ID,
				})
			}
		case "nvr":
			if device.OnlineStatus != 1 {
				issues = append(issues, Issue{
					Severity:   IssueSeverityWarn,
					Type:       IssueOfflineNVR,
					Message:    fmt.Sprintf("NVR %s 离线", device.Name),
					EntityType: "device",
					EntityID:   device.ID,
				})
			}
		}
	}

	for cameraID, count := range boundsByCamera {
		if count > 1 {
			issues = append(issues, Issue{
				Severity:   IssueSeverityInfo,
				Type:       IssueCameraBoundManySpaces,
				Message:    "同一摄像头绑定了多个空间",
				EntityType: "camera",
				EntityID:   cameraID,
			})
		}
	}
	for spaceID, count := range boundsBySpace {
		if count > 1 {
			issues = append(issues, Issue{
				Severity:   IssueSeverityInfo,
				Type:       IssueSpaceBoundManyCameras,
				Message:    "同一空间绑定了多个摄像头",
				EntityType: "space",
				EntityID:   spaceID,
			})
		}
	}

	sortIssues(issues)
	return issues
}

func buildSpaceTree(spaces []Space, cameras []Camera, relations []AreaDeviceRelation) []SpaceNode {
	cameraByID := map[int64]Camera{}
	for _, camera := range cameras {
		cameraByID[camera.ID] = camera
	}
	boundBySpace := map[int64][]Camera{}
	for _, relation := range relations {
		if camera, ok := cameraByID[relation.DeviceID]; ok {
			boundBySpace[relation.AreaID] = append(boundBySpace[relation.AreaID], camera)
		}
	}
	for spaceID := range boundBySpace {
		sort.Slice(boundBySpace[spaceID], func(i, j int) bool { return boundBySpace[spaceID][i].ID < boundBySpace[spaceID][j].ID })
	}

	spaceIDs := map[int64]struct{}{}
	for _, space := range spaces {
		spaceIDs[space.ID] = struct{}{}
	}
	childrenByParent := map[int64][]Space{}
	for _, space := range spaces {
		parentID := space.ParentID
		if parentID != 0 {
			if _, ok := spaceIDs[parentID]; !ok {
				parentID = 0
			}
		}
		childrenByParent[parentID] = append(childrenByParent[parentID], space)
	}
	for parentID := range childrenByParent {
		sortSpaces(childrenByParent[parentID])
	}

	var build func(parentID int64) []SpaceNode
	build = func(parentID int64) []SpaceNode {
		nodes := []SpaceNode{}
		for _, space := range childrenByParent[parentID] {
			boundCameras := append([]Camera{}, boundBySpace[space.ID]...)
			cameraIDs := make([]int64, 0, len(boundCameras))
			for _, camera := range boundCameras {
				cameraIDs = append(cameraIDs, camera.ID)
			}
			space.BoundCameraIDs = cameraIDs
			space.BoundCameraCount = len(cameraIDs)
			nodes = append(nodes, SpaceNode{
				Space:        space,
				BoundCameras: boundCameras,
				Children:     build(space.ID),
			})
		}
		return nodes
	}
	return build(0)
}

func buildDeviceTree(devices []Device, cameras []Camera) DeviceTree {
	nvrNodes := []NVRNode{}
	for _, device := range devices {
		if device.Category != "nvr" {
			continue
		}
		node := NVRNode{Device: device}
		for _, camera := range cameras {
			if camera.NVRID == device.ID {
				node.Cameras = append(node.Cameras, camera)
			}
		}
		nvrNodes = append(nvrNodes, node)
	}
	sort.Slice(nvrNodes, func(i, j int) bool { return nvrNodes[i].ID < nvrNodes[j].ID })
	return DeviceTree{Edges: devicesByCategory(devices, "edge"), NVRs: nvrNodes}
}

func devicesByCategory(devices []Device, category string) []Device {
	out := []Device{}
	for _, device := range devices {
		if device.Category == category {
			out = append(out, device)
		}
	}
	return out
}

func devicesByID(devices []Device) map[int64]Device {
	out := map[int64]Device{}
	for _, device := range devices {
		out[device.ID] = device
	}
	return out
}

func spacesByID(spaces []Space) map[int64]Space {
	out := map[int64]Space{}
	for _, space := range spaces {
		out[space.ID] = space
	}
	return out
}

func spacePath(spaces map[int64]Space, id int64) string {
	names := []string{}
	for id != 0 {
		space, ok := spaces[id]
		if !ok {
			break
		}
		names = append([]string{space.Name}, names...)
		id = space.ParentID
	}
	return strings.Join(names, " / ")
}

func sortSpaces(spaces []Space) {
	sort.Slice(spaces, func(i, j int) bool {
		if spaces[i].Level == spaces[j].Level {
			if spaces[i].SortOrder == spaces[j].SortOrder {
				return spaces[i].ID < spaces[j].ID
			}
			return spaces[i].SortOrder < spaces[j].SortOrder
		}
		return spaces[i].Level < spaces[j].Level
	})
}

func categoryRank(category string) int {
	switch category {
	case "edge":
		return 1
	case "nvr":
		return 2
	case "camera":
		return 3
	default:
		return 9
	}
}

func enabledText(status int) string {
	if status == 1 {
		return "启用"
	}
	return "停用"
}

func onlineText(status int) string {
	if status == 1 {
		return "在线"
	}
	return "离线"
}

func cityName(cityID int64) string {
	if cityID == 0 {
		return ""
	}
	return fmt.Sprintf("城市 %d", cityID)
}

func extSummary(value string) string {
	return strings.TrimSpace(value)
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

func sortIssues(issues []Issue) {
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Type == issues[j].Type {
			if issues[i].EntityType == issues[j].EntityType {
				return issues[i].EntityID < issues[j].EntityID
			}
			return issues[i].EntityType < issues[j].EntityType
		}
		return issues[i].Type < issues[j].Type
	})
}
