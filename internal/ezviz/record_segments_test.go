package ezviz

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestQueryRecordSegmentsSendsHeadersQueryAndRefreshesToken(t *testing.T) {
	var tokenRequests int
	var queryRequests int

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v3/device/local/video/unify/query":
			queryRequests++
			if r.Header.Get("accessToken") == "stale-token" {
				return jsonResponse(`{"meta":{"code":10014,"message":"token expired"},"data":null}`), nil
			}
			if r.Header.Get("accessToken") != "fresh-token" {
				t.Fatalf("unexpected accessToken header %q", r.Header.Get("accessToken"))
			}
			if r.Header.Get("deviceSerial") != "AZ3988334" {
				t.Fatalf("unexpected deviceSerial header %q", r.Header.Get("deviceSerial"))
			}
			if r.Header.Get("localIndex") != "2" {
				t.Fatalf("unexpected localIndex header %q", r.Header.Get("localIndex"))
			}
			if r.URL.Query().Get("pageSize") != "500" {
				t.Fatalf("unexpected pageSize %q", r.URL.Query().Get("pageSize"))
			}
			if r.URL.Query().Get("startTime") == "" || r.URL.Query().Get("endTime") == "" {
				t.Fatalf("missing startTime/endTime in query: %s", r.URL.RawQuery)
			}
			return jsonResponse(`{"meta":{"code":"200","message":"ok"},"data":{"records":[{"startTime":1731945592,"endTime":1731949200,"type":"PLAN","size":"100MB"}],"fromNvr":true,"deviceSerial":"AZ3988334","localIndex":2,"hasMore":false}}`), nil
		case "/api/lapp/token/get":
			tokenRequests++
			return jsonResponse(`{"code":"200","msg":"ok","data":{"accessToken":"fresh-token","expireTime":9999999999999}}`), nil
		default:
			return jsonResponse(`{"code":"404","msg":"not found"}`), nil
		}
	})

	client := NewClient(ClientOptions{BaseURL: "https://ezviz.test", HTTPClient: &http.Client{Transport: transport}})
	client.mu.Lock()
	client.tokens["test"] = tokenCache{accessToken: "stale-token", expiresAt: farFuture()}
	client.mu.Unlock()

	result, err := client.QueryRecordSegments(context.Background(), Account{Name: "test", AppKey: "k", AppSecret: "s"}, RecordSegmentsQuery{
		DeviceSerial: "az3988334",
		ChannelNo:    2,
		Date:         time.Date(2024, 11, 19, 10, 0, 0, 0, time.UTC),
		PageSize:     600,
	})
	if err != nil {
		t.Fatalf("query record segments: %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("expected one token refresh, got %d", tokenRequests)
	}
	if queryRequests != 2 {
		t.Fatalf("expected query retry, got %d", queryRequests)
	}
	if len(result.Records) != 1 || result.Records[0].Type != "PLAN" || !result.FromNvr {
		t.Fatalf("unexpected result: %#v", result)
	}
	if string(result.LocalIndex) != "2" {
		t.Fatalf("unexpected localIndex %q", result.LocalIndex)
	}
}

func TestQueryRecordSegmentsUsesShanghaiDayRange(t *testing.T) {
	var capturedStartTime string
	var capturedEndTime string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		capturedStartTime = r.URL.Query().Get("startTime")
		capturedEndTime = r.URL.Query().Get("endTime")
		return jsonResponse(`{"meta":{"code":200,"message":"ok"},"data":{"records":[],"hasMore":false}}`), nil
	})

	client := NewClient(ClientOptions{BaseURL: "https://ezviz.test", HTTPClient: &http.Client{Transport: transport}})
	client.mu.Lock()
	client.tokens["test"] = tokenCache{accessToken: "tok", expiresAt: farFuture()}
	client.mu.Unlock()

	_, err := client.QueryRecordSegments(context.Background(), Account{Name: "test", AppKey: "k", AppSecret: "s"}, RecordSegmentsQuery{
		DeviceSerial: "AZ3988334",
		ChannelNo:    2,
		Date:         time.Date(2024, 11, 19, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("query record segments: %v", err)
	}
	if capturedStartTime != "1731945600" || capturedEndTime != "1732031999" {
		t.Fatalf("unexpected Shanghai day range %s-%s", capturedStartTime, capturedEndTime)
	}
}

func TestQueryRecordSegmentsFollowsNextFileTimePages(t *testing.T) {
	var starts []string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		starts = append(starts, r.URL.Query().Get("startTime"))
		if len(starts) == 1 {
			return jsonResponse(`{"meta":{"code":200,"message":"ok"},"data":{"records":[{"startTime":1731945600,"endTime":1731949200,"type":"PLAN"}],"hasMore":true,"nextFileTime":1731949201}}`), nil
		}
		return jsonResponse(`{"meta":{"code":200,"message":"ok"},"data":{"records":[{"startTime":1731949201,"endTime":1731952800,"type":"ALARM"}],"hasMore":false}}`), nil
	})

	client := NewClient(ClientOptions{BaseURL: "https://ezviz.test", HTTPClient: &http.Client{Transport: transport}})
	client.mu.Lock()
	client.tokens["test"] = tokenCache{accessToken: "tok", expiresAt: farFuture()}
	client.mu.Unlock()

	result, err := client.QueryRecordSegments(context.Background(), Account{Name: "test", AppKey: "k", AppSecret: "s"}, RecordSegmentsQuery{
		DeviceSerial: "AZ3988334",
		ChannelNo:    2,
		Date:         time.Date(2024, 11, 19, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("query record segments: %v", err)
	}
	if strings.Join(starts, ",") != "1731945600,1731949201" {
		t.Fatalf("unexpected page starts %v", starts)
	}
	if len(result.Records) != 2 || result.Records[0].Type != "PLAN" || result.Records[1].Type != "ALARM" {
		t.Fatalf("unexpected merged records: %#v", result.Records)
	}
}
