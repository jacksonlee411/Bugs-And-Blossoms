CREATE TABLE IF NOT EXISTS iam.dicts (
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  dict_code text NOT NULL,
  name text NOT NULL,
  enabled_on date NOT NULL,
  disabled_on date NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, dict_code),
  CONSTRAINT dicts_dict_code_check CHECK (
    dict_code = lower(dict_code)
    AND dict_code = btrim(dict_code)
    AND dict_code ~ '^[a-z][a-z0-9_]{0,63}$'
  ),
  CONSTRAINT dicts_name_check CHECK (name = btrim(name) AND name <> ''),
  CONSTRAINT dicts_window_check CHECK (disabled_on IS NULL OR enabled_on < disabled_on)
);

CREATE INDEX IF NOT EXISTS dicts_lookup_idx
  ON iam.dicts (tenant_uuid, enabled_on, disabled_on, dict_code);

ALTER TABLE iam.dicts ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.dicts FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.dicts;
CREATE POLICY tenant_isolation ON iam.dicts
USING (
  tenant_uuid = current_setting('app.current_tenant')::uuid
)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE TABLE IF NOT EXISTS iam.dict_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  dict_code text NOT NULL,
  event_type text NOT NULL,
  effective_day date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  before_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
  after_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_uuid uuid NULL,
  tx_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT dict_events_event_uuid_unique UNIQUE (event_uuid),
  CONSTRAINT dict_events_request_unique UNIQUE (tenant_uuid, request_id),
  CONSTRAINT dict_events_dict_fk FOREIGN KEY (tenant_uuid, dict_code) REFERENCES iam.dicts (tenant_uuid, dict_code),
  CONSTRAINT dict_events_event_type_check CHECK (
    event_type IN (
      'DICT_CREATED',
      'DICT_DISABLED'
    )
  ),
  CONSTRAINT dict_events_payload_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT dict_events_before_snapshot_object_check CHECK (jsonb_typeof(before_snapshot) = 'object'),
  CONSTRAINT dict_events_after_snapshot_object_check CHECK (jsonb_typeof(after_snapshot) = 'object')
);

CREATE INDEX IF NOT EXISTS dict_events_lookup_idx
  ON iam.dict_events (tenant_uuid, dict_code, id DESC);

ALTER TABLE iam.dict_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.dict_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.dict_events;
CREATE POLICY tenant_isolation ON iam.dict_events
USING (
  tenant_uuid = current_setting('app.current_tenant')::uuid
)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

INSERT INTO iam.dicts (tenant_uuid, dict_code, name, enabled_on, disabled_on)
SELECT
  tenant_uuid,
  dict_code,
  initcap(replace(dict_code, '_', ' ')) AS name,
  min(enabled_on) AS enabled_on,
  NULL::date AS disabled_on
FROM iam.dict_value_segments
GROUP BY tenant_uuid, dict_code
ON CONFLICT (tenant_uuid, dict_code) DO NOTHING;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'dict_value_segments_dict_fk'
      AND connamespace = 'iam'::regnamespace
  ) THEN
    ALTER TABLE iam.dict_value_segments
      ADD CONSTRAINT dict_value_segments_dict_fk
      FOREIGN KEY (tenant_uuid, dict_code)
      REFERENCES iam.dicts (tenant_uuid, dict_code);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'dict_value_events_dict_fk'
      AND connamespace = 'iam'::regnamespace
  ) THEN
    ALTER TABLE iam.dict_value_events
      ADD CONSTRAINT dict_value_events_dict_fk
      FOREIGN KEY (tenant_uuid, dict_code)
      REFERENCES iam.dicts (tenant_uuid, dict_code);
  END IF;
END
$$;

CREATE OR REPLACE FUNCTION iam.submit_dict_event(
  p_tenant_uuid uuid,
  p_dict_code text,
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
  v_event_type text := upper(btrim(COALESCE(p_event_type, '')));
  v_request_id text := btrim(COALESCE(p_request_id, ''));
  v_payload jsonb := COALESCE(p_payload, '{}'::jsonb);
  v_now timestamptz := now();
  v_name text := '';
  v_existing iam.dict_events%ROWTYPE;
  v_dict iam.dicts%ROWTYPE;
  v_before jsonb := '{}'::jsonb;
  v_after jsonb := '{}'::jsonb;
BEGIN
  PERFORM public.assert_current_tenant(p_tenant_uuid);

  IF v_dict_code = '' THEN
    RAISE EXCEPTION 'DICT_CODE_REQUIRED' USING ERRCODE = 'P0001';
  END IF;
  IF p_effective_day IS NULL THEN
    RAISE EXCEPTION 'DICT_ENABLED_ON_REQUIRED' USING ERRCODE = 'P0001';
  END IF;
  IF v_request_id = '' THEN
    RAISE EXCEPTION 'DICT_REQUEST_CODE_REQUIRED' USING ERRCODE = 'P0001';
  END IF;
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION 'DICT_PAYLOAD_INVALID' USING ERRCODE = 'P0001';
  END IF;
  IF v_event_type NOT IN ('DICT_CREATED', 'DICT_DISABLED') THEN
    RAISE EXCEPTION 'DICT_EVENT_TYPE_INVALID' USING ERRCODE = 'P0001';
  END IF;

  SELECT *
  INTO v_existing
  FROM iam.dict_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = v_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing.event_type = v_event_type
      AND v_existing.dict_code = v_dict_code
      AND v_existing.effective_day = p_effective_day
      AND v_existing.payload = v_payload THEN
      RETURN QUERY SELECT v_existing.id, true;
      RETURN;
    END IF;
    RAISE EXCEPTION 'DICT_CODE_CONFLICT' USING ERRCODE = 'P0001';
  END IF;

  IF v_event_type = 'DICT_CREATED' THEN
    v_name := btrim(COALESCE(v_payload->>'name', ''));
    IF v_name = '' THEN
      RAISE EXCEPTION 'DICT_NAME_REQUIRED' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO iam.dicts (
      tenant_uuid,
      dict_code,
      name,
      enabled_on,
      disabled_on,
      created_at,
      updated_at
    )
    VALUES (
      p_tenant_uuid,
      v_dict_code,
      v_name,
      p_effective_day,
      NULL,
      v_now,
      v_now
    );

    SELECT *
    INTO v_dict
    FROM iam.dicts
    WHERE tenant_uuid = p_tenant_uuid
      AND dict_code = v_dict_code
    LIMIT 1;
  ELSE
    SELECT *
    INTO v_dict
    FROM iam.dicts
    WHERE tenant_uuid = p_tenant_uuid
      AND dict_code = v_dict_code
    LIMIT 1
    FOR UPDATE;

    IF NOT FOUND THEN
      RAISE EXCEPTION 'DICT_NOT_FOUND' USING ERRCODE = 'P0001';
    END IF;
    IF p_effective_day <= v_dict.enabled_on THEN
      RAISE EXCEPTION 'DICT_CODE_CONFLICT' USING ERRCODE = 'P0001';
    END IF;
    IF v_dict.disabled_on IS NOT NULL AND p_effective_day >= v_dict.disabled_on THEN
      RAISE EXCEPTION 'DICT_CODE_CONFLICT' USING ERRCODE = 'P0001';
    END IF;

    v_before := jsonb_build_object(
      'dict_code', v_dict.dict_code,
      'name', v_dict.name,
      'status', 'active',
      'enabled_on', v_dict.enabled_on,
      'disabled_on', v_dict.disabled_on
    );

    UPDATE iam.dicts
    SET disabled_on = p_effective_day,
        updated_at = v_now
    WHERE tenant_uuid = v_dict.tenant_uuid
      AND dict_code = v_dict.dict_code;

    SELECT *
    INTO v_dict
    FROM iam.dicts
    WHERE tenant_uuid = p_tenant_uuid
      AND dict_code = v_dict_code
    LIMIT 1;
  END IF;

  IF v_before = '{}'::jsonb THEN
    v_before := jsonb_build_object();
  END IF;

  v_after := jsonb_build_object(
    'dict_code', v_dict.dict_code,
    'name', v_dict.name,
    'status', CASE
      WHEN p_effective_day >= v_dict.enabled_on
        AND (v_dict.disabled_on IS NULL OR p_effective_day < v_dict.disabled_on)
      THEN 'active'
      ELSE 'inactive'
    END,
    'enabled_on', v_dict.enabled_on,
    'disabled_on', v_dict.disabled_on
  );

  INSERT INTO iam.dict_events (
    tenant_uuid,
    dict_code,
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
  WHEN unique_violation THEN
    RAISE EXCEPTION 'DICT_CODE_CONFLICT' USING ERRCODE = 'P0001';
END;
$$;

REVOKE EXECUTE ON FUNCTION iam.submit_dict_event(uuid, text, text, date, jsonb, text, uuid) FROM PUBLIC;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'GRANT EXECUTE ON FUNCTION iam.submit_dict_event(uuid, text, text, date, jsonb, text, uuid) TO app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT EXECUTE ON FUNCTION iam.submit_dict_event(uuid, text, text, date, jsonb, text, uuid) TO superadmin_runtime';
  END IF;
END
$$;
