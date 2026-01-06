-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION jobcatalog.assert_current_tenant(p_tenant_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_ctx_raw text;
  v_ctx_tenant uuid;
BEGIN
  IF p_tenant_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'tenant_id is required';
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

  IF v_ctx_tenant <> p_tenant_id THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_MISMATCH',
      DETAIL = format('tenant_param=%s tenant_ctx=%s', p_tenant_id, v_ctx_tenant);
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
  p_tenant_id uuid,
  p_setid text,
  p_job_family_group_id uuid
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
  PERFORM jobcatalog.assert_current_tenant(p_tenant_id);
  v_setid := jobcatalog.normalize_setid(p_setid);

  DELETE FROM jobcatalog.job_family_group_versions
  WHERE tenant_id = p_tenant_id AND setid = v_setid AND job_family_group_id = p_job_family_group_id;

  v_prev := NULL;
  FOR v_row IN
    SELECT id, event_type, effective_date, payload
    FROM jobcatalog.job_family_group_events
    WHERE tenant_id = p_tenant_id
      AND setid = v_setid
      AND job_family_group_id = p_job_family_group_id
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
    WHERE e.tenant_id = p_tenant_id
      AND e.setid = v_setid
      AND e.job_family_group_id = p_job_family_group_id
      AND (e.effective_date, e.id) > (v_row.effective_date, v_row.id)
    ORDER BY e.effective_date ASC, e.id ASC
    LIMIT 1;

    v_validity := daterange(v_row.effective_date, v_next_date, '[)');

    INSERT INTO jobcatalog.job_family_group_versions (
      tenant_id,
      setid,
      job_family_group_id,
      validity,
      name,
      description,
      is_active,
      external_refs,
      last_event_id
    ) VALUES (
      p_tenant_id,
      v_setid,
      p_job_family_group_id,
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
  p_event_id uuid,
  p_tenant_id uuid,
  p_setid text,
  p_job_family_group_id uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
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
  PERFORM jobcatalog.assert_current_tenant(p_tenant_id);
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'request_id is required';
  END IF;
  IF p_job_family_group_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'job_family_group_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
      DETAIL = 'effective_date is required';
  END IF;

  v_setid := jobcatalog.normalize_setid(p_setid);

  IF p_event_type = 'CREATE' THEN
    v_code := NULLIF(btrim(COALESCE(p_payload->>'code', '')), '');
    v_name := NULLIF(btrim(COALESCE(p_payload->>'name', '')), '');
    IF v_code IS NULL OR v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_INVALID_ARGUMENT',
        DETAIL = 'code/name is required';
    END IF;

    INSERT INTO jobcatalog.job_family_groups (tenant_id, setid, id, code)
    VALUES (p_tenant_id, v_setid, p_job_family_group_id, v_code);
  ELSE
    IF NOT EXISTS (
      SELECT 1 FROM jobcatalog.job_family_groups
      WHERE tenant_id = p_tenant_id AND setid = v_setid AND id = p_job_family_group_id
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'JOBCATALOG_NOT_FOUND',
        DETAIL = format('job_family_group_id=%s', p_job_family_group_id);
    END IF;
  END IF;

  INSERT INTO jobcatalog.job_family_group_events (
    event_id, tenant_id, setid, job_family_group_id, event_type, effective_date, payload, request_id, initiator_id
  )
  VALUES (
    p_event_id, p_tenant_id, v_setid, p_job_family_group_id, p_event_type, p_effective_date, COALESCE(p_payload, '{}'::jsonb), p_request_id, p_initiator_id
  )
  RETURNING id INTO v_evt_db_id;

  PERFORM jobcatalog.replay_job_family_group_versions(p_tenant_id, v_setid, p_job_family_group_id);

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
