-- Target: db_pm_erzhuang, institution 10086 only. No DDL or permission writes.
-- Operations must review the read-only diagnosis first and pause store writes.
-- Production data has NOT been queried by Codex; test internal IDs are not used.
-- Stop on any error, warning, duplicate mapping, or name conflict and ROLLBACK.
USE db_pm_erzhuang;
SET NAMES utf8mb4;

-- Must be InnoDB, no unreviewed trigger; inspect before starting the transaction.
SELECT ENGINE FROM information_schema.TABLES
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='tb_stores';
SELECT TRIGGER_NAME FROM information_schema.TRIGGERS
WHERE TRIGGER_SCHEMA=DATABASE() AND EVENT_OBJECT_TABLE='tb_stores';

START TRANSACTION;
-- Expected exactly: 10086 / 南京金陵中环店 / 162 / 1.
SELECT id,name,city_id,status FROM tb_crm_admin_tenant
WHERE id=10086 LOCK IN SHARE MODE;

-- First run: no rows. Rerun: one exact mapping only. Otherwise ROLLBACK.
SELECT id,external_org_id,city,name,normalized_name
FROM tb_stores
WHERE BINARY TRIM(external_org_id)=BINARY '10086'
   OR normalized_name='南京金陵中环'
   OR name='南京金陵中环店'
FOR UPDATE;

-- Preserve these rows and all existing user permissions.
SELECT * FROM tb_stores
WHERE BINARY TRIM(external_org_id) IN (BINARY '10030',BINARY '10053')
ORDER BY id;

INSERT INTO tb_stores
  (city,name,short_name,normalized_name,external_org_id,
   design_plan_status,overall_status)
SELECT '南京','南京金陵中环店','','南京金陵中环','10086',
       'not_uploaded','partial'
FROM tb_crm_admin_tenant t
WHERE t.id=10086 AND t.city_id=162 AND t.status=1
  AND BINARY t.name=BINARY '南京金陵中环店'
  AND NOT EXISTS (
    SELECT 1 FROM tb_stores s
    WHERE BINARY TRIM(s.external_org_id)=BINARY '10086'
  )
  AND NOT EXISTS (
    SELECT 1 FROM tb_stores s
    WHERE s.normalized_name='南京金陵中环' OR s.name='南京金陵中环店'
  );
SELECT ROW_COUNT() AS inserted_rows;
SHOW WARNINGS;

-- Exactly one canonical target mapping; record the actual generated internal ID.
SELECT id,external_org_id,city,name,short_name,normalized_name,
       design_plan_status,overall_status,created_at,updated_at
FROM tb_stores WHERE BINARY TRIM(external_org_id)=BINARY '10086';
SELECT * FROM tb_stores
WHERE BINARY TRIM(external_org_id) IN (BINARY '10030',BINARY '10053')
ORDER BY id;

-- First run inserted_rows=1; rerun=0 is valid only for an exact existing mapping.
-- No warnings, no conflicts, original rows unchanged: separately execute COMMIT;
-- Any unexpected result: separately execute ROLLBACK;
-- Do not leave this transaction open while waiting for approval.
