CREATE OR REPLACE FUNCTION orgunit.get_org_snapshot(p_tenant_id uuid, p_query_date date)
RETURNS TABLE (
  org_id uuid,
  parent_id uuid,
  name varchar(255),
  full_name_path text,
  depth int,
  manager_id uuid,
  node_path ltree
)
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);

  IF p_query_date IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = 'query_date is required';
  END IF;

  RETURN QUERY
  WITH snapshot AS (
    SELECT
      v.org_id,
      v.parent_id,
      v.node_path,
      v.name,
      v.manager_id,
      v.path_ids
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_id = p_tenant_id
      AND v.hierarchy_type = 'OrgUnit'
      AND v.status = 'active'
      AND v.validity @> p_query_date
  )
  SELECT
    s.org_id,
    s.parent_id,
    s.name,
    (
      SELECT string_agg(a.name, ' / ' ORDER BY t.idx)
      FROM unnest(s.path_ids) WITH ORDINALITY AS t(uid, idx)
      JOIN orgunit.org_unit_versions a
        ON a.tenant_id = p_tenant_id
       AND a.hierarchy_type = 'OrgUnit'
       AND a.org_id = t.uid
       AND a.validity @> p_query_date
    ) AS full_name_path,
    nlevel(s.node_path) - 1 AS depth,
    s.manager_id,
    s.node_path
  FROM snapshot s
  ORDER BY s.node_path;
END;
$$;
