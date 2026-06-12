package storespace

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

type Service struct {
	repo    Repository
	scanner ChannelScanner
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func NewServiceWithScanner(repo Repository, scanner ChannelScanner) *Service {
	return &Service{repo: repo, scanner: scanner}
}

type ChannelScanner interface {
	ScanRecorderChannels(ctx context.Context, account EzvizAccount, recorder Recorder) ([]ScannedChannel, error)
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

func (s *Service) SaveDesignPlan(ctx context.Context, storeID int64, input SaveDesignPlanInput) (*Store, error) {
	input.UploadID = strings.TrimSpace(input.UploadID)
	input.PDFFileName = strings.TrimSpace(input.PDFFileName)
	input.OriginalPDFPath = strings.TrimSpace(input.OriginalPDFPath)
	input.PreviewImagePath = strings.TrimSpace(input.PreviewImagePath)
	input.ThumbnailPath = strings.TrimSpace(input.ThumbnailPath)
	for index := range input.Areas {
		input.Areas[index].DisplayName = strings.TrimSpace(input.Areas[index].DisplayName)
		input.Areas[index].NumberText = strings.TrimSpace(input.Areas[index].NumberText)
	}
	if err := validateSaveDesignPlanInput(input); err != nil {
		return nil, err
	}
	return s.repo.SaveDesignPlan(ctx, storeID, input)
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

func (s *Service) ScanRecorderChannels(ctx context.Context, recorderID int64) (*Recorder, error) {
	if s.scanner == nil {
		return nil, ErrNotImplemented
	}
	recorder, err := s.repo.GetRecorder(ctx, recorderID)
	if err != nil {
		return nil, err
	}
	if recorder.EzvizAccountID == 0 {
		return nil, &ValidationError{Fields: map[string]string{"ezviz_account_id": "缺少萤石云账号"}}
	}
	account, err := s.repo.GetEzvizAccount(ctx, recorder.EzvizAccountID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &ValidationError{Fields: map[string]string{"ezviz_account_id": "找不到萤石云账号"}}
		}
		return nil, err
	}
	scannedChannels, err := s.scanner.ScanRecorderChannels(ctx, *account, *recorder)
	if err != nil {
		return nil, err
	}
	channelInputs := make([]ChannelInput, 0, len(scannedChannels))
	for _, channel := range scannedChannels {
		if channel.ChannelNo <= 0 || !channel.Active {
			continue
		}
		channelInputs = append(channelInputs, ChannelInput{
			ChannelNo:   channel.ChannelNo,
			ChannelName: strings.TrimSpace(channel.ChannelName),
			IsActive:    true,
		})
	}
	return s.repo.ReplaceRecorderChannels(ctx, recorderID, channelInputs)
}

func (s *Service) ConfirmChannel(ctx context.Context, channelID int64, input ChannelConfirmationInput) (*Store, error) {
	input.AreaNumber = strings.TrimSpace(input.AreaNumber)
	if input.SceneType == "" {
		input.SceneType = SceneTypeUnknown
	}
	number, err := validateChannelConfirmationInput(input)
	if err != nil {
		return nil, err
	}
	if input.AreaType != "" {
		input.AreaNumber = strconv.Itoa(number)
		input.SceneType = SceneType(input.AreaType)
	}
	return s.repo.ConfirmChannel(ctx, channelID, input)
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
