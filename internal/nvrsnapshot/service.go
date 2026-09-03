package nvrsnapshot

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

const (
	defaultCaptureTimeout  = 20 * time.Second
	defaultRequestInterval = 2 * time.Second
	maxConsecutiveFailures = 3
)

var ErrCircuitOpen = errors.New("nvr snapshot circuit opened")

type ObjectStore interface {
	Save(ctx context.Context, key string, body io.Reader, contentType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, string, error)
}

type CameraCapture interface {
	Capture(ctx context.Context, cameraID int64, request StreamRequest) (JPEG, ErrorCode)
}

type BackfillOptions struct {
	Selection       Selection
	Timeout         time.Duration
	RequestInterval time.Duration
	Force           bool
}

type Summary struct {
	Selected  int
	Skipped   int
	Succeeded int
	Failed    int
	Failures  map[ErrorCode]int
}

type BackfillService struct {
	repository Repository
	capture    CameraCapture
	objects    ObjectStore
	sleep      func(context.Context, time.Duration) error
}

func NewBackfillService(repository Repository, capture CameraCapture, objects ObjectStore) *BackfillService {
	return &BackfillService{repository: repository, capture: capture, objects: objects, sleep: sleepContext}
}

func (s *BackfillService) Run(ctx context.Context, options BackfillOptions) (Summary, error) {
	if s == nil || s.repository == nil || s.capture == nil || s.objects == nil {
		return Summary{}, errors.New("nvr snapshot backfill is not configured")
	}
	if err := validateSelection(options.Selection); err != nil {
		return Summary{}, err
	}
	if options.Timeout == 0 {
		options.Timeout = defaultCaptureTimeout
	}
	if options.RequestInterval == 0 {
		options.RequestInterval = defaultRequestInterval
	}
	if options.Timeout <= 0 || options.Timeout > defaultCaptureTimeout || options.RequestInterval < defaultRequestInterval {
		return Summary{}, errors.New("nvr snapshot backfill options are invalid")
	}
	candidates, err := s.repository.ListCandidates(ctx, options.Selection)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{Selected: len(candidates), Failures: map[ErrorCode]int{}}
	consecutive := 0
	captured := false
	for _, candidate := range candidates {
		if !options.Force {
			exists, err := s.objectExists(ctx, candidate)
			if err != nil {
				return summary, err
			}
			if exists {
				summary.Skipped++
				continue
			}
		}
		if captured {
			if err := s.sleep(ctx, options.RequestInterval); err != nil {
				return summary, err
			}
		}
		captured = true
		code, err := s.processCandidate(ctx, candidate, options.Timeout)
		if err != nil {
			return summary, err
		}
		if code == SnapshotStatusSucceeded {
			summary.Succeeded++
			consecutive = 0
			continue
		}
		summary.Failed++
		summary.Failures[code]++
		if isCircuitFailure(code) {
			consecutive++
			if consecutive >= maxConsecutiveFailures {
				return summary, ErrCircuitOpen
			}
		} else {
			consecutive = 0
		}
	}
	return summary, nil
}

func (s *BackfillService) processCandidate(ctx context.Context, candidate Candidate, timeout time.Duration) (ErrorCode, error) {
	captureCtx, cancel := context.WithTimeout(ctx, timeout)
	jpeg, code := s.capture.Capture(captureCtx, candidate.CameraID, StreamRequest{Mode: StreamModeLive})
	cancel()
	defer wipeBytes(jpeg.Bytes)
	if code != "" {
		return code, nil
	}
	if !validBackfillJPEG(jpeg) {
		return ErrorThumbnailInvalid, nil
	}
	key := snapshotObjectKey(candidate.TenantID, candidate.CameraID)
	if err := s.objects.Save(ctx, key, bytes.NewReader(jpeg.Bytes), jpeg.ContentType); err != nil {
		return ErrorOSSUploadFailed, nil
	}
	return SnapshotStatusSucceeded, nil
}

func (s *BackfillService) objectExists(ctx context.Context, candidate Candidate) (bool, error) {
	reader, _, err := s.objects.Open(ctx, snapshotObjectKey(candidate.TenantID, candidate.CameraID))
	if errors.Is(err, assets.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := reader.Close(); err != nil {
		return false, err
	}
	return true, nil
}

func validBackfillJPEG(jpeg JPEG) bool {
	return len(jpeg.Bytes) > 0 && len(jpeg.Bytes) <= maxJPEGBytes && jpeg.ContentType == "image/jpeg" && jpeg.Width > 0 && jpeg.Width <= maxThumbnailEdge && jpeg.Height > 0 && jpeg.Height <= maxThumbnailEdge
}

func isCircuitFailure(code ErrorCode) bool {
	return code == ErrorAuthorizationFailed || code == ErrorWSSConnectFailed || code == ErrorWSSConnectTimeout
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
