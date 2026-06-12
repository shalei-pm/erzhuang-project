package storespace

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListEzvizAccounts(ctx context.Context) ([]EzvizAccount, error) {
	return s.repo.ListEzvizAccounts(ctx)
}

func (s *Service) CreateEzvizAccount(ctx context.Context, input CreateEzvizAccountInput) (*EzvizAccount, error) {
	if err := validateCreateEzvizAccountInput(input); err != nil {
		return nil, err
	}
	input.AccountName = strings.TrimSpace(input.AccountName)
	exists, err := s.repo.EzvizAccountNameExists(ctx, input.AccountName)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, &ValidationError{Fields: map[string]string{"account_name": "萤石云账号名称已存在"}}
	}
	return s.repo.CreateEzvizAccount(ctx, input)
}

func (s *Service) ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error) {
	return s.repo.ListStores(ctx, filters)
}

func (s *Service) GetStore(ctx context.Context, id int64) (*Store, error) {
	return s.repo.GetStore(ctx, id)
}

func (s *Service) CreateStore(ctx context.Context, input CreateStoreInput) (*Store, error) {
	if err := validateCreateStoreInput(input); err != nil {
		return nil, err
	}
	input.City = strings.TrimSpace(input.City)
	input.Name = strings.TrimSpace(input.Name)
	input.ExternalOrgID = strings.TrimSpace(input.ExternalOrgID)
	input.DesignPlanUploadID = strings.TrimSpace(input.DesignPlanUploadID)

	if err := s.ensureNoExactDuplicate(ctx, input.Name, 0); err != nil {
		return nil, err
	}
	for index := range input.Recorders {
		input.Recorders[index].DeviceCode = normalizeDeviceCode(input.Recorders[index].DeviceCode)
		if input.Recorders[index].DeviceCode == "" {
			continue
		}
		exists, err := s.repo.DeviceCodeExists(ctx, input.Recorders[index].DeviceCode, 0)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, &ValidationError{Fields: map[string]string{
				"recorders[" + strconv.Itoa(index) + "].device_code": "录像机设备编码已存在",
			}}
		}
	}

	return s.repo.CreateStore(ctx, input)
}

func (s *Service) DeleteStore(ctx context.Context, id int64) error {
	return s.repo.DeleteStore(ctx, id)
}

func (s *Service) AddRecorder(ctx context.Context, storeID int64, input AddRecorderInput) (*Store, error) {
	if err := validateAddRecorderInput(input); err != nil {
		return nil, err
	}
	store, err := s.repo.GetStore(ctx, storeID)
	if err != nil {
		return nil, err
	}
	if len(store.Recorders) >= 3 {
		return nil, &ValidationError{Fields: map[string]string{"recorders": "单门店最多 3 台录像机"}}
	}

	input.DeviceCode = normalizeDeviceCode(input.DeviceCode)
	exists, err := s.repo.DeviceCodeExists(ctx, input.DeviceCode, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, &ValidationError{Fields: map[string]string{"device_code": "录像机设备编码已存在"}}
	}
	return s.repo.AddRecorder(ctx, storeID, input)
}

func (s *Service) DeleteRecorder(ctx context.Context, recorderID int64) error {
	return s.repo.DeleteRecorder(ctx, recorderID)
}

func (s *Service) CheckDuplicate(ctx context.Context, request DuplicateCheckRequest) (DuplicateCheckResult, error) {
	if strings.TrimSpace(request.Name) == "" {
		return DuplicateCheckResult{}, &ValidationError{Fields: map[string]string{"name": "门店名称必填"}}
	}
	return s.repo.CheckDuplicate(ctx, strings.TrimSpace(request.Name), request.ExcludeStoreID)
}

func (s *Service) FindOrCreateArea(ctx context.Context, input AreaLookup) (*Area, error) {
	input.NumberText = strings.TrimSpace(input.NumberText)
	if input.Source == "" {
		input.Source = AreaSourceManual
	}
	number, err := validateAreaLookup(input)
	if err != nil {
		return nil, err
	}
	return s.repo.FindOrCreateArea(ctx, input, number)
}

func (s *Service) ScanRecorderChannels(ctx context.Context, recorderID int64) error {
	return ErrNotImplemented
}

func (s *Service) RecognizeRecorderChannels(ctx context.Context, recorderID int64) error {
	return ErrNotImplemented
}

var ErrNotImplemented = errors.New("not implemented")

func (s *Service) ensureNoExactDuplicate(ctx context.Context, name string, excludeStoreID int64) error {
	result, err := s.repo.CheckDuplicate(ctx, name, excludeStoreID)
	if err != nil {
		return err
	}
	if result.ExactMatch != nil {
		return &ValidationError{Fields: map[string]string{"name": "已存在同名门店"}}
	}
	return nil
}
