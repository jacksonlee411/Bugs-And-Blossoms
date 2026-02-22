CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS iam.dict_value_segments (
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  dict_code text NOT NULL,
  code text NOT NULL,
  label text NOT NULL,
  enabled_on date NOT NULL,
  disabled_on date NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, dict_code, code, enabled_on),
  CONSTRAINT dict_value_segments_dict_code_check CHECK (
    dict_code = lower(dict_code)
    AND dict_code = btrim(dict_code)
    AND dict_code ~ '^[a-z][a-z0-9_]{0,63}$'
  ),
  CONSTRAINT dict_value_segments_code_check CHECK (code = btrim(code) AND code <> ''),
  CONSTRAINT dict_value_segments_label_check CHECK (label = btrim(label) AND label <> ''),
  CONSTRAINT dict_value_segments_status_check CHECK (status IN ('active', 'inactive')),
  CONSTRAINT dict_value_segments_window_check CHECK (disabled_on IS NULL OR enabled_on < disabled_on)
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'dict_value_segments_no_overlap'
      AND connamespace = 'iam'::regnamespace
  ) THEN
    ALTER TABLE iam.dict_value_segments
      ADD CONSTRAINT dict_value_segments_no_overlap
      EXCLUDE USING gist (
        tenant_uuid WITH =,
        dict_code WITH =,
        code WITH =,
        daterange(enabled_on, COALESCE(disabled_on, 'infinity'::date), '[)') WITH &&
      );
  END IF;
END
$$;

CREATE INDEX IF NOT EXISTS dict_value_segments_lookup_idx
  ON iam.dict_value_segments (tenant_uuid, dict_code, code, enabled_on DESC);

CREATE INDEX IF NOT EXISTS dict_value_segments_active_idx
  ON iam.dict_value_segments (tenant_uuid, dict_code, enabled_on, disabled_on);

ALTER TABLE iam.dict_value_segments ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.dict_value_segments FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.dict_value_segments;
CREATE POLICY tenant_isolation ON iam.dict_value_segments
USING (
  tenant_uuid = current_setting('app.current_tenant')::uuid
  OR tenant_uuid = '00000000-0000-0000-0000-000000000000'::uuid
)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE TABLE IF NOT EXISTS iam.dict_value_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  dict_code text NOT NULL,
  code text NOT NULL,
  event_type text NOT NULL,
  effective_day date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  before_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
  after_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_uuid uuid NULL,
  tx_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT dict_value_events_event_uuid_unique UNIQUE (event_uuid),
  CONSTRAINT dict_value_events_request_unique UNIQUE (tenant_uuid, request_id),
  CONSTRAINT dict_value_events_dict_code_check CHECK (
    dict_code = lower(dict_code)
    AND dict_code = btrim(dict_code)
    AND dict_code ~ '^[a-z][a-z0-9_]{0,63}$'
  ),
  CONSTRAINT dict_value_events_code_check CHECK (code = btrim(code) AND code <> ''),
  CONSTRAINT dict_value_events_event_type_check CHECK (
    event_type IN (
      'DICT_VALUE_CREATED',
      'DICT_VALUE_LABEL_CORRECTED',
      'DICT_VALUE_DISABLED',
      'DICT_VALUE_REENABLED',
      'DICT_VALUE_RESCINDED'
    )
  ),
  CONSTRAINT dict_value_events_payload_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT dict_value_events_before_snapshot_object_check CHECK (jsonb_typeof(before_snapshot) = 'object'),
  CONSTRAINT dict_value_events_after_snapshot_object_check CHECK (jsonb_typeof(after_snapshot) = 'object')
);

CREATE INDEX IF NOT EXISTS dict_value_events_lookup_idx
  ON iam.dict_value_events (tenant_uuid, dict_code, code, id DESC);

ALTER TABLE iam.dict_value_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.dict_value_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.dict_value_events;
CREATE POLICY tenant_isolation ON iam.dict_value_events
USING (
  tenant_uuid = current_setting('app.current_tenant')::uuid
  OR tenant_uuid = '00000000-0000-0000-0000-000000000000'::uuid
)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE OR REPLACE FUNCTION iam.submit_dict_value_event(
  p_tenant_uuid uuid,
  p_dict_code text,
  p_code text,
  p_event_type text,
  p_effective_day date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS TABLE(event_id bigint, was_retry boolean)
LANGUAGE plpgsql
AS $$
DECLARE
  v_dict_code text := lower(btrim(COALESCE(p_dict_code, '')));
  v_code text := btrim(COALESCE(p_code, ''));
  v_event_type text := upper(btrim(COALESCE(p_event_type, '')));
  v_request_id text := btrim(COALESCE(p_request_id, ''));
  v_payload jsonb := COALESCE(p_payload, '{}'::jsonb);
  v_now timestamptz := now();
  v_label text := '';
  v_existing iam.dict_value_events%ROWTYPE;
  v_target iam.dict_value_segments%ROWTYPE;
  v_before jsonb := '{}'::jsonb;
  v_after jsonb := '{}'::jsonb;
BEGIN
  PERFORM public.assert_current_tenant(p_tenant_uuid);

  IF v_dict_code = '' THEN
    RAISE EXCEPTION 'DICT_CODE_REQUIRED' USING ERRCODE = 'P0001';
  END IF;
  IF v_code = '' THEN
    RAISE EXCEPTION 'DICT_VALUE_CODE_REQUIRED' USING ERRCODE = 'P0001';
  END IF;
  IF p_effective_day IS NULL THEN
    RAISE EXCEPTION 'DICT_EFFECTIVE_DAY_REQUIRED' USING ERRCODE = 'P0001';
  END IF;
  IF v_request_id = '' THEN
    RAISE EXCEPTION 'DICT_REQUEST_CODE_REQUIRED' USING ERRCODE = 'P0001';
  END IF;
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION 'DICT_PAYLOAD_INVALID' USING ERRCODE = 'P0001';
  END IF;
  IF v_event_type NOT IN (
    'DICT_VALUE_CREATED',
    'DICT_VALUE_LABEL_CORRECTED',
    'DICT_VALUE_DISABLED',
    'DICT_VALUE_REENABLED',
    'DICT_VALUE_RESCINDED'
  ) THEN
    RAISE EXCEPTION 'DICT_EVENT_TYPE_INVALID' USING ERRCODE = 'P0001';
  END IF;

  SELECT *
  INTO v_existing
  FROM iam.dict_value_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = v_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing.event_type = v_event_type
      AND v_existing.dict_code = v_dict_code
      AND v_existing.code = v_code
      AND v_existing.effective_day = p_effective_day
      AND v_existing.payload = v_payload THEN
      RETURN QUERY SELECT v_existing.id, true;
      RETURN;
    END IF;
    RAISE EXCEPTION 'DICT_VALUE_CONFLICT' USING ERRCODE = 'P0001';
  END IF;

  IF v_event_type IN ('DICT_VALUE_CREATED', 'DICT_VALUE_REENABLED', 'DICT_VALUE_LABEL_CORRECTED') THEN
    v_label := btrim(COALESCE(v_payload->>'label', ''));
    IF v_label = '' THEN
      RAISE EXCEPTION 'DICT_VALUE_LABEL_REQUIRED' USING ERRCODE = 'P0001';
    END IF;
  END IF;

  IF v_event_type IN ('DICT_VALUE_DISABLED', 'DICT_VALUE_LABEL_CORRECTED', 'DICT_VALUE_RESCINDED') THEN
    SELECT *
    INTO v_target
    FROM iam.dict_value_segments
    WHERE tenant_uuid = p_tenant_uuid
      AND dict_code = v_dict_code
      AND code = v_code
      AND enabled_on <= p_effective_day
      AND (disabled_on IS NULL OR p_effective_day < disabled_on)
    ORDER BY enabled_on DESC
    LIMIT 1
    FOR UPDATE;

    IF NOT FOUND THEN
      RAISE EXCEPTION 'DICT_VALUE_NOT_FOUND_AS_OF' USING ERRCODE = 'P0001';
    END IF;
  END IF;

  IF v_event_type IN ('DICT_VALUE_CREATED', 'DICT_VALUE_REENABLED') THEN
    INSERT INTO iam.dict_value_segments (
      tenant_uuid,
      dict_code,
      code,
      label,
      enabled_on,
      disabled_on,
      status,
      created_at,
      updated_at
    )
    VALUES (
      p_tenant_uuid,
      v_dict_code,
      v_code,
      v_label,
      p_effective_day,
      NULL,
      'active',
      v_now,
      v_now
    );

    SELECT *
    INTO v_target
    FROM iam.dict_value_segments
    WHERE tenant_uuid = p_tenant_uuid
      AND dict_code = v_dict_code
      AND code = v_code
      AND enabled_on = p_effective_day
    LIMIT 1;

  ELSIF v_event_type = 'DICT_VALUE_DISABLED' THEN
    IF p_effective_day <= v_target.enabled_on THEN
      RAISE EXCEPTION 'DICT_VALUE_CONFLICT' USING ERRCODE = 'P0001';
    END IF;

    v_before := jsonb_build_object(
      'dict_code', v_target.dict_code,
      'code', v_target.code,
      'label', v_target.label,
      'status', v_target.status,
      'enabled_on', v_target.enabled_on,
      'disabled_on', v_target.disabled_on
    );

    UPDATE iam.dict_value_segments
    SET disabled_on = p_effective_day,
        status = 'inactive',
        updated_at = v_now
    WHERE tenant_uuid = v_target.tenant_uuid
      AND dict_code = v_target.dict_code
      AND code = v_target.code
      AND enabled_on = v_target.enabled_on;

    SELECT *
    INTO v_target
    FROM iam.dict_value_segments
    WHERE tenant_uuid = p_tenant_uuid
      AND dict_code = v_dict_code
      AND code = v_code
      AND enabled_on = v_target.enabled_on
    LIMIT 1;

  ELSIF v_event_type = 'DICT_VALUE_LABEL_CORRECTED' THEN
    v_before := jsonb_build_object(
      'dict_code', v_target.dict_code,
      'code', v_target.code,
      'label', v_target.label,
      'status', v_target.status,
      'enabled_on', v_target.enabled_on,
      'disabled_on', v_target.disabled_on
    );

    UPDATE iam.dict_value_segments
    SET label = v_label,
        updated_at = v_now
    WHERE tenant_uuid = v_target.tenant_uuid
      AND dict_code = v_target.dict_code
      AND code = v_target.code
      AND enabled_on = v_target.enabled_on;

    SELECT *
    INTO v_target
    FROM iam.dict_value_segments
    WHERE tenant_uuid = p_tenant_uuid
      AND dict_code = v_dict_code
      AND code = v_code
      AND enabled_on = v_target.enabled_on
    LIMIT 1;
  ELSE
    RAISE EXCEPTION 'DICT_EVENT_TYPE_INVALID' USING ERRCODE = 'P0001';
  END IF;

  IF v_before = '{}'::jsonb THEN
    v_before := jsonb_build_object();
  END IF;
  v_after := jsonb_build_object(
    'dict_code', v_target.dict_code,
    'code', v_target.code,
    'label', v_target.label,
    'status', v_target.status,
    'enabled_on', v_target.enabled_on,
    'disabled_on', v_target.disabled_on
  );

  INSERT INTO iam.dict_value_events (
    tenant_uuid,
    dict_code,
    code,
    event_type,
    effective_day,
    payload,
    before_snapshot,
    after_snapshot,
    request_id,
    initiator_uuid,
    tx_time,
    created_at
  )
  VALUES (
    p_tenant_uuid,
    v_dict_code,
    v_code,
    v_event_type,
    p_effective_day,
    v_payload,
    v_before,
    v_after,
    v_request_id,
    p_initiator_uuid,
    v_now,
    v_now
  )
  RETURNING id INTO event_id;

  RETURN QUERY SELECT event_id, false;
  RETURN;
EXCEPTION
  WHEN unique_violation OR exclusion_violation THEN
    RAISE EXCEPTION 'DICT_VALUE_CONFLICT' USING ERRCODE = 'P0001';
END;
$$;

REVOKE EXECUTE ON FUNCTION iam.submit_dict_value_event(uuid, text, text, text, date, jsonb, text, uuid) FROM PUBLIC;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'GRANT EXECUTE ON FUNCTION iam.submit_dict_value_event(uuid, text, text, text, date, jsonb, text, uuid) TO app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT EXECUTE ON FUNCTION iam.submit_dict_value_event(uuid, text, text, text, date, jsonb, text, uuid) TO superadmin_runtime';
  END IF;
END
$$;

DO $$
DECLARE
  v_global uuid := '00000000-0000-0000-0000-000000000000'::uuid;
  v_local uuid := '00000000-0000-0000-0000-000000000001'::uuid;
BEGIN
  PERFORM set_config('app.current_tenant', v_global::text, true);
  IF EXISTS (SELECT 1 FROM iam.tenants WHERE id = v_global) THEN
    INSERT INTO iam.dict_value_segments (tenant_uuid, dict_code, code, label, enabled_on, disabled_on, status)
    VALUES
      (v_global, 'org_type', '10', '部门', '1970-01-01', NULL, 'active'),
      (v_global, 'org_type', '20', '单位', '1970-01-01', NULL, 'active')
    ON CONFLICT (tenant_uuid, dict_code, code, enabled_on) DO UPDATE
    SET label = EXCLUDED.label,
        disabled_on = NULL,
        status = 'active',
        updated_at = now();
  END IF;

  PERFORM set_config('app.current_tenant', v_local::text, true);
  IF EXISTS (SELECT 1 FROM iam.tenants WHERE id = v_local) THEN
    INSERT INTO iam.dict_value_segments (tenant_uuid, dict_code, code, label, enabled_on, disabled_on, status)
    VALUES
      (v_local, 'org_type', '10', '部门', '1970-01-01', NULL, 'active'),
      (v_local, 'org_type', '20', '单位', '1970-01-01', NULL, 'active')
    ON CONFLICT (tenant_uuid, dict_code, code, enabled_on) DO UPDATE
    SET label = EXCLUDED.label,
        disabled_on = NULL,
        status = 'active',
        updated_at = now();
  END IF;
END
$$;
