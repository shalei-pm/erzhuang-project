-- DBA migration for tb_auth_sessions.
-- The preflight queries are read-only. The procedure below is idempotent for
-- the table, column, and index existence cases described by the preflight.
-- Application startup does not execute this migration.

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
-- 2. Conditional create/patch migration
-- ============================================================================
-- The temporary procedure is dropped after the call. Run this whole file in a
-- MySQL client that supports DELIMITER commands, such as the mysql CLI.

drop procedure if exists erzhuang_migrate_tb_auth_sessions_tmp;

delimiter $$

create procedure erzhuang_migrate_tb_auth_sessions_tmp()
begin
  declare v_table_exists bigint default 0;
  declare v_column_exists bigint default 0;
  declare v_index_exists bigint default 0;

  select count(*) into v_table_exists
  from information_schema.tables
  where table_schema = database()
    and table_name = 'tb_auth_sessions';

  if v_table_exists = 0 then
    -- Create branch: all required fields and indexes are created together.
    create table tb_auth_sessions (
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
  else
    -- Patch branch: add and initialize the activity column before NOT NULL.
    select count(*) into v_column_exists
    from information_schema.columns
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and column_name = 'last_activity_at';

    if v_column_exists = 0 then
      alter table tb_auth_sessions
        add column last_activity_at datetime(3) null after created_at;
    end if;

    update tb_auth_sessions
    set last_activity_at = created_at
    where last_activity_at is null;

    alter table tb_auth_sessions
      modify column last_activity_at datetime(3) not null;

    select count(*) into v_index_exists
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'idx_tb_auth_sessions_user_activity';

    if v_index_exists = 0 then
      alter table tb_auth_sessions
        add key idx_tb_auth_sessions_user_activity (user_id, last_activity_at);
    end if;

    select count(*) into v_index_exists
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'idx_tb_auth_sessions_expires_at';

    if v_index_exists = 0 then
      alter table tb_auth_sessions
        add key idx_tb_auth_sessions_expires_at (expires_at);
    end if;

    select count(*) into v_index_exists
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'uq_tb_auth_sessions_token_hash';

    if v_index_exists = 0 then
      alter table tb_auth_sessions
        add unique key uq_tb_auth_sessions_token_hash (session_token_hash);
    end if;
  end if;
end$$

call erzhuang_migrate_tb_auth_sessions_tmp()$$

drop procedure erzhuang_migrate_tb_auth_sessions_tmp$$

delimiter ;
