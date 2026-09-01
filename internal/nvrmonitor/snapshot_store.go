package nvrmonitor

import (
	"bytes"
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

type assetSnapshotRollback struct {
	assets  assets.Store
	key     string
	body    []byte
	existed bool
}

func (r *assetSnapshotRollback) Rollback(ctx context.Context) error {
	if r == nil || r.assets == nil {
		return errors.New("snapshot rollback is not configured")
	}
	if r.existed {
		return r.assets.Save(ctx, r.key, bytes.NewReader(r.body), "image/jpeg")
	}
	deleter, ok := r.assets.(interface {
		Delete(context.Context, string) error
	})
	if !ok {
		return errors.New("asset store does not support exact delete")
	}
	return deleter.Delete(ctx, r.key)
}

func (s *AssetSnapshotStore) SaveSnapshotWithRollback(ctx context.Context, tenantID int64, cameraID int64, body io.Reader) (SnapshotRollback, error) {
	if s == nil || s.assets == nil || tenantID <= 0 || cameraID <= 0 || body == nil {
		return nil, ErrSnapshotNotFound
	}
	key := snapshotObjectKey(tenantID, cameraID)
	oldReader, _, openErr := s.assets.Open(ctx, key)
	var oldBody []byte
	existed := false
	switch {
	case openErr == nil:
		oldBody, openErr = io.ReadAll(io.LimitReader(oldReader, maxSnapshotUploadBytes+1))
		closeErr := oldReader.Close()
		if openErr == nil {
			openErr = closeErr
		}
		if openErr == nil && len(oldBody) > maxSnapshotUploadBytes {
			openErr = errors.New("existing snapshot is too large")
		}
		existed = openErr == nil
	case errors.Is(openErr, assets.ErrNotFound):
		if _, ok := s.assets.(interface {
			Delete(context.Context, string) error
		}); !ok {
			return nil, errors.New("asset store does not support exact delete")
		}
		openErr = nil
	default:
		// An indeterminate read must never be treated as a missing object.
		return nil, openErr
	}
	if openErr != nil {
		return nil, openErr
	}
	if err := s.Save(ctx, tenantID, cameraID, body); err != nil {
		return nil, err
	}
	return &assetSnapshotRollback{assets: s.assets, key: key, body: oldBody, existed: existed}, nil
}

func (s *AssetSnapshotStore) Delete(ctx context.Context, tenantID int64, cameraID int64) error {
	if s == nil || s.assets == nil || tenantID <= 0 || cameraID <= 0 {
		return ErrSnapshotNotFound
	}
	deleter, ok := s.assets.(interface {
		Delete(context.Context, string) error
	})
	if !ok {
		return errors.New("asset store does not support exact delete")
	}
	return deleter.Delete(ctx, snapshotObjectKey(tenantID, cameraID))
}

func snapshotObjectKey(tenantID int64, cameraID int64) string {
	return "nvr-camera-snapshots/" + strconv.FormatInt(tenantID, 10) + "/" + strconv.FormatInt(cameraID, 10) + ".jpg"
}

var _ SnapshotStore = (*AssetSnapshotStore)(nil)
var _ SnapshotWriter = (*AssetSnapshotStore)(nil)
