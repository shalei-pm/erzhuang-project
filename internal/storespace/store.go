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
	ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error)
	GetStore(ctx context.Context, id int64) (*Store, error)
	CreateStore(ctx context.Context, input CreateStoreInput) (*Store, error)
	AddRecorder(ctx context.Context, storeID int64, input AddRecorderInput) (*Store, error)
	GetRecorder(ctx context.Context, recorderID int64) (*Recorder, error)
	GetEzvizAccount(ctx context.Context, accountID int64) (*EzvizAccount, error)
	ReplaceRecorderChannels(ctx context.Context, recorderID int64, channels []ChannelInput) (*Recorder, error)
	DeleteStore(ctx context.Context, id int64) error
	DeleteRecorder(ctx context.Context, recorderID int64) error
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
			nextChannels := make([]Channel, 0, len(channels))
			for index, channel := range channels {
				if !channel.IsActive {
					continue
				}
				nextChannels = append(nextChannels, Channel{
					ID:                  int64(index + 1),
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
			recorder.Channels = nextChannels
			recorder.EffectiveChannelCount = len(nextChannels)
			recorder.LastScannedAt = &now
			if len(nextChannels) > 0 {
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
			count(distinct a.id) filter (where a.area_type = 'treatment') as treatment_count,
			count(distinct a.id) filter (where a.area_type = 'consultation') as consultation_count,
			count(distinct a.id) filter (where a.area_type = 'beauty') as beauty_count
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
			&item.TreatmentCount,
			&item.ConsultationCount,
			&item.BeautyCount,
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
	return &store, nil
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

	if _, err := tx.ExecContext(ctx, `delete from video_channels where recorder_id = $1`, recorderID); err != nil {
		return nil, err
	}
	for _, channel := range channels {
		if !channel.IsActive || channel.ChannelNo <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			insert into video_channels (recorder_id, channel_no, channel_name, status, is_active, scene_type)
			values ($1, $2, $3, $4, true, $5)
		`, recorderID, channel.ChannelNo, strings.TrimSpace(channel.ChannelName), ChannelStatusPendingRecognition, SceneTypeUnknown); err != nil {
			return nil, err
		}
	}

	status := RecorderStatusOffline
	if len(channels) > 0 {
		status = RecorderStatusOnline
	}
	if _, err := tx.ExecContext(ctx, `
		update video_recorders
		set status = $1,
			effective_channel_count = (
				select count(*)
				from video_channels
				where recorder_id = $2 and is_active
			),
			last_scanned_at = now(),
			updated_at = now()
		where id = $2
	`, status, recorderID); err != nil {
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
	for rows.Next() {
		var recorder Recorder
		if err := rows.Scan(&recorder.ID, &recorder.StoreID, &recorder.EzvizAccountID, &recorder.DeviceCode, &recorder.Status, &recorder.EffectiveChannelCount, &recorder.LastScannedAt, &recorder.CreatedAt, &recorder.UpdatedAt); err != nil {
			return nil, err
		}
		recorders = append(recorders, recorder)
	}
	return recorders, rows.Err()
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
			coalesce(area_id, 0), recognition_attempts, confirmed_at, created_at, updated_at
		from video_channels
		where recorder_id = $1
		order by channel_no
	`, recorderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	channels := []Channel{}
	for rows.Next() {
		var channel Channel
		if err := rows.Scan(
			&channel.ID,
			&channel.RecorderID,
			&channel.ChannelNo,
			&channel.ChannelName,
			&channel.Status,
			&channel.IsActive,
			&channel.SceneType,
			&channel.AreaType,
			&channel.AreaNumber,
			&channel.AreaID,
			&channel.RecognitionAttempts,
			&channel.ConfirmedAt,
			&channel.CreatedAt,
			&channel.UpdatedAt,
		); err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
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
	for _, area := range store.Areas {
		switch area.Type {
		case AreaTypeTreatment:
			item.TreatmentCount++
		case AreaTypeConsultation:
			item.ConsultationCount++
		case AreaTypeBeauty:
			item.BeautyCount++
		}
	}
	return item
}

func cloneStore(store Store) Store {
	copy := store
	copy.Areas = append([]Area(nil), store.Areas...)
	copy.DesignPlans = append([]DesignPlan(nil), store.DesignPlans...)
	copy.Recorders = append([]Recorder(nil), store.Recorders...)
	for index := range copy.Recorders {
		copy.Recorders[index].Channels = append([]Channel(nil), copy.Recorders[index].Channels...)
	}
	return copy
}

func areaDisplayName(areaType AreaType, number int) string {
	switch areaType {
	case AreaTypeTreatment:
		return fmt.Sprintf("治疗室 %d", number)
	case AreaTypeConsultation:
		return fmt.Sprintf("面诊室 %d", number)
	case AreaTypeBeauty:
		return fmt.Sprintf("生美 %d", number)
	default:
		return fmt.Sprintf("%s %d", areaType, number)
	}
}
