package storespace

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
)

func TestEzvizScannerReturnsPlanLimitWithoutCaptureProbe(t *testing.T) {
	var captureRequests int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		switch r.URL.Path {
		case "/api/lapp/device/camera/list":
			return jsonResponse(`{"code":"10026","msg":"设备数量超出个人版限制，当前设备无法操作"}`), nil
		case "/api/lapp/device/capture":
			captureRequests++
			return jsonResponse(`{"code":"200","msg":"操作成功!","data":{"picUrl":"https://example.test/snapshot.jpg"}}`), nil
		default:
			return jsonResponse(`{"code":"404","msg":"not found"}`), nil
		}
	})
	client := ezviz.NewClient(ezviz.ClientOptions{BaseURL: "https://ezviz.test", HTTPClient: &http.Client{Transport: transport}})
	scanner := NewEzvizScanner(client, []ezviz.Account{{
		Name:        "华东",
		AppKey:      "app-key",
		AppSecret:   "app-secret",
		AccessToken: "token",
	}})

	_, err := scanner.ScanRecorderChannels(context.Background(), EzvizAccount{AccountName: "华东"}, Recorder{DeviceCode: "GF8132547"})
	if err == nil {
		t.Fatal("expected camera list plan limit error")
	}
	if code := ezviz.ErrorCode(err); code != "10026" {
		t.Fatalf("expected ezviz error code 10026, got %q: %v", code, err)
	}
	if captureRequests != 0 {
		t.Fatalf("expected no synchronous capture probing on plan limit, got %d requests", captureRequests)
	}
}

func TestEzvizScannerDoesNotFallbackForUnauthorizedDevice(t *testing.T) {
	var captureRequests int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		switch r.URL.Path {
		case "/api/lapp/device/camera/list":
			return jsonResponse(`{"code":"20018","msg":"该用户不拥有该设备"}`), nil
		case "/api/lapp/device/capture":
			captureRequests++
			return jsonResponse(`{"code":"200","msg":"操作成功!","data":{"picUrl":"https://example.test/snapshot.jpg"}}`), nil
		default:
			return jsonResponse(`{"code":"404","msg":"not found"}`), nil
		}
	})
	client := ezviz.NewClient(ezviz.ClientOptions{BaseURL: "https://ezviz.test", HTTPClient: &http.Client{Transport: transport}})
	scanner := NewEzvizScanner(client, []ezviz.Account{{
		Name:        "华东",
		AppKey:      "app-key",
		AppSecret:   "app-secret",
		AccessToken: "token",
	}})

	_, err := scanner.ScanRecorderChannels(context.Background(), EzvizAccount{AccountName: "华东"}, Recorder{DeviceCode: "GF8132547"})
	if err == nil {
		t.Fatal("expected original camera list error")
	}
	if captureRequests != 0 {
		t.Fatalf("expected no capture fallback for unauthorized device, got %d requests", captureRequests)
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
