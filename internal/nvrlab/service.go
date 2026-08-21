package nvrlab

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
)

const maxPlaybackWindow = 30 * time.Minute

type AuthorizationClient interface {
	CreateStreamURL(ctx context.Context, cameraID int64, request StreamSessionRequest) (string, error)
}

type Service struct {
	repository    resourceview.Repository
	authorization AuthorizationClient
}

func NewService(repository resourceview.Repository, authorization AuthorizationClient) *Service {
	return &Service{repository: repository, authorization: authorization}
}

func (s *Service) ListCameras(ctx context.Context, tenantID int64) (CameraListResponse, error) {
	records, err := s.storeRecords(ctx, tenantID)
	if err != nil {
		return CameraListResponse{}, err
	}
	cameras := camerasFromRecords(records)
	return CameraListResponse{TenantID: records.Tenant.ID, StoreName: records.Tenant.Name, Cameras: cameras}, nil
}

func (s *Service) CreateSession(ctx context.Context, tenantID, cameraID int64, request StreamSessionRequest) (StreamSessionResponse, error) {
	if err := validateStreamSessionRequest(request); err != nil {
		return StreamSessionResponse{}, err
	}
	response, err := s.ListCameras(ctx, tenantID)
	if err != nil {
		return StreamSessionResponse{}, err
	}
	found := false
	for _, camera := range response.Cameras {
		if camera.ID == cameraID {
			found = true
			break
		}
	}
	if !found {
		return StreamSessionResponse{}, ErrCameraNotFound
	}
	if s.authorization == nil {
		return StreamSessionResponse{}, ErrNotConfigured
	}
	streamURL, err := s.authorization.CreateStreamURL(ctx, cameraID, request)
	if err != nil {
		return StreamSessionResponse{}, err
	}
	if strings.TrimSpace(streamURL) == "" {
		return StreamSessionResponse{}, ErrAuthorizationFailed
	}
	return StreamSessionResponse{URL: streamURL, Mode: request.Mode}, nil
}

func (s *Service) storeRecords(ctx context.Context, tenantID int64) (resourceview.StoreRecords, error) {
	if tenantID != ExperimentTenantID || s == nil || s.repository == nil {
		return resourceview.StoreRecords{}, ErrExperimentNotFound
	}
	records, err := s.repository.GetStoreRecords(ctx, ExperimentTenantID)
	if errors.Is(err, resourceview.ErrNotFound) {
		return resourceview.StoreRecords{}, ErrExperimentNotFound
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

func camerasFromRecords(records resourceview.StoreRecords) []Camera {
	spaces := make(map[int64]resourceview.BusinessSpace, len(records.Spaces))
	for _, space := range records.Spaces {
		spaces[space.ID] = space
	}
	relationSpaces := make(map[int64][]resourceview.BusinessSpace)
	for _, relation := range records.Relations {
		if space, ok := spaces[relation.AreaID]; ok && space.ParentID != 2387 {
			relationSpaces[relation.DeviceID] = append(relationSpaces[relation.DeviceID], space)
		}
	}

	cameras := make([]Camera, 0)
	for _, device := range records.Devices {
		if !isEligibleCamera(device) {
			continue
		}
		camera := Camera{ID: device.ID, Name: strings.TrimSpace(device.Name)}
		if space := preferredSpace(relationSpaces[device.ID]); space != nil {
			camera.SpaceName = strings.TrimSpace(space.Name)
			camera.SpaceType = spaceType(*space, spaces)
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
