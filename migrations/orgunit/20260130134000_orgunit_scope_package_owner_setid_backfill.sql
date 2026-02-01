-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
  v_multi_count integer;
  v_missing_count integer;
  v_version_missing_count integer;
BEGIN
  WITH subs AS (
    SELECT s.tenant_uuid,
           s.package_id,
           array_agg(DISTINCT s.setid) AS setids,
           count(DISTINCT s.setid) AS setid_count
    FROM orgunit.setid_scope_subscriptions s
    WHERE s.package_owner_tenant_uuid = s.tenant_uuid
    GROUP BY s.tenant_uuid, s.package_id
  )
  UPDATE orgunit.setid_scope_packages p
  SET owner_setid = subs.setids[1]
  FROM subs
  WHERE p.tenant_uuid = subs.tenant_uuid
    AND p.package_id = subs.package_id
    AND subs.setid_count = 1
    AND p.owner_setid IS NULL;

  SELECT count(*) INTO v_multi_count
  FROM (
    SELECT p.tenant_uuid, p.package_id
    FROM orgunit.setid_scope_packages p
    JOIN orgunit.setid_scope_subscriptions s
      ON s.tenant_uuid = p.tenant_uuid
     AND s.package_id = p.package_id
     AND s.package_owner_tenant_uuid = s.tenant_uuid
    WHERE p.owner_setid IS NULL
    GROUP BY p.tenant_uuid, p.package_id
    HAVING count(DISTINCT s.setid) > 1
  ) t;

  IF v_multi_count > 0 THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'OWNER_SETID_BACKFILL_MULTI_SUBSCRIBERS',
      DETAIL = format('packages=%s', v_multi_count);
  END IF;

  SELECT count(*) INTO v_missing_count
  FROM orgunit.setid_scope_packages p
  WHERE p.owner_setid IS NULL;

  IF v_missing_count > 0 THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'OWNER_SETID_BACKFILL_MISSING',
      DETAIL = format('packages=%s', v_missing_count);
  END IF;

  UPDATE orgunit.setid_scope_package_versions v
  SET owner_setid = p.owner_setid
  FROM orgunit.setid_scope_packages p
  WHERE v.tenant_uuid = p.tenant_uuid
    AND v.package_id = p.package_id
    AND v.owner_setid IS NULL;

  SELECT count(*) INTO v_version_missing_count
  FROM orgunit.setid_scope_package_versions v
  WHERE v.owner_setid IS NULL;

  IF v_version_missing_count > 0 THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'OWNER_SETID_BACKFILL_VERSION_MISSING',
      DETAIL = format('versions=%s', v_version_missing_count);
  END IF;
END
$$;

ALTER TABLE orgunit.setid_scope_packages
  ALTER COLUMN owner_setid SET NOT NULL;

ALTER TABLE orgunit.setid_scope_package_versions
  ALTER COLUMN owner_setid SET NOT NULL;
-- +goose StatementEnd
