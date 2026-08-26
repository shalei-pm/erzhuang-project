package nvrsnapshot

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestBackfillServiceRunsSeriallyWithRequestSpacing(t *testing.T) {
	repo := &fakeSnapshotRepository{candidates: []Candidate{{TenantID: 10001, CameraID: 1}, {TenantID: 10001, CameraID: 2}}}
	capture := &fakeCameraCapture{jpeg: JPEG{Bytes: []byte{1}, ContentType: "image/jpeg", Width: 1, Height: 1}}
	service := NewBackfillService(repo, capture, fakeObjectStore{})
	var sleeps int
	service.now = func() time.Time { return time.Unix(1, 0) }
	service.sleep = func(context.Context, time.Duration) error { sleeps++; return nil }
	summary, err := service.Run(context.Background(), BackfillOptions{Selection: Selection{TenantID: 10001, Mode: SelectionMissingOnly}})
	if err != nil || summary.Succeeded != 2 || sleeps != 1 || capture.calls != 2 {
		t.Fatalf("Run() = %#v, %v; sleeps=%d calls=%d", summary, err, sleeps, capture.calls)
	}
	if len(repo.snapshots) != 2 || repo.snapshots[0].Status != SnapshotStatusSucceeded {
		t.Fatalf("snapshots = %#v", repo.snapshots)
	}
}

func TestBackfillServicePersistsFailuresAndTripsCircuit(t *testing.T) {
	repo := &fakeSnapshotRepository{candidates: []Candidate{{TenantID: 10001, CameraID: 1}, {TenantID: 10001, CameraID: 2}, {TenantID: 10001, CameraID: 3}, {TenantID: 10001, CameraID: 4}}}
	capture := &fakeCameraCapture{code: ErrorWSSConnectFailed}
	service := NewBackfillService(repo, capture, fakeObjectStore{})
	service.sleep = func(context.Context, time.Duration) error { return nil }
	summary, err := service.Run(context.Background(), BackfillOptions{Selection: Selection{TenantID: 10001, Mode: SelectionMissingOnly}})
	if !errors.Is(err, ErrCircuitOpen) || summary.Failed != 3 || capture.calls != 3 {
		t.Fatalf("Run() = %#v, %v; calls=%d", summary, err, capture.calls)
	}
	for _, snapshot := range repo.snapshots {
		if snapshot.Status != ErrorWSSConnectFailed || snapshot.ErrorCode != ErrorWSSConnectFailed {
			t.Fatalf("failure snapshot = %#v", snapshot)
		}
	}
}

type fakeSnapshotRepository struct {
	candidates []Candidate
	snapshots  []Snapshot
}

func (r *fakeSnapshotRepository) ListCandidates(context.Context, Selection) ([]Candidate, error) {
	return r.candidates, nil
}
func (r *fakeSnapshotRepository) UpsertSnapshot(_ context.Context, snapshot Snapshot) error {
	r.snapshots = append(r.snapshots, snapshot)
	return nil
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

type fakeObjectStore struct{}

func (fakeObjectStore) Save(context.Context, string, io.Reader, string) error { return nil }
