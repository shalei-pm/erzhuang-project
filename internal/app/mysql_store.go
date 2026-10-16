package app

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

type MySQLStore struct {
	db *sql.DB
}

var errAuditLogTransactionNil = errors.New("audit log transaction is nil")

const mysqlUserResourceScopesTableDDL = `
	create table if not exists tb_user_resource_scopes (
		id bigint primary key auto_increment,
		user_id bigint not null,
		resource_type varchar(32) not null,
		resource_id bigint not null,
		external_key varchar(128) not null,
		scope varchar(64) not null,
		created_at datetime(3) not null default current_timestamp(3),
		updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
		constraint fk_user_resource_scopes_user foreign key (user_id) references tb_users(id) on delete cascade,
		unique key uk_user_resource_scope (user_id, resource_type, resource_id, scope),
		key idx_user_scope (user_id, resource_type, scope),
		key idx_resource_external_scope (resource_type, external_key, scope)
	) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_general_ci
`

const mysqlAuthSessionCreateSQL = `
	insert into tb_auth_sessions (
		session_token_hash, user_id, sso_subject, ip_address, user_agent,
		created_at, last_activity_at, expires_at
	) values (?, ?, ?, ?, ?, utc_timestamp(3), utc_timestamp(3),
		date_add(utc_timestamp(3), interval 30 minute))
`

const mysqlAuthSessionLockUserSQL = `
	select id from tb_users
	where id = ? and enabled = 1
	for update
`

const mysqlAuthSessionConsumedSQL = `
	select id from tb_auth_sessions
	where user_id = ? and sso_subject = ?
	limit 1
	for update
`

const mysqlAuthSessionTouchSQL = `
	update tb_auth_sessions
	set expires_at = least(date_add(created_at, interval 8 hour),
		date_add(greatest(last_activity_at, utc_timestamp(3)), interval 30 minute)),
		last_activity_at = greatest(last_activity_at, utc_timestamp(3))
	where session_token_hash = ?
		and user_id = ?
		and revoked_at is null
		and expires_at > utc_timestamp(3)
		and last_activity_at > date_sub(utc_timestamp(3), interval 30 minute)
		and created_at > date_sub(utc_timestamp(3), interval 8 hour)
`

const mysqlAuthSessionValidSQL = `
	select 1
	from tb_auth_sessions
	where session_token_hash = ?
		and user_id = ?
		and revoked_at is null
		and expires_at > utc_timestamp(3)
		and last_activity_at > date_sub(utc_timestamp(3), interval 30 minute)
		and created_at > date_sub(utc_timestamp(3), interval 8 hour)
	for update
`

const mysqlAuthSessionAbsoluteExpiredSQL = `
	select 1
	from tb_auth_sessions
	where session_token_hash = ?
		and user_id = ?
		and revoked_at is null
		and created_at <= date_sub(utc_timestamp(3), interval 8 hour)
	for update
`

const mysqlAuthSessionStatusSQL = `
	select
		timestampdiff(microsecond, utc_timestamp(3), least(expires_at, date_add(last_activity_at, interval 30 minute))) div 1000,
		timestampdiff(microsecond, utc_timestamp(3), date_add(created_at, interval 8 hour)) div 1000,
		revoked_at is not null
	from tb_auth_sessions
	where session_token_hash = ? and user_id = ?
`

const mysqlAuthSessionRevokeSQL = `
	update tb_auth_sessions
	set revoked_at = ?, revoked_reason = ?
	where session_token_hash = ?
		and user_id = ?
		and revoked_at is null
`

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) Name() string {
	return "mysql"
}

func (s *MySQLStore) CreateAuthSession(ctx context.Context, input AuthSessionCreate) (string, error) {
	token, err := newAuthSessionToken()
	if err != nil {
		return "", err
	}
	hash := hashAuthSessionToken(token)
	subject := strings.TrimSpace(input.SSOSubject)
	args := []any{
		hex.EncodeToString(hash[:]),
		input.UserID,
		subject,
		sanitizeAuditMetadata(input.IPAddress, 64),
		sanitizeAuditMetadata(input.UserAgent, 512),
	}
	if isSSOCredentialFingerprint(subject) {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return "", err
		}
		defer tx.Rollback()

		// Every fingerprint writer must take this lock, including when no
		// consumption record exists yet. Expired/revoked records stay consumed.
		var id int64
		if err := tx.QueryRowContext(ctx, mysqlAuthSessionLockUserSQL, input.UserID).Scan(&id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", errSessionReauthenticationRequired
			}
			return "", err
		}
		err = tx.QueryRowContext(ctx, mysqlAuthSessionConsumedSQL, input.UserID, subject).Scan(&id)
		if err == nil {
			return "", errSessionReauthenticationRequired
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		if _, err := tx.ExecContext(ctx, mysqlAuthSessionCreateSQL, args...); err != nil {
			return "", err
		}
		if err := tx.Commit(); err != nil {
			return "", err
		}
	} else {
		// Legacy direct-store callers remain compatible; application callbacks
		// must supply the versioned JWT fingerprint, never an ordinary subject.
		if _, err := s.db.ExecContext(ctx, mysqlAuthSessionCreateSQL, args...); err != nil {
			return "", err
		}
	}
	return token, nil
}

func (s *MySQLStore) TouchAuthSession(ctx context.Context, token string, userID int64, _ time.Time, _ time.Duration) (bool, error) {
	hash := hashAuthSessionToken(token)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, mysqlAuthSessionTouchSQL,
		hex.EncodeToString(hash[:]),
		userID,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 1 {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return true, nil
	}
	if affected != 0 {
		return false, nil
	}

	var exists int
	err = tx.QueryRowContext(ctx, mysqlAuthSessionValidSQL,
		hex.EncodeToString(hash[:]),
		userID,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, mysqlAuthSessionAbsoluteExpiredSQL,
			hex.EncodeToString(hash[:]), userID,
		).Scan(&exists)
		if err == nil {
			return false, errSessionAbsoluteTimeout
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) GetAuthSessionStatus(ctx context.Context, token string, userID int64, _ time.Time) (AuthSessionStatus, error) {
	hash := hashAuthSessionToken(token)
	var status AuthSessionStatus
	var revoked bool
	err := s.db.QueryRowContext(ctx, mysqlAuthSessionStatusSQL, hex.EncodeToString(hash[:]), userID).
		Scan(&status.IdleRemainingMS, &status.AbsoluteRemainingMS, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthSessionStatus{}, errSessionIdleTimeout
	}
	if err != nil {
		return AuthSessionStatus{}, err
	}
	if status.AbsoluteRemainingMS <= 0 {
		return AuthSessionStatus{}, errSessionAbsoluteTimeout
	}
	if revoked || status.IdleRemainingMS <= 0 {
		return AuthSessionStatus{}, errSessionIdleTimeout
	}
	return status, nil
}

func (s *MySQLStore) RevokeAuthSession(ctx context.Context, token string, userID int64, reason string, now time.Time) error {
	hash := hashAuthSessionToken(token)
	_, err := s.db.ExecContext(ctx, mysqlAuthSessionRevokeSQL,
		now,
		sanitizeAuditMetadata(reason, 255),
		hex.EncodeToString(hash[:]),
		userID,
	)
	return err
}

func (s *MySQLStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *MySQLStore) ensureUserResourceScopesTable(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, mysqlUserResourceScopesTableDDL)
	if err != nil {
		return fmt.Errorf("ensure user resource scopes table: %w", err)
	}
	return nil
}

func (s *MySQLStore) ListTasks(ctx context.Context) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, title, done
		from tb_tasks
		order by id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.Title, &task.Done); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *MySQLStore) CreateAuditLog(ctx context.Context, log AuditLog) error {
	return s.RecordAudit(ctx, log)
}

// RecordAudit adapts the shared audit write port to the legacy app store API.
func (s *MySQLStore) RecordAudit(ctx context.Context, event AuditLog) error {
	return insertAuditLog(ctx, s.db, event)
}

// RecorderForTx returns an audit writer bound to a caller-owned transaction.
func (s *MySQLStore) RecorderForTx(tx *sql.Tx) auditlog.AuditRecorder {
	return mysqlAuditLogTxRecorder{tx: tx}
}

type mysqlAuditLogTxRecorder struct {
	tx *sql.Tx
}

func (r mysqlAuditLogTxRecorder) RecordAudit(ctx context.Context, event auditlog.AuditEvent) error {
	if r.tx == nil {
		return errAuditLogTransactionNil
	}
	return insertAuditLog(ctx, r.tx, event)
}

type auditLogExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertAuditLog(ctx context.Context, executor auditLogExecer, log AuditLog) error {
	detailJSON := sanitizeAuditDetail(log.DetailJSON)
	_, err := executor.ExecContext(ctx, `
		insert into tb_audit_logs (
			user_id, actor_display_name, user_email, action, entity_type, entity_id,
			store_id, external_org_id, channel_id, asset_logical_key, ip_address, user_agent, request_id,
			result, detail_json
		) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		log.UserID,
		strings.TrimSpace(log.ActorDisplayName),
		strings.TrimSpace(log.UserEmail),
		strings.TrimSpace(log.Action),
		strings.TrimSpace(log.EntityType),
		log.EntityID,
		log.StoreID,
		strings.TrimSpace(log.ExternalOrgID),
		log.ChannelID,
		strings.TrimSpace(log.AssetLogicalKey),
		sanitizeAuditMetadata(log.IPAddress, 64),
		sanitizeAuditMetadata(log.UserAgent, 512),
		sanitizeAuditMetadata(log.RequestID, 128),
		strings.TrimSpace(log.Result),
		detailJSON,
	)
	return err
}

func (s *MySQLStore) ListAuditLogs(ctx context.Context, filter AuditLogFilter) (AuditLogPage, error) {
	filter, offset := normalizeAuditLogFilter(filter)
	where, args := mysqlAuditLogWhere(filter)

	var total int
	if err := s.db.QueryRowContext(ctx, `
		select count(*)
		from tb_audit_logs
		where `+where,
		args...,
	).Scan(&total); err != nil {
		return AuditLogPage{}, err
	}

	rowArgs := append(append([]any(nil), args...), filter.PageSize, offset)
	rows, err := s.db.QueryContext(ctx, `
		select id, user_id, actor_display_name, user_email, action, entity_type,
			entity_id, store_id, external_org_id, channel_id, asset_logical_key,
			ip_address, user_agent, request_id, result, detail_json, created_at
		from tb_audit_logs
		where `+where+`
		order by created_at desc, id desc
		limit ? offset ?
	`, rowArgs...)
	if err != nil {
		return AuditLogPage{}, err
	}
	defer rows.Close()

	items := make([]AuditLog, 0)
	for rows.Next() {
		log, err := scanAuditLog(rows)
		if err != nil {
			return AuditLogPage{}, err
		}
		items = append(items, log)
	}
	if err := rows.Err(); err != nil {
		return AuditLogPage{}, err
	}
	return AuditLogPage{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

func mysqlAuditLogWhere(filter AuditLogFilter) (string, []any) {
	clauses := []string{"created_at >= ?", "created_at < ?", "action <> ?"}
	args := []any{filter.StartAt, filter.EndAt}
	args = append(args, internalAuditActionSnapshotRefreshPrepare)
	if filter.UserID != nil {
		clauses = append(clauses, "user_id = ?")
		args = append(args, *filter.UserID)
	}
	if filter.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, filter.Action)
	}
	return strings.Join(clauses, " and "), args
}

func scanAuditLog(scanner interface {
	Scan(dest ...any) error
}) (AuditLog, error) {
	var log AuditLog
	var userID, entityID, storeID, channelID sql.NullInt64
	var detailJSON sql.NullString
	if err := scanner.Scan(
		&log.ID,
		&userID,
		&log.ActorDisplayName,
		&log.UserEmail,
		&log.Action,
		&log.EntityType,
		&entityID,
		&storeID,
		&log.ExternalOrgID,
		&channelID,
		&log.AssetLogicalKey,
		&log.IPAddress,
		&log.UserAgent,
		&log.RequestID,
		&log.Result,
		&detailJSON,
		&log.CreatedAt,
	); err != nil {
		return AuditLog{}, err
	}
	if userID.Valid {
		log.UserID = &userID.Int64
	}
	if entityID.Valid {
		log.EntityID = &entityID.Int64
	}
	if storeID.Valid {
		log.StoreID = &storeID.Int64
	}
	if channelID.Valid {
		log.ChannelID = &channelID.Int64
	}
	log.AssetLogicalKey = strings.TrimSpace(log.AssetLogicalKey)
	if detailJSON.Valid {
		log.DetailJSON = sanitizeAuditDetail([]byte(detailJSON.String))
	}
	return log, nil
}

func (s *MySQLStore) GetAIProvider(ctx context.Context) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `
		select value
		from tb_app_settings
		where `+"`key`"+` = 'ai_provider'
	`).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (s *MySQLStore) SetAIProvider(ctx context.Context, provider string) error {
	_, err := s.db.ExecContext(ctx, `
		insert into tb_app_settings (`+"`key`"+`, value, updated_at)
		values ('ai_provider', ?, current_timestamp(3))
		on duplicate key update
			value = values(value),
			updated_at = values(updated_at)
	`, NormalizeAIProvider(provider))
	return err
}

func (s *MySQLStore) GetMonitorScreenshotWatermarkEnabled(ctx context.Context) (bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `
		select value
		from tb_app_settings
		where `+"`key`"+` = ?
	`, monitorScreenshotWatermarkSettingKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return NormalizeMonitorScreenshotWatermarkEnabled(value), nil
}

func (s *MySQLStore) SetMonitorScreenshotWatermarkEnabled(ctx context.Context, enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	_, err := s.db.ExecContext(ctx, `
		insert into tb_app_settings (`+"`key`"+`, value, updated_at)
		values (?, ?, current_timestamp(3))
		on duplicate key update
			value = values(value),
			updated_at = values(updated_at)
	`, monitorScreenshotWatermarkSettingKey, value)
	return err
}

func (s *MySQLStore) GetAuthUserByEmail(ctx context.Context, email string) (AuthUserRecord, error) {
	record, err := scanAuthUser(s.db.QueryRowContext(ctx, `
		select u.id, u.email, u.username, u.display_name, u.feishu_user_id, u.mobile,
			coalesce((
				select r.code
				from tb_user_roles ur, tb_roles r
				where ur.user_id = u.id
					and r.id = ur.role_id
				order by case r.code
					when 'admin' then 1
					when 'editor' then 2
					else 3
				end
				limit 1
			), 'viewer') as role,
			u.enabled,
			u.last_login_at
		from tb_users u
		where lower(u.email) = lower(?)
	`, normalizeEmail(email)))
	if errors.Is(err, sql.ErrNoRows) {
		return AuthUserRecord{}, errAuthUserNotFound
	}
	return record, err
}

func (s *MySQLStore) GetAuthUserByID(ctx context.Context, id int64) (AuthUserRecord, error) {
	return s.getAuthUserByID(ctx, id)
}

func (s *MySQLStore) UpdateAuthUserProfile(ctx context.Context, patch AuthUserPatch) (AuthUserRecord, error) {
	result, err := s.db.ExecContext(ctx, `
		update tb_users
		set username = case when ? <> '' then ? else username end,
			display_name = case when ? <> '' then ? else display_name end,
			feishu_user_id = case when ? <> '' then ? else feishu_user_id end,
			mobile = case when ? <> '' then ? else mobile end,
			last_login_at = current_timestamp(3),
			updated_at = current_timestamp(3)
		where lower(email) = lower(?)
	`,
		strings.TrimSpace(patch.Username), strings.TrimSpace(patch.Username),
		strings.TrimSpace(patch.DisplayName), strings.TrimSpace(patch.DisplayName),
		strings.TrimSpace(patch.FeishuUserID), strings.TrimSpace(patch.FeishuUserID),
		strings.TrimSpace(patch.Phone), strings.TrimSpace(patch.Phone),
		normalizeEmail(patch.Email),
	)
	if err != nil {
		return AuthUserRecord{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AuthUserRecord{}, err
	}
	if affected == 0 {
		return AuthUserRecord{}, errAuthUserNotFound
	}
	return s.GetAuthUserByEmail(ctx, patch.Email)
}

func (s *MySQLStore) ListAuthUsers(ctx context.Context) ([]AuthUserRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		select u.id, u.email, u.username, u.display_name, u.feishu_user_id, u.mobile,
			coalesce((
				select r.code
				from tb_user_roles ur, tb_roles r
				where ur.user_id = u.id
					and r.id = ur.role_id
				order by case r.code
					when 'admin' then 1
					when 'editor' then 2
					else 3
				end
				limit 1
			), 'viewer') as role,
			u.enabled,
			u.last_login_at
		from tb_users u
		order by u.enabled desc, lower(u.email) asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []AuthUserRecord{}
	for rows.Next() {
		user, err := scanAuthUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachMonitorScopes(ctx, users); err != nil {
		return nil, err
	}
	return users, nil
}

func (s *MySQLStore) CreateAuthUser(ctx context.Context, input AuthUserMutation) (AuthUserRecord, error) {
	return s.createAuthUser(ctx, input, nil)
}

func (s *MySQLStore) CreateAuthUserWithAudit(ctx context.Context, input AuthUserMutation, event auditlog.AuditEvent) (AuthUserRecord, error) {
	return s.createAuthUser(ctx, input, &event)
}

func (s *MySQLStore) createAuthUser(ctx context.Context, input AuthUserMutation, event *auditlog.AuditEvent) (AuthUserRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AuthUserRecord{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		insert into tb_users (email, username, display_name, enabled)
		values (?, ?, ?, ?)
	`,
		normalizeEmail(input.Email),
		strings.TrimSpace(input.Username),
		strings.TrimSpace(input.DisplayName),
		input.Enabled,
	)
	if err != nil {
		return AuthUserRecord{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return AuthUserRecord{}, err
	}
	if err := setMySQLUserRole(ctx, tx, id, normalizeRole(input.Role)); err != nil {
		return AuthUserRecord{}, err
	}
	if err := setMySQLUserMonitorScopesWithAudit(ctx, tx, id, input.Role, input.MonitorInstitutionIDs, event); err != nil {
		return AuthUserRecord{}, err
	}
	saved, err := s.getAuthUserByIDFrom(ctx, tx, id)
	if err != nil {
		return AuthUserRecord{}, err
	}
	if input.MonitorInstitutionIDs == nil {
		annotateMonitorScopeAudit(event, saved.MonitorStoreScopes, nil)
	}
	if event != nil {
		event.EntityID = &id
		if err := (mysqlAuditLogTxRecorder{tx: tx}).RecordAudit(ctx, *event); err != nil {
			return AuthUserRecord{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return AuthUserRecord{}, fmt.Errorf("%w: %v", errAuthUserCommitUnknown, err)
	}
	return saved, nil
}

func (s *MySQLStore) UpdateAuthUser(ctx context.Context, id int64, input AuthUserMutation) (AuthUserRecord, error) {
	return s.updateAuthUser(ctx, id, input, nil)
}

func (s *MySQLStore) UpdateAuthUserWithAudit(ctx context.Context, id int64, input AuthUserMutation, event auditlog.AuditEvent) (AuthUserRecord, error) {
	return s.updateAuthUser(ctx, id, input, &event)
}

func (s *MySQLStore) updateAuthUser(ctx context.Context, id int64, input AuthUserMutation, event *auditlog.AuditEvent) (AuthUserRecord, error) {
	if input.MonitorScopeRevision == "" {
		return AuthUserRecord{}, errMonitorScopeConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AuthUserRecord{}, err
	}
	defer tx.Rollback()
	if err := setMySQLUserMonitorScopesWithAudit(ctx, tx, id, input.Role, input.MonitorInstitutionIDs, event, input.MonitorScopeRevision); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthUserRecord{}, errAuthUserNotFound
		}
		return AuthUserRecord{}, err
	}

	result, err := tx.ExecContext(ctx, `
		update tb_users
		set username = ?,
			display_name = ?,
			enabled = ?,
			updated_at = current_timestamp(3)
		where id = ?
	`, strings.TrimSpace(input.Username), strings.TrimSpace(input.DisplayName), input.Enabled, id)
	if err != nil {
		return AuthUserRecord{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AuthUserRecord{}, err
	}
	if affected == 0 {
		var existing int64
		if err := tx.QueryRowContext(ctx, "select id from tb_users where id = ?", id).Scan(&existing); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return AuthUserRecord{}, errAuthUserNotFound
			}
			return AuthUserRecord{}, err
		}
	}
	if err := setMySQLUserRole(ctx, tx, id, normalizeRole(input.Role)); err != nil {
		return AuthUserRecord{}, err
	}
	saved, err := s.getAuthUserByIDFrom(ctx, tx, id)
	if err != nil {
		return AuthUserRecord{}, err
	}
	if input.MonitorInstitutionIDs == nil {
		annotateMonitorScopeAudit(event, saved.MonitorStoreScopes, nil)
	}
	if event != nil {
		event.EntityID = &id
		if err := (mysqlAuditLogTxRecorder{tx: tx}).RecordAudit(ctx, *event); err != nil {
			return AuthUserRecord{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return AuthUserRecord{}, fmt.Errorf("%w: %v", errAuthUserCommitUnknown, err)
	}
	return saved, nil
}

func (s *MySQLStore) getAuthUserByID(ctx context.Context, id int64) (AuthUserRecord, error) {
	return s.getAuthUserByIDFrom(ctx, s.db, id)
}

func (s *MySQLStore) getAuthUserByIDFrom(ctx context.Context, q monitorScopeQueryer, id int64) (AuthUserRecord, error) {
	record, err := scanAuthUser(q.QueryRowContext(ctx, `
		select u.id, u.email, u.username, u.display_name, u.feishu_user_id, u.mobile,
			coalesce((
				select r.code
				from tb_user_roles ur, tb_roles r
				where ur.user_id = u.id
					and r.id = ur.role_id
				order by case r.code
					when 'admin' then 1
					when 'editor' then 2
					else 3
				end
				limit 1
			), 'viewer') as role,
			u.enabled,
			u.last_login_at
		from tb_users u
		where u.id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return AuthUserRecord{}, errAuthUserNotFound
	}
	if err != nil {
		return AuthUserRecord{}, err
	}
	snapshots, err := readCRMMonitorScopes(ctx, q, []int64{record.ID}, true)
	if err != nil {
		return AuthUserRecord{}, err
	}
	record.MonitorStoreScopes = snapshots[record.ID].Scopes
	record.MonitorScopeRevision = userMonitorScopeRevision(snapshots[record.ID].Tokens, record.Role, record.Enabled)
	record.MonitorStoreScopeCount = len(record.MonitorStoreScopes)
	return record, nil
}

func setMySQLUserRole(ctx context.Context, tx *sql.Tx, userID int64, role string) error {
	if _, err := tx.ExecContext(ctx, `
		insert ignore into tb_roles (code, name, description, is_system)
		values
			('admin', '管理员', '全量机构和系统管理权限', 1),
			('editor', '编辑运维', '维护门店、设计图、录像机和通道', 1),
			('viewer', '普通查看', '查看门店、设计图、通道和监控', 1)
	`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from tb_user_roles where user_id = ?`, userID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		insert ignore into tb_user_roles (user_id, role_id)
		select ?, r.id
		from tb_roles r
		where r.code = ?
	`, userID, normalizeRole(role))
	return err
}

func setMySQLUserMonitorScopes(ctx context.Context, tx *sql.Tx, userID int64, role string, storeIDs []int64, expectedRevision ...string) error {
	return setMySQLUserMonitorScopesWithAudit(ctx, tx, userID, role, storeIDs, nil, expectedRevision...)
}

func setMySQLUserMonitorScopesWithAudit(ctx context.Context, tx *sql.Tx, userID int64, role string, storeIDs []int64, event *auditlog.AuditEvent, expectedRevision ...string) error {
	if storeIDs == nil && len(expectedRevision) == 0 {
		return nil
	}
	for _, id := range storeIDs {
		if id <= 0 || id > 9007199254740991 {
			return errors.New("机构 ID 必须为安全范围内的正整数")
		}
	}
	var lockedID int64
	var enabled bool
	var currentRole string
	if err := tx.QueryRowContext(ctx, `select u.id, u.enabled, coalesce((select r.code from tb_user_roles ur, tb_roles r
		where ur.user_id = u.id and r.id = ur.role_id order by case r.code when 'admin' then 1 when 'editor' then 2 else 3 end limit 1), 'viewer')
		from tb_users u where u.id = ? for update`, userID).Scan(&lockedID, &enabled, &currentRole); err != nil {
		return err
	}
	previous, err := readCRMMonitorScopes(ctx, tx, []int64{userID}, false)
	if err != nil {
		return err
	}
	if len(expectedRevision) > 0 && expectedRevision[0] != userMonitorScopeRevision(previous[userID].Tokens, currentRole, enabled) {
		return errMonitorScopeConflict
	}
	if storeIDs == nil {
		return nil
	}
	candidates, err := listCRMMonitorInstitutions(ctx, tx, nil, true)
	if err != nil {
		return fmt.Errorf("读取 CRM 机构失败，权限未保存: %w", err)
	}
	allowed := map[int64]bool{}
	for _, scope := range previous[userID].Scopes {
		allowed[scope.StoreID] = true
	}
	for _, scope := range candidates {
		allowed[scope.StoreID] = true
	}
	ids := uniquePositiveInt64s(storeIDs)
	for _, id := range ids {
		if !allowed[id] {
			return fmt.Errorf("机构 %d 不在当前可选范围（机构须启用且已配置有效工控机），不能新增授权", id)
		}
	}
	annotateMonitorScopeAudit(event, previous[userID].Scopes, ids)
	if _, err := tx.ExecContext(ctx, `
		delete from tb_user_resource_scopes
		where user_id = ? and resource_type in (?, ?) and scope = ?
	`, userID, ResourceTypeStore, ResourceTypeCRMInstitution, ScopeMonitorView); err != nil {
		return err
	}
	for _, storeID := range ids {
		_, err := tx.ExecContext(ctx, `
			insert into tb_user_resource_scopes (user_id, resource_type, resource_id, external_key, scope)
			values (?, ?, ?, ?, ?)
		`, userID, ResourceTypeCRMInstitution, storeID, fmt.Sprint(storeID), ScopeMonitorView)
		if err != nil {
			return err
		}
	}
	return nil
}

func uniquePositiveInt64s(values []int64) []int64 {
	seen := map[int64]bool{}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func (s *MySQLStore) ListMonitorStoreScopeCandidates(ctx context.Context) ([]AuthUserResourceScope, error) {
	return listCRMMonitorInstitutions(ctx, s.db, nil, true)
}

func (s *MySQLStore) GetUserMonitorStoreScopes(ctx context.Context, userID int64) ([]AuthUserResourceScope, error) {
	scopes, err := readCRMMonitorScopes(ctx, s.db, []int64{userID}, true)
	if err != nil {
		return nil, err
	}
	return scopes[userID].Scopes, nil
}

func (s *MySQLStore) CanUserViewMonitorStore(ctx context.Context, user AuthUserRecord, externalOrgID string) (bool, error) {
	orgID := strings.TrimSpace(externalOrgID)
	id, valid := canonicalInstitutionID(orgID)
	if !valid {
		return false, nil
	}
	var exists int
	if normalizeRole(user.Role) != RoleViewer {
		err := s.db.QueryRowContext(ctx, "select 1 from tb_crm_admin_tenant where id = ? and status = 1", id).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return err == nil, err
	}
	err := s.db.QueryRowContext(ctx, `
		select 1
		from tb_user_resource_scopes urs
		where user_id = ?
			and scope = ?
			and binary external_key = ?
			and (binary resource_type = ? or (binary resource_type = ? and resource_id = ?))
			and not exists (select 1 from tb_user_resource_scopes other
				where other.user_id = urs.user_id and other.scope = urs.scope
				and other.resource_type in ('store', 'crm_institution') and other.resource_type <> urs.resource_type)
			and exists (select 1 from tb_crm_admin_tenant t where t.id = ? and t.status = 1)
		limit 1
	`, user.ID, ScopeMonitorView, orgID, ResourceTypeStore, ResourceTypeCRMInstitution, id, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *MySQLStore) attachMonitorScopes(ctx context.Context, users []AuthUserRecord) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]int64, len(users))
	for i := range users {
		ids[i] = users[i].ID
	}
	scopesByUserID, err := readCRMMonitorScopes(ctx, s.db, ids, true)
	if err != nil {
		return err
	}
	for index := range users {
		snapshot := scopesByUserID[users[index].ID]
		users[index].MonitorStoreScopes = snapshot.Scopes
		users[index].MonitorScopeRevision = userMonitorScopeRevision(snapshot.Tokens, users[index].Role, users[index].Enabled)
		users[index].MonitorStoreScopeCount = len(users[index].MonitorStoreScopes)
	}
	return nil
}
