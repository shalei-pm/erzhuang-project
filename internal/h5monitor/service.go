package h5monitor

import (
	"context"
	"errors"
	"fmt"
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

const (
	defaultPilotExternalOrgID = "10030"
	defaultPilotDeviceSerial  = "GN0941203"
)

type Service struct {
	repo        StoreRepository
	player      EzvizPlayer
	pilotOrgID  string
	pilotDevice string
	mu          sync.Mutex
	concurrency map[string]*concurrencyState
}

func NewService(repo StoreRepository, player EzvizPlayer) *Service {
	return &Service{
		repo:        repo,
		player:      player,
		pilotOrgID:  defaultPilotExternalOrgID,
		pilotDevice: defaultPilotDeviceSerial,
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
	if !s.isPilotAllowedOrg(orgID) {
		return MonitorHomeResponse{}, ErrNotFound
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
		Groups:        groupChannels(s.filterPilotChannels(orgID, channels)),
	}, nil
}

func (s *Service) GetLiveURL(ctx context.Context, externalOrgID string, channelID int64, userID string, isAdmin bool) (LiveURLResponse, error) {
	channel, err := s.validateChannel(ctx, externalOrgID, channelID)
	if err != nil {
		return LiveURLResponse{}, err
	}
	if err := s.acquireConcurrency(userID, isAdmin); err != nil {
		return LiveURLResponse{}, err
	}
	account := channelToAccount(channel)
	_ = s.player.EnsureAACTransfer(ctx, account, channel.DeviceSerial, channel.ChannelNo)
	result, err := s.player.LiveAddress(ctx, account, ezviz.LiveAddressRequest{
		DeviceSerial: channel.DeviceSerial,
		ChannelNo:    channel.ChannelNo,
		Protocol:     4,
		Type:         1,
		Quality:      2,
		ExpireTime:   600,
		SupportH265:  true,
		Mute:         ezviz.IntPtr(0),
	})
	if err != nil {
		s.releaseConcurrency(userID)
		return LiveURLResponse{}, err
	}
	return LiveURLResponse{URL: result.URL, ExpireTime: result.ExpireTime, URLID: result.ID}, nil
}

func (s *Service) GetRecordSegments(ctx context.Context, externalOrgID string, channelID int64, dateValue string) (RecordSegmentsResponse, error) {
	channel, err := s.validateChannel(ctx, externalOrgID, channelID)
	if err != nil {
		return RecordSegmentsResponse{}, err
	}
	date, normalizedDate, err := parseDate(dateValue)
	if err != nil {
		return RecordSegmentsResponse{}, &ValidationError{Fields: map[string]string{"date": "日期格式应为 YYYY-MM-DD"}}
	}
	result, err := s.player.QueryRecordSegments(ctx, channelToAccount(channel), ezviz.RecordSegmentsQuery{
		DeviceSerial: channel.DeviceSerial,
		ChannelNo:    channel.ChannelNo,
		Date:         date,
	})
	if err != nil {
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
	return RecordSegmentsResponse{Date: normalizedDate, Segments: segments}, nil
}

func (s *Service) GetPlaybackURL(ctx context.Context, externalOrgID string, channelID int64, request PlaybackURLRequest) (PlaybackURLResponse, error) {
	channel, err := s.validateChannel(ctx, externalOrgID, channelID)
	if err != nil {
		return PlaybackURLResponse{}, err
	}
	if request.StartTime <= 0 || request.StopTime <= request.StartTime {
		return PlaybackURLResponse{}, &ValidationError{Fields: map[string]string{"time": "回放时间范围无效"}}
	}
	if err := s.acquireConcurrency(request.UserID, request.IsAdmin); err != nil {
		return PlaybackURLResponse{}, err
	}
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
		return PlaybackURLResponse{}, err
	}
	return PlaybackURLResponse{URL: result.URL, ExpireTime: result.ExpireTime, URLID: result.ID}, nil
}

func (s *Service) DisableURL(ctx context.Context, externalOrgID string, channelID int64, urlID string, userID string) error {
	channel, err := s.validateChannel(ctx, externalOrgID, channelID)
	if err != nil {
		s.releaseConcurrency(userID)
		return err
	}
	disableErr := s.player.DisableLiveAddress(ctx, channelToAccount(channel), ezviz.DisableLiveAddressRequest{
		DeviceSerial: channel.DeviceSerial,
		ChannelNo:    channel.ChannelNo,
		URLID:        strings.TrimSpace(urlID),
	})
	s.releaseConcurrency(userID)
	return disableErr
}

func (s *Service) validateChannel(ctx context.Context, externalOrgID string, channelID int64) (*ChannelInfo, error) {
	orgID := strings.TrimSpace(externalOrgID)
	if orgID == "" {
		return nil, &ValidationError{Fields: map[string]string{"external_org_id": "机构 ID 不能为空"}}
	}
	if !s.isPilotAllowedOrg(orgID) {
		return nil, ErrNotFound
	}
	store, err := s.repo.GetStoreByExternalOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	channel, err := s.repo.GetChannelByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if channel.StoreID != store.ID || !validChannel(*channel) || !s.isPilotAllowedChannel(orgID, *channel) {
		return nil, ErrNotFound
	}
	if strings.TrimSpace(channel.DeviceSerial) == "" || channel.ChannelNo <= 0 || strings.TrimSpace(channel.AppKey) == "" || strings.TrimSpace(channel.AppSecret) == "" {
		return nil, &ValidationError{Fields: map[string]string{"channel": "通道缺少可用的萤石云播放配置"}}
	}
	return channel, nil
}

func (s *Service) isPilotAllowedOrg(externalOrgID string) bool {
	return strings.TrimSpace(externalOrgID) == s.pilotOrgID
}

func (s *Service) filterPilotChannels(externalOrgID string, channels []ChannelInfo) []ChannelInfo {
	result := make([]ChannelInfo, 0, len(channels))
	for _, channel := range channels {
		if s.isPilotAllowedChannel(externalOrgID, channel) {
			result = append(result, channel)
		}
	}
	return result
}

func (s *Service) isPilotAllowedChannel(externalOrgID string, channel ChannelInfo) bool {
	return s.isPilotAllowedOrg(externalOrgID) && strings.EqualFold(strings.TrimSpace(channel.DeviceSerial), s.pilotDevice)
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
	if !channel.IsActive {
		return false
	}
	return channel.Status == "confirmed_business" || channel.Status == "confirmed_non_business"
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
