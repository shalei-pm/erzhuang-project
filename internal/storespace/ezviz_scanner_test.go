package storespace

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
)

func TestEzvizScannerFallsBackToCaptureProbeWhenCameraListHitsPlanLimit(t *testing.T) {
	var capturedChannels []string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		switch r.URL.Path {
		case "/api/lapp/device/camera/list":
			return jsonResponse(`{"code":"10026","msg":"设备数量超出个人版限制，当前设备无法操作"}`), nil
		case "/api/lapp/device/capture":
			if r.Form.Get("deviceSerial") != "GF8132547" {
				t.Fatalf("unexpected deviceSerial %q", r.Form.Get("deviceSerial"))
			}
			channelNo := r.Form.Get("channelNo")
			capturedChannels = append(capturedChannels, channelNo)
			if channelNo == "1" || channelNo == "2" {
				return jsonResponse(`{"code":"200","msg":"操作成功!","data":{"picUrl":"https://example.test/snapshot.jpg"}}`), nil
			}
			return jsonResponse(`{"code":"60012","msg":"未知错误"}`), nil
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

	channels, err := scanner.ScanRecorderChannels(context.Background(), EzvizAccount{AccountName: "华东"}, Recorder{DeviceCode: "GF8132547"})
	if err != nil {
		t.Fatalf("scan recorder channels: %v", err)
	}

	if strings.Join(capturedChannels, ",") != "1,2,3,4,5,6,7" {
		t.Fatalf("expected capture probing to stop after five consecutive failures, got %#v", capturedChannels)
	}
	if len(channels) != 2 {
		t.Fatalf("expected two probed active channels, got %#v", channels)
	}
	for index, channel := range channels {
		expectedNo := index + 1
		if channel.ChannelNo != expectedNo || !channel.Active || channel.ChannelName != "通道"+strconv.Itoa(expectedNo) {
			t.Fatalf("unexpected probed channel at index %d: %#v", index, channel)
		}
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
