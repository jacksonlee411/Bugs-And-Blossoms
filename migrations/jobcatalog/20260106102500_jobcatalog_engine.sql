-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION jobcatalog.assert_current_tenant(p_tenant_uuid uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_ctx_raw text;
  v_ctx_tenant uuid;
BEGIN
  IF p_tenant_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'tenant_uuid is required';
  END IF;

  v_ctx_raw := current_setting('app.current_tenant', true);
  IF v_ctx_raw IS NULL OR btrim(v_ctx_raw) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_CONTEXT_MISSING',
      DETAIL = 'app.current_tenant is required';
  END IF;

  BEGIN
    v_ctx_tenant := v_ctx_raw::uuid;
  EXCEPTION
    WHEN invalid_text_representation THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'RLS_TENANT_CONTEXT_INVALID',
        DETAIL = format('app.current_tenant=%s', v_ctx_raw);
  END;

  IF v_ctx_tenant <> p_tenant_uuid THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_MISMATCH',
      DETAIL = format('tenant_param=%s tenant_ctx=%s', p_tenant_uuid, v_ctx_tenant);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.normalize_setid(p_setid text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  v text;
BEGIN
  IF p_setid IS NULL OR btrim(p_setid) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'setid is required';
  END IF;
  v := upper(btrim(p_setid));
  IF v !~ '^[A-Z0-9]{1,5}$' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = format('setid=%s', v);
  END IF;
  RETURN v;
END;
$$;

CREATE OR REPLACE FUNCTION jobcatalog.replay_job_family_group_versions(
  p_tenant_uuid uuid,
  p_setid text,
  p_job_family_group_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_setid text;
  v_state jsonb;
  v_prev jsonb;
  v_row RECORD;
  v_next_date date;
  v_validity daterange;
BEGIN
  PERFORM jobcatalog.assert_current_tenant(p_tenant_uuid);
  v_setid := jobcatalog.normalize_setid(p_setid);

  DELETE FROM jobcatalog.job_family_group_versions
  WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid AND job_family_group_uuid = p_job_family_group_uuid;

  v_prev := NULL;
  FOR v_row IN
    SELECT id, event_type, effective_date, payload
    FROM jobcatalog.job_family_group_events
    WHERE tenant_uuid = p_tenant_uuid
      AND setid = v_setid
      AND job_family_group_uuid = p_job_family_group_uuid
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
        'external_refs', COALESCE(v_row.payload->'external_refs', '{}'::jsonb)
      );
    ELSIF v_row.event_type = 'UPDATE' THEN
      IF v_prev IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_INVALID_EVENT',
          DETAIL = 'UPDATE requires prior state';
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
    FROM jobcatalog.job_family_group_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.setid = v_setid
      AND e.job_family_group_uuid = p_job_family_group_uuid
      AND (e.effective_date, e.id) > (v_row.effective_date, v_row.id)
    ORDER BY e.effective_date ASC, e.id ASC
    LIMIT 1;

    v_validity := daterange(v_row.effective_date, v_next_date, '[)');

    INSERT INTO jobcatalog.job_family_group_versions (
      tenant_uuid,
      setid,
      job_family_group_uuid,
      validity,
      name,
      description,
      is_active,
      external_refs,
      last_event_id
    ) VALUES (
      p_tenant_uuid,
      v_setid,
      p_job_family_group_uuid,
      v_validity,
      COALESCE(NULLIF(btrim(v_state->>'name'), ''), '[missing]'),
      CASE
        WHEN jsonb_typeof(v_state->'description') = 'null' THEN NULL
        ELSE v_state->>'description'
      END,
      COALESCE((v_state->>'is_active')::boolean, true),
      COALESCE(v_state->'external_refs', '{}'::jsonb),
      v_row.id
    );

    v_prev := v_state;
  END LOOP;
END;
$$;

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
  v_setid text;
  v_evt_db_id bigint;
  v_code text;
  v_name text;
BEGIN
  PERFORM jobcatalog.assert_current_tenant(p_tenant_uuid);
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

  v_setid := jobcatalog.normalize_setid(p_setid);

  IF p_event_type = 'CREATE' THEN
    v_code := NULLIF(btrim(COALESCE(p_payload->>'job_family_group_code', '')), '');
    v_name := NULLIF(btrim(COALESCE(p_payload->>'name', '')), '');
    IF v_code IS NULL OR v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'job_family_group_code/name is required';
    END IF;

    INSERT INTO jobcatalog.job_family_groups (tenant_uuid, setid, job_family_group_uuid, job_family_group_code)
    VALUES (p_tenant_uuid, v_setid, p_job_family_group_uuid, v_code);
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
    p_event_uuid, p_tenant_uuid, v_setid, p_job_family_group_uuid, p_event_type, p_effective_date, COALESCE(p_payload, '{}'::jsonb), p_request_code, p_initiator_uuid
  )
  RETURNING id INTO v_evt_db_id;

  PERFORM jobcatalog.replay_job_family_group_versions(p_tenant_uuid, v_setid, p_job_family_group_uuid);

  RETURN v_evt_db_id;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS jobcatalog.submit_job_family_group_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid);
DROP FUNCTION IF EXISTS jobcatalog.replay_job_family_group_versions(uuid, text, uuid);
DROP FUNCTION IF EXISTS jobcatalog.normalize_setid(text);
DROP FUNCTION IF EXISTS jobcatalog.assert_current_tenant(uuid);
-- +goose StatementEnd
