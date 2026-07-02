package h5monitor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
)

type StoreRepository interface {
	GetStoreByExternalOrgID(ctx context.Context, externalOrgID string) (*StoreInfo, error)
	ListActiveChannelsByOrgID(ctx context.Context, externalOrgID string) ([]ChannelInfo, error)
	GetChannelByID(ctx context.Context, channelID int64) (*ChannelInfo, error)
	ListMonitorStores(ctx context.Context) ([]MonitorStoreInfo, error)
}

type StoreInfo struct {
	ID            int64
	Name          string
	City          string
	ExternalOrgID string
}

type ChannelInfo struct {
	ID             int64
	StoreID        int64
	RecorderID     int64
	ChannelNo      int
	ChannelName    string
	Status         string
	IsActive       bool
	AreaType       string
	SceneType      string
	AreaNumber     int
	BedLabel       string
	AreaNote       string
	ThumbnailURL   string
	DeviceSerial   string
	EzvizAccountID int64
	AccountName    string
	AppKey         string
	AppSecret      string
	AccessToken    string
}

type EzvizPlayer interface {
	EnsureAACTransfer(ctx context.Context, account ezviz.Account, deviceSerial string, channelNo int) error
	LiveAddress(ctx context.Context, account ezviz.Account, input ezviz.LiveAddressRequest) (ezviz.LiveAddressResult, error)
	PlaybackAddress(ctx context.Context, account ezviz.Account, input ezviz.PlaybackRequest) (ezviz.PlaybackResult, error)
	QueryRecordSegments(ctx context.Context, account ezviz.Account, input ezviz.RecordSegmentsQuery) (ezviz.RecordSegmentsResult, error)
	DisableLiveAddress(ctx context.Context, account ezviz.Account, input ezviz.DisableLiveAddressRequest) error
}

type Service struct {
	repo        StoreRepository
	player      EzvizPlayer
	mu          sync.Mutex
	concurrency map[string]*concurrencyState
}

func NewService(repo StoreRepository, player EzvizPlayer) *Service {
	return &Service{
		repo:        repo,
		player:      player,
		concurrency: map[string]*concurrencyState{},
	}
}

var ErrNotFound = errors.New("h5monitor: not found")
var ErrConcurrencyLimit = errors.New("h5monitor: concurrency limit reached")

type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return "validation error"
}

func (s *Service) GetMonitorHome(ctx context.Context, externalOrgID string) (MonitorHomeResponse, error) {
	orgID := strings.TrimSpace(externalOrgID)
	if orgID == "" {
		return MonitorHomeResponse{}, &ValidationError{Fields: map[string]string{"external_org_id": "机构 ID 不能为空"}}
	}
	store, err := s.repo.GetStoreByExternalOrgID(ctx, orgID)
	if err != nil {
		return MonitorHomeResponse{}, err
	}
	channels, err := s.repo.ListActiveChannelsByOrgID(ctx, orgID)
	if err != nil {
		return MonitorHomeResponse{}, err
	}
	return MonitorHomeResponse{
		ExternalOrgID: store.ExternalOrgID,
		StoreName:     store.Name,
		City:          store.City,
		Groups:        groupChannels(channels),
	}, nil
}

func (s *Service) ListMonitorStores(ctx context.Context) (MonitorStoresResponse, error) {
	stores, err := s.repo.ListMonitorStores(ctx)
	if err != nil {
		return MonitorStoresResponse{}, err
	}
	sort.SliceStable(stores, func(i, j int) bool {
		leftCity, rightCity := monitorStoreCity(stores[i].City), monitorStoreCity(stores[j].City)
		if leftCity != rightCity {
			return leftCity < rightCity
		}
		if stores[i].StoreName != stores[j].StoreName {
			return stores[i].StoreName < stores[j].StoreName
		}
		return stores[i].ExternalOrgID < stores[j].ExternalOrgID
	})

	groups := []MonitorStoreCityGroup{}
	for _, store := range stores {
		store.City = monitorStoreCity(store.City)
		if len(groups) == 0 || groups[len(groups)-1].City != store.City {
			groups = append(groups, MonitorStoreCityGroup{City: store.City})
		}
		groups[len(groups)-1].Stores = append(groups[len(groups)-1].Stores, store)
	}
	return MonitorStoresResponse{Cities: groups}, nil
}

func (s *Service) GetLiveURL(ctx context.Context, externalOrgID string, channelID int64, userID string, isAdmin bool, protocolValue string, qualityValue string) (LiveURLResponse, error) {
	startedAt := time.Now()
	channel, err := s.validateChannel(ctx, externalOrgID, channelID)
	if err != nil {
		log.Printf("h5monitor: live-url failed stage=validate external_org_id=%q channel_id=%d duration_ms=%d error=%q", externalOrgID, channelID, elapsedMilliseconds(startedAt), err)
		return LiveURLResponse{}, err
	}
	if err := s.acquireConcurrency(userID, isAdmin); err != nil {
		log.Printf("h5monitor: live-url failed stage=concurrency external_org_id=%q channel_id=%d device=%s channel_no=%d user=%s admin=%t duration_ms=%d error=%q", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, safeLogID(userID), isAdmin, elapsedMilliseconds(startedAt), err)
		return LiveURLResponse{}, err
	}
	account := channelToAccount(channel)
	_ = s.player.EnsureAACTransfer(ctx, account, channel.DeviceSerial, channel.ChannelNo)
	protocol, ezvizProtocol, supportH265 := normalizeLiveProtocol(protocolValue)
	quality := normalizeStreamQuality(qualityValue)
	log.Printf("h5monitor: live-url request external_org_id=%q channel_id=%d device=%s channel_no=%d protocol=%s quality=%d user=%s admin=%t", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, protocol, quality, safeLogID(userID), isAdmin)
	result, err := s.player.LiveAddress(ctx, account, ezviz.LiveAddressRequest{
		DeviceSerial: channel.DeviceSerial,
		ChannelNo:    channel.ChannelNo,
		Protocol:     ezvizProtocol,
		Type:         1,
		Quality:      quality,
		ExpireTime:   600,
		SupportH265:  supportH265,
		Mute:         ezviz.IntPtr(0),
	})
	if err != nil {
		s.releaseConcurrency(userID)
		log.Printf("h5monitor: live-url failed stage=ezviz external_org_id=%q channel_id=%d device=%s channel_no=%d protocol=%s quality=%d duration_ms=%d error=%q", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, protocol, quality, elapsedMilliseconds(startedAt), err)
		return LiveURLResponse{}, err
	}
	log.Printf("h5monitor: live-url completed external_org_id=%q channel_id=%d device=%s channel_no=%d protocol=%s quality=%d url_id=%s duration_ms=%d", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, protocol, quality, safeLogID(result.ID), elapsedMilliseconds(startedAt))
	return LiveURLResponse{URL: result.URL, ExpireTime: result.ExpireTime, URLID: result.ID, Protocol: protocol}, nil
}

func (s *Service) GetRecordSegments(ctx context.Context, externalOrgID string, channelID int64, dateValue string) (RecordSegmentsResponse, error) {
	startedAt := time.Now()
	channel, err := s.validateChannel(ctx, externalOrgID, channelID)
	if err != nil {
		log.Printf("h5monitor: record-segments failed stage=validate external_org_id=%q channel_id=%d date=%q duration_ms=%d error=%q", externalOrgID, channelID, dateValue, elapsedMilliseconds(startedAt), err)
		return RecordSegmentsResponse{}, err
	}
	date, normalizedDate, err := parseDate(dateValue)
	if err != nil {
		log.Printf("h5monitor: record-segments failed stage=parse-date external_org_id=%q channel_id=%d device=%s channel_no=%d date=%q duration_ms=%d error=%q", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, dateValue, elapsedMilliseconds(startedAt), err)
		return RecordSegmentsResponse{}, &ValidationError{Fields: map[string]string{"date": "日期格式应为 YYYY-MM-DD"}}
	}
	log.Printf("h5monitor: record-segments request external_org_id=%q channel_id=%d device=%s channel_no=%d date=%s", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, normalizedDate)
	result, err := s.player.QueryRecordSegments(ctx, channelToAccount(channel), ezviz.RecordSegmentsQuery{
		DeviceSerial: channel.DeviceSerial,
		ChannelNo:    channel.ChannelNo,
		Date:         date,
	})
	if err != nil {
		log.Printf("h5monitor: record-segments failed stage=ezviz external_org_id=%q channel_id=%d device=%s channel_no=%d date=%s duration_ms=%d error=%q", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, normalizedDate, elapsedMilliseconds(startedAt), err)
		return RecordSegmentsResponse{}, err
	}
	segments := make([]RecordSegmentResponse, 0, len(result.Records))
	for _, segment := range result.Records {
		segments = append(segments, RecordSegmentResponse{
			StartTime: segment.StartTime,
			EndTime:   segment.EndTime,
			Type:      segment.Type,
			TypeLabel: segmentTypeLabel(segment.Type),
		})
	}
	log.Printf("h5monitor: record-segments completed external_org_id=%q channel_id=%d device=%s channel_no=%d date=%s records=%d duration_ms=%d", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, normalizedDate, len(segments), elapsedMilliseconds(startedAt))
	return RecordSegmentsResponse{Date: normalizedDate, Segments: segments}, nil
}

func (s *Service) GetPlaybackURL(ctx context.Context, externalOrgID string, channelID int64, request PlaybackURLRequest) (PlaybackURLResponse, error) {
	startedAt := time.Now()
	channel, err := s.validateChannel(ctx, externalOrgID, channelID)
	if err != nil {
		log.Printf("h5monitor: playback-url failed stage=validate external_org_id=%q channel_id=%d duration_ms=%d error=%q", externalOrgID, channelID, elapsedMilliseconds(startedAt), err)
		return PlaybackURLResponse{}, err
	}
	if request.StartTime <= 0 || request.StopTime <= request.StartTime {
		log.Printf("h5monitor: playback-url failed stage=validate-time external_org_id=%q channel_id=%d device=%s channel_no=%d start=%d stop=%d duration_ms=%d", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, request.StartTime, request.StopTime, elapsedMilliseconds(startedAt))
		return PlaybackURLResponse{}, &ValidationError{Fields: map[string]string{"time": "回放时间范围无效"}}
	}
	if err := s.acquireConcurrency(request.UserID, request.IsAdmin); err != nil {
		log.Printf("h5monitor: playback-url failed stage=concurrency external_org_id=%q channel_id=%d device=%s channel_no=%d user=%s admin=%t duration_ms=%d error=%q", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, safeLogID(request.UserID), request.IsAdmin, elapsedMilliseconds(startedAt), err)
		return PlaybackURLResponse{}, err
	}
	log.Printf("h5monitor: playback-url request external_org_id=%q channel_id=%d device=%s channel_no=%d start=%d stop=%d user=%s admin=%t", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, request.StartTime, request.StopTime, safeLogID(request.UserID), request.IsAdmin)
	result, err := s.player.PlaybackAddress(ctx, channelToAccount(channel), ezviz.PlaybackRequest{
		DeviceSerial: channel.DeviceSerial,
		ChannelNo:    channel.ChannelNo,
		StartTime:    time.Unix(request.StartTime, 0),
		StopTime:     time.Unix(request.StopTime, 0),
		Protocol:     4,
		Quality:      2,
		ExpireTime:   600,
	})
	if err != nil {
		s.releaseConcurrency(request.UserID)
		log.Printf("h5monitor: playback-url failed stage=ezviz external_org_id=%q channel_id=%d device=%s channel_no=%d start=%d stop=%d duration_ms=%d error=%q", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, request.StartTime, request.StopTime, elapsedMilliseconds(startedAt), err)
		return PlaybackURLResponse{}, err
	}
	log.Printf("h5monitor: playback-url completed external_org_id=%q channel_id=%d device=%s channel_no=%d start=%d stop=%d url_id=%s duration_ms=%d", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, request.StartTime, request.StopTime, safeLogID(result.ID), elapsedMilliseconds(startedAt))
	return PlaybackURLResponse{URL: result.URL, ExpireTime: result.ExpireTime, URLID: result.ID}, nil
}

func (s *Service) DisableURL(ctx context.Context, externalOrgID string, channelID int64, urlID string, userID string) error {
	startedAt := time.Now()
	channel, err := s.validateChannel(ctx, externalOrgID, channelID)
	if err != nil {
		s.releaseConcurrency(userID)
		log.Printf("h5monitor: disable-url failed stage=validate external_org_id=%q channel_id=%d url_id=%s user=%s duration_ms=%d error=%q", externalOrgID, channelID, safeLogID(urlID), safeLogID(userID), elapsedMilliseconds(startedAt), err)
		return err
	}
	disableErr := s.player.DisableLiveAddress(ctx, channelToAccount(channel), ezviz.DisableLiveAddressRequest{
		DeviceSerial: channel.DeviceSerial,
		ChannelNo:    channel.ChannelNo,
		URLID:        strings.TrimSpace(urlID),
	})
	s.releaseConcurrency(userID)
	if disableErr != nil {
		log.Printf("h5monitor: disable-url failed stage=ezviz external_org_id=%q channel_id=%d device=%s channel_no=%d url_id=%s user=%s duration_ms=%d error=%q", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, safeLogID(urlID), safeLogID(userID), elapsedMilliseconds(startedAt), disableErr)
	} else {
		log.Printf("h5monitor: disable-url completed external_org_id=%q channel_id=%d device=%s channel_no=%d url_id=%s user=%s duration_ms=%d", externalOrgID, channelID, safeLogID(channel.DeviceSerial), channel.ChannelNo, safeLogID(urlID), safeLogID(userID), elapsedMilliseconds(startedAt))
	}
	return disableErr
}

func normalizeLiveProtocol(value string) (protocol string, ezvizProtocol int, supportH265 bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "hls", "m3u8":
		return "hls", 2, false
	default:
		return "flv", 4, true
	}
}

func normalizeStreamQuality(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "hd", "high", "高清", "1":
		return 1
	default:
		return 2
	}
}

func (s *Service) validateChannel(ctx context.Context, externalOrgID string, channelID int64) (*ChannelInfo, error) {
	orgID := strings.TrimSpace(externalOrgID)
	if orgID == "" {
		return nil, &ValidationError{Fields: map[string]string{"external_org_id": "机构 ID 不能为空"}}
	}
	store, err := s.repo.GetStoreByExternalOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	channel, err := s.repo.GetChannelByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if channel.StoreID != store.ID || !validChannel(*channel) {
		return nil, ErrNotFound
	}
	return channel, nil
}

func (s *Service) acquireConcurrency(userID string, isAdmin bool) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	maxCount := 15
	if isAdmin {
		maxCount = 20
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.concurrency[userID]
	if state == nil {
		state = &concurrencyState{}
		s.concurrency[userID] = state
	}
	state.MaxCount = maxCount
	if state.ActiveCount >= maxCount {
		return fmt.Errorf("%w: %d/%d", ErrConcurrencyLimit, state.ActiveCount, maxCount)
	}
	state.ActiveCount++
	state.AcquiredAt = time.Now()
	return nil
}

func (s *Service) releaseConcurrency(userID string) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.concurrency[userID]
	if state == nil || state.ActiveCount <= 0 {
		return
	}
	state.ActiveCount--
}

func groupChannels(channels []ChannelInfo) []MonitorGroup {
	filtered := make([]ChannelInfo, 0, len(channels))
	for _, channel := range channels {
		if validChannel(channel) {
			filtered = append(filtered, channel)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].ChannelNo < filtered[j].ChannelNo
	})

	byCategory := map[string][]MonitorChannel{}
	for _, channel := range filtered {
		category := channelCategory(channel)
		byCategory[category] = append(byCategory[category], MonitorChannel{
			ID:           channel.ID,
			ChannelNo:    channel.ChannelNo,
			ChannelName:  channel.ChannelName,
			Category:     category,
			AreaType:     channel.AreaType,
			SceneType:    channel.SceneType,
			AreaNumber:   channel.AreaNumber,
			BedLabel:     channel.BedLabel,
			AreaNote:     channel.AreaNote,
			ThumbnailURL: channel.ThumbnailURL,
		})
	}

	order := []string{"consultation", "treatment", "beauty", "front_waiting", "other"}
	groups := make([]MonitorGroup, 0, len(order))
	for _, category := range order {
		channels := byCategory[category]
		if len(channels) == 0 {
			continue
		}
		sortMonitorChannels(category, channels)
		groups = append(groups, MonitorGroup{
			Category: category,
			Label:    channelCategoryLabel(category),
			Channels: channels,
		})
	}
	return groups
}

func validChannel(channel ChannelInfo) bool {
	if !channel.IsActive || channel.ChannelNo <= 0 {
		return false
	}
	if strings.TrimSpace(channel.DeviceSerial) == "" || channel.EzvizAccountID <= 0 {
		return false
	}
	if strings.TrimSpace(channel.AppKey) == "" || strings.TrimSpace(channel.AppSecret) == "" {
		return false
	}
	return true
}

func monitorStoreCity(city string) string {
	city = strings.TrimSpace(city)
	if city == "" {
		return "未分组"
	}
	return city
}

func channelCategory(channel ChannelInfo) string {
	switch channel.AreaType {
	case "consultation":
		return "consultation"
	case "treatment", "vip_treatment":
		return "treatment"
	case "beauty":
		return "beauty"
	}
	text := channel.ChannelName + " " + channel.AreaNote + " " + channel.SceneType
	if strings.Contains(text, "前台") || strings.Contains(text, "候诊") || strings.Contains(text, "等候") ||
		channel.SceneType == "front_desk" || channel.SceneType == "waiting_area" {
		return "front_waiting"
	}
	return "other"
}

func sortMonitorChannels(category string, channels []MonitorChannel) {
	sort.SliceStable(channels, func(i, j int) bool {
		left, right := channels[i], channels[j]
		switch category {
		case "consultation", "treatment", "beauty":
			if left.AreaNumber != right.AreaNumber {
				return left.AreaNumber < right.AreaNumber
			}
			if left.AreaNote != right.AreaNote {
				return left.AreaNote < right.AreaNote
			}
			if left.ChannelName != right.ChannelName {
				return left.ChannelName < right.ChannelName
			}
			return left.ChannelNo < right.ChannelNo
		case "front_waiting", "other":
			leftText := firstNonEmpty(left.AreaNote, left.ChannelName, strconv.Itoa(left.ChannelNo))
			rightText := firstNonEmpty(right.AreaNote, right.ChannelName, strconv.Itoa(right.ChannelNo))
			if leftText != rightText {
				return leftText < rightText
			}
			return left.ChannelNo < right.ChannelNo
		default:
			return left.ChannelNo < right.ChannelNo
		}
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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

func safeLogID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	if len(value) <= 8 {
		return value
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func parseDate(value string) (time.Time, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		now := time.Now()
		return now, now.Format("2006-01-02"), nil
	}
	date, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, "", err
	}
	return date, value, nil
}

func channelToAccount(channel *ChannelInfo) ezviz.Account {
	return ezviz.Account{
		Name:        channel.AccountName,
		AppKey:      channel.AppKey,
		AppSecret:   channel.AppSecret,
		AccessToken: channel.AccessToken,
	}
}
