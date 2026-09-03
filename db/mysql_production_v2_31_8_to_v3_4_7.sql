-- Baseline: production application version 2.31.8 (commit c95545a).
-- Target: current 3.4.7 database requirements.
-- Scope: additive only. No new table, data deletion, or tb_crm_* change.

-- 1. Enforce the 30-minute idle-session timeout.
alter table tb_auth_sessions
  add column last_activity_at datetime(3) null after created_at;

update tb_auth_sessions
set last_activity_at = created_at
where last_activity_at is null;

alter table tb_auth_sessions
  modify column last_activity_at datetime(3) not null,
  add key idx_tb_auth_sessions_user_activity (user_id, last_activity_at);

-- 2. Display the SSO nickname in audit logs.
alter table tb_audit_logs
  add column actor_display_name varchar(255) not null default '' after user_id;

-- Post-change verification.
select table_name, column_name, column_type, is_nullable, column_default
from information_schema.columns
where table_schema = database()
  and (
    (table_name = 'tb_auth_sessions' and column_name = 'last_activity_at')
    or (table_name = 'tb_audit_logs' and column_name = 'actor_display_name')
  )
order by table_name, ordinal_position;

select table_name, index_name, non_unique, seq_in_index, column_name
from information_schema.statistics
where table_schema = database()
  and table_name = 'tb_auth_sessions'
  and index_name = 'idx_tb_auth_sessions_user_activity'
order by seq_in_index;
