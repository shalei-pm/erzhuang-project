package ezviz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type metaResponse[T any] struct {
	Meta struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"meta"`
	Data T `json:"data"`
}

func (c *Client) EnsureAACTransfer(ctx context.Context, account Account, deviceSerial string, channelNo int) error {
	serial := strings.ToUpper(strings.TrimSpace(deviceSerial))
	if serial == "" {
		return errors.New("deviceSerial is required for AAC transfer")
	}
	if channelNo <= 0 {
		return errors.New("channelNo must be > 0 for AAC transfer")
	}

	token, err := c.accessToken(ctx, account)
	if err != nil {
		return err
	}
	if err := c.callAACTransfer(ctx, account, token, serial, channelNo); err != nil {
		if !isTokenError(err) {
			return err
		}
		token, err = c.refreshToken(ctx, account)
		if err != nil {
			return err
		}
		return c.callAACTransfer(ctx, account, token, serial, channelNo)
	}
	return nil
}

func (c *Client) callAACTransfer(ctx context.Context, account Account, token string, deviceSerial string, channelNo int) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/service/media/aac/transfer?enable=1", nil)
	if err != nil {
		return err
	}
	request.Header.Set("accessToken", token)
	request.Header.Set("deviceSerial", deviceSerial)
	request.Header.Set("localIndex", strconv.Itoa(channelNo))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("ezviz aac transfer http status=%d body=%s", response.StatusCode, string(bytes.TrimSpace(payload)))
	}
	var result metaResponse[any]
	if err := json.Unmarshal(payload, &result); err != nil {
		return fmt.Errorf("decode ezviz aac transfer response: %w", err)
	}
	if result.Meta.Code != 200 {
		return &Error{Code: strconv.Itoa(result.Meta.Code), Msg: redact(result.Meta.Message, account)}
	}
	return nil
}

func isTokenError(err error) bool {
	var apiError *Error
	if errors.As(err, &apiError) {
		return isExpiredTokenCode(apiError.Code)
	}
	return false
}
