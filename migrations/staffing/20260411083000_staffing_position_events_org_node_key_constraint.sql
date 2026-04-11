-- +goose Up
-- +goose StatementBegin
ALTER TABLE staffing.position_events
  DROP CONSTRAINT IF EXISTS position_events_payload_allowed_keys_check;

UPDATE staffing.position_events
SET payload = CASE
  WHEN payload ? 'org_unit_id' AND payload ? 'org_node_key' THEN payload - 'org_unit_id'
  WHEN payload ? 'org_unit_id' THEN (payload - 'org_unit_id') || jsonb_build_object('org_node_key', payload->'org_unit_id')
  ELSE payload
END
WHERE payload ? 'org_unit_id';

ALTER TABLE staffing.position_events
  ADD CONSTRAINT position_events_payload_allowed_keys_check CHECK (
    (
      payload
      - 'org_node_key'
      - 'name'
      - 'reports_to_position_uuid'
      - 'job_profile_uuid'
      - 'lifecycle_status'
      - 'capacity_fte'
    ) = '{}'::jsonb
  );

ALTER TABLE staffing.position_versions
  ADD COLUMN IF NOT EXISTS org_node_key char(8);

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'staffing'
      AND table_name = 'position_versions'
      AND column_name = 'org_unit_id'
  ) THEN
    EXECUTE $sql$
      UPDATE staffing.position_versions
      SET org_node_key = orgunit.encode_org_node_key(org_unit_id::bigint)
      WHERE org_node_key IS NULL
        AND org_unit_id IS NOT NULL
    $sql$;
  END IF;
END
$$;

ALTER TABLE staffing.position_versions
  DROP CONSTRAINT IF EXISTS position_versions_org_node_key_check,
  DROP CONSTRAINT IF EXISTS position_versions_org_unit_id_check;

ALTER TABLE staffing.position_versions
  ALTER COLUMN org_node_key TYPE char(8) USING btrim(org_node_key::text)::char(8),
  ALTER COLUMN org_node_key SET NOT NULL;

ALTER TABLE staffing.position_versions
  ADD CONSTRAINT position_versions_org_node_key_check CHECK (orgunit.is_valid_org_node_key(btrim(org_node_key::text)));

CREATE OR REPLACE FUNCTION staffing.replay_position_versions(
  p_tenant_uuid uuid,
  p_position_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_prev_effective date;
  v_last_validity daterange;
  v_org_node_key text;
  v_reports_to_position_uuid uuid;
  v_jobcatalog_setid text;
  v_jobcatalog_setid_as_of date;
  v_jobcatalog_package_uuid uuid;
  v_job_profile_uuid uuid;
  v_name text;
  v_lifecycle_status text;
  v_capacity_fte numeric(9,2);
  v_profile jsonb;
  v_tmp_text text;
  v_target_status text;
  v_row RECORD;
  v_validity daterange;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_uuid);

  IF p_position_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'position_uuid is required';
  END IF;

  v_lock_key := format('staffing:position:%s:%s', p_tenant_uuid, p_position_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM staffing.position_versions
  WHERE tenant_uuid = p_tenant_uuid AND position_uuid = p_position_uuid;

  v_org_node_key := NULL;
  v_reports_to_position_uuid := NULL;
  v_jobcatalog_setid := NULL;
  v_jobcatalog_setid_as_of := NULL;
  v_job_profile_uuid := NULL;
  v_name := NULL;
  v_lifecycle_status := 'active';
  v_capacity_fte := 1.0;
  v_profile := '{}'::jsonb;
  v_prev_effective := NULL;

  FOR v_row IN
    SELECT
      e.id AS event_db_id,
      e.event_type,
      e.effective_date,
      e.payload,
      lead(effective_date) OVER (ORDER BY effective_date ASC, id ASC) AS next_effective
    FROM staffing.position_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.position_uuid = p_position_uuid
    ORDER BY effective_date ASC, id ASC
  LOOP
    IF v_row.event_type = 'CREATE' THEN
      IF v_prev_effective IS NOT NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'CREATE must be the first event';
      END IF;

      v_tmp_text := NULLIF(btrim(v_row.payload->>'org_node_key'), '');
      IF v_tmp_text IS NULL THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_INVALID_ARGUMENT',
          DETAIL = 'org_node_key is required';
      END IF;
      IF NOT orgunit.is_valid_org_node_key(v_tmp_text) THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_INVALID_ARGUMENT',
          DETAIL = format('org_node_key=%s', v_row.payload->>'org_node_key');
      END IF;
      v_org_node_key := v_tmp_text;

      v_name := NULLIF(btrim(v_row.payload->>'name'), '');

      IF v_row.payload ? 'reports_to_position_uuid' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'reports_to_position_uuid'), '');
        IF v_tmp_text IS NULL THEN
          v_reports_to_position_uuid := NULL;
        ELSE
          BEGIN
            v_reports_to_position_uuid := v_tmp_text::uuid;
          EXCEPTION
            WHEN invalid_text_representation THEN
              RAISE EXCEPTION USING
                MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                DETAIL = format('reports_to_position_uuid=%s', v_row.payload->>'reports_to_position_uuid');
          END;
        END IF;
      ELSE
        v_reports_to_position_uuid := NULL;
      END IF;

      IF v_row.payload ? 'lifecycle_status' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'lifecycle_status'), '');
        IF v_tmp_text IS NULL OR v_tmp_text NOT IN ('active','disabled') THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('lifecycle_status=%s', v_row.payload->>'lifecycle_status');
        END IF;
        v_lifecycle_status := v_tmp_text;
      ELSE
        v_lifecycle_status := 'active';
      END IF;

      IF v_row.payload ? 'capacity_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'capacity_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'capacity_fte is required';
        END IF;
        BEGIN
          v_capacity_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END;
        IF v_capacity_fte <= 0 THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END IF;
      ELSE
        v_capacity_fte := 1.0;
      END IF;

      IF v_row.payload ? 'job_profile_uuid' THEN
        IF v_row.payload->'job_profile_uuid' IS NULL THEN
          v_job_profile_uuid := NULL;
        ELSE
          v_tmp_text := NULLIF(btrim(v_row.payload->>'job_profile_uuid'), '');
          IF v_tmp_text IS NULL THEN
            v_job_profile_uuid := NULL;
          ELSE
            BEGIN
              v_job_profile_uuid := v_tmp_text::uuid;
            EXCEPTION
              WHEN invalid_text_representation THEN
                RAISE EXCEPTION USING
                  MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                  DETAIL = format('job_profile_uuid=%s', v_row.payload->>'job_profile_uuid');
            END;
          END IF;
        END IF;
      END IF;
    ELSIF v_row.event_type = 'UPDATE' THEN
      IF v_prev_effective IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'UPDATE requires prior state';
      END IF;

      IF v_row.payload ? 'org_node_key' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'org_node_key'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'org_node_key is required';
        END IF;
        IF NOT orgunit.is_valid_org_node_key(v_tmp_text) THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('org_node_key=%s', v_row.payload->>'org_node_key');
        END IF;
        v_org_node_key := v_tmp_text;
      END IF;

      IF v_row.payload ? 'reports_to_position_uuid' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'reports_to_position_uuid'), '');
        IF v_tmp_text IS NULL THEN
          v_reports_to_position_uuid := NULL;
        ELSE
          BEGIN
            v_reports_to_position_uuid := v_tmp_text::uuid;
          EXCEPTION
            WHEN invalid_text_representation THEN
              RAISE EXCEPTION USING
                MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                DETAIL = format('reports_to_position_uuid=%s', v_row.payload->>'reports_to_position_uuid');
          END;
        END IF;
      END IF;

      IF v_row.payload ? 'name' THEN
        v_name := NULLIF(btrim(v_row.payload->>'name'), '');
      END IF;

      IF v_row.payload ? 'lifecycle_status' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'lifecycle_status'), '');
        IF v_tmp_text IS NULL OR v_tmp_text NOT IN ('active','disabled') THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('lifecycle_status=%s', v_row.payload->>'lifecycle_status');
        END IF;
        v_lifecycle_status := v_tmp_text;
      END IF;

      IF v_row.payload ? 'capacity_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'capacity_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'capacity_fte is required';
        END IF;
        BEGIN
          v_capacity_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END;
        IF v_capacity_fte <= 0 THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END IF;
      END IF;

      IF v_row.payload ? 'job_profile_uuid' THEN
        IF v_row.payload->'job_profile_uuid' IS NULL THEN
          v_job_profile_uuid := NULL;
        ELSE
          v_tmp_text := NULLIF(btrim(v_row.payload->>'job_profile_uuid'), '');
          IF v_tmp_text IS NULL THEN
            v_job_profile_uuid := NULL;
          ELSE
            BEGIN
              v_job_profile_uuid := v_tmp_text::uuid;
            EXCEPTION
              WHEN invalid_text_representation THEN
                RAISE EXCEPTION USING
                  MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                  DETAIL = format('job_profile_uuid=%s', v_row.payload->>'job_profile_uuid');
            END;
          END IF;
        END IF;
      END IF;
    ELSE
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', v_row.event_type);
    END IF;

    IF v_org_node_key IS NOT NULL THEN
      IF NOT EXISTS (
        SELECT 1
        FROM orgunit.org_unit_versions ouv
        WHERE ouv.tenant_uuid = p_tenant_uuid
          AND CASE
            WHEN to_jsonb(ouv) ? 'org_node_key'
              THEN btrim(COALESCE(to_jsonb(ouv)->>'org_node_key', '')) = v_org_node_key
            ELSE NULLIF(to_jsonb(ouv)->>'org_id', '')::int = orgunit.decode_org_node_key(v_org_node_key::char(8))::int
          END
          AND ouv.status = 'active'
          AND ouv.validity @> v_row.effective_date
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_ORG_UNIT_NOT_FOUND_AS_OF',
          DETAIL = format('org_node_key=%s as_of=%s', v_org_node_key, v_row.effective_date);
      END IF;
    END IF;

    IF v_job_profile_uuid IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = 'job_profile_uuid is required';
    END IF;

    v_jobcatalog_setid := orgunit.resolve_setid(
      p_tenant_uuid,
      v_org_node_key::char(8),
      v_row.effective_date
    );
    v_jobcatalog_setid_as_of := v_row.effective_date;
    SELECT package_id
    INTO v_jobcatalog_package_uuid
    FROM orgunit.resolve_scope_package(p_tenant_uuid, v_jobcatalog_setid, 'jobcatalog', v_row.effective_date);

    IF NOT EXISTS (
      SELECT 1
      FROM jobcatalog.job_profile_versions jpv
      WHERE jpv.tenant_uuid = p_tenant_uuid
        AND jpv.package_uuid = v_jobcatalog_package_uuid
        AND COALESCE(jpv.setid, '') = v_jobcatalog_setid
        AND jpv.job_profile_uuid = v_job_profile_uuid
        AND jpv.is_active = true
        AND jpv.validity @> v_row.effective_date
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
        DETAIL = format('job_profile_uuid=%s', v_job_profile_uuid);
    END IF;

    IF v_reports_to_position_uuid IS NOT NULL THEN
      IF v_reports_to_position_uuid = p_position_uuid THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_REPORTS_TO_SELF',
          DETAIL = format('position_uuid=%s', p_position_uuid);
      END IF;

      SELECT lifecycle_status INTO v_target_status
      FROM staffing.position_versions pv
      WHERE pv.tenant_uuid = p_tenant_uuid
        AND pv.position_uuid = v_reports_to_position_uuid
        AND pv.validity @> v_row.effective_date
      ORDER BY lower(pv.validity) DESC
      LIMIT 1;
      IF NOT FOUND THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_NOT_FOUND_AS_OF',
          DETAIL = format('position_uuid=%s as_of=%s', v_reports_to_position_uuid, v_row.effective_date);
      END IF;
      IF v_target_status <> 'active' THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_DISABLED_AS_OF',
          DETAIL = format('position_uuid=%s as_of=%s', v_reports_to_position_uuid, v_row.effective_date);
      END IF;

      IF EXISTS (
        WITH RECURSIVE chain AS (
          SELECT pv.position_uuid, pv.reports_to_position_uuid
          FROM staffing.position_versions pv
          WHERE pv.tenant_uuid = p_tenant_uuid
            AND pv.position_uuid = v_reports_to_position_uuid
            AND pv.validity @> v_row.effective_date
          UNION ALL
          SELECT pv.position_uuid, pv.reports_to_position_uuid
          FROM staffing.position_versions pv
          JOIN chain c ON pv.position_uuid = c.reports_to_position_uuid
          WHERE pv.tenant_uuid = p_tenant_uuid
            AND pv.validity @> v_row.effective_date
            AND c.reports_to_position_uuid IS NOT NULL
        )
        SELECT 1
        FROM chain
        WHERE position_uuid = p_position_uuid
           OR reports_to_position_uuid = p_position_uuid
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_REPORTS_TO_CYCLE',
          DETAIL = format('position_uuid=%s', p_position_uuid);
      END IF;
    END IF;

    IF v_row.next_effective IS NULL THEN
      v_validity := daterange(v_row.effective_date, NULL, '[)');
    ELSE
      v_validity := daterange(v_row.effective_date, v_row.next_effective, '[)');
    END IF;

    IF v_lifecycle_status = 'disabled' THEN
      IF EXISTS (
        SELECT 1
        FROM staffing.assignment_versions av
        WHERE av.tenant_uuid = p_tenant_uuid
          AND av.position_uuid = p_position_uuid
          AND av.status = 'active'
          AND av.validity && v_validity
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF',
          DETAIL = format('position_uuid=%s as_of=%s', p_position_uuid, v_row.effective_date);
      END IF;
    END IF;

    INSERT INTO staffing.position_versions (
      tenant_uuid,
      position_uuid,
      org_node_key,
      reports_to_position_uuid,
      name,
      lifecycle_status,
      capacity_fte,
      profile,
      validity,
      last_event_id,
      jobcatalog_setid,
      jobcatalog_setid_as_of,
      job_profile_uuid
    )
    VALUES (
      p_tenant_uuid,
      p_position_uuid,
      v_org_node_key,
      v_reports_to_position_uuid,
      v_name,
      v_lifecycle_status,
      v_capacity_fte,
      v_profile,
      v_validity,
      v_row.event_db_id,
      v_jobcatalog_setid,
      v_jobcatalog_setid_as_of,
      v_job_profile_uuid
    );

    IF v_lifecycle_status = 'active' THEN
      PERFORM staffing.assert_position_capacity(p_tenant_uuid, p_position_uuid, v_validity);
    END IF;

    v_prev_effective := v_row.effective_date;
  END LOOP;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        validity,
        lag(validity) OVER (ORDER BY lower(validity)) AS prev_validity
      FROM staffing.position_versions
      WHERE tenant_uuid = p_tenant_uuid AND position_uuid = p_position_uuid
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
  WHERE tenant_uuid = p_tenant_uuid AND position_uuid = p_position_uuid
  ORDER BY lower(validity) DESC
  LIMIT 1;
  IF v_last_validity IS NOT NULL AND upper(v_last_validity) IS NOT NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_NOT_INFINITE',
      DETAIL = 'position_versions must end at infinity';
  END IF;
END;
$$;

DROP FUNCTION IF EXISTS staffing.get_position_snapshot(uuid, date);

CREATE FUNCTION staffing.get_position_snapshot(
  p_tenant_uuid uuid,
  p_query_date date
)
RETURNS TABLE (
  position_uuid uuid,
  org_node_key char(8),
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
    pv.org_node_key,
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

ALTER TABLE staffing.position_versions
  DROP COLUMN IF EXISTS org_unit_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE staffing.position_versions
  ADD COLUMN IF NOT EXISTS org_unit_id integer;

UPDATE staffing.position_versions
SET org_unit_id = orgunit.decode_org_node_key(org_node_key::char(8))::int
WHERE org_unit_id IS NULL
  AND org_node_key IS NOT NULL;

ALTER TABLE staffing.position_versions
  DROP CONSTRAINT IF EXISTS position_versions_org_node_key_check,
  DROP CONSTRAINT IF EXISTS position_versions_org_unit_id_check;

ALTER TABLE staffing.position_versions
  ALTER COLUMN org_unit_id SET NOT NULL;

ALTER TABLE staffing.position_versions
  ADD CONSTRAINT position_versions_org_unit_id_check CHECK (org_unit_id >= 10000000 AND org_unit_id <= 99999999);

ALTER TABLE staffing.position_events
  DROP CONSTRAINT IF EXISTS position_events_payload_allowed_keys_check;

UPDATE staffing.position_events
SET payload = CASE
  WHEN payload ? 'org_node_key' AND payload ? 'org_unit_id' THEN payload - 'org_node_key'
  WHEN payload ? 'org_node_key' THEN (payload - 'org_node_key') || jsonb_build_object('org_unit_id', payload->'org_node_key')
  ELSE payload
END
WHERE payload ? 'org_node_key';

ALTER TABLE staffing.position_events
  ADD CONSTRAINT position_events_payload_allowed_keys_check CHECK (
    (
      payload
      - 'org_unit_id'
      - 'name'
      - 'reports_to_position_uuid'
      - 'job_profile_uuid'
      - 'lifecycle_status'
      - 'capacity_fte'
    ) = '{}'::jsonb
  );

CREATE OR REPLACE FUNCTION staffing.replay_position_versions(
  p_tenant_uuid uuid,
  p_position_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_prev_effective date;
  v_last_validity daterange;
  v_org_unit_id int;
  v_reports_to_position_uuid uuid;
  v_jobcatalog_setid text;
  v_jobcatalog_setid_as_of date;
  v_jobcatalog_package_uuid uuid;
  v_job_profile_uuid uuid;
  v_name text;
  v_lifecycle_status text;
  v_capacity_fte numeric(9,2);
  v_profile jsonb;
  v_tmp_text text;
  v_target_status text;
  v_row RECORD;
  v_validity daterange;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_uuid);

  IF p_position_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'position_uuid is required';
  END IF;

  v_lock_key := format('staffing:position:%s:%s', p_tenant_uuid, p_position_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM staffing.position_versions
  WHERE tenant_uuid = p_tenant_uuid AND position_uuid = p_position_uuid;

  v_org_unit_id := NULL;
  v_reports_to_position_uuid := NULL;
  v_jobcatalog_setid := NULL;
  v_jobcatalog_setid_as_of := NULL;
  v_job_profile_uuid := NULL;
  v_name := NULL;
  v_lifecycle_status := 'active';
  v_capacity_fte := 1.0;
  v_profile := '{}'::jsonb;
  v_prev_effective := NULL;

  FOR v_row IN
    SELECT
      e.id AS event_db_id,
      e.event_type,
      e.effective_date,
      e.payload,
      lead(effective_date) OVER (ORDER BY effective_date ASC, id ASC) AS next_effective
    FROM staffing.position_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.position_uuid = p_position_uuid
    ORDER BY effective_date ASC, id ASC
  LOOP
    IF v_row.event_type = 'CREATE' THEN
      IF v_prev_effective IS NOT NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'CREATE must be the first event';
      END IF;

      v_tmp_text := NULLIF(btrim(v_row.payload->>'org_unit_id'), '');
      IF v_tmp_text IS NULL THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_INVALID_ARGUMENT',
          DETAIL = 'org_unit_id is required';
      END IF;
      BEGIN
        v_org_unit_id := v_tmp_text::int;
      EXCEPTION
        WHEN invalid_text_representation THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('org_unit_id=%s', v_row.payload->>'org_unit_id');
      END;

      v_name := NULLIF(btrim(v_row.payload->>'name'), '');

      IF v_row.payload ? 'reports_to_position_uuid' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'reports_to_position_uuid'), '');
        IF v_tmp_text IS NULL THEN
          v_reports_to_position_uuid := NULL;
        ELSE
          BEGIN
            v_reports_to_position_uuid := v_tmp_text::uuid;
          EXCEPTION
            WHEN invalid_text_representation THEN
              RAISE EXCEPTION USING
                MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                DETAIL = format('reports_to_position_uuid=%s', v_row.payload->>'reports_to_position_uuid');
          END;
        END IF;
      ELSE
        v_reports_to_position_uuid := NULL;
      END IF;

      IF v_row.payload ? 'lifecycle_status' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'lifecycle_status'), '');
        IF v_tmp_text IS NULL OR v_tmp_text NOT IN ('active','disabled') THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('lifecycle_status=%s', v_row.payload->>'lifecycle_status');
        END IF;
        v_lifecycle_status := v_tmp_text;
      ELSE
        v_lifecycle_status := 'active';
      END IF;

      IF v_row.payload ? 'capacity_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'capacity_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'capacity_fte is required';
        END IF;
        BEGIN
          v_capacity_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END;
        IF v_capacity_fte <= 0 THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END IF;
      ELSE
        v_capacity_fte := 1.0;
      END IF;

      IF v_row.payload ? 'job_profile_uuid' THEN
        IF v_row.payload->'job_profile_uuid' IS NULL THEN
          v_job_profile_uuid := NULL;
        ELSE
          v_tmp_text := NULLIF(btrim(v_row.payload->>'job_profile_uuid'), '');
          IF v_tmp_text IS NULL THEN
            v_job_profile_uuid := NULL;
          ELSE
            BEGIN
              v_job_profile_uuid := v_tmp_text::uuid;
            EXCEPTION
              WHEN invalid_text_representation THEN
                RAISE EXCEPTION USING
                  MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                  DETAIL = format('job_profile_uuid=%s', v_row.payload->>'job_profile_uuid');
            END;
          END IF;
        END IF;
      END IF;
    ELSIF v_row.event_type = 'UPDATE' THEN
      IF v_prev_effective IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'UPDATE requires prior state';
      END IF;

      IF v_row.payload ? 'org_unit_id' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'org_unit_id'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'org_unit_id is required';
        END IF;
        BEGIN
          v_org_unit_id := v_tmp_text::int;
        EXCEPTION
          WHEN invalid_text_representation THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('org_unit_id=%s', v_row.payload->>'org_unit_id');
        END;
      END IF;

      IF v_row.payload ? 'reports_to_position_uuid' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'reports_to_position_uuid'), '');
        IF v_tmp_text IS NULL THEN
          v_reports_to_position_uuid := NULL;
        ELSE
          BEGIN
            v_reports_to_position_uuid := v_tmp_text::uuid;
          EXCEPTION
            WHEN invalid_text_representation THEN
              RAISE EXCEPTION USING
                MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                DETAIL = format('reports_to_position_uuid=%s', v_row.payload->>'reports_to_position_uuid');
          END;
        END IF;
      END IF;

      IF v_row.payload ? 'name' THEN
        v_name := NULLIF(btrim(v_row.payload->>'name'), '');
      END IF;

      IF v_row.payload ? 'lifecycle_status' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'lifecycle_status'), '');
        IF v_tmp_text IS NULL OR v_tmp_text NOT IN ('active','disabled') THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('lifecycle_status=%s', v_row.payload->>'lifecycle_status');
        END IF;
        v_lifecycle_status := v_tmp_text;
      END IF;

      IF v_row.payload ? 'capacity_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'capacity_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'capacity_fte is required';
        END IF;
        BEGIN
          v_capacity_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END;
        IF v_capacity_fte <= 0 THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('capacity_fte=%s', v_row.payload->>'capacity_fte');
        END IF;
      END IF;

      IF v_row.payload ? 'job_profile_uuid' THEN
        IF v_row.payload->'job_profile_uuid' IS NULL THEN
          v_job_profile_uuid := NULL;
        ELSE
          v_tmp_text := NULLIF(btrim(v_row.payload->>'job_profile_uuid'), '');
          IF v_tmp_text IS NULL THEN
            v_job_profile_uuid := NULL;
          ELSE
            BEGIN
              v_job_profile_uuid := v_tmp_text::uuid;
            EXCEPTION
              WHEN invalid_text_representation THEN
                RAISE EXCEPTION USING
                  MESSAGE = 'STAFFING_INVALID_ARGUMENT',
                  DETAIL = format('job_profile_uuid=%s', v_row.payload->>'job_profile_uuid');
            END;
          END IF;
        END IF;
      END IF;
    ELSE
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', v_row.event_type);
    END IF;

    IF v_org_unit_id IS NOT NULL THEN
      IF NOT EXISTS (
        SELECT 1
        FROM orgunit.org_unit_versions ouv
        WHERE ouv.tenant_uuid = p_tenant_uuid
          AND ouv.org_id = v_org_unit_id
          AND ouv.status = 'active'
          AND ouv.validity @> v_row.effective_date
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_ORG_UNIT_NOT_FOUND_AS_OF',
          DETAIL = format('org_unit_id=%s as_of=%s', v_org_unit_id, v_row.effective_date);
      END IF;
    END IF;

    IF v_job_profile_uuid IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = 'job_profile_uuid is required';
    END IF;

    v_jobcatalog_setid := orgunit.resolve_setid(p_tenant_uuid, v_org_unit_id, v_row.effective_date);
    v_jobcatalog_setid_as_of := v_row.effective_date;
    SELECT package_id
    INTO v_jobcatalog_package_uuid
    FROM orgunit.resolve_scope_package(p_tenant_uuid, v_jobcatalog_setid, 'jobcatalog', v_row.effective_date);

    IF NOT EXISTS (
      SELECT 1
      FROM jobcatalog.job_profile_versions jpv
      WHERE jpv.tenant_uuid = p_tenant_uuid
        AND jpv.package_uuid = v_jobcatalog_package_uuid
        AND COALESCE(jpv.setid, '') = v_jobcatalog_setid
        AND jpv.job_profile_uuid = v_job_profile_uuid
        AND jpv.is_active = true
        AND jpv.validity @> v_row.effective_date
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
        DETAIL = format('job_profile_uuid=%s', v_job_profile_uuid);
    END IF;

    IF v_reports_to_position_uuid IS NOT NULL THEN
      IF v_reports_to_position_uuid = p_position_uuid THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_REPORTS_TO_SELF',
          DETAIL = format('position_uuid=%s', p_position_uuid);
      END IF;

      SELECT lifecycle_status INTO v_target_status
      FROM staffing.position_versions pv
      WHERE pv.tenant_uuid = p_tenant_uuid
        AND pv.position_uuid = v_reports_to_position_uuid
        AND pv.validity @> v_row.effective_date
      ORDER BY lower(pv.validity) DESC
      LIMIT 1;
      IF NOT FOUND THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_NOT_FOUND_AS_OF',
          DETAIL = format('position_uuid=%s as_of=%s', v_reports_to_position_uuid, v_row.effective_date);
      END IF;
      IF v_target_status <> 'active' THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_DISABLED_AS_OF',
          DETAIL = format('position_uuid=%s as_of=%s', v_reports_to_position_uuid, v_row.effective_date);
      END IF;

      IF EXISTS (
        WITH RECURSIVE chain AS (
          SELECT pv.position_uuid, pv.reports_to_position_uuid
          FROM staffing.position_versions pv
          WHERE pv.tenant_uuid = p_tenant_uuid
            AND pv.position_uuid = v_reports_to_position_uuid
            AND pv.validity @> v_row.effective_date
          UNION ALL
          SELECT pv.position_uuid, pv.reports_to_position_uuid
          FROM staffing.position_versions pv
          JOIN chain c ON pv.position_uuid = c.reports_to_position_uuid
          WHERE pv.tenant_uuid = p_tenant_uuid
            AND pv.validity @> v_row.effective_date
            AND c.reports_to_position_uuid IS NOT NULL
        )
        SELECT 1
        FROM chain
        WHERE position_uuid = p_position_uuid
           OR reports_to_position_uuid = p_position_uuid
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_REPORTS_TO_CYCLE',
          DETAIL = format('position_uuid=%s', p_position_uuid);
      END IF;
    END IF;

    IF v_row.next_effective IS NULL THEN
      v_validity := daterange(v_row.effective_date, NULL, '[)');
    ELSE
      v_validity := daterange(v_row.effective_date, v_row.next_effective, '[)');
    END IF;

    IF v_lifecycle_status = 'disabled' THEN
      IF EXISTS (
        SELECT 1
        FROM staffing.assignment_versions av
        WHERE av.tenant_uuid = p_tenant_uuid
          AND av.position_uuid = p_position_uuid
          AND av.status = 'active'
          AND av.validity && v_validity
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF',
          DETAIL = format('position_uuid=%s as_of=%s', p_position_uuid, v_row.effective_date);
      END IF;
    END IF;

    INSERT INTO staffing.position_versions (
      tenant_uuid,
      position_uuid,
      org_unit_id,
      reports_to_position_uuid,
      name,
      lifecycle_status,
      capacity_fte,
      profile,
      validity,
      last_event_id,
      jobcatalog_setid,
      jobcatalog_setid_as_of,
      job_profile_uuid
    )
    VALUES (
      p_tenant_uuid,
      p_position_uuid,
      v_org_unit_id,
      v_reports_to_position_uuid,
      v_name,
      v_lifecycle_status,
      v_capacity_fte,
      v_profile,
      v_validity,
      v_row.event_db_id,
      v_jobcatalog_setid,
      v_jobcatalog_setid_as_of,
      v_job_profile_uuid
    );

    IF v_lifecycle_status = 'active' THEN
      PERFORM staffing.assert_position_capacity(p_tenant_uuid, p_position_uuid, v_validity);
    END IF;

    v_prev_effective := v_row.effective_date;
  END LOOP;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        validity,
        lag(validity) OVER (ORDER BY lower(validity)) AS prev_validity
      FROM staffing.position_versions
      WHERE tenant_uuid = p_tenant_uuid AND position_uuid = p_position_uuid
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
  WHERE tenant_uuid = p_tenant_uuid AND position_uuid = p_position_uuid
  ORDER BY lower(validity) DESC
  LIMIT 1;
  IF v_last_validity IS NOT NULL AND upper(v_last_validity) IS NOT NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_NOT_INFINITE',
      DETAIL = 'position_versions must end at infinity';
  END IF;
END;
$$;

DROP FUNCTION IF EXISTS staffing.get_position_snapshot(uuid, date);

CREATE FUNCTION staffing.get_position_snapshot(
  p_tenant_uuid uuid,
  p_query_date date
)
RETURNS TABLE (
  position_uuid uuid,
  org_unit_id integer,
  reports_to_position_uuid uuid,
  jobcatalog_setid text,
  jobcatalog_setid_as_of date,
  job_profile_uuid uuid,
  job_profile_code text,
  name text,
  lifecycle_status text,
  capacity_fte numeric,
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

ALTER TABLE staffing.position_versions
  DROP COLUMN IF EXISTS org_node_key;
-- +goose StatementEnd
