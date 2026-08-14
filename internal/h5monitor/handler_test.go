package h5monitor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
)

type fakeRepo struct {
	store    *StoreInfo
	channels []ChannelInfo
	byID     map[int64]ChannelInfo
}

func (r *fakeRepo) GetStoreByExternalOrgID(ctx context.Context, externalOrgID string) (*StoreInfo, error) {
	if r.store == nil || r.store.ExternalOrgID != externalOrgID {
		return nil, ErrNotFound
	}
	copy := *r.store
	return &copy, nil
}

func (r *fakeRepo) ListActiveChannelsByOrgID(ctx context.Context, externalOrgID string) ([]ChannelInfo, error) {
	if r.store == nil || r.store.ExternalOrgID != externalOrgID {
		return nil, ErrNotFound
	}
	result := append([]ChannelInfo(nil), r.channels...)
	return result, nil
}

func (r *fakeRepo) GetChannelByID(ctx context.Context, channelID int64) (*ChannelInfo, error) {
	channel, ok := r.byID[channelID]
	if !ok {
		return nil, ErrNotFound
	}
	copy := channel
	return &copy, nil
}

func (r *fakeRepo) ListMonitorStores(ctx context.Context) ([]MonitorStoreInfo, error) {
	if r.store == nil {
		return nil, nil
	}
	count := 0
	for _, channel := range r.channels {
		if validChannel(channel) {
			count++
		}
	}
	if count == 0 {
		return nil, nil
	}
	return []MonitorStoreInfo{{
		ExternalOrgID:         r.store.ExternalOrgID,
		StoreName:             r.store.Name,
		City:                  r.store.City,
		AvailableChannelCount: count,
	}}, nil
}

type fakePlayer struct {
	liveInput     ezviz.LiveAddressRequest
	playbackInput ezviz.PlaybackRequest
	disableInput  ezviz.DisableLiveAddressRequest
	disableErr    error
	disabledIDs   []string
}

type fakeAuthorizer struct {
	allowed map[string]bool
}

func (a fakeAuthorizer) CurrentUser(r *http.Request) (AuthContext, error) {
	return AuthContext{UserID: 1, Role: "viewer"}, nil
}

func (a fakeAuthorizer) CanViewMonitorStore(r *http.Request, externalOrgID string) (bool, error) {
	return a.allowed[externalOrgID], nil
}

func (a fakeAuthorizer) FilterMonitorStores(r *http.Request, response MonitorStoresResponse) (MonitorStoresResponse, error) {
	filtered := MonitorStoresResponse{}
	for _, group := range response.Cities {
		nextGroup := MonitorStoreCityGroup{City: group.City}
		for _, store := range group.Stores {
			if a.allowed[store.ExternalOrgID] {
				nextGroup.Stores = append(nextGroup.Stores, store)
			}
		}
		if len(nextGroup.Stores) > 0 {
			filtered.Cities = append(filtered.Cities, nextGroup)
		}
	}
	return filtered, nil
}

func (p *fakePlayer) EnsureAACTransfer(ctx context.Context, account ezviz.Account, deviceSerial string, channelNo int) error {
	return errors.New("aac best effort failure")
}

func (p *fakePlayer) LiveAddress(ctx context.Context, account ezviz.Account, input ezviz.LiveAddressRequest) (ezviz.LiveAddressResult, error) {
	p.liveInput = input
	return ezviz.LiveAddressResult{ID: "live-url-id", URL: "https://example.test/live.flv", ExpireTime: "2026-12-31 23:59:59"}, nil
}

func (p *fakePlayer) PlaybackAddress(ctx context.Context, account ezviz.Account, input ezviz.PlaybackRequest) (ezviz.PlaybackResult, error) {
	p.playbackInput = input
	return ezviz.PlaybackResult{ID: "play-url-id", URL: "https://example.test/play.flv", ExpireTime: "2026-12-31 23:59:59"}, nil
}

func (p *fakePlayer) QueryRecordSegments(ctx context.Context, account ezviz.Account, input ezviz.RecordSegmentsQuery) (ezviz.RecordSegmentsResult, error) {
	return ezviz.RecordSegmentsResult{Records: []ezviz.RecordSegment{{StartTime: 1731945592, EndTime: 1731949200, Type: "ALARM"}}}, nil
}

func (p *fakePlayer) DisableLiveAddress(ctx context.Context, account ezviz.Account, input ezviz.DisableLiveAddressRequest) error {
	p.disableInput = input
	p.disabledIDs = append(p.disabledIDs, input.URLID)
	return p.disableErr
}

func newFakeService() (*Service, *fakePlayer) {
	channels := []ChannelInfo{
		{ID: 1, StoreID: 10, ChannelNo: 6, ChannelName: "治疗6", Status: "confirmed_business", IsActive: true, AreaType: "treatment", AreaNumber: 6, DeviceSerial: "GN0941203", EzvizAccountID: 1, AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
		{ID: 2, StoreID: 10, ChannelNo: 2, ChannelName: "面诊2", Status: "confirmed_business", IsActive: true, AreaType: "consultation", AreaNumber: 2, DeviceSerial: "GN0941203", EzvizAccountID: 1, AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
		{ID: 3, StoreID: 10, ChannelNo: 4, ChannelName: "VIP治疗", Status: "confirmed_business", IsActive: true, AreaType: "vip_treatment", AreaNumber: 1, DeviceSerial: "GN0941203", EzvizAccountID: 1, AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
		{ID: 4, StoreID: 10, ChannelNo: 8, ChannelName: "前台", Status: "confirmed_non_business", IsActive: true, SceneType: "unknown", AreaNote: "前台等候区", DeviceSerial: "GN0941203", EzvizAccountID: 1, AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
		{ID: 5, StoreID: 10, ChannelNo: 1, ChannelName: "过道", Status: "confirmed_non_business", IsActive: true, SceneType: "corridor", DeviceSerial: "GN0941203", EzvizAccountID: 1, AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
		{ID: 6, StoreID: 10, ChannelNo: 9, ChannelName: "其他录像机", Status: "confirmed_business", IsActive: true, AreaType: "consultation", AreaNumber: 9, DeviceSerial: "GQ2603603", EzvizAccountID: 1, AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
	}
	repo := &fakeRepo{
		store:    &StoreInfo{ID: 10, Name: "北京测试店", City: "北京", ExternalOrgID: "10030"},
		channels: channels,
		byID:     map[int64]ChannelInfo{},
	}
	for _, channel := range channels {
		repo.byID[channel.ID] = channel
	}
	player := &fakePlayer{}
	return NewService(repo, player), player
}

func TestSnapshotRefreshRouteIsNotRegisteredForH5Monitor(t *testing.T) {
	service, _ := newFakeService()
	mux := http.NewServeMux()
	RegisterRoutes(mux, service)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10030/monitor/channels/2/snapshot", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s, want 404", response.Code, response.Body.String())
	}
}

func TestMonitorStoresListsCitiesAndStoresWithEffectiveChannels(t *testing.T) {
	service, _ := newFakeService()
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodGet, "/api/h5/monitor/stores", nil)
	response := httptest.NewRecorder()

	handler.getMonitorStores(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var result MonitorStoresResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result.Cities) != 1 {
		t.Fatalf("cities = %#v, want one city", result.Cities)
	}
	if result.Cities[0].City != "北京" {
		t.Fatalf("city = %q, want 北京", result.Cities[0].City)
	}
	if len(result.Cities[0].Stores) != 1 {
		t.Fatalf("stores = %#v, want one store", result.Cities[0].Stores)
	}
	store := result.Cities[0].Stores[0]
	if store.ExternalOrgID != "10030" || store.StoreName != "北京测试店" || store.AvailableChannelCount != 6 {
		t.Fatalf("unexpected store: %#v", store)
	}
}

func TestH5MonitorAuthorizerFiltersStores(t *testing.T) {
	service, _ := newFakeService()
	handler := NewHandlerWithAuthorizer(service, fakeAuthorizer{allowed: map[string]bool{}})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/monitor/stores", nil)
	response := httptest.NewRecorder()

	handler.getMonitorStores(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result MonitorStoresResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Cities) != 0 {
		t.Fatalf("expected no visible stores, got %#v", result.Cities)
	}
}

func TestH5MonitorAuthorizerBlocksDirectMonitorHome(t *testing.T) {
	service, _ := newFakeService()
	handler := NewHandlerWithAuthorizer(service, fakeAuthorizer{allowed: map[string]bool{}})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/orgs/10030/monitor", nil)
	request.SetPathValue("externalOrgId", "10030")
	response := httptest.NewRecorder()

	handler.getMonitorHome(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s, want 403", response.Code, response.Body.String())
	}
}

func TestMonitorStoresCanCountUnconfirmedEffectiveChannels(t *testing.T) {
	channel := ChannelInfo{ID: 71, StoreID: 71, ChannelNo: 1, ChannelName: "待确认通道", Status: "pending_confirmation", IsActive: true, DeviceSerial: "PENDING001", EzvizAccountID: 7, AccountName: "华东", AppKey: "app-key", AppSecret: "app-secret"}
	repo := &fakeRepo{
		store:    &StoreInfo{ID: 71, Name: "待确认有效门店", City: "", ExternalOrgID: "10071"},
		channels: []ChannelInfo{channel},
		byID:     map[int64]ChannelInfo{71: channel},
	}
	handler := NewHandler(NewService(repo, &fakePlayer{}))
	storesRequest := httptest.NewRequest(http.MethodGet, "/api/h5/monitor/stores", nil)
	storesResponse := httptest.NewRecorder()

	handler.getMonitorStores(storesResponse, storesRequest)

	if storesResponse.Code != http.StatusOK {
		t.Fatalf("stores status = %d body=%s", storesResponse.Code, storesResponse.Body.String())
	}
	var result MonitorStoresResponse
	if err := json.NewDecoder(storesResponse.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result.Cities) != 1 || result.Cities[0].City != "未分组" || len(result.Cities[0].Stores) != 1 {
		t.Fatalf("unexpected city grouping: %#v", result.Cities)
	}
	if result.Cities[0].Stores[0].AvailableChannelCount != 1 {
		t.Fatalf("available channel count = %d, want 1", result.Cities[0].Stores[0].AvailableChannelCount)
	}

	homeRequest := httptest.NewRequest(http.MethodGet, "/api/h5/orgs/10071/monitor", nil)
	homeRequest.SetPathValue("externalOrgId", "10071")
	homeResponse := httptest.NewRecorder()
	handler.getMonitorHome(homeResponse, homeRequest)
	if homeResponse.Code != http.StatusOK {
		t.Fatalf("home status = %d body=%s", homeResponse.Code, homeResponse.Body.String())
	}
	if !strings.Contains(homeResponse.Body.String(), `"id":71`) || !strings.Contains(homeResponse.Body.String(), `"category":"other"`) {
		t.Fatalf("pending effective channel did not appear in monitor home: %s", homeResponse.Body.String())
	}
}

func TestMonitorHomeReturnsNotFoundForMissingStore(t *testing.T) {
	handler := NewHandler(NewService(&fakeRepo{}, &fakePlayer{}))
	request := httptest.NewRequest(http.MethodGet, "/api/h5/orgs/404/monitor", nil)
	request.SetPathValue("externalOrgId", "404")
	response := httptest.NewRecorder()

	handler.getMonitorHome(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s, want 404", response.Code, response.Body.String())
	}
}

func TestLiveURLReturnsNotFoundForMissingChannel(t *testing.T) {
	repo := &fakeRepo{
		store: &StoreInfo{ID: 80, Name: "测试门店", City: "北京", ExternalOrgID: "10080"},
		byID:  map[int64]ChannelInfo{},
	}
	handler := NewHandler(NewService(repo, &fakePlayer{}))
	request := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10080/monitor/channels/999/live-url", strings.NewReader(`{"user_id":"u1"}`))
	request.SetPathValue("externalOrgId", "10080")
	request.SetPathValue("channelId", "999")
	response := httptest.NewRecorder()

	handler.getLiveURL(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s, want 404", response.Code, response.Body.String())
	}
}

func TestMonitorHomeGroupsChannelsAndDoesNotLeakSecrets(t *testing.T) {
	service, _ := newFakeService()
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodGet, "/api/h5/orgs/10030/monitor", nil)
	request.SetPathValue("externalOrgId", "10030")
	response := httptest.NewRecorder()

	handler.getMonitorHome(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, secret := range []string{"GN0941203", "GQ2603603", "app-key", "app-secret", "access-token", "华北"} {
		if strings.Contains(body, secret) {
			t.Fatalf("response leaked secret/internal value %q in %s", secret, body)
		}
	}
	var payload MonitorHomeResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	gotOrder := make([]string, 0, len(payload.Groups))
	for _, group := range payload.Groups {
		gotOrder = append(gotOrder, group.Category)
	}
	wantOrder := []string{"consultation", "treatment", "front_waiting", "other"}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("group order = %v, want %v", gotOrder, wantOrder)
	}
	if payload.Groups[1].Channels[0].ID != 3 || payload.Groups[1].Channels[1].ID != 1 {
		t.Fatalf("treatment channels not sorted by area number: %#v", payload.Groups[1].Channels)
	}
	totalChannels := 0
	for _, group := range payload.Groups {
		totalChannels += len(group.Channels)
	}
	if totalChannels != 6 {
		t.Fatalf("home channel count = %d, want 6 to match monitor store count", totalChannels)
	}
}

func TestMonitorHomeAllowsAnyStoreWithExternalOrgID(t *testing.T) {
	channels := []ChannelInfo{
		{ID: 31, StoreID: 31, ChannelNo: 1, ChannelName: "治疗1", Status: "confirmed_business", IsActive: true, AreaType: "treatment", AreaNumber: 1, DeviceSerial: "ANY001", EzvizAccountID: 3, AccountName: "华东", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
	}
	repo := &fakeRepo{
		store:    &StoreInfo{ID: 31, Name: "非试点门店", City: "上海", ExternalOrgID: "10031"},
		channels: channels,
		byID:     map[int64]ChannelInfo{31: channels[0]},
	}
	service := NewService(repo, &fakePlayer{})
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodGet, "/api/h5/orgs/10031/monitor", nil)
	request.SetPathValue("externalOrgId", "10031")
	response := httptest.NewRecorder()

	handler.getMonitorHome(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"store_name":"非试点门店"`) || !strings.Contains(response.Body.String(), `"id":31`) {
		t.Fatalf("home response did not use non-pilot store channels: %s", response.Body.String())
	}
}

func TestShanghaiKaidePilotUsesStoreOwnChannels(t *testing.T) {
	channels := []ChannelInfo{
		{ID: 11, StoreID: 47, ChannelNo: 1, ChannelName: "上海通道1", Status: "confirmed_business", IsActive: true, AreaType: "consultation", AreaNumber: 1, DeviceSerial: "SHANGHAI001", EzvizAccountID: 4, AccountName: "华东", AppKey: "east-key", AppSecret: "east-secret", AccessToken: "east-token"},
	}
	repo := &fakeRepo{
		store:    &StoreInfo{ID: 47, Name: "新氧青春诊所(上海凯德晶萃店)", City: "上海", ExternalOrgID: "10047"},
		channels: channels,
		byID:     map[int64]ChannelInfo{11: channels[0]},
	}
	player := &fakePlayer{}
	service := NewService(repo, player)
	handler := NewHandler(service)

	homeRequest := httptest.NewRequest(http.MethodGet, "/api/h5/orgs/10047/monitor", nil)
	homeRequest.SetPathValue("externalOrgId", "10047")
	homeResponse := httptest.NewRecorder()
	handler.getMonitorHome(homeResponse, homeRequest)
	if homeResponse.Code != http.StatusOK {
		t.Fatalf("home status = %d body=%s", homeResponse.Code, homeResponse.Body.String())
	}
	if !strings.Contains(homeResponse.Body.String(), `"store_name":"新氧青春诊所(上海凯德晶萃店)"`) || !strings.Contains(homeResponse.Body.String(), `"id":11`) {
		t.Fatalf("home response did not use Shanghai store channels: %s", homeResponse.Body.String())
	}

	playRequest := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10047/monitor/channels/11/live-url", strings.NewReader(`{"user_id":"u1"}`))
	playRequest.SetPathValue("externalOrgId", "10047")
	playRequest.SetPathValue("channelId", "11")
	playResponse := httptest.NewRecorder()
	handler.getLiveURL(playResponse, playRequest)
	if playResponse.Code != http.StatusOK {
		t.Fatalf("play status = %d body=%s", playResponse.Code, playResponse.Body.String())
	}
	if player.liveInput.DeviceSerial != "SHANGHAI001" || player.liveInput.ChannelNo != 1 {
		t.Fatalf("unexpected Shanghai live input: %#v", player.liveInput)
	}
}

func TestBeijingHomeAndPlaybackAllowStoreOwnEffectiveChannels(t *testing.T) {
	service, player := newFakeService()
	handler := NewHandler(service)

	homeRequest := httptest.NewRequest(http.MethodGet, "/api/h5/orgs/10030/monitor", nil)
	homeRequest.SetPathValue("externalOrgId", "10030")
	homeResponse := httptest.NewRecorder()
	handler.getMonitorHome(homeResponse, homeRequest)
	if homeResponse.Code != http.StatusOK {
		t.Fatalf("home status = %d body=%s", homeResponse.Code, homeResponse.Body.String())
	}
	if !strings.Contains(homeResponse.Body.String(), `"id":6`) {
		t.Fatalf("store-owned effective channel did not appear in home response: %s", homeResponse.Body.String())
	}

	playRequest := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10030/monitor/channels/6/live-url", strings.NewReader(`{"user_id":"u1"}`))
	playRequest.SetPathValue("externalOrgId", "10030")
	playRequest.SetPathValue("channelId", "6")
	playResponse := httptest.NewRecorder()
	handler.getLiveURL(playResponse, playRequest)
	if playResponse.Code != http.StatusOK {
		t.Fatalf("play status = %d body=%s", playResponse.Code, playResponse.Body.String())
	}
	if player.liveInput.DeviceSerial != "GQ2603603" || player.liveInput.ChannelNo != 9 {
		t.Fatalf("unexpected Beijing live input: %#v", player.liveInput)
	}
}

func TestLiveURLUsesRequestedHLSParameters(t *testing.T) {
	service, player := newFakeService()
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10030/monitor/channels/2/live-url", strings.NewReader(`{"user_id":"u1","protocol":"hls"}`))
	request.SetPathValue("externalOrgId", "10030")
	request.SetPathValue("channelId", "2")
	response := httptest.NewRecorder()

	handler.getLiveURL(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if player.liveInput.Protocol != 2 || player.liveInput.Type != 1 || player.liveInput.Quality != 2 || player.liveInput.ExpireTime != 600 || player.liveInput.SupportH265 {
		t.Fatalf("unexpected live input: %#v", player.liveInput)
	}
	if player.liveInput.Mute == nil || *player.liveInput.Mute != 0 {
		t.Fatalf("expected explicit mute=0, got %#v", player.liveInput.Mute)
	}
	if !strings.Contains(response.Body.String(), `"protocol":"hls"`) {
		t.Fatalf("expected hls protocol in response, got %s", response.Body.String())
	}
}

func TestLiveURLUsesRequestedFLVParameters(t *testing.T) {
	service, player := newFakeService()
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10030/monitor/channels/2/live-url", strings.NewReader(`{"user_id":"u1","protocol":"flv"}`))
	request.SetPathValue("externalOrgId", "10030")
	request.SetPathValue("channelId", "2")
	response := httptest.NewRecorder()

	handler.getLiveURL(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if player.liveInput.Protocol != 4 || player.liveInput.Type != 1 || player.liveInput.Quality != 2 || player.liveInput.ExpireTime != 600 || !player.liveInput.SupportH265 {
		t.Fatalf("unexpected live input: %#v", player.liveInput)
	}
	if !strings.Contains(response.Body.String(), `"protocol":"flv"`) {
		t.Fatalf("expected flv protocol in response, got %s", response.Body.String())
	}
}

func TestLiveURLUsesRequestedHDQuality(t *testing.T) {
	service, player := newFakeService()
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10030/monitor/channels/2/live-url", strings.NewReader(`{"user_id":"u1","protocol":"flv","quality":"hd"}`))
	request.SetPathValue("externalOrgId", "10030")
	request.SetPathValue("channelId", "2")
	response := httptest.NewRecorder()

	handler.getLiveURL(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if player.liveInput.Quality != 1 {
		t.Fatalf("live quality = %d, want 1 for hd", player.liveInput.Quality)
	}
}

func TestPlaybackURLUsesRequestedTimeRange(t *testing.T) {
	service, player := newFakeService()
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10030/monitor/channels/2/playback-url", strings.NewReader(`{"start_time":1731945592,"stop_time":1731949200,"user_id":"u1"}`))
	request.SetPathValue("externalOrgId", "10030")
	request.SetPathValue("channelId", "2")
	response := httptest.NewRecorder()

	handler.getPlaybackURL(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if player.playbackInput.Protocol != 4 || player.playbackInput.Quality != 2 || player.playbackInput.ExpireTime != 600 {
		t.Fatalf("unexpected playback input: %#v", player.playbackInput)
	}
	if !player.playbackInput.StartTime.Equal(time.Unix(1731945592, 0)) || !player.playbackInput.StopTime.Equal(time.Unix(1731949200, 0)) {
		t.Fatalf("unexpected playback range: %#v", player.playbackInput)
	}
}

func TestPlaybackURLIgnoresRequestedQuality(t *testing.T) {
	service, player := newFakeService()
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10030/monitor/channels/2/playback-url", strings.NewReader(`{"start_time":1731945592,"stop_time":1731949200,"user_id":"u1","quality":"hd"}`))
	request.SetPathValue("externalOrgId", "10030")
	request.SetPathValue("channelId", "2")
	response := httptest.NewRecorder()

	handler.getPlaybackURL(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if player.playbackInput.Quality != 2 {
		t.Fatalf("playback quality = %d, want 2 because replay does not support quality switching", player.playbackInput.Quality)
	}
}

func TestDisableURLReturnsOKFalseOnEzvizFailure(t *testing.T) {
	service, player := newFakeService()
	player.disableErr = errors.New("ezviz disable failed")
	handler := NewHandler(service)

	request := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10030/monitor/channels/2/disable-url?user_id=u1", strings.NewReader(`{"url_id":"live-url-id"}`))
	request.SetPathValue("externalOrgId", "10030")
	request.SetPathValue("channelId", "2")
	response := httptest.NewRecorder()

	handler.disableURL(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"ok":false`) {
		t.Fatalf("expected ok false, got %s", response.Body.String())
	}
	if len(player.disabledIDs) != 1 || player.disabledIDs[0] != "live-url-id" {
		t.Fatalf("unexpected disabled ids: %v", player.disabledIDs)
	}
	if player.disableInput.DeviceSerial != "GN0941203" || player.disableInput.ChannelNo != 2 {
		t.Fatalf("unexpected disable input: %#v", player.disableInput)
	}
}
