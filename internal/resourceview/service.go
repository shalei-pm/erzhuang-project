package resourceview

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListStores(ctx context.Context, filters StoreFilters, access func(int64) MonitorAccess) (StoreListResult, error) {
	if s == nil || s.repo == nil {
		return StoreListResult{}, errors.New("resource view repository is not configured")
	}
	filters = normalizeStoreFilters(filters)
	records, err := s.repo.ListStores(ctx, filters)
	if err != nil {
		return StoreListResult{}, err
	}

	details := make([]StoreDetail, 0, len(records))
	for _, record := range records {
		details = append(details, BuildStoreDetail(record, MonitorAccess{}))
	}

	result := StoreListResult{
		Page:     filters.Page,
		PageSize: filters.PageSize,
		Total:    len(details),
		Summary:  summarizeStoreDetails(details),
		Cities:   cityOptions(details),
	}

	start := (filters.Page - 1) * filters.PageSize
	if start >= len(details) {
		return result, nil
	}
	end := start + filters.PageSize
	if end > len(details) {
		end = len(details)
	}
	for index, detail := range details[start:end] {
		monitorAccess := MonitorAccess{}
		if access != nil {
			monitorAccess = access(detail.TenantID)
		}
		result.Items = append(result.Items, storeListItem(detail, records[start+index], monitorAccess))
	}
	return result, nil
}

func (s *Service) GetStore(ctx context.Context, tenantID int64, access MonitorAccess) (StoreDetail, error) {
	if s == nil || s.repo == nil {
		return StoreDetail{}, errors.New("resource view repository is not configured")
	}
	records, err := s.repo.GetStoreRecords(ctx, tenantID)
	if err != nil {
		return StoreDetail{}, err
	}
	return BuildStoreDetail(records, access), nil
}

func BuildStoreDetail(records StoreRecords, access MonitorAccess) StoreDetail {
	spaces := normalizedSpaces(records.Spaces)
	devices := normalizedDevices(records.Devices)
	relations := cameraRelevantRelations(normalizedRelations(records.Relations), devices)
	cameras := buildCameras(devices, relations, spaces)
	bindings := buildValidBindings(spaces, cameras, relations)
	spaces = enrichSpaces(spaces, bindings)
	issues := buildIssues(devices, spaces, relations, cameras, bindings)
	summary := buildSummary(devices, spaces, bindings, issues)

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
		SpaceTree:      buildSpaceTree(spaces, bindings),
		DeviceTree:     buildDeviceTree(devices, cameras),
		Issues:         issues,
		CanViewMonitor: access.CanViewMonitor,
		MonitorURL:     strings.TrimSpace(access.MonitorURL),
	}
}

func normalizeStoreFilters(filters StoreFilters) StoreFilters {
	filters.Query = strings.TrimSpace(filters.Query)
	if filters.Page <= 0 {
		filters.Page = 1
	}
	if filters.PageSize <= 0 {
		filters.PageSize = 20
	}
	if filters.PageSize > 100 {
		filters.PageSize = 100
	}
	return filters
}

func summarizeStoreDetails(details []StoreDetail) StoreSummary {
	summary := StoreSummary{StoreCount: len(details)}
	for _, detail := range details {
		summary.EdgeCount += detail.Summary.EdgeCount
		summary.NVRCount += detail.Summary.NVRCount
		summary.CameraCount += detail.Summary.CameraCount
		summary.SpaceCount += detail.Summary.SpaceCount
		summary.BoundCameraCount += detail.Summary.BoundCameraCount
		summary.UnboundCameraCount += detail.Summary.UnboundCameraCount
		summary.OfflineDeviceCount += detail.Summary.OfflineDeviceCount
		summary.WarningCount += detail.Summary.WarningCount
	}
	return summary
}

func cityOptions(details []StoreDetail) []CityOption {
	counts := map[int64]int{}
	for _, detail := range details {
		if detail.CityID > 0 {
			counts[detail.CityID]++
		}
	}
	ids := make([]int64, 0, len(counts))
	for cityID := range counts {
		ids = append(ids, cityID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	options := make([]CityOption, 0, len(ids))
	for _, cityID := range ids {
		options = append(options, CityOption{CityID: cityID, Name: cityName(cityID), Count: counts[cityID]})
	}
	return options
}

func storeListItem(detail StoreDetail, records StoreRecords, access MonitorAccess) StoreListItem {
	return StoreListItem{
		TenantID:           detail.TenantID,
		StoreName:          detail.StoreName,
		HospitalName:       detail.HospitalName,
		CityID:             detail.CityID,
		CityName:           detail.CityName,
		EdgeCount:          detail.Summary.EdgeCount,
		NVRCount:           detail.Summary.NVRCount,
		CameraCount:        detail.Summary.CameraCount,
		SpaceCount:         detail.Summary.SpaceCount,
		BoundCameraCount:   detail.Summary.BoundCameraCount,
		UnboundCameraCount: detail.Summary.UnboundCameraCount,
		OfflineDeviceCount: detail.Summary.OfflineDeviceCount,
		WarningCount:       detail.Summary.WarningCount,
		CamerasFullyBound:  detail.Summary.CameraCount > 0 && detail.Summary.UnboundCameraCount == 0,
		UpdatedAt:          latestRelationUpdatedAt(records.Relations),
		CanViewMonitor:     access.CanViewMonitor,
		MonitorURL:         strings.TrimSpace(access.MonitorURL),
	}
}

func latestRelationUpdatedAt(relations []BusinessAreaDeviceRelation) string {
	var latest time.Time
	for _, relation := range relations {
		if relation.CreatedAt.After(latest) {
			latest = relation.CreatedAt
		}
	}
	if latest.IsZero() {
		return ""
	}
	return latest.Format(time.RFC3339)
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
	byKey := map[relationKey]AreaDeviceRelation{}
	for _, relation := range input {
		normalized := AreaDeviceRelation{
			ID:           relation.ID,
			DeviceID:     relation.DeviceID,
			AreaID:       relation.AreaID,
			FunctionType: strings.TrimSpace(relation.FunctionType),
		}
		key := relationKey{
			deviceID:     normalized.DeviceID,
			areaID:       normalized.AreaID,
			functionType: normalized.FunctionType,
		}
		existing, ok := byKey[key]
		if !ok || normalized.ID < existing.ID {
			byKey[key] = normalized
		}
	}
	out := make([]AreaDeviceRelation, 0, len(byKey))
	for _, relation := range byKey {
		out = append(out, relation)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func cameraRelevantRelations(relations []AreaDeviceRelation, devices []Device) []AreaDeviceRelation {
	devicesByID := devicesByID(devices)
	filtered := make([]AreaDeviceRelation, 0, len(relations))
	for _, relation := range relations {
		if device, ok := devicesByID[relation.DeviceID]; ok {
			if device.Category == "camera" {
				filtered = append(filtered, relation)
			}
			continue
		}
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(relation.FunctionType)), "camera") {
			filtered = append(filtered, relation)
		}
	}
	return filtered
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
	seenSpaceByCamera := map[int64]map[int64]struct{}{}
	for _, relation := range relations {
		if _, ok := spaceByID[relation.AreaID]; ok {
			if seenSpaceByCamera[relation.DeviceID] == nil {
				seenSpaceByCamera[relation.DeviceID] = map[int64]struct{}{}
			}
			if _, ok := seenSpaceByCamera[relation.DeviceID][relation.AreaID]; ok {
				continue
			}
			seenSpaceByCamera[relation.DeviceID][relation.AreaID] = struct{}{}
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

type relationKey struct {
	deviceID     int64
	areaID       int64
	functionType string
}

type cameraSpaceKey struct {
	cameraID int64
	spaceID  int64
}

type bindingIndex struct {
	camerasBySpace   map[int64][]Camera
	cameraIDsBySpace map[int64][]int64
	spaceIDsByCamera map[int64][]int64
	boundCameraIDs   map[int64]struct{}
}

func buildValidBindings(spaces []Space, cameras []Camera, relations []AreaDeviceRelation) bindingIndex {
	spaceByID := spacesByID(spaces)
	cameraByID := map[int64]Camera{}
	for _, camera := range cameras {
		cameraByID[camera.ID] = camera
	}
	bindings := bindingIndex{
		camerasBySpace:   map[int64][]Camera{},
		cameraIDsBySpace: map[int64][]int64{},
		spaceIDsByCamera: map[int64][]int64{},
		boundCameraIDs:   map[int64]struct{}{},
	}
	seen := map[cameraSpaceKey]struct{}{}
	for _, relation := range relations {
		camera, hasCamera := cameraByID[relation.DeviceID]
		if !hasCamera {
			continue
		}
		if _, hasSpace := spaceByID[relation.AreaID]; !hasSpace {
			continue
		}
		key := cameraSpaceKey{cameraID: relation.DeviceID, spaceID: relation.AreaID}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		bindings.camerasBySpace[relation.AreaID] = append(bindings.camerasBySpace[relation.AreaID], camera)
		bindings.cameraIDsBySpace[relation.AreaID] = append(bindings.cameraIDsBySpace[relation.AreaID], camera.ID)
		bindings.spaceIDsByCamera[camera.ID] = append(bindings.spaceIDsByCamera[camera.ID], relation.AreaID)
		bindings.boundCameraIDs[camera.ID] = struct{}{}
	}
	for spaceID := range bindings.camerasBySpace {
		sort.Slice(bindings.camerasBySpace[spaceID], func(i, j int) bool {
			return bindings.camerasBySpace[spaceID][i].ID < bindings.camerasBySpace[spaceID][j].ID
		})
		sort.Slice(bindings.cameraIDsBySpace[spaceID], func(i, j int) bool {
			return bindings.cameraIDsBySpace[spaceID][i] < bindings.cameraIDsBySpace[spaceID][j]
		})
	}
	for cameraID := range bindings.spaceIDsByCamera {
		sort.Slice(bindings.spaceIDsByCamera[cameraID], func(i, j int) bool {
			return bindings.spaceIDsByCamera[cameraID][i] < bindings.spaceIDsByCamera[cameraID][j]
		})
	}
	return bindings
}

func enrichSpaces(spaces []Space, bindings bindingIndex) []Space {
	out := make([]Space, len(spaces))
	for i, space := range spaces {
		out[i] = space
		out[i].BoundCameraIDs = append([]int64{}, bindings.cameraIDsBySpace[space.ID]...)
		out[i].BoundCameraCount = len(out[i].BoundCameraIDs)
	}
	return out
}

func buildSummary(devices []Device, spaces []Space, bindings bindingIndex, issues []Issue) StoreSummary {
	cameraIDs := map[int64]struct{}{}
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
	for cameraID := range bindings.boundCameraIDs {
		if _, ok := cameraIDs[cameraID]; ok {
			summary.BoundCameraCount++
		}
	}
	summary.UnboundCameraCount = summary.CameraCount - summary.BoundCameraCount
	return summary
}

func buildIssues(devices []Device, spaces []Space, relations []AreaDeviceRelation, cameras []Camera, bindings bindingIndex) []Issue {
	issues := []Issue{}
	deviceByID := devicesByID(devices)
	spaceByID := spacesByID(spaces)
	inactiveBoundSpaceIDs := map[int64]struct{}{}

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
				Type:       IssueMissingSpace,
				Message:    fmt.Sprintf("绑定关系 %d 指向不存在的空间", relation.ID),
				EntityType: "relation",
				EntityID:   relation.ID,
			})
			continue
		}
		if space.Status != 1 {
			if _, ok := inactiveBoundSpaceIDs[space.ID]; !ok {
				inactiveBoundSpaceIDs[space.ID] = struct{}{}
				issues = append(issues, Issue{
					Severity:   IssueSeverityWarn,
					Type:       IssueInactiveBoundSpace,
					Message:    fmt.Sprintf("空间 %s 已停用但仍绑定摄像头", space.Name),
					EntityType: "space",
					EntityID:   space.ID,
				})
			}
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

	for cameraID, spaceIDs := range bindings.spaceIDsByCamera {
		if len(spaceIDs) > 1 {
			issues = append(issues, Issue{
				Severity:   IssueSeverityInfo,
				Type:       IssueCameraBoundManySpaces,
				Message:    "同一摄像头绑定了多个空间",
				EntityType: "camera",
				EntityID:   cameraID,
			})
		}
	}
	for spaceID, cameraIDs := range bindings.cameraIDsBySpace {
		if len(cameraIDs) > 1 {
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

func buildSpaceTree(spaces []Space, bindings bindingIndex) []SpaceNode {
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
			boundCameras := append([]Camera{}, bindings.camerasBySpace[space.ID]...)
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
