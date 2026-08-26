package nvrsnapshot

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

// AcquireJobLock prevents two independently submitted Jobs from capturing in
// parallel. The returned release function must be called after Run completes.
func (r *MySQLRepository) AcquireJobLock(ctx context.Context) (func(), error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nvr snapshot repository is not configured")
	}
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var acquired int
	if err := conn.QueryRowContext(ctx, "select get_lock('erzhuang:nvr-snapshot-backfill', 0)").Scan(&acquired); err != nil || acquired != 1 {
		conn.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("nvr snapshot backfill is already running")
	}
	return func() {
		_, _ = conn.ExecContext(context.Background(), "select release_lock('erzhuang:nvr-snapshot-backfill')")
		_ = conn.Close()
	}, nil
}

func (r *MySQLRepository) ListCandidates(ctx context.Context, selection Selection) ([]Candidate, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nvr snapshot repository is not configured")
	}
	if err := validateSelection(selection); err != nil {
		return nil, err
	}

	query := `
		select d.tenant_id, d.id
		from tb_crm_iot_device d
		left join tb_nvr_camera_snapshots s on s.camera_id = d.id
		where d.tenant_id = ?
		  and d.category = 'camera'
		  and d.provider = 'HikVisionNvrChannel'
		  and d.status = 1
		  and d.deleted_at is null`
	args := []any{selection.TenantID}
	if selection.CameraID > 0 {
		query += " and d.id = ?"
		args = append(args, selection.CameraID)
	}
	switch selection.Mode {
	case SelectionMissingOnly:
		query += " and s.camera_id is null"
	case SelectionResumeFailed:
		query += " and (s.camera_id is null or s.status <> 'succeeded')"
	}
	query += " order by d.id asc"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Candidate{}
	for rows.Next() {
		var candidate Candidate
		if err := rows.Scan(&candidate.TenantID, &candidate.CameraID); err != nil {
			return nil, err
		}
		result = append(result, candidate)
	}
	return result, rows.Err()
}

func (r *MySQLRepository) UpsertSnapshot(ctx context.Context, snapshot Snapshot) error {
	if r == nil || r.db == nil {
		return errors.New("nvr snapshot repository is not configured")
	}
	if err := validateSnapshot(snapshot); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `
		insert into tb_nvr_camera_snapshots (
			camera_id, tenant_id, status, oss_object_key, content_type, width, height,
			byte_size, captured_at, attempted_at, error_code
		) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on duplicate key update
			tenant_id = values(tenant_id),
			status = values(status),
			oss_object_key = values(oss_object_key),
			content_type = values(content_type),
			width = values(width),
			height = values(height),
			byte_size = values(byte_size),
			captured_at = values(captured_at),
			attempted_at = values(attempted_at),
			error_code = values(error_code)`,
		snapshot.CameraID, snapshot.TenantID, snapshot.Status, snapshot.ObjectKey, snapshot.ContentType,
		snapshot.Width, snapshot.Height, snapshot.ByteSize, snapshot.CapturedAt, snapshot.AttemptedAt.UTC(), snapshot.ErrorCode,
	)
	return err
}

func validateSelection(selection Selection) error {
	if selection.TenantID <= 0 || selection.CameraID < 0 {
		return errors.New("nvr snapshot selection is invalid")
	}
	if selection.Mode != SelectionMissingOnly && selection.Mode != SelectionResumeFailed {
		return errors.New("nvr snapshot selection mode is invalid")
	}
	return nil
}

func validateSnapshot(snapshot Snapshot) error {
	if snapshot.TenantID <= 0 || snapshot.CameraID <= 0 || snapshot.AttemptedAt.IsZero() {
		return errors.New("nvr snapshot is invalid")
	}
	if snapshot.Status == SnapshotStatusSucceeded {
		if snapshot.ObjectKey != snapshotObjectKey(snapshot.TenantID, snapshot.CameraID) ||
			snapshot.ContentType != "image/jpeg" || snapshot.Width <= 0 || snapshot.Width > maxThumbnailEdge ||
			snapshot.Height <= 0 || snapshot.Height > maxThumbnailEdge || snapshot.ByteSize <= 0 ||
			snapshot.ByteSize > maxJPEGBytes || snapshot.CapturedAt == nil || snapshot.ErrorCode != "" {
			return errors.New("nvr snapshot success metadata is invalid")
		}
		return nil
	}
	if !isPersistableFailure(snapshot.Status) || snapshot.ErrorCode != snapshot.Status || snapshot.ObjectKey != "" ||
		snapshot.ContentType != "" || snapshot.Width != 0 || snapshot.Height != 0 || snapshot.ByteSize != 0 || snapshot.CapturedAt != nil {
		return errors.New("nvr snapshot failure metadata is invalid")
	}
	return nil
}

func isPersistableFailure(code ErrorCode) bool {
	switch code {
	case ErrorAuthorizationFailed, ErrorWSSConnectFailed, ErrorWSSConnectTimeout, ErrorMediaTimeout,
		ErrorDemuxFailed, ErrorDecodeFailed, ErrorThumbnailInvalid, ErrorOSSUploadFailed:
		return true
	default:
		return false
	}
}

func formatPositiveID(value int64) string {
	if value <= 0 {
		return "0"
	}
	return strconv.FormatInt(value, 10)
}

var _ Repository = (*MySQLRepository)(nil)
