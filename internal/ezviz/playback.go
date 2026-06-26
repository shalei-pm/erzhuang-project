package ezviz

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PlaybackRequest struct {
	DeviceSerial string
	ChannelNo    int
	StartTime    time.Time
	StopTime     time.Time
	Protocol     int
	Quality      int
	ExpireTime   int
	Code         string
}

type PlaybackResult struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	ExpireTime string `json:"expireTime"`
}

type DisableLiveAddressRequest struct {
	DeviceSerial string
	ChannelNo    int
	URLID        string
}

func (c *Client) PlaybackAddress(ctx context.Context, account Account, input PlaybackRequest) (PlaybackResult, error) {
	if strings.TrimSpace(input.DeviceSerial) == "" {
		return PlaybackResult{}, errors.New("deviceSerial is required for playback")
	}
	if input.ChannelNo <= 0 {
		return PlaybackResult{}, errors.New("channelNo must be > 0 for playback")
	}
	if !input.StopTime.After(input.StartTime) {
		return PlaybackResult{}, errors.New("stopTime must be after startTime for playback")
	}

	token, err := c.accessToken(ctx, account)
	if err != nil {
		return PlaybackResult{}, err
	}

	values := playbackValues(token, input)
	var response apiResponse[PlaybackResult]
	if err := c.postForm(ctx, "/api/lapp/v2/live/address/get", values, &response); err != nil {
		return PlaybackResult{}, err
	}
	if isExpiredTokenCode(response.Code) {
		token, err = c.refreshToken(ctx, account)
		if err != nil {
			return PlaybackResult{}, err
		}
		values = playbackValues(token, input)
		if err := c.postForm(ctx, "/api/lapp/v2/live/address/get", values, &response); err != nil {
			return PlaybackResult{}, err
		}
	}
	if response.Code != "200" {
		return PlaybackResult{}, &Error{Code: response.Code, Msg: redact(response.Msg, account)}
	}
	if strings.TrimSpace(response.Data.URL) == "" {
		return PlaybackResult{}, errors.New("ezviz playback response missing url")
	}
	return response.Data, nil
}

func (c *Client) DisableLiveAddress(ctx context.Context, account Account, input DisableLiveAddressRequest) error {
	deviceSerial := strings.ToUpper(strings.TrimSpace(input.DeviceSerial))
	urlID := strings.TrimSpace(input.URLID)
	if deviceSerial == "" {
		return errors.New("deviceSerial is required to disable address")
	}
	if input.ChannelNo <= 0 {
		return errors.New("channelNo must be > 0 to disable address")
	}
	if urlID == "" {
		return errors.New("urlID is required to disable address")
	}
	token, err := c.accessToken(ctx, account)
	if err != nil {
		return err
	}

	values := url.Values{}
	values.Set("accessToken", token)
	values.Set("deviceSerial", deviceSerial)
	values.Set("channelNo", strconv.Itoa(input.ChannelNo))
	values.Set("urlId", urlID)
	var response apiResponse[any]
	if err := c.postForm(ctx, "/api/lapp/v2/live/address/disable", values, &response); err != nil {
		return err
	}
	if isExpiredTokenCode(response.Code) {
		token, err = c.refreshToken(ctx, account)
		if err != nil {
			return err
		}
		values.Set("accessToken", token)
		if err := c.postForm(ctx, "/api/lapp/v2/live/address/disable", values, &response); err != nil {
			return err
		}
	}
	if response.Code != "200" {
		return &Error{Code: response.Code, Msg: redact(response.Msg, account)}
	}
	return nil
}

func playbackValues(token string, input PlaybackRequest) url.Values {
	protocol := input.Protocol
	if protocol == 0 {
		protocol = 4
	}
	quality := input.Quality
	if quality == 0 {
		quality = 2
	}
	expireTime := input.ExpireTime
	if expireTime == 0 {
		expireTime = 600
	}

	values := url.Values{}
	values.Set("accessToken", token)
	values.Set("deviceSerial", strings.ToUpper(strings.TrimSpace(input.DeviceSerial)))
	values.Set("channelNo", strconv.Itoa(input.ChannelNo))
	values.Set("protocol", strconv.Itoa(protocol))
	values.Set("type", "2")
	values.Set("quality", strconv.Itoa(quality))
	values.Set("expireTime", strconv.Itoa(expireTime))
	values.Set("supportH265", "1")
	values.Set("mute", "0")
	values.Set("startTime", strconv.FormatInt(input.StartTime.Unix(), 10))
	values.Set("stopTime", strconv.FormatInt(input.StopTime.Unix(), 10))
	if code := strings.TrimSpace(input.Code); code != "" {
		values.Set("code", code)
	}
	return values
}
