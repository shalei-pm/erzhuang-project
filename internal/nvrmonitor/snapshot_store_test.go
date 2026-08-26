package nvrmonitor

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

func TestAssetSnapshotStoreOpensOnlyDeterministicCameraKey(t *testing.T) {
	ctx := context.Background()
	store := assets.NewLocalStore(t.TempDir())
	if err := store.Save(ctx, snapshotObjectKey(10001, 111), strings.NewReader("jpeg-data"), "image/jpeg"); err != nil {
		t.Fatal(err)
	}

	snapshots := NewAssetSnapshotStore(store)
	reader, contentType, err := snapshots.Open(ctx, 10001, 111)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "jpeg-data" || contentType != "image/jpeg" {
		t.Fatalf("data=%q contentType=%q", data, contentType)
	}

	_, _, err = snapshots.Open(ctx, 10001, 112)
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("Open(missing) error = %v, want ErrSnapshotNotFound", err)
	}
}
