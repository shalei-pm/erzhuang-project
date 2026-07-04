package storespace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
	"github.com/shalei-pm/erzhuang-project/internal/h5monitor"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) ListEzvizAccounts(ctx context.Context) ([]EzvizAccount, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, account_name, status, last_verified_at, created_at, updated_at
		from tb_ezviz_accounts
		order by account_name, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := []EzvizAccount{}
	for rows.Next() {
		var account EzvizAccount
		if err := rows.Scan(&account.ID, &account.AccountName, &account.Status, &account.LastVerifiedAt, &account.CreatedAt, &account.UpdatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (s *MySQLStore) CreateEzvizAccount(ctx context.Context, input CreateEzvizAccountInput) (*EzvizAccount, error) {
	result, err := s.db.ExecContext(ctx, `
		insert into tb_ezviz_accounts (account_name, app_key, app_secret_ciphertext, access_token_ciphertext, status)
		values (?, '', '', '', 'unverified')
	`, strings.TrimSpace(input.AccountName))
	if err != nil {
		return nil, err
	}
	accountID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetEzvizAccount(ctx, accountID)
}

func (s *MySQLStore) EzvizAccountNameExists(ctx context.Context, accountName string) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		select id
		from tb_ezviz_accounts
		where account_name = ?
		limit 1
	`, strings.TrimSpace(accountName)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *MySQLStore) UpsertEzvizAccountName(ctx context.Context, accountName string) error {
	cleanName := strings.TrimSpace(accountName)
	if cleanName == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		insert into tb_ezviz_accounts (account_name, status)
		values (?, 'available')
		on duplicate key update
			status = 'available',
			updated_at = current_timestamp(3)
	`, cleanName)
	return err
}

func (s *MySQLStore) ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error) {
	filters = normalizeFilters(filters)
	whereSQL, whereArgs := mysqlStoreListWhere(filters)
	offset := (filters.Page - 1) * filters.PageSize

	var total int
	countArgs := append([]any{}, whereArgs...)
	if err := s.db.QueryRowContext(ctx, `
		select count(*)
		from tb_stores s
	`+whereSQL, countArgs...).Scan(&total); err != nil {
		return StoreListResult{}, fmt.Errorf("mysql list stores count: %w", err)
	}

	rawLike, hasSearch := mysqlStoreSearchLike(filters.Query)
	cities, err := s.listStoreCities(ctx, rawLike, hasSearch)
	if err != nil {
		cities = []string{}
	}

	rowArgs := append([]any{}, whereArgs...)
	rowArgs = append(rowArgs, filters.PageSize, offset)
	rows, err := s.db.QueryContext(ctx, `
		select
			s.id,
			coalesce(s.city, ''),
			coalesce(s.name, ''),
			coalesce(s.short_name, ''),
			coalesce(s.external_org_id, ''),
			coalesce(s.design_plan_status, 'not_uploaded'),
			coalesce(s.overall_status, 'partial'),
			date_format(coalesce(s.updated_at, s.created_at, current_timestamp(3)), '%Y-%m-%d %H:%i:%s.%f')
		from tb_stores s
	`+whereSQL+`
		order by coalesce(s.updated_at, s.created_at, current_timestamp(3)) desc
		limit ? offset ?
	`, rowArgs...)
	if err != nil {
		return StoreListResult{}, fmt.Errorf("mysql list stores rows: %w", err)
	}
	defer rows.Close()

	items := []StoreListItem{}
	for rows.Next() {
		var item StoreListItem
		var updatedAt mysqlDateTimeText
		if err := rows.Scan(&item.ID, &item.City, &item.Name, &item.ShortName, &item.ExternalOrgID, &item.DesignPlanStatus, &item.OverallStatus, &updatedAt); err != nil {
			return StoreListResult{}, fmt.Errorf("mysql list stores scan: %w", err)
		}
		item.UpdatedAt = updatedAt.Time()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return StoreListResult{}, fmt.Errorf("mysql list stores read rows: %w", err)
	}
	for index := range items {
		if err := s.populateStoreListItemMetrics(ctx, &items[index]); err != nil {
			items[index].ChannelsFullyConfirmed = false
		}
	}
	summary := summarizeStoreListItems(items)
	summary.StoreCount = total
	return StoreListResult{Items: items, Page: filters.Page, PageSize: filters.PageSize, Total: total, Summary: summary, Cities: cities}, nil
}

func mysqlStoreListWhere(filters StoreFilters) (string, []any) {
	rawLike, hasSearch := mysqlStoreSearchLike(filters.Query)
	city := strings.TrimSpace(filters.City)
	whereSQL := ""
	args := []any{}
	if city != "" {
		whereSQL = mysqlAppendStoreListClause(whereSQL, "coalesce(nullif(trim(s.city), ''), '未设置') = ?")
		args = append(args, city)
	}
	if hasSearch {
		whereSQL = mysqlAppendStoreListClause(whereSQL, "(replace(lower(coalesce(s.name, '')), ' ', '') like ? or coalesce(s.external_org_id, '') like ?)")
		args = append(args, rawLike, rawLike)
	}
	return whereSQL, args
}

func mysqlAppendStoreListClause(whereSQL string, clause string) string {
	if whereSQL == "" {
		return "\n\t\twhere " + clause
	}
	return whereSQL + "\n\t\t\tand " + clause
}

func mysqlStoreSearchLike(query string) (string, bool) {
	cleanQuery := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(query), " ", ""))
	if cleanQuery == "" {
		return "", false
	}
	return "%" + cleanQuery + "%", true
}

type mysqlDateTimeText struct {
	value string
	valid bool
}

func (t *mysqlDateTimeText) Scan(value any) error {
	if value == nil {
		t.value = ""
		t.valid = false
		return nil
	}
	t.valid = true
	switch v := value.(type) {
	case time.Time:
		t.value = v.Format("2006-01-02 15:04:05.999999")
	case []byte:
		t.value = string(v)
	case string:
		t.value = v
	default:
		return fmt.Errorf("unsupported mysql datetime type %T", value)
	}
	return nil
}

func (t mysqlDateTimeText) Time() time.Time {
	if !t.valid {
		return time.Time{}
	}
	value := strings.TrimSpace(t.value)
	if value == "" || strings.HasPrefix(value, "0000-00-00") {
		return time.Time{}
	}
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
	} {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func (s *MySQLStore) populateStoreListItemMetrics(ctx context.Context, item *StoreListItem) error {
	if err := s.db.QueryRowContext(ctx, `
		select count(*)
		from tb_video_recorders
		where store_id = ?
	`, item.ID).Scan(&item.RecorderCount); err != nil {
		return err
	}

	var unconfirmedCount int
	if err := s.db.QueryRowContext(ctx, `
		select
			count(*),
			coalesce(sum(case when c.status not in ('confirmed_business', 'confirmed_non_business') then 1 else 0 end), 0)
		from tb_video_channels c, tb_video_recorders r
		where c.is_active = 1
			and c.recorder_id = r.id
			and r.store_id = ?
	`, item.ID).Scan(&item.ChannelCount, &unconfirmedCount); err != nil {
		return err
	}
	item.ChannelsFullyConfirmed = item.ChannelCount > 0 && unconfirmedCount == 0

	if err := s.db.QueryRowContext(ctx, `
		select
			count(*),
			coalesce(sum(case when area_type in ('treatment', 'vip_treatment') then 1 else 0 end), 0),
			coalesce(sum(case when area_type = 'consultation' then 1 else 0 end), 0),
			coalesce(sum(case when area_type = 'beauty' then 1 else 0 end), 0)
		from tb_store_areas
		where store_id = ?
	`, item.ID).Scan(&item.AreaCount, &item.TreatmentCount, &item.ConsultationCount, &item.BeautyCount); err != nil {
		return err
	}
	return nil
}

func (s *MySQLStore) GetStore(ctx context.Context, id int64) (*Store, error) {
	store, err := s.getStoreBase(ctx, id)
	if err != nil {
		return nil, err
	}
	store.Areas, err = s.listAreas(ctx, id)
	if err != nil {
		return nil, err
	}
	store.DesignPlans, err = s.listDesignPlans(ctx, id)
	if err != nil {
		return nil, err
	}
	store.Recorders, err = s.listRecorders(ctx, id)
	if err != nil {
		return nil, err
	}
	return store, nil
}

func (s *MySQLStore) GetStoreDesignPlanData(ctx context.Context, id int64) (*Store, error) {
	store, err := s.getStoreBase(ctx, id)
	if err != nil {
		return nil, err
	}
	store.Areas, err = s.listAreas(ctx, id)
	if err != nil {
		return nil, err
	}
	store.DesignPlans, err = s.listDesignPlans(ctx, id)
	if err != nil {
		return nil, err
	}
	return store, nil
}

func (s *MySQLStore) GetStoreChannelData(ctx context.Context, id int64) (*Store, error) {
	store, err := s.getStoreBase(ctx, id)
	if err != nil {
		return nil, err
	}
	store.Recorders, err = s.listRecorders(ctx, id)
	if err != nil {
		return nil, err
	}
	return store, nil
}

func (s *MySQLStore) CreateStore(ctx context.Context, input CreateStoreInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	designPlanStatus := DesignPlanStatusNotUploaded
	if strings.TrimSpace(input.DesignPlanUploadID) != "" {
		designPlanStatus = DesignPlanStatusPendingRecognition
	}

	result, err := tx.ExecContext(ctx, `
		insert into tb_stores (city, name, short_name, normalized_name, external_org_id, design_plan_status, overall_status)
		values (?, ?, ?, ?, ?, ?, ?)
	`, strings.TrimSpace(input.City), strings.TrimSpace(input.Name), strings.TrimSpace(input.ShortName), NormalizeStoreName(input.Name), strings.TrimSpace(input.ExternalOrgID),
		designPlanStatus, OverallStatusPartial)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	if uploadID := strings.TrimSpace(input.DesignPlanUploadID); uploadID != "" {
		if _, err := tx.ExecContext(ctx, `
			insert into tb_store_design_plans (store_id, upload_id, recognition_status)
			values (?, ?, ?)
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
			insert into tb_video_recorders (store_id, ezviz_account_id, device_code, status)
			values (?, nullif(?, 0), ?, ?)
		`, id, recorder.EzvizAccountID, code, RecorderStatusOffline); err != nil {
			return nil, err
		}
	}

	if err := mysqlInsertOperationLog(ctx, tx, "create", "store", id, id, fmt.Sprintf("created store %s", strings.TrimSpace(input.Name))); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, id)
}

func (s *MySQLStore) UpdateStoreBasicInfo(ctx context.Context, id int64, input UpdateStoreBasicInfoInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		update tb_stores
		set city = ?,
			name = ?,
			normalized_name = ?,
			short_name = ?,
			external_org_id = ?,
			updated_at = current_timestamp(3)
		where id = ?
	`, strings.TrimSpace(input.City), strings.TrimSpace(input.Name), NormalizeStoreName(input.Name), strings.TrimSpace(input.ShortName), strings.TrimSpace(input.ExternalOrgID), id)
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
	if err := mysqlInsertOperationLog(ctx, tx, "update", "store", id, id, fmt.Sprintf("updated store basic info %s", strings.TrimSpace(input.Name))); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, id)
}

func (s *MySQLStore) SaveDesignPlan(ctx context.Context, storeID int64, input SaveDesignPlanInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var existingStoreID int64
	if err := tx.QueryRowContext(ctx, `select id from tb_stores where id = ?`, storeID).Scan(&existingStoreID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	plan, err := mysqlUpsertStoreDesignPlan(ctx, tx, storeID, input)
	if err != nil {
		return nil, err
	}
	for _, areaInput := range input.Areas {
		area, err := mysqlUpsertDesignArea(ctx, tx, storeID, areaInput)
		if err != nil {
			return nil, err
		}
		if err := mysqlUpsertDesignAnnotation(ctx, tx, plan.ID, area.ID, areaInput.Box); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		update tb_stores
		set design_plan_status = ?,
			updated_at = current_timestamp(3)
		where id = ?
	`, DesignPlanStatusCompleted, storeID); err != nil {
		return nil, err
	}
	if err := mysqlInsertOperationLog(ctx, tx, "save_design_plan", "store", storeID, storeID, "saved design plan annotations"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, storeID)
}

func (s *MySQLStore) AddRecorder(ctx context.Context, storeID int64, input AddRecorderInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var existingStoreID int64
	if err := tx.QueryRowContext(ctx, `select id from tb_stores where id = ?`, storeID).Scan(&existingStoreID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	code := normalizeDeviceCode(input.DeviceCode)
	result, err := tx.ExecContext(ctx, `
		insert into tb_video_recorders (store_id, ezviz_account_id, device_code, status)
		values (?, nullif(?, 0), ?, ?)
	`, storeID, input.EzvizAccountID, code, RecorderStatusOffline)
	if err != nil {
		return nil, err
	}
	recorderID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update tb_stores set updated_at = current_timestamp(3) where id = ?`, storeID); err != nil {
		return nil, err
	}
	if err := mysqlInsertOperationLog(ctx, tx, "create", "recorder", recorderID, storeID, fmt.Sprintf("added recorder %s", code)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, storeID)
}

func (s *MySQLStore) GetRecorder(ctx context.Context, recorderID int64) (*Recorder, error) {
	recorder, err := s.queryRecorder(ctx, recorderID)
	if err != nil {
		return nil, err
	}
	recorder.Channels, err = s.listChannels(ctx, recorder.ID)
	if err != nil {
		return nil, err
	}
	return recorder, nil
}

func (s *MySQLStore) GetChannelContext(ctx context.Context, channelID int64) (*Channel, *Recorder, *EzvizAccount, error) {
	channel, err := s.GetChannel(ctx, channelID)
	if err != nil {
		return nil, nil, nil, err
	}
	recorder, err := s.GetRecorder(ctx, channel.RecorderID)
	if err != nil {
		return nil, nil, nil, err
	}
	account, err := s.GetEzvizAccount(ctx, recorder.EzvizAccountID)
	if err != nil {
		return nil, nil, nil, err
	}
	return channel, recorder, account, nil
}

func (s *MySQLStore) GetEzvizAccount(ctx context.Context, accountID int64) (*EzvizAccount, error) {
	var account EzvizAccount
	err := s.db.QueryRowContext(ctx, `
		select id, account_name, status, last_verified_at, created_at, updated_at
		from tb_ezviz_accounts
		where id = ?
	`, accountID).Scan(&account.ID, &account.AccountName, &account.Status, &account.LastVerifiedAt, &account.CreatedAt, &account.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *MySQLStore) ReplaceRecorderChannels(ctx context.Context, recorderID int64, channels []ChannelInput) (*Recorder, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	recorder, err := s.queryRecorder(ctx, recorderID)
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
				insert into tb_video_channels (recorder_id, channel_no, channel_name, status, is_active, scene_type)
				values (?, ?, ?, ?, true, ?)
				on duplicate key update
					channel_name = values(channel_name),
					is_active = true,
					status = case
						when status = ? and area_type is not null then ?
						when status = ? and (
							area_id is not null
							or area_number is not null
							or confirmed_at is not null
						) then ?
						when status = ? then ?
						else status
					end,
					scene_type = case
						when status = ? and (
							area_id is not null
							or area_type is not null
							or area_number is not null
							or confirmed_at is not null
						) then scene_type
						when status = ? then ?
						else scene_type
					end,
					area_type = case
						when status = ? and (
							area_id is not null
							or area_type is not null
							or area_number is not null
							or confirmed_at is not null
						) then area_type
						when status = ? then null
						else area_type
					end,
					area_number = case
						when status = ? and (
							area_id is not null
							or area_type is not null
							or area_number is not null
							or confirmed_at is not null
						) then area_number
						when status = ? then null
						else area_number
					end,
					area_id = case
						when status = ? and (
							area_id is not null
							or area_type is not null
							or area_number is not null
							or confirmed_at is not null
						) then area_id
						when status = ? then null
						else area_id
					end,
					confirmed_at = case
						when status = ? and (
							area_id is not null
							or area_type is not null
							or area_number is not null
							or confirmed_at is not null
						) then confirmed_at
						when status = ? then null
						else confirmed_at
					end,
					area_note = case
						when status = ? and (
							area_id is not null
							or area_type is not null
							or area_number is not null
							or confirmed_at is not null
						) then area_note
						when status = ? then ''
						else area_note
					end,
					bed_label = case
						when status = ? and (
							area_id is not null
							or area_type is not null
							or area_number is not null
							or confirmed_at is not null
						) then bed_label
						when status = ? then ''
						else bed_label
					end,
					updated_at = current_timestamp(3)
			`, recorderID, channel.ChannelNo, strings.TrimSpace(channel.ChannelName), ChannelStatusPendingRecognition, SceneTypeUnknown,
				ChannelStatusInactive, ChannelStatusConfirmedBusiness,
				ChannelStatusInactive, ChannelStatusConfirmedNonBusiness,
				ChannelStatusInactive, ChannelStatusPendingRecognition,
				ChannelStatusInactive, ChannelStatusInactive, SceneTypeUnknown,
				ChannelStatusInactive, ChannelStatusInactive,
				ChannelStatusInactive, ChannelStatusInactive,
				ChannelStatusInactive, ChannelStatusInactive,
				ChannelStatusInactive, ChannelStatusInactive,
				ChannelStatusInactive, ChannelStatusInactive,
				ChannelStatusInactive, ChannelStatusInactive,
			); err != nil {
				return nil, err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			update tb_video_channels
			set is_active = false,
				status = ?,
				updated_at = current_timestamp(3)
			where recorder_id = ? and channel_no = ?
		`, ChannelStatusInactive, recorderID, channel.ChannelNo); err != nil {
			return nil, err
		}
	}

	if err := mysqlDeactivateMissingChannels(ctx, tx, recorderID, scannedNumbers); err != nil {
		return nil, err
	}
	activeCount, err := mysqlActiveChannelCount(ctx, tx, recorderID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		update tb_video_recorders
		set status = ?,
			effective_channel_count = ?,
			last_scanned_at = current_timestamp(3),
			updated_at = current_timestamp(3)
		where id = ?
	`, mysqlRecorderStatusForActiveCount(activeCount), activeCount, recorderID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update tb_stores set updated_at = current_timestamp(3) where id = ?`, recorder.StoreID); err != nil {
		return nil, err
	}
	if err := mysqlInsertOperationLog(ctx, tx, "scan_channels", "recorder", recorderID, recorder.StoreID, fmt.Sprintf("scanned recorder %s", recorder.DeviceCode)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetRecorder(ctx, recorderID)
}

func (s *MySQLStore) UpsertRecorderChannel(ctx context.Context, recorderID int64, channel ChannelInput) (*Channel, error) {
	if _, err := mysqlValidateScannedChannel(channel); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := s.queryRecorder(ctx, recorderID); err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `
		insert into tb_video_channels (recorder_id, channel_no, channel_name, status, is_active, scene_type)
		values (?, ?, ?, ?, true, ?)
		on duplicate key update
			channel_name = values(channel_name),
			is_active = true,
			status = case
				when status = ? and (
					area_id is not null
					or area_type is not null
					or area_number is not null
					or confirmed_at is not null
				) then case
					when area_type is not null then ?
					else ?
				end
				when status = ? then ?
				else status
			end,
			scene_type = case
				when status = ? then ?
				else scene_type
			end,
			updated_at = current_timestamp(3)
	`, recorderID, channel.ChannelNo, strings.TrimSpace(channel.ChannelName), ChannelStatusPendingRecognition, SceneTypeUnknown,
		ChannelStatusInactive, ChannelStatusConfirmedBusiness, ChannelStatusConfirmedNonBusiness,
		ChannelStatusInactive, ChannelStatusPendingRecognition,
		ChannelStatusInactive, SceneTypeUnknown,
	)
	if err != nil {
		return nil, err
	}
	channelID, err := result.LastInsertId()
	if err != nil || channelID == 0 {
		if err := tx.QueryRowContext(ctx, `
			select id
			from tb_video_channels
			where recorder_id = ? and channel_no = ?
		`, recorderID, channel.ChannelNo).Scan(&channelID); err != nil {
			return nil, err
		}
	}
	activeCount, err := mysqlActiveChannelCount(ctx, tx, recorderID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		update tb_video_recorders
		set status = ?,
			effective_channel_count = ?,
			last_scanned_at = current_timestamp(3),
			updated_at = current_timestamp(3)
		where id = ?
	`, mysqlRecorderStatusForActiveCount(activeCount), activeCount, recorderID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetChannel(ctx, channelID)
}

func (s *MySQLStore) SaveChannelSnapshot(ctx context.Context, channelID int64, input ChannelSnapshotInput) (*Channel, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var recorderID int64
	if err := tx.QueryRowContext(ctx, `select recorder_id from tb_video_channels where id = ?`, channelID).Scan(&recorderID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.ThumbnailPath) != "" || strings.TrimSpace(input.FullImagePath) != "" {
		if _, err := tx.ExecContext(ctx, `
			insert into tb_channel_snapshots (channel_id, thumbnail_path, full_image_path, full_image_expires_at, created_at)
			values (?, ?, ?, ?, current_timestamp(3))
		`, channelID, input.ThumbnailPath, input.FullImagePath, input.FullImageExpiresAt); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		update tb_video_channels
		set recognition_attempts = recognition_attempts + case when ? then 1 else 0 end,
			recognition_result = case when ? or char_length(?) > 0 then case when char_length(?) = 0 then null else ? end else recognition_result end,
			status = case when char_length(?) = 0 then status else ? end,
			scene_type = case when char_length(?) = 0 then scene_type else ? end,
			area_type = case when char_length(?) = 0 then area_type else case when char_length(?) = 0 then null else ? end end,
			area_number = case when char_length(?) = 0 then area_number else nullif(?, 0) end,
			area_note = case when char_length(?) = 0 then area_note else ? end,
			bed_label = case when char_length(?) = 0 then bed_label else '' end,
			updated_at = current_timestamp(3)
		where id = ?
	`, mysqlChannelSnapshotUpdateArgs(input, channelID)...); err != nil {
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

func mysqlChannelSnapshotUpdateArgs(input ChannelSnapshotInput, channelID int64) []any {
	countAttempt := input.CountAttempt
	recognitionResult := strings.TrimSpace(input.RecognitionResult)
	status := string(input.Status)
	sceneType := string(input.SceneType)
	areaType := string(input.AreaType)
	areaNumber := mustPositiveInt(input.AreaNumberText)
	areaNote := strings.TrimSpace(input.AreaNote)
	return []any{
		countAttempt,
		countAttempt,
		recognitionResult,
		recognitionResult,
		recognitionResult,
		status,
		status,
		status,
		sceneType,
		status,
		areaType,
		areaType,
		status,
		areaNumber,
		status,
		areaNote,
		status,
		channelID,
	}
}

func (s *MySQLStore) UnlockChannelForEdit(ctx context.Context, channelID int64) (*Channel, error) {
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
		from tb_video_channels c, tb_video_recorders r
		where r.id = c.recorder_id and c.id = ?
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
		update tb_video_channels
		set status = ?,
			confirmed_at = null,
			updated_at = current_timestamp(3)
		where id = ?
	`, ChannelStatusPendingConfirmation, channelID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update tb_stores set updated_at = current_timestamp(3) where id = ?`, storeID); err != nil {
		return nil, err
	}
	if err := mysqlInsertOperationLog(ctx, tx, "unlock_channel", "channel", channelID, storeID, "unlocked video channel for editing"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetChannel(ctx, channelID)
}

func (s *MySQLStore) ConfirmChannel(ctx context.Context, channelID int64, input ChannelConfirmationInput) (*Store, error) {
	if input.AreaType == "" {
		return s.confirmNonBusinessChannel(ctx, channelID, input)
	}
	number, err := mysqlConfirmationAreaNumber(input)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	storeID, err := mysqlChannelStoreID(ctx, tx, channelID)
	if err != nil {
		return nil, err
	}
	area, err := s.mysqlUpdateOrCreateVideoArea(ctx, tx, storeID, input.AreaType, number)
	if err != nil {
		return nil, err
	}
	if area.Source != AreaSourceVideoChannel && area.Source != AreaSourceMultiple {
		if _, err := tx.ExecContext(ctx, `
			update tb_store_areas
			set source = ?, updated_at = current_timestamp(3)
			where id = ?
		`, AreaSourceMultiple, area.ID); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		update tb_video_channels
		set status = ?,
			scene_type = ?,
			area_type = ?,
			area_number = ?,
			bed_label = ?,
			area_note = '',
			area_id = ?,
			confirmed_at = current_timestamp(3),
			updated_at = current_timestamp(3)
		where id = ?
	`, ChannelStatusConfirmedBusiness, SceneType(input.AreaType), input.AreaType, number, strings.TrimSpace(input.BedLabel), area.ID, channelID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update tb_stores set updated_at = current_timestamp(3) where id = ?`, storeID); err != nil {
		return nil, err
	}
	if err := mysqlInsertOperationLog(ctx, tx, "confirm_channel", "channel", channelID, storeID, "confirmed video channel mapping"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, storeID)
}

func (s *MySQLStore) DeleteStore(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var name string
	if err := tx.QueryRowContext(ctx, `select name from tb_stores where id = ?`, id).Scan(&name); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from tb_stores where id = ?`, id); err != nil {
		return err
	}
	if err := mysqlInsertOperationLog(ctx, tx, "delete", "store", id, id, fmt.Sprintf("deleted store %s", name)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) DeleteRecorder(ctx context.Context, recorderID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var storeID int64
	var deviceCode string
	if err := tx.QueryRowContext(ctx, `
		select store_id, device_code
		from tb_video_recorders
		where id = ?
	`, recorderID).Scan(&storeID, &deviceCode); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `delete from tb_video_recorders where id = ?`, recorderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `update tb_stores set updated_at = current_timestamp(3) where id = ?`, storeID); err != nil {
		return err
	}
	if err := mysqlInsertOperationLog(ctx, tx, "delete", "recorder", recorderID, storeID, fmt.Sprintf("deleted recorder %s", deviceCode)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) DeleteChannel(ctx context.Context, channelID int64) (*Store, error) {
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
		from tb_video_channels c, tb_video_recorders r
		where r.id = c.recorder_id and c.id = ?
	`, channelID).Scan(&storeID, &recorderID, &channelNo); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `delete from tb_video_channels where id = ?`, channelID); err != nil {
		return nil, err
	}
	if err := mysqlDeleteUnusedVideoAreas(ctx, tx, storeID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		update tb_video_recorders
		set effective_channel_count = (
				select count(*)
				from tb_video_channels
				where recorder_id = ? and is_active and status <> ?
			),
			status = case
				when exists (
					select 1
					from tb_video_channels
					where recorder_id = ? and is_active and status <> ?
				) then ?
				else ?
			end,
			updated_at = current_timestamp(3)
		where id = ?
	`, recorderID, ChannelStatusInactive, recorderID, ChannelStatusInactive, RecorderStatusOnline, RecorderStatusOffline, recorderID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update tb_stores set updated_at = current_timestamp(3) where id = ?`, storeID); err != nil {
		return nil, err
	}
	if err := mysqlInsertOperationLog(ctx, tx, "delete", "channel", channelID, storeID, fmt.Sprintf("deleted channel %d", channelNo)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, storeID)
}

func (s *MySQLStore) CheckDuplicate(ctx context.Context, name string, excludeStoreID int64) (DuplicateCheckResult, error) {
	normalized := NormalizeStoreName(name)
	rows, err := s.db.QueryContext(ctx, `
		select id, name, short_name, normalized_name, overall_status, updated_at
		from tb_stores
		where id <> ?
			and (
				normalized_name = ?
				or normalized_name like ?
				or ? like concat('%', normalized_name, '%')
			)
		order by updated_at desc
		limit 20
	`, excludeStoreID, normalized, "%"+normalized+"%", normalized)
	if err != nil {
		return DuplicateCheckResult{}, err
	}
	defer rows.Close()

	result := DuplicateCheckResult{SimilarMatches: []DuplicateMatch{}}
	for rows.Next() {
		var match DuplicateMatch
		if err := rows.Scan(&match.ID, &match.Name, &match.ShortName, &match.NormalizedName, &match.OverallStatus, &match.UpdatedAt); err != nil {
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
	return result, rows.Err()
}

func (s *MySQLStore) DeviceCodeExists(ctx context.Context, deviceCode string, excludeRecorderID int64) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		select id
		from tb_video_recorders
		where device_code = ? and id <> ?
		limit 1
	`, normalizeDeviceCode(deviceCode), excludeRecorderID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *MySQLStore) FindOrCreateArea(ctx context.Context, input AreaLookup, areaNumber int) (*Area, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	area, err := s.mysqlUpdateOrCreateVideoArea(ctx, tx, input.StoreID, input.Type, areaNumber)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return area, nil
}

func mysqlValidateScannedChannel(channel ChannelInput) (ChannelInput, error) {
	if channel.ChannelNo <= 0 || !channel.IsActive {
		return ChannelInput{}, ErrNotFound
	}
	channel.ChannelName = strings.TrimSpace(channel.ChannelName)
	return channel, nil
}

func mysqlRecorderStatusForActiveCount(activeCount int) RecorderStatus {
	if activeCount > 0 {
		return RecorderStatusOnline
	}
	return RecorderStatusOffline
}

type mysqlQueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func mysqlActiveChannelCount(ctx context.Context, q mysqlQueryRower, recorderID int64) (int, error) {
	var activeCount int
	err := q.QueryRowContext(ctx, `
		select count(*)
		from tb_video_channels
		where recorder_id = ? and is_active and status <> ?
	`, recorderID, ChannelStatusInactive).Scan(&activeCount)
	return activeCount, err
}

func mysqlDeactivateMissingChannels(ctx context.Context, tx *sql.Tx, recorderID int64, scannedNumbers []int) error {
	if len(scannedNumbers) == 0 {
		_, err := tx.ExecContext(ctx, `
			update tb_video_channels
			set is_active = false,
				status = ?,
				updated_at = current_timestamp(3)
			where recorder_id = ?
		`, ChannelStatusInactive, recorderID)
		return err
	}
	args := []any{ChannelStatusInactive, recorderID}
	placeholders := ""
	for index, channelNo := range scannedNumbers {
		if index > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, channelNo)
	}
	_, err := tx.ExecContext(ctx, `
		update tb_video_channels
		set is_active = false,
			status = ?,
			updated_at = current_timestamp(3)
		where recorder_id = ? and channel_no not in (`+placeholders+`)
	`, args...)
	return err
}

func mysqlInsertOperationLog(ctx context.Context, tx *sql.Tx, action string, targetType string, targetID int64, storeID int64, summary string) error {
	_, err := tx.ExecContext(ctx, `
		insert into tb_operation_logs (store_id, action, entity_type, entity_id, summary)
		values (?, ?, ?, ?, ?)
	`, storeID, action, targetType, targetID, summary)
	return err
}

func mysqlChannelStoreID(ctx context.Context, q mysqlQueryRower, channelID int64) (int64, error) {
	var storeID int64
	err := q.QueryRowContext(ctx, `
		select r.store_id
		from tb_video_channels c, tb_video_recorders r
		where r.id = c.recorder_id and c.id = ?
	`, channelID).Scan(&storeID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return storeID, err
}

func mysqlConfirmationAreaNumber(input ChannelConfirmationInput) (int, error) {
	if strings.TrimSpace(input.AreaNumber) == "" {
		if input.AreaType == AreaTypeVIPTreatment {
			return 0, nil
		}
		return 0, &ValidationError{Fields: map[string]string{"area_number": "区域编号必须是正整数"}}
	}
	number, err := strconv.Atoi(strings.TrimSpace(input.AreaNumber))
	if err != nil || number <= 0 {
		return 0, &ValidationError{Fields: map[string]string{"area_number": "区域编号必须是正整数"}}
	}
	return number, nil
}

func (s *MySQLStore) confirmNonBusinessChannel(ctx context.Context, channelID int64, input ChannelConfirmationInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	storeID, err := mysqlChannelStoreID(ctx, tx, channelID)
	if err != nil {
		return nil, err
	}
	sceneType := input.SceneType
	if sceneType == "" {
		sceneType = SceneTypeUnknown
	}
	if _, err := tx.ExecContext(ctx, `
		update tb_video_channels
		set status = ?,
			scene_type = ?,
			area_type = null,
			area_number = null,
			bed_label = '',
			area_note = ?,
			area_id = null,
			confirmed_at = current_timestamp(3),
			updated_at = current_timestamp(3)
		where id = ?
	`, ChannelStatusConfirmedNonBusiness, sceneType, strings.TrimSpace(input.AreaNote), channelID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update tb_stores set updated_at = current_timestamp(3) where id = ?`, storeID); err != nil {
		return nil, err
	}
	if err := mysqlInsertOperationLog(ctx, tx, "confirm_channel", "channel", channelID, storeID, "confirmed video channel mapping"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetStore(ctx, storeID)
}

func (s *MySQLStore) mysqlUpdateOrCreateVideoArea(ctx context.Context, tx *sql.Tx, storeID int64, areaType AreaType, number int) (*Area, error) {
	displayName := areaDisplayName(areaType, number)
	var area Area
	err := tx.QueryRowContext(ctx, `
		select id, store_id, area_type, area_number, display_name, source, status, created_at, updated_at
		from tb_store_areas
		where store_id = ? and area_type = ? and area_number = ?
		limit 1
	`, storeID, areaType, number).Scan(&area.ID, &area.StoreID, &area.Type, &area.Number, &area.DisplayName, &area.Source, &area.Status, &area.CreatedAt, &area.UpdatedAt)
	if err == nil {
		return &area, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `
		insert into tb_store_areas (store_id, area_type, area_number, display_name, source, status)
		values (?, ?, ?, ?, ?, ?)
	`, storeID, areaType, number, displayName, AreaSourceVideoChannel, AreaStatusConfirmed)
	if err != nil {
		return nil, err
	}
	areaID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	area.ID = areaID
	area.StoreID = storeID
	area.Type = areaType
	area.Number = number
	area.DisplayName = displayName
	area.Source = AreaSourceVideoChannel
	area.Status = AreaStatusConfirmed
	return &area, nil
}

func mysqlUpsertStoreDesignPlan(ctx context.Context, tx *sql.Tx, storeID int64, input SaveDesignPlanInput) (*DesignPlan, error) {
	var existingID int64
	err := tx.QueryRowContext(ctx, `
		select id
		from tb_store_design_plans
		where store_id = ?
		order by updated_at desc, id desc
		limit 1
	`, storeID).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if existingID != 0 {
		if _, err := tx.ExecContext(ctx, `
			update tb_store_design_plans
			set upload_id = ?,
				pdf_file_name = ?,
				original_pdf_path = ?,
				preview_image_path = ?,
				thumbnail_path = ?,
				page_count = ?,
				recognition_status = ?,
				recognition_result = case when length(?) = 0 then null else ? end,
				updated_at = current_timestamp(3)
			where id = ?
		`, input.UploadID, input.PDFFileName, input.OriginalPDFPath, input.PreviewImagePath, input.ThumbnailPath,
			input.PageCount, RecognitionStatusCompleted, input.RecognitionResult, input.RecognitionResult, existingID); err != nil {
			return nil, err
		}
		return mysqlQueryDesignPlan(ctx, tx, existingID)
	}

	result, err := tx.ExecContext(ctx, `
		insert into tb_store_design_plans (
			store_id, upload_id, pdf_file_name, original_pdf_path, preview_image_path,
			thumbnail_path, page_count, recognition_status, recognition_result
		)
		values (?, ?, ?, ?, ?, ?, ?, ?, case when length(?) = 0 then null else ? end)
	`, storeID, input.UploadID, input.PDFFileName, input.OriginalPDFPath, input.PreviewImagePath,
		input.ThumbnailPath, input.PageCount, RecognitionStatusCompleted, input.RecognitionResult, input.RecognitionResult)
	if err != nil {
		return nil, err
	}
	planID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return mysqlQueryDesignPlan(ctx, tx, planID)
}

func mysqlQueryDesignPlan(ctx context.Context, tx *sql.Tx, planID int64) (*DesignPlan, error) {
	var plan DesignPlan
	err := tx.QueryRowContext(ctx, `
		select id, store_id, upload_id, pdf_file_name, original_pdf_path, preview_image_path,
			thumbnail_path, page_count, recognition_status, created_at, updated_at
		from tb_store_design_plans
		where id = ?
	`, planID).Scan(
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
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func mysqlUpsertDesignArea(ctx context.Context, tx *sql.Tx, storeID int64, input DesignAreaInput) (*Area, error) {
	number, _ := strconv.Atoi(strings.TrimSpace(input.NumberText))
	if input.ID != 0 {
		if area, err := mysqlUpdateDesignAreaByID(ctx, tx, storeID, input, number); err == nil {
			return area, nil
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	area, err := mysqlQueryArea(ctx, tx, storeID, input.Type, number)
	if err == nil {
		return mysqlUpdateDesignAreaSource(ctx, tx, area, input, number)
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	result, err := tx.ExecContext(ctx, `
		insert into tb_store_areas (store_id, area_type, area_number, display_name, source, status)
		values (?, ?, ?, ?, ?, ?)
	`, storeID, input.Type, number, displayNameOrDefault(input.DisplayName, input.Type, number), AreaSourceDesignPlan, AreaStatusConfirmed)
	if err != nil {
		return nil, err
	}
	areaID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return mysqlQueryAreaByID(ctx, tx, areaID, storeID)
}

func mysqlUpdateDesignAreaByID(ctx context.Context, tx *sql.Tx, storeID int64, input DesignAreaInput, number int) (*Area, error) {
	area, err := mysqlQueryAreaByID(ctx, tx, input.ID, storeID)
	if err != nil {
		return nil, err
	}
	if area.Source == AreaSourceVideoChannel || area.Source == AreaSourceMultiple {
		nextSource := mergeAreaSource(area.Source, AreaSourceDesignPlan)
		if area.Source != nextSource {
			if _, err := tx.ExecContext(ctx, `
				update tb_store_areas
				set source = ?, updated_at = current_timestamp(3)
				where id = ?
			`, nextSource, area.ID); err != nil {
				return nil, err
			}
			area.Source = nextSource
		}
		return area, nil
	}
	return mysqlUpdateDesignAreaSource(ctx, tx, area, input, number)
}

func mysqlUpdateDesignAreaSource(ctx context.Context, tx *sql.Tx, area *Area, input DesignAreaInput, number int) (*Area, error) {
	nextSource := mergeAreaSource(area.Source, AreaSourceDesignPlan)
	if _, err := tx.ExecContext(ctx, `
		update tb_store_areas
		set area_type = ?,
			area_number = ?,
			display_name = ?,
			source = ?,
			status = ?,
			updated_at = current_timestamp(3)
		where id = ?
	`, input.Type, number, displayNameOrDefault(input.DisplayName, input.Type, number), nextSource, AreaStatusConfirmed, area.ID); err != nil {
		return nil, err
	}
	return mysqlQueryAreaByID(ctx, tx, area.ID, area.StoreID)
}

func mysqlQueryArea(ctx context.Context, tx *sql.Tx, storeID int64, areaType AreaType, number int) (*Area, error) {
	var area Area
	err := tx.QueryRowContext(ctx, `
		select id, store_id, area_type, area_number, display_name, source, status, created_at, updated_at
		from tb_store_areas
		where store_id = ? and area_type = ? and area_number = ?
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

func mysqlQueryAreaByID(ctx context.Context, tx *sql.Tx, areaID int64, storeID int64) (*Area, error) {
	var area Area
	err := tx.QueryRowContext(ctx, `
		select id, store_id, area_type, area_number, display_name, source, status, created_at, updated_at
		from tb_store_areas
		where id = ? and store_id = ?
	`, areaID, storeID).Scan(
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

func mysqlUpsertDesignAnnotation(ctx context.Context, tx *sql.Tx, planID int64, areaID int64, box *AreaBox) error {
	if box == nil {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		insert into tb_design_plan_annotations (
			design_plan_id, area_id, box_x, box_y, box_width, box_height, status
		)
		values (?, ?, ?, ?, ?, ?, ?)
		on duplicate key update
			box_x = values(box_x),
			box_y = values(box_y),
			box_width = values(box_width),
			box_height = values(box_height),
			status = values(status),
			updated_at = current_timestamp(3)
	`, planID, areaID, box.X, box.Y, box.Width, box.Height, "confirmed")
	return err
}

func mysqlDeleteUnusedVideoAreas(ctx context.Context, tx *sql.Tx, storeID int64) error {
	_, err := tx.ExecContext(ctx, `
		delete from tb_store_areas
		where store_id = ?
			and source = ?
			and not exists (
				select 1 from tb_design_plan_annotations dpa
				where dpa.area_id = tb_store_areas.id
			)
			and not exists (
				select 1 from tb_video_channels vc
				where vc.area_id = tb_store_areas.id
			)
	`, storeID, AreaSourceVideoChannel)
	return err
}

func (s *MySQLStore) getStoreBase(ctx context.Context, id int64) (*Store, error) {
	var store Store
	err := s.db.QueryRowContext(ctx, `
		select id, city, name, short_name, normalized_name, external_org_id, design_plan_status,
			overall_status, created_at, updated_at
		from tb_stores
		where id = ?
	`, id).Scan(&store.ID, &store.City, &store.Name, &store.ShortName, &store.NormalizedName, &store.ExternalOrgID, &store.DesignPlanStatus, &store.OverallStatus, &store.CreatedAt, &store.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &store, nil
}

func (s *MySQLStore) listAreas(ctx context.Context, storeID int64) ([]Area, error) {
	rows, err := s.db.QueryContext(ctx, `
		select
			a.id,
			a.store_id,
			a.area_type,
			a.area_number,
			a.display_name,
			a.source,
			a.status,
			(select dpa.box_x
				from tb_design_plan_annotations dpa, tb_store_design_plans sdp
				where sdp.id = dpa.design_plan_id
					and sdp.store_id = a.store_id
					and dpa.area_id = a.id
				order by dpa.updated_at desc, dpa.id desc
				limit 1),
			(select dpa.box_y
				from tb_design_plan_annotations dpa, tb_store_design_plans sdp
				where sdp.id = dpa.design_plan_id
					and sdp.store_id = a.store_id
					and dpa.area_id = a.id
				order by dpa.updated_at desc, dpa.id desc
				limit 1),
			(select dpa.box_width
				from tb_design_plan_annotations dpa, tb_store_design_plans sdp
				where sdp.id = dpa.design_plan_id
					and sdp.store_id = a.store_id
					and dpa.area_id = a.id
				order by dpa.updated_at desc, dpa.id desc
				limit 1),
			(select dpa.box_height
				from tb_design_plan_annotations dpa, tb_store_design_plans sdp
				where sdp.id = dpa.design_plan_id
					and sdp.store_id = a.store_id
					and dpa.area_id = a.id
				order by dpa.updated_at desc, dpa.id desc
				limit 1),
			a.created_at,
			a.updated_at
		from tb_store_areas a
		where a.store_id = ?
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
		if err := rows.Scan(&area.ID, &area.StoreID, &area.Type, &area.Number, &area.DisplayName, &area.Source, &area.Status, &boxX, &boxY, &boxWidth, &boxHeight, &area.CreatedAt, &area.UpdatedAt); err != nil {
			return nil, err
		}
		if box, ok := parseAreaBox(boxX, boxY, boxWidth, boxHeight); ok {
			area.Box = box
		}
		areas = append(areas, area)
	}
	return areas, rows.Err()
}

func (s *MySQLStore) listDesignPlans(ctx context.Context, storeID int64) ([]DesignPlan, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, store_id, upload_id, pdf_file_name, original_pdf_path, preview_image_path,
			thumbnail_path, page_count, recognition_status, created_at, updated_at
		from tb_store_design_plans
		where store_id = ?
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

func (s *MySQLStore) listRecorders(ctx context.Context, storeID int64) ([]Recorder, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, store_id, coalesce(ezviz_account_id, 0), device_code, status,
			effective_channel_count, last_scanned_at, created_at, updated_at
		from tb_video_recorders
		where store_id = ?
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
		if ok {
			recorders[index].Channels = append(recorders[index].Channels, channel)
		}
	}
	return recorders, nil
}

func (s *MySQLStore) queryRecorder(ctx context.Context, recorderID int64) (*Recorder, error) {
	var recorder Recorder
	err := s.db.QueryRowContext(ctx, `
		select id, store_id, coalesce(ezviz_account_id, 0), device_code, status,
			effective_channel_count, last_scanned_at, created_at, updated_at
		from tb_video_recorders
		where id = ?
	`, recorderID).Scan(&recorder.ID, &recorder.StoreID, &recorder.EzvizAccountID, &recorder.DeviceCode, &recorder.Status, &recorder.EffectiveChannelCount, &recorder.LastScannedAt, &recorder.CreatedAt, &recorder.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &recorder, nil
}

func (s *MySQLStore) GetChannel(ctx context.Context, channelID int64) (*Channel, error) {
	row := s.db.QueryRowContext(ctx, mysqlChannelSelect(`
		where c.id = ?
	`), channelID)
	channel, err := scanChannel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return channel, err
}

func (s *MySQLStore) listChannels(ctx context.Context, recorderID int64) ([]Channel, error) {
	rows, err := s.db.QueryContext(ctx, mysqlChannelSelect(`
		where c.recorder_id = ?
		order by c.channel_no
	`), recorderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChannels(rows)
}

func (s *MySQLStore) listChannelsForStore(ctx context.Context, storeID int64) ([]Channel, error) {
	rows, err := s.db.QueryContext(ctx, mysqlChannelSelect(`
		where c.recorder_id in (
			select r.id
			from tb_video_recorders r
			where r.store_id = ?
		)
		order by c.recorder_id, c.channel_no
	`), storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChannels(rows)
}

func scanChannels(rows *sql.Rows) ([]Channel, error) {
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

func mysqlChannelSelect(extra string) string {
	return `
		select c.id, c.recorder_id, c.channel_no, c.channel_name, c.status, c.is_active,
			c.scene_type, coalesce(c.area_type, ''), coalesce(c.area_number, 0),
			coalesce(c.bed_label, ''), coalesce(c.area_note, ''), coalesce(c.area_id, 0), c.recognition_attempts,
			coalesce(cast(c.recognition_result as char), ''),
			(select snapshot.thumbnail_path
				from tb_channel_snapshots snapshot
				where snapshot.channel_id = c.id
				order by snapshot.created_at desc, snapshot.id desc
				limit 1),
			(select snapshot.full_image_path
				from tb_channel_snapshots snapshot
				where snapshot.channel_id = c.id
				order by snapshot.created_at desc, snapshot.id desc
				limit 1),
			(select snapshot.full_image_expires_at
				from tb_channel_snapshots snapshot
				where snapshot.channel_id = c.id
				order by snapshot.created_at desc, snapshot.id desc
				limit 1),
			c.confirmed_at, c.created_at, c.updated_at
		from tb_video_channels c
	` + extra
}

func (s *MySQLStore) listStoreCities(ctx context.Context, rawLike string, hasSearch bool) ([]string, error) {
	sqlText := `
		select distinct coalesce(nullif(trim(city), ''), '未设置') as city_option
		from tb_stores s
	`
	args := []any{}
	if hasSearch {
		sqlText += `
		where replace(lower(coalesce(s.name, '')), ' ', '') like ?
			or coalesce(s.external_org_id, '') like ?
		`
		args = append(args, rawLike, rawLike)
	}
	sqlText += `
		order by city_option
	`
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cities := []string{}
	for rows.Next() {
		var city string
		if err := rows.Scan(&city); err != nil {
			return nil, err
		}
		cities = append(cities, city)
	}
	return cities, rows.Err()
}

func (s *MySQLStore) storeListSummary(ctx context.Context, rawLike string, normalizedLike string, city string) (StoreListSummary, error) {
	var summary StoreListSummary
	err := s.db.QueryRowContext(ctx, `
		select
			(select count(*)
				from tb_stores s
				where (? = '' or coalesce(nullif(trim(s.city), ''), '未设置') = ?)
					and (? = '%%' or replace(lower(s.name), ' ', '') like ? or (? <> '%%' and s.normalized_name like ?))
			),
			(select count(*)
				from tb_store_areas a
				where a.area_type in ('treatment', 'vip_treatment')
					and a.store_id in (
						select s.id from tb_stores s
						where (? = '' or coalesce(nullif(trim(s.city), ''), '未设置') = ?)
							and (? = '%%' or replace(lower(s.name), ' ', '') like ? or (? <> '%%' and s.normalized_name like ?))
					)
			),
			(select count(*)
				from tb_store_areas a
				where a.area_type = 'consultation'
					and a.store_id in (
						select s.id from tb_stores s
						where (? = '' or coalesce(nullif(trim(s.city), ''), '未设置') = ?)
							and (? = '%%' or replace(lower(s.name), ' ', '') like ? or (? <> '%%' and s.normalized_name like ?))
					)
			),
			(select count(*)
				from tb_store_areas a
				where a.area_type = 'beauty'
					and a.store_id in (
						select s.id from tb_stores s
						where (? = '' or coalesce(nullif(trim(s.city), ''), '未设置') = ?)
							and (? = '%%' or replace(lower(s.name), ' ', '') like ? or (? <> '%%' and s.normalized_name like ?))
					)
			)
	`,
		city, city, rawLike, rawLike, normalizedLike, normalizedLike,
		city, city, rawLike, rawLike, normalizedLike, normalizedLike,
		city, city, rawLike, rawLike, normalizedLike, normalizedLike,
		city, city, rawLike, rawLike, normalizedLike, normalizedLike,
	).Scan(&summary.StoreCount, &summary.TreatmentCount, &summary.ConsultationCount, &summary.BeautyCount)
	return summary, err
}

type MySQLH5MonitorRepository struct {
	store    *MySQLStore
	accounts map[string]ezviz.Account
}

func NewMySQLH5MonitorRepository(store *MySQLStore, accounts []ezviz.Account) *MySQLH5MonitorRepository {
	accountMap := map[string]ezviz.Account{}
	for _, account := range accounts {
		name := strings.TrimSpace(account.Name)
		if name != "" {
			accountMap[name] = account
		}
	}
	return &MySQLH5MonitorRepository{store: store, accounts: accountMap}
}

func (r *MySQLH5MonitorRepository) GetStoreByExternalOrgID(ctx context.Context, externalOrgID string) (*h5monitor.StoreInfo, error) {
	var store h5monitor.StoreInfo
	err := r.store.db.QueryRowContext(ctx, `
		select id, name, city, external_org_id
		from tb_stores
		where external_org_id = ?
	`, strings.TrimSpace(externalOrgID)).Scan(&store.ID, &store.Name, &store.City, &store.ExternalOrgID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, h5monitor.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &store, nil
}

func (r *MySQLH5MonitorRepository) ListActiveChannelsByOrgID(ctx context.Context, externalOrgID string) ([]h5monitor.ChannelInfo, error) {
	rows, err := r.store.db.QueryContext(ctx, mysqlH5MonitorChannelQuery(`
		and s.external_org_id = ?
		order by c.channel_no
	`), strings.TrimSpace(externalOrgID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanH5MonitorChannels(rows, externalOrgID)
}

func (r *MySQLH5MonitorRepository) GetChannelByID(ctx context.Context, channelID int64) (*h5monitor.ChannelInfo, error) {
	row := r.store.db.QueryRowContext(ctx, mysqlH5MonitorChannelQuery(`
		and c.id = ?
	`), channelID)
	channel, err := scanH5MonitorChannel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, h5monitor.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.applyCredentials(&channel)
	return &channel, nil
}

func (r *MySQLH5MonitorRepository) ListMonitorStores(ctx context.Context) ([]h5monitor.MonitorStoreInfo, error) {
	rows, err := r.store.db.QueryContext(ctx, `
		select
			s.external_org_id,
			s.name,
			s.city,
			ea.account_name,
			count(c.id) as channel_count
		from tb_stores s, tb_video_recorders r, tb_video_channels c, tb_ezviz_accounts ea
		where trim(s.external_org_id) <> ''
			and r.store_id = s.id
			and c.recorder_id = r.id
			and ea.id = r.ezviz_account_id
			and c.is_active = 1
			and c.channel_no > 0
			and trim(r.device_code) <> ''
			and r.ezviz_account_id is not null
		group by s.external_org_id, s.name, s.city, ea.account_name
		order by s.city, s.name, s.external_org_id, ea.account_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	storesByOrgID := map[string]*h5monitor.MonitorStoreInfo{}
	order := []string{}
	for rows.Next() {
		var externalOrgID, storeName, city, accountName string
		var channelCount int
		if err := rows.Scan(&externalOrgID, &storeName, &city, &accountName, &channelCount); err != nil {
			return nil, err
		}
		account, ok := r.accounts[strings.TrimSpace(accountName)]
		if !ok || strings.TrimSpace(account.AppKey) == "" || strings.TrimSpace(account.AppSecret) == "" {
			continue
		}
		externalOrgID = strings.TrimSpace(externalOrgID)
		store := storesByOrgID[externalOrgID]
		if store == nil {
			storesByOrgID[externalOrgID] = &h5monitor.MonitorStoreInfo{ExternalOrgID: externalOrgID, StoreName: storeName, City: city}
			order = append(order, externalOrgID)
			store = storesByOrgID[externalOrgID]
		}
		store.AvailableChannelCount += channelCount
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	stores := make([]h5monitor.MonitorStoreInfo, 0, len(order))
	for _, externalOrgID := range order {
		store := storesByOrgID[externalOrgID]
		if store.AvailableChannelCount > 0 {
			stores = append(stores, *store)
		}
	}
	return stores, nil
}

func (r *MySQLH5MonitorRepository) scanH5MonitorChannels(rows *sql.Rows, externalOrgID string) ([]h5monitor.ChannelInfo, error) {
	channels := []h5monitor.ChannelInfo{}
	for rows.Next() {
		channel, err := scanH5MonitorChannel(rows)
		if err != nil {
			return nil, err
		}
		r.applyCredentials(&channel)
		if strings.TrimSpace(channel.AppKey) == "" || strings.TrimSpace(channel.AppSecret) == "" {
			continue
		}
		channels = append(channels, channel)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		if _, err := r.GetStoreByExternalOrgID(context.Background(), externalOrgID); err != nil {
			return nil, err
		}
	}
	return channels, nil
}

func (r *MySQLH5MonitorRepository) applyCredentials(channel *h5monitor.ChannelInfo) {
	account, ok := r.accounts[strings.TrimSpace(channel.AccountName)]
	if !ok {
		return
	}
	channel.AppKey = account.AppKey
	channel.AppSecret = account.AppSecret
	channel.AccessToken = account.AccessToken
}

func mysqlH5MonitorChannelQuery(extraCondition string) string {
	return `
		select
			s.id,
			r.id,
			c.id,
			c.channel_no,
			c.channel_name,
			c.status,
			c.is_active,
			coalesce(c.area_type, ''),
			c.scene_type,
			coalesce(c.area_number, 0),
			coalesce(c.bed_label, ''),
			coalesce(c.area_note, ''),
			coalesce((select snapshot.thumbnail_path
				from tb_channel_snapshots snapshot
				where snapshot.channel_id = c.id
				order by snapshot.created_at desc, snapshot.id desc
				limit 1), ''),
			r.device_code,
			coalesce(r.ezviz_account_id, 0),
			coalesce((select ea.account_name
				from tb_ezviz_accounts ea
				where ea.id = r.ezviz_account_id
				limit 1), '')
		from tb_video_channels c, tb_video_recorders r, tb_stores s
		where c.is_active = 1
			and r.id = c.recorder_id
			and s.id = r.store_id
			and c.channel_no > 0
			and trim(r.device_code) <> ''
			and r.ezviz_account_id is not null
	` + extraCondition
}

type h5MonitorChannelScanner interface {
	Scan(dest ...any) error
}

func scanH5MonitorChannel(scanner h5MonitorChannelScanner) (h5monitor.ChannelInfo, error) {
	var channel h5monitor.ChannelInfo
	err := scanner.Scan(
		&channel.StoreID,
		&channel.RecorderID,
		&channel.ID,
		&channel.ChannelNo,
		&channel.ChannelName,
		&channel.Status,
		&channel.IsActive,
		&channel.AreaType,
		&channel.SceneType,
		&channel.AreaNumber,
		&channel.BedLabel,
		&channel.AreaNote,
		&channel.ThumbnailURL,
		&channel.DeviceSerial,
		&channel.EzvizAccountID,
		&channel.AccountName,
	)
	return channel, err
}

var _ Repository = (*MySQLStore)(nil)
var _ h5monitor.StoreRepository = (*MySQLH5MonitorRepository)(nil)
