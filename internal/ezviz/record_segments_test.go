package ezviz

import (
	"context"
	"net/http"
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
			return jsonResponse(`{"meta":{"code":200,"message":"ok"},"data":{"records":[{"startTime":1731945592,"endTime":1731949200,"type":"PLAN","size":"100MB"}],"fromNvr":true,"deviceSerial":"AZ3988334","localIndex":2,"hasMore":false}}`), nil
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
