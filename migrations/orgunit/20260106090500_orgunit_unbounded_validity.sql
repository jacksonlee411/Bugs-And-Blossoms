-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.apply_create_logic(
  p_tenant_uuid uuid,
  p_hierarchy_type text,
  p_org_id int,
  p_parent_id int,
  p_effective_date date,
  p_name text,
  p_manager_uuid uuid,
  p_is_business_unit boolean,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_parent_path ltree;
  v_node_path ltree;
  v_root_org_id int;
  v_is_business_unit boolean;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_hierarchy_type <> 'OrgUnit' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported hierarchy_type: %s', p_hierarchy_type);
  END IF;
  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_name IS NULL OR btrim(p_name) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'name is required';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions
    WHERE tenant_uuid = p_tenant_uuid AND hierarchy_type = p_hierarchy_type AND org_id = p_org_id
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_ALREADY_EXISTS', DETAIL = format('org_id=%s', p_org_id);
  END IF;

  IF p_parent_id IS NULL THEN
    SELECT t.root_org_id INTO v_root_org_id
    FROM orgunit.org_trees t
    WHERE t.tenant_uuid = p_tenant_uuid AND t.hierarchy_type = p_hierarchy_type
    FOR UPDATE;

    IF v_root_org_id IS NOT NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_ROOT_ALREADY_EXISTS',
        DETAIL = format('root_org_id=%s', v_root_org_id);
    END IF;

    INSERT INTO orgunit.org_trees (tenant_uuid, hierarchy_type, root_org_id)
    VALUES (p_tenant_uuid, p_hierarchy_type, p_org_id);

    v_node_path := text2ltree(orgunit.org_ltree_label(p_org_id));
    IF p_is_business_unit IS NOT NULL AND p_is_business_unit = false THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_ROOT_BUSINESS_UNIT_REQUIRED',
        DETAIL = format('org_id=%s', p_org_id);
    END IF;
    v_is_business_unit := true;
  ELSE
    SELECT t.root_org_id INTO v_root_org_id
    FROM orgunit.org_trees t
    WHERE t.tenant_uuid = p_tenant_uuid AND t.hierarchy_type = p_hierarchy_type;

    IF v_root_org_id IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_TREE_NOT_INITIALIZED',
        DETAIL = format('tenant_uuid=%s', p_tenant_uuid);
    END IF;

    SELECT v.node_path INTO v_parent_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.hierarchy_type = p_hierarchy_type
      AND v.org_id = p_parent_id
      AND v.status = 'active'
      AND v.validity @> p_effective_date
    LIMIT 1;

    IF v_parent_path IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_PARENT_NOT_FOUND_AS_OF',
        DETAIL = format('parent_id=%s as_of=%s', p_parent_id, p_effective_date);
    END IF;

    v_node_path := v_parent_path || text2ltree(orgunit.org_ltree_label(p_org_id));
    v_is_business_unit := COALESCE(p_is_business_unit, false);
  END IF;

  INSERT INTO orgunit.org_unit_versions (
    tenant_uuid,
    hierarchy_type,
    org_id,
    parent_id,
    node_path,
    validity,
    name,
    full_name_path,
    status,
    is_business_unit,
    manager_uuid,
    last_event_id
  )
  VALUES (
    p_tenant_uuid,
	    p_hierarchy_type,
	    p_org_id,
	    p_parent_id,
    v_node_path,
    daterange(p_effective_date, NULL, '[)'),
    p_name,
    p_name,
    'active',
    v_is_business_unit,
    p_manager_uuid,
    p_event_db_id
  );
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS orgunit.apply_create_logic(uuid, text, int, int, date, text, uuid, boolean, bigint);
-- +goose StatementEnd
