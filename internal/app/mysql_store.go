package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) Name() string {
	return "mysql"
}

func (s *MySQLStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
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
	return users, rows.Err()
}

func (s *MySQLStore) CreateAuthUser(ctx context.Context, input AuthUserMutation) (AuthUserRecord, error) {
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
	if err := tx.Commit(); err != nil {
		return AuthUserRecord{}, err
	}
	return s.getAuthUserByID(ctx, id)
}

func (s *MySQLStore) UpdateAuthUser(ctx context.Context, id int64, input AuthUserMutation) (AuthUserRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AuthUserRecord{}, err
	}
	defer tx.Rollback()

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
		return AuthUserRecord{}, errAuthUserNotFound
	}
	if err := setMySQLUserRole(ctx, tx, id, normalizeRole(input.Role)); err != nil {
		return AuthUserRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return AuthUserRecord{}, err
	}
	return s.getAuthUserByID(ctx, id)
}

func (s *MySQLStore) getAuthUserByID(ctx context.Context, id int64) (AuthUserRecord, error) {
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
		where u.id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return AuthUserRecord{}, errAuthUserNotFound
	}
	return record, err
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
