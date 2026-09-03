package nvrmonitor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
)

const maxPlaybackWindow = time.Hour
const consultingAreaContainerID int64 = 2387

type AuthorizationClient interface {
	CreateStreamURL(ctx context.Context, cameraID int64, request StreamSessionRequest) (string, error)
}

// SnapshotStore is an optional, private thumbnail source populated by the
// one-shot NVR backfill Job. Object existence is the source of truth: no
// database row is needed to expose a successfully captured thumbnail.
type SnapshotStore interface {
	Open(ctx context.Context, tenantID int64, cameraID int64) (io.ReadCloser, string, error)
}

// SnapshotWriter persists a browser-captured JPEG under the same private,
// deterministic key used by SnapshotStore. It deliberately has no database
// dependency: object existence is the backfill record.
type SnapshotWriter interface {
	Save(ctx context.Context, tenantID int64, cameraID int64, body io.Reader) error
}

type SnapshotDeleter interface {
	Delete(ctx context.Context, tenantID int64, cameraID int64) error
}

// SnapshotRollback is returned after a snapshot replacement. Callers must
// run it when a later step, such as audit persistence, fails.
type SnapshotRollback interface {
	Rollback(ctx context.Context) error
}

// SnapshotTransactionalWriter is required for audited refreshes. It prevents
// a failed audit write from leaving a half-completed thumbnail replacement.
type SnapshotTransactionalWriter interface {
	SaveSnapshotWithRollback(ctx context.Context, tenantID int64, cameraID int64, body io.Reader) (SnapshotRollback, error)
}

type Service struct {
	repository    resourceview.Repository
	authorization AuthorizationClient
	snapshots     SnapshotStore
}

func NewService(repository resourceview.Repository, authorization AuthorizationClient) *Service {
	return &Service{repository: repository, authorization: authorization}
}

func NewServiceWithSnapshotStore(repository resourceview.Repository, authorization AuthorizationClient, snapshots SnapshotStore) *Service {
	return &Service{repository: repository, authorization: authorization, snapshots: snapshots}
}

func (s *Service) ListStores(ctx context.Context) (MonitorStoresResponse, error) {
	if s == nil || s.repository == nil {
		return MonitorStoresResponse{}, ErrNotConfigured
	}
	records, err := s.repository.ListNVRMonitorStores(ctx)
	if err != nil {
		return MonitorStoresResponse{}, err
	}
	byCity := map[string][]StoreInfo{}
	for _, record := range records {
		cameras := camerasFromRecords(record, false)
		if len(cameras) == 0 {
			continue
		}
		city := cityLabel(record.Tenant.CityID)
		byCity[city] = append(byCity[city], StoreInfo{
			ExternalOrgID:        strconv.FormatInt(record.Tenant.ID, 10),
			StoreName:            strings.TrimSpace(record.Tenant.Name),
			City:                 city,
			AvailableCameraCount: len(cameras),
		})
	}
	cities := make([]string, 0, len(byCity))
	for city := range byCity {
		cities = append(cities, city)
	}
	sort.Strings(cities)
	result := MonitorStoresResponse{Cities: make([]StoreCityGroup, 0, len(cities))}
	for _, city := range cities {
		stores := byCity[city]
		sort.Slice(stores, func(i, j int) bool {
			if stores[i].StoreName != stores[j].StoreName {
				return stores[i].StoreName < stores[j].StoreName
			}
			return stores[i].ExternalOrgID < stores[j].ExternalOrgID
		})
		result.Cities = append(result.Cities, StoreCityGroup{City: city, Stores: stores})
	}
	return result, nil
}

func (s *Service) GetCameras(ctx context.Context, externalOrgID string) (CameraListResponse, error) {
	records, err := s.storeRecords(ctx, externalOrgID)
	if err != nil {
		return CameraListResponse{}, err
	}
	return CameraListResponse{
		ExternalOrgID: strconv.FormatInt(records.Tenant.ID, 10),
		TenantID:      records.Tenant.ID,
		StoreName:     strings.TrimSpace(records.Tenant.Name),
		City:          cityLabel(records.Tenant.CityID),
		Cameras:       camerasFromRecords(records, s.snapshots != nil),
	}, nil
}

func (s *Service) OpenSnapshot(ctx context.Context, externalOrgID string, cameraID int64) (io.ReadCloser, string, error) {
	records, err := s.storeRecords(ctx, externalOrgID)
	if err != nil {
		return nil, "", err
	}
	if !containsCamera(camerasFromRecords(records, false), cameraID) {
		return nil, "", ErrCameraNotFound
	}
	if s.snapshots == nil {
		return nil, "", ErrSnapshotNotFound
	}
	reader, contentType, err := s.snapshots.Open(ctx, records.Tenant.ID, cameraID)
	if err != nil {
		return nil, "", ErrSnapshotNotFound
	}
	return reader, contentType, nil
}

// HasSnapshot is a storage-only existence check for another authenticated
// project view. Camera eligibility and access checks remain at its own route.
func (s *Service) HasSnapshot(ctx context.Context, tenantID int64, cameraID int64) bool {
	if s == nil || s.snapshots == nil || tenantID <= 0 || cameraID <= 0 {
		return false
	}
	reader, _, err := s.snapshots.Open(ctx, tenantID, cameraID)
	if err != nil {
		return false
	}
	return reader.Close() == nil
}

func SnapshotURL(tenantID int64, cameraID int64) string {
	return fmt.Sprintf("/api/h5/nvr-monitor/orgs/%d/cameras/%d/snapshot", tenantID, cameraID)
}

func (s *Service) SaveSnapshot(ctx context.Context, externalOrgID string, cameraID int64, body io.Reader) error {
	records, err := s.storeRecords(ctx, externalOrgID)
	if err != nil {
		return err
	}
	if !containsCamera(camerasFromRecords(records, false), cameraID) {
		return ErrCameraNotFound
	}
	writer, ok := s.snapshots.(SnapshotWriter)
	if !ok || writer == nil {
		return ErrNotConfigured
	}
	return writer.Save(ctx, records.Tenant.ID, cameraID, body)
}

func (s *Service) SaveSnapshotWithRollback(ctx context.Context, externalOrgID string, cameraID int64, body io.Reader) (SnapshotRollback, error) {
	records, err := s.storeRecords(ctx, externalOrgID)
	if err != nil {
		return nil, err
	}
	if !containsCamera(camerasFromRecords(records, false), cameraID) {
		return nil, ErrCameraNotFound
	}
	writer, ok := s.snapshots.(SnapshotTransactionalWriter)
	if !ok || writer == nil {
		return nil, ErrSnapshotTransactionUnavailable
	}
	return writer.SaveSnapshotWithRollback(ctx, records.Tenant.ID, cameraID, body)
}

func (s *Service) CreateSession(ctx context.Context, externalOrgID string, cameraID int64, request StreamSessionRequest) (StreamSessionResponse, error) {
	response, _, err := s.CreateSessionWithAuditTarget(ctx, externalOrgID, cameraID, request)
	return response, err
}

// CreateSessionWithAuditTarget returns the server-resolved camera context that
// is needed to make a media-view audit entry understandable to reviewers.
func (s *Service) CreateSessionWithAuditTarget(ctx context.Context, externalOrgID string, cameraID int64, request StreamSessionRequest) (StreamSessionResponse, CameraAuditTarget, error) {
	if err := validateStreamSessionRequest(request); err != nil {
		return StreamSessionResponse{}, CameraAuditTarget{}, err
	}
	response, err := s.GetCameras(ctx, externalOrgID)
	if err != nil {
		return StreamSessionResponse{}, CameraAuditTarget{}, err
	}
	var target CameraAuditTarget
	found := false
	for _, camera := range response.Cameras {
		if camera.ID != cameraID {
			continue
		}
		target = CameraAuditTarget{
			ExternalOrgID: response.ExternalOrgID,
			StoreName:     response.StoreName,
			CameraID:      camera.ID,
			CameraName:    camera.Name,
			SpaceType:     camera.SpaceType,
			SpaceName:     camera.SpaceName,
		}
		found = true
		break
	}
	if !found {
		return StreamSessionResponse{}, CameraAuditTarget{}, ErrCameraNotFound
	}
	if s.authorization == nil {
		return StreamSessionResponse{}, target, ErrNotConfigured
	}
	streamURL, err := s.authorization.CreateStreamURL(ctx, cameraID, request)
	if err != nil {
		return StreamSessionResponse{}, target, err
	}
	if strings.TrimSpace(streamURL) == "" {
		return StreamSessionResponse{}, target, ErrAuthorizationFailed
	}
	return StreamSessionResponse{URL: streamURL, Mode: request.Mode}, target, nil
}

func (s *Service) storeRecords(ctx context.Context, externalOrgID string) (resourceview.StoreRecords, error) {
	if s == nil || s.repository == nil {
		return resourceview.StoreRecords{}, ErrNotConfigured
	}
	tenantID, err := strconv.ParseInt(strings.TrimSpace(externalOrgID), 10, 64)
	if err != nil || tenantID <= 0 {
		return resourceview.StoreRecords{}, ErrStoreNotFound
	}
	records, err := s.repository.GetNVRMonitorStoreRecords(ctx, tenantID)
	if errors.Is(err, resourceview.ErrNotFound) {
		return resourceview.StoreRecords{}, ErrStoreNotFound
	}
	if err != nil {
		return resourceview.StoreRecords{}, err
	}
	return records, nil
}

func validateStreamSessionRequest(request StreamSessionRequest) error {
	switch request.Mode {
	case ModeLive:
		if request.StartTime != 0 || request.EndTime != 0 {
			return ErrInvalidStreamMode
		}
	case ModePlayback:
		if request.StartTime <= 0 || request.EndTime <= request.StartTime || request.EndTime-request.StartTime > int64(maxPlaybackWindow/time.Second) {
			return ErrInvalidPlaybackWindow
		}
	default:
		return ErrInvalidStreamMode
	}
	return nil
}

func containsCamera(cameras []Camera, cameraID int64) bool {
	for _, camera := range cameras {
		if camera.ID == cameraID {
			return true
		}
	}
	return false
}

func camerasFromRecords(records resourceview.StoreRecords, snapshotsConfigured bool) []Camera {
	spaces := map[int64]resourceview.BusinessSpace{}
	for _, space := range records.Spaces {
		spaces[space.ID] = space
	}
	relationSpaces := map[int64][]resourceview.BusinessSpace{}
	for _, relation := range records.Relations {
		space, ok := spaces[relation.AreaID]
		if !ok || space.ParentID == consultingAreaContainerID {
			continue
		}
		relationSpaces[relation.DeviceID] = append(relationSpaces[relation.DeviceID], space)
	}

	cameras := make([]Camera, 0)
	for _, device := range records.Devices {
		if !isEligibleCamera(device) {
			continue
		}
		camera := Camera{ID: device.ID, Name: strings.TrimSpace(device.Name), ThumbnailKind: "unassigned"}
		if space := preferredSpace(relationSpaces[device.ID]); space != nil {
			camera.SpaceName = strings.TrimSpace(space.Name)
			camera.SpaceType = spaceType(*space, spaces)
			camera.ThumbnailKind = resourceview.CameraThumbnailKind(camera.SpaceType, camera.SpaceName, space.Level)
		}
		if snapshotsConfigured {
			camera.ThumbnailURL = SnapshotURL(records.Tenant.ID, device.ID)
		} else if channelNo := nvrChannelNo(device.HardwareID); channelNo > 0 && records.LegacyCameraSnapshots[channelNo] != "" {
			camera.ThumbnailURL = fmt.Sprintf("/api/store-space-resource-view/stores/%d/cameras/%d/snapshot", records.Tenant.ID, device.ID)
		}
		cameras = append(cameras, camera)
	}
	sort.Slice(cameras, func(i, j int) bool {
		if cameras[i].SpaceType != cameras[j].SpaceType {
			return cameras[i].SpaceType < cameras[j].SpaceType
		}
		if cameras[i].SpaceName != cameras[j].SpaceName {
			return cameras[i].SpaceName < cameras[j].SpaceName
		}
		return cameras[i].ID < cameras[j].ID
	})
	return cameras
}

func isEligibleCamera(device resourceview.BusinessDevice) bool {
	return device.Category == "camera" && device.Provider == "HikVisionNvrChannel" && device.Status == 1 && device.DeletedAt == nil
}

func preferredSpace(spaces []resourceview.BusinessSpace) *resourceview.BusinessSpace {
	if len(spaces) == 0 {
		return nil
	}
	sort.SliceStable(spaces, func(i, j int) bool { return spaces[i].ID < spaces[j].ID })
	return &spaces[0]
}

func spaceType(space resourceview.BusinessSpace, spaces map[int64]resourceview.BusinessSpace) string {
	if space.Level == 3 {
		return "治疗室"
	}
	parent, ok := spaces[space.ParentID]
	if !ok {
		return ""
	}
	return strings.TrimSpace(parent.Name)
}

func nvrChannelNo(hardwareID string) int {
	parts := strings.Split(strings.TrimSpace(hardwareID), "-")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "NVRCHANNEL:") {
		return 0
	}
	channelNo, err := strconv.Atoi(parts[1])
	if err != nil || channelNo <= 0 {
		return 0
	}
	return channelNo
}

func cityLabel(cityID int64) string {
	if cityID <= 0 {
		return "未分城市"
	}
	return resourceview.CityName(cityID)
}
