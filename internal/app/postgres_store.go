package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/shalei-pm/erzhuang-project/internal/designplan"
	"github.com/shalei-pm/erzhuang-project/internal/storespace"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Name() string {
	return "postgres"
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *PostgresStore) ListTasks(ctx context.Context) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, title, done
		from tasks
		order by id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.Title, &task.Done); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (s *PostgresStore) GetAIProvider(ctx context.Context) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `
		select value
		from app_settings
		where key = 'ai_provider'
	`).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *PostgresStore) SetAIProvider(ctx context.Context, provider string) error {
	_, err := s.db.ExecContext(ctx, `
		insert into app_settings (key, value, updated_at)
		values ('ai_provider', $1, now())
		on conflict (key) do update
		set value = excluded.value,
			updated_at = excluded.updated_at
	`, NormalizeAIProvider(provider))
	return err
}

func (s *PostgresStore) GetAuthUserByEmail(ctx context.Context, email string) (AuthUserRecord, error) {
	record, err := scanAuthUser(s.db.QueryRowContext(ctx, `
		select id, email, username, display_name, feishu_user_id, phone, role, enabled, last_login_at
		from tb_users
		where lower(email) = lower($1)
	`, normalizeEmail(email)))
	if errors.Is(err, sql.ErrNoRows) {
		return AuthUserRecord{}, errAuthUserNotFound
	}
	return record, err
}

func (s *PostgresStore) UpdateAuthUserProfile(ctx context.Context, patch AuthUserPatch) (AuthUserRecord, error) {
	record, err := scanAuthUser(s.db.QueryRowContext(ctx, `
		update tb_users
		set username = case when $2 <> '' then $2 else username end,
			display_name = case when $3 <> '' then $3 else display_name end,
			feishu_user_id = case when $4 <> '' then $4 else feishu_user_id end,
			phone = case when $5 <> '' then $5 else phone end,
			last_login_at = now(),
			updated_at = now()
		where lower(email) = lower($1)
		returning id, email, username, display_name, feishu_user_id, phone, role, enabled, last_login_at
	`,
		normalizeEmail(patch.Email),
		strings.TrimSpace(patch.Username),
		strings.TrimSpace(patch.DisplayName),
		strings.TrimSpace(patch.FeishuUserID),
		strings.TrimSpace(patch.Phone),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return AuthUserRecord{}, errAuthUserNotFound
	}
	return record, err
}

func EnsurePostgresSchema(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`create table if not exists tasks (
			id integer primary key,
			title text not null,
			done boolean not null default false
		)`,
		`insert into tasks (id, title, done) values
			(1, '学习 Codex 本地开发', true),
			(2, '用 Git 管理版本', false),
			(3, '部署到腾讯云 Lighthouse', true),
			(4, '接入 Supabase PostgreSQL', false)
		on conflict (id) do nothing`,
		`alter table tasks enable row level security`,
		`drop policy if exists tasks_no_client_access on tasks`,
		`create policy tasks_no_client_access on tasks
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`create table if not exists app_settings (
			key text primary key,
			value text not null,
			updated_at timestamptz not null default now()
		)`,
		`alter table app_settings enable row level security`,
		`drop policy if exists app_settings_no_client_access on app_settings`,
		`create policy app_settings_no_client_access on app_settings
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`create table if not exists tb_users (
			id bigserial primary key,
			email text not null,
			username text not null default '',
			display_name text not null default '',
			feishu_user_id text not null default '',
			phone text not null default '',
			role text not null default 'viewer',
			enabled boolean not null default true,
			last_login_at timestamptz null,
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now()
		)`,
		`create unique index if not exists tb_users_email_lower_key on tb_users (lower(email))`,
		`create index if not exists tb_users_enabled_idx on tb_users (enabled)`,
		`insert into tb_users (email, username, display_name, role, enabled)
			select 'shalei@soyoung.com', 'shalei', '', 'admin', true
			where not exists (
				select 1 from tb_users where lower(email) = 'shalei@soyoung.com'
			)`,
		`alter table tb_users enable row level security`,
		`drop policy if exists tb_users_no_client_access on tb_users`,
		`create policy tb_users_no_client_access on tb_users
			for all to anon, authenticated
			using (false)
			with check (false)`,
	}

	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	if err := designplan.EnsurePostgresSchema(ctx, db); err != nil {
		return err
	}
	return storespace.EnsurePostgresSchema(ctx, db)
}
