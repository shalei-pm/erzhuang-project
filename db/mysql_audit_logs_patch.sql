-- DBA 仅供核对后执行：为既有 tb_audit_logs 增加审计操作者展示名。
-- 本文件不由应用自动执行。

alter table tb_audit_logs
  add column actor_display_name varchar(255) not null default '' after user_id;
