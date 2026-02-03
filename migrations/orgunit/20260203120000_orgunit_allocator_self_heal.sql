-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.allocate_org_id(p_tenant_uuid uuid)
RETURNS int
LANGUAGE plpgsql
AS $$
DECLARE
  v_next int;
  v_max int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  SELECT COALESCE(MAX(org_id), 9999999)
  INTO v_max
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid;

  IF v_max >= 99999999 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ID_EXHAUSTED',
      DETAIL = format('tenant_uuid=%s', p_tenant_uuid);
  END IF;

  INSERT INTO orgunit.org_id_allocators (tenant_uuid, next_org_id)
  VALUES (p_tenant_uuid, v_max + 2)
  ON CONFLICT (tenant_uuid) DO UPDATE
  SET next_org_id = GREATEST(orgunit.org_id_allocators.next_org_id, v_max + 1) + 1,
      updated_at = now()
  WHERE orgunit.org_id_allocators.next_org_id <= 99999999
  RETURNING next_org_id - 1 INTO v_next;

  IF v_next IS NULL OR v_next > 99999999 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ID_EXHAUSTED',
      DETAIL = format('tenant_uuid=%s', p_tenant_uuid);
  END IF;

  RETURN v_next;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.allocate_org_id(p_tenant_uuid uuid)
RETURNS int
LANGUAGE plpgsql
AS $$
DECLARE
  v_next int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  INSERT INTO orgunit.org_id_allocators (tenant_uuid, next_org_id)
  VALUES (p_tenant_uuid, 10000001)
  ON CONFLICT (tenant_uuid) DO UPDATE
  SET next_org_id = orgunit.org_id_allocators.next_org_id + 1,
      updated_at = now()
  WHERE orgunit.org_id_allocators.next_org_id <= 99999999
  RETURNING next_org_id - 1 INTO v_next;

  IF v_next IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ID_EXHAUSTED',
      DETAIL = format('tenant_uuid=%s', p_tenant_uuid);
  END IF;

  RETURN v_next;
END;
$$;
-- +goose StatementEnd
