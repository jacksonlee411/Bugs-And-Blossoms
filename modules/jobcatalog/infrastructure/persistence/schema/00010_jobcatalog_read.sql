CREATE OR REPLACE FUNCTION jobcatalog.get_job_catalog_snapshot(
  p_tenant_id uuid,
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
  PERFORM jobcatalog.assert_current_tenant(p_tenant_id);
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
          'job_family_group_id', g.id,
          'code', g.code,
          'name', v.name,
          'description', v.description,
          'is_active', v.is_active,
          'external_refs', v.external_refs,
          'valid_from', lower(v.validity),
          'valid_to_excl', upper(v.validity),
          'last_event_db_id', v.last_event_id
        )
        ORDER BY g.code
      )
      FROM jobcatalog.job_family_groups g
      JOIN jobcatalog.job_family_group_versions v
        ON v.tenant_id = p_tenant_id
       AND v.setid = v_setid
       AND v.job_family_group_id = g.id
       AND v.validity @> p_query_date
      WHERE g.tenant_id = p_tenant_id
        AND g.setid = v_setid
    ), '[]'::jsonb) AS groups,
    COALESCE((
      SELECT jsonb_agg(
        jsonb_build_object(
          'job_family_id', f.id,
          'code', f.code,
          'job_family_group_id', v.job_family_group_id,
          'name', v.name,
          'description', v.description,
          'is_active', v.is_active,
          'external_refs', v.external_refs,
          'valid_from', lower(v.validity),
          'valid_to_excl', upper(v.validity),
          'last_event_db_id', v.last_event_id
        )
        ORDER BY f.code
      )
      FROM jobcatalog.job_families f
      JOIN jobcatalog.job_family_versions v
        ON v.tenant_id = p_tenant_id
       AND v.setid = v_setid
       AND v.job_family_id = f.id
       AND v.validity @> p_query_date
      WHERE f.tenant_id = p_tenant_id
        AND f.setid = v_setid
    ), '[]'::jsonb) AS families,
    COALESCE((
      SELECT jsonb_agg(
        jsonb_build_object(
          'job_level_id', l.id,
          'code', l.code,
          'name', v.name,
          'description', v.description,
          'is_active', v.is_active,
          'external_refs', v.external_refs,
          'valid_from', lower(v.validity),
          'valid_to_excl', upper(v.validity),
          'last_event_db_id', v.last_event_id
        )
        ORDER BY l.code
      )
      FROM jobcatalog.job_levels l
      JOIN jobcatalog.job_level_versions v
        ON v.tenant_id = p_tenant_id
       AND v.setid = v_setid
       AND v.job_level_id = l.id
       AND v.validity @> p_query_date
      WHERE l.tenant_id = p_tenant_id
        AND l.setid = v_setid
    ), '[]'::jsonb) AS levels,
    COALESCE((
      SELECT jsonb_agg(
        jsonb_build_object(
          'job_profile_id', p.id,
          'code', p.code,
          'name', v.name,
          'description', v.description,
          'is_active', v.is_active,
          'external_refs', v.external_refs,
          'valid_from', lower(v.validity),
          'valid_to_excl', upper(v.validity),
          'last_event_db_id', v.last_event_id,
          'job_family_ids', COALESCE(fam.job_family_ids, '[]'::jsonb),
          'primary_job_family_id', fam.primary_job_family_id
        )
        ORDER BY p.code
      )
      FROM jobcatalog.job_profiles p
      JOIN jobcatalog.job_profile_versions v
        ON v.tenant_id = p_tenant_id
       AND v.setid = v_setid
       AND v.job_profile_id = p.id
       AND v.validity @> p_query_date
      LEFT JOIN LATERAL (
        SELECT
          jsonb_agg(f.job_family_id ORDER BY f.job_family_id) AS job_family_ids,
          (
            SELECT f2.job_family_id
            FROM jobcatalog.job_profile_version_job_families f2
            WHERE f2.tenant_id = p_tenant_id
              AND f2.setid = v_setid
              AND f2.job_profile_version_id = v.id
              AND f2.is_primary = true
            LIMIT 1
          ) AS primary_job_family_id
        FROM jobcatalog.job_profile_version_job_families f
        WHERE f.tenant_id = p_tenant_id
          AND f.setid = v_setid
          AND f.job_profile_version_id = v.id
      ) fam ON true
      WHERE p.tenant_id = p_tenant_id
        AND p.setid = v_setid
    ), '[]'::jsonb) AS profiles;
END;
$$;

