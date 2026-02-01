-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.get_org_snapshot(p_tenant_uuid uuid, p_query_date date)
RETURNS TABLE (
  org_id int,
  parent_id int,
  name varchar(255),
  is_business_unit boolean,
  full_name_path text,
  depth int,
  manager_uuid uuid,
  node_path ltree
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
    v.org_id,
    v.parent_id,
    v.name,
    v.is_business_unit,
    v.full_name_path,
    nlevel(v.node_path) - 1 AS depth,
    v.manager_uuid,
    v.node_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.hierarchy_type = 'OrgUnit'
    AND v.status = 'active'
    AND v.validity @> p_query_date
  ORDER BY v.node_path;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS orgunit.get_org_snapshot(uuid, date);
-- +goose StatementEnd
