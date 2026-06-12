package ezviz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://open.ys7.com"

type Account struct {
	Name        string
	AppKey      string
	AppSecret   string
	AccessToken string
}

type AccountWithDevice struct {
	Region       string
	Account      Account
	DeviceSerial string
}

type ClientOptions struct {
	BaseURL    string
	HTTPClient *http.Client
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.Mutex
	tokens     map[string]tokenCache
}

type tokenCache struct {
	accessToken string
	expiresAt   time.Time
}

type Camera struct {
	DeviceSerial string `json:"deviceSerial"`
	ChannelNo    int    `json:"channelNo"`
	CameraName   string `json:"cameraName"`
	Status       int    `json:"status"`
}

type CaptureResult struct {
	PicURL string `json:"picUrl"`
}

type apiResponse[T any] struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

type tokenData struct {
	AccessToken string `json:"accessToken"`
	ExpireTime  int64  `json:"expireTime"`
}

type Error struct {
	Code string
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("ezviz api error code=%s msg=%s", e.Code, e.Msg)
}

func NewClient(options ClientOptions) *Client {
	baseURL := strings.TrimRight(options.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		tokens:     map[string]tokenCache{},
	}
}

func (c *Client) CameraList(ctx context.Context, account Account, deviceSerial string) ([]Camera, error) {
	token, err := c.accessToken(ctx, account)
	if err != nil {
		return nil, err
	}

	values := url.Values{}
	values.Set("accessToken", token)
	values.Set("deviceSerial", strings.ToUpper(strings.TrimSpace(deviceSerial)))

	var response apiResponse[[]Camera]
	if err := c.postForm(ctx, "/api/lapp/device/camera/list", values, &response); err != nil {
		return nil, err
	}
	if isExpiredTokenCode(response.Code) {
		token, err = c.refreshToken(ctx, account)
		if err != nil {
			return nil, err
		}
		values.Set("accessToken", token)
		if err := c.postForm(ctx, "/api/lapp/device/camera/list", values, &response); err != nil {
			return nil, err
		}
	}
	if response.Code != "200" {
		return nil, &Error{Code: response.Code, Msg: redact(response.Msg, account)}
	}
	return response.Data, nil
}

func (c *Client) Capture(ctx context.Context, account Account, deviceSerial string, channelNo int) (CaptureResult, error) {
	token, err := c.accessToken(ctx, account)
	if err != nil {
		return CaptureResult{}, err
	}

	values := url.Values{}
	values.Set("accessToken", token)
	values.Set("deviceSerial", strings.ToUpper(strings.TrimSpace(deviceSerial)))
	values.Set("channelNo", strconv.Itoa(channelNo))

	var response apiResponse[CaptureResult]
	if err := c.postForm(ctx, "/api/lapp/device/capture", values, &response); err != nil {
		return CaptureResult{}, err
	}
	if isExpiredTokenCode(response.Code) {
		token, err = c.refreshToken(ctx, account)
		if err != nil {
			return CaptureResult{}, err
		}
		values.Set("accessToken", token)
		if err := c.postForm(ctx, "/api/lapp/device/capture", values, &response); err != nil {
			return CaptureResult{}, err
		}
	}
	if response.Code != "200" {
		return CaptureResult{}, &Error{Code: response.Code, Msg: redact(response.Msg, account)}
	}
	return response.Data, nil
}

func (c *Client) accessToken(ctx context.Context, account Account) (string, error) {
	accountName := strings.TrimSpace(account.Name)
	if accountName == "" {
		accountName = strings.TrimSpace(account.AppKey)
	}
	now := time.Now()

	c.mu.Lock()
	cached := c.tokens[accountName]
	if cached.accessToken != "" && now.Before(cached.expiresAt.Add(-time.Hour)) {
		c.mu.Unlock()
		return cached.accessToken, nil
	}
	c.mu.Unlock()

	if strings.TrimSpace(account.AccessToken) != "" {
		return strings.TrimSpace(account.AccessToken), nil
	}
	return c.refreshToken(ctx, account)
}

func (c *Client) refreshToken(ctx context.Context, account Account) (string, error) {
	values := url.Values{}
	values.Set("appKey", strings.TrimSpace(account.AppKey))
	values.Set("appSecret", strings.TrimSpace(account.AppSecret))

	var response apiResponse[tokenData]
	if err := c.postForm(ctx, "/api/lapp/token/get", values, &response); err != nil {
		return "", err
	}
	if response.Code != "200" {
		return "", &Error{Code: response.Code, Msg: redact(response.Msg, account)}
	}
	if response.Data.AccessToken == "" {
		return "", errors.New("ezviz token response missing accessToken")
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if response.Data.ExpireTime > 0 {
		expiresAt = time.UnixMilli(response.Data.ExpireTime)
	}
	accountName := strings.TrimSpace(account.Name)
	if accountName == "" {
		accountName = strings.TrimSpace(account.AppKey)
	}

	c.mu.Lock()
	c.tokens[accountName] = tokenCache{accessToken: response.Data.AccessToken, expiresAt: expiresAt}
	c.mu.Unlock()

	return response.Data.AccessToken, nil
}

func (c *Client) postForm(ctx context.Context, path string, values url.Values, target any) error {
	body := values.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("ezviz http status=%d body=%s", response.StatusCode, string(bytes.TrimSpace(payload)))
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode ezviz response: %w", err)
	}
	return nil
}

func ParseAccountsMarkdown(source []byte) ([]AccountWithDevice, error) {
	lines := strings.Split(string(source), "\n")
	accounts := []AccountWithDevice{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") || strings.Contains(line, "大区") {
			continue
		}
		parts := strings.Split(line, "|")
		cells := []string{}
		for _, part := range parts {
			cell := strings.TrimSpace(part)
			if cell != "" {
				cells = append(cells, cell)
			}
		}
		if len(cells) < 6 {
			continue
		}
		accounts = append(accounts, AccountWithDevice{
			Region: cells[0],
			Account: Account{
				Name:        cells[1],
				AppKey:      cells[2],
				AppSecret:   cells[3],
				AccessToken: cells[4],
			},
			DeviceSerial: strings.ToUpper(cells[5]),
		})
	}
	if len(accounts) == 0 {
		return nil, errors.New("no ezviz accounts found in markdown")
	}
	return accounts, nil
}

func FindAccountByRegion(accounts []AccountWithDevice, region string) (AccountWithDevice, bool) {
	cleanRegion := strings.TrimSpace(region)
	for _, account := range accounts {
		if account.Region == cleanRegion {
			return account, true
		}
	}
	return AccountWithDevice{}, false
}

func isExpiredTokenCode(code string) bool {
	return code == "10002" || code == "10014"
}

func redact(value string, account Account) string {
	redacted := value
	for _, secret := range []string{account.AppKey, account.AppSecret, account.AccessToken} {
		secret = strings.TrimSpace(secret)
		if secret != "" {
			redacted = strings.ReplaceAll(redacted, secret, "***REDACTED***")
		}
	}
	return redacted
}
