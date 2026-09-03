package nvrsnapshot

import (
	"context"
	"database/sql"
	"errors"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
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
		where d.category = 'camera'
		  and d.provider = 'HikVisionNvrChannel'
		  and d.status = 1
		  and d.deleted_at is null`
	args := []any{}
	if !selection.AllTenants {
		query += " and d.tenant_id = ?"
		args = append(args, selection.TenantID)
	}
	if selection.CameraID > 0 {
		query += " and d.id = ?"
		args = append(args, selection.CameraID)
	}
	query += " order by d.tenant_id asc, d.id asc"

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

func validateSelection(selection Selection) error {
	if selection.CameraID < 0 {
		return errors.New("nvr snapshot selection is invalid")
	}
	if selection.AllTenants {
		if selection.TenantID != 0 || selection.CameraID != 0 {
			return errors.New("nvr snapshot selection is invalid")
		}
		return nil
	}
	if selection.TenantID <= 0 {
		return errors.New("nvr snapshot selection is invalid")
	}
	return nil
}

var _ Repository = (*MySQLRepository)(nil)
