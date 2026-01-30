-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
  v_missing integer;
BEGIN
  IF to_regclass('orgunit.setid_scope_packages') IS NULL THEN
    RAISE NOTICE 'skip package_code backfill: orgunit.setid_scope_packages missing';
    RETURN;
  END IF;

  EXECUTE $SQL$
UPDATE jobcatalog.job_family_groups g
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE g.tenant_id = p.tenant_id
  AND g.package_id = p.package_id
  AND g.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_family_group_events e
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE e.tenant_id = p.tenant_id
  AND e.package_id = p.package_id
  AND e.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_family_group_versions v
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE v.tenant_id = p.tenant_id
  AND v.package_id = p.package_id
  AND v.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_families f
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE f.tenant_id = p.tenant_id
  AND f.package_id = p.package_id
  AND f.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_family_events e
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE e.tenant_id = p.tenant_id
  AND e.package_id = p.package_id
  AND e.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_family_versions v
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE v.tenant_id = p.tenant_id
  AND v.package_id = p.package_id
  AND v.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_levels l
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE l.tenant_id = p.tenant_id
  AND l.package_id = p.package_id
  AND l.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_level_events e
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE e.tenant_id = p.tenant_id
  AND e.package_id = p.package_id
  AND e.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_level_versions v
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE v.tenant_id = p.tenant_id
  AND v.package_id = p.package_id
  AND v.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_profiles j
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE j.tenant_id = p.tenant_id
  AND j.package_id = p.package_id
  AND j.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_profile_events e
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE e.tenant_id = p.tenant_id
  AND e.package_id = p.package_id
  AND e.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_profile_versions v
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE v.tenant_id = p.tenant_id
  AND v.package_id = p.package_id
  AND v.package_code IS NULL;
$SQL$;

  EXECUTE $SQL$
UPDATE jobcatalog.job_profile_version_job_families jvf
SET package_code = p.package_code
FROM orgunit.setid_scope_packages p
WHERE jvf.tenant_id = p.tenant_id
  AND jvf.package_id = p.package_id
  AND jvf.package_code IS NULL;
$SQL$;

  SELECT count(*) INTO v_missing
  FROM (
    SELECT 1 FROM jobcatalog.job_family_groups WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_family_group_events WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_family_group_versions WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_families WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_family_events WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_family_versions WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_levels WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_level_events WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_level_versions WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_profiles WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_profile_events WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_profile_versions WHERE package_code IS NULL AND package_id IS NOT NULL
    UNION ALL SELECT 1 FROM jobcatalog.job_profile_version_job_families WHERE package_code IS NULL AND package_id IS NOT NULL
  ) t;

  IF v_missing > 0 THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'PACKAGE_CODE_BACKFILL_MISSING',
      DETAIL = format('rows=%s', v_missing);
  END IF;
END
$$;
-- +goose StatementEnd
