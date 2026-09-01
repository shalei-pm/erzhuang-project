-- DBA migration material for tb_auth_sessions.
-- This file is intentionally not executed by application startup.
-- Run the read-only preflight first, then execute only the applicable section.
-- Do not run all sections blindly against an existing table.

-- ============================================================================
-- 1. Read-only preflight
-- ============================================================================

select table_name, engine, table_collation
from information_schema.tables
where table_schema = database()
  and table_name = 'tb_auth_sessions';

select column_name, column_type, is_nullable, column_default, ordinal_position
from information_schema.columns
where table_schema = database()
  and table_name = 'tb_auth_sessions'
order by ordinal_position;

select index_name, non_unique, seq_in_index, column_name
from information_schema.statistics
where table_schema = database()
  and table_name = 'tb_auth_sessions'
order by index_name, seq_in_index;

-- ============================================================================
-- 2. Execute only when the preflight shows the table is absent
-- ============================================================================

create table if not exists tb_auth_sessions (
  id bigint not null auto_increment,
  session_token_hash char(64) not null,
  user_id bigint not null,
  sso_subject varchar(255) not null default '',
  ip_address varchar(64) not null default '',
  user_agent varchar(512) not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  last_activity_at datetime(3) not null,
  expires_at datetime(3) not null,
  revoked_at datetime(3) null,
  revoked_reason varchar(255) not null default '',
  primary key (id),
  unique key uq_tb_auth_sessions_token_hash (session_token_hash),
  key idx_tb_auth_sessions_user (user_id, created_at),
  key idx_tb_auth_sessions_user_activity (user_id, last_activity_at),
  key idx_tb_auth_sessions_expires_at (expires_at),
  constraint fk_tb_auth_sessions_user
    foreign key (user_id) references tb_users(id)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

-- ============================================================================
-- 3. Execute only when the preflight shows an existing table lacks the column
-- ============================================================================
-- The temporary NULL state is required so existing rows can be initialized.

alter table tb_auth_sessions
  add column last_activity_at datetime(3) null after created_at;

update tb_auth_sessions
set last_activity_at = created_at
where last_activity_at is null;

alter table tb_auth_sessions
  modify column last_activity_at datetime(3) not null;

-- If the column exists but is nullable, run only the UPDATE above (as needed)
-- and this MODIFY statement.

-- ============================================================================
-- 4. Execute only when the preflight shows a required index is absent
-- ============================================================================

alter table tb_auth_sessions
  add key idx_tb_auth_sessions_user_activity (user_id, last_activity_at);

alter table tb_auth_sessions
  add key idx_tb_auth_sessions_expires_at (expires_at);

-- The token-hash index must be unique. If the preflight shows it is absent,
-- first verify duplicate hashes, then execute this statement.
select session_token_hash, count(*) as duplicate_count
from tb_auth_sessions
group by session_token_hash
having count(*) > 1;

alter table tb_auth_sessions
  add unique key uq_tb_auth_sessions_token_hash (session_token_hash);
