package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/nvrlab"
	"github.com/shalei-pm/erzhuang-project/internal/nvrsnapshot"
)

func TestRunRejectsMissingOrInvalidCameraID(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing", args: nil},
		{name: "zero", args: []string{"--camera-id=0"}},
		{name: "negative", args: []string{"--camera-id=-1"}},
		{name: "not an integer", args: []string{"--camera-id=service-secret"}},
		{name: "repeated", args: []string{"--camera-id=12", "--camera-id=13"}},
		{name: "mixed repeated spelling", args: []string{"--camera-id=12", "-camera-id=13"}},
		{name: "extra argument", args: []string{"--camera-id=12", "another-camera"}},
		{name: "non-positive timeout", args: []string{"--camera-id=12", "--timeout=0s"}},
		{name: "timeout exceeds capture budget", args: []string{"--camera-id=12", "--timeout=21s"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &strings.Builder{}, &strings.Builder{}
			status := run(test.args, func(string) string { return "" }, stdout, stderr, Dependencies{})
			if status == 0 {
				t.Fatal("run() status = 0, want non-zero")
			}
			if got, want := stdout.String(), "camera_id=0 error_code=invalid_arguments\n"; got != want {
				t.Fatalf("stdout = %q, want %q", got, want)
			}
			if got := stderr.String(); got != "" {
				t.Fatalf("stderr = %q, want empty", got)
			}
		})
	}
}

func TestRunRejectsMissingAuthorizationWithoutCreatingCaptureService(t *testing.T) {
	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	called := false
	status := run([]string{"--camera-id=42"}, func(string) string { return "" }, stdout, stderr, Dependencies{
		NewCaptureService: func(string) captureService {
			called = true
			return &fakeCapture{}
		},
	})

	if status == 0 {
		t.Fatal("run() status = 0, want non-zero")
	}
	if called {
		t.Fatal("NewCaptureService() was called without authorization")
	}
	if got, want := stdout.String(), "camera_id=42 error_code=authorization_failed\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestRunUsesFallbackAuthorizationAndRedactsSuccessfulOutput(t *testing.T) {
	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	var authorization string
	capture := &fakeCapture{jpeg: nvrsnapshot.JPEG{
		Bytes:       []byte("raw-jpeg-bytes-are-never-output"),
		ContentType: "image/jpeg",
		Width:       640,
		Height:      360,
	}}
	status := run([]string{"--camera-id=42"}, func(name string) string {
		if name == nvrStreamAuthorizationEnv {
			return "fallback-service-secret"
		}
		return ""
	}, stdout, stderr, Dependencies{
		NewCaptureService: func(value string) captureService {
			authorization = value
			return capture
		},
	})

	if status != 0 {
		t.Fatalf("run() status = %d, want 0", status)
	}
	if authorization != "fallback-service-secret" {
		t.Fatalf("authorization = %q, want fallback value", authorization)
	}
	if got, want := stdout.String(), "camera_id=42 content_type=image/jpeg width=640 height=360 byte_size=31\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if strings.Contains(stdout.String(), "secret") || strings.Contains(stdout.String(), "raw-jpeg") {
		t.Fatalf("stdout leaked a sensitive value: %q", stdout.String())
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
	for _, value := range capture.jpeg.Bytes {
		if value != 0 {
			t.Fatal("capture JPEG bytes were not cleared before run() returned")
		}
	}
}

func TestRunRedactsCaptureFailure(t *testing.T) {
	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	status := run([]string{"--camera-id=42"}, configuredEnvironment, stdout, stderr, Dependencies{
		NewCaptureService: func(string) captureService {
			return &fakeCapture{code: nvrsnapshot.ErrorCode("upstream-token=private-value")}
		},
	})

	if status == 0 {
		t.Fatal("run() status = 0, want non-zero")
	}
	if got, want := stdout.String(), "camera_id=42 error_code=decode_failed\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if strings.Contains(stdout.String(), "private") || strings.Contains(stdout.String(), "token") {
		t.Fatalf("stdout leaked upstream detail: %q", stdout.String())
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestRunPassesTimeoutContextToCapture(t *testing.T) {
	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	capture := fakeCapture{}
	status := run([]string{"--camera-id=42", "--timeout=3s"}, configuredEnvironment, stdout, stderr, Dependencies{
		NewCaptureService: func(string) captureService { return &capture },
	})

	if status != 0 {
		t.Fatalf("run() status = %d, want 0", status)
	}
	if !capture.hadDeadline {
		t.Fatal("Capture() did not receive a deadline")
	}
	if capture.timeout < 2*time.Second || capture.timeout > 3*time.Second {
		t.Fatalf("capture timeout = %s, want approximately 3s", capture.timeout)
	}
}

func TestNVRLabAuthorizerMapsOnlyLiveRequestAndRedactsTimeout(t *testing.T) {
	client := &fakeAuthorizationClient{url: "wss://short-lived.example.test/ws?token=private"}
	authorizer := nvrLabAuthorizer{client: client}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	url, code := authorizer.AuthorizeStream(ctx, 42, nvrsnapshot.StreamRequest{Mode: nvrsnapshot.StreamModeLive})
	if code != "" || url != client.url {
		t.Fatalf("AuthorizeStream() = (%q, %q), want URL and no code", url, code)
	}
	if client.cameraID != 42 || client.request != (nvrlab.StreamSessionRequest{Mode: nvrlab.ModeLive}) {
		t.Fatalf("CreateStreamURL() = (%d, %#v), want live request", client.cameraID, client.request)
	}
	if !client.hadDeadline {
		t.Fatal("CreateStreamURL() did not receive context deadline")
	}

	client.err = context.DeadlineExceeded
	url, code = authorizer.AuthorizeStream(ctx, 42, nvrsnapshot.StreamRequest{Mode: nvrsnapshot.StreamModeLive})
	if url != "" || code != nvrsnapshot.ErrorAuthorizationFailed {
		t.Fatalf("timeout result = (%q, %q), want authorization failure without URL", url, code)
	}

	_, code = authorizer.AuthorizeStream(ctx, 42, nvrsnapshot.StreamRequest{Mode: nvrsnapshot.StreamModePlayback})
	if code != nvrsnapshot.ErrorAuthorizationFailed {
		t.Fatalf("playback code = %q, want authorization failure", code)
	}
}

type fakeCapture struct {
	jpeg        nvrsnapshot.JPEG
	code        nvrsnapshot.ErrorCode
	hadDeadline bool
	timeout     time.Duration
}

func (c *fakeCapture) Capture(ctx context.Context, _ int64, _ nvrsnapshot.StreamRequest) (nvrsnapshot.JPEG, nvrsnapshot.ErrorCode) {
	deadline, ok := ctx.Deadline()
	c.hadDeadline = ok
	if ok {
		c.timeout = time.Until(deadline)
	}
	if c.jpeg.ContentType == "" && c.code == "" {
		c.jpeg = nvrsnapshot.JPEG{Bytes: []byte("jpeg"), ContentType: "image/jpeg", Width: 1, Height: 1}
	}
	return c.jpeg, c.code
}

func configuredEnvironment(name string) string {
	if name == k8sNVRStreamAuthorizationEnv {
		return "preferred-service-secret"
	}
	return ""
}

type fakeAuthorizationClient struct {
	url         string
	err         error
	cameraID    int64
	request     nvrlab.StreamSessionRequest
	hadDeadline bool
}

func (c *fakeAuthorizationClient) CreateStreamURL(ctx context.Context, cameraID int64, request nvrlab.StreamSessionRequest) (string, error) {
	_, c.hadDeadline = ctx.Deadline()
	c.cameraID = cameraID
	c.request = request
	return c.url, c.err
}

var _ captureService = (*fakeCapture)(nil)
var _ streamURLCreator = (*fakeAuthorizationClient)(nil)
