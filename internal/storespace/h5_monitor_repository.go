package storespace

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
	"github.com/shalei-pm/erzhuang-project/internal/h5monitor"
)

type H5MonitorRepository struct {
	store    *PostgresStore
	accounts map[string]ezviz.Account
}

func NewH5MonitorRepository(store *PostgresStore, accounts []ezviz.Account) *H5MonitorRepository {
	accountMap := map[string]ezviz.Account{}
	for _, account := range accounts {
		name := strings.TrimSpace(account.Name)
		if name == "" {
			continue
		}
		accountMap[name] = account
	}
	return &H5MonitorRepository{store: store, accounts: accountMap}
}

func (r *H5MonitorRepository) GetStoreByExternalOrgID(ctx context.Context, externalOrgID string) (*h5monitor.StoreInfo, error) {
	var store h5monitor.StoreInfo
	err := r.store.db.QueryRowContext(ctx, `
		select id, name, city, external_org_id
		from stores
		where external_org_id = $1
	`, strings.TrimSpace(externalOrgID)).Scan(&store.ID, &store.Name, &store.City, &store.ExternalOrgID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &store, nil
}

func (r *H5MonitorRepository) ListActiveChannelsByOrgID(ctx context.Context, externalOrgID string) ([]h5monitor.ChannelInfo, error) {
	rows, err := r.store.db.QueryContext(ctx, h5MonitorChannelQuery(`
		and s.external_org_id = $1
		order by c.channel_no
	`), strings.TrimSpace(externalOrgID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
		if _, err := r.GetStoreByExternalOrgID(ctx, externalOrgID); err != nil {
			return nil, err
		}
	}
	return channels, nil
}

func (r *H5MonitorRepository) GetChannelByID(ctx context.Context, channelID int64) (*h5monitor.ChannelInfo, error) {
	row := r.store.db.QueryRowContext(ctx, h5MonitorChannelQuery(`
		and c.id = $1
	`), channelID)
	channel, err := scanH5MonitorChannel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.applyCredentials(&channel)
	return &channel, nil
}

func (r *H5MonitorRepository) applyCredentials(channel *h5monitor.ChannelInfo) {
	account, ok := r.accounts[strings.TrimSpace(channel.AccountName)]
	if !ok {
		return
	}
	channel.AppKey = account.AppKey
	channel.AppSecret = account.AppSecret
	channel.AccessToken = account.AccessToken
}

type h5MonitorChannelScanner interface {
	Scan(dest ...any) error
}

func h5MonitorChannelQuery(extraCondition string) string {
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
			coalesce(c.area_note, ''),
			coalesce((
				select cs.thumbnail_path
				from channel_snapshots cs
				where cs.channel_id = c.id
				order by cs.created_at desc, cs.id desc
				limit 1
			), ''),
			r.device_code,
			coalesce(r.ezviz_account_id, 0),
			coalesce((
				select ea.account_name
				from ezviz_accounts ea
				where ea.id = r.ezviz_account_id
				limit 1
			), '')
		from video_channels c, video_recorders r, stores s
		where r.id = c.recorder_id
			and s.id = r.store_id
			and c.is_active
			and c.status in ('confirmed_business', 'confirmed_non_business')
			and r.device_code <> ''
			and r.ezviz_account_id is not null
	` + extraCondition
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
		&channel.AreaNote,
		&channel.ThumbnailURL,
		&channel.DeviceSerial,
		&channel.EzvizAccountID,
		&channel.AccountName,
	)
	return channel, err
}
