package storespace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	repo       Repository
	scanner    ChannelScanner
	recognizer ChannelRecognizer
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

type ChannelScanner interface {
	ScanRecorderChannels(ctx context.Context, account EzvizAccount, recorder Recorder) ([]ScannedChannel, error)
	CaptureChannel(ctx context.Context, account EzvizAccount, recorder Recorder, channel Channel) (ChannelSnapshotInput, error)
}

type ChannelRecognizer interface {
	RecognizeChannel(ctx context.Context, imageURL string) (ChannelRecognitionResult, error)
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
		if !channel.IsActive || channel.Status == ChannelStatusInactive {
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
		})
	}
	snapshot.RecognitionResult = channelRecognitionStatusJSON("captured", "", captureMS, 0, elapsedMilliseconds(channelStarted))
	if s.recognizer != nil && !isConfirmedChannelStatus(channel.Status) {
		recognitionStarted := time.Now()
		result, err := s.recognizer.RecognizeChannel(ctx, firstNonEmpty(snapshot.FullImagePath, snapshot.ThumbnailPath))
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
