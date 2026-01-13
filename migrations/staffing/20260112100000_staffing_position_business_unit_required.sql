-- +goose Up
-- +goose StatementBegin
ALTER TABLE staffing.position_events DISABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.position_versions DISABLE ROW LEVEL SECURITY;

UPDATE staffing.position_events
SET payload = payload || jsonb_build_object('business_unit_id', 'BU000')
WHERE event_type = 'CREATE'
  AND (NOT (payload ? 'business_unit_id') OR btrim(payload->>'business_unit_id') = '');

UPDATE staffing.position_events
SET payload = payload - 'business_unit_id'
WHERE event_type = 'UPDATE'
  AND payload ? 'business_unit_id'
  AND btrim(payload->>'business_unit_id') = '';

WITH first_create AS (
  SELECT DISTINCT ON (tenant_id, position_id)
    tenant_id,
    position_id,
    payload
  FROM staffing.position_events
  WHERE event_type = 'CREATE'
  ORDER BY tenant_id, position_id, effective_date ASC, id ASC
)
UPDATE staffing.position_versions pv
SET business_unit_id = upper(btrim(fc.payload->>'business_unit_id'))
FROM first_create fc
WHERE pv.tenant_id = fc.tenant_id
  AND pv.position_id = fc.position_id
  AND (pv.business_unit_id IS NULL OR btrim(pv.business_unit_id) = '');

UPDATE staffing.position_versions
SET business_unit_id = 'BU000'
WHERE business_unit_id IS NULL OR btrim(business_unit_id) = '';

UPDATE staffing.position_versions
SET business_unit_id = upper(btrim(business_unit_id))
WHERE business_unit_id <> upper(btrim(business_unit_id));

ALTER TABLE staffing.position_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.position_events FORCE ROW LEVEL SECURITY;
ALTER TABLE staffing.position_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.position_versions FORCE ROW LEVEL SECURITY;

ALTER TABLE staffing.position_versions
  ALTER COLUMN business_unit_id SET NOT NULL;

ALTER TABLE staffing.position_versions
  DROP CONSTRAINT IF EXISTS position_versions_business_unit_id_format_check,
  ADD CONSTRAINT position_versions_business_unit_id_format_check CHECK (business_unit_id ~ '^[A-Z0-9]{1,5}$');

CREATE OR REPLACE FUNCTION staffing.replay_position_versions(
  p_tenant_id uuid,
  p_position_id uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_prev_effective date;
  v_last_validity daterange;
  v_org_unit_id uuid;
  v_reports_to_position_id uuid;
  v_business_unit_id text;
  v_jobcatalog_setid text;
  v_job_profile_id uuid;
  v_capacity_fte numeric(9,2);
  v_name text;
  v_lifecycle_status text;
  v_reports_to_status text;
  v_tmp_text text;
  v_row RECORD;
  v_validity daterange;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_position_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'position_id is required';
  END IF;

  v_lock_key := format('staffing:position:%s:%s', p_tenant_id, p_position_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM staffing.position_versions
  WHERE tenant_id = p_tenant_id AND position_id = p_position_id;

  v_org_unit_id := NULL;
  v_reports_to_position_id := NULL;
  v_business_unit_id := NULL;
  v_jobcatalog_setid := NULL;
  v_job_profile_id := NULL;
  v_capacity_fte := 1.0;
  v_name := NULL;
  v_lifecycle_status := 'active';
  v_prev_effective := NULL;

  FOR v_row IN
    SELECT
      e.id AS event_db_id,
      e.event_type,
      e.effective_date,
      e.payload,
      lead(e.effective_date) OVER (ORDER BY e.effective_date ASC, e.id ASC) AS next_effective
    FROM staffing.position_events e
    WHERE e.tenant_id = p_tenant_id AND e.position_id = p_position_id
    ORDER BY e.effective_date ASC, e.id ASC
  LOOP
    IF v_row.event_type = 'CREATE' THEN
      IF v_prev_effective IS NOT NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'CREATE must be the first event';
      END IF;

      v_org_unit_id := NULLIF(v_row.payload->>'org_unit_id', '')::uuid;
      IF v_org_unit_id IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_ARGUMENT',
          DETAIL = 'org_unit_id is required';
      END IF;

      v_name := NULLIF(btrim(v_row.payload->>'name'), '');
      v_reports_to_position_id := NULL;
      v_business_unit_id := NULLIF(btrim(v_row.payload->>'business_unit_id'), '');
      IF v_business_unit_id IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_ARGUMENT',
          DETAIL = 'business_unit_id is required';
      END IF;
      v_business_unit_id := upper(v_business_unit_id);
      IF v_business_unit_id !~ '^[A-Z0-9]{1,5}$' THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_ARGUMENT',
          DETAIL = format('invalid business_unit_id: %s', v_row.payload->>'business_unit_id');
      END IF;
      v_job_profile_id := NULL;
      IF v_row.payload ? 'job_profile_id' THEN
        v_job_profile_id := NULLIF(v_row.payload->>'job_profile_id', '')::uuid;
      END IF;
      v_capacity_fte := 1.0;
      IF v_row.payload ? 'capacity_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'capacity_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'capacity_fte is required';
        END IF;
        BEGIN
          v_capacity_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              ERRCODE = 'P0001',
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('invalid capacity_fte: %s', v_row.payload->>'capacity_fte');
        END;
        IF v_capacity_fte <= 0 THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('invalid capacity_fte: %s', v_row.payload->>'capacity_fte');
        END IF;
      END IF;
      v_lifecycle_status := 'active';
    ELSIF v_row.event_type = 'UPDATE' THEN
      IF v_prev_effective IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'UPDATE requires prior state';
      END IF;

      IF v_row.payload ? 'org_unit_id' THEN
        v_org_unit_id := NULLIF(v_row.payload->>'org_unit_id', '')::uuid;
        IF v_org_unit_id IS NULL THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'org_unit_id is required';
        END IF;
      END IF;
      IF v_row.payload ? 'name' THEN
        v_name := NULLIF(btrim(v_row.payload->>'name'), '');
      END IF;
      IF v_row.payload ? 'reports_to_position_id' THEN
        v_reports_to_position_id := NULLIF(v_row.payload->>'reports_to_position_id', '')::uuid;
      END IF;
      IF v_row.payload ? 'business_unit_id' THEN
        v_business_unit_id := NULLIF(btrim(v_row.payload->>'business_unit_id'), '');
        IF v_business_unit_id IS NULL THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'business_unit_id is required';
        END IF;
        v_business_unit_id := upper(v_business_unit_id);
        IF v_business_unit_id !~ '^[A-Z0-9]{1,5}$' THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('invalid business_unit_id: %s', v_row.payload->>'business_unit_id');
        END IF;
      END IF;
      IF v_row.payload ? 'job_profile_id' THEN
        v_job_profile_id := NULLIF(v_row.payload->>'job_profile_id', '')::uuid;
      END IF;
      IF v_row.payload ? 'capacity_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'capacity_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'capacity_fte is required';
        END IF;
        BEGIN
          v_capacity_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              ERRCODE = 'P0001',
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('invalid capacity_fte: %s', v_row.payload->>'capacity_fte');
        END;
        IF v_capacity_fte <= 0 THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('invalid capacity_fte: %s', v_row.payload->>'capacity_fte');
        END IF;
      END IF;
      IF v_row.payload ? 'lifecycle_status' THEN
        v_lifecycle_status := NULLIF(btrim(v_row.payload->>'lifecycle_status'), '');
        IF v_lifecycle_status IS NULL OR v_lifecycle_status NOT IN ('active','disabled') THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('invalid lifecycle_status: %s', v_row.payload->>'lifecycle_status');
        END IF;
      END IF;
    ELSE
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', v_row.event_type);
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_id = p_tenant_id
        AND v.hierarchy_type = 'OrgUnit'
        AND v.org_id = v_org_unit_id
        AND v.status = 'active'
        AND v.validity @> v_row.effective_date
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_ORG_UNIT_NOT_FOUND_AS_OF',
        DETAIL = format('org_unit_id=%s as_of=%s', v_org_unit_id, v_row.effective_date);
    END IF;

    v_jobcatalog_setid := orgunit.resolve_setid(p_tenant_id, v_business_unit_id, 'jobcatalog');
    IF v_job_profile_id IS NOT NULL THEN
      IF NOT EXISTS (
        SELECT 1
        FROM jobcatalog.job_profiles jp
        WHERE jp.tenant_id = p_tenant_id
          AND jp.setid = v_jobcatalog_setid
          AND jp.id = v_job_profile_id
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
          DETAIL = format('job_profile_id=%s setid=%s', v_job_profile_id, v_jobcatalog_setid);
      END IF;
    END IF;

    IF v_row.next_effective IS NULL THEN
      v_validity := daterange(v_row.effective_date, NULL, '[)');
    ELSE
      v_validity := daterange(v_row.effective_date, v_row.next_effective, '[)');
    END IF;

    IF v_reports_to_position_id IS NOT NULL THEN
      IF v_reports_to_position_id = p_position_id THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_REPORTS_TO_SELF',
          DETAIL = format('position_id=%s as_of=%s', p_position_id, v_row.effective_date);
      END IF;

      SELECT pv.lifecycle_status INTO v_reports_to_status
      FROM staffing.position_versions pv
      WHERE pv.tenant_id = p_tenant_id
        AND pv.position_id = v_reports_to_position_id
        AND pv.validity @> v_row.effective_date
      LIMIT 1;
      IF NOT FOUND THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_NOT_FOUND_AS_OF',
          DETAIL = format('position_id=%s as_of=%s', v_reports_to_position_id, v_row.effective_date);
      END IF;
      IF v_reports_to_status <> 'active' THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_DISABLED_AS_OF',
          DETAIL = format('position_id=%s as_of=%s', v_reports_to_position_id, v_row.effective_date);
      END IF;

      IF EXISTS (
        WITH RECURSIVE chain AS (
          SELECT
            pv.position_id,
            pv.reports_to_position_id,
            ARRAY[pv.position_id]::uuid[] AS path
          FROM staffing.position_versions pv
          WHERE pv.tenant_id = p_tenant_id
            AND pv.position_id = v_reports_to_position_id
            AND pv.validity @> v_row.effective_date
          UNION ALL
          SELECT
            pv.position_id,
            pv.reports_to_position_id,
            c.path || pv.position_id
          FROM chain c
          JOIN staffing.position_versions pv
            ON pv.tenant_id = p_tenant_id
           AND pv.position_id = c.reports_to_position_id
           AND pv.validity @> v_row.effective_date
          WHERE c.reports_to_position_id IS NOT NULL
            AND NOT (pv.position_id = ANY(c.path))
        )
        SELECT 1
        FROM chain
        WHERE reports_to_position_id = p_position_id
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_REPORTS_TO_CYCLE',
          DETAIL = format('position_id=%s reports_to_position_id=%s as_of=%s', p_position_id, v_reports_to_position_id, v_row.effective_date);
      END IF;
    END IF;

    IF v_lifecycle_status = 'disabled' AND EXISTS (
      SELECT 1
      FROM staffing.assignment_versions av
      WHERE av.tenant_id = p_tenant_id
        AND av.position_id = p_position_id
        AND av.status = 'active'
        AND av.validity && v_validity
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF',
        DETAIL = format('position_id=%s as_of=%s', p_position_id, lower(v_validity));
    END IF;

    INSERT INTO staffing.position_versions (
      tenant_id,
      position_id,
      org_unit_id,
      reports_to_position_id,
      business_unit_id,
      jobcatalog_setid,
      job_profile_id,
      name,
      lifecycle_status,
      capacity_fte,
      profile,
      validity,
      last_event_id
    )
    VALUES (
      p_tenant_id,
      p_position_id,
      v_org_unit_id,
      v_reports_to_position_id,
      v_business_unit_id,
      v_jobcatalog_setid,
      v_job_profile_id,
      v_name,
      v_lifecycle_status,
      v_capacity_fte,
      '{}'::jsonb,
      v_validity,
      v_row.event_db_id
    );

    PERFORM staffing.assert_position_capacity(p_tenant_id, p_position_id, v_validity);

    v_prev_effective := v_row.effective_date;
  END LOOP;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        validity,
        lag(validity) OVER (ORDER BY lower(validity)) AS prev_validity
      FROM staffing.position_versions
      WHERE tenant_id = p_tenant_id AND position_id = p_position_id
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL
      AND lower(validity) <> upper(prev_validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_GAP',
      DETAIL = 'position_versions must be gapless';
  END IF;

  SELECT validity INTO v_last_validity
  FROM staffing.position_versions
  WHERE tenant_id = p_tenant_id AND position_id = p_position_id
  ORDER BY lower(validity) DESC
  LIMIT 1;

  IF v_last_validity IS NOT NULL AND NOT upper_inf(v_last_validity) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_NOT_INFINITE',
      DETAIL = 'last position version validity must be unbounded (infinity)';
  END IF;
END;
$$;
-- +goose StatementEnd
