create table if not exists tb_tasks (
  id int not null,
  title varchar(255) not null,
  done tinyint(1) not null default 0,
  primary key (id)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

insert ignore into tb_tasks (id, title, done) values
  (1, '学习 Codex 本地开发', 1),
  (2, '用 Git 管理版本', 0),
  (3, '部署到腾讯云 Lighthouse', 1),
  (4, '接入 Supabase PostgreSQL', 0);

create table if not exists tb_app_settings (
  `key` varchar(191) not null,
  value text not null,
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (`key`)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_design_plan_stores (
  id bigint not null auto_increment,
  name varchar(255) not null,
  normalized_name varchar(255) not null,
  pdf_file_name varchar(512) not null default '',
  original_pdf_path varchar(1024) not null default '',
  preview_image_path varchar(1024) not null default '',
  thumbnail_path varchar(1024) not null default '',
  page_count int not null default 0,
  status varchar(32) not null default 'completed',
  recognition_result json null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_design_plan_stores_normalized_name (normalized_name),
  key idx_tb_design_plan_stores_updated_at (updated_at),
  constraint chk_tb_design_plan_stores_status
    check (status in ('completed', 'needs_review', 'incomplete'))
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_design_plan_store_areas (
  id bigint not null auto_increment,
  store_id bigint not null,
  display_order int not null,
  name varchar(255) not null,
  area_type varchar(32) not null,
  area_number int null,
  confidence varchar(16) not null default 'high',
  needs_review tinyint(1) not null default 0,
  box_x decimal(18,10) not null,
  box_y decimal(18,10) not null,
  box_width decimal(18,10) not null,
  box_height decimal(18,10) not null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_design_plan_area_number (store_id, area_type, area_number),
  key idx_tb_design_plan_areas_store_order (store_id, display_order),
  constraint fk_tb_design_plan_areas_store
    foreign key (store_id) references tb_design_plan_stores(id) on delete cascade,
  constraint chk_tb_design_plan_areas_type
    check (area_type in ('treatment', 'vip_treatment', 'consultation', 'beauty')),
  constraint chk_tb_design_plan_areas_confidence
    check (confidence in ('high', 'medium', 'low')),
  constraint chk_tb_design_plan_areas_box
    check (
      box_x >= 0 and box_x <= 1 and
      box_y >= 0 and box_y <= 1 and
      box_width > 0 and box_width <= 1 and
      box_height > 0 and box_height <= 1 and
      box_x + box_width <= 1 and
      box_y + box_height <= 1
    )
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_design_plan_operation_logs (
  id bigint not null auto_increment,
  action varchar(32) not null,
  store_id bigint null,
  store_name varchar(255) not null,
  actor varchar(128) not null default 'admin',
  summary text not null,
  created_at datetime(3) not null default current_timestamp(3),
  primary key (id),
  key idx_tb_design_plan_logs_store_id (store_id, created_at),
  constraint chk_tb_design_plan_logs_action
    check (action in ('create', 'update', 'delete', 'replace'))
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_stores (
  id bigint not null auto_increment,
  city varchar(128) not null default '',
  name varchar(255) not null,
  short_name varchar(255) not null default '',
  normalized_name varchar(255) not null,
  external_org_id varchar(255) not null default '',
  design_plan_status varchar(32) not null default 'not_uploaded',
  overall_status varchar(32) not null default 'partial',
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_stores_normalized_name (normalized_name),
  key idx_tb_stores_updated_at (updated_at),
  constraint chk_tb_stores_design_plan_status
    check (design_plan_status in ('not_uploaded', 'pending_recognition', 'pending_annotation', 'completed')),
  constraint chk_tb_stores_overall_status
    check (overall_status in ('incomplete', 'partial', 'completed', 'exception'))
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_store_areas (
  id bigint not null auto_increment,
  store_id bigint not null,
  area_type varchar(32) not null,
  area_number int not null,
  display_name varchar(255) not null,
  source varchar(32) not null default 'manual',
  status varchar(32) not null default 'confirmed',
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_store_areas_number (store_id, area_type, area_number),
  constraint fk_tb_store_areas_store
    foreign key (store_id) references tb_stores(id) on delete cascade,
  constraint chk_tb_store_areas_type
    check (area_type in ('treatment', 'vip_treatment', 'consultation', 'beauty')),
  constraint chk_tb_store_areas_source
    check (source in ('manual', 'design_plan', 'video_channel', 'multiple')),
  constraint chk_tb_store_areas_status
    check (status in ('candidate', 'confirmed')),
  constraint chk_tb_store_areas_number
    check ((area_type = 'vip_treatment' and area_number >= 0) or (area_type <> 'vip_treatment' and area_number > 0))
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_store_design_plans (
  id bigint not null auto_increment,
  store_id bigint not null,
  upload_id varchar(255) not null default '',
  pdf_file_name varchar(512) not null default '',
  original_pdf_path varchar(1024) not null default '',
  preview_image_path varchar(1024) not null default '',
  thumbnail_path varchar(1024) not null default '',
  page_count int not null default 0,
  recognition_status varchar(32) not null default 'not_started',
  recognition_result json null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  key idx_tb_store_design_plans_store_id (store_id),
  constraint fk_tb_store_design_plans_store
    foreign key (store_id) references tb_stores(id) on delete cascade,
  constraint chk_tb_store_design_plans_status
    check (recognition_status in ('not_started', 'running', 'failed', 'completed'))
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_design_plan_annotations (
  id bigint not null auto_increment,
  design_plan_id bigint not null,
  area_id bigint not null,
  box_x decimal(18,10) not null,
  box_y decimal(18,10) not null,
  box_width decimal(18,10) not null,
  box_height decimal(18,10) not null,
  status varchar(32) not null default 'pending',
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_design_plan_annotations_area (design_plan_id, area_id),
  key idx_tb_design_plan_annotations_area_id (area_id),
  constraint fk_tb_design_plan_annotations_plan
    foreign key (design_plan_id) references tb_store_design_plans(id) on delete cascade,
  constraint fk_tb_design_plan_annotations_area
    foreign key (area_id) references tb_store_areas(id) on delete cascade,
  constraint chk_tb_design_plan_annotations_status
    check (status in ('pending', 'confirmed')),
  constraint chk_tb_design_plan_annotations_box
    check (
      box_x >= 0 and box_x <= 1 and
      box_y >= 0 and box_y <= 1 and
      box_width > 0 and box_width <= 1 and
      box_height > 0 and box_height <= 1 and
      box_x + box_width <= 1 and
      box_y + box_height <= 1
    )
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_ezviz_accounts (
  id bigint not null auto_increment,
  account_name varchar(255) not null,
  app_key varchar(255) not null default '',
  app_secret_ciphertext text not null,
  access_token_ciphertext text not null,
  status varchar(32) not null default 'unverified',
  last_verified_at datetime(3) null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_ezviz_accounts_account_name (account_name),
  constraint chk_tb_ezviz_accounts_status
    check (status in ('unverified', 'available', 'unavailable'))
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_video_recorders (
  id bigint not null auto_increment,
  store_id bigint not null,
  ezviz_account_id bigint null,
  device_code varchar(255) not null,
  status varchar(32) not null default 'offline',
  effective_channel_count int not null default 0,
  last_scanned_at datetime(3) null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_video_recorders_device_code (device_code),
  key idx_tb_video_recorders_store_id (store_id),
  key idx_tb_video_recorders_ezviz_account_id (ezviz_account_id),
  constraint fk_tb_video_recorders_store
    foreign key (store_id) references tb_stores(id) on delete cascade,
  constraint fk_tb_video_recorders_ezviz_account
    foreign key (ezviz_account_id) references tb_ezviz_accounts(id),
  constraint chk_tb_video_recorders_status
    check (status in ('online', 'offline')),
  constraint chk_tb_video_recorders_channel_count
    check (effective_channel_count >= 0)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_video_channels (
  id bigint not null auto_increment,
  recorder_id bigint not null,
  channel_no int not null,
  channel_name varchar(255) not null default '',
  status varchar(32) not null default 'pending_recognition',
  is_active tinyint(1) not null default 1,
  scene_type varchar(32) not null default 'unknown',
  area_type varchar(32) null,
  area_number int null,
  bed_label varchar(64) not null default '',
  area_note text not null,
  area_id bigint null,
  recognition_attempts int not null default 0,
  recognition_result json null,
  confirmed_at datetime(3) null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_video_channels_channel (recorder_id, channel_no),
  key idx_tb_video_channels_area_id (area_id),
  constraint fk_tb_video_channels_recorder
    foreign key (recorder_id) references tb_video_recorders(id) on delete cascade,
  constraint fk_tb_video_channels_area
    foreign key (area_id) references tb_store_areas(id),
  constraint chk_tb_video_channels_status
    check (status in ('pending_recognition', 'pending_confirmation', 'confirmed_business', 'confirmed_non_business', 'recognition_failed', 'inactive')),
  constraint chk_tb_video_channels_scene_type
    check (scene_type in ('treatment', 'vip_treatment', 'consultation', 'beauty', 'front_desk', 'corridor', 'passage', 'waiting_area', 'hall', 'entrance', 'storage', 'pharmacy', 'machine_room', 'unknown')),
  constraint chk_tb_video_channels_area_type
    check (area_type is null or area_type in ('treatment', 'vip_treatment', 'consultation', 'beauty')),
  constraint chk_tb_video_channels_channel_no
    check (channel_no > 0),
  constraint chk_tb_video_channels_attempts
    check (recognition_attempts >= 0)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_channel_snapshots (
  id bigint not null auto_increment,
  channel_id bigint not null,
  thumbnail_path varchar(1024) not null default '',
  full_image_path varchar(1024) not null default '',
  full_image_expires_at datetime(3) null,
  created_at datetime(3) not null default current_timestamp(3),
  primary key (id),
  key idx_tb_channel_snapshots_channel_id (channel_id, created_at),
  constraint fk_tb_channel_snapshots_channel
    foreign key (channel_id) references tb_video_channels(id) on delete cascade
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_operation_logs (
  id bigint not null auto_increment,
  action varchar(64) not null,
  entity_type varchar(64) not null,
  entity_id bigint null,
  store_id bigint null,
  actor varchar(128) not null default 'admin',
  summary text not null,
  created_at datetime(3) not null default current_timestamp(3),
  primary key (id),
  key idx_tb_operation_logs_store_id (store_id, created_at)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;
