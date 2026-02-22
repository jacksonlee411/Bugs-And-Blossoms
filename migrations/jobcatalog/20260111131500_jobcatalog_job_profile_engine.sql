-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION jobcatalog.replay_job_profile_versions(
  p_tenant_uuid uuid,
  p_setid text,
  p_job_profile_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_setid text;
  v_state jsonb;
  v_prev jsonb;
  v_row RECORD;
  v_next_date date;
  v_validity daterange;
  v_version_id bigint;
  v_family_ids uuid[];
  v_primary_family_id uuid;
  v_family_id uuid;
BEGIN
  PERFORM jobcatalog.assert_current_tenant(p_tenant_uuid);
  v_setid := jobcatalog.normalize_setid(p_setid);

  v_lock_key := format('jobcatalog:write-lock:%s:%s', p_tenant_uuid, 'JobCatalog');
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM jobcatalog.job_profile_versions
  WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid AND job_profile_uuid = p_job_profile_uuid;

  v_prev := NULL;
  FOR v_row IN
    SELECT id, event_type, effective_date, payload
    FROM jobcatalog.job_profile_events
    WHERE tenant_uuid = p_tenant_uuid
      AND setid = v_setid
      AND job_profile_uuid = p_job_profile_uuid
    ORDER BY effective_date ASC, id ASC
  LOOP
    v_state := COALESCE(v_prev, '{}'::jsonb);

    IF v_row.event_type = 'CREATE' THEN
      IF v_prev IS NOT NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_EVENT',
          DETAIL = 'CREATE must be the first event';
      END IF;
      v_state := jsonb_build_object(
        'name', v_row.payload->>'name',
        'description', v_row.payload->'description',
        'is_active', true,
        'external_refs', COALESCE(v_row.payload->'external_refs', '{}'::jsonb),
        'job_family_uuids', COALESCE(v_row.payload->'job_family_uuids', '[]'::jsonb),
        'primary_job_family_uuid', v_row.payload->>'primary_job_family_uuid'
      );
    ELSIF v_row.event_type = 'UPDATE' THEN
      IF v_prev IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_EVENT',
          DETAIL = 'UPDATE requires prior state';
      END IF;
      IF v_row.payload ? 'is_active' THEN
        v_state := jsonb_set(v_state, '{is_active}', v_row.payload->'is_active', true);
      END IF;
      IF v_row.payload ? 'name' THEN
        v_state := jsonb_set(v_state, '{name}', to_jsonb(v_row.payload->>'name'), true);
      END IF;
      IF v_row.payload ? 'description' THEN
        v_state := jsonb_set(v_state, '{description}', v_row.payload->'description', true);
      END IF;
      IF v_row.payload ? 'external_refs' THEN
        v_state := jsonb_set(v_state, '{external_refs}', v_row.payload->'external_refs', true);
      END IF;
      IF v_row.payload ? 'job_family_uuids' THEN
        v_state := jsonb_set(v_state, '{job_family_uuids}', v_row.payload->'job_family_uuids', true);
      END IF;
      IF v_row.payload ? 'primary_job_family_uuid' THEN
        v_state := jsonb_set(v_state, '{primary_job_family_uuid}', to_jsonb(v_row.payload->>'primary_job_family_uuid'), true);
      END IF;
    ELSIF v_row.event_type = 'DISABLE' THEN
      IF v_prev IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_EVENT',
          DETAIL = 'DISABLE requires prior state';
      END IF;
      v_state := jsonb_set(v_state, '{is_active}', 'false'::jsonb, true);
    ELSE
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_EVENT',
        DETAIL = format('unsupported event_type=%s', v_row.event_type);
    END IF;

    IF jsonb_typeof(v_state->'job_family_uuids') <> 'array' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'job_family_uuids must be an array';
    END IF;
    IF jsonb_array_length(COALESCE(v_state->'job_family_uuids', '[]'::jsonb)) = 0 THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'job_family_uuids must be non-empty';
    END IF;

    BEGIN
      SELECT array_agg(NULLIF(btrim(value), '')::uuid) INTO v_family_ids
      FROM jsonb_array_elements_text(v_state->'job_family_uuids') AS t(value);
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
          DETAIL = 'job_family_uuids contains invalid uuid';
    END;
    IF v_family_ids IS NULL OR array_length(v_family_ids, 1) IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'job_family_uuids must be non-empty';
    END IF;
    IF (SELECT count(*) <> count(DISTINCT id) FROM unnest(v_family_ids) AS t(id)) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'job_family_uuids contains duplicates';
    END IF;

    BEGIN
      v_primary_family_id := NULLIF(btrim(COALESCE(v_state->>'primary_job_family_uuid', '')), '')::uuid;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
          DETAIL = 'primary_job_family_uuid is invalid';
    END;
    IF v_primary_family_id IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'primary_job_family_uuid is required';
    END IF;
    IF NOT (v_primary_family_id = ANY(v_family_ids)) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'primary_job_family_uuid must be included in job_family_uuids';
    END IF;

    v_next_date := NULL;
    SELECT e.effective_date INTO v_next_date
    FROM jobcatalog.job_profile_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.setid = v_setid
      AND e.job_profile_uuid = p_job_profile_uuid
      AND (e.effective_date, e.id) > (v_row.effective_date, v_row.id)
    ORDER BY e.effective_date ASC, e.id ASC
    LIMIT 1;

    v_validity := daterange(v_row.effective_date, v_next_date, '[)');

    INSERT INTO jobcatalog.job_profile_versions (
      tenant_uuid,
      setid,
      job_profile_uuid,
      validity,
      name,
      description,
      is_active,
      external_refs,
      last_event_id
    ) VALUES (
      p_tenant_uuid,
      v_setid,
      p_job_profile_uuid,
      v_validity,
      COALESCE(NULLIF(btrim(v_state->>'name'), ''), '[missing]'),
      CASE
        WHEN jsonb_typeof(v_state->'description') = 'null' THEN NULL
        ELSE v_state->>'description'
      END,
      COALESCE((v_state->>'is_active')::boolean, true),
      COALESCE(v_state->'external_refs', '{}'::jsonb),
      v_row.id
    )
    RETURNING id INTO v_version_id;

    FOREACH v_family_id IN ARRAY v_family_ids LOOP
      INSERT INTO jobcatalog.job_profile_version_job_families (
        tenant_uuid,
        setid,
        job_profile_version_id,
        job_family_uuid,
        is_primary
      ) VALUES (
        p_tenant_uuid,
        v_setid,
        v_version_id,
        v_family_id,
        v_family_id = v_primary_family_id
      );
    END LOOP;

    v_prev := v_state;
  END LOOP;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        validity,
        lag(validity) OVER (ORDER BY lower(validity)) AS prev_validity
      FROM jobcatalog.job_profile_versions
      WHERE tenant_uuid = p_tenant_uuid
        AND setid = v_setid
        AND job_profile_uuid = p_job_profile_uuid
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL
      AND lower(validity) <> upper(prev_validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_VALIDITY_GAP',
      DETAIL = format('job_profile_uuid=%s', p_job_profile_uuid);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (
      SELECT validity
      FROM jobcatalog.job_profile_versions
      WHERE tenant_uuid = p_tenant_uuid
        AND setid = v_setid
        AND job_profile_uuid = p_job_profile_uuid
      ORDER BY lower(validity) DESC
      LIMIT 1
    ) last
    WHERE NOT upper_inf(last.validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_VALIDITY_NOT_INFINITE',
      DETAIL = format('job_profile_uuid=%s', p_job_profile_uuid);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.submit_job_profile_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_setid text,
  p_job_profile_uuid uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_setid text;
  v_evt_db_id bigint;
  v_code text;
  v_name text;
  v_payload jsonb;
  v_existing jobcatalog.job_profile_events%ROWTYPE;
  v_existing_profile jobcatalog.job_profiles%ROWTYPE;
  v_family_ids uuid[];
  v_primary_family_id uuid;
  v_missing_family_id uuid;
BEGIN
  PERFORM jobcatalog.assert_current_tenant(p_tenant_uuid);
  IF p_event_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'event_uuid is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'request_id is required';
  END IF;
  IF p_job_profile_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'job_profile_uuid is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'effective_date is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'initiator_uuid is required';
  END IF;
  IF p_event_type NOT IN ('CREATE','UPDATE','DISABLE') THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type=%s', p_event_type);
  END IF;

  v_setid := jobcatalog.normalize_setid(p_setid);

  v_lock_key := format('jobcatalog:write-lock:%s:%s', p_tenant_uuid, 'JobCatalog');
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'payload must be an object';
  END IF;

  IF p_event_type = 'CREATE' THEN
    IF EXISTS (
      SELECT 1
      FROM jsonb_object_keys(v_payload) AS k
      WHERE k NOT IN ('job_profile_code', 'name', 'description', 'external_refs', 'job_family_uuids', 'primary_job_family_uuid')
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload has unknown keys for CREATE';
    END IF;
    IF v_payload ? 'description' AND jsonb_typeof(v_payload->'description') NOT IN ('string','null') THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.description must be string or null';
    END IF;
    IF v_payload ? 'external_refs' AND jsonb_typeof(v_payload->'external_refs') <> 'object' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.external_refs must be an object';
    END IF;
    IF jsonb_typeof(v_payload->'job_family_uuids') <> 'array' OR jsonb_array_length(v_payload->'job_family_uuids') = 0 THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.job_family_uuids must be a non-empty array';
    END IF;
    IF NULLIF(btrim(COALESCE(v_payload->>'primary_job_family_uuid', '')), '') IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.primary_job_family_uuid is required';
    END IF;
  ELSIF p_event_type = 'UPDATE' THEN
    IF v_payload ? 'job_profile_code' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.job_profile_code is not allowed for UPDATE';
    END IF;
    IF EXISTS (
      SELECT 1
      FROM jsonb_object_keys(v_payload) AS k
      WHERE k NOT IN ('name', 'description', 'is_active', 'external_refs', 'job_family_uuids', 'primary_job_family_uuid')
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload has unknown keys for UPDATE';
    END IF;
    IF v_payload ? 'name' AND NULLIF(btrim(COALESCE(v_payload->>'name', '')), '') IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.name must be non-empty';
    END IF;
    IF v_payload ? 'description' AND jsonb_typeof(v_payload->'description') NOT IN ('string','null') THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.description must be string or null';
    END IF;
    IF v_payload ? 'is_active' AND jsonb_typeof(v_payload->'is_active') <> 'boolean' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.is_active must be boolean';
    END IF;
    IF v_payload ? 'external_refs' AND jsonb_typeof(v_payload->'external_refs') <> 'object' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.external_refs must be an object';
    END IF;
    IF v_payload ? 'job_family_uuids' THEN
      IF jsonb_typeof(v_payload->'job_family_uuids') <> 'array' OR jsonb_array_length(v_payload->'job_family_uuids') = 0 THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
          DETAIL = 'payload.job_family_uuids must be a non-empty array';
      END IF;
      IF NOT (v_payload ? 'primary_job_family_uuid') THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
          DETAIL = 'payload.primary_job_family_uuid is required when job_family_uuids is present';
      END IF;
    END IF;
    IF v_payload ? 'primary_job_family_uuid' AND NULLIF(btrim(COALESCE(v_payload->>'primary_job_family_uuid', '')), '') IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.primary_job_family_uuid must be non-empty';
    END IF;
    IF NOT (v_payload ? 'name' OR v_payload ? 'description' OR v_payload ? 'is_active' OR v_payload ? 'external_refs' OR v_payload ? 'job_family_uuids' OR v_payload ? 'primary_job_family_uuid') THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'UPDATE payload must include at least one patch field';
    END IF;
  ELSE
    IF v_payload <> '{}'::jsonb THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'DISABLE payload must be empty';
    END IF;
  END IF;

  IF v_payload ? 'job_family_uuids' THEN
    BEGIN
      SELECT array_agg(NULLIF(btrim(value), '')::uuid) INTO v_family_ids
      FROM jsonb_array_elements_text(v_payload->'job_family_uuids') AS t(value);
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
          DETAIL = 'payload.job_family_uuids contains invalid uuid';
    END;
    IF v_family_ids IS NULL OR array_length(v_family_ids, 1) IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.job_family_uuids must be non-empty';
    END IF;
    IF (SELECT count(*) <> count(DISTINCT id) FROM unnest(v_family_ids) AS t(id)) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.job_family_uuids contains duplicates';
    END IF;
  END IF;

  IF v_payload ? 'primary_job_family_uuid' THEN
    BEGIN
      v_primary_family_id := NULLIF(btrim(COALESCE(v_payload->>'primary_job_family_uuid', '')), '')::uuid;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
          DETAIL = 'payload.primary_job_family_uuid is invalid';
    END;
    IF v_primary_family_id IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.primary_job_family_uuid must be non-empty';
    END IF;
  END IF;

  IF v_family_ids IS NOT NULL AND v_primary_family_id IS NOT NULL THEN
    IF NOT (v_primary_family_id = ANY(v_family_ids)) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.primary_job_family_uuid must be included in payload.job_family_uuids';
    END IF;
  END IF;

  IF p_event_type = 'CREATE' THEN
    v_code := NULLIF(btrim(COALESCE(v_payload->>'job_profile_code', '')), '');
    v_name := NULLIF(btrim(COALESCE(v_payload->>'name', '')), '');
    IF v_code IS NULL OR v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'job_profile_code/name is required';
    END IF;

    INSERT INTO jobcatalog.job_profiles (tenant_uuid, setid, job_profile_uuid, job_profile_code)
    VALUES (p_tenant_uuid, v_setid, p_job_profile_uuid, v_code)
    ON CONFLICT (job_profile_uuid) DO NOTHING;

    SELECT * INTO v_existing_profile
    FROM jobcatalog.job_profiles
    WHERE job_profile_uuid = p_job_profile_uuid;

    IF v_existing_profile.tenant_uuid <> p_tenant_uuid
      OR v_existing_profile.setid <> v_setid
      OR v_existing_profile.job_profile_code <> v_code
    THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = format('job_profile_uuid=%s', p_job_profile_uuid);
    END IF;
  ELSE
    IF NOT EXISTS (
      SELECT 1 FROM jobcatalog.job_profiles
      WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid AND job_profile_uuid = p_job_profile_uuid
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_NOT_FOUND',
        DETAIL = format('job_profile_uuid=%s', p_job_profile_uuid);
    END IF;
  END IF;

  IF v_family_ids IS NOT NULL THEN
    SELECT missing.job_family_uuid INTO v_missing_family_id
    FROM (
      SELECT t.id AS job_family_uuid
      FROM unnest(v_family_ids) AS t(id)
      LEFT JOIN jobcatalog.job_families f
        ON f.tenant_uuid = p_tenant_uuid
       AND f.setid = v_setid
       AND f.job_family_uuid = t.id
      WHERE f.job_family_uuid IS NULL
      LIMIT 1
    ) missing;
    IF v_missing_family_id IS NOT NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
        DETAIL = format('job_family_uuid=%s', v_missing_family_id);
    END IF;
  END IF;

  IF v_primary_family_id IS NOT NULL THEN
    IF NOT EXISTS (
      SELECT 1
      FROM jobcatalog.job_families
      WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid AND job_family_uuid = v_primary_family_id
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
        DETAIL = format('job_family_uuid=%s', v_primary_family_id);
    END IF;
  END IF;

  INSERT INTO jobcatalog.job_profile_events (
    event_uuid, tenant_uuid, setid, job_profile_uuid, event_type, effective_date, payload, request_id, initiator_uuid
  )
  VALUES (
    p_event_uuid, p_tenant_uuid, v_setid, p_job_profile_uuid, p_event_type, p_effective_date, v_payload, p_request_id, p_initiator_uuid
  )
  ON CONFLICT (event_uuid) DO NOTHING
  RETURNING id INTO v_evt_db_id;

  IF v_evt_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM jobcatalog.job_profile_events
    WHERE event_uuid = p_event_uuid;

    IF v_existing.tenant_uuid <> p_tenant_uuid
      OR v_existing.setid <> v_setid
      OR v_existing.job_profile_uuid <> p_job_profile_uuid
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_IDEMPOTENCY_REUSED',
        DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  PERFORM jobcatalog.replay_job_profile_versions(p_tenant_uuid, v_setid, p_job_profile_uuid);

  RETURN v_evt_db_id;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS jobcatalog.submit_job_profile_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid);
DROP FUNCTION IF EXISTS jobcatalog.replay_job_profile_versions(uuid, text, uuid);
-- +goose StatementEnd
