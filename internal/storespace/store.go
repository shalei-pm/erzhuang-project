package storespace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("store space resource not found")

type Repository interface {
	ListEzvizAccounts(ctx context.Context) ([]EzvizAccount, error)
	CreateEzvizAccount(ctx context.Context, input CreateEzvizAccountInput) (*EzvizAccount, error)
	EzvizAccountNameExists(ctx context.Context, accountName string) (bool, error)
	UpsertEzvizAccountName(ctx context.Context, accountName string) error
	ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error)
	GetStore(ctx context.Context, id int64) (*Store, error)
	GetStoreDesignPlanData(ctx context.Context, id int64) (*Store, error)
	GetStoreChannelData(ctx context.Context, id int64) (*Store, error)
	CreateStore(ctx context.Context, input CreateStoreInput) (*Store, error)
	UpdateStoreBasicInfo(ctx context.Context, id int64, input UpdateStoreBasicInfoInput) (*Store, error)
	SaveDesignPlan(ctx context.Context, storeID int64, input SaveDesignPlanInput) (*Store, error)
	AddRecorder(ctx context.Context, storeID int64, input AddRecorderInput) (*Store, error)
	GetRecorder(ctx context.Context, recorderID int64) (*Recorder, error)
	GetChannelContext(ctx context.Context, channelID int64) (*Channel, *Recorder, *EzvizAccount, error)
	GetEzvizAccount(ctx context.Context, accountID int64) (*EzvizAccount, error)
	ReplaceRecorderChannels(ctx context.Context, recorderID int64, channels []ChannelInput) (*Recorder, error)
	UpsertRecorderChannel(ctx context.Context, recorderID int64, channel ChannelInput) (*Channel, error)
	SaveChannelSnapshot(ctx context.Context, channelID int64, input ChannelSnapshotInput) (*Channel, error)
	UnlockChannelForEdit(ctx context.Context, channelID int64) (*Channel, error)
	ConfirmChannel(ctx context.Context, channelID int64, input ChannelConfirmationInput) (*Store, error)
	DeleteStore(ctx context.Context, id int64) error
	DeleteRecorder(ctx context.Context, recorderID int64) error
	DeleteChannel(ctx context.Context, channelID int64) (*Store, error)
	CheckDuplicate(ctx context.Context, name string, excludeStoreID int64) (DuplicateCheckResult, error)
	DeviceCodeExists(ctx context.Context, deviceCode string, excludeRecorderID int64) (bool, error)
	FindOrCreateArea(ctx context.Context, input AreaLookup, areaNumber int) (*Area, error)
}

type MemoryStore struct {
	mu             sync.Mutex
	nextStoreID    int64
	nextAreaID     int64
	nextPlanID     int64
	nextRecorderID int64
	nextAccountID  int64
	stores         map[int64]*Store
	accounts       map[int64]*EzvizAccount
	deviceCodes    map[string]int64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextStoreID:    1,
		nextAreaID:     1,
		nextPlanID:     1,
		nextRecorderID: 1,
		nextAccountID:  1,
		stores:         map[int64]*Store{},
		accounts:       map[int64]*EzvizAccount{},
		deviceCodes:    map[string]int64{},
	}
}

func (s *MemoryStore) ListEzvizAccounts(ctx context.Context) ([]EzvizAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accounts := make([]EzvizAccount, 0, len(s.accounts))
	for _, account := range s.accounts {
		accounts = append(accounts, *account)
	}
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].AccountName == accounts[j].AccountName {
			return accounts[i].ID < accounts[j].ID
		}
		return accounts[i].AccountName < accounts[j].AccountName
	})
	return accounts, nil
}

func (s *MemoryStore) CreateEzvizAccount(ctx context.Context, input CreateEzvizAccountInput) (*EzvizAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	account := EzvizAccount{
		ID:          s.nextAccountID,
		AccountName: strings.TrimSpace(input.AccountName),
		Status:      "unverified",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.nextAccountID++
	s.accounts[account.ID] = &account
	copy := account
	return &copy, nil
}

func (s *MemoryStore) EzvizAccountNameExists(ctx context.Context, accountName string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanName := strings.TrimSpace(accountName)
	for _, account := range s.accounts {
		if account.AccountName == cleanName {
			return true, nil
		}
	}
	return false, nil
}

func (s *MemoryStore) UpsertEzvizAccountName(ctx context.Context, accountName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanName := strings.TrimSpace(accountName)
	if cleanName == "" {
		return nil
	}
	now := time.Now().UTC()
	for _, account := range s.accounts {
		if account.AccountName == cleanName {
			account.Status = "available"
			account.UpdatedAt = now
			return nil
		}
	}
	account := EzvizAccount{
		ID:          s.nextAccountID,
		AccountName: cleanName,
		Status:      "available",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.nextAccountID++
	s.accounts[account.ID] = &account
	return nil
}

func (s *MemoryStore) ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filters = normalizeFilters(filters)
	items := []StoreListItem{}
	for _, store := range s.stores {
		if !MatchesStoreSearch(store.Name, store.NormalizedName, filters.Query) {
			continue
		}
		items = append(items, storeListItem(*store))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	total := len(items)
	start := (filters.Page - 1) * filters.PageSize
	if start > total {
		start = total
	}
	end := start + filters.PageSize
	if end > total {
		end = total
	}

	return StoreListResult{
		Items:    items[start:end],
		Page:     filters.Page,
		PageSize: filters.PageSize,
		Total:    total,
	}, nil
}

func (s *MemoryStore) GetStore(ctx context.Context, id int64) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, ok := s.stores[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := cloneStore(*store)
	return &copy, nil
}

func (s *MemoryStore) GetStoreDesignPlanData(ctx context.Context, id int64) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, ok := s.stores[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := cloneStore(*store)
	copy.Recorders = nil
	return &copy, nil
}

func (s *MemoryStore) GetStoreChannelData(ctx context.Context, id int64) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, ok := s.stores[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := cloneStore(*store)
	copy.Areas = nil
	copy.DesignPlans = nil
	return &copy, nil
}

func (s *MemoryStore) CreateStore(ctx context.Context, input CreateStoreInput) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	store := Store{
		ID:               s.nextStoreID,
		City:             strings.TrimSpace(input.City),
		Name:             strings.TrimSpace(input.Name),
		NormalizedName:   NormalizeStoreName(input.Name),
		ExternalOrgID:    strings.TrimSpace(input.ExternalOrgID),
		DesignPlanStatus: DesignPlanStatusNotUploaded,
		OverallStatus:    OverallStatusPartial,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.nextStoreID++

	if strings.TrimSpace(input.DesignPlanUploadID) != "" {
		store.DesignPlanStatus = DesignPlanStatusPendingRecognition
		store.DesignPlans = append(store.DesignPlans, DesignPlan{
			ID:                s.nextPlanID,
			StoreID:           store.ID,
			UploadID:          strings.TrimSpace(input.DesignPlanUploadID),
			RecognitionStatus: RecognitionStatusNotStarted,
			CreatedAt:         now,
			UpdatedAt:         now,
		})
		s.nextPlanID++
	}

	for _, recorderInput := range input.Recorders {
		code := normalizeDeviceCode(recorderInput.DeviceCode)
		if code == "" {
			continue
		}
		recorder := Recorder{
			ID:             s.nextRecorderID,
			StoreID:        store.ID,
			EzvizAccountID: recorderInput.EzvizAccountID,
			DeviceCode:     code,
			Status:         RecorderStatusOffline,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		s.nextRecorderID++
		store.Recorders = append(store.Recorders, recorder)
		s.deviceCodes[code] = recorder.ID
	}

	s.stores[store.ID] = &store
	copy := cloneStore(store)
	return &copy, nil
}

func (s *MemoryStore) UpdateStoreBasicInfo(ctx context.Context, id int64, input UpdateStoreBasicInfoInput) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, ok := s.stores[id]
	if !ok {
		return nil, ErrNotFound
	}
	store.City = strings.TrimSpace(input.City)
	store.Name = strings.TrimSpace(input.Name)
	store.NormalizedName = NormalizeStoreName(input.Name)
	store.ExternalOrgID = strings.TrimSpace(input.ExternalOrgID)
	store.UpdatedAt = time.Now().UTC()
	copy := cloneStore(*store)
	return &copy, nil
}

func (s *MemoryStore) SaveDesignPlan(ctx context.Context, storeID int64, input SaveDesignPlanInput) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, ok := s.stores[storeID]
	if !ok {
		return nil, ErrNotFound
	}

	now := time.Now().UTC()
	plan := DesignPlan{
		ID:                s.nextPlanID,
		StoreID:           storeID,
		UploadID:          strings.TrimSpace(input.UploadID),
		PDFFileName:       strings.TrimSpace(input.PDFFileName),
		OriginalPDFPath:   strings.TrimSpace(input.OriginalPDFPath),
		PreviewImagePath:  strings.TrimSpace(input.PreviewImagePath),
		ThumbnailPath:     strings.TrimSpace(input.ThumbnailPath),
		PageCount:         input.PageCount,
		RecognitionStatus: RecognitionStatusCompleted,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if len(store.DesignPlans) > 0 {
		plan.ID = store.DesignPlans[0].ID
		plan.CreatedAt = store.DesignPlans[0].CreatedAt
	} else {
		s.nextPlanID++
	}
	store.DesignPlans = []DesignPlan{plan}

	for _, areaInput := range input.Areas {
		number, _ := strconv.Atoi(strings.TrimSpace(areaInput.NumberText))
		area := findAreaByID(store.Areas, areaInput.ID)
		if area == nil {
			area = findAreaByTypeNumber(store.Areas, areaInput.Type, number)
		}
		source := AreaSourceDesignPlan
		if area != nil {
			source = mergeAreaSource(area.Source, AreaSourceDesignPlan)
			if area.Source != AreaSourceVideoChannel && area.Source != AreaSourceMultiple {
				area.Type = areaInput.Type
				area.Number = number
				area.DisplayName = displayNameOrDefault(areaInput.DisplayName, areaInput.Type, number)
			}
			area.Source = source
			area.Status = AreaStatusConfirmed
			area.Box = areaInput.Box
			area.UpdatedAt = now
			continue
		}
		store.Areas = append(store.Areas, Area{
			ID:          s.nextAreaID,
			StoreID:     store.ID,
			Type:        areaInput.Type,
			Number:      number,
			DisplayName: displayNameOrDefault(areaInput.DisplayName, areaInput.Type, number),
			Source:      source,
			Status:      AreaStatusConfirmed,
			Box:         areaInput.Box,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		s.nextAreaID++
	}

	store.DesignPlanStatus = DesignPlanStatusCompleted
	store.UpdatedAt = now
	copy := cloneStore(*store)
	return &copy, nil
}

func (s *MemoryStore) AddRecorder(ctx context.Context, storeID int64, input AddRecorderInput) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, ok := s.stores[storeID]
	if !ok {
		return nil, ErrNotFound
	}

	now := time.Now().UTC()
	code := normalizeDeviceCode(input.DeviceCode)
	recorder := Recorder{
		ID:             s.nextRecorderID,
		StoreID:        store.ID,
		EzvizAccountID: input.EzvizAccountID,
		DeviceCode:     code,
		Status:         RecorderStatusOffline,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.nextRecorderID++
	store.Recorders = append(store.Recorders, recorder)
	store.UpdatedAt = now
	s.deviceCodes[code] = recorder.ID

	copy := cloneStore(*store)
	return &copy, nil
}

func (s *MemoryStore) GetRecorder(ctx context.Context, recorderID int64) (*Recorder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, store := range s.stores {
		for _, recorder := range store.Recorders {
			if recorder.ID == recorderID {
				copy := recorder
				copy.Channels = append([]Channel(nil), recorder.Channels...)
				return &copy, nil
			}
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) GetEzvizAccount(ctx context.Context, accountID int64) (*EzvizAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	account, ok := s.accounts[accountID]
	if !ok {
		return nil, ErrNotFound
	}
	copy := *account
	return &copy, nil
}

func (s *MemoryStore) GetChannelContext(ctx context.Context, channelID int64) (*Channel, *Recorder, *EzvizAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, store := range s.stores {
		for recorderIndex := range store.Recorders {
			recorder := &store.Recorders[recorderIndex]
			for channelIndex := range recorder.Channels {
				channel := &recorder.Channels[channelIndex]
				if channel.ID != channelID {
					continue
				}
				account, ok := s.accounts[recorder.EzvizAccountID]
				if !ok {
					return nil, nil, nil, ErrNotFound
				}
				channelCopy := *channel
				recorderCopy := *recorder
				accountCopy := *account
				return &channelCopy, &recorderCopy, &accountCopy, nil
			}
		}
	}
	return nil, nil, nil, ErrNotFound
}

func (s *MemoryStore) ReplaceRecorderChannels(ctx context.Context, recorderID int64, channels []ChannelInput) (*Recorder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for _, store := range s.stores {
		for recorderIndex := range store.Recorders {
			recorder := &store.Recorders[recorderIndex]
			if recorder.ID != recorderID {
				continue
			}
			scannedByNo := map[int]ChannelInput{}
			for _, channel := range channels {
				if channel.ChannelNo > 0 {
					scannedByNo[channel.ChannelNo] = channel
				}
			}
			maxID := int64(0)
			for index := range recorder.Channels {
				channel := &recorder.Channels[index]
				if channel.ID > maxID {
					maxID = channel.ID
				}
				scanned, ok := scannedByNo[channel.ChannelNo]
				if !ok || !scanned.IsActive {
					channel.IsActive = false
					channel.Status = ChannelStatusInactive
					channel.UpdatedAt = now
					delete(scannedByNo, channel.ChannelNo)
					continue
				}
				channel.ChannelName = strings.TrimSpace(scanned.ChannelName)
				channel.IsActive = true
				if channel.Status == ChannelStatusInactive {
					if channel.AreaID != 0 || channel.AreaType != "" || channel.AreaNumber != 0 || channel.ConfirmedAt != nil {
						if channel.AreaType != "" {
							channel.Status = ChannelStatusConfirmedBusiness
						} else {
							channel.Status = ChannelStatusConfirmedNonBusiness
						}
					} else {
						channel.Status = ChannelStatusPendingRecognition
						channel.SceneType = SceneTypeUnknown
						channel.AreaType = ""
						channel.AreaNumber = 0
						channel.AreaID = 0
						channel.ConfirmedAt = nil
					}
				}
				channel.UpdatedAt = now
				delete(scannedByNo, channel.ChannelNo)
			}
			for _, channel := range scannedByNo {
				if !channel.IsActive || channel.ChannelNo <= 0 {
					continue
				}
				maxID++
				recorder.Channels = append(recorder.Channels, Channel{
					ID:                  maxID,
					RecorderID:          recorder.ID,
					ChannelNo:           channel.ChannelNo,
					ChannelName:         strings.TrimSpace(channel.ChannelName),
					Status:              ChannelStatusPendingRecognition,
					IsActive:            true,
					SceneType:           SceneTypeUnknown,
					RecognitionAttempts: 0,
					CreatedAt:           now,
					UpdatedAt:           now,
				})
			}
			sort.Slice(recorder.Channels, func(i, j int) bool {
				return recorder.Channels[i].ChannelNo < recorder.Channels[j].ChannelNo
			})
			recorder.EffectiveChannelCount = activeChannelCount(recorder.Channels)
			recorder.LastScannedAt = &now
			if recorder.EffectiveChannelCount > 0 {
				recorder.Status = RecorderStatusOnline
			} else {
				recorder.Status = RecorderStatusOffline
			}
			recorder.UpdatedAt = now
			store.UpdatedAt = now

			copy := *recorder
			copy.Channels = append([]Channel(nil), recorder.Channels...)
			return &copy, nil
		}
	}
	return nil, ErrNotFound
}

func activeChannelCount(channels []Channel) int {
	count := 0
	for _, channel := range channels {
		if channel.IsActive && channel.Status != ChannelStatusInactive {
			count++
		}
	}
	return count
}

func (s *MemoryStore) UpsertRecorderChannel(ctx context.Context, recorderID int64, input ChannelInput) (*Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for _, store := range s.stores {
		for recorderIndex := range store.Recorders {
			recorder := &store.Recorders[recorderIndex]
			if recorder.ID != recorderID {
				continue
			}
			if input.ChannelNo <= 0 || !input.IsActive {
				return nil, ErrNotFound
			}
			maxID := int64(0)
			for channelIndex := range recorder.Channels {
				channel := &recorder.Channels[channelIndex]
				if channel.ID > maxID {
					maxID = channel.ID
				}
				if channel.ChannelNo != input.ChannelNo {
					continue
				}
				channel.ChannelName = strings.TrimSpace(input.ChannelName)
				channel.IsActive = true
				if channel.Status == ChannelStatusInactive {
					if channel.AreaID != 0 || channel.AreaType != "" || channel.AreaNumber != 0 || channel.ConfirmedAt != nil {
						if channel.AreaType != "" {
							channel.Status = ChannelStatusConfirmedBusiness
						} else {
							channel.Status = ChannelStatusConfirmedNonBusiness
						}
					} else {
						channel.Status = ChannelStatusPendingRecognition
						channel.SceneType = SceneTypeUnknown
					}
				}
				channel.UpdatedAt = now
				recorder.EffectiveChannelCount = activeChannelCount(recorder.Channels)
				recorder.LastScannedAt = &now
				recorder.Status = RecorderStatusOnline
				recorder.UpdatedAt = now
				store.UpdatedAt = now
				copy := *channel
				return &copy, nil
			}
			channel := Channel{
				ID:                  maxID + 1,
				RecorderID:          recorder.ID,
				ChannelNo:           input.ChannelNo,
				ChannelName:         strings.TrimSpace(input.ChannelName),
				Status:              ChannelStatusPendingRecognition,
				IsActive:            true,
				SceneType:           SceneTypeUnknown,
				RecognitionAttempts: 0,
				CreatedAt:           now,
				UpdatedAt:           now,
			}
			recorder.Channels = append(recorder.Channels, channel)
			sort.Slice(recorder.Channels, func(i, j int) bool {
				return recorder.Channels[i].ChannelNo < recorder.Channels[j].ChannelNo
			})
			recorder.EffectiveChannelCount = activeChannelCount(recorder.Channels)
			recorder.LastScannedAt = &now
			recorder.Status = RecorderStatusOnline
			recorder.UpdatedAt = now
			store.UpdatedAt = now
			return &channel, nil
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) SaveChannelSnapshot(ctx context.Context, channelID int64, input ChannelSnapshotInput) (*Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for _, store := range s.stores {
		for recorderIndex := range store.Recorders {
			recorder := &store.Recorders[recorderIndex]
			for channelIndex := range recorder.Channels {
				channel := &recorder.Channels[channelIndex]
				if channel.ID != channelID {
					continue
				}
				if strings.TrimSpace(input.ThumbnailPath) != "" || strings.TrimSpace(input.FullImagePath) != "" {
					channel.ThumbnailURL = strings.TrimSpace(input.ThumbnailPath)
					channel.FullImageURL = strings.TrimSpace(input.FullImagePath)
					channel.FullImageExpiresAt = input.FullImageExpiresAt
				}
				if input.Status != "" {
					channel.Status = input.Status
				}
				if input.SceneType != "" {
					channel.SceneType = input.SceneType
				}
				if input.AreaType != "" {
					channel.AreaType = input.AreaType
				} else if input.Status == ChannelStatusPendingConfirmation {
					channel.AreaType = ""
				}
				if number := mustPositiveInt(input.AreaNumberText); number > 0 {
					channel.AreaNumber = number
					channel.AreaNote = ""
				} else if input.Status == ChannelStatusPendingConfirmation {
					channel.AreaNumber = 0
				}
				if input.AreaNote != "" || input.Status == ChannelStatusPendingConfirmation {
					channel.AreaNote = strings.TrimSpace(input.AreaNote)
				}
				if strings.TrimSpace(input.RecognitionResult) != "" || input.CountAttempt {
					channel.RecognitionResult = strings.TrimSpace(input.RecognitionResult)
				}
				if input.CountAttempt {
					channel.RecognitionAttempts++
				}
				channel.UpdatedAt = now
				copy := *channel
				return &copy, nil
			}
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) ConfirmChannel(ctx context.Context, channelID int64, input ChannelConfirmationInput) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for _, store := range s.stores {
		for recorderIndex := range store.Recorders {
			recorder := &store.Recorders[recorderIndex]
			for channelIndex := range recorder.Channels {
				channel := &recorder.Channels[channelIndex]
				if channel.ID != channelID {
					continue
				}

				channel.SceneType = input.SceneType
				channel.UpdatedAt = now
				channel.ConfirmedAt = &now
				if input.AreaType == "" {
					channel.AreaType = ""
					channel.AreaNumber = 0
					channel.AreaNote = strings.TrimSpace(input.AreaNote)
					channel.AreaID = 0
					channel.Status = ChannelStatusConfirmedNonBusiness
					s.deleteUnusedVideoAreasLocked(store)
					store.UpdatedAt = now
					copy := cloneStore(*store)
					return &copy, nil
				}

				number := 0
				if strings.TrimSpace(input.AreaNumber) != "" {
					parsedNumber, err := strconv.Atoi(strings.TrimSpace(input.AreaNumber))
					if err != nil || parsedNumber <= 0 {
						return nil, &ValidationError{Fields: map[string]string{"area_number": "区域编号必须是正整数"}}
					}
					number = parsedNumber
				} else if input.AreaType != AreaTypeVIPTreatment {
					return nil, &ValidationError{Fields: map[string]string{"area_number": "区域编号必须是正整数"}}
				}
				area, err := s.updateOrFindVideoAreaLocked(store, channel.AreaID, input.AreaType, number, now)
				if err != nil {
					return nil, err
				}
				channel.AreaType = area.Type
				channel.AreaNumber = area.Number
				channel.AreaNote = ""
				channel.AreaID = area.ID
				channel.SceneType = SceneType(area.Type)
				channel.Status = ChannelStatusConfirmedBusiness
				s.deleteUnusedVideoAreasLocked(store)
				store.UpdatedAt = now
				copy := cloneStore(*store)
				return &copy, nil
			}
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) UnlockChannelForEdit(ctx context.Context, channelID int64) (*Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for _, store := range s.stores {
		for recorderIndex := range store.Recorders {
			recorder := &store.Recorders[recorderIndex]
			for channelIndex := range recorder.Channels {
				channel := &recorder.Channels[channelIndex]
				if channel.ID != channelID {
					continue
				}
				if channel.Status == ChannelStatusInactive || !channel.IsActive {
					return nil, &ValidationError{Fields: map[string]string{"channel": "通道已失效，无法编辑"}}
				}
				channel.Status = ChannelStatusPendingConfirmation
				channel.ConfirmedAt = nil
				channel.UpdatedAt = now
				store.UpdatedAt = now
				copy := *channel
				return &copy, nil
			}
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) updateOrFindVideoAreaLocked(store *Store, currentAreaID int64, areaType AreaType, areaNumber int, now time.Time) (*Area, error) {
	if currentAreaID != 0 {
		for index := range store.Areas {
			area := &store.Areas[index]
			if area.ID != currentAreaID || (area.Source != AreaSourceVideoChannel && area.Source != AreaSourceMultiple) {
				continue
			}
			if existing := findAreaByTypeNumber(store.Areas, areaType, areaNumber); existing != nil && existing.ID != currentAreaID {
				if existing.Source != AreaSourceVideoChannel && existing.Source != AreaSourceMultiple {
					existing.Source = AreaSourceMultiple
				}
				existing.UpdatedAt = now
				return existing, nil
			}
			area.Type = areaType
			area.Number = areaNumber
			area.DisplayName = areaDisplayName(areaType, areaNumber)
			area.UpdatedAt = now
			return area, nil
		}
	}
	return s.findOrCreateAreaLocked(store, areaType, areaNumber, AreaSourceVideoChannel, now)
}

func (s *MemoryStore) DeleteStore(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, ok := s.stores[id]
	if !ok {
		return ErrNotFound
	}
	for _, recorder := range store.Recorders {
		delete(s.deviceCodes, recorder.DeviceCode)
	}
	delete(s.stores, id)
	return nil
}

func (s *MemoryStore) DeleteRecorder(ctx context.Context, recorderID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, store := range s.stores {
		for index, recorder := range store.Recorders {
			if recorder.ID != recorderID {
				continue
			}
			delete(s.deviceCodes, recorder.DeviceCode)
			store.Recorders = append(store.Recorders[:index], store.Recorders[index+1:]...)
			store.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return ErrNotFound
}

func (s *MemoryStore) DeleteChannel(ctx context.Context, channelID int64) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for _, store := range s.stores {
		for recorderIndex := range store.Recorders {
			recorder := &store.Recorders[recorderIndex]
			for channelIndex := range recorder.Channels {
				channel := recorder.Channels[channelIndex]
				if channel.ID != channelID {
					continue
				}
				recorder.Channels = append(recorder.Channels[:channelIndex], recorder.Channels[channelIndex+1:]...)
				recorder.EffectiveChannelCount = activeChannelCount(recorder.Channels)
				if recorder.EffectiveChannelCount == 0 {
					recorder.Status = RecorderStatusOffline
				}
				recorder.UpdatedAt = now
				store.UpdatedAt = now
				s.deleteUnusedVideoAreasLocked(store)
				copy := cloneStore(*store)
				return &copy, nil
			}
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) CheckDuplicate(ctx context.Context, name string, excludeStoreID int64) (DuplicateCheckResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := NormalizeStoreName(name)
	result := DuplicateCheckResult{SimilarMatches: []DuplicateMatch{}}
	for _, store := range s.stores {
		if store.ID == excludeStoreID {
			continue
		}
		match := DuplicateMatch{
			ID:             store.ID,
			Name:           store.Name,
			NormalizedName: store.NormalizedName,
			OverallStatus:  store.OverallStatus,
			UpdatedAt:      store.UpdatedAt,
		}
		if store.NormalizedName == normalized {
			match.Reason = "exact"
			if result.ExactMatch == nil {
				copy := match
				result.ExactMatch = &copy
			}
			continue
		}
		if IsSimilarStoreName(name, store.Name) {
			match.Reason = "similar"
			result.SimilarMatches = append(result.SimilarMatches, match)
		}
	}
	return result, nil
}

func (s *MemoryStore) DeviceCodeExists(ctx context.Context, deviceCode string, excludeRecorderID int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.deviceCodes[normalizeDeviceCode(deviceCode)]
	return ok && id != excludeRecorderID, nil
}

func (s *MemoryStore) FindOrCreateArea(ctx context.Context, input AreaLookup, areaNumber int) (*Area, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, ok := s.stores[input.StoreID]
	if !ok {
		return nil, ErrNotFound
	}
	for index := range store.Areas {
		area := &store.Areas[index]
		if area.Type == input.Type && area.Number == areaNumber {
			if area.Source != input.Source && input.Source != "" {
				area.Source = AreaSourceMultiple
				area.UpdatedAt = time.Now().UTC()
				store.UpdatedAt = area.UpdatedAt
			}
			copy := *area
			return &copy, nil
		}
	}

	now := time.Now().UTC()
	source := input.Source
	if source == "" {
		source = AreaSourceManual
	}
	area := Area{
		ID:          s.nextAreaID,
		StoreID:     store.ID,
		Type:        input.Type,
		Number:      areaNumber,
		DisplayName: areaDisplayName(input.Type, areaNumber),
		Source:      source,
		Status:      AreaStatusConfirmed,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.nextAreaID++
	store.Areas = append(store.Areas, area)
	store.UpdatedAt = now
	copy := area
	return &copy, nil
}

func (s *MemoryStore) findOrCreateAreaLocked(store *Store, areaType AreaType, areaNumber int, source AreaSource, now time.Time) (*Area, error) {
	if area := findAreaByTypeNumber(store.Areas, areaType, areaNumber); area != nil {
		if area.Source != source && source != "" {
			area.Source = AreaSourceMultiple
			area.UpdatedAt = now
		}
		return area, nil
	}

	area := Area{
		ID:          s.nextAreaID,
		StoreID:     store.ID,
		Type:        areaType,
		Number:      areaNumber,
		DisplayName: areaDisplayName(areaType, areaNumber),
		Source:      source,
		Status:      AreaStatusConfirmed,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.nextAreaID++
	store.Areas = append(store.Areas, area)
	return &store.Areas[len(store.Areas)-1], nil
}

func findAreaByTypeNumber(areas []Area, areaType AreaType, areaNumber int) *Area {
	for index := range areas {
		if areas[index].Type == areaType && areas[index].Number == areaNumber {
			return &areas[index]
		}
	}
	return nil
}

func findAreaByID(areas []Area, id int64) *Area {
	if id == 0 {
		return nil
	}
	for index := range areas {
		if areas[index].ID == id {
			return &areas[index]
		}
	}
	return nil
}

func mergeAreaSource(existing AreaSource, incoming AreaSource) AreaSource {
	if existing == "" {
		return incoming
	}
	if incoming == "" || existing == incoming {
		return existing
	}
	return AreaSourceMultiple
}

func displayNameOrDefault(name string, areaType AreaType, areaNumber int) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return areaDisplayName(areaType, areaNumber)
}

func (s *MemoryStore) deleteUnusedVideoAreasLocked(store *Store) {
	referenced := map[int64]bool{}
	for _, recorder := range store.Recorders {
		for _, channel := range recorder.Channels {
			if channel.AreaID != 0 {
				referenced[channel.AreaID] = true
			}
		}
	}
	nextAreas := store.Areas[:0]
	for _, area := range store.Areas {
		if area.Source == AreaSourceVideoChannel && area.Box == nil && !referenced[area.ID] {
			continue
		}
		nextAreas = append(nextAreas, area)
	}
	store.Areas = nextAreas
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) ListEzvizAccounts(ctx context.Context) ([]EzvizAccount, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, account_name, status, last_verified_at, created_at, updated_at
		from ezviz_accounts
		order by account_name, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := []EzvizAccount{}
	for rows.Next() {
		var account EzvizAccount
		if err := rows.Scan(
			&account.ID,
			&account.AccountName,
			&account.Status,
			&account.LastVerifiedAt,
			&account.CreatedAt,
			&account.UpdatedAt,
		); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (s *PostgresStore) CreateEzvizAccount(ctx context.Context, input CreateEzvizAccountInput) (*EzvizAccount, error) {
	var account EzvizAccount
	err := s.db.QueryRowContext(ctx, `
		insert into ezviz_accounts (account_name, status)
		values ($1, 'unverified')
		returning id, account_name, status, last_verified_at, created_at, updated_at
	`, strings.TrimSpace(input.AccountName)).Scan(
		&account.ID,
		&account.AccountName,
		&account.Status,
		&account.LastVerifiedAt,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *PostgresStore) EzvizAccountNameExists(ctx context.Context, accountName string) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		select id
		from ezviz_accounts
		where account_name = $1
		limit 1
	`, strings.TrimSpace(accountName)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *PostgresStore) UpsertEzvizAccountName(ctx context.Context, accountName string) error {
	cleanName := strings.TrimSpace(accountName)
	if cleanName == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		insert into ezviz_accounts (account_name, status)
		values ($1, 'available')
		on conflict (account_name) do update set
			status = 'available',
			updated_at = now()
	`, cleanName)
	return err
}

func (s *PostgresStore) ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error) {
	filters = normalizeFilters(filters)
	rawLike := "%" + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(filters.Query), " ", "")) + "%"
	normalizedLike := "%" + NormalizeStoreName(filters.Query) + "%"
	offset := (filters.Page - 1) * filters.PageSize

	var total int
	if err := s.db.QueryRowContext(ctx, `
		select count(*)
		from stores
		where $1 = '%%'
			or replace(lower(name), ' ', '') like $1
			or ($2 <> '%%' and normalized_name like $2)
	`, rawLike, normalizedLike).Scan(&total); err != nil {
		return StoreListResult{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
		select
			s.id,
			s.city,
			s.name,
			s.external_org_id,
			s.design_plan_status,
			s.overall_status,
			s.updated_at,
			count(distinct r.id) as recorder_count,
			count(distinct c.id) filter (where c.is_active) as channel_count,
			count(distinct c.id) filter (where c.is_active) > 0
				and count(distinct c.id) filter (
					where c.is_active
						and c.status not in ('confirmed_business', 'confirmed_non_business')
				) = 0 as channels_fully_confirmed,
			count(distinct a.id) filter (where a.area_type in ('treatment', 'vip_treatment')) as treatment_count,
			count(distinct a.id) filter (where a.area_type = 'consultation') as consultation_count,
			count(distinct a.id) filter (where a.area_type = 'beauty') as beauty_count,
			count(distinct a.id) as area_count
		from stores s
		left join store_areas a on a.store_id = s.id
		left join video_recorders r on r.store_id = s.id
		left join video_channels c on c.recorder_id = r.id
		where $1 = '%%'
			or replace(lower(s.name), ' ', '') like $1
			or ($2 <> '%%' and s.normalized_name like $2)
		group by s.id
		order by s.updated_at desc
		limit $3 offset $4
	`, rawLike, normalizedLike, filters.PageSize, offset)
	if err != nil {
		return StoreListResult{}, err
	}
	defer rows.Close()

	items := []StoreListItem{}
	for rows.Next() {
		var item StoreListItem
		if err := rows.Scan(
			&item.ID,
			&item.City,
			&item.Name,
			&item.ExternalOrgID,
			&item.DesignPlanStatus,
			&item.OverallStatus,
			&item.UpdatedAt,
			&item.RecorderCount,
			&item.ChannelCount,
			&item.ChannelsFullyConfirmed,
			&item.TreatmentCount,
			&item.ConsultationCount,
			&item.BeautyCount,
			&item.AreaCount,
		); err != nil {
			return StoreListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return StoreListResult{}, err
	}
	return StoreListResult{Items: items, Page: filters.Page, PageSize: filters.PageSize, Total: total}, nil
}

func (s *PostgresStore) GetStore(ctx context.Context, id int64) (*Store, error) {
	store, err := s.getStoreBase(ctx, id)
	if err != nil {
		return nil, err
	}

	areas, err := s.listAreas(ctx, id)
	if err != nil {
		return nil, err
	}
	store.Areas = areas
	plans, err := s.listDesignPlans(ctx, id)
	if err != nil {
		return nil, err
	}
	store.DesignPlans = plans
	recorders, err := s.listRecorders(ctx, id)
	if err != nil {
		return nil, err
	}
	store.Recorders = recorders
	return store, nil
}

func (s *PostgresStore) getStoreBase(ctx context.Context, id int64) (*Store, error) {
	var store Store
	err := s.db.QueryRowContext(ctx, `
		select id, city, name, normalized_name, external_org_id, design_plan_status,
			overall_status, created_at, updated_at
		from stores
		where id = $1
	`, id).Scan(
		&store.ID,
		&store.City,
		&store.Name,
		&store.NormalizedName,
		&store.ExternalOrgID,
		&store.DesignPlanStatus,
		&store.OverallStatus,
		&store.CreatedAt,
		&store.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &store, nil
}

func (s *PostgresStore) GetStoreDesignPlanData(ctx context.Context, id int64) (*Store, error) {
	store, err := s.getStoreBase(ctx, id)
	if err != nil {
		return nil, err
	}
	areas, err := s.listAreas(ctx, id)
	if err != nil {
		return nil, err
	}
	store.Areas = areas
	plans, err := s.listDesignPlans(ctx, id)
	if err != nil {
		return nil, err
	}
	store.DesignPlans = plans
	return store, nil
}

func (s *PostgresStore) GetStoreChannelData(ctx context.Context, id int64) (*Store, error) {
	store, err := s.getStoreBase(ctx, id)
	if err != nil {
		return nil, err
	}
	recorders, err := s.listRecorders(ctx, id)
	if err != nil {
		return nil, err
	}
	store.Recorders = recorders
	return store, nil
}

func (s *PostgresStore) CreateStore(ctx context.Context, input CreateStoreInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	designPlanStatus := DesignPlanStatusNotUploaded
	if strings.TrimSpace(input.DesignPlanUploadID) != "" {
		designPlanStatus = DesignPlanStatusPendingRecognition
	}

	var id int64
	err = tx.QueryRowContext(ctx, `
		insert into stores (city, name, normalized_name, external_org_id, design_plan_status, overall_status)
		values ($1, $2, $3, $4, $5, $6)
		returning id
	`, strings.TrimSpace(input.City), strings.TrimSpace(input.Name), NormalizeStoreName(input.Name), strings.TrimSpace(input.ExternalOrgID),
		designPlanStatus, OverallStatusPartial).Scan(&id)
	if err != nil {
		return nil, err
	}

	if uploadID := strings.TrimSpace(input.DesignPlanUploadID); uploadID != "" {
		if _, err := tx.ExecContext(ctx, `
			insert into store_design_plans (store_id, upload_id, recognition_status)
			values ($1, $2, $3)
		`, id, uploadID, RecognitionStatusNotStarted); err != nil {
			return nil, err
		}
	}

	for _, recorder := range input.Recorders {
		code := normalizeDeviceCode(recorder.DeviceCode)
		if code == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			insert into video_recorders (store_id, ezviz_account_id, device_code, status)
			values ($1, nullif($2::bigint, 0), $3, $4)
		`, id, recorder.EzvizAccountID, code, RecorderStatusOffline); err != nil {
			return nil, err
		}
	}

	if err := insertOperationLog(ctx, tx, "create", "store", id, id, fmt.Sprintf("created store %s", strings.TrimSpace(input.Name))); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, id)
}

func (s *PostgresStore) UpdateStoreBasicInfo(ctx context.Context, id int64, input UpdateStoreBasicInfoInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		update stores
		set city = $2,
			name = $3,
			normalized_name = $4,
			external_org_id = $5,
			updated_at = now()
		where id = $1
	`, id, strings.TrimSpace(input.City), strings.TrimSpace(input.Name), NormalizeStoreName(input.Name), strings.TrimSpace(input.ExternalOrgID))
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, ErrNotFound
	}
	if err := insertOperationLog(ctx, tx, "update", "store", id, id, fmt.Sprintf("updated store basic info %s", strings.TrimSpace(input.Name))); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, id)
}

func (s *PostgresStore) SaveDesignPlan(ctx context.Context, storeID int64, input SaveDesignPlanInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var existingStoreID int64
	if err := tx.QueryRowContext(ctx, `select id from stores where id = $1`, storeID).Scan(&existingStoreID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	plan, err := upsertStoreDesignPlan(ctx, tx, storeID, input)
	if err != nil {
		return nil, err
	}
	for _, areaInput := range input.Areas {
		area, err := upsertDesignArea(ctx, tx, storeID, areaInput)
		if err != nil {
			return nil, err
		}
		if err := upsertDesignAnnotation(ctx, tx, plan.ID, area.ID, areaInput.Box); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		update stores
		set design_plan_status = $1,
			updated_at = now()
		where id = $2
	`, DesignPlanStatusCompleted, storeID); err != nil {
		return nil, err
	}
	if err := insertOperationLog(ctx, tx, "save_design_plan", "store", storeID, storeID, "saved design plan annotations"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, storeID)
}

func (s *PostgresStore) AddRecorder(ctx context.Context, storeID int64, input AddRecorderInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var existingStoreID int64
	if err := tx.QueryRowContext(ctx, `select id from stores where id = $1`, storeID).Scan(&existingStoreID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	code := normalizeDeviceCode(input.DeviceCode)
	var recorderID int64
	if err := tx.QueryRowContext(ctx, `
		insert into video_recorders (store_id, ezviz_account_id, device_code, status)
		values ($1, nullif($2::bigint, 0), $3, $4)
		returning id
	`, storeID, input.EzvizAccountID, code, RecorderStatusOffline).Scan(&recorderID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update stores set updated_at = now() where id = $1`, storeID); err != nil {
		return nil, err
	}
	if err := insertOperationLog(ctx, tx, "create", "recorder", recorderID, storeID, fmt.Sprintf("added recorder %s", code)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, storeID)
}

func (s *PostgresStore) GetRecorder(ctx context.Context, recorderID int64) (*Recorder, error) {
	recorder, err := s.queryRecorder(ctx, s.db, recorderID)
	if err != nil {
		return nil, err
	}
	channels, err := s.listChannels(ctx, recorder.ID)
	if err != nil {
		return nil, err
	}
	recorder.Channels = channels
	return recorder, nil
}

func (s *PostgresStore) GetEzvizAccount(ctx context.Context, accountID int64) (*EzvizAccount, error) {
	var account EzvizAccount
	err := s.db.QueryRowContext(ctx, `
		select id, account_name, status, last_verified_at, created_at, updated_at
		from ezviz_accounts
		where id = $1
	`, accountID).Scan(
		&account.ID,
		&account.AccountName,
		&account.Status,
		&account.LastVerifiedAt,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *PostgresStore) GetChannelContext(ctx context.Context, channelID int64) (*Channel, *Recorder, *EzvizAccount, error) {
	var recorderID int64
	err := s.db.QueryRowContext(ctx, `
		select recorder_id
		from video_channels
		where id = $1
	`, channelID).Scan(&recorderID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, nil, err
	}
	recorder, err := s.GetRecorder(ctx, recorderID)
	if err != nil {
		return nil, nil, nil, err
	}
	var channel *Channel
	for index := range recorder.Channels {
		if recorder.Channels[index].ID == channelID {
			channel = &recorder.Channels[index]
			break
		}
	}
	if channel == nil {
		return nil, nil, nil, ErrNotFound
	}
	account, err := s.GetEzvizAccount(ctx, recorder.EzvizAccountID)
	if err != nil {
		return nil, nil, nil, err
	}
	channelCopy := *channel
	return &channelCopy, recorder, account, nil
}

func (s *PostgresStore) GetChannel(ctx context.Context, channelID int64) (*Channel, error) {
	row := s.db.QueryRowContext(ctx, `
		select id, recorder_id, channel_no, channel_name, status, is_active,
			scene_type, coalesce(area_type, ''), coalesce(area_number, 0),
			coalesce(area_note, ''), coalesce(area_id, 0), recognition_attempts, coalesce(recognition_result::text, ''),
			snapshot.thumbnail_path, snapshot.full_image_path, snapshot.full_image_expires_at,
			confirmed_at, created_at, updated_at
		from video_channels
		left join lateral (
			select thumbnail_path, full_image_path, full_image_expires_at
			from channel_snapshots
			where channel_id = video_channels.id
			order by created_at desc, id desc
			limit 1
		) snapshot on true
		where id = $1
	`, channelID)
	channel, err := scanChannel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return channel, nil
}

func (s *PostgresStore) ReplaceRecorderChannels(ctx context.Context, recorderID int64, channels []ChannelInput) (*Recorder, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	recorder, err := s.queryRecorder(ctx, tx, recorderID)
	if err != nil {
		return nil, err
	}

	scannedNumbers := []int{}
	for _, channel := range channels {
		if channel.ChannelNo <= 0 {
			continue
		}
		scannedNumbers = append(scannedNumbers, channel.ChannelNo)
		if channel.IsActive {
			if _, err := tx.ExecContext(ctx, `
				insert into video_channels (recorder_id, channel_no, channel_name, status, is_active, scene_type)
				values ($1, $2, $3, $4, true, $5)
				on conflict (recorder_id, channel_no) do update
				set channel_name = excluded.channel_name,
					is_active = true,
					status = case
						when video_channels.status = $6 and (
							video_channels.area_id is not null
							or video_channels.area_type is not null
							or video_channels.area_number is not null
							or video_channels.confirmed_at is not null
						) and video_channels.area_type is not null then $7
						when video_channels.status = $6 and (
							video_channels.area_id is not null
							or video_channels.area_type is not null
							or video_channels.area_number is not null
							or video_channels.confirmed_at is not null
						) then $8
						when video_channels.status = $6 then $4
						else video_channels.status
					end,
					scene_type = case
						when video_channels.status = $6 and (
							video_channels.area_id is not null
							or video_channels.area_type is not null
							or video_channels.area_number is not null
							or video_channels.confirmed_at is not null
						) then video_channels.scene_type
						when video_channels.status = $6 then $5
						else video_channels.scene_type
					end,
					area_type = case
						when video_channels.status = $6 and (
							video_channels.area_id is not null
							or video_channels.area_type is not null
							or video_channels.area_number is not null
							or video_channels.confirmed_at is not null
						) then video_channels.area_type
						when video_channels.status = $6 then null
						else video_channels.area_type
					end,
					area_number = case
						when video_channels.status = $6 and (
							video_channels.area_id is not null
							or video_channels.area_type is not null
							or video_channels.area_number is not null
							or video_channels.confirmed_at is not null
						) then video_channels.area_number
						when video_channels.status = $6 then null
						else video_channels.area_number
					end,
					area_id = case
						when video_channels.status = $6 and (
							video_channels.area_id is not null
							or video_channels.area_type is not null
							or video_channels.area_number is not null
							or video_channels.confirmed_at is not null
						) then video_channels.area_id
						when video_channels.status = $6 then null
						else video_channels.area_id
					end,
					confirmed_at = case
						when video_channels.status = $6 and (
							video_channels.area_id is not null
							or video_channels.area_type is not null
							or video_channels.area_number is not null
							or video_channels.confirmed_at is not null
						) then video_channels.confirmed_at
						when video_channels.status = $6 then null
						else video_channels.confirmed_at
					end,
					area_note = case
						when video_channels.status = $6 and (
							video_channels.area_id is not null
							or video_channels.area_type is not null
							or video_channels.area_number is not null
							or video_channels.confirmed_at is not null
						) then video_channels.area_note
						when video_channels.status = $6 then ''
						else video_channels.area_note
					end,
					updated_at = now()
			`, recorderID, channel.ChannelNo, strings.TrimSpace(channel.ChannelName), ChannelStatusPendingRecognition, SceneTypeUnknown, ChannelStatusInactive, ChannelStatusConfirmedBusiness, ChannelStatusConfirmedNonBusiness); err != nil {
				return nil, err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			update video_channels
			set is_active = false,
				status = $1,
				updated_at = now()
			where recorder_id = $2 and channel_no = $3
		`, ChannelStatusInactive, recorderID, channel.ChannelNo); err != nil {
			return nil, err
		}
	}
	if len(scannedNumbers) > 0 {
		args := []any{ChannelStatusInactive, recorderID}
		placeholders := make([]string, 0, len(scannedNumbers))
		for index, channelNo := range scannedNumbers {
			args = append(args, channelNo)
			placeholders = append(placeholders, fmt.Sprintf("$%d", index+3))
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
			update video_channels
			set is_active = false,
				status = $1,
				updated_at = now()
			where recorder_id = $2 and channel_no not in (%s)
		`, strings.Join(placeholders, ", ")), args...); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			update video_channels
			set is_active = false,
				status = $1,
				updated_at = now()
			where recorder_id = $2
		`, ChannelStatusInactive, recorderID); err != nil {
			return nil, err
		}
	}

	status := RecorderStatusOffline
	activeCount := 0
	if err := tx.QueryRowContext(ctx, `
		select count(*)
		from video_channels
		where recorder_id = $1 and is_active and status <> $2
	`, recorderID, ChannelStatusInactive).Scan(&activeCount); err != nil {
		return nil, err
	}
	if activeCount > 0 {
		status = RecorderStatusOnline
	}
	if _, err := tx.ExecContext(ctx, `
		update video_recorders
		set status = $1,
			effective_channel_count = $3,
			last_scanned_at = now(),
			updated_at = now()
		where id = $2
	`, status, recorderID, activeCount); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update stores set updated_at = now() where id = $1`, recorder.StoreID); err != nil {
		return nil, err
	}
	if err := insertOperationLog(ctx, tx, "scan_channels", "recorder", recorderID, recorder.StoreID, fmt.Sprintf("scanned recorder %s", recorder.DeviceCode)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetRecorder(ctx, recorderID)
}

func (s *PostgresStore) UpsertRecorderChannel(ctx context.Context, recorderID int64, channel ChannelInput) (*Channel, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if channel.ChannelNo <= 0 || !channel.IsActive {
		return nil, ErrNotFound
	}
	if _, err := s.queryRecorder(ctx, tx, recorderID); err != nil {
		return nil, err
	}
	var channelID int64
	if err := tx.QueryRowContext(ctx, `
		insert into video_channels (recorder_id, channel_no, channel_name, status, is_active, scene_type)
		values ($1, $2, $3, $4, true, $5)
		on conflict (recorder_id, channel_no) do update
		set channel_name = excluded.channel_name,
			is_active = true,
			status = case
				when video_channels.status = $6 and (
					video_channels.area_id is not null
					or video_channels.area_type is not null
					or video_channels.area_number is not null
					or video_channels.confirmed_at is not null
				) then case
					when video_channels.area_type is not null then $7
					else $8
				end
				when video_channels.status = $6 then $4
				else video_channels.status
			end,
			scene_type = case
				when video_channels.status = $6 then $5
				else video_channels.scene_type
			end,
			updated_at = now()
		returning id
	`, recorderID, channel.ChannelNo, strings.TrimSpace(channel.ChannelName), ChannelStatusPendingRecognition, SceneTypeUnknown, ChannelStatusInactive, ChannelStatusConfirmedBusiness, ChannelStatusConfirmedNonBusiness).Scan(&channelID); err != nil {
		return nil, err
	}

	var activeCount int
	if err := tx.QueryRowContext(ctx, `
		select count(*)
		from video_channels
		where recorder_id = $1 and is_active and status <> $2
	`, recorderID, ChannelStatusInactive).Scan(&activeCount); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		update video_recorders
		set status = $1,
			effective_channel_count = $2,
			last_scanned_at = now(),
			updated_at = now()
		where id = $3
	`, RecorderStatusOnline, activeCount, recorderID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetChannel(ctx, channelID)
}

func (s *PostgresStore) SaveChannelSnapshot(ctx context.Context, channelID int64, input ChannelSnapshotInput) (*Channel, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var recorderID int64
	if err := tx.QueryRowContext(ctx, `select recorder_id from video_channels where id = $1`, channelID).Scan(&recorderID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.ThumbnailPath) != "" || strings.TrimSpace(input.FullImagePath) != "" {
		if _, err := tx.ExecContext(ctx, `
			insert into channel_snapshots (channel_id, thumbnail_path, full_image_path, full_image_expires_at)
			values ($1, $2, $3, $4)
		`, channelID, input.ThumbnailPath, input.FullImagePath, input.FullImageExpiresAt); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		update video_channels
		set recognition_attempts = recognition_attempts + case when $8 then 1 else 0 end,
			recognition_result = case when $8 or nullif($1, '') is not null then nullif($1, '')::jsonb else recognition_result end,
			status = case when nullif($3, '') is null then status else $3 end,
			scene_type = case when nullif($4, '') is null then scene_type else $4 end,
			area_type = case when nullif($3, '') is null then area_type else nullif($5, '') end,
			area_number = case when nullif($3, '') is null then area_number else nullif($6, 0) end,
			area_note = case when nullif($3, '') is null then area_note else $7 end,
			updated_at = now()
		where id = $2
	`, input.RecognitionResult, channelID, input.Status, input.SceneType, input.AreaType, mustPositiveInt(input.AreaNumberText), strings.TrimSpace(input.AreaNote), input.CountAttempt); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	channels, err := s.listChannels(ctx, recorderID)
	if err != nil {
		return nil, err
	}
	for index := range channels {
		if channels[index].ID == channelID {
			return &channels[index], nil
		}
	}
	return nil, ErrNotFound
}

func (s *PostgresStore) ConfirmChannel(ctx context.Context, channelID int64, input ChannelConfirmationInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var storeID int64
	err = tx.QueryRowContext(ctx, `
		select r.store_id
		from video_channels c
		join video_recorders r on r.id = c.recorder_id
		where c.id = $1
	`, channelID).Scan(&storeID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if input.AreaType == "" {
		sceneType := input.SceneType
		if sceneType == "" {
			sceneType = SceneTypeUnknown
		}
		if _, err := tx.ExecContext(ctx, `
			update video_channels
			set status = $1,
				scene_type = $2,
				area_type = null,
				area_number = null,
				area_note = $4,
				area_id = null,
				confirmed_at = now(),
				updated_at = now()
			where id = $3
		`, ChannelStatusConfirmedNonBusiness, sceneType, channelID, strings.TrimSpace(input.AreaNote)); err != nil {
			return nil, err
		}
		if err := deleteUnusedVideoAreas(ctx, tx, storeID); err != nil {
			return nil, err
		}
	} else {
		number := 0
		if strings.TrimSpace(input.AreaNumber) != "" {
			parsedNumber, err := strconv.Atoi(strings.TrimSpace(input.AreaNumber))
			if err != nil || parsedNumber <= 0 {
				return nil, &ValidationError{Fields: map[string]string{"area_number": "区域编号必须是正整数"}}
			}
			number = parsedNumber
		} else if input.AreaType != AreaTypeVIPTreatment {
			return nil, &ValidationError{Fields: map[string]string{"area_number": "区域编号必须是正整数"}}
		}
		area, err := updateOrFindVideoArea(ctx, tx, storeID, channelID, input.AreaType, number)
		if err != nil {
			return nil, err
		}
		if area.Source != AreaSourceVideoChannel && area.Source != AreaSourceMultiple {
			if _, err := tx.ExecContext(ctx, `
				update store_areas
				set source = $1, updated_at = now()
				where id = $2
			`, AreaSourceMultiple, area.ID); err != nil {
				return nil, err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			update video_channels
			set status = $1,
				scene_type = $2,
				area_type = $3,
				area_number = $4,
				area_note = '',
				area_id = $5,
				confirmed_at = now(),
				updated_at = now()
			where id = $6
		`, ChannelStatusConfirmedBusiness, SceneType(input.AreaType), input.AreaType, number, area.ID, channelID); err != nil {
			return nil, err
		}
		if err := deleteUnusedVideoAreas(ctx, tx, storeID); err != nil {
			return nil, err
		}
	}

	if _, err := tx.ExecContext(ctx, `update stores set updated_at = now() where id = $1`, storeID); err != nil {
		return nil, err
	}
	if err := insertOperationLog(ctx, tx, "confirm_channel", "channel", channelID, storeID, "confirmed video channel mapping"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, storeID)
}

func (s *PostgresStore) UnlockChannelForEdit(ctx context.Context, channelID int64) (*Channel, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var storeID int64
	var status ChannelStatus
	var isActive bool
	err = tx.QueryRowContext(ctx, `
		select r.store_id, c.status, c.is_active
		from video_channels c
		join video_recorders r on r.id = c.recorder_id
		where c.id = $1
	`, channelID).Scan(&storeID, &status, &isActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status == ChannelStatusInactive || !isActive {
		return nil, &ValidationError{Fields: map[string]string{"channel": "通道已失效，无法编辑"}}
	}

	if _, err := tx.ExecContext(ctx, `
		update video_channels
		set status = $1,
			confirmed_at = null,
			updated_at = now()
		where id = $2
	`, ChannelStatusPendingConfirmation, channelID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update stores set updated_at = now() where id = $1`, storeID); err != nil {
		return nil, err
	}
	if err := insertOperationLog(ctx, tx, "unlock_channel", "channel", channelID, storeID, "unlocked video channel for editing"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetChannel(ctx, channelID)
}

func (s *PostgresStore) DeleteStore(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var name string
	if err := tx.QueryRowContext(ctx, `select name from stores where id = $1`, id).Scan(&name); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from stores where id = $1`, id); err != nil {
		return err
	}
	if err := insertOperationLog(ctx, tx, "delete", "store", id, id, fmt.Sprintf("deleted store %s", name)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) DeleteRecorder(ctx context.Context, recorderID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var storeID int64
	var deviceCode string
	if err := tx.QueryRowContext(ctx, `
		select store_id, device_code
		from video_recorders
		where id = $1
	`, recorderID).Scan(&storeID, &deviceCode); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `delete from video_recorders where id = $1`, recorderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `update stores set updated_at = now() where id = $1`, storeID); err != nil {
		return err
	}
	if err := insertOperationLog(ctx, tx, "delete", "recorder", storeID, storeID, fmt.Sprintf("deleted recorder %s", deviceCode)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) DeleteChannel(ctx context.Context, channelID int64) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var storeID int64
	var recorderID int64
	var channelNo int
	if err := tx.QueryRowContext(ctx, `
		select r.store_id, c.recorder_id, c.channel_no
		from video_channels c
		join video_recorders r on r.id = c.recorder_id
		where c.id = $1
	`, channelID).Scan(&storeID, &recorderID, &channelNo); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `delete from video_channels where id = $1`, channelID); err != nil {
		return nil, err
	}
	if err := deleteUnusedVideoAreas(ctx, tx, storeID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		update video_recorders
		set effective_channel_count = (
				select count(*)
				from video_channels
				where recorder_id = $1 and is_active and status <> $2
			),
			status = case
				when exists (
					select 1
					from video_channels
					where recorder_id = $1 and is_active and status <> $2
				) then $3
				else $4
			end,
			updated_at = now()
		where id = $1
	`, recorderID, ChannelStatusInactive, RecorderStatusOnline, RecorderStatusOffline); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update stores set updated_at = now() where id = $1`, storeID); err != nil {
		return nil, err
	}
	if err := insertOperationLog(ctx, tx, "delete", "channel", channelID, storeID, fmt.Sprintf("deleted channel %d", channelNo)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, storeID)
}

func (s *PostgresStore) CheckDuplicate(ctx context.Context, name string, excludeStoreID int64) (DuplicateCheckResult, error) {
	normalized := NormalizeStoreName(name)
	rows, err := s.db.QueryContext(ctx, `
		select id, name, normalized_name, overall_status, updated_at
		from stores
		where id <> $1
			and (
				normalized_name = $2
				or normalized_name like $3
				or $2 like '%' || normalized_name || '%'
			)
		order by updated_at desc
		limit 20
	`, excludeStoreID, normalized, "%"+normalized+"%")
	if err != nil {
		return DuplicateCheckResult{}, err
	}
	defer rows.Close()

	result := DuplicateCheckResult{SimilarMatches: []DuplicateMatch{}}
	for rows.Next() {
		var match DuplicateMatch
		if err := rows.Scan(&match.ID, &match.Name, &match.NormalizedName, &match.OverallStatus, &match.UpdatedAt); err != nil {
			return DuplicateCheckResult{}, err
		}
		if match.NormalizedName == normalized {
			match.Reason = "exact"
			if result.ExactMatch == nil {
				copy := match
				result.ExactMatch = &copy
			}
			continue
		}
		if IsSimilarStoreName(name, match.Name) {
			match.Reason = "similar"
			result.SimilarMatches = append(result.SimilarMatches, match)
		}
	}
	if err := rows.Err(); err != nil {
		return DuplicateCheckResult{}, err
	}
	return result, nil
}

func (s *PostgresStore) DeviceCodeExists(ctx context.Context, deviceCode string, excludeRecorderID int64) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		select id
		from video_recorders
		where device_code = $1 and id <> $2
		limit 1
	`, normalizeDeviceCode(deviceCode), excludeRecorderID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *PostgresStore) FindOrCreateArea(ctx context.Context, input AreaLookup, areaNumber int) (*Area, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var storeID int64
	if err := tx.QueryRowContext(ctx, `select id from stores where id = $1`, input.StoreID).Scan(&storeID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	area, err := queryArea(ctx, tx, input.StoreID, input.Type, areaNumber)
	if err == nil {
		if input.Source != "" && area.Source != input.Source {
			area.Source = AreaSourceMultiple
			if _, err := tx.ExecContext(ctx, `
				update store_areas set source = $1, updated_at = now()
				where id = $2
			`, area.Source, area.ID); err != nil {
				return nil, err
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return area, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	source := input.Source
	if source == "" {
		source = AreaSourceManual
	}
	var created Area
	err = tx.QueryRowContext(ctx, `
		insert into store_areas (store_id, area_type, area_number, display_name, source, status)
		values ($1, $2, $3, $4, $5, $6)
		returning id, store_id, area_type, area_number, display_name, source, status, created_at, updated_at
	`, input.StoreID, input.Type, areaNumber, areaDisplayName(input.Type, areaNumber), source, AreaStatusConfirmed).Scan(
		&created.ID,
		&created.StoreID,
		&created.Type,
		&created.Number,
		&created.DisplayName,
		&created.Source,
		&created.Status,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *PostgresStore) listAreas(ctx context.Context, storeID int64) ([]Area, error) {
	rows, err := s.db.QueryContext(ctx, `
		select
			a.id,
			a.store_id,
			a.area_type,
			a.area_number,
			a.display_name,
			a.source,
			a.status,
			annotation.box_x,
			annotation.box_y,
			annotation.box_width,
			annotation.box_height,
			a.created_at,
			a.updated_at
		from store_areas a
		left join lateral (
			select box_x, box_y, box_width, box_height
			from design_plan_annotations dpa
			join store_design_plans sdp on sdp.id = dpa.design_plan_id
			where dpa.area_id = a.id
				and sdp.store_id = a.store_id
			order by dpa.updated_at desc, dpa.id desc
			limit 1
		) annotation on true
		where a.store_id = $1
		order by a.area_type, a.area_number
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	areas := []Area{}
	for rows.Next() {
		var area Area
		var boxX, boxY, boxWidth, boxHeight sql.NullString
		if err := rows.Scan(
			&area.ID,
			&area.StoreID,
			&area.Type,
			&area.Number,
			&area.DisplayName,
			&area.Source,
			&area.Status,
			&boxX,
			&boxY,
			&boxWidth,
			&boxHeight,
			&area.CreatedAt,
			&area.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if box, ok := parseAreaBox(boxX, boxY, boxWidth, boxHeight); ok {
			area.Box = box
		}
		areas = append(areas, area)
	}
	return areas, rows.Err()
}

func parseAreaBox(boxX, boxY, boxWidth, boxHeight sql.NullString) (*AreaBox, bool) {
	if !boxX.Valid || !boxY.Valid || !boxWidth.Valid || !boxHeight.Valid {
		return nil, false
	}
	x, err := strconv.ParseFloat(boxX.String, 64)
	if err != nil {
		return nil, false
	}
	y, err := strconv.ParseFloat(boxY.String, 64)
	if err != nil {
		return nil, false
	}
	width, err := strconv.ParseFloat(boxWidth.String, 64)
	if err != nil {
		return nil, false
	}
	height, err := strconv.ParseFloat(boxHeight.String, 64)
	if err != nil {
		return nil, false
	}
	return &AreaBox{X: x, Y: y, Width: width, Height: height}, true
}

func (s *PostgresStore) listDesignPlans(ctx context.Context, storeID int64) ([]DesignPlan, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, store_id, upload_id, pdf_file_name, original_pdf_path, preview_image_path,
			thumbnail_path, page_count, recognition_status, created_at, updated_at
		from store_design_plans
		where store_id = $1
		order by id
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	plans := []DesignPlan{}
	for rows.Next() {
		var plan DesignPlan
		if err := rows.Scan(&plan.ID, &plan.StoreID, &plan.UploadID, &plan.PDFFileName, &plan.OriginalPDFPath, &plan.PreviewImagePath, &plan.ThumbnailPath, &plan.PageCount, &plan.RecognitionStatus, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, rows.Err()
}

func (s *PostgresStore) listRecorders(ctx context.Context, storeID int64) ([]Recorder, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, store_id, coalesce(ezviz_account_id, 0), device_code, status,
			effective_channel_count, last_scanned_at, created_at, updated_at
		from video_recorders
		where store_id = $1
		order by id
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	recorders := []Recorder{}
	recorderIndexes := map[int64]int{}
	for rows.Next() {
		var recorder Recorder
		if err := rows.Scan(&recorder.ID, &recorder.StoreID, &recorder.EzvizAccountID, &recorder.DeviceCode, &recorder.Status, &recorder.EffectiveChannelCount, &recorder.LastScannedAt, &recorder.CreatedAt, &recorder.UpdatedAt); err != nil {
			return nil, err
		}
		recorderIndexes[recorder.ID] = len(recorders)
		recorders = append(recorders, recorder)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	channels, err := s.listChannelsForStore(ctx, storeID)
	if err != nil {
		return nil, err
	}
	for _, channel := range channels {
		index, ok := recorderIndexes[channel.RecorderID]
		if !ok {
			continue
		}
		recorders[index].Channels = append(recorders[index].Channels, channel)
	}
	return recorders, nil
}

func (s *PostgresStore) queryRecorder(ctx context.Context, runner queryRunner, recorderID int64) (*Recorder, error) {
	var recorder Recorder
	err := runner.QueryRowContext(ctx, `
		select id, store_id, coalesce(ezviz_account_id, 0), device_code, status,
			effective_channel_count, last_scanned_at, created_at, updated_at
		from video_recorders
		where id = $1
	`, recorderID).Scan(
		&recorder.ID,
		&recorder.StoreID,
		&recorder.EzvizAccountID,
		&recorder.DeviceCode,
		&recorder.Status,
		&recorder.EffectiveChannelCount,
		&recorder.LastScannedAt,
		&recorder.CreatedAt,
		&recorder.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &recorder, nil
}

func (s *PostgresStore) listChannels(ctx context.Context, recorderID int64) ([]Channel, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, recorder_id, channel_no, channel_name, status, is_active,
			scene_type, coalesce(area_type, ''), coalesce(area_number, 0),
			coalesce(area_note, ''), coalesce(area_id, 0), recognition_attempts, coalesce(recognition_result::text, ''),
			snapshot.thumbnail_path, snapshot.full_image_path, snapshot.full_image_expires_at,
			confirmed_at, created_at, updated_at
		from video_channels
		left join lateral (
			select thumbnail_path, full_image_path, full_image_expires_at
			from channel_snapshots
			where channel_id = video_channels.id
			order by created_at desc, id desc
			limit 1
		) snapshot on true
		where recorder_id = $1
		order by channel_no
	`, recorderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	channels := []Channel{}
	for rows.Next() {
		channel, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, *channel)
	}
	return channels, rows.Err()
}

func (s *PostgresStore) listChannelsForStore(ctx context.Context, storeID int64) ([]Channel, error) {
	rows, err := s.db.QueryContext(ctx, `
		select c.id, c.recorder_id, c.channel_no, c.channel_name, c.status, c.is_active,
			c.scene_type, coalesce(c.area_type, ''), coalesce(c.area_number, 0),
			coalesce(c.area_note, ''), coalesce(c.area_id, 0), c.recognition_attempts, coalesce(c.recognition_result::text, ''),
			snapshot.thumbnail_path, snapshot.full_image_path, snapshot.full_image_expires_at,
			c.confirmed_at, c.created_at, c.updated_at
		from video_channels c
		join video_recorders r on r.id = c.recorder_id
		left join lateral (
			select thumbnail_path, full_image_path, full_image_expires_at
			from channel_snapshots
			where channel_id = c.id
			order by created_at desc, id desc
			limit 1
		) snapshot on true
		where r.store_id = $1
		order by c.recorder_id, c.channel_no
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	channels := []Channel{}
	for rows.Next() {
		channel, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, *channel)
	}
	return channels, rows.Err()
}

type channelScanner interface {
	Scan(dest ...any) error
}

func scanChannel(scanner channelScanner) (*Channel, error) {
	var channel Channel
	var thumbnailPath sql.NullString
	var fullImagePath sql.NullString
	var fullImageExpiresAt sql.NullTime
	if err := scanner.Scan(
		&channel.ID,
		&channel.RecorderID,
		&channel.ChannelNo,
		&channel.ChannelName,
		&channel.Status,
		&channel.IsActive,
		&channel.SceneType,
		&channel.AreaType,
		&channel.AreaNumber,
		&channel.AreaNote,
		&channel.AreaID,
		&channel.RecognitionAttempts,
		&channel.RecognitionResult,
		&thumbnailPath,
		&fullImagePath,
		&fullImageExpiresAt,
		&channel.ConfirmedAt,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	); err != nil {
		return nil, err
	}
	channel.ThumbnailURL = thumbnailPath.String
	channel.FullImageURL = fullImagePath.String
	if fullImageExpiresAt.Valid {
		channel.FullImageExpiresAt = &fullImageExpiresAt.Time
	}
	normalizeChannelSnapshot(&channel)
	return &channel, nil
}

type queryRunner interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func queryArea(ctx context.Context, tx queryRunner, storeID int64, areaType AreaType, number int) (*Area, error) {
	var area Area
	err := tx.QueryRowContext(ctx, `
		select id, store_id, area_type, area_number, display_name, source, status, created_at, updated_at
		from store_areas
		where store_id = $1 and area_type = $2 and area_number = $3
	`, storeID, areaType, number).Scan(
		&area.ID,
		&area.StoreID,
		&area.Type,
		&area.Number,
		&area.DisplayName,
		&area.Source,
		&area.Status,
		&area.CreatedAt,
		&area.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &area, nil
}

func upsertStoreDesignPlan(ctx context.Context, tx queryRunner, storeID int64, input SaveDesignPlanInput) (*DesignPlan, error) {
	var existingID int64
	err := tx.QueryRowContext(ctx, `
		select id
		from store_design_plans
		where store_id = $1
		order by updated_at desc, id desc
		limit 1
	`, storeID).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var plan DesignPlan
	if existingID != 0 {
		err = tx.QueryRowContext(ctx, `
			update store_design_plans
			set upload_id = $1,
				pdf_file_name = $2,
				original_pdf_path = $3,
				preview_image_path = $4,
				thumbnail_path = $5,
				page_count = $6,
				recognition_status = $7,
				recognition_result = nullif($8, '')::jsonb,
				updated_at = now()
			where id = $9
			returning id, store_id, upload_id, pdf_file_name, original_pdf_path, preview_image_path,
				thumbnail_path, page_count, recognition_status, created_at, updated_at
		`, input.UploadID, input.PDFFileName, input.OriginalPDFPath, input.PreviewImagePath, input.ThumbnailPath,
			input.PageCount, RecognitionStatusCompleted, input.RecognitionResult, existingID).Scan(
			&plan.ID,
			&plan.StoreID,
			&plan.UploadID,
			&plan.PDFFileName,
			&plan.OriginalPDFPath,
			&plan.PreviewImagePath,
			&plan.ThumbnailPath,
			&plan.PageCount,
			&plan.RecognitionStatus,
			&plan.CreatedAt,
			&plan.UpdatedAt,
		)
		return &plan, err
	}

	err = tx.QueryRowContext(ctx, `
		insert into store_design_plans (
			store_id, upload_id, pdf_file_name, original_pdf_path, preview_image_path,
			thumbnail_path, page_count, recognition_status, recognition_result
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, nullif($9, '')::jsonb)
		returning id, store_id, upload_id, pdf_file_name, original_pdf_path, preview_image_path,
			thumbnail_path, page_count, recognition_status, created_at, updated_at
	`, storeID, input.UploadID, input.PDFFileName, input.OriginalPDFPath, input.PreviewImagePath,
		input.ThumbnailPath, input.PageCount, RecognitionStatusCompleted, input.RecognitionResult).Scan(
		&plan.ID,
		&plan.StoreID,
		&plan.UploadID,
		&plan.PDFFileName,
		&plan.OriginalPDFPath,
		&plan.PreviewImagePath,
		&plan.ThumbnailPath,
		&plan.PageCount,
		&plan.RecognitionStatus,
		&plan.CreatedAt,
		&plan.UpdatedAt,
	)
	return &plan, err
}

func upsertDesignArea(ctx context.Context, tx queryRunner, storeID int64, input DesignAreaInput) (*Area, error) {
	number, _ := strconv.Atoi(strings.TrimSpace(input.NumberText))
	if input.ID != 0 {
		if area, err := updateDesignAreaByID(ctx, tx, storeID, input, number); err == nil {
			return area, nil
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	area, err := queryArea(ctx, tx, storeID, input.Type, number)
	if err == nil {
		return updateDesignAreaSource(ctx, tx, area, input, number)
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	var created Area
	err = tx.QueryRowContext(ctx, `
		insert into store_areas (store_id, area_type, area_number, display_name, source, status)
		values ($1, $2, $3, $4, $5, $6)
		returning id, store_id, area_type, area_number, display_name, source, status, created_at, updated_at
	`, storeID, input.Type, number, displayNameOrDefault(input.DisplayName, input.Type, number), AreaSourceDesignPlan, AreaStatusConfirmed).Scan(
		&created.ID,
		&created.StoreID,
		&created.Type,
		&created.Number,
		&created.DisplayName,
		&created.Source,
		&created.Status,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func updateDesignAreaByID(ctx context.Context, tx queryRunner, storeID int64, input DesignAreaInput, number int) (*Area, error) {
	var area Area
	err := tx.QueryRowContext(ctx, `
		select id, store_id, area_type, area_number, display_name, source, status, created_at, updated_at
		from store_areas
		where id = $1 and store_id = $2
	`, input.ID, storeID).Scan(
		&area.ID,
		&area.StoreID,
		&area.Type,
		&area.Number,
		&area.DisplayName,
		&area.Source,
		&area.Status,
		&area.CreatedAt,
		&area.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if area.Source == AreaSourceVideoChannel || area.Source == AreaSourceMultiple {
		nextSource := mergeAreaSource(area.Source, AreaSourceDesignPlan)
		if area.Source != nextSource {
			if _, err := tx.ExecContext(ctx, `
				update store_areas
				set source = $1, updated_at = now()
				where id = $2
			`, nextSource, area.ID); err != nil {
				return nil, err
			}
			area.Source = nextSource
		}
		return &area, nil
	}
	return updateDesignAreaSource(ctx, tx, &area, input, number)
}

func updateDesignAreaSource(ctx context.Context, tx queryRunner, area *Area, input DesignAreaInput, number int) (*Area, error) {
	nextSource := mergeAreaSource(area.Source, AreaSourceDesignPlan)
	err := tx.QueryRowContext(ctx, `
		update store_areas
		set area_type = $1,
			area_number = $2,
			display_name = $3,
			source = $4,
			status = $5,
			updated_at = now()
		where id = $6
		returning id, store_id, area_type, area_number, display_name, source, status, created_at, updated_at
	`, input.Type, number, displayNameOrDefault(input.DisplayName, input.Type, number), nextSource, AreaStatusConfirmed, area.ID).Scan(
		&area.ID,
		&area.StoreID,
		&area.Type,
		&area.Number,
		&area.DisplayName,
		&area.Source,
		&area.Status,
		&area.CreatedAt,
		&area.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return area, nil
}

func upsertDesignAnnotation(ctx context.Context, tx queryRunner, planID int64, areaID int64, box *AreaBox) error {
	if box == nil {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		insert into design_plan_annotations (
			design_plan_id, area_id, box_x, box_y, box_width, box_height, status
		)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (design_plan_id, area_id)
		do update set
			box_x = excluded.box_x,
			box_y = excluded.box_y,
			box_width = excluded.box_width,
			box_height = excluded.box_height,
			status = excluded.status,
			updated_at = now()
	`, planID, areaID, box.X, box.Y, box.Width, box.Height, "confirmed")
	return err
}

func updateOrFindVideoArea(ctx context.Context, tx queryRunner, storeID int64, channelID int64, areaType AreaType, areaNumber int) (*Area, error) {
	var currentAreaID int64
	if err := tx.QueryRowContext(ctx, `
		select coalesce(area_id, 0)
		from video_channels
		where id = $1
	`, channelID).Scan(&currentAreaID); err != nil {
		return nil, err
	}

	if currentAreaID != 0 {
		var existingID int64
		err := tx.QueryRowContext(ctx, `
			select id
			from store_areas
			where store_id = $1 and area_type = $2 and area_number = $3
		`, storeID, areaType, areaNumber).Scan(&existingID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil && existingID != currentAreaID {
			area, queryErr := queryArea(ctx, tx, storeID, areaType, areaNumber)
			if queryErr != nil {
				return nil, queryErr
			}
			if area.Source != AreaSourceVideoChannel && area.Source != AreaSourceMultiple {
				if _, updateErr := tx.ExecContext(ctx, `
					update store_areas
					set source = $1, updated_at = now()
					where id = $2
				`, AreaSourceMultiple, area.ID); updateErr != nil {
					return nil, updateErr
				}
				area.Source = AreaSourceMultiple
			}
			return area, nil
		}

		var area Area
		var annotationCount int
		err = tx.QueryRowContext(ctx, `
			select a.id, a.store_id, a.area_type, a.area_number, a.display_name, a.source, a.status,
				a.created_at, a.updated_at, count(dpa.id)
			from store_areas a
			left join design_plan_annotations dpa on dpa.area_id = a.id
			where a.id = $1 and a.store_id = $2
			group by a.id
		`, currentAreaID, storeID).Scan(
			&area.ID,
			&area.StoreID,
			&area.Type,
			&area.Number,
			&area.DisplayName,
			&area.Source,
			&area.Status,
			&area.CreatedAt,
			&area.UpdatedAt,
			&annotationCount,
		)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil && (area.Source == AreaSourceVideoChannel || area.Source == AreaSourceMultiple) {
			err = tx.QueryRowContext(ctx, `
				update store_areas
				set area_type = $1,
					area_number = $2,
					display_name = $3,
					updated_at = now()
				where id = $4
				returning id, store_id, area_type, area_number, display_name, source, status, created_at, updated_at
			`, areaType, areaNumber, areaDisplayName(areaType, areaNumber), currentAreaID).Scan(
				&area.ID,
				&area.StoreID,
				&area.Type,
				&area.Number,
				&area.DisplayName,
				&area.Source,
				&area.Status,
				&area.CreatedAt,
				&area.UpdatedAt,
			)
			if err != nil {
				return nil, err
			}
			return &area, nil
		}
	}

	area, err := queryArea(ctx, tx, storeID, areaType, areaNumber)
	if errors.Is(err, ErrNotFound) {
		createdArea := Area{}
		err = tx.QueryRowContext(ctx, `
			insert into store_areas (store_id, area_type, area_number, display_name, source, status)
			values ($1, $2, $3, $4, $5, $6)
			returning id, store_id, area_type, area_number, display_name, source, status, created_at, updated_at
		`, storeID, areaType, areaNumber, areaDisplayName(areaType, areaNumber), AreaSourceVideoChannel, AreaStatusConfirmed).Scan(
			&createdArea.ID,
			&createdArea.StoreID,
			&createdArea.Type,
			&createdArea.Number,
			&createdArea.DisplayName,
			&createdArea.Source,
			&createdArea.Status,
			&createdArea.CreatedAt,
			&createdArea.UpdatedAt,
		)
		area = &createdArea
	}
	if err != nil {
		return nil, err
	}
	return area, nil
}

func deleteUnusedVideoAreas(ctx context.Context, tx queryRunner, storeID int64) error {
	_, err := tx.ExecContext(ctx, `
		delete from store_areas a
		where a.store_id = $1
			and a.source = $2
			and not exists (
				select 1 from design_plan_annotations dpa
				where dpa.area_id = a.id
			)
			and not exists (
				select 1 from video_channels vc
				where vc.area_id = a.id
			)
	`, storeID, AreaSourceVideoChannel)
	return err
}

func insertOperationLog(ctx context.Context, tx queryRunner, action string, entityType string, entityID int64, storeID int64, summary string) error {
	_, err := tx.ExecContext(ctx, `
		insert into operation_logs (action, entity_type, entity_id, store_id, actor, summary)
		values ($1, $2, $3, $4, 'admin', $5)
	`, action, entityType, entityID, storeID, summary)
	return err
}

func normalizeFilters(filters StoreFilters) StoreFilters {
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

func storeListItem(store Store) StoreListItem {
	item := StoreListItem{
		ID:               store.ID,
		City:             store.City,
		Name:             store.Name,
		ExternalOrgID:    store.ExternalOrgID,
		DesignPlanStatus: store.DesignPlanStatus,
		OverallStatus:    store.OverallStatus,
		RecorderCount:    len(store.Recorders),
		UpdatedAt:        store.UpdatedAt,
	}
	for _, recorder := range store.Recorders {
		for _, channel := range recorder.Channels {
			if channel.IsActive {
				item.ChannelCount++
			}
		}
	}
	item.ChannelsFullyConfirmed = activeChannelsFullyConfirmed(store.Recorders)
	for _, area := range store.Areas {
		item.AreaCount++
		switch area.Type {
		case AreaTypeTreatment, AreaTypeVIPTreatment:
			item.TreatmentCount++
		case AreaTypeConsultation:
			item.ConsultationCount++
		case AreaTypeBeauty:
			item.BeautyCount++
		}
	}
	return item
}

func activeChannelsFullyConfirmed(recorders []Recorder) bool {
	hasActiveChannel := false
	for _, recorder := range recorders {
		for _, channel := range recorder.Channels {
			if !channel.IsActive {
				continue
			}
			hasActiveChannel = true
			if channel.Status != ChannelStatusConfirmedBusiness && channel.Status != ChannelStatusConfirmedNonBusiness {
				return false
			}
		}
	}
	return hasActiveChannel
}

func cloneStore(store Store) Store {
	copy := store
	copy.Areas = append([]Area(nil), store.Areas...)
	copy.DesignPlans = append([]DesignPlan(nil), store.DesignPlans...)
	copy.Recorders = append([]Recorder(nil), store.Recorders...)
	for index := range copy.Recorders {
		copy.Recorders[index].Channels = append([]Channel(nil), copy.Recorders[index].Channels...)
		for channelIndex := range copy.Recorders[index].Channels {
			normalizeChannelSnapshot(&copy.Recorders[index].Channels[channelIndex])
		}
	}
	return copy
}

func normalizeChannelSnapshot(channel *Channel) {
	if channel == nil || channel.FullImageExpiresAt == nil {
		return
	}
	if time.Now().Before(*channel.FullImageExpiresAt) {
		return
	}
	if isSystemChannelSnapshotURL(channel.ThumbnailURL) || isSystemChannelSnapshotURL(channel.FullImageURL) {
		return
	}
	channel.ThumbnailURL = ""
	channel.FullImageURL = ""
}

func isSystemChannelSnapshotURL(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "/api/store-space/channel-snapshots/")
}

func areaDisplayName(areaType AreaType, number int) string {
	switch areaType {
	case AreaTypeTreatment:
		return fmt.Sprintf("治疗室 %d", number)
	case AreaTypeVIPTreatment:
		if number > 0 {
			return fmt.Sprintf("VIP治疗室 %d", number)
		}
		return "VIP治疗室"
	case AreaTypeConsultation:
		return fmt.Sprintf("面诊室 %d", number)
	case AreaTypeBeauty:
		return fmt.Sprintf("生美 %d", number)
	default:
		return fmt.Sprintf("%s %d", areaType, number)
	}
}
