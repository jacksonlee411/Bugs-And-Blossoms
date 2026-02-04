CREATE OR REPLACE FUNCTION staffing.get_position_snapshot(
  p_tenant_uuid uuid,
  p_query_date date
)
RETURNS TABLE (
  position_uuid uuid,
  org_unit_id int,
  reports_to_position_uuid uuid,
  jobcatalog_setid text,
  jobcatalog_setid_as_of date,
  job_profile_uuid uuid,
  job_profile_code text,
  name text,
  lifecycle_status text,
  capacity_fte numeric(9,2),
  effective_date date
)
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_uuid);
  IF p_query_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'query_date is required';
  END IF;

	  RETURN QUERY
	  SELECT
    pv.position_uuid,
    pv.org_unit_id,
    pv.reports_to_position_uuid,
    pv.jobcatalog_setid,
    pv.jobcatalog_setid_as_of,
    pv.job_profile_uuid,
    jp.job_profile_code::text AS job_profile_code,
	    pv.name,
	    pv.lifecycle_status,
	    pv.capacity_fte,
	    lower(pv.validity) AS effective_date
  FROM staffing.position_versions pv
  LEFT JOIN LATERAL orgunit.resolve_scope_package(
    p_tenant_uuid,
    pv.jobcatalog_setid,
    'jobcatalog',
    pv.jobcatalog_setid_as_of
  ) sp(package_uuid, package_owner_tenant_uuid)
    ON pv.jobcatalog_setid IS NOT NULL
  LEFT JOIN jobcatalog.job_profiles jp
    ON jp.tenant_uuid = pv.tenant_uuid
   AND jp.package_uuid = sp.package_uuid
   AND jp.job_profile_uuid = pv.job_profile_uuid
  WHERE pv.tenant_uuid = p_tenant_uuid
    AND pv.validity @> p_query_date;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.get_assignment_snapshot(
  p_tenant_uuid uuid,
  p_person_uuid uuid,
  p_query_date date
)
RETURNS TABLE (
  assignment_uuid uuid,
  person_uuid uuid,
  position_uuid uuid,
  status text,
  effective_date date
)
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_uuid);
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
    av.assignment_uuid,
    av.person_uuid,
    av.position_uuid,
    av.status,
    lower(av.validity) AS effective_date
  FROM staffing.assignment_versions av
  WHERE av.tenant_uuid = p_tenant_uuid
    AND av.person_uuid = p_person_uuid
    AND av.validity @> p_query_date;
END;
$$;
