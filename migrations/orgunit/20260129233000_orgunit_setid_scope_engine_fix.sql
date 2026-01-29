-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.resolve_scope_package(
  p_tenant_id uuid,
  p_setid text,
  p_scope_code text,
  p_as_of_date date
)
RETURNS TABLE(package_id uuid, package_owner_tenant_id uuid)
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
  v_setid text;
  v_scope_mode text;
  v_ctx_tenant text;
  v_allow_share text;
  v_package_id uuid;
  v_package_owner_tenant_id uuid;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);

  IF p_setid IS NULL OR btrim(p_setid) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'setid is required';
  END IF;
  IF p_scope_code IS NULL OR NOT orgunit.scope_code_is_valid(p_scope_code) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_CODE_INVALID';
  END IF;
  IF p_as_of_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = 'as_of_date is required';
  END IF;

  v_setid := orgunit.normalize_setid(p_setid);
  IF v_setid = 'SHARE' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_RESERVED_WORD',
      DETAIL = 'SHARE is reserved';
  END IF;

  SELECT s.package_id, s.package_owner_tenant_id
  INTO v_package_id, v_package_owner_tenant_id
  FROM orgunit.setid_scope_subscriptions s
  WHERE s.tenant_id = p_tenant_id
    AND s.setid = v_setid
    AND s.scope_code = p_scope_code
    AND s.validity @> p_as_of_date
  ORDER BY lower(s.validity) DESC
  LIMIT 1;

  IF v_package_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_SUBSCRIPTION_MISSING',
      DETAIL = format('setid=%s scope_code=%s as_of=%s', v_setid, p_scope_code, p_as_of_date);
  END IF;

  v_scope_mode := orgunit.scope_code_share_mode(p_scope_code);
  v_ctx_tenant := current_setting('app.current_tenant');
  v_allow_share := current_setting('app.allow_share_read', true);

  IF v_package_owner_tenant_id = p_tenant_id THEN
    IF v_scope_mode = 'shared-only' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_SCOPE_MISMATCH';
    END IF;
    IF NOT EXISTS (
      SELECT 1
      FROM orgunit.setid_scope_package_versions v
      WHERE v.tenant_id = p_tenant_id
        AND v.scope_code = p_scope_code
        AND v.package_id = v_package_id
        AND v.validity @> p_as_of_date
        AND v.status = 'active'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_INACTIVE_AS_OF';
    END IF;
  ELSIF v_package_owner_tenant_id = orgunit.global_tenant_id() THEN
    IF v_scope_mode = 'tenant-only' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_SCOPE_MISMATCH';
    END IF;
    PERFORM set_config('app.current_tenant', orgunit.global_tenant_id()::text, true);
    PERFORM set_config('app.allow_share_read', 'on', true);
    IF NOT EXISTS (
      SELECT 1
      FROM orgunit.global_setid_scope_package_versions v
      WHERE v.tenant_id = orgunit.global_tenant_id()
        AND v.scope_code = p_scope_code
        AND v.package_id = v_package_id
        AND v.validity @> p_as_of_date
        AND v.status = 'active'
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_INACTIVE_AS_OF';
    END IF;
  ELSE
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'PACKAGE_OWNER_INVALID';
  END IF;

  PERFORM set_config('app.current_tenant', v_ctx_tenant, true);
  PERFORM set_config('app.allow_share_read', COALESCE(v_allow_share, 'off'), true);

  package_id := v_package_id;
  package_owner_tenant_id := v_package_owner_tenant_id;
  RETURN NEXT;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- no-op
-- +goose StatementEnd
