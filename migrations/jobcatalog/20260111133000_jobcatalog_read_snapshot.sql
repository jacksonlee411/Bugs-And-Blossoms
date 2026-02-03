-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION jobcatalog.get_job_catalog_snapshot(
  p_tenant_uuid uuid,
  p_setid text,
  p_query_date date
)
RETURNS TABLE (
  groups jsonb,
  families jsonb,
  levels jsonb,
  profiles jsonb
)
LANGUAGE plpgsql
AS $$
DECLARE
  v_setid text;
BEGIN
  PERFORM jobcatalog.assert_current_tenant(p_tenant_uuid);
  IF p_query_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'query_date is required';
  END IF;

  v_setid := jobcatalog.normalize_setid(p_setid);

  RETURN QUERY
  SELECT
    COALESCE((
      SELECT jsonb_agg(
        jsonb_build_object(
          'job_family_group_uuid', g.job_family_group_uuid,
          'job_family_group_code', g.job_family_group_code,
          'name', v.name,
          'description', v.description,
          'is_active', v.is_active,
          'external_refs', v.external_refs,
          'valid_from', lower(v.validity),
          'valid_to_excl', upper(v.validity),
          'last_event_db_id', v.last_event_id
        )
        ORDER BY g.job_family_group_code
      )
      FROM jobcatalog.job_family_groups g
      JOIN jobcatalog.job_family_group_versions v
        ON v.tenant_uuid = p_tenant_uuid
       AND v.setid = v_setid
       AND v.job_family_group_uuid = g.job_family_group_uuid
       AND v.validity @> p_query_date
      WHERE g.tenant_uuid = p_tenant_uuid
        AND g.setid = v_setid
    ), '[]'::jsonb) AS groups,
    COALESCE((
      SELECT jsonb_agg(
        jsonb_build_object(
          'job_family_uuid', f.job_family_uuid,
          'job_family_code', f.job_family_code,
          'job_family_group_uuid', v.job_family_group_uuid,
          'name', v.name,
          'description', v.description,
          'is_active', v.is_active,
          'external_refs', v.external_refs,
          'valid_from', lower(v.validity),
          'valid_to_excl', upper(v.validity),
          'last_event_db_id', v.last_event_id
        )
        ORDER BY f.job_family_code
      )
      FROM jobcatalog.job_families f
      JOIN jobcatalog.job_family_versions v
        ON v.tenant_uuid = p_tenant_uuid
       AND v.setid = v_setid
       AND v.job_family_uuid = f.job_family_uuid
       AND v.validity @> p_query_date
      WHERE f.tenant_uuid = p_tenant_uuid
        AND f.setid = v_setid
    ), '[]'::jsonb) AS families,
    COALESCE((
      SELECT jsonb_agg(
        jsonb_build_object(
          'job_level_uuid', l.job_level_uuid,
          'job_level_code', l.job_level_code,
          'name', v.name,
          'description', v.description,
          'is_active', v.is_active,
          'external_refs', v.external_refs,
          'valid_from', lower(v.validity),
          'valid_to_excl', upper(v.validity),
          'last_event_db_id', v.last_event_id
        )
        ORDER BY l.job_level_code
      )
      FROM jobcatalog.job_levels l
      JOIN jobcatalog.job_level_versions v
        ON v.tenant_uuid = p_tenant_uuid
       AND v.setid = v_setid
       AND v.job_level_uuid = l.job_level_uuid
       AND v.validity @> p_query_date
      WHERE l.tenant_uuid = p_tenant_uuid
        AND l.setid = v_setid
    ), '[]'::jsonb) AS levels,
    COALESCE((
      SELECT jsonb_agg(
        jsonb_build_object(
          'job_profile_uuid', p.job_profile_uuid,
          'job_profile_code', p.job_profile_code,
          'name', v.name,
          'description', v.description,
          'is_active', v.is_active,
          'external_refs', v.external_refs,
          'valid_from', lower(v.validity),
          'valid_to_excl', upper(v.validity),
          'last_event_db_id', v.last_event_id,
          'job_family_uuids', COALESCE(fam.job_family_uuids, '[]'::jsonb),
          'primary_job_family_uuid', fam.primary_job_family_uuid
        )
        ORDER BY p.job_profile_code
      )
      FROM jobcatalog.job_profiles p
      JOIN jobcatalog.job_profile_versions v
        ON v.tenant_uuid = p_tenant_uuid
       AND v.setid = v_setid
       AND v.job_profile_uuid = p.job_profile_uuid
       AND v.validity @> p_query_date
      LEFT JOIN LATERAL (
        SELECT
          jsonb_agg(f.job_family_uuid ORDER BY f.job_family_uuid) AS job_family_uuids,
          (
            SELECT f2.job_family_uuid
            FROM jobcatalog.job_profile_version_job_families f2
            WHERE f2.tenant_uuid = p_tenant_uuid
              AND f2.setid = v_setid
              AND f2.job_profile_version_id = v.id
              AND f2.is_primary = true
            LIMIT 1
          ) AS primary_job_family_uuid
        FROM jobcatalog.job_profile_version_job_families f
        WHERE f.tenant_uuid = p_tenant_uuid
          AND f.setid = v_setid
          AND f.job_profile_version_id = v.id
      ) fam ON true
      WHERE p.tenant_uuid = p_tenant_uuid
        AND p.setid = v_setid
    ), '[]'::jsonb) AS profiles;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS jobcatalog.get_job_catalog_snapshot(uuid, text, date);
-- +goose StatementEnd
