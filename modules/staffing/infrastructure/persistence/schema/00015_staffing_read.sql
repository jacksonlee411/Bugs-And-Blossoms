CREATE OR REPLACE FUNCTION staffing.get_position_snapshot(
  p_tenant_id uuid,
  p_query_date date
)
RETURNS TABLE (
  position_id uuid,
  org_unit_id uuid,
  reports_to_position_id uuid,
  jobcatalog_setid text,
  jobcatalog_setid_as_of date,
  job_profile_id uuid,
  job_profile_code text,
  name text,
  lifecycle_status text,
  capacity_fte numeric(9,2),
  effective_date date
)
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);
  IF p_query_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'query_date is required';
  END IF;

	  RETURN QUERY
	  SELECT
    pv.position_id,
    pv.org_unit_id,
    pv.reports_to_position_id,
    pv.jobcatalog_setid,
    pv.jobcatalog_setid_as_of,
    pv.job_profile_id,
	    jp.code::text AS job_profile_code,
	    pv.name,
	    pv.lifecycle_status,
	    pv.capacity_fte,
	    lower(pv.validity) AS effective_date
  FROM staffing.position_versions pv
  LEFT JOIN LATERAL orgunit.resolve_scope_package(
    p_tenant_id,
    pv.jobcatalog_setid,
    'jobcatalog',
    pv.jobcatalog_setid_as_of
  ) sp(package_id, package_owner_tenant_id)
    ON pv.jobcatalog_setid IS NOT NULL
  LEFT JOIN jobcatalog.job_profiles jp
    ON jp.tenant_id = pv.tenant_id
   AND jp.package_id = sp.package_id
   AND jp.id = pv.job_profile_id
  WHERE pv.tenant_id = p_tenant_id
    AND pv.validity @> p_query_date;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.get_assignment_snapshot(
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_query_date date
)
RETURNS TABLE (
  assignment_id uuid,
  person_uuid uuid,
  position_id uuid,
  status text,
  effective_date date
)
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'person_uuid is required';
  END IF;
  IF p_query_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'query_date is required';
  END IF;

  RETURN QUERY
  SELECT
    av.assignment_id,
    av.person_uuid,
    av.position_id,
    av.status,
    lower(av.validity) AS effective_date
  FROM staffing.assignment_versions av
  WHERE av.tenant_id = p_tenant_id
    AND av.person_uuid = p_person_uuid
    AND av.validity @> p_query_date;
END;
$$;
