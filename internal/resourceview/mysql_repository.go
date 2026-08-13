package resourceview

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) ListStores(ctx context.Context, filters StoreFilters) ([]StoreRecords, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("mysql resource view repository is not configured")
	}
	filters = normalizeStoreFilters(filters)
	query := listStoreBaseSQL
	args := []any{}
	if filters.CityID > 0 {
		query += " and t.city_id = ?"
		args = append(args, filters.CityID)
	}
	if filters.Query != "" {
		query += " and (t.name like ? or t.hospital_name like ? or cast(t.id as char) = ?)"
		like := "%" + filters.Query + "%"
		args = append(args, like, like, filters.Query)
	}
	query += " order by t.city_id asc, t.id asc"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []StoreRecords{}
	for rows.Next() {
		tenant, err := scanBusinessTenant(rows)
		if err != nil {
			return nil, err
		}
		record, err := r.getStoreRecordsForTenant(ctx, tenant)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *MySQLRepository) GetStoreRecords(ctx context.Context, tenantID int64) (StoreRecords, error) {
	if r == nil || r.db == nil {
		return StoreRecords{}, errors.New("mysql resource view repository is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
select id, name, hospital_name, status, province_id, city_id
from tb_crm_admin_tenant
where id = ? and status = 1
  and exists (
    select 1
    from tb_crm_iot_device d
    where d.tenant_id = tb_crm_admin_tenant.id
      and d.category = 'edge'
      and d.status = 1
      and d.deleted_at is null
  )`, tenantID)
	tenant, err := scanBusinessTenant(row)
	if errors.Is(err, sql.ErrNoRows) {
		return StoreRecords{}, ErrNotFound
	}
	if err != nil {
		return StoreRecords{}, err
	}
	return r.getStoreRecordsForTenant(ctx, tenant)
}

func (r *MySQLRepository) getStoreRecordsForTenant(ctx context.Context, tenant BusinessTenant) (StoreRecords, error) {
	devices, err := r.listDevices(ctx, tenant.ID)
	if err != nil {
		return StoreRecords{}, err
	}
	spaces, err := r.listSpaces(ctx, tenant.ID)
	if err != nil {
		return StoreRecords{}, err
	}
	relations, err := r.listRelations(ctx, tenant.ID)
	if err != nil {
		return StoreRecords{}, err
	}
	return StoreRecords{Tenant: tenant, Devices: devices, Spaces: spaces, Relations: relations}, nil
}

func (r *MySQLRepository) listDevices(ctx context.Context, tenantID int64) ([]BusinessDevice, error) {
	rows, err := r.db.QueryContext(ctx, `
select id, tenant_id, coalesce(parent_id, 0), name, hardware_id, sn, ip, category, provider,
       status, online_status, ext_params, heartbeat_at, deleted_at
from tb_crm_iot_device
where tenant_id = ?
  and deleted_at is null
  and category in ('edge', 'nvr', 'camera')
order by id asc`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := []BusinessDevice{}
	for rows.Next() {
		device, err := scanBusinessDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

func (r *MySQLRepository) listSpaces(ctx context.Context, tenantID int64) ([]BusinessSpace, error) {
	rows, err := r.db.QueryContext(ctx, `
select id, tenant_id, coalesce(parent_id, 0), name, code, level, status, dict_id, sort_order
from tb_crm_consulting_room
where tenant_id = ?
order by level asc, sort_order asc, id asc`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	spaces := []BusinessSpace{}
	for rows.Next() {
		space, err := scanBusinessSpace(rows)
		if err != nil {
			return nil, err
		}
		spaces = append(spaces, space)
	}
	return spaces, rows.Err()
}

func (r *MySQLRepository) listRelations(ctx context.Context, tenantID int64) ([]BusinessAreaDeviceRelation, error) {
	rows, err := r.db.QueryContext(ctx, `
select id, device_id, area_id, function_type, created_at
from tb_crm_iot_area_device_relation
where area_id in (
  select id from tb_crm_consulting_room where tenant_id = ?
)
order by id asc`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	relations := []BusinessAreaDeviceRelation{}
	for rows.Next() {
		relation, err := scanBusinessAreaDeviceRelation(rows)
		if err != nil {
			return nil, err
		}
		relations = append(relations, relation)
	}
	return relations, rows.Err()
}

const listStoreBaseSQL = `
select t.id, t.name, t.hospital_name, t.status, t.province_id, t.city_id
from tb_crm_admin_tenant t
where t.status = 1
  and exists (
    select 1
    from tb_crm_iot_device d
    where d.tenant_id = t.id
      and d.category = 'edge'
      and d.status = 1
      and d.deleted_at is null
  )`

func scanBusinessTenant(scanner interface{ Scan(dest ...any) error }) (BusinessTenant, error) {
	var tenant BusinessTenant
	var name, hospitalName sql.NullString
	var status, provinceID, cityID sql.NullInt64
	if err := scanner.Scan(&tenant.ID, &name, &hospitalName, &status, &provinceID, &cityID); err != nil {
		return BusinessTenant{}, err
	}
	tenant.Name = strings.TrimSpace(name.String)
	tenant.HospitalName = strings.TrimSpace(hospitalName.String)
	tenant.Status = int(nullInt64(status))
	tenant.ProvinceID = nullInt64(provinceID)
	tenant.CityID = nullInt64(cityID)
	return tenant, nil
}

func scanBusinessDevice(scanner interface{ Scan(dest ...any) error }) (BusinessDevice, error) {
	var device BusinessDevice
	var name, hardwareID, sn, ip, category, provider, extParams sql.NullString
	var parentID, status, onlineStatus sql.NullInt64
	var heartbeatAt, deletedAt sql.NullTime
	if err := scanner.Scan(
		&device.ID,
		&device.TenantID,
		&parentID,
		&name,
		&hardwareID,
		&sn,
		&ip,
		&category,
		&provider,
		&status,
		&onlineStatus,
		&extParams,
		&heartbeatAt,
		&deletedAt,
	); err != nil {
		return BusinessDevice{}, err
	}
	device.ParentID = nullInt64(parentID)
	device.Name = strings.TrimSpace(name.String)
	device.HardwareID = strings.TrimSpace(hardwareID.String)
	device.SN = strings.TrimSpace(sn.String)
	device.IP = strings.TrimSpace(ip.String)
	device.Category = strings.TrimSpace(category.String)
	device.Provider = strings.TrimSpace(provider.String)
	device.Status = int(nullInt64(status))
	device.OnlineStatus = int(nullInt64(onlineStatus))
	device.ExtParams = strings.TrimSpace(extParams.String)
	device.HeartbeatAt = nullTimePtr(heartbeatAt)
	device.DeletedAt = nullTimePtr(deletedAt)
	return device, nil
}

func scanBusinessSpace(scanner interface{ Scan(dest ...any) error }) (BusinessSpace, error) {
	var space BusinessSpace
	var name, code sql.NullString
	var parentID, level, status, dictID, sortOrder sql.NullInt64
	if err := scanner.Scan(
		&space.ID,
		&space.TenantID,
		&parentID,
		&name,
		&code,
		&level,
		&status,
		&dictID,
		&sortOrder,
	); err != nil {
		return BusinessSpace{}, err
	}
	space.ParentID = nullInt64(parentID)
	space.Name = strings.TrimSpace(name.String)
	space.Code = strings.TrimSpace(code.String)
	space.Level = int(nullInt64(level))
	space.Status = int(nullInt64(status))
	space.DictID = nullInt64(dictID)
	space.SortOrder = int(nullInt64(sortOrder))
	return space, nil
}

func scanBusinessAreaDeviceRelation(scanner interface{ Scan(dest ...any) error }) (BusinessAreaDeviceRelation, error) {
	var relation BusinessAreaDeviceRelation
	var functionType sql.NullString
	var createdAt sql.NullTime
	if err := scanner.Scan(&relation.ID, &relation.DeviceID, &relation.AreaID, &functionType, &createdAt); err != nil {
		return BusinessAreaDeviceRelation{}, err
	}
	relation.FunctionType = strings.TrimSpace(functionType.String)
	if createdAt.Valid {
		relation.CreatedAt = createdAt.Time
	}
	return relation, nil
}

func nullInt64(value sql.NullInt64) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
