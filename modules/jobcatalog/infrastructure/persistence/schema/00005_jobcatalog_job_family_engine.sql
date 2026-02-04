CREATE OR REPLACE FUNCTION jobcatalog.replay_job_family_versions(
  p_tenant_uuid uuid,
  p_package_uuid text,
  p_job_family_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_package_uuid uuid;
  v_state jsonb;
  v_prev jsonb;
  v_row RECORD;
  v_next_date date;
  v_validity daterange;
  v_group_id uuid;
BEGIN
  PERFORM jobcatalog.assert_current_tenant(p_tenant_uuid);
  v_package_uuid := jobcatalog.normalize_package_uuid(p_package_uuid);

  v_lock_key := format('jobcatalog:write-lock:%s:%s', p_tenant_uuid, 'JobCatalog');
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM jobcatalog.job_family_versions
  WHERE tenant_uuid = p_tenant_uuid AND package_uuid = v_package_uuid AND job_family_uuid = p_job_family_uuid;

  v_prev := NULL;
  FOR v_row IN
    SELECT id, event_type, effective_date, payload
    FROM jobcatalog.job_family_events
    WHERE tenant_uuid = p_tenant_uuid
      AND package_uuid = v_package_uuid
      AND job_family_uuid = p_job_family_uuid
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

      v_group_id := NULLIF(btrim(COALESCE(v_row.payload->>'job_family_group_uuid', '')), '')::uuid;
      v_state := jsonb_build_object(
        'name', v_row.payload->>'name',
        'description', v_row.payload->'description',
        'is_active', true,
        'external_refs', COALESCE(v_row.payload->'external_refs', '{}'::jsonb),
        'job_family_group_uuid', v_group_id::text
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
      IF v_row.payload ? 'job_family_group_uuid' THEN
        v_state := jsonb_set(v_state, '{job_family_group_uuid}', to_jsonb(v_row.payload->>'job_family_group_uuid'), true);
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

    v_next_date := NULL;
    SELECT e.effective_date INTO v_next_date
    FROM jobcatalog.job_family_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.package_uuid = v_package_uuid
      AND e.job_family_uuid = p_job_family_uuid
      AND (e.effective_date, e.id) > (v_row.effective_date, v_row.id)
    ORDER BY e.effective_date ASC, e.id ASC
    LIMIT 1;

    v_validity := daterange(v_row.effective_date, v_next_date, '[)');

    INSERT INTO jobcatalog.job_family_versions (
      tenant_uuid,
      package_uuid,
      job_family_uuid,
      validity,
      name,
      description,
      is_active,
      external_refs,
      job_family_group_uuid,
      last_event_id
    ) VALUES (
      p_tenant_uuid,
      v_package_uuid,
      p_job_family_uuid,
      v_validity,
      COALESCE(NULLIF(btrim(v_state->>'name'), ''), '[missing]'),
      CASE
        WHEN jsonb_typeof(v_state->'description') = 'null' THEN NULL
        ELSE v_state->>'description'
      END,
      COALESCE((v_state->>'is_active')::boolean, true),
      COALESCE(v_state->'external_refs', '{}'::jsonb),
      NULLIF(btrim(COALESCE(v_state->>'job_family_group_uuid', '')), '')::uuid,
      v_row.id
    );

    v_prev := v_state;
  END LOOP;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        validity,
        lag(validity) OVER (ORDER BY lower(validity)) AS prev_validity
      FROM jobcatalog.job_family_versions
      WHERE tenant_uuid = p_tenant_uuid
        AND package_uuid = v_package_uuid
        AND job_family_uuid = p_job_family_uuid
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
      DETAIL = format('job_family_uuid=%s', p_job_family_uuid);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (
      SELECT validity
      FROM jobcatalog.job_family_versions
      WHERE tenant_uuid = p_tenant_uuid
        AND package_uuid = v_package_uuid
        AND job_family_uuid = p_job_family_uuid
      ORDER BY lower(validity) DESC
      LIMIT 1
    ) last
    WHERE NOT upper_inf(last.validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_VALIDITY_NOT_INFINITE',
      DETAIL = format('job_family_uuid=%s', p_job_family_uuid);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.replay_job_family_versions(
  p_tenant_uuid uuid,
  p_package_uuid uuid,
  p_job_family_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM jobcatalog.replay_job_family_versions(p_tenant_uuid, p_package_uuid::text, p_job_family_uuid);
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.submit_job_family_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_package_uuid text,
  p_job_family_uuid uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_code text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_package_uuid uuid;
  v_evt_db_id bigint;
  v_code text;
  v_name text;
  v_payload jsonb;
  v_existing jobcatalog.job_family_events%ROWTYPE;
  v_existing_family jobcatalog.job_families%ROWTYPE;
  v_group_id uuid;
BEGIN
  PERFORM jobcatalog.assert_current_tenant(p_tenant_uuid);
  IF p_event_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'event_uuid is required';
  END IF;
  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'request_code is required';
  END IF;
  IF p_job_family_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'job_family_uuid is required';
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

  v_package_uuid := jobcatalog.normalize_package_uuid(p_package_uuid);

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
      WHERE k NOT IN ('job_family_code', 'name', 'description', 'external_refs', 'job_family_group_uuid')
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
  ELSIF p_event_type = 'UPDATE' THEN
    IF v_payload ? 'job_family_code' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.job_family_code is not allowed for UPDATE';
    END IF;
    IF EXISTS (
      SELECT 1
      FROM jsonb_object_keys(v_payload) AS k
      WHERE k NOT IN ('name', 'description', 'is_active', 'external_refs', 'job_family_group_uuid')
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
    IF v_payload ? 'job_family_group_uuid' AND NULLIF(btrim(COALESCE(v_payload->>'job_family_group_uuid', '')), '') IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.job_family_group_uuid must be non-empty';
    END IF;
    IF NOT (v_payload ? 'name' OR v_payload ? 'description' OR v_payload ? 'is_active' OR v_payload ? 'external_refs' OR v_payload ? 'job_family_group_uuid') THEN
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

  IF p_event_type = 'CREATE' THEN
    v_code := NULLIF(btrim(COALESCE(v_payload->>'job_family_code', '')), '');
    v_name := NULLIF(btrim(COALESCE(v_payload->>'name', '')), '');
    IF v_code IS NULL OR v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'job_family_code/name is required';
    END IF;

    BEGIN
      v_group_id := NULLIF(btrim(COALESCE(v_payload->>'job_family_group_uuid', '')), '')::uuid;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
          DETAIL = 'job_family_group_uuid must be uuid';
    END;

    IF v_group_id IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'job_family_group_uuid is required';
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM jobcatalog.job_family_groups
      WHERE tenant_uuid = p_tenant_uuid AND package_uuid = v_package_uuid AND job_family_group_uuid = v_group_id
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
        DETAIL = format('job_family_group_uuid=%s', v_group_id);
    END IF;

    INSERT INTO jobcatalog.job_families (tenant_uuid, package_uuid, job_family_uuid, job_family_code)
    VALUES (p_tenant_uuid, v_package_uuid, p_job_family_uuid, v_code)
    ON CONFLICT (job_family_uuid) DO NOTHING;

    SELECT * INTO v_existing_family
    FROM jobcatalog.job_families
    WHERE job_family_uuid = p_job_family_uuid;

    IF v_existing_family.tenant_uuid <> p_tenant_uuid
      OR v_existing_family.package_uuid <> v_package_uuid
      OR v_existing_family.job_family_code <> v_code
    THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = format('job_family_uuid=%s', p_job_family_uuid);
    END IF;
  ELSE
    IF NOT EXISTS (
      SELECT 1 FROM jobcatalog.job_families
      WHERE tenant_uuid = p_tenant_uuid AND package_uuid = v_package_uuid AND job_family_uuid = p_job_family_uuid
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_NOT_FOUND',
        DETAIL = format('job_family_uuid=%s', p_job_family_uuid);
    END IF;
  END IF;

  IF (p_event_type = 'CREATE' OR p_event_type = 'UPDATE') AND v_payload ? 'job_family_group_uuid' THEN
    BEGIN
      v_group_id := NULLIF(btrim(COALESCE(v_payload->>'job_family_group_uuid', '')), '')::uuid;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
          DETAIL = 'job_family_group_uuid must be uuid';
    END;

    IF v_group_id IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'job_family_group_uuid must be non-empty';
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM jobcatalog.job_family_groups
      WHERE tenant_uuid = p_tenant_uuid AND package_uuid = v_package_uuid AND job_family_group_uuid = v_group_id
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
        DETAIL = format('job_family_group_uuid=%s', v_group_id);
    END IF;
  END IF;

  INSERT INTO jobcatalog.job_family_events (
    event_uuid, tenant_uuid, package_uuid, job_family_uuid, event_type, effective_date, payload, request_code, initiator_uuid
  )
  VALUES (
    p_event_uuid, p_tenant_uuid, v_package_uuid, p_job_family_uuid, p_event_type, p_effective_date, v_payload, p_request_code, p_initiator_uuid
  )
  ON CONFLICT (event_uuid) DO NOTHING
  RETURNING id INTO v_evt_db_id;

  IF v_evt_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM jobcatalog.job_family_events
    WHERE event_uuid = p_event_uuid;

    IF v_existing.tenant_uuid <> p_tenant_uuid
      OR v_existing.package_uuid <> v_package_uuid
      OR v_existing.job_family_uuid <> p_job_family_uuid
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_code <> p_request_code
      OR v_existing.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_IDEMPOTENCY_REUSED',
        DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  PERFORM jobcatalog.replay_job_family_versions(p_tenant_uuid, v_package_uuid, p_job_family_uuid);

  RETURN v_evt_db_id;
END;
$$;
