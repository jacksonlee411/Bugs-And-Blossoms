-- +goose Up
-- +goose StatementBegin
ALTER TABLE staffing.position_events
  DROP CONSTRAINT IF EXISTS position_events_payload_allowed_keys_check;

ALTER TABLE staffing.position_events
  ADD CONSTRAINT position_events_payload_allowed_keys_check CHECK (
    (
      payload
      - 'org_unit_id'
      - 'name'
      - 'reports_to_position_id'
      - 'business_unit_id'
      - 'job_profile_id'
      - 'lifecycle_status'
      - 'capacity_fte'
    ) = '{}'::jsonb
  );
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE staffing.assignment_events
  DROP CONSTRAINT IF EXISTS assignment_events_payload_allowed_keys_check;

ALTER TABLE staffing.assignment_events
  ADD CONSTRAINT assignment_events_payload_allowed_keys_check CHECK (
    (
      payload
      - 'position_id'
      - 'status'
      - 'base_salary'
      - 'allocated_fte'
      - 'currency'
      - 'profile'
    ) = '{}'::jsonb
  );
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION staffing.get_position_snapshot(
  p_tenant_id uuid,
  p_query_date date
)
RETURNS TABLE (
  position_id uuid,
  org_unit_id uuid,
  reports_to_position_id uuid,
  business_unit_id text,
  jobcatalog_setid text,
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
    pv.business_unit_id,
    pv.jobcatalog_setid,
    pv.job_profile_id,
    jp.code AS job_profile_code,
    pv.name,
    pv.lifecycle_status,
    pv.capacity_fte,
    lower(pv.validity) AS effective_date
  FROM staffing.position_versions pv
  LEFT JOIN jobcatalog.job_profiles jp
    ON jp.tenant_id = pv.tenant_id
   AND jp.setid = pv.jobcatalog_setid
   AND jp.id = pv.job_profile_id
  WHERE pv.tenant_id = p_tenant_id
    AND pv.validity @> p_query_date;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
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
-- +goose StatementEnd
