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
	citySet := map[string]struct{}{}
	for _, store := range s.stores {
		if !MatchesStoreSearch(store.Name, store.NormalizedName, filters.Query) {
			continue
		}
		citySet[storeCityOption(store.City)] = struct{}{}
		if !matchesStoreCity(store.City, filters.City) {
			continue
		}
		items = append(items, storeListItem(*store))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	total := len(items)
	summary := summarizeStoreListItems(items)
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
		Summary:  summary,
		Cities:   sortedCityOptions(citySet),
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
		ShortName:        strings.TrimSpace(input.ShortName),
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
	store.ShortName = strings.TrimSpace(input.ShortName)
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
					channel.BedLabel = ""
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
				channel.BedLabel = strings.TrimSpace(input.BedLabel)
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
			ShortName:      store.ShortName,
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
		&channel.BedLabel,
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

func normalizeFilters(filters StoreFilters) StoreFilters {
	filters.Query = strings.TrimSpace(filters.Query)
	filters.City = strings.TrimSpace(filters.City)
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

func matchesStoreCity(storeCity string, filterCity string) bool {
	filterCity = strings.TrimSpace(filterCity)
	if filterCity == "" {
		return true
	}
	return storeCityOption(storeCity) == filterCity
}

func storeCityOption(city string) string {
	city = strings.TrimSpace(city)
	if city == "" {
		return "未设置"
	}
	return city
}

func sortedCityOptions(citySet map[string]struct{}) []string {
	cities := make([]string, 0, len(citySet))
	for city := range citySet {
		cities = append(cities, city)
	}
	sort.Slice(cities, func(i, j int) bool {
		return cities[i] < cities[j]
	})
	return cities
}

func storeListItem(store Store) StoreListItem {
	item := StoreListItem{
		ID:               store.ID,
		City:             store.City,
		Name:             store.Name,
		ShortName:        store.ShortName,
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

func summarizeStoreListItems(items []StoreListItem) StoreListSummary {
	summary := StoreListSummary{StoreCount: len(items)}
	for _, item := range items {
		summary.TreatmentCount += item.TreatmentCount
		summary.ConsultationCount += item.ConsultationCount
		summary.BeautyCount += item.BeautyCount
	}
	return summary
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
