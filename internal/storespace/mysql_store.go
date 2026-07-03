package storespace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	return nil, ErrNotImplemented
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
	return nil, ErrNotImplemented
}

func (s *MySQLStore) UpdateStoreBasicInfo(ctx context.Context, id int64, input UpdateStoreBasicInfoInput) (*Store, error) {
	return nil, ErrNotImplemented
}

func (s *MySQLStore) SaveDesignPlan(ctx context.Context, storeID int64, input SaveDesignPlanInput) (*Store, error) {
	return nil, ErrNotImplemented
}

func (s *MySQLStore) AddRecorder(ctx context.Context, storeID int64, input AddRecorderInput) (*Store, error) {
	return nil, ErrNotImplemented
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
	return nil, ErrNotImplemented
}

func (s *MySQLStore) UpsertRecorderChannel(ctx context.Context, recorderID int64, channel ChannelInput) (*Channel, error) {
	return nil, ErrNotImplemented
}

func (s *MySQLStore) SaveChannelSnapshot(ctx context.Context, channelID int64, input ChannelSnapshotInput) (*Channel, error) {
	return nil, ErrNotImplemented
}

func (s *MySQLStore) UnlockChannelForEdit(ctx context.Context, channelID int64) (*Channel, error) {
	return nil, ErrNotImplemented
}

func (s *MySQLStore) ConfirmChannel(ctx context.Context, channelID int64, input ChannelConfirmationInput) (*Store, error) {
	return nil, ErrNotImplemented
}

func (s *MySQLStore) DeleteStore(ctx context.Context, id int64) error {
	return ErrNotImplemented
}

func (s *MySQLStore) DeleteRecorder(ctx context.Context, recorderID int64) error {
	return ErrNotImplemented
}

func (s *MySQLStore) DeleteChannel(ctx context.Context, channelID int64) (*Store, error) {
	return nil, ErrNotImplemented
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
	return nil, ErrNotImplemented
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

var _ Repository = (*MySQLStore)(nil)
var _ h5monitor.StoreRepository = (*MySQLH5MonitorRepository)(nil)
