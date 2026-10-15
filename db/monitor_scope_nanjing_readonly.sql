-- Read-only diagnosis for the original monitor permission candidates.
-- Run through the approved operations channel against db_pm_erzhuang.
-- No schema, store records, or user permissions are changed by this file.
SELECT DATABASE() AS database_name;

SELECT target.tenant_id,
       t.name AS tenant_name,
       t.city_id,
       t.status AS tenant_status,
       s.id AS internal_store_id,
       s.name AS legacy_store_name,
       s.city AS legacy_city,
       s.external_org_id,
       (s.external_org_id = CAST(target.tenant_id AS CHAR)) AS raw_id_match,
       EXISTS (
         SELECT 1 FROM tb_crm_iot_device d
         WHERE d.tenant_id = target.tenant_id AND d.category = 'edge'
           AND d.status = 1 AND d.deleted_at IS NULL
       ) AS has_active_edge,
       EXISTS (
         SELECT 1 FROM tb_crm_iot_device d
         WHERE d.tenant_id = target.tenant_id AND d.category = 'camera'
           AND d.provider = 'HikVisionNvrChannel'
           AND d.status = 1 AND d.deleted_at IS NULL
       ) AS has_active_nvr_camera
FROM (
  SELECT 10030 AS tenant_id UNION ALL SELECT 10053 UNION ALL SELECT 10086
) target
LEFT JOIN tb_crm_admin_tenant t ON t.id = target.tenant_id
LEFT JOIN tb_stores s ON TRIM(s.external_org_id) = CAST(target.tenant_id AS CHAR)
ORDER BY target.tenant_id, s.id;

-- An absent internal_store_id above means no matching legacy authorization row.
-- Existing rows may instead be hidden by a different legacy city value.
-- Detect duplicate mappings before preparing any repair statements.
SELECT TRIM(external_org_id) AS external_org_id, COUNT(*) AS mapping_count
FROM tb_stores
WHERE TRIM(external_org_id) IN ('10030', '10053', '10086')
GROUP BY TRIM(external_org_id)
HAVING COUNT(*) > 1;

-- Count consistency problems without returning user identity or permission rows.
SELECT COUNT(*) AS inconsistent_scope_count
FROM tb_user_resource_scopes u
LEFT JOIN tb_stores s ON s.id = u.resource_id
WHERE u.resource_type = 'store' AND u.scope = 'monitor:view'
  AND TRIM(u.external_key) IN ('10030', '10053', '10086')
  AND (s.id IS NULL OR TRIM(u.external_key) <> TRIM(s.external_org_id));

-- Check whether this affects other stores in the current resource-list scope.
SELECT COUNT(*) AS active_resource_orgs_without_permission_mapping
FROM tb_crm_admin_tenant t
WHERE t.status = 1
  AND EXISTS (
    SELECT 1 FROM tb_crm_iot_device d
    WHERE d.tenant_id = t.id AND d.category = 'edge'
      AND d.status = 1 AND d.deleted_at IS NULL
  )
  AND NOT EXISTS (
    SELECT 1 FROM tb_stores s
    WHERE TRIM(s.external_org_id) = CAST(t.id AS CHAR)
  );
