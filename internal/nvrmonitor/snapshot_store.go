package nvrmonitor

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"strconv"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

// MySQLSnapshotStore only reads the private, Job-owned snapshot table and
// opens the deterministic object key for rows marked succeeded.
type MySQLSnapshotStore struct {
	db     *sql.DB
	assets assets.Store
}

func NewMySQLSnapshotStore(db *sql.DB, store assets.Store) *MySQLSnapshotStore {
	return &MySQLSnapshotStore{db: db, assets: store}
}

func (s *MySQLSnapshotStore) ListAvailable(ctx context.Context, tenantID int64) (map[int64]bool, error) {
	if s == nil || s.db == nil || tenantID <= 0 {
		return nil, ErrSnapshotNotFound
	}
	rows, err := s.db.QueryContext(ctx, `
		select camera_id
		from tb_nvr_camera_snapshots
		where tenant_id = ?
		  and status = 'succeeded'
		  and oss_object_key = concat('nvr-camera-snapshots/', tenant_id, '/', camera_id, '.jpg')
		  and content_type = 'image/jpeg'`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	available := map[int64]bool{}
	for rows.Next() {
		var cameraID int64
		if err := rows.Scan(&cameraID); err != nil {
			return nil, err
		}
		if cameraID > 0 {
			available[cameraID] = true
		}
	}
	return available, rows.Err()
}

func (s *MySQLSnapshotStore) Open(ctx context.Context, tenantID int64, cameraID int64) (io.ReadCloser, string, error) {
	if s == nil || s.db == nil || s.assets == nil || tenantID <= 0 || cameraID <= 0 {
		return nil, "", ErrSnapshotNotFound
	}
	key := snapshotObjectKey(tenantID, cameraID)
	var found int
	err := s.db.QueryRowContext(ctx, `
		select 1
		from tb_nvr_camera_snapshots
		where tenant_id = ? and camera_id = ? and status = 'succeeded'
		  and oss_object_key = ? and content_type = 'image/jpeg'`, tenantID, cameraID, key).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrSnapshotNotFound
	}
	if err != nil || found != 1 {
		return nil, "", ErrSnapshotNotFound
	}
	reader, contentType, err := s.assets.Open(ctx, key)
	if errors.Is(err, assets.ErrNotFound) {
		return nil, "", ErrSnapshotNotFound
	}
	return reader, contentType, err
}

func snapshotObjectKey(tenantID int64, cameraID int64) string {
	return "nvr-camera-snapshots/" + strconv.FormatInt(tenantID, 10) + "/" + strconv.FormatInt(cameraID, 10) + ".jpg"
}

var _ SnapshotStore = (*MySQLSnapshotStore)(nil)
