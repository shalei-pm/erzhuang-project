create table if not exists tasks (
  id integer primary key,
  title text not null,
  done boolean not null default false
);

insert into tasks (id, title, done) values
  (1, '学习 Codex 本地开发', true),
  (2, '用 Git 管理版本', false),
  (3, '部署到腾讯云 Lighthouse', true),
  (4, '接入 Supabase PostgreSQL', false)
on conflict (id) do nothing;
