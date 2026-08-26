package nvrmonitor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAuthorizationEndpoint = "https://sec.sy.soyoung.com/api/auth/camera"
	defaultWebSocketEndpoint     = "wss://prime-crm.soyoung.com/nvrapi/ws"
)

type HTTPAuthorizationClient struct {
	client                *http.Client
	authorizationEndpoint string
	webSocketEndpoint     string
	authorization         string
}

func NewHTTPAuthorizationClient(client *http.Client, authorization string) *HTTPAuthorizationClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &HTTPAuthorizationClient{
		client:                client,
		authorizationEndpoint: defaultAuthorizationEndpoint,
		webSocketEndpoint:     defaultWebSocketEndpoint,
		authorization:         strings.TrimSpace(authorization),
	}
}

func (c *HTTPAuthorizationClient) CreateStreamURL(ctx context.Context, cameraID int64, request StreamSessionRequest) (string, error) {
	if c == nil || strings.TrimSpace(c.authorization) == "" {
		return "", ErrNotConfigured
	}
	endpoint, err := url.Parse(c.authorizationEndpoint)
	if err != nil {
		return "", ErrAuthorizationFailed
	}
	query := endpoint.Query()
	query.Set("camera_id", strconv.FormatInt(cameraID, 10))
	query.Set("stream_type", "2")
	if request.Mode == ModePlayback {
		query.Set("start_time", strconv.FormatInt(request.StartTime, 10))
		query.Set("end_time", strconv.FormatInt(request.EndTime, 10))
	}
	endpoint.RawQuery = query.Encode()

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", ErrAuthorizationFailed
	}
	httpRequest.Header.Set("Authorization", c.authorization)
	response, err := c.client.Do(httpRequest)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", ErrAuthorizationTimeout
		}
		return "", ErrAuthorizationFailed
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", ErrAuthorizationFailed
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&payload); err != nil {
		return "", ErrAuthorizationFailed
	}
	if payload.Code != 0 || strings.TrimSpace(payload.Data.Token) == "" {
		return "", ErrAuthorizationFailed
	}
	webSocketEndpoint, err := url.Parse(c.webSocketEndpoint)
	if err != nil {
		return "", ErrAuthorizationFailed
	}
	websocketQuery := webSocketEndpoint.Query()
	websocketQuery.Set("token", payload.Data.Token)
	webSocketEndpoint.RawQuery = websocketQuery.Encode()
	return webSocketEndpoint.String(), nil
}
