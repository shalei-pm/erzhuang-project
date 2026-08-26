-- NVR default-thumbnail backfill owned table.
-- Run through the company DBA change process in test and production separately.
-- The Web service and one-shot runner must never create or alter this table.

create table tb_nvr_camera_snapshots (
  id bigint unsigned not null auto_increment,
  camera_id bigint unsigned not null,
  tenant_id bigint unsigned not null,
  status varchar(32) character set ascii collate ascii_bin not null,
  oss_object_key varchar(512) character set ascii collate ascii_bin not null default '',
  content_type varchar(64) character set ascii collate ascii_bin not null default '',
  width int unsigned not null default 0,
  height int unsigned not null default 0,
  byte_size int unsigned not null default 0,
  captured_at datetime(3) null,
  attempted_at datetime(3) not null,
  error_code varchar(64) character set ascii collate ascii_bin not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),

  primary key (id),
  unique key uq_tb_nvr_camera_snapshots_camera (camera_id),
  key idx_tb_nvr_camera_snapshots_tenant_status_attempted
    (tenant_id, status, attempted_at, camera_id),
  key idx_tb_nvr_camera_snapshots_tenant_updated
    (tenant_id, updated_at, camera_id),

  constraint chk_tb_nvr_camera_snapshots_ids
    check (camera_id > 0 and tenant_id > 0),
  constraint chk_tb_nvr_camera_snapshots_status
    check (status in (
      'succeeded',
      'authorization_failed',
      'wss_connect_failed',
      'wss_connect_timeout',
      'media_timeout',
      'demux_failed',
      'decode_failed',
      'thumbnail_invalid',
      'oss_upload_failed'
    )),
  constraint chk_tb_nvr_camera_snapshots_payload
    check (
      (
        status = 'succeeded'
        and oss_object_key = concat(
          'nvr-camera-snapshots/', tenant_id, '/', camera_id, '.jpg'
        )
        and content_type = 'image/jpeg'
        and width between 1 and 640
        and height between 1 and 640
        and byte_size between 1 and 1048576
        and captured_at is not null
        and error_code = ''
      )
      or
      (
        status <> 'succeeded'
        and oss_object_key = ''
        and content_type = ''
        and width = 0
        and height = 0
        and byte_size = 0
        and captured_at is null
        and error_code = status
      )
    )
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

-- Test-environment validation after DDL and each scoped Job run.
select index_name, non_unique, seq_in_index, column_name
from information_schema.statistics
where table_schema = database()
  and table_name = 'tb_nvr_camera_snapshots'
order by index_name, seq_in_index;

select status, count(*) as row_count
from tb_nvr_camera_snapshots
where tenant_id = 10001
group by status;

select count(*) as invalid_success_rows
from tb_nvr_camera_snapshots
where status = 'succeeded' and (
  oss_object_key <> concat('nvr-camera-snapshots/', tenant_id, '/', camera_id, '.jpg')
  or content_type <> 'image/jpeg'
  or width not between 1 and 640
  or height not between 1 and 640
  or byte_size not between 1 and 1048576
  or captured_at is null
  or error_code <> ''
);

select count(*) as invalid_failure_rows
from tb_nvr_camera_snapshots
where status <> 'succeeded' and (
  oss_object_key <> '' or content_type <> '' or width <> 0
  or height <> 0 or byte_size <> 0 or captured_at is not null
  or error_code <> status
);

-- Rollback order: stop/delete the temporary Job, roll back the Web read path,
-- retain rows and private objects for audit. The following is test-only and
-- requires explicit data-owner approval; it does not delete OSS objects.
-- delete from tb_nvr_camera_snapshots where tenant_id = 10001;
-- drop table tb_nvr_camera_snapshots;
