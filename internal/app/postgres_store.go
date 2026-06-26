package app

import (
	"context"
	"database/sql"

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
