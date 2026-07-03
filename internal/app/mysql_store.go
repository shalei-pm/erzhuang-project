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
		select id, email, username, display_name, feishu_user_id, coalesce(nullif(phone, ''), mobile), role, enabled, last_login_at
		from tb_users
		where lower(email) = lower(?)
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
			phone = case when ? <> '' then ? else phone end,
			mobile = case when ? <> '' then ? else mobile end,
			last_login_at = current_timestamp(3),
			updated_at = current_timestamp(3)
		where lower(email) = lower(?)
	`,
		strings.TrimSpace(patch.Username), strings.TrimSpace(patch.Username),
		strings.TrimSpace(patch.DisplayName), strings.TrimSpace(patch.DisplayName),
		strings.TrimSpace(patch.FeishuUserID), strings.TrimSpace(patch.FeishuUserID),
		strings.TrimSpace(patch.Phone), strings.TrimSpace(patch.Phone),
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
		select id, email, username, display_name, feishu_user_id, coalesce(nullif(phone, ''), mobile), role, enabled, last_login_at
		from tb_users
		order by enabled desc, lower(email) asc
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
	result, err := s.db.ExecContext(ctx, `
		insert into tb_users (email, username, display_name, role, enabled)
		values (?, ?, ?, ?, ?)
	`,
		normalizeEmail(input.Email),
		strings.TrimSpace(input.Username),
		strings.TrimSpace(input.DisplayName),
		normalizeRole(input.Role),
		input.Enabled,
	)
	if err != nil {
		return AuthUserRecord{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return AuthUserRecord{}, err
	}
	return s.getAuthUserByID(ctx, id)
}

func (s *MySQLStore) UpdateAuthUser(ctx context.Context, id int64, input AuthUserMutation) (AuthUserRecord, error) {
	result, err := s.db.ExecContext(ctx, `
		update tb_users
		set username = ?,
			display_name = ?,
			role = ?,
			enabled = ?,
			updated_at = current_timestamp(3)
		where id = ?
	`, strings.TrimSpace(input.Username), strings.TrimSpace(input.DisplayName), normalizeRole(input.Role), input.Enabled, id)
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
	return s.getAuthUserByID(ctx, id)
}

func (s *MySQLStore) getAuthUserByID(ctx context.Context, id int64) (AuthUserRecord, error) {
	record, err := scanAuthUser(s.db.QueryRowContext(ctx, `
		select id, email, username, display_name, feishu_user_id, coalesce(nullif(phone, ''), mobile), role, enabled, last_login_at
		from tb_users
		where id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return AuthUserRecord{}, errAuthUserNotFound
	}
	return record, err
}
