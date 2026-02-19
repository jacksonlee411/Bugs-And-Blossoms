-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS orgunit.tenant_field_policies (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  field_key text NOT NULL,
  scope_type text NOT NULL,
  scope_key text NOT NULL,
  maintainable boolean NOT NULL DEFAULT true,
  default_mode text NOT NULL DEFAULT 'NONE',
  default_rule_expr text NULL,
  enabled_on date NOT NULL,
  disabled_on date NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz NULL,
  CONSTRAINT tenant_field_policies_field_key_format_check CHECK (field_key ~ '^[a-z][a-z0-9_]{0,62}$'),
  CONSTRAINT tenant_field_policies_scope_type_check CHECK (scope_type IN ('GLOBAL','FORM')),
  CONSTRAINT tenant_field_policies_scope_key_check CHECK (
    (scope_type = 'GLOBAL' AND scope_key = 'global')
    OR
    (scope_type = 'FORM' AND scope_key IN (
      'orgunit.create_dialog',
      'orgunit.details.add_version_dialog',
      'orgunit.details.insert_version_dialog',
      'orgunit.details.correct_dialog'
    ))
  ),
  CONSTRAINT tenant_field_policies_default_mode_check CHECK (default_mode IN ('NONE','CEL')),
  CONSTRAINT tenant_field_policies_default_rule_expr_required_check CHECK (
    (default_mode = 'NONE' AND default_rule_expr IS NULL)
    OR
    (default_mode = 'CEL' AND default_rule_expr IS NOT NULL AND btrim(default_rule_expr) <> '')
  ),
  CONSTRAINT tenant_field_policies_disabled_on_check CHECK (disabled_on IS NULL OR disabled_on > enabled_on)
);

CREATE INDEX IF NOT EXISTS tenant_field_policies_tenant_field_scope_idx
  ON orgunit.tenant_field_policies (tenant_uuid, field_key, scope_type, scope_key, enabled_on DESC);

CREATE TABLE IF NOT EXISTS orgunit.tenant_field_policy_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL,
  event_type text NOT NULL,
  field_key text NOT NULL,
  scope_type text NOT NULL,
  scope_key text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_code text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT tenant_field_policy_events_event_type_check CHECK (event_type IN ('UPSERT','DISABLE')),
  CONSTRAINT tenant_field_policy_events_field_key_format_check CHECK (field_key ~ '^[a-z][a-z0-9_]{0,62}$'),
  CONSTRAINT tenant_field_policy_events_scope_type_check CHECK (scope_type IN ('GLOBAL','FORM')),
  CONSTRAINT tenant_field_policy_events_scope_key_non_empty_check CHECK (btrim(scope_key) <> ''),
  CONSTRAINT tenant_field_policy_events_request_code_unique UNIQUE (tenant_uuid, request_code),
  CONSTRAINT tenant_field_policy_events_event_uuid_unique UNIQUE (event_uuid),
  CONSTRAINT tenant_field_policy_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS tenant_field_policy_events_tenant_time_idx
  ON orgunit.tenant_field_policy_events (tenant_uuid, transaction_time DESC, id DESC);

ALTER TABLE orgunit.tenant_field_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.tenant_field_policies FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.tenant_field_policies;
CREATE POLICY tenant_isolation ON orgunit.tenant_field_policies
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.tenant_field_policy_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.tenant_field_policy_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.tenant_field_policy_events;
CREATE POLICY tenant_isolation ON orgunit.tenant_field_policy_events
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE OR REPLACE FUNCTION orgunit.guard_tenant_field_policies_write()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF current_user <> 'orgunit_kernel' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORGUNIT_FIELD_POLICIES_WRITE_FORBIDDEN',
      DETAIL = format('role=%s table=%s', current_user, TG_TABLE_NAME);
  END IF;
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS guard_tenant_field_policies_write ON orgunit.tenant_field_policies;
CREATE TRIGGER guard_tenant_field_policies_write
BEFORE INSERT OR UPDATE OR DELETE ON orgunit.tenant_field_policies
FOR EACH ROW EXECUTE FUNCTION orgunit.guard_tenant_field_policies_write();

DROP TRIGGER IF EXISTS guard_tenant_field_policy_events_write ON orgunit.tenant_field_policy_events;
CREATE TRIGGER guard_tenant_field_policy_events_write
BEFORE INSERT OR UPDATE OR DELETE ON orgunit.tenant_field_policy_events
FOR EACH ROW EXECUTE FUNCTION orgunit.guard_tenant_field_policies_write();

CREATE OR REPLACE FUNCTION orgunit.assert_tenant_field_policies_non_overlapping()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  v_overlap_id bigint;
BEGIN
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;

  SELECT p.id
  INTO v_overlap_id
  FROM orgunit.tenant_field_policies p
  WHERE p.tenant_uuid = NEW.tenant_uuid
    AND p.field_key = NEW.field_key
    AND p.scope_type = NEW.scope_type
    AND p.scope_key = NEW.scope_key
    AND p.id <> NEW.id
    AND daterange(
      NEW.enabled_on,
      COALESCE(NEW.disabled_on, 'infinity'::date),
      '[)'
    ) && daterange(
      p.enabled_on,
      COALESCE(p.disabled_on, 'infinity'::date),
      '[)'
    )
  LIMIT 1;

  IF v_overlap_id IS NOT NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'FIELD_POLICY_SCOPE_OVERLAP',
      DETAIL = format(
        'field_key=%s scope_type=%s scope_key=%s overlap_id=%s',
        NEW.field_key, NEW.scope_type, NEW.scope_key, v_overlap_id
      );
  END IF;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS tenant_field_policies_non_overlapping ON orgunit.tenant_field_policies;
CREATE TRIGGER tenant_field_policies_non_overlapping
BEFORE INSERT OR UPDATE ON orgunit.tenant_field_policies
FOR EACH ROW EXECUTE FUNCTION orgunit.assert_tenant_field_policies_non_overlapping();

CREATE OR REPLACE FUNCTION orgunit.upsert_tenant_field_policy(
  p_tenant_uuid uuid,
  p_field_key text,
  p_scope_type text,
  p_scope_key text,
  p_maintainable boolean,
  p_default_mode text,
  p_default_rule_expr text,
  p_enabled_on date,
  p_request_code text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_scope_type text := upper(btrim(p_scope_type));
  v_scope_key text := btrim(p_scope_key);
  v_default_mode text := upper(btrim(p_default_mode));
  v_default_rule_expr text := NULLIF(btrim(p_default_rule_expr), '');
  v_event_type text;
  v_policy_id bigint;
  v_open_id bigint;
  v_open_enabled_on date;
  v_next_enabled_on date;
BEGIN
  IF v_scope_type NOT IN ('GLOBAL','FORM') THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'scope_type invalid';
  END IF;
  IF v_scope_type = 'GLOBAL' THEN
    v_scope_key := 'global';
  END IF;
  IF v_scope_key = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'scope_key required';
  END IF;
  IF v_default_mode NOT IN ('NONE','CEL') THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'default_mode invalid';
  END IF;
  IF v_default_mode = 'NONE' THEN
    v_default_rule_expr := NULL;
  END IF;
  IF v_default_mode = 'CEL' AND v_default_rule_expr IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'FIELD_POLICY_EXPR_INVALID', DETAIL = 'default_rule_expr required';
  END IF;

  PERFORM pg_advisory_xact_lock(hashtextextended(
    format('orgunit.field_policy:%s:%s:%s:%s', p_tenant_uuid, p_field_key, v_scope_type, v_scope_key),
    0
  ));

  SELECT event_type, (payload->>'policy_id')::bigint
  INTO v_event_type, v_policy_id
  FROM orgunit.tenant_field_policy_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_code = p_request_code
  LIMIT 1;

  IF FOUND THEN
    IF v_event_type <> 'UPSERT' THEN
      RAISE EXCEPTION USING MESSAGE = 'ORG_REQUEST_ID_CONFLICT', DETAIL = format('request_code=%s', p_request_code);
    END IF;
    RETURN v_policy_id;
  END IF;

  SELECT id, enabled_on
  INTO v_open_id, v_open_enabled_on
  FROM orgunit.tenant_field_policies
  WHERE tenant_uuid = p_tenant_uuid
    AND field_key = p_field_key
    AND scope_type = v_scope_type
    AND scope_key = v_scope_key
    AND disabled_on IS NULL
  ORDER BY enabled_on DESC
  LIMIT 1
  FOR UPDATE;

  IF v_open_id IS NOT NULL AND v_open_enabled_on < p_enabled_on THEN
    UPDATE orgunit.tenant_field_policies
    SET disabled_on = p_enabled_on, disabled_at = now(), updated_at = now()
    WHERE id = v_open_id;
  END IF;

  UPDATE orgunit.tenant_field_policies
  SET maintainable = p_maintainable,
      default_mode = v_default_mode,
      default_rule_expr = v_default_rule_expr,
      updated_at = now()
  WHERE tenant_uuid = p_tenant_uuid
    AND field_key = p_field_key
    AND scope_type = v_scope_type
    AND scope_key = v_scope_key
    AND enabled_on = p_enabled_on
  RETURNING id INTO v_policy_id;

  IF v_policy_id IS NULL THEN
    SELECT MIN(enabled_on)
    INTO v_next_enabled_on
    FROM orgunit.tenant_field_policies
    WHERE tenant_uuid = p_tenant_uuid
      AND field_key = p_field_key
      AND scope_type = v_scope_type
      AND scope_key = v_scope_key
      AND enabled_on > p_enabled_on;

    INSERT INTO orgunit.tenant_field_policies (
      tenant_uuid,
      field_key,
      scope_type,
      scope_key,
      maintainable,
      default_mode,
      default_rule_expr,
      enabled_on,
      disabled_on
    ) VALUES (
      p_tenant_uuid,
      p_field_key,
      v_scope_type,
      v_scope_key,
      p_maintainable,
      v_default_mode,
      v_default_rule_expr,
      p_enabled_on,
      v_next_enabled_on
    )
    RETURNING id INTO v_policy_id;
  END IF;

  INSERT INTO orgunit.tenant_field_policy_events (
    event_uuid,
    tenant_uuid,
    event_type,
    field_key,
    scope_type,
    scope_key,
    payload,
    request_code,
    initiator_uuid
  ) VALUES (
    gen_random_uuid(),
    p_tenant_uuid,
    'UPSERT',
    p_field_key,
    v_scope_type,
    v_scope_key,
    jsonb_build_object('policy_id', v_policy_id),
    p_request_code,
    p_initiator_uuid
  );

  RETURN v_policy_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.disable_tenant_field_policy(
  p_tenant_uuid uuid,
  p_field_key text,
  p_scope_type text,
  p_scope_key text,
  p_disabled_on date,
  p_request_code text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_scope_type text := upper(btrim(p_scope_type));
  v_scope_key text := btrim(p_scope_key);
  v_event_type text;
  v_policy_id bigint;
  v_enabled_on date;
BEGIN
  IF v_scope_type NOT IN ('GLOBAL','FORM') THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'scope_type invalid';
  END IF;
  IF v_scope_type = 'GLOBAL' THEN
    v_scope_key := 'global';
  END IF;
  IF v_scope_key = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'scope_key required';
  END IF;

  PERFORM pg_advisory_xact_lock(hashtextextended(
    format('orgunit.field_policy:%s:%s:%s:%s', p_tenant_uuid, p_field_key, v_scope_type, v_scope_key),
    0
  ));

  SELECT event_type, (payload->>'policy_id')::bigint
  INTO v_event_type, v_policy_id
  FROM orgunit.tenant_field_policy_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_code = p_request_code
  LIMIT 1;

  IF FOUND THEN
    IF v_event_type <> 'DISABLE' THEN
      RAISE EXCEPTION USING MESSAGE = 'ORG_REQUEST_ID_CONFLICT', DETAIL = format('request_code=%s', p_request_code);
    END IF;
    RETURN v_policy_id;
  END IF;

  SELECT id, enabled_on
  INTO v_policy_id, v_enabled_on
  FROM orgunit.tenant_field_policies
  WHERE tenant_uuid = p_tenant_uuid
    AND field_key = p_field_key
    AND scope_type = v_scope_type
    AND scope_key = v_scope_key
    AND enabled_on < p_disabled_on
    AND p_disabled_on < COALESCE(disabled_on, 'infinity'::date)
  ORDER BY enabled_on DESC
  LIMIT 1
  FOR UPDATE;

  IF v_policy_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_FIELD_POLICY_NOT_FOUND', DETAIL = format('field_key=%s', p_field_key);
  END IF;
  IF p_disabled_on <= v_enabled_on THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID', DETAIL = format('field_key=%s', p_field_key);
  END IF;

  UPDATE orgunit.tenant_field_policies
  SET disabled_on = p_disabled_on, disabled_at = now(), updated_at = now()
  WHERE id = v_policy_id;

  INSERT INTO orgunit.tenant_field_policy_events (
    event_uuid,
    tenant_uuid,
    event_type,
    field_key,
    scope_type,
    scope_key,
    payload,
    request_code,
    initiator_uuid
  ) VALUES (
    gen_random_uuid(),
    p_tenant_uuid,
    'DISABLE',
    p_field_key,
    v_scope_type,
    v_scope_key,
    jsonb_build_object('policy_id', v_policy_id, 'disabled_on', p_disabled_on),
    p_request_code,
    p_initiator_uuid
  );

  RETURN v_policy_id;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS orgunit.disable_tenant_field_policy(uuid, text, text, text, date, text, uuid);
DROP FUNCTION IF EXISTS orgunit.upsert_tenant_field_policy(uuid, text, text, text, boolean, text, text, date, text, uuid);
DROP TRIGGER IF EXISTS tenant_field_policies_non_overlapping ON orgunit.tenant_field_policies;
DROP FUNCTION IF EXISTS orgunit.assert_tenant_field_policies_non_overlapping();
DROP TRIGGER IF EXISTS guard_tenant_field_policy_events_write ON orgunit.tenant_field_policy_events;
DROP TRIGGER IF EXISTS guard_tenant_field_policies_write ON orgunit.tenant_field_policies;
DROP FUNCTION IF EXISTS orgunit.guard_tenant_field_policies_write();
DROP POLICY IF EXISTS tenant_isolation ON orgunit.tenant_field_policy_events;
ALTER TABLE IF EXISTS orgunit.tenant_field_policy_events DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.tenant_field_policies;
ALTER TABLE IF EXISTS orgunit.tenant_field_policies DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS orgunit.tenant_field_policy_events;
DROP TABLE IF EXISTS orgunit.tenant_field_policies;
-- +goose StatementEnd
