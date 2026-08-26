package nvrsnapshot

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

func TestBackfillServiceRunsSeriallyWithRequestSpacing(t *testing.T) {
	repo := &fakeSnapshotRepository{candidates: []Candidate{{TenantID: 10001, CameraID: 1}, {TenantID: 10001, CameraID: 2}}}
	capture := &fakeCameraCapture{jpeg: JPEG{Bytes: []byte{1}, ContentType: "image/jpeg", Width: 1, Height: 1}}
	service := NewBackfillService(repo, capture, fakeObjectStore{})
	var sleeps int
	service.sleep = func(context.Context, time.Duration) error { sleeps++; return nil }
	summary, err := service.Run(context.Background(), BackfillOptions{Selection: Selection{TenantID: 10001}})
	if err != nil || summary.Succeeded != 2 || sleeps != 1 || capture.calls != 2 {
		t.Fatalf("Run() = %#v, %v; sleeps=%d calls=%d", summary, err, sleeps, capture.calls)
	}
}

func TestBackfillServicePersistsFailuresAndTripsCircuit(t *testing.T) {
	repo := &fakeSnapshotRepository{candidates: []Candidate{{TenantID: 10001, CameraID: 1}, {TenantID: 10001, CameraID: 2}, {TenantID: 10001, CameraID: 3}, {TenantID: 10001, CameraID: 4}}}
	capture := &fakeCameraCapture{code: ErrorWSSConnectFailed}
	service := NewBackfillService(repo, capture, fakeObjectStore{})
	service.sleep = func(context.Context, time.Duration) error { return nil }
	summary, err := service.Run(context.Background(), BackfillOptions{Selection: Selection{TenantID: 10001}})
	if !errors.Is(err, ErrCircuitOpen) || summary.Failed != 3 || capture.calls != 3 {
		t.Fatalf("Run() = %#v, %v; calls=%d", summary, err, capture.calls)
	}
}

func TestBackfillServiceSkipsExistingObjectUnlessForceRequested(t *testing.T) {
	repo := &fakeSnapshotRepository{candidates: []Candidate{{TenantID: 10001, CameraID: 1}}}
	capture := &fakeCameraCapture{jpeg: JPEG{Bytes: []byte{1}, ContentType: "image/jpeg", Width: 1, Height: 1}}
	objects := fakeObjectStore{existing: map[string]bool{snapshotObjectKey(10001, 1): true}}
	service := NewBackfillService(repo, capture, objects)

	summary, err := service.Run(context.Background(), BackfillOptions{Selection: Selection{TenantID: 10001}})
	if err != nil || summary.Skipped != 1 || capture.calls != 0 {
		t.Fatalf("default Run() = %#v, %v; calls=%d", summary, err, capture.calls)
	}
	summary, err = service.Run(context.Background(), BackfillOptions{Selection: Selection{TenantID: 10001}, Force: true})
	if err != nil || summary.Succeeded != 1 || capture.calls != 1 {
		t.Fatalf("forced Run() = %#v, %v; calls=%d", summary, err, capture.calls)
	}
}

func TestBackfillServiceRejectsAmbiguousAllTenantSelection(t *testing.T) {
	service := NewBackfillService(&fakeSnapshotRepository{}, &fakeCameraCapture{}, fakeObjectStore{})
	_, err := service.Run(context.Background(), BackfillOptions{Selection: Selection{AllTenants: true, CameraID: 1}})
	if err == nil {
		t.Fatal("Run() accepted an all-tenant selection with a camera filter")
	}
}

type fakeSnapshotRepository struct {
	candidates []Candidate
}

func (r *fakeSnapshotRepository) ListCandidates(context.Context, Selection) ([]Candidate, error) {
	return r.candidates, nil
}

type fakeCameraCapture struct {
	jpeg  JPEG
	code  ErrorCode
	calls int
}

func (c *fakeCameraCapture) Capture(context.Context, int64, StreamRequest) (JPEG, ErrorCode) {
	c.calls++
	return c.jpeg, c.code
}

type fakeObjectStore struct{ existing map[string]bool }

func (fakeObjectStore) Save(context.Context, string, io.Reader, string) error { return nil }
func (s fakeObjectStore) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	if s.existing[key] {
		return io.NopCloser(strings.NewReader("jpeg")), "image/jpeg", nil
	}
	return nil, "", assets.ErrNotFound
}
