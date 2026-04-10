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
  IF NOT orgunit.is_valid_org_node_key(v_key) THEN
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

CREATE OR REPLACE FUNCTION orgunit.allocate_org_node_key(p_tenant_uuid uuid)
RETURNS char(8)
LANGUAGE plpgsql
AS $$
DECLARE
  v_seq bigint;
  v_org_node_key char(8);
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_tenant_uuid IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = 'tenant_uuid is required';
  END IF;

  SELECT nextval('orgunit.org_node_key_seq') INTO v_seq;
  IF v_seq IS NULL OR v_seq <= 0 OR v_seq > 824633720831 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NODE_KEY_EXHAUSTED',
      DETAIL = format('tenant_uuid=%s seq=%s', p_tenant_uuid, COALESCE(v_seq::text, '<null>'));
  END IF;

  v_org_node_key := orgunit.encode_org_node_key(v_seq);

  INSERT INTO orgunit.org_node_key_registry (
    org_node_key,
    seq,
    tenant_uuid
  )
  VALUES (
    v_org_node_key,
    v_seq,
    p_tenant_uuid
  );

  RETURN v_org_node_key;
END;
$$;
