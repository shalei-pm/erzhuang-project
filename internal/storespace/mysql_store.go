package storespace

import (
	"context"
	"database/sql"
	"errors"
	"strings"

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
	rawLike := "%" + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(filters.Query), " ", "")) + "%"
	normalizedLike := "%" + NormalizeStoreName(filters.Query) + "%"
	city := strings.TrimSpace(filters.City)
	offset := (filters.Page - 1) * filters.PageSize

	var total int
	if err := s.db.QueryRowContext(ctx, `
		select count(*)
		from tb_stores s
		where (? = '' or coalesce(nullif(trim(s.city), ''), '未设置') = ?)
			and (
				? = '%%'
				or replace(lower(s.name), ' ', '') like ?
				or (? <> '%%' and s.normalized_name like ?)
			)
	`, city, city, rawLike, rawLike, normalizedLike, normalizedLike).Scan(&total); err != nil {
		return StoreListResult{}, err
	}

	cities, err := s.listStoreCities(ctx, rawLike, normalizedLike)
	if err != nil {
		return StoreListResult{}, err
	}
	summary, err := s.storeListSummary(ctx, rawLike, normalizedLike, city)
	if err != nil {
		return StoreListResult{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
		select
			s.id,
			s.city,
			s.name,
			s.short_name,
			s.external_org_id,
			s.design_plan_status,
			s.overall_status,
			s.updated_at,
			count(distinct r.id) as recorder_count,
			count(distinct case when c.is_active = 1 then c.id end) as channel_count,
			count(distinct case when c.is_active = 1 then c.id end) > 0
				and count(distinct case
					when c.is_active = 1 and c.status not in ('confirmed_business', 'confirmed_non_business') then c.id
				end) = 0 as channels_fully_confirmed,
			count(distinct case when a.area_type in ('treatment', 'vip_treatment') then a.id end) as treatment_count,
			count(distinct case when a.area_type = 'consultation' then a.id end) as consultation_count,
			count(distinct case when a.area_type = 'beauty' then a.id end) as beauty_count,
			count(distinct a.id) as area_count
		from tb_stores s
		left join tb_store_areas a on a.store_id = s.id
		left join tb_video_recorders r on r.store_id = s.id
		left join tb_video_channels c on c.recorder_id = r.id
		where (? = '' or coalesce(nullif(trim(s.city), ''), '未设置') = ?)
			and (
				? = '%%'
				or replace(lower(s.name), ' ', '') like ?
				or (? <> '%%' and s.normalized_name like ?)
			)
		group by s.id, s.city, s.name, s.short_name, s.external_org_id, s.design_plan_status, s.overall_status, s.updated_at
		order by s.updated_at desc
		limit ? offset ?
	`, city, city, rawLike, rawLike, normalizedLike, normalizedLike, filters.PageSize, offset)
	if err != nil {
		return StoreListResult{}, err
	}
	defer rows.Close()

	items := []StoreListItem{}
	for rows.Next() {
		var item StoreListItem
		if err := rows.Scan(&item.ID, &item.City, &item.Name, &item.ShortName, &item.ExternalOrgID, &item.DesignPlanStatus, &item.OverallStatus, &item.UpdatedAt, &item.RecorderCount, &item.ChannelCount, &item.ChannelsFullyConfirmed, &item.TreatmentCount, &item.ConsultationCount, &item.BeautyCount, &item.AreaCount); err != nil {
			return StoreListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return StoreListResult{}, err
	}
	return StoreListResult{Items: items, Page: filters.Page, PageSize: filters.PageSize, Total: total, Summary: summary, Cities: cities}, nil
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
			annotation.box_x,
			annotation.box_y,
			annotation.box_width,
			annotation.box_height,
			a.created_at,
			a.updated_at
		from tb_store_areas a
		left join (
			select dpa.area_id, dpa.box_x, dpa.box_y, dpa.box_width, dpa.box_height
			from tb_design_plan_annotations dpa
			join tb_store_design_plans sdp on sdp.id = dpa.design_plan_id
			where sdp.store_id = ?
				and not exists (
					select 1
					from tb_design_plan_annotations newer
					join tb_store_design_plans newer_plan on newer_plan.id = newer.design_plan_id
					where newer.area_id = dpa.area_id
						and newer_plan.store_id = sdp.store_id
						and (newer.updated_at > dpa.updated_at or (newer.updated_at = dpa.updated_at and newer.id > dpa.id))
				)
		) annotation on annotation.area_id = a.id
		where a.store_id = ?
		order by a.area_type, a.area_number
	`, storeID, storeID)
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
		join tb_video_recorders filter_r on filter_r.id = c.recorder_id
		where filter_r.store_id = ?
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
			snapshot.thumbnail_path, snapshot.full_image_path, snapshot.full_image_expires_at,
			c.confirmed_at, c.created_at, c.updated_at
		from tb_video_channels c
		left join tb_channel_snapshots snapshot
			on snapshot.channel_id = c.id
			and not exists (
				select 1
				from tb_channel_snapshots newer
				where newer.channel_id = c.id
					and (newer.created_at > snapshot.created_at or (newer.created_at = snapshot.created_at and newer.id > snapshot.id))
			)
	` + extra
}

func (s *MySQLStore) listStoreCities(ctx context.Context, rawLike string, normalizedLike string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		select distinct coalesce(nullif(trim(city), ''), '未设置') as city_option
		from tb_stores s
		where ? = '%%'
			or replace(lower(s.name), ' ', '') like ?
			or (? <> '%%' and s.normalized_name like ?)
		order by city_option
	`, rawLike, rawLike, normalizedLike, normalizedLike)
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
		from tb_stores s
		join tb_video_recorders r on r.store_id = s.id
		join tb_video_channels c on c.recorder_id = r.id
		join tb_ezviz_accounts ea on ea.id = r.ezviz_account_id
		where trim(s.external_org_id) <> ''
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
			coalesce(snapshot.thumbnail_path, ''),
			r.device_code,
			coalesce(r.ezviz_account_id, 0),
			coalesce(ea.account_name, '')
		from tb_video_channels c
		join tb_video_recorders r on r.id = c.recorder_id
		join tb_stores s on s.id = r.store_id
		left join tb_ezviz_accounts ea on ea.id = r.ezviz_account_id
		left join tb_channel_snapshots snapshot
			on snapshot.channel_id = c.id
			and not exists (
				select 1
				from tb_channel_snapshots newer
				where newer.channel_id = c.id
					and (newer.created_at > snapshot.created_at or (newer.created_at = snapshot.created_at and newer.id > snapshot.id))
			)
		where c.is_active = 1
			and c.channel_no > 0
			and trim(r.device_code) <> ''
			and r.ezviz_account_id is not null
	` + extraCondition
}

var _ Repository = (*MySQLStore)(nil)
var _ h5monitor.StoreRepository = (*MySQLH5MonitorRepository)(nil)
