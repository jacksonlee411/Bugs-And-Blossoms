-- DEV-PLAN-320 P3: org_node_key runtime read overlay
DROP FUNCTION IF EXISTS orgunit.get_org_snapshot(uuid, date);

CREATE OR REPLACE FUNCTION orgunit.get_org_snapshot(p_tenant_uuid uuid, p_query_date date)
RETURNS TABLE (
  org_node_key char(8),
  parent_org_node_key char(8),
  name varchar(255),
  is_business_unit boolean,
  full_name_path text,
  depth int,
  manager_uuid uuid,
  node_path ltree,
  path_node_keys text[]
)
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_query_date IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = 'query_date is required';
  END IF;

  RETURN QUERY
  SELECT
    v.org_node_key,
    v.parent_org_node_key,
    v.name,
    v.is_business_unit,
    v.full_name_path,
    nlevel(v.node_path) - 1 AS depth,
    v.manager_uuid,
    v.node_path,
    v.path_node_keys
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.status = 'active'
    AND v.validity @> p_query_date
  ORDER BY v.node_path;
END;
$$;
