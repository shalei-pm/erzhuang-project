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

type fakePlayer struct {
	liveInput     ezviz.LiveAddressRequest
	playbackInput ezviz.PlaybackRequest
	disableInput  ezviz.DisableLiveAddressRequest
	disableErr    error
	disabledIDs   []string
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
		{ID: 1, StoreID: 10, ChannelNo: 6, ChannelName: "治疗6", Status: "confirmed_business", IsActive: true, AreaType: "treatment", AreaNumber: 6, DeviceSerial: "GN0941203", AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
		{ID: 2, StoreID: 10, ChannelNo: 2, ChannelName: "面诊2", Status: "confirmed_business", IsActive: true, AreaType: "consultation", AreaNumber: 2, DeviceSerial: "GN0941203", AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
		{ID: 3, StoreID: 10, ChannelNo: 4, ChannelName: "VIP治疗", Status: "confirmed_business", IsActive: true, AreaType: "vip_treatment", AreaNumber: 1, DeviceSerial: "GN0941203", AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
		{ID: 4, StoreID: 10, ChannelNo: 8, ChannelName: "前台", Status: "confirmed_non_business", IsActive: true, SceneType: "unknown", AreaNote: "前台等候区", DeviceSerial: "GN0941203", AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
		{ID: 5, StoreID: 10, ChannelNo: 1, ChannelName: "过道", Status: "confirmed_non_business", IsActive: true, SceneType: "corridor", DeviceSerial: "GN0941203", AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
		{ID: 6, StoreID: 10, ChannelNo: 9, ChannelName: "其他录像机", Status: "confirmed_business", IsActive: true, AreaType: "consultation", AreaNumber: 9, DeviceSerial: "GQ2603603", AccountName: "华北", AppKey: "app-key", AppSecret: "app-secret", AccessToken: "access-token"},
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
}

func TestMonitorHomeRejectsNonPilotOrg(t *testing.T) {
	service, _ := newFakeService()
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodGet, "/api/h5/orgs/10031/monitor", nil)
	request.SetPathValue("externalOrgId", "10031")
	response := httptest.NewRecorder()

	handler.getMonitorHome(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s, want 404", response.Code, response.Body.String())
	}
}

func TestPilotHomeAndPlaybackRejectNonPilotDevice(t *testing.T) {
	service, _ := newFakeService()
	handler := NewHandler(service)

	homeRequest := httptest.NewRequest(http.MethodGet, "/api/h5/orgs/10030/monitor", nil)
	homeRequest.SetPathValue("externalOrgId", "10030")
	homeResponse := httptest.NewRecorder()
	handler.getMonitorHome(homeResponse, homeRequest)
	if homeResponse.Code != http.StatusOK {
		t.Fatalf("home status = %d body=%s", homeResponse.Code, homeResponse.Body.String())
	}
	if strings.Contains(homeResponse.Body.String(), `"id":6`) {
		t.Fatalf("non-pilot device channel appeared in home response: %s", homeResponse.Body.String())
	}

	playRequest := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10030/monitor/channels/6/live-url", strings.NewReader(`{"user_id":"u1"}`))
	playRequest.SetPathValue("externalOrgId", "10030")
	playRequest.SetPathValue("channelId", "6")
	playResponse := httptest.NewRecorder()
	handler.getLiveURL(playResponse, playRequest)
	if playResponse.Code != http.StatusNotFound {
		t.Fatalf("play status = %d body=%s, want 404", playResponse.Code, playResponse.Body.String())
	}
}

func TestLiveURLUsesH5FLVParameters(t *testing.T) {
	service, player := newFakeService()
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/orgs/10030/monitor/channels/2/live-url", strings.NewReader(`{"user_id":"u1"}`))
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
	if player.liveInput.Mute == nil || *player.liveInput.Mute != 0 {
		t.Fatalf("expected explicit mute=0, got %#v", player.liveInput.Mute)
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
