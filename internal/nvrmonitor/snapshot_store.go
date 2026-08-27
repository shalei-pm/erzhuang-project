package nvrmonitor

import (
	"context"
	"errors"
	"io"
	"strconv"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

// AssetSnapshotStore reads JPEGs written by the one-shot backfill Job. The
// object key is deterministic, so OSS existence is the complete completion
// record and this read path needs no schema migration or table lookup.
type AssetSnapshotStore struct {
	assets assets.Store
}

func NewAssetSnapshotStore(store assets.Store) *AssetSnapshotStore {
	return &AssetSnapshotStore{assets: store}
}

func (s *AssetSnapshotStore) Open(ctx context.Context, tenantID int64, cameraID int64) (io.ReadCloser, string, error) {
	if s == nil || s.assets == nil || tenantID <= 0 || cameraID <= 0 {
		return nil, "", ErrSnapshotNotFound
	}
	key := snapshotObjectKey(tenantID, cameraID)
	reader, contentType, err := s.assets.Open(ctx, key)
	if errors.Is(err, assets.ErrNotFound) {
		return nil, "", ErrSnapshotNotFound
	}
	return reader, contentType, err
}

func (s *AssetSnapshotStore) Save(ctx context.Context, tenantID int64, cameraID int64, body io.Reader) error {
	if s == nil || s.assets == nil || tenantID <= 0 || cameraID <= 0 || body == nil {
		return ErrSnapshotNotFound
	}
	return s.assets.Save(ctx, snapshotObjectKey(tenantID, cameraID), body, "image/jpeg")
}

func snapshotObjectKey(tenantID int64, cameraID int64) string {
	return "nvr-camera-snapshots/" + strconv.FormatInt(tenantID, 10) + "/" + strconv.FormatInt(cameraID, 10) + ".jpg"
}

var _ SnapshotStore = (*AssetSnapshotStore)(nil)
var _ SnapshotWriter = (*AssetSnapshotStore)(nil)
