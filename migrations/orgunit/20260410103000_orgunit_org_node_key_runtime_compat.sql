-- +goose Up
-- +goose StatementBegin
-- DEV-PLAN-320: 在正式 DB cutover 前，为现有 org_id 内核补 org_node_key 运行时兼容层，
-- 让应用主链可以只使用 org_code/org_node_key，而底层仍委托给现有 int 内核。

CREATE OR REPLACE FUNCTION orgunit.encode_org_node_key(p_seq bigint)
RETURNS char(8)
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  v_guard_alphabet constant text := 'ABCDEFGHJKLMNPQRSTUVWXYZ';
  v_body_alphabet constant text := 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
  v_body_base constant bigint := 32;
  v_body_width constant integer := 7;
  v_capacity constant bigint := 34359738368;
  v_guard_index integer;
  v_body_value bigint;
  v_body text := '';
  v_digit_index integer;
BEGIN
  IF p_seq IS NULL OR p_seq <= 0 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = 'seq must be positive';
  END IF;

  IF p_seq > 824633720831 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NODE_KEY_EXHAUSTED',
      DETAIL = format('seq=%s', p_seq);
  END IF;

  v_guard_index := floor(p_seq::numeric / v_capacity)::integer % length(v_guard_alphabet);
  v_body_value := mod(p_seq, v_capacity);

  FOR i IN REVERSE v_body_width..1 LOOP
    v_digit_index := mod(v_body_value, v_body_base)::integer;
    v_body := substr(v_body_alphabet, v_digit_index + 1, 1) || v_body;
    v_body_value := floor(v_body_value::numeric / v_body_base)::bigint;
  END LOOP;

  RETURN (substr(v_guard_alphabet, v_guard_index + 1, 1) || v_body)::char(8);
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.decode_org_node_key(p_org_node_key char(8))
RETURNS bigint
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  v_key text := btrim(COALESCE(p_org_node_key::text, ''));
  v_guard_alphabet constant text := 'ABCDEFGHJKLMNPQRSTUVWXYZ';
  v_body_alphabet constant text := 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
  v_body_base constant bigint := 32;
  v_capacity constant bigint := 34359738368;
  v_guard_index integer;
  v_digit_index integer;
  v_body_value bigint := 0;
BEGIN
  IF v_key = '' OR v_key !~ '^[ABCDEFGHJKLMNPQRSTUVWXYZ][ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{7}$' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('org_node_key invalid: %s', COALESCE(v_key, '<null>'));
  END IF;

  v_guard_index := strpos(v_guard_alphabet, substr(v_key, 1, 1)) - 1;
  IF v_guard_index < 0 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('org_node_key invalid: %s', v_key);
  END IF;

  FOR i IN 2..length(v_key) LOOP
    v_digit_index := strpos(v_body_alphabet, substr(v_key, i, 1)) - 1;
    IF v_digit_index < 0 THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_INVALID_ARGUMENT',
        DETAIL = format('org_node_key invalid: %s', v_key);
    END IF;
    v_body_value := v_body_value * v_body_base + v_digit_index;
  END LOOP;

  RETURN v_guard_index::bigint * v_capacity + v_body_value;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.translate_org_node_key_payload(p_payload jsonb)
RETURNS jsonb
LANGUAGE plpgsql
AS $$
DECLARE
  v_payload jsonb := COALESCE(p_payload, '{}'::jsonb);
  v_parent_org_node_key text;
  v_new_parent_org_node_key text;
BEGIN
  IF v_payload ? 'parent_org_node_key' THEN
    v_parent_org_node_key := NULLIF(btrim(COALESCE(v_payload->>'parent_org_node_key', '')), '');
    v_payload := v_payload - 'parent_org_node_key';
    IF v_parent_org_node_key IS NULL THEN
      v_payload := jsonb_set(v_payload, '{parent_id}', 'null'::jsonb, true);
    ELSE
      v_payload := jsonb_set(
        v_payload,
        '{parent_id}',
        to_jsonb(orgunit.decode_org_node_key(v_parent_org_node_key::char(8))::int),
        true
      );
    END IF;
  END IF;

  IF v_payload ? 'new_parent_org_node_key' THEN
    v_new_parent_org_node_key := NULLIF(btrim(COALESCE(v_payload->>'new_parent_org_node_key', '')), '');
    v_payload := v_payload - 'new_parent_org_node_key';
    IF v_new_parent_org_node_key IS NULL THEN
      v_payload := jsonb_set(v_payload, '{new_parent_id}', 'null'::jsonb, true);
    ELSE
      v_payload := jsonb_set(
        v_payload,
        '{new_parent_id}',
        to_jsonb(orgunit.decode_org_node_key(v_new_parent_org_node_key::char(8))::int),
        true
      );
    END IF;
  END IF;

  RETURN v_payload;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_org_node_key char(8),
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
  v_org_id int;
  v_payload jsonb;
BEGIN
  v_payload := orgunit.translate_org_node_key_payload(p_payload);

  IF p_org_node_key IS NOT NULL AND btrim(p_org_node_key::text) <> '' THEN
    v_org_id := orgunit.decode_org_node_key(p_org_node_key)::int;
  END IF;

  RETURN orgunit.submit_org_event(
    p_event_uuid,
    p_tenant_uuid,
    v_org_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid
  );
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event_correction(
  p_tenant_uuid uuid,
  p_org_node_key char(8),
  p_target_effective_date date,
  p_patch jsonb,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN orgunit.submit_org_event_correction(
    p_tenant_uuid,
    orgunit.decode_org_node_key(p_org_node_key)::int,
    p_target_effective_date,
    orgunit.translate_org_node_key_payload(p_patch),
    p_request_id,
    p_initiator_uuid
  );
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event_rescind(
  p_tenant_uuid uuid,
  p_org_node_key char(8),
  p_target_effective_date date,
  p_reason text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN orgunit.submit_org_event_rescind(
    p_tenant_uuid,
    orgunit.decode_org_node_key(p_org_node_key)::int,
    p_target_effective_date,
    p_reason,
    p_request_id,
    p_initiator_uuid
  );
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_status_correction(
  p_tenant_uuid uuid,
  p_org_node_key char(8),
  p_target_effective_date date,
  p_target_status text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN orgunit.submit_org_status_correction(
    p_tenant_uuid,
    orgunit.decode_org_node_key(p_org_node_key)::int,
    p_target_effective_date,
    p_target_status,
    p_request_id,
    p_initiator_uuid
  );
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_rescind(
  p_tenant_uuid uuid,
  p_org_node_key char(8),
  p_reason text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS int
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN orgunit.submit_org_rescind(
    p_tenant_uuid,
    orgunit.decode_org_node_key(p_org_node_key)::int,
    p_reason,
    p_request_id,
    p_initiator_uuid
  );
END;
$$;

ALTER FUNCTION orgunit.encode_org_node_key(bigint)
  OWNER TO orgunit_kernel;

ALTER FUNCTION orgunit.decode_org_node_key(char)
  OWNER TO orgunit_kernel;

ALTER FUNCTION orgunit.translate_org_node_key_payload(jsonb)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.translate_org_node_key_payload(jsonb)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.translate_org_node_key_payload(jsonb)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, char, text, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, char, text, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, char, text, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_event_correction(uuid, char, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, char, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, char, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, char, date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, char, date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, char, date, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_status_correction(uuid, char, date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_status_correction(uuid, char, date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_status_correction(uuid, char, date, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_rescind(uuid, char, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_rescind(uuid, char, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_rescind(uuid, char, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS orgunit.submit_org_rescind(uuid, char(8), text, text, uuid);
DROP FUNCTION IF EXISTS orgunit.submit_org_status_correction(uuid, char(8), date, text, text, uuid);
DROP FUNCTION IF EXISTS orgunit.submit_org_event_rescind(uuid, char(8), date, text, text, uuid);
DROP FUNCTION IF EXISTS orgunit.submit_org_event_correction(uuid, char(8), date, jsonb, text, uuid);
DROP FUNCTION IF EXISTS orgunit.submit_org_event(uuid, uuid, char(8), text, date, jsonb, text, uuid);
DROP FUNCTION IF EXISTS orgunit.translate_org_node_key_payload(jsonb);
DROP FUNCTION IF EXISTS orgunit.decode_org_node_key(char(8));
DROP FUNCTION IF EXISTS orgunit.encode_org_node_key(bigint);
-- +goose StatementEnd
