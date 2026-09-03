package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/nvrlab"
	"github.com/shalei-pm/erzhuang-project/internal/nvrsnapshot"
)

const (
	k8sNVRStreamAuthorizationEnv = "K8S_SECRET_NVR_STREAM_AUTHORIZATION"
	nvrStreamAuthorizationEnv    = "NVR_STREAM_AUTHORIZATION"
	defaultTimeout               = 20 * time.Second

	errorInvalidArguments nvrsnapshot.ErrorCode = "invalid_arguments"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr, Dependencies{}))
}

type options struct {
	cameraID int64
	timeout  time.Duration
}

type captureService interface {
	Capture(context.Context, int64, nvrsnapshot.StreamRequest) (nvrsnapshot.JPEG, nvrsnapshot.ErrorCode)
}

type streamURLCreator interface {
	CreateStreamURL(context.Context, int64, nvrlab.StreamSessionRequest) (string, error)
}

// Dependencies make the command independently testable without opening an
// HTTP, WSS, or ffmpeg connection. Zero-value dependencies use production code.
type Dependencies struct {
	NewCaptureService func(authorization string) captureService
}

func run(args []string, getenv func(string) string, stdout, _ io.Writer, dependencies Dependencies) int {
	options, ok := parseOptions(args)
	if !ok {
		writeFailure(stdout, 0, errorInvalidArguments)
		return 2
	}

	authorization := readAuthorization(getenv)
	if authorization == "" {
		writeFailure(stdout, options.cameraID, nvrsnapshot.ErrorAuthorizationFailed)
		return 1
	}

	newCaptureService := dependencies.NewCaptureService
	if newCaptureService == nil {
		newCaptureService = newProductionCaptureService
	}
	capture := newCaptureService(authorization)
	if capture == nil {
		writeFailure(stdout, options.cameraID, nvrsnapshot.ErrorAuthorizationFailed)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.timeout)
	defer cancel()

	jpeg, code := capture.Capture(ctx, options.cameraID, nvrsnapshot.StreamRequest{Mode: nvrsnapshot.StreamModeLive})
	defer wipeJPEG(&jpeg)
	if code != "" {
		writeFailure(stdout, options.cameraID, safeErrorCode(code))
		return 1
	}
	if !validJPEGMetadata(jpeg) {
		writeFailure(stdout, options.cameraID, nvrsnapshot.ErrorThumbnailInvalid)
		return 1
	}

	fmt.Fprintf(stdout, "camera_id=%d content_type=%s width=%d height=%d byte_size=%d\n", options.cameraID, jpeg.ContentType, jpeg.Width, jpeg.Height, len(jpeg.Bytes))
	return 0
}

func parseOptions(args []string) (options, bool) {
	if cameraIDFlagCount(args) != 1 {
		return options{}, false
	}
	flags := flag.NewFlagSet("nvr-snapshot-spike", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	cameraID := flags.Int64("camera-id", 0, "")
	timeout := flags.Duration("timeout", defaultTimeout, "")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *cameraID <= 0 || *timeout <= 0 || *timeout > defaultTimeout {
		return options{}, false
	}
	return options{cameraID: *cameraID, timeout: *timeout}, true
}

func cameraIDFlagCount(args []string) int {
	count := 0
	for _, arg := range args {
		if arg == "--camera-id" || arg == "-camera-id" || strings.HasPrefix(arg, "--camera-id=") || strings.HasPrefix(arg, "-camera-id=") {
			count++
		}
	}
	return count
}

func readAuthorization(getenv func(string) string) string {
	if getenv == nil {
		return ""
	}
	if authorization := strings.TrimSpace(getenv(k8sNVRStreamAuthorizationEnv)); authorization != "" {
		return authorization
	}
	return strings.TrimSpace(getenv(nvrStreamAuthorizationEnv))
}

func newProductionCaptureService(authorization string) captureService {
	authorizer := nvrLabAuthorizer{
		client: nvrlab.NewHTTPAuthorizationClient(&http.Client{Timeout: defaultTimeout}, authorization),
	}
	return nvrsnapshot.NewCaptureService(
		authorizer,
		nvrsnapshot.NewWebSocketJPEGCapture(nvrsnapshot.NhooyrWebSocketDialer{}, nvrsnapshot.ExecCommandFactory{}),
	)
}

type nvrLabAuthorizer struct {
	client streamURLCreator
}

func (a nvrLabAuthorizer) AuthorizeStream(ctx context.Context, cameraID int64, request nvrsnapshot.StreamRequest) (string, nvrsnapshot.ErrorCode) {
	if a.client == nil || request.Mode != nvrsnapshot.StreamModeLive || request.StartTime != 0 || request.EndTime != 0 {
		return "", nvrsnapshot.ErrorAuthorizationFailed
	}
	wssURL, err := a.client.CreateStreamURL(ctx, cameraID, nvrlab.StreamSessionRequest{Mode: nvrlab.ModeLive})
	if err != nil || strings.TrimSpace(wssURL) == "" {
		return "", nvrsnapshot.ErrorAuthorizationFailed
	}
	return wssURL, ""
}

func validJPEGMetadata(jpeg nvrsnapshot.JPEG) bool {
	return len(jpeg.Bytes) > 0 && jpeg.ContentType == "image/jpeg" && jpeg.Width > 0 && jpeg.Height > 0
}

func safeErrorCode(code nvrsnapshot.ErrorCode) nvrsnapshot.ErrorCode {
	switch code {
	case errorInvalidArguments,
		nvrsnapshot.ErrorAuthorizationFailed,
		nvrsnapshot.ErrorWSSConnectFailed,
		nvrsnapshot.ErrorWSSConnectTimeout,
		nvrsnapshot.ErrorMediaTimeout,
		nvrsnapshot.ErrorDemuxFailed,
		nvrsnapshot.ErrorDecodeFailed,
		nvrsnapshot.ErrorThumbnailInvalid:
		return code
	default:
		return nvrsnapshot.ErrorDecodeFailed
	}
}

func writeFailure(writer io.Writer, cameraID int64, code nvrsnapshot.ErrorCode) {
	fmt.Fprintf(writer, "camera_id=%d error_code=%s\n", cameraID, safeErrorCode(code))
}

func wipeJPEG(jpeg *nvrsnapshot.JPEG) {
	if jpeg == nil {
		return
	}
	clear(jpeg.Bytes[:cap(jpeg.Bytes)])
	jpeg.Bytes = nil
}
