package ezviz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type RecordSegment struct {
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Type      string `json:"type"`
	Size      string `json:"size"`
}

type RecordSegmentsResult struct {
	Records      []RecordSegment `json:"records"`
	FromNvr      bool            `json:"fromNvr"`
	DeviceSerial string          `json:"deviceSerial"`
	LocalIndex   FlexibleString  `json:"localIndex"`
	HasMore      bool            `json:"hasMore"`
	NextFileTime int64           `json:"nextFileTime"`
}

type FlexibleString string

func (s *FlexibleString) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*s = FlexibleString(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		*s = FlexibleString(number.String())
		return nil
	}
	return fmt.Errorf("decode flexible string: %s", string(data))
}

type RecordSegmentsQuery struct {
	DeviceSerial string
	ChannelNo    int
	Date         time.Time
	PageSize     int
}

func (c *Client) QueryRecordSegments(ctx context.Context, account Account, input RecordSegmentsQuery) (RecordSegmentsResult, error) {
	serial := strings.ToUpper(strings.TrimSpace(input.DeviceSerial))
	if serial == "" {
		return RecordSegmentsResult{}, errors.New("deviceSerial is required for record segments query")
	}
	if input.ChannelNo <= 0 {
		return RecordSegmentsResult{}, errors.New("channelNo must be > 0 for record segments query")
	}

	startTime, endTime := dayRange(input.Date)
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 500 {
		pageSize = 500
	}

	token, err := c.accessToken(ctx, account)
	if err != nil {
		return RecordSegmentsResult{}, err
	}
	result, err := c.callRecordSegments(ctx, account, token, serial, input.ChannelNo, startTime, endTime, pageSize)
	if err != nil {
		if !isTokenError(err) {
			return RecordSegmentsResult{}, err
		}
		token, err = c.refreshToken(ctx, account)
		if err != nil {
			return RecordSegmentsResult{}, err
		}
		return c.callRecordSegments(ctx, account, token, serial, input.ChannelNo, startTime, endTime, pageSize)
	}
	return result, nil
}

func (c *Client) callRecordSegments(ctx context.Context, account Account, token string, deviceSerial string, channelNo int, startTime int64, endTime int64, pageSize int) (RecordSegmentsResult, error) {
	query := url.Values{}
	query.Set("startTime", strconv.FormatInt(startTime, 10))
	query.Set("endTime", strconv.FormatInt(endTime, 10))
	query.Set("pageSize", strconv.Itoa(pageSize))

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v3/device/local/video/unify/query?"+query.Encode(), nil)
	if err != nil {
		return RecordSegmentsResult{}, err
	}
	request.Header.Set("accessToken", token)
	request.Header.Set("deviceSerial", deviceSerial)
	request.Header.Set("localIndex", strconv.Itoa(channelNo))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return RecordSegmentsResult{}, err
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if err != nil {
		return RecordSegmentsResult{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return RecordSegmentsResult{}, fmt.Errorf("ezviz record segments http status=%d body=%s", response.StatusCode, string(payload))
	}

	var result metaResponse[RecordSegmentsResult]
	if err := json.Unmarshal(payload, &result); err != nil {
		return RecordSegmentsResult{}, fmt.Errorf("decode ezviz record segments response: %w", err)
	}
	if result.Meta.Code != 200 {
		return RecordSegmentsResult{}, &Error{Code: strconv.Itoa(result.Meta.Code), Msg: redact(result.Meta.Message, account)}
	}
	return result.Data, nil
}

func dayRange(date time.Time) (int64, int64) {
	if date.IsZero() {
		date = time.Now()
	}
	year, month, day := date.Date()
	start := time.Date(year, month, day, 0, 0, 0, 0, date.Location())
	end := start.Add(24*time.Hour - time.Second)
	return start.Unix(), end.Unix()
}
