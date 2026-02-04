-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION jobcatalog.submit_job_family_group_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_setid text,
  p_job_family_group_uuid uuid,
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
  v_setid text;
  v_evt_db_id bigint;
  v_code text;
  v_name text;
  v_payload jsonb;
  v_existing jobcatalog.job_family_group_events%ROWTYPE;
  v_existing_group jobcatalog.job_family_groups%ROWTYPE;
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
  IF p_job_family_group_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'job_family_group_uuid is required';
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
      WHERE k NOT IN ('job_family_group_code', 'name', 'description', 'external_refs')
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
    IF v_payload ? 'job_family_group_code' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'payload.job_family_group_code is not allowed for UPDATE';
    END IF;
    IF EXISTS (
      SELECT 1
      FROM jsonb_object_keys(v_payload) AS k
      WHERE k NOT IN ('name', 'description', 'is_active', 'external_refs')
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
    IF NOT (v_payload ? 'name' OR v_payload ? 'description' OR v_payload ? 'is_active' OR v_payload ? 'external_refs') THEN
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
    v_code := NULLIF(btrim(COALESCE(v_payload->>'job_family_group_code', '')), '');
    v_name := NULLIF(btrim(COALESCE(v_payload->>'name', '')), '');
    IF v_code IS NULL OR v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'job_family_group_code/name is required';
    END IF;

    INSERT INTO jobcatalog.job_family_groups (tenant_uuid, setid, job_family_group_uuid, job_family_group_code)
    VALUES (p_tenant_uuid, v_setid, p_job_family_group_uuid, v_code)
    ON CONFLICT (job_family_group_uuid) DO NOTHING;

    SELECT * INTO v_existing_group
    FROM jobcatalog.job_family_groups
    WHERE job_family_group_uuid = p_job_family_group_uuid;

    IF v_existing_group.tenant_uuid <> p_tenant_uuid
      OR v_existing_group.setid <> v_setid
      OR v_existing_group.job_family_group_code <> v_code
    THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = format('job_family_group_uuid=%s', p_job_family_group_uuid);
    END IF;
  ELSE
    IF NOT EXISTS (
      SELECT 1 FROM jobcatalog.job_family_groups
      WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid AND job_family_group_uuid = p_job_family_group_uuid
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_NOT_FOUND',
        DETAIL = format('job_family_group_uuid=%s', p_job_family_group_uuid);
    END IF;
  END IF;

  INSERT INTO jobcatalog.job_family_group_events (
    event_uuid, tenant_uuid, setid, job_family_group_uuid, event_type, effective_date, payload, request_code, initiator_uuid
  )
  VALUES (
    p_event_uuid, p_tenant_uuid, v_setid, p_job_family_group_uuid, p_event_type, p_effective_date, v_payload, p_request_code, p_initiator_uuid
  )
  ON CONFLICT (event_uuid) DO NOTHING
  RETURNING id INTO v_evt_db_id;

  IF v_evt_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM jobcatalog.job_family_group_events
    WHERE event_uuid = p_event_uuid;

    IF v_existing.tenant_uuid <> p_tenant_uuid
      OR v_existing.setid <> v_setid
      OR v_existing.job_family_group_uuid <> p_job_family_group_uuid
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

  PERFORM jobcatalog.replay_job_family_group_versions(p_tenant_uuid, v_setid, p_job_family_group_uuid);

  RETURN v_evt_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.submit_job_family_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_setid text,
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
  v_setid text;
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
      WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid AND job_family_group_uuid = v_group_id
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
        DETAIL = format('job_family_group_uuid=%s', v_group_id);
    END IF;

    INSERT INTO jobcatalog.job_families (tenant_uuid, setid, job_family_uuid, job_family_code)
    VALUES (p_tenant_uuid, v_setid, p_job_family_uuid, v_code)
    ON CONFLICT (job_family_uuid) DO NOTHING;

    SELECT * INTO v_existing_family
    FROM jobcatalog.job_families
    WHERE job_family_uuid = p_job_family_uuid;

    IF v_existing_family.tenant_uuid <> p_tenant_uuid
      OR v_existing_family.setid <> v_setid
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
      WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid AND job_family_uuid = p_job_family_uuid
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
      WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid AND job_family_group_uuid = v_group_id
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
        DETAIL = format('job_family_group_uuid=%s', v_group_id);
    END IF;
  END IF;

  INSERT INTO jobcatalog.job_family_events (
    event_uuid, tenant_uuid, setid, job_family_uuid, event_type, effective_date, payload, request_code, initiator_uuid
  )
  VALUES (
    p_event_uuid, p_tenant_uuid, v_setid, p_job_family_uuid, p_event_type, p_effective_date, v_payload, p_request_code, p_initiator_uuid
  )
  ON CONFLICT (event_uuid) DO NOTHING
  RETURNING id INTO v_evt_db_id;

  IF v_evt_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM jobcatalog.job_family_events
    WHERE event_uuid = p_event_uuid;

    IF v_existing.tenant_uuid <> p_tenant_uuid
      OR v_existing.setid <> v_setid
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

  PERFORM jobcatalog.replay_job_family_versions(p_tenant_uuid, v_setid, p_job_family_uuid);

  RETURN v_evt_db_id;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
