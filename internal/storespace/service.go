package storespace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

type Service struct {
	repo          Repository
	scanner       ChannelScanner
	recognizer    ChannelRecognizer
	snapshotStore SnapshotStore
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func NewServiceWithScanner(repo Repository, scanner ChannelScanner) *Service {
	return &Service{repo: repo, scanner: scanner}
}

func NewServiceWithScannerAndRecognizer(repo Repository, scanner ChannelScanner, recognizer ChannelRecognizer) *Service {
	return &Service{repo: repo, scanner: scanner, recognizer: recognizer}
}

func (s *Service) UseSnapshotStore(store SnapshotStore) {
	s.snapshotStore = store
}

func (s *Service) OpenChannelSnapshot(ctx context.Context, name string) (io.ReadCloser, string, error) {
	if s.snapshotStore == nil {
		return nil, "", ErrNotFound
	}
	return s.snapshotStore.Open(ctx, name)
}

func (s *Service) DiagnoseChannelSnapshot(ctx context.Context, name string) SnapshotDiagnostics {
	diagnostics := SnapshotDiagnostics{
		Code:         "snapshot_open_ok",
		Stage:        "open_snapshot",
		AssetStore:   assets.ModeFromEnv(),
		SnapshotName: strings.TrimSpace(name),
		SnapshotKey:  snapshotKeyForDiagnostics(name),
		Exists:       true,
	}
	if s.snapshotStore == nil {
		diagnostics.Code = "snapshot_store_not_configured"
		diagnostics.Exists = false
		diagnostics.Detail = "snapshot store is not configured"
		return diagnostics
	}
	reader, _, err := s.snapshotStore.Open(ctx, name)
	if err != nil {
		diagnostics.Exists = false
		if errors.Is(err, ErrNotFound) {
			diagnostics.Code = "snapshot_not_found"
		} else {
			diagnostics.Code = "snapshot_open_failed"
			diagnostics.Detail = sanitizeDiagnosticDetail(err.Error())
		}
		return diagnostics
	}
	if reader != nil {
		_ = reader.Close()
	}
	return diagnostics
}

type ChannelScanner interface {
	ScanRecorderChannels(ctx context.Context, account EzvizAccount, recorder Recorder) ([]ScannedChannel, error)
	CaptureChannel(ctx context.Context, account EzvizAccount, recorder Recorder, channel Channel) (ChannelSnapshotInput, error)
	LiveAddress(ctx context.Context, account EzvizAccount, recorder Recorder, channelNo int, code string) (LiveAddressResult, error)
}

type ChannelRecognizer interface {
	RecognizeChannel(ctx context.Context, imageURL string) (ChannelRecognitionResult, error)
}

type LiveAddressInput struct {
	AccountID    int64  `json:"ezviz_account_id"`
	AccountName  string `json:"account_name"`
	DeviceSerial string `json:"device_serial"`
	ChannelNo    int    `json:"channel_no"`
	Code         string `json:"code"`
}

type LiveAddressResult struct {
	URL        string `json:"url"`
	URLID      string `json:"url_id"`
	ExpireTime string `json:"expire_time"`
	Protocol   string `json:"protocol"`
}

func (s *Service) ListEzvizAccounts(ctx context.Context) ([]EzvizAccount, error) {
	return s.repo.ListEzvizAccounts(ctx)
}

func (s *Service) GetLiveAddress(ctx context.Context, input LiveAddressInput) (LiveAddressResult, error) {
	if s.scanner == nil {
		return LiveAddressResult{}, ErrNotImplemented
	}
	deviceSerial := strings.ToUpper(strings.TrimSpace(input.DeviceSerial))
	if deviceSerial == "" {
		return LiveAddressResult{}, &ValidationError{Fields: map[string]string{"device_serial": "录像机设备编码必填"}}
	}
	if input.ChannelNo <= 0 {
		return LiveAddressResult{}, &ValidationError{Fields: map[string]string{"channel_no": "通道号必须大于 0"}}
	}
	account := EzvizAccount{ID: input.AccountID, AccountName: strings.TrimSpace(input.AccountName)}
	if account.ID > 0 {
		stored, err := s.repo.GetEzvizAccount(ctx, account.ID)
		if err != nil {
			return LiveAddressResult{}, err
		}
		account = *stored
	}
	if strings.TrimSpace(account.AccountName) == "" {
		return LiveAddressResult{}, &ValidationError{Fields: map[string]string{"ezviz_account_id": "请选择萤石云账号区域"}}
	}
	return s.scanner.LiveAddress(ctx, account, Recorder{DeviceCode: deviceSerial}, input.ChannelNo, strings.TrimSpace(input.Code))
}

func (s *Service) SyncEzvizAccountNames(ctx context.Context, accountNames []string) error {
	seen := map[string]struct{}{}
	for _, accountName := range accountNames {
		cleanName := strings.TrimSpace(accountName)
		if cleanName == "" {
			continue
		}
		if _, ok := seen[cleanName]; ok {
			continue
		}
		seen[cleanName] = struct{}{}
		if err := s.repo.UpsertEzvizAccountName(ctx, cleanName); err != nil {
			return err
		}
	}
	return nil
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

func (s *Service) GetStoreDesignPlanData(ctx context.Context, id int64) (*Store, error) {
	return s.repo.GetStoreDesignPlanData(ctx, id)
}

func (s *Service) GetStoreChannelData(ctx context.Context, id int64) (*Store, error) {
	return s.repo.GetStoreChannelData(ctx, id)
}

func (s *Service) ExportChannelMappingExcel(ctx context.Context, storeID int64) (*ChannelMappingExport, error) {
	store, err := s.repo.GetStore(ctx, storeID)
	if err != nil {
		return nil, err
	}
	rows := channelMappingExportRows(*store)
	if len(rows) == 0 {
		return nil, &ValidationError{Fields: map[string]string{"channels": "当前门店暂无可导出的有效通道"}}
	}
	content, err := buildChannelMappingExcel(ctx, rows, s.snapshotStore)
	if err != nil {
		return nil, err
	}
	return &ChannelMappingExport{
		FileName:    exportFileName(store.Name, time.Now()),
		Content:     content,
		ContentType: channelMappingExcelContentType,
	}, nil
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

func (s *Service) UpdateStoreBasicInfo(ctx context.Context, id int64, input UpdateStoreBasicInfoInput) (*Store, error) {
	if err := validateUpdateStoreBasicInfoInput(input); err != nil {
		return nil, err
	}
	input.City = strings.TrimSpace(input.City)
	input.Name = strings.TrimSpace(input.Name)
	input.ExternalOrgID = strings.TrimSpace(input.ExternalOrgID)
	if err := s.ensureNoExactDuplicate(ctx, input.Name, id); err != nil {
		return nil, err
	}
	return s.repo.UpdateStoreBasicInfo(ctx, id, input)
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

func (s *Service) DeleteChannel(ctx context.Context, channelID int64) (*Store, error) {
	return s.repo.DeleteChannel(ctx, channelID)
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

func (s *Service) ProbeRecognizeChannel(ctx context.Context, recorderID int64, input ProbeRecognizeChannelInput) (ProbeRecognizeChannelResult, error) {
	if s.scanner == nil {
		return ProbeRecognizeChannelResult{}, ErrNotImplemented
	}
	if input.ChannelNo <= 0 {
		return ProbeRecognizeChannelResult{}, &ValidationError{Fields: map[string]string{"channel_no": "通道号必须大于 0"}}
	}
	recorder, err := s.repo.GetRecorder(ctx, recorderID)
	if err != nil {
		return ProbeRecognizeChannelResult{}, err
	}
	if recorder.EzvizAccountID == 0 {
		return ProbeRecognizeChannelResult{}, &ValidationError{Fields: map[string]string{"ezviz_account_id": "缺少萤石云账号"}}
	}
	account, err := s.repo.GetEzvizAccount(ctx, recorder.EzvizAccountID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ProbeRecognizeChannelResult{}, &ValidationError{Fields: map[string]string{"ezviz_account_id": "找不到萤石云账号"}}
		}
		return ProbeRecognizeChannelResult{}, err
	}

	probeChannel := Channel{
		RecorderID:  recorder.ID,
		ChannelNo:   input.ChannelNo,
		ChannelName: fmt.Sprintf("通道%d", input.ChannelNo),
		Status:      ChannelStatusPendingRecognition,
		IsActive:    true,
		SceneType:   SceneTypeUnknown,
	}
	channelStarted := time.Now()
	captureStarted := time.Now()
	snapshot, err := s.scanner.CaptureChannel(ctx, *account, *recorder, probeChannel)
	captureMS := elapsedMilliseconds(captureStarted)
	if err != nil {
		return ProbeRecognizeChannelResult{
			Active:  false,
			Message: err.Error(),
		}, nil
	}
	channel, err := s.repo.UpsertRecorderChannel(ctx, recorderID, ChannelInput{
		ChannelNo:   input.ChannelNo,
		ChannelName: probeChannel.ChannelName,
		IsActive:    true,
	})
	if err != nil {
		return ProbeRecognizeChannelResult{}, err
	}

	recognitionImageURL := firstNonEmpty(snapshot.FullImagePath, snapshot.ThumbnailPath)
	if s.snapshotStore != nil && strings.TrimSpace(recognitionImageURL) != "" {
		localURL, err := s.snapshotStore.SaveRemote(ctx, recognitionImageURL)
		if err != nil {
			updated, saveErr := s.repo.SaveChannelSnapshot(ctx, channel.ID, ChannelSnapshotInput{
				RecognitionResult: channelRecognitionErrorJSON(err, captureMS, 0, elapsedMilliseconds(channelStarted)),
				CountAttempt:      true,
			})
			if saveErr != nil {
				return ProbeRecognizeChannelResult{}, saveErr
			}
			return ProbeRecognizeChannelResult{Channel: updated, Active: true, Message: err.Error()}, nil
		}
		snapshot.ThumbnailPath = localURL
		snapshot.FullImagePath = localURL
		snapshot.FullImageExpiresAt = nil
	}
	snapshot.CountAttempt = true
	snapshot.RecognitionResult = channelRecognitionStatusJSON("captured", "", captureMS, 0, elapsedMilliseconds(channelStarted))
	if s.recognizer != nil && !isConfirmedChannelStatus(channel.Status) {
		recognitionStarted := time.Now()
		result, err := s.recognizer.RecognizeChannel(ctx, recognitionImageURL)
		recognitionMS := elapsedMilliseconds(recognitionStarted)
		if err != nil {
			snapshot.Status = ChannelStatusRecognitionFailed
			snapshot.RecognitionResult = channelRecognitionErrorJSON(err, captureMS, recognitionMS, elapsedMilliseconds(channelStarted))
		} else {
			applyChannelRecognition(&snapshot, result, captureMS, recognitionMS, elapsedMilliseconds(channelStarted))
		}
	}
	updated, err := s.repo.SaveChannelSnapshot(ctx, channel.ID, snapshot)
	if err != nil {
		return ProbeRecognizeChannelResult{}, err
	}
	return ProbeRecognizeChannelResult{Channel: updated, Active: true}, nil
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
		if input.AreaType == AreaTypeVIPTreatment && number == 0 {
			input.AreaNumber = ""
		} else {
			input.AreaNumber = strconv.Itoa(number)
		}
		input.SceneType = SceneType(input.AreaType)
	}
	return s.repo.ConfirmChannel(ctx, channelID, input)
}

func (s *Service) UnlockChannelForEdit(ctx context.Context, channelID int64) (*Channel, error) {
	return s.repo.UnlockChannelForEdit(ctx, channelID)
}

func (s *Service) RecognizeRecorderChannels(ctx context.Context, recorderID int64) (*Recorder, error) {
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
	channels := make([]Channel, 0, len(recorder.Channels))
	for _, channel := range recorder.Channels {
		if !channel.IsActive || channel.Status == ChannelStatusInactive || isConfirmedChannelStatus(channel.Status) {
			continue
		}
		channels = append(channels, channel)
	}
	for index, channel := range channels {
		if _, err := s.recognizeChannel(ctx, *account, *recorder, channel); err != nil {
			return nil, err
		}
		if index < len(channels)-1 {
			time.Sleep(1200 * time.Millisecond)
		}
	}
	return s.repo.GetRecorder(ctx, recorderID)
}

func (s *Service) RecognizeChannel(ctx context.Context, channelID int64) (*Channel, error) {
	if s.scanner == nil {
		return nil, ErrNotImplemented
	}
	channel, recorder, account, err := s.repo.GetChannelContext(ctx, channelID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !channel.IsActive || channel.Status == ChannelStatusInactive {
		return nil, &ValidationError{Fields: map[string]string{"channel": "通道已失效，无法识别"}}
	}
	if isConfirmedChannelStatus(channel.Status) {
		return nil, &ValidationError{Fields: map[string]string{"channel": "通道已确认，如需重新识别请先点击编辑"}}
	}
	return s.recognizeChannel(ctx, *account, *recorder, *channel)
}

func (s *Service) recognizeChannel(ctx context.Context, account EzvizAccount, recorder Recorder, channel Channel) (*Channel, error) {
	channelStarted := time.Now()
	captureStarted := time.Now()
	snapshot, err := s.scanner.CaptureChannel(ctx, account, recorder, channel)
	captureMS := elapsedMilliseconds(captureStarted)
	if err != nil {
		return s.repo.SaveChannelSnapshot(ctx, channel.ID, ChannelSnapshotInput{
			RecognitionResult: channelRecognitionErrorJSON(err, captureMS, 0, elapsedMilliseconds(channelStarted)),
			CountAttempt:      true,
		})
	}
	recognitionImageURL := firstNonEmpty(snapshot.FullImagePath, snapshot.ThumbnailPath)
	if s.snapshotStore != nil && strings.TrimSpace(recognitionImageURL) != "" {
		localURL, err := s.snapshotStore.SaveRemote(ctx, recognitionImageURL)
		if err != nil {
			return s.repo.SaveChannelSnapshot(ctx, channel.ID, ChannelSnapshotInput{
				RecognitionResult: channelRecognitionErrorJSON(err, captureMS, 0, elapsedMilliseconds(channelStarted)),
				CountAttempt:      true,
			})
		}
		snapshot.ThumbnailPath = localURL
		snapshot.FullImagePath = localURL
		snapshot.FullImageExpiresAt = nil
	}
	snapshot.CountAttempt = true
	snapshot.RecognitionResult = channelRecognitionStatusJSON("captured", "", captureMS, 0, elapsedMilliseconds(channelStarted))
	if s.recognizer != nil && !isConfirmedChannelStatus(channel.Status) {
		recognitionStarted := time.Now()
		result, err := s.recognizer.RecognizeChannel(ctx, recognitionImageURL)
		recognitionMS := elapsedMilliseconds(recognitionStarted)
		if err != nil {
			snapshot.Status = ChannelStatusRecognitionFailed
			snapshot.RecognitionResult = channelRecognitionErrorJSON(err, captureMS, recognitionMS, elapsedMilliseconds(channelStarted))
		} else {
			applyChannelRecognition(&snapshot, result, captureMS, recognitionMS, elapsedMilliseconds(channelStarted))
		}
	}
	return s.repo.SaveChannelSnapshot(ctx, channel.ID, snapshot)
}

func (s *Service) RefreshChannelSnapshot(ctx context.Context, channelID int64) (*Channel, error) {
	if s.scanner == nil {
		return nil, ErrNotImplemented
	}
	channel, recorder, account, err := s.repo.GetChannelContext(ctx, channelID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !channel.IsActive || channel.Status == ChannelStatusInactive {
		return nil, &ValidationError{Fields: map[string]string{"channel": "通道已失效，无法刷新截图"}}
	}
	snapshot, err := s.scanner.CaptureChannel(ctx, *account, *recorder, *channel)
	if err != nil {
		return nil, err
	}
	imageURL := firstNonEmpty(snapshot.FullImagePath, snapshot.ThumbnailPath)
	if s.snapshotStore != nil && strings.TrimSpace(imageURL) != "" {
		localURL, err := s.snapshotStore.SaveRemote(ctx, imageURL)
		if err != nil {
			return nil, err
		}
		snapshot.ThumbnailPath = localURL
		snapshot.FullImagePath = localURL
		snapshot.FullImageExpiresAt = nil
	}
	snapshot.Status = ""
	snapshot.SceneType = ""
	snapshot.AreaType = ""
	snapshot.AreaNumberText = ""
	snapshot.AreaNote = ""
	snapshot.RecognitionResult = ""
	snapshot.CountAttempt = false
	return s.repo.SaveChannelSnapshot(ctx, channel.ID, snapshot)
}

func (s *Service) RefreshH5ChannelSnapshot(ctx context.Context, channelID int64) (string, error) {
	channel, err := s.RefreshChannelSnapshot(ctx, channelID)
	if err != nil {
		return "", err
	}
	return firstNonEmpty(channel.ThumbnailURL, channel.FullImageURL), nil
}

var ErrNotImplemented = errors.New("not implemented")

func channelRecognitionStatusJSON(status string, message string, captureMS int64, recognitionMS int64, totalMS int64) string {
	payload := map[string]any{
		"status":         status,
		"capture_ms":     captureMS,
		"recognition_ms": recognitionMS,
		"total_ms":       totalMS,
	}
	if strings.TrimSpace(message) != "" {
		payload["message"] = strings.TrimSpace(message)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func channelRecognitionErrorJSON(err error, captureMS int64, recognitionMS int64, totalMS int64) string {
	message := "截图失败"
	if err != nil {
		message = fmt.Sprintf("%v", err)
	}
	status := "capture_failed"
	if captureMS > 0 {
		status = "recognition_failed"
	}
	return channelRecognitionStatusJSON(status, message, captureMS, recognitionMS, totalMS)
}

func applyChannelRecognition(snapshot *ChannelSnapshotInput, result ChannelRecognitionResult, captureMS int64, recognitionMS int64, totalMS int64) {
	sceneType := normalizeRecognitionSceneType(result.SceneType)
	areaType := normalizeRecognitionAreaType(result.AreaType)
	number := strings.TrimSpace(result.AreaNumber)
	if areaType != "" {
		sceneType = SceneType(areaType)
	}
	snapshot.Status = ChannelStatusPendingConfirmation
	snapshot.SceneType = sceneType
	snapshot.AreaType = areaType
	if onlyDigits(number) {
		snapshot.AreaNumberText = number
	} else if areaType == "" {
		snapshot.AreaNote = recognitionSceneNote(sceneType, number)
	}
	payload := map[string]any{
		"status":          "recognized",
		"provider":        strings.TrimSpace(result.Provider),
		"scene_type":      sceneType,
		"area_type":       areaType,
		"area_number":     number,
		"card_text":       strings.TrimSpace(result.CardText),
		"decision_source": normalizeRecognitionDecisionSource(result.DecisionSource),
		"confidence":      normalizeRecognitionConfidence(result.Confidence),
		"needs_review":    result.NeedsReview,
		"raw_notes":       strings.TrimSpace(result.RawNotes),
		"capture_ms":      captureMS,
		"recognition_ms":  recognitionMS,
		"total_ms":        totalMS,
	}
	if strings.TrimSpace(result.RawResult) != "" {
		payload["raw_result"] = json.RawMessage(result.RawResult)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		snapshot.RecognitionResult = channelRecognitionStatusJSON("recognized", "", captureMS, recognitionMS, totalMS)
		return
	}
	snapshot.RecognitionResult = string(data)
}

func recognitionSceneNote(sceneType SceneType, recognizedNumber string) string {
	note := strings.TrimSpace(recognizedNumber)
	if note != "" && !onlyDigits(note) {
		return note
	}
	switch sceneType {
	case SceneTypeFrontDesk:
		return "前台"
	case SceneTypeCorridor:
		return "走廊"
	case SceneTypePassage:
		return "通道"
	case SceneTypeWaitingArea:
		return "候诊区"
	case SceneTypeHall:
		return "大厅"
	case SceneTypeEntrance:
		return "门口"
	case SceneTypeStorage:
		return "库房"
	case SceneTypePharmacy:
		return "药房"
	case SceneTypeMachineRoom:
		return "机房"
	default:
		return ""
	}
}

func normalizeRecognitionAreaType(value string) AreaType {
	switch AreaType(strings.ToLower(strings.TrimSpace(value))) {
	case AreaTypeTreatment:
		return AreaTypeTreatment
	case AreaTypeVIPTreatment:
		return AreaTypeVIPTreatment
	case AreaTypeConsultation:
		return AreaTypeConsultation
	case AreaTypeBeauty:
		return AreaTypeBeauty
	default:
		return ""
	}
}

func normalizeRecognitionSceneType(value string) SceneType {
	sceneType := SceneType(strings.ToLower(strings.TrimSpace(value)))
	if validSceneType(sceneType) {
		return sceneType
	}
	return SceneTypeUnknown
}

func normalizeRecognitionDecisionSource(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "number_card", "scene", "none":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "none"
	}
}

func normalizeRecognitionConfidence(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "medium"
	}
}

func elapsedMilliseconds(started time.Time) int64 {
	if started.IsZero() {
		return 0
	}
	elapsed := time.Since(started).Milliseconds()
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isConfirmedChannelStatus(status ChannelStatus) bool {
	return status == ChannelStatusConfirmedBusiness || status == ChannelStatusConfirmedNonBusiness
}

func channelMappingExportRows(store Store) []ChannelMappingExportRow {
	rows := []ChannelMappingExportRow{}
	for _, recorder := range store.Recorders {
		if recorder.Status == RecorderStatusOffline {
			continue
		}
		for _, channel := range recorder.Channels {
			if !channel.IsActive || channel.Status == ChannelStatusInactive {
				continue
			}
			rows = append(rows, ChannelMappingExportRow{
				City:          store.City,
				StoreName:     store.Name,
				ExternalOrgID: store.ExternalOrgID,
				RecorderCode:  recorder.DeviceCode,
				ChannelNo:     channel.ChannelNo,
				SnapshotPath:  firstNonEmpty(channel.FullImageURL, channel.ThumbnailURL),
				AreaTypeLabel: channelAreaTypeLabel(channel),
				NumberOrNote:  channelNumberOrNote(channel),
			})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		if channelExportTypeRank(left.AreaTypeLabel) != channelExportTypeRank(right.AreaTypeLabel) {
			return channelExportTypeRank(left.AreaTypeLabel) < channelExportTypeRank(right.AreaTypeLabel)
		}
		if compareNumberOrText(left.NumberOrNote, right.NumberOrNote) != 0 {
			return compareNumberOrText(left.NumberOrNote, right.NumberOrNote) < 0
		}
		if left.RecorderCode != right.RecorderCode {
			return left.RecorderCode < right.RecorderCode
		}
		return left.ChannelNo < right.ChannelNo
	})
	for index := range rows {
		rows[index].Index = index + 1
	}
	return rows
}

func channelAreaTypeLabel(channel Channel) string {
	switch channel.AreaType {
	case AreaTypeConsultation:
		return "面诊室"
	case AreaTypeTreatment:
		return "治疗室"
	case AreaTypeVIPTreatment:
		return "VIP治疗室"
	case AreaTypeBeauty:
		return "生美"
	default:
		return "其他区域"
	}
}

func channelNumberOrNote(channel Channel) string {
	if channel.AreaType != "" && channel.AreaNumber > 0 {
		return strconv.Itoa(channel.AreaNumber)
	}
	if strings.TrimSpace(channel.AreaNote) != "" {
		return strings.TrimSpace(channel.AreaNote)
	}
	return "-"
}

func channelExportTypeRank(label string) int {
	switch label {
	case "面诊室":
		return 0
	case "治疗室", "VIP治疗室":
		return 1
	case "生美":
		return 2
	default:
		return 3
	}
}

func compareNumberOrText(left string, right string) int {
	leftNumber, leftOK := parseLeadingInt(left)
	rightNumber, rightOK := parseLeadingInt(right)
	if leftOK && rightOK && leftNumber != rightNumber {
		if leftNumber < rightNumber {
			return -1
		}
		return 1
	}
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func parseLeadingInt(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	match := regexp.MustCompile(`^\d+`).FindString(value)
	if match == "" {
		return 0, false
	}
	number, err := strconv.Atoi(match)
	return number, err == nil
}

func exportFileName(storeName string, now time.Time) string {
	name := strings.TrimSpace(storeName)
	if name == "" {
		name = "门店"
	}
	replacer := strings.NewReplacer("/", "-", `\`, "-", ":", "-", "*", "-", "?", "-", `"`, "'", "<", "-", ">", "-", "|", "-")
	name = replacer.Replace(name)
	return fmt.Sprintf("%s-通道映射确认表-%s.xlsx", name, now.Format("20060102-1504"))
}

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
