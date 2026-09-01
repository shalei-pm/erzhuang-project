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
-- Run this whole file in a MySQL client that supports DELIMITER commands,
-- such as the mysql CLI. The procedure is removed after it is called.
-- All existing-table blockers are checked before ALTER/UPDATE statements;
-- MySQL DDL may implicitly commit, so this migration does not promise rollback.

drop procedure if exists erzhuang_migrate_tb_auth_sessions_tmp;

delimiter $$

create procedure erzhuang_migrate_tb_auth_sessions_tmp()
begin
  declare v_table_exists bigint default 0;
  declare v_users_exists bigint default 0;
  declare v_missing_columns bigint default 0;
  declare v_null_created_at bigint default 0;
  declare v_nullable_token_hash bigint default 0;
  declare v_null_token_hash bigint default 0;
  declare v_null_expires_at bigint default 0;
  declare v_column_exists bigint default 0;
  declare v_index_entries bigint default 0;
  declare v_index_matches bigint default 0;
  declare v_duplicate_hashes bigint default 0;
  declare v_primary_index_entries bigint default 0;
  declare v_primary_index_matches bigint default 0;
  declare v_user_index_entries bigint default 0;
  declare v_user_index_matches bigint default 0;
  declare v_token_index_missing bigint default 0;
  declare v_user_activity_index_missing bigint default 0;
  declare v_expiry_index_missing bigint default 0;

  select count(*) into v_table_exists
  from information_schema.tables
  where table_schema = database()
    and table_name = 'tb_auth_sessions';

  if v_table_exists = 0 then
    select count(*) into v_users_exists
    from information_schema.tables
    where table_schema = database()
      and table_name = 'tb_users';

    if v_users_exists = 0 then
      signal sqlstate '45000'
        set message_text = 'tb_users is required before creating tb_auth_sessions';
    end if;

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
    -- Patch branch: block before any ALTER if the existing base is incomplete.
    select count(*) into v_missing_columns
    from (
      select 'id' as column_name
      union all select 'session_token_hash'
      union all select 'user_id'
      union all select 'sso_subject'
      union all select 'ip_address'
      union all select 'user_agent'
      union all select 'created_at'
      union all select 'expires_at'
      union all select 'revoked_at'
      union all select 'revoked_reason'
    ) required_columns
    left join information_schema.columns c
      on c.table_schema = database()
      and c.table_name = 'tb_auth_sessions'
      and c.column_name = required_columns.column_name
    where c.column_name is null;

    if v_missing_columns > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions is missing required base columns; inspect information_schema.columns';
    end if;

    select count(*) into v_nullable_token_hash
    from information_schema.columns
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and column_name = 'session_token_hash'
      and is_nullable <> 'NO';

    if v_nullable_token_hash > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions.session_token_hash must be NOT NULL before migration';
    end if;

    select count(*) into v_null_created_at
    from tb_auth_sessions
    where created_at is null;

    if v_null_created_at > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions contains rows with NULL created_at; repair data before migration';
    end if;

    select count(*) into v_null_token_hash
    from tb_auth_sessions
    where session_token_hash is null;

    if v_null_token_hash > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions contains rows with NULL session_token_hash; repair data before migration';
    end if;

    select count(*) into v_null_expires_at
    from tb_auth_sessions
    where expires_at is null;

    if v_null_expires_at > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions contains rows with NULL expires_at; repair data before migration';
    end if;

    select count(*) into v_duplicate_hashes
    from (
      select session_token_hash
      from tb_auth_sessions
      group by session_token_hash
      having count(*) > 1
    ) duplicate_hashes;

    if v_duplicate_hashes > 0 then
      signal sqlstate '45000'
        set message_text = 'duplicate session_token_hash values must be repaired before migration';
    end if;

    select count(*), coalesce(sum(
      case when non_unique = 0
        and index_type = 'BTREE'
        and (sub_part = 0 or sub_part is null)
        and seq_in_index = 1
        and column_name = 'id'
        then 1 else 0 end), 0)
      into v_primary_index_entries, v_primary_index_matches
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'PRIMARY';

    if v_primary_index_entries <> 1 or v_primary_index_matches <> 1 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions PRIMARY index must be a single BTREE on id without a prefix';
    end if;

    select count(*), coalesce(sum(
      case when non_unique = 1
        and index_type = 'BTREE'
        and (sub_part = 0 or sub_part is null)
        and ((seq_in_index = 1 and column_name = 'user_id')
          or (seq_in_index = 2 and column_name = 'created_at'))
        then 1 else 0 end), 0)
      into v_user_index_entries, v_user_index_matches
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'idx_tb_auth_sessions_user';

    if v_user_index_entries > 0
      and (v_user_index_entries <> 2 or v_user_index_matches <> 2) then
      signal sqlstate '45000'
        set message_text = 'idx_tb_auth_sessions_user has the wrong uniqueness, BTREE type, prefix, or column order';
    end if;

    -- Validate existing named indexes before any ALTER or UPDATE. A wrong
    -- definition is blocked rather than silently treated as missing.
    select count(*), coalesce(sum(
      case when non_unique = 0
        and index_type = 'BTREE'
        and (sub_part = 0 or sub_part is null)
        and seq_in_index = 1
        and column_name = 'session_token_hash'
        then 1 else 0 end), 0)
      into v_index_entries, v_index_matches
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'uq_tb_auth_sessions_token_hash';

    if v_index_entries = 0 then
      set v_token_index_missing = 1;
    elseif v_index_entries <> 1 or v_index_matches <> 1 then
      signal sqlstate '45000'
        set message_text = 'uq_tb_auth_sessions_token_hash has the wrong uniqueness, BTREE type, prefix, or column order';
    end if;

    select count(*), coalesce(sum(
      case when non_unique = 1
        and index_type = 'BTREE'
        and (sub_part = 0 or sub_part is null)
        and ((seq_in_index = 1 and column_name = 'user_id')
          or (seq_in_index = 2 and column_name = 'last_activity_at'))
        then 1 else 0 end), 0)
      into v_index_entries, v_index_matches
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'idx_tb_auth_sessions_user_activity';

    if v_index_entries = 0 then
      set v_user_activity_index_missing = 1;
    elseif v_index_entries <> 2 or v_index_matches <> 2 then
      signal sqlstate '45000'
        set message_text = 'idx_tb_auth_sessions_user_activity has the wrong uniqueness, BTREE type, prefix, or column order';
    end if;

    select count(*), coalesce(sum(
      case when non_unique = 1
        and index_type = 'BTREE'
        and (sub_part = 0 or sub_part is null)
        and seq_in_index = 1
        and column_name = 'expires_at'
        then 1 else 0 end), 0)
      into v_index_entries, v_index_matches
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'idx_tb_auth_sessions_expires_at';

    if v_index_entries = 0 then
      set v_expiry_index_missing = 1;
    elseif v_index_entries <> 1 or v_index_matches <> 1 then
      signal sqlstate '45000'
        set message_text = 'idx_tb_auth_sessions_expires_at has the wrong uniqueness, BTREE type, prefix, or column order';
    end if;

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

    if v_token_index_missing = 1 then
      alter table tb_auth_sessions
        add unique key uq_tb_auth_sessions_token_hash (session_token_hash);
    end if;

    if v_user_activity_index_missing = 1 then
      alter table tb_auth_sessions
        add key idx_tb_auth_sessions_user_activity (user_id, last_activity_at);
    end if;

    if v_expiry_index_missing = 1 then
      alter table tb_auth_sessions
        add key idx_tb_auth_sessions_expires_at (expires_at);
    end if;
  end if;
end$$

call erzhuang_migrate_tb_auth_sessions_tmp()$$

drop procedure erzhuang_migrate_tb_auth_sessions_tmp$$

delimiter ;
