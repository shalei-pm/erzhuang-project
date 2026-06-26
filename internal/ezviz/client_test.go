package ezviz

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestClientRefreshesTokenAfterExpiredTokenResponse(t *testing.T) {
	var tokenRequests int
	var cameraRequests int

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		var body string
		switch r.URL.Path {
		case "/api/lapp/device/camera/list":
			cameraRequests++
			if r.Form.Get("accessToken") == "expired-token" {
				body = `{"code":"10002","msg":"access token expired"}`
				return jsonResponse(body), nil
			}
			if r.Form.Get("accessToken") != "fresh-token" {
				t.Fatalf("unexpected accessToken %q", r.Form.Get("accessToken"))
			}
			if r.Form.Get("deviceSerial") != "GN0941203" {
				t.Fatalf("unexpected deviceSerial %q", r.Form.Get("deviceSerial"))
			}
			body = `{"code":"200","msg":"操作成功!","data":[{"deviceSerial":"GN0941203","channelNo":1,"cameraName":"通道1","status":1}]}`
		case "/api/lapp/token/get":
			tokenRequests++
			if r.Form.Get("appKey") != "app-key" || r.Form.Get("appSecret") != "app-secret" {
				t.Fatalf("unexpected credentials")
			}
			body = `{"code":"200","msg":"操作成功!","data":{"accessToken":"fresh-token","expireTime":1999999999000}}`
		default:
			return jsonResponse(`{"code":"404","msg":"not found"}`), nil
		}
		return jsonResponse(body), nil
	})

	client := NewClient(ClientOptions{BaseURL: "https://ezviz.test", HTTPClient: &http.Client{Transport: transport}})
	cameras, err := client.CameraList(context.Background(), Account{
		Name:        "华北",
		AppKey:      "app-key",
		AppSecret:   "app-secret",
		AccessToken: "expired-token",
	}, "gn0941203")
	if err != nil {
		t.Fatalf("camera list: %v", err)
	}

	if tokenRequests != 1 {
		t.Fatalf("expected one token refresh, got %d", tokenRequests)
	}
	if cameraRequests != 2 {
		t.Fatalf("expected camera list retry, got %d", cameraRequests)
	}
	if len(cameras) != 1 || cameras[0].ChannelNo != 1 || cameras[0].CameraName != "通道1" || cameras[0].Status != 1 {
		t.Fatalf("unexpected cameras: %#v", cameras)
	}
}

func TestClientDoesNotExposeSecretsInErrors(t *testing.T) {
	client := NewClient(ClientOptions{
		BaseURL: "https://ezviz.test",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(`{"code":"10030","msg":"appKey and appSecret mismatch"}`), nil
		})},
	})
	account := Account{
		Name:        "华北",
		AppKey:      "secret-app-key",
		AppSecret:   "secret-app-secret",
		AccessToken: "secret-access-token",
	}
	_, err := client.CameraList(context.Background(), account, "GN0941203")
	if err == nil {
		t.Fatal("expected error")
	}

	message := err.Error()
	for _, secret := range []string{account.AppKey, account.AppSecret, account.AccessToken} {
		if strings.Contains(message, secret) {
			t.Fatalf("error leaked secret %q in %q", secret, message)
		}
	}
	if !strings.Contains(message, "10030") {
		t.Fatalf("expected original code in error, got %q", message)
	}
}

func TestClientLiveAddressRequestsHLSAndRefreshesToken(t *testing.T) {
	var tokenRequests int
	var liveAddressRequests int

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		switch r.URL.Path {
		case "/api/lapp/v2/live/address/get":
			liveAddressRequests++
			if r.Form.Get("accessToken") == "expired-token" {
				return jsonResponse(`{"code":"10002","msg":"access token expired"}`), nil
			}
			if r.Form.Get("accessToken") != "fresh-token" {
				t.Fatalf("unexpected accessToken %q", r.Form.Get("accessToken"))
			}
			if r.Form.Get("deviceSerial") != "GN0941203" {
				t.Fatalf("unexpected deviceSerial %q", r.Form.Get("deviceSerial"))
			}
			if r.Form.Get("channelNo") != "2" {
				t.Fatalf("unexpected channelNo %q", r.Form.Get("channelNo"))
			}
			if r.Form.Get("protocol") != "2" {
				t.Fatalf("unexpected protocol %q", r.Form.Get("protocol"))
			}
			if r.Form.Get("type") != "1" {
				t.Fatalf("unexpected type %q", r.Form.Get("type"))
			}
			if r.Form.Get("quality") != "2" {
				t.Fatalf("unexpected quality %q", r.Form.Get("quality"))
			}
			if r.Form.Get("expireTime") != "600" {
				t.Fatalf("unexpected expireTime %q", r.Form.Get("expireTime"))
			}
			if r.Form.Get("supportH265") != "0" {
				t.Fatalf("unexpected supportH265 %q", r.Form.Get("supportH265"))
			}
			if r.Form.Get("code") != "verify-code" {
				t.Fatalf("unexpected code %q", r.Form.Get("code"))
			}
			return jsonResponse(`{"code":"200","msg":"操作成功","data":{"id":"url-id-1","url":"https://open.ys7.com/v3/openlive/GN0941203_2_1.m3u8","expireTime":"2026-06-24 12:00:00"}}`), nil
		case "/api/lapp/token/get":
			tokenRequests++
			return jsonResponse(`{"code":"200","msg":"操作成功!","data":{"accessToken":"fresh-token","expireTime":1999999999000}}`), nil
		default:
			return jsonResponse(`{"code":"404","msg":"not found"}`), nil
		}
	})

	client := NewClient(ClientOptions{BaseURL: "https://ezviz.test", HTTPClient: &http.Client{Transport: transport}})
	result, err := client.LiveAddress(context.Background(), Account{
		Name:        "华北",
		AppKey:      "app-key",
		AppSecret:   "app-secret",
		AccessToken: "expired-token",
	}, LiveAddressRequest{
		DeviceSerial: "gn0941203",
		ChannelNo:    2,
		Protocol:     2,
		Quality:      2,
		ExpireTime:   600,
		Code:         "verify-code",
	})
	if err != nil {
		t.Fatalf("live address: %v", err)
	}

	if tokenRequests != 1 {
		t.Fatalf("expected one token refresh, got %d", tokenRequests)
	}
	if liveAddressRequests != 2 {
		t.Fatalf("expected two live address requests, got %d", liveAddressRequests)
	}
	if result.ID != "url-id-1" || result.URL == "" || result.ExpireTime != "2026-06-24 12:00:00" {
		t.Fatalf("unexpected live address result: %#v", result)
	}
}

func TestParseAccountsMarkdownSelectsNorthChina(t *testing.T) {
	source := []byte(`
| 大区 | 账户名 | appKey | appSecret | accessToken | 测试录像机设备编码 |
|---|---|---|---|---|---|
| 华北 | north-account | north-key | north-secret | north-token | GN0941203 |
| 华南 | south-account | south-key | south-secret | south-token | GQ2603603 |
`)

	accounts, err := ParseAccountsMarkdown(source)
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	selected, ok := FindAccountByRegion(accounts, "华北")
	if !ok {
		t.Fatal("expected north china account")
	}

	if selected.Region != "华北" ||
		selected.Account.Name != "north-account" ||
		selected.Account.AppKey != "north-key" ||
		selected.Account.AppSecret != "north-secret" ||
		selected.Account.AccessToken != "north-token" ||
		selected.DeviceSerial != "GN0941203" {
		t.Fatalf("unexpected selected account: %#v", selected)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
