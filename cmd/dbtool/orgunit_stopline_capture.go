package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type orgunitStoplineTenantSample struct {
	TenantUUID string `json:"tenant_uuid"`
	Hostname   string `json:"hostname,omitempty"`
	Name       string `json:"name,omitempty"`
}

type orgunitStoplineNodeSample struct {
	OrgID           int    `json:"org_id,omitempty"`
	OrgNodeKey      string `json:"org_node_key,omitempty"`
	ParentOrgID     int    `json:"parent_org_id,omitempty"`
	ParentOrgCode   string `json:"parent_org_code,omitempty"`
	ParentNodeKey   string `json:"parent_org_node_key,omitempty"`
	OrgCode         string `json:"org_code"`
	Name            string `json:"name"`
	NodePath        string `json:"node_path,omitempty"`
	SearchQuery     string `json:"search_query,omitempty"`
	SubtreeSize     int    `json:"subtree_size,omitempty"`
	PositionCount   int    `json:"position_count,omitempty"`
	SetID           string `json:"setid,omitempty"`
	PositionUUID    string `json:"position_uuid,omitempty"`
	EffectiveDate   string `json:"effective_date,omitempty"`
	OrgUnitID       int    `json:"org_unit_id,omitempty"`
	JobCatalogSetID string `json:"jobcatalog_setid,omitempty"`
}

type orgunitStoplineSamples struct {
	AsOfDate string `json:"as_of_date"`
	Heavy    struct {
		Tenant            orgunitStoplineTenantSample `json:"tenant"`
		Root              orgunitStoplineNodeSample   `json:"root"`
		ChildrenParent    orgunitStoplineNodeSample   `json:"children_parent"`
		DetailsTarget     orgunitStoplineNodeSample   `json:"details_target"`
		SubtreeFilter     orgunitStoplineNodeSample   `json:"subtree_filter"`
		MoveTarget        orgunitStoplineNodeSample   `json:"move_target"`
		MoveNewParent     orgunitStoplineNodeSample   `json:"move_new_parent"`
		SearchQuery       string                      `json:"search_query"`
		MoveEffectiveDate string                      `json:"move_effective_date"`
	} `json:"heavy"`
	Chain struct {
		Tenant         orgunitStoplineTenantSample `json:"tenant"`
		BusinessUnit   orgunitStoplineNodeSample   `json:"business_unit"`
		PositionSample orgunitStoplineNodeSample   `json:"position_sample"`
	} `json:"chain"`
}

type orgunitStoplineShadowBindingRow struct {
	OrgCode   string
	SetID     string
	ValidFrom string
	ValidTo   string
}

type orgunitStoplineShadowPositionRow struct {
	PositionUUID    string
	OrgCode         string
	JobCatalogSetID string
	Name            string
	ValidFrom       string
	ValidTo         string
}

type orgunitStoplineCaptureSpec struct {
	Key         string   `json:"key"`
	Stage       string   `json:"stage"`
	Description string   `json:"description"`
	SQL         string   `json:"sql"`
	Args        []any    `json:"args"`
	TenantUUID  string   `json:"tenant_uuid,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

type orgunitStoplineExplainMetrics struct {
	PlanningTimeMS      float64 `json:"planning_time_ms"`
	ExecutionTimeMS     float64 `json:"execution_time_ms"`
	PlanRows            float64 `json:"plan_rows"`
	ActualRows          float64 `json:"actual_rows"`
	SharedHitBlocks     int64   `json:"shared_hit_blocks"`
	SharedReadBlocks    int64   `json:"shared_read_blocks"`
	SharedDirtiedBlocks int64   `json:"shared_dirtied_blocks"`
	SharedWrittenBlocks int64   `json:"shared_written_blocks"`
}

type orgunitStoplineExplainRecord struct {
	Key         string                        `json:"key"`
	Stage       string                        `json:"stage"`
	Description string                        `json:"description"`
	SQL         string                        `json:"sql"`
	Args        []any                         `json:"args"`
	TenantUUID  string                        `json:"tenant_uuid,omitempty"`
	Notes       []string                      `json:"notes,omitempty"`
	Metrics     orgunitStoplineExplainMetrics `json:"metrics"`
	RawJSONFile string                        `json:"raw_json_file"`
}

type orgunitStoplineReport struct {
	CapturedAt time.Time                      `json:"captured_at"`
	AsOfDate   string                         `json:"as_of_date"`
	Samples    orgunitStoplineSamples         `json:"samples"`
	Results    []orgunitStoplineExplainRecord `json:"results"`
}

type orgunitStoplineExplainEnvelope []map[string]any

const (
	orgunitStoplineHeavyTenantSQL = `
WITH ranked AS (
  SELECT tenant_uuid, count(*) AS org_count
  FROM orgunit.org_unit_versions
  WHERE validity @> $1::date
  GROUP BY tenant_uuid
  HAVING count(*) >= 10
)
SELECT
  r.tenant_uuid::text,
  COALESCE(td.hostname, '') AS hostname,
  COALESCE(t.name, '') AS tenant_name
FROM ranked r
LEFT JOIN iam.tenant_domains td
  ON td.tenant_uuid = r.tenant_uuid
 AND td.is_primary = true
LEFT JOIN iam.tenants t
  ON t.id = r.tenant_uuid
ORDER BY r.org_count DESC, r.tenant_uuid
LIMIT 1;
`

	orgunitStoplineChainTenantSQL = `
WITH orgs AS (
  SELECT tenant_uuid, count(*) AS org_count
  FROM orgunit.org_unit_versions
  WHERE validity @> $1::date
  GROUP BY tenant_uuid
),
bindings AS (
  SELECT tenant_uuid, count(*) AS binding_count
  FROM orgunit.setid_binding_versions
  WHERE validity @> $1::date
  GROUP BY tenant_uuid
),
positions AS (
  SELECT tenant_uuid, count(*) AS position_count
  FROM staffing.position_versions
  WHERE validity @> $1::date
  GROUP BY tenant_uuid
)
SELECT
  o.tenant_uuid::text,
  COALESCE(td.hostname, '') AS hostname,
  COALESCE(t.name, '') AS tenant_name
FROM orgs o
JOIN bindings b
  ON b.tenant_uuid = o.tenant_uuid
JOIN positions p
  ON p.tenant_uuid = o.tenant_uuid
LEFT JOIN iam.tenant_domains td
  ON td.tenant_uuid = o.tenant_uuid
 AND td.is_primary = true
LEFT JOIN iam.tenants t
  ON t.id = o.tenant_uuid
WHERE o.org_count > 1
  AND b.binding_count > 0
  AND p.position_count > 0
ORDER BY p.position_count DESC, o.org_count DESC, b.binding_count DESC, o.tenant_uuid
LIMIT 1;
`

	orgunitStoplineHeavyNodesSQL = `
SELECT
  v.org_id,
  COALESCE(v.parent_id, 0) AS parent_id,
  c.org_code,
  v.name,
  v.node_path::text AS node_path,
  COALESCE(pc.org_code, '') AS parent_org_code,
  (
    SELECT count(*)
    FROM orgunit.org_unit_versions d
    WHERE d.tenant_uuid = v.tenant_uuid
      AND d.status = 'active'
      AND d.validity @> $2::date
      AND d.node_path <@ v.node_path
  ) AS subtree_size
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_id = v.org_id
LEFT JOIN orgunit.org_unit_codes pc
  ON pc.tenant_uuid = v.tenant_uuid
 AND pc.org_id = v.parent_id
WHERE v.tenant_uuid = $1::uuid
  AND v.status = 'active'
  AND v.validity @> $2::date
ORDER BY v.node_path;
`

	orgunitStoplineChainBusinessUnitSQL = `
WITH active_positions AS (
  SELECT DISTINCT org_unit_id
  FROM staffing.position_versions
  WHERE tenant_uuid = $1::uuid
    AND validity @> $2::date
),
bound_business_units AS (
  SELECT
    v.org_id,
    c.org_code,
    v.name,
    b.setid
  FROM orgunit.setid_binding_versions b
  JOIN orgunit.org_unit_versions v
    ON v.tenant_uuid = b.tenant_uuid
   AND v.org_id = b.org_id
  JOIN orgunit.org_unit_codes c
    ON c.tenant_uuid = b.tenant_uuid
   AND c.org_id = b.org_id
  WHERE b.tenant_uuid = $1::uuid
    AND b.validity @> $2::date
    AND v.validity @> $2::date
    AND v.status = 'active'
    AND v.is_business_unit = true
)
SELECT
  b.org_id,
  b.org_code,
  b.name,
  b.setid
FROM bound_business_units b
JOIN active_positions p
  ON p.org_unit_id = b.org_id
ORDER BY b.org_code
LIMIT 1;
`

	orgunitStoplineChainPositionSQL = `
SELECT
  pv.position_uuid::text,
  pv.org_unit_id,
  COALESCE(pv.name, '') AS position_name,
  COALESCE(pv.jobcatalog_setid, '') AS jobcatalog_setid,
  lower(pv.validity)::text AS effective_date
FROM staffing.position_versions pv
WHERE pv.tenant_uuid = $1::uuid
  AND pv.validity @> $2::date
  AND pv.org_unit_id = $3::int
ORDER BY lower(pv.validity), pv.position_uuid
LIMIT 1;
`

	sourceOrgRootsSQL = `
SELECT
  v.org_id::text,
  c.org_code,
  v.name,
  v.is_business_unit,
  EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions child
    WHERE child.tenant_uuid = $1::uuid
      AND child.parent_id = v.org_id
      AND child.status = 'active'
      AND child.validity @> $2::date
  ) AS has_children
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_id = v.org_id
WHERE v.tenant_uuid = $1::uuid
  AND v.status = 'active'
  AND v.validity @> $2::date
  AND v.parent_id IS NULL
ORDER BY v.node_path;
`

	sourceOrgChildrenSQL = `
SELECT
  v.org_id::text,
  c.org_code,
  v.name,
  v.is_business_unit,
  EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions child
    WHERE child.tenant_uuid = $1::uuid
      AND child.parent_id = v.org_id
      AND child.status = 'active'
      AND child.validity @> $3::date
  ) AS has_children
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_id = v.org_id
WHERE v.tenant_uuid = $1::uuid
  AND v.parent_id = $2::int
  AND v.status = 'active'
  AND v.validity @> $3::date
ORDER BY v.node_path;
`

	sourceOrgDetailsSQL = `
SELECT
  v.org_id::text,
  c.org_code,
  v.name,
  v.status,
  COALESCE(v.parent_id, 0) AS parent_id,
  COALESCE(pc.org_code, '') AS parent_org_code,
  COALESCE(pv.name, '') AS parent_name,
  v.is_business_unit,
  v.path_ids,
  COALESCE(v.full_name_path, '') AS full_name_path
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_id = v.org_id
LEFT JOIN orgunit.org_unit_codes pc
  ON pc.tenant_uuid = v.tenant_uuid
 AND pc.org_id = v.parent_id
LEFT JOIN orgunit.org_unit_versions pv
  ON pv.tenant_uuid = v.tenant_uuid
 AND pv.org_id = v.parent_id
 AND pv.status = 'active'
 AND pv.validity @> $3::date
WHERE v.tenant_uuid = $1::uuid
  AND v.org_id = $2::int
  AND v.status = 'active'
  AND v.validity @> $3::date
LIMIT 1;
`

	sourceOrgSearchSQL = `
SELECT v.org_id::text, c.org_code, v.name
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_id = v.org_id
WHERE v.tenant_uuid = $1::uuid
  AND v.status = 'active'
  AND v.validity @> $3::date
  AND v.name ILIKE $2::text
ORDER BY v.node_path
LIMIT $4::int;
`

	sourceOrgSubtreeFilterSQL = `
SELECT v.org_id::text, c.org_code, v.node_path::text
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_id = v.org_id
WHERE v.tenant_uuid = $1::uuid
  AND v.status = 'active'
  AND v.validity @> $2::date
  AND v.node_path <@ $3::ltree
ORDER BY v.node_path;
`

	sourceOrgAncestorChainSQL = `
SELECT string_agg(a.name, ' / ' ORDER BY t.idx)
FROM orgunit.org_unit_versions v
JOIN LATERAL unnest(v.path_ids) WITH ORDINALITY AS t(uid, idx)
  ON true
JOIN orgunit.org_unit_versions a
  ON a.tenant_uuid = v.tenant_uuid
 AND a.org_id = t.uid
 AND a.validity @> lower(v.validity)
WHERE v.tenant_uuid = $1::uuid
  AND v.org_id = $2::int
  AND v.validity @> $3::date
GROUP BY v.org_id;
`

	sourceOrgFullNamePathSQL = `
UPDATE orgunit.org_unit_versions v
SET full_name_path = (
  SELECT string_agg(a.name, ' / ' ORDER BY t.idx)
  FROM unnest(v.path_ids) WITH ORDINALITY AS t(uid, idx)
  JOIN orgunit.org_unit_versions a
    ON a.tenant_uuid = v.tenant_uuid
   AND a.org_id = t.uid
   AND a.validity @> lower(v.validity)
)
WHERE v.tenant_uuid = $1::uuid
  AND v.node_path <@ $2::ltree
  AND lower(v.validity) >= $3::date;
`

	sourceOrgMoveSQL = `
WITH source_node AS (
  SELECT v.node_path AS old_path, nlevel(v.node_path) AS old_level
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = $1::uuid
    AND v.org_id = $2::int
    AND v.validity @> $4::date
  ORDER BY lower(v.validity) DESC
  LIMIT 1
),
parent_node AS (
  SELECT v.node_path AS new_parent_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = $1::uuid
    AND v.org_id = $3::int
    AND v.validity @> $4::date
  ORDER BY lower(v.validity) DESC
  LIMIT 1
),
split AS (
  SELECT v.*
  FROM orgunit.org_unit_versions v
  CROSS JOIN source_node s
  WHERE v.tenant_uuid = $1::uuid
    AND v.node_path <@ s.old_path
    AND v.validity @> $4::date
    AND lower(v.validity) < $4::date
),
upd AS (
  UPDATE orgunit.org_unit_versions v
  SET validity = daterange(lower(v.validity), $4::date, '[)')
  FROM split s
  WHERE v.id = s.id
  RETURNING s.*
)
INSERT INTO orgunit.org_unit_versions (
  tenant_uuid,
  org_id,
  parent_id,
  node_path,
  validity,
  name,
  full_name_path,
  status,
  is_business_unit,
  manager_uuid,
  last_event_id,
  ext_labels_snapshot
)
SELECT
  u.tenant_uuid,
  u.org_id,
  CASE WHEN u.org_id = $2::int THEN $3::int ELSE u.parent_id END,
  rewritten.new_path,
  daterange($4::date, upper(u.validity), '[)'),
  u.name,
  u.full_name_path,
  u.status,
  u.is_business_unit,
  u.manager_uuid,
  u.last_event_id,
  u.ext_labels_snapshot
FROM upd u
CROSS JOIN source_node s
CROSS JOIN parent_node p
CROSS JOIN LATERAL (
  SELECT CASE
    WHEN u.org_id = $2::int THEN p.new_parent_path || text2ltree(orgunit.org_ltree_label($2::int))
    ELSE p.new_parent_path || text2ltree(orgunit.org_ltree_label($2::int)) || subpath(u.node_path, s.old_level)
  END AS new_path
) rewritten;
`

	sourceSetIDResolveSQL = `
WITH target AS (
  SELECT v.status, v.node_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = $1::uuid
    AND v.org_id = $2::int
    AND v.validity @> $3::date
  ORDER BY lower(v.validity) DESC
  LIMIT 1
)
SELECT b.setid
FROM target t
JOIN orgunit.setid_binding_versions b
  ON b.tenant_uuid = $1::uuid
JOIN orgunit.org_unit_versions o
  ON o.tenant_uuid = b.tenant_uuid
 AND o.org_id = b.org_id
WHERE b.validity @> $3::date
  AND o.validity @> $3::date
  AND o.status = 'active'
  AND o.is_business_unit = true
  AND o.node_path @> t.node_path
ORDER BY nlevel(o.node_path) DESC
LIMIT 1;
`

	sourceStaffingByOrgSQL = `
SELECT
  pv.position_uuid::text,
  pv.org_unit_id,
  COALESCE(pv.jobcatalog_setid, '') AS jobcatalog_setid,
  COALESCE(pv.name, '') AS position_name,
  lower(pv.validity)::text AS effective_date
FROM staffing.position_versions pv
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = pv.tenant_uuid
 AND c.org_id = pv.org_unit_id
WHERE pv.tenant_uuid = $1::uuid
  AND pv.validity @> $2::date
  AND c.org_code = $3::text
ORDER BY lower(pv.validity), pv.position_uuid;
`

	targetStoplineShadowBootstrapSQL = `
CREATE SCHEMA IF NOT EXISTS stopline;

CREATE TABLE IF NOT EXISTS stopline.setid_binding_versions (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  org_node_key char(8) NOT NULL,
  setid text NOT NULL,
  validity daterange NOT NULL,
  CONSTRAINT stopline_setid_binding_validity_check CHECK (lower_inc(validity) AND NOT upper_inc(validity))
);

CREATE INDEX IF NOT EXISTS stopline_setid_binding_lookup_btree
  ON stopline.setid_binding_versions (tenant_uuid, org_node_key, lower(validity));

CREATE INDEX IF NOT EXISTS stopline_setid_binding_active_day_gist
  ON stopline.setid_binding_versions
  USING gist (tenant_uuid gist_uuid_ops, validity);

CREATE TABLE IF NOT EXISTS stopline.position_versions (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  position_uuid uuid NOT NULL,
  org_node_key char(8) NOT NULL,
  jobcatalog_setid text NULL,
  name text NOT NULL DEFAULT '',
  validity daterange NOT NULL,
  CONSTRAINT stopline_position_validity_check CHECK (lower_inc(validity) AND NOT upper_inc(validity))
);

CREATE INDEX IF NOT EXISTS stopline_position_lookup_btree
  ON stopline.position_versions (tenant_uuid, org_node_key, lower(validity));

CREATE INDEX IF NOT EXISTS stopline_position_active_day_gist
  ON stopline.position_versions
  USING gist (tenant_uuid gist_uuid_ops, validity);
`

	targetSetIDResolveSQL = `
WITH target AS (
  SELECT v.status, v.node_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = $1::uuid
    AND v.org_node_key = $2::char(8)
    AND v.validity @> $3::date
  ORDER BY lower(v.validity) DESC
  LIMIT 1
)
SELECT b.setid
FROM target t
JOIN stopline.setid_binding_versions b
  ON b.tenant_uuid = $1::uuid
JOIN orgunit.org_unit_versions o
  ON o.tenant_uuid = b.tenant_uuid
 AND o.org_node_key = b.org_node_key
WHERE b.validity @> $3::date
  AND o.validity @> $3::date
  AND o.status = 'active'
  AND o.is_business_unit = true
  AND o.node_path @> t.node_path
ORDER BY nlevel(o.node_path) DESC
LIMIT 1;
`

	targetStaffingByOrgSQL = `
SELECT
  pv.position_uuid::text,
  c.org_code,
  COALESCE(pv.jobcatalog_setid, '') AS jobcatalog_setid,
  COALESCE(pv.name, '') AS position_name,
  lower(pv.validity)::text AS effective_date
FROM stopline.position_versions pv
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = pv.tenant_uuid
 AND c.org_node_key = pv.org_node_key
WHERE pv.tenant_uuid = $1::uuid
  AND pv.validity @> $2::date
  AND c.org_code = $3::text
ORDER BY lower(pv.validity), pv.position_uuid;
`

	targetOrgRootsSQL = `
SELECT
  v.org_node_key::text,
  c.org_code,
  v.name,
  v.is_business_unit,
  EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions child
    WHERE child.tenant_uuid = $1::uuid
      AND child.parent_org_node_key = v.org_node_key
      AND child.status = 'active'
      AND child.validity @> $2::date
  ) AS has_children
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_node_key = v.org_node_key
WHERE v.tenant_uuid = $1::uuid
  AND v.status = 'active'
  AND v.validity @> $2::date
  AND v.parent_org_node_key IS NULL
ORDER BY v.node_path;
`

	targetOrgChildrenSQL = `
SELECT
  v.org_node_key::text,
  c.org_code,
  v.name,
  v.is_business_unit,
  EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions child
    WHERE child.tenant_uuid = $1::uuid
      AND child.parent_org_node_key = v.org_node_key
      AND child.status = 'active'
      AND child.validity @> $3::date
  ) AS has_children
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_node_key = v.org_node_key
WHERE v.tenant_uuid = $1::uuid
  AND v.parent_org_node_key = $2::char(8)
  AND v.status = 'active'
  AND v.validity @> $3::date
ORDER BY v.node_path;
`

	targetOrgDetailsSQL = `
SELECT
  v.org_node_key::text,
  c.org_code,
  v.name,
  v.status,
  COALESCE(v.parent_org_node_key::text, '') AS parent_org_node_key,
  COALESCE(pc.org_code, '') AS parent_org_code,
  COALESCE(pv.name, '') AS parent_name,
  v.is_business_unit,
  COALESCE(array_to_string(v.path_node_keys, ','), '') AS path_node_keys,
  COALESCE(v.full_name_path, '') AS full_name_path
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_node_key = v.org_node_key
LEFT JOIN orgunit.org_unit_codes pc
  ON pc.tenant_uuid = v.tenant_uuid
 AND pc.org_node_key = v.parent_org_node_key
LEFT JOIN orgunit.org_unit_versions pv
  ON pv.tenant_uuid = v.tenant_uuid
 AND pv.org_node_key = v.parent_org_node_key
 AND pv.status = 'active'
 AND pv.validity @> $3::date
WHERE v.tenant_uuid = $1::uuid
  AND v.org_node_key = $2::char(8)
  AND v.status = 'active'
  AND v.validity @> $3::date
LIMIT 1;
`

	targetOrgSearchSQL = `
SELECT v.org_node_key::text, c.org_code, v.name
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_node_key = v.org_node_key
WHERE v.tenant_uuid = $1::uuid
  AND v.status = 'active'
  AND v.validity @> $3::date
  AND v.name ILIKE $2::text
ORDER BY v.node_path
LIMIT $4::int;
`

	targetOrgSubtreeFilterSQL = `
SELECT v.org_node_key::text, c.org_code, v.node_path::text
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_node_key = v.org_node_key
WHERE v.tenant_uuid = $1::uuid
  AND v.status = 'active'
  AND v.validity @> $2::date
  AND v.node_path <@ $3::ltree
ORDER BY v.node_path;
`

	targetOrgAncestorChainSQL = `
SELECT string_agg(a.name, ' / ' ORDER BY t.idx)
FROM orgunit.org_unit_versions v
JOIN LATERAL unnest(v.path_node_keys) WITH ORDINALITY AS t(org_node_key, idx)
  ON true
JOIN orgunit.org_unit_versions a
  ON a.tenant_uuid = v.tenant_uuid
 AND a.org_node_key = t.org_node_key::char(8)
 AND a.validity @> lower(v.validity)
WHERE v.tenant_uuid = $1::uuid
  AND v.org_node_key = $2::char(8)
  AND v.validity @> $3::date
GROUP BY v.org_node_key;
`

	targetOrgFullNamePathSQL = `
UPDATE orgunit.org_unit_versions v
SET full_name_path = (
  SELECT string_agg(a.name, ' / ' ORDER BY t.idx)
  FROM unnest(v.path_node_keys) WITH ORDINALITY AS t(org_node_key, idx)
  JOIN orgunit.org_unit_versions a
    ON a.tenant_uuid = v.tenant_uuid
   AND a.org_node_key = t.org_node_key::char(8)
   AND a.validity @> lower(v.validity)
)
WHERE v.tenant_uuid = $1::uuid
  AND v.node_path <@ $2::ltree
  AND lower(v.validity) >= $3::date;
`

	targetOrgMoveSQL = `
WITH source_node AS (
  SELECT v.node_path AS old_path, nlevel(v.node_path) AS old_level
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = $1::uuid
    AND v.org_node_key = $2::char(8)
    AND v.validity @> $4::date
  ORDER BY lower(v.validity) DESC
  LIMIT 1
),
parent_node AS (
  SELECT v.node_path AS new_parent_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = $1::uuid
    AND v.org_node_key = $3::char(8)
    AND v.validity @> $4::date
  ORDER BY lower(v.validity) DESC
  LIMIT 1
),
split AS (
  SELECT v.*
  FROM orgunit.org_unit_versions v
  CROSS JOIN source_node s
  WHERE v.tenant_uuid = $1::uuid
    AND v.node_path <@ s.old_path
    AND v.validity @> $4::date
    AND lower(v.validity) < $4::date
),
upd AS (
  UPDATE orgunit.org_unit_versions v
  SET validity = daterange(lower(v.validity), $4::date, '[)')
  FROM split s
  WHERE v.id = s.id
  RETURNING s.*
)
INSERT INTO orgunit.org_unit_versions (
  tenant_uuid,
  org_node_key,
  parent_org_node_key,
  node_path,
  validity,
  name,
  full_name_path,
  status,
  is_business_unit,
  manager_uuid,
  last_event_id
)
SELECT
  u.tenant_uuid,
  u.org_node_key,
  CASE WHEN u.org_node_key = $2::char(8) THEN $3::char(8) ELSE u.parent_org_node_key END,
  rewritten.new_path,
  daterange($4::date, upper(u.validity), '[)'),
  u.name,
  u.full_name_path,
  u.status,
  u.is_business_unit,
  u.manager_uuid,
  u.last_event_id
FROM upd u
CROSS JOIN source_node s
CROSS JOIN parent_node p
CROSS JOIN LATERAL (
  SELECT CASE
    WHEN u.org_node_key = $2::char(8) THEN p.new_parent_path || text2ltree(orgunit.org_ltree_label($2::text))
    ELSE p.new_parent_path || text2ltree(orgunit.org_ltree_label($2::text)) || subpath(u.node_path, s.old_level)
  END AS new_path
) rewritten;
`
)

func orgunitStoplineCapture(args []string) {
	fs := flag.NewFlagSet("orgunit-stopline-capture", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var sourceURL string
	var targetURL string
	var asOfDate string
	var outputDir string
	fs.StringVar(&sourceURL, "source-url", "", "postgres connection string for the current org_id runtime database")
	fs.StringVar(&targetURL, "target-url", "", "postgres connection string for the org_node_key rehearsal database")
	fs.StringVar(&asOfDate, "as-of", "", "as-of date (YYYY-MM-DD)")
	fs.StringVar(&outputDir, "output-dir", "", "directory for explain JSON and reports")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if sourceURL == "" {
		fatalf("missing --source-url")
	}
	if targetURL == "" {
		fatalf("missing --target-url")
	}
	if asOfDate == "" {
		fatalf("missing --as-of")
	}
	if outputDir == "" {
		fatalf("missing --output-dir")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	sourceConn, err := pgx.Connect(ctx, sourceURL)
	if err != nil {
		fatal(err)
	}
	defer sourceConn.Close(context.Background())

	targetConn, err := pgx.Connect(ctx, targetURL)
	if err != nil {
		fatal(err)
	}
	defer targetConn.Close(context.Background())

	samples, err := discoverOrgunitStoplineSamples(ctx, sourceConn, targetConn, asOfDate)
	if err != nil {
		fatal(err)
	}
	if err := prepareTargetStoplineShadowData(ctx, sourceConn, targetConn, samples); err != nil {
		fatal(err)
	}

	specs := buildOrgunitStoplineSpecs(samples)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fatal(err)
	}

	results := make([]orgunitStoplineExplainRecord, 0, len(specs))
	for _, spec := range specs {
		conn := sourceConn
		if spec.Stage == "target-real" || spec.Stage == "target-shadow" {
			conn = targetConn
		}
		record, err := captureOrgunitStoplineExplain(ctx, conn, outputDir, spec)
		if err != nil {
			fatalf("%s: %v", spec.Key, err)
		}
		results = append(results, record)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Stage == results[j].Stage {
			return results[i].Key < results[j].Key
		}
		return results[i].Stage < results[j].Stage
	})

	report := orgunitStoplineReport{
		CapturedAt: time.Now().UTC(),
		AsOfDate:   asOfDate,
		Samples:    samples,
		Results:    results,
	}
	if err := writeJSONFile(filepath.Join(outputDir, "report.json"), report); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "report.md"), []byte(renderOrgunitStoplineMarkdown(report)), 0o644); err != nil {
		fatal(err)
	}
	if err := writeJSONFile(filepath.Join(outputDir, "samples.json"), samples); err != nil {
		fatal(err)
	}

	fmt.Printf("[orgunit-stopline-capture] OK output_dir=%s results=%d\n", outputDir, len(results))
}

func discoverOrgunitStoplineSamples(ctx context.Context, sourceConn *pgx.Conn, targetConn *pgx.Conn, asOfDate string) (orgunitStoplineSamples, error) {
	var samples orgunitStoplineSamples
	samples.AsOfDate = asOfDate

	heavyTenant, err := queryTenantSample(ctx, sourceConn, orgunitStoplineHeavyTenantSQL, asOfDate)
	if err != nil {
		return samples, err
	}
	if heavyTenant.TenantUUID == "" {
		return samples, fmt.Errorf("heavy org tenant not found for as_of=%s", asOfDate)
	}
	samples.Heavy.Tenant = heavyTenant
	samples.Heavy.MoveEffectiveDate = nextDay(asOfDate)

	chainTenant, err := queryTenantSample(ctx, sourceConn, orgunitStoplineChainTenantSQL, asOfDate)
	if err != nil {
		return samples, err
	}
	if chainTenant.TenantUUID == "" {
		return samples, fmt.Errorf("chain tenant not found for as_of=%s", asOfDate)
	}
	samples.Chain.Tenant = chainTenant

	heavyNodes, err := queryHeavyNodes(ctx, sourceConn, heavyTenant.TenantUUID, asOfDate)
	if err != nil {
		return samples, err
	}
	if len(heavyNodes) == 0 {
		return samples, fmt.Errorf("heavy tenant %s has no active org nodes", heavyTenant.TenantUUID)
	}

	root := heavyNodes[0]
	for _, node := range heavyNodes {
		if node.ParentOrgID == 0 {
			root = node
			break
		}
	}
	samples.Heavy.Root = root
	samples.Heavy.ChildrenParent = root

	detailsTarget := firstHeavyLeaf(heavyNodes, root.OrgCode)
	samples.Heavy.DetailsTarget = detailsTarget
	samples.Heavy.SubtreeFilter = firstHeavySubtree(heavyNodes)
	samples.Heavy.MoveTarget, samples.Heavy.MoveNewParent = pickHeavyMoveNodes(heavyNodes, root.OrgID)
	if samples.Heavy.MoveTarget.OrgID == 0 || samples.Heavy.MoveNewParent.OrgID == 0 {
		return samples, fmt.Errorf("failed to pick heavy move nodes for tenant %s", heavyTenant.TenantUUID)
	}
	samples.Heavy.SearchQuery = firstNonEmpty([]string{
		findSearchQuery(heavyNodes, "人力资源部"),
		findSearchQuery(heavyNodes, "共享服务中心"),
		strings.TrimSpace(samples.Heavy.DetailsTarget.Name),
	})

	businessUnit, err := queryChainBusinessUnit(ctx, sourceConn, chainTenant.TenantUUID, asOfDate)
	if err != nil {
		return samples, err
	}
	if businessUnit.OrgID == 0 {
		return samples, fmt.Errorf("chain tenant %s has no bound business unit with positions", chainTenant.TenantUUID)
	}
	samples.Chain.BusinessUnit = businessUnit

	positionSample, err := queryChainPosition(ctx, sourceConn, chainTenant.TenantUUID, asOfDate, businessUnit.OrgID)
	if err != nil {
		return samples, err
	}
	samples.Chain.PositionSample = positionSample

	if err := fillTargetKeys(ctx, targetConn, &samples); err != nil {
		return samples, err
	}
	return samples, nil
}

func buildOrgunitStoplineSpecs(samples orgunitStoplineSamples) []orgunitStoplineCaptureSpec {
	asOfDate := samples.AsOfDate
	heavyTenant := samples.Heavy.Tenant.TenantUUID
	heavyRoot := samples.Heavy.Root
	heavyDetails := samples.Heavy.DetailsTarget
	heavySubtree := samples.Heavy.SubtreeFilter
	heavyMoveTarget := samples.Heavy.MoveTarget
	heavyMoveNewParent := samples.Heavy.MoveNewParent
	heavySearch := "%" + samples.Heavy.SearchQuery + "%"
	chainTenant := samples.Chain.Tenant.TenantUUID
	chainBU := samples.Chain.BusinessUnit

	return []orgunitStoplineCaptureSpec{
		{
			Key:         "org-roots",
			Stage:       "source-real",
			Description: "旧 org_id 运行库：根节点列表主查询",
			SQL:         sourceOrgRootsSQL,
			Args:        []any{heavyTenant, asOfDate},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "org-children",
			Stage:       "source-real",
			Description: "旧 org_id 运行库：children 主查询",
			SQL:         sourceOrgChildrenSQL,
			Args:        []any{heavyTenant, heavyRoot.OrgID, asOfDate},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "org-details",
			Stage:       "source-real",
			Description: "旧 org_id 运行库：详情页主查询",
			SQL:         sourceOrgDetailsSQL,
			Args:        []any{heavyTenant, heavyDetails.OrgID, asOfDate},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "org-search",
			Stage:       "source-real",
			Description: "旧 org_id 运行库：搜索候选主查询",
			SQL:         sourceOrgSearchSQL,
			Args:        []any{heavyTenant, heavySearch, asOfDate, 8},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "org-subtree-filter",
			Stage:       "source-real",
			Description: "旧 org_id 运行库：node_path <@ 子树过滤",
			SQL:         sourceOrgSubtreeFilterSQL,
			Args:        []any{heavyTenant, asOfDate, heavySubtree.NodePath},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "org-ancestor-chain",
			Stage:       "source-real",
			Description: "旧 org_id 运行库：path_ids 祖先链展开",
			SQL:         sourceOrgAncestorChainSQL,
			Args:        []any{heavyTenant, heavyDetails.OrgID, asOfDate},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "org-full-name-rebuild",
			Stage:       "source-real",
			Description: "旧 org_id 运行库：full_name_path 更新 SQL",
			SQL:         sourceOrgFullNamePathSQL,
			Args:        []any{heavyTenant, heavySubtree.NodePath, asOfDate},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "org-move",
			Stage:       "source-real",
			Description: "旧 org_id 运行库：move 子树版本切分与重写 SQL",
			SQL:         sourceOrgMoveSQL,
			Args:        []any{heavyTenant, heavyMoveTarget.OrgID, heavyMoveNewParent.OrgID, samples.Heavy.MoveEffectiveDate},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "setid-resolve",
			Stage:       "source-real",
			Description: "旧 org_id 运行库：SetID 基于组织祖先链解析",
			SQL:         sourceSetIDResolveSQL,
			Args:        []any{chainTenant, chainBU.OrgID, asOfDate},
			TenantUUID:  chainTenant,
		},
		{
			Key:         "staffing-by-org",
			Stage:       "source-real",
			Description: "旧 org_id 运行库：Staffing 通过组织引用联查 position",
			SQL:         sourceStaffingByOrgSQL,
			Args:        []any{chainTenant, asOfDate, chainBU.OrgCode},
			TenantUUID:  chainTenant,
		},
		{
			Key:         "setid-resolve",
			Stage:       "target-shadow",
			Description: "target org_node_key 库 + stopline shadow：SetID 基于组织祖先链解析",
			SQL:         targetSetIDResolveSQL,
			Args:        []any{chainTenant, chainBU.OrgNodeKey, asOfDate},
			TenantUUID:  chainTenant,
			Notes: []string{
				"consumer runtime 的 SetID schema 尚未切到 org_node_key",
				"此处使用 stopline shadow 表按 org_code -> org_node_key 导入当前态样本，仅用于 explain 对比",
			},
		},
		{
			Key:         "staffing-by-org",
			Stage:       "target-shadow",
			Description: "target org_node_key 库 + stopline shadow：Staffing 通过组织引用联查 position",
			SQL:         targetStaffingByOrgSQL,
			Args:        []any{chainTenant, asOfDate, chainBU.OrgCode},
			TenantUUID:  chainTenant,
			Notes: []string{
				"consumer runtime 的 Staffing schema 尚未切到 org_node_key",
				"此处使用 stopline shadow 表按 org_code -> org_node_key 导入当前态样本，仅用于 explain 对比",
			},
		},
		{
			Key:         "org-roots",
			Stage:       "target-real",
			Description: "target org_node_key 库：根节点列表等价 SQL",
			SQL:         targetOrgRootsSQL,
			Args:        []any{heavyTenant, asOfDate},
			TenantUUID:  heavyTenant,
			Notes:       []string{"target bootstrap 当前仅覆盖 org 纯 schema；无 person/setid/staffing 关联表"},
		},
		{
			Key:         "org-children",
			Stage:       "target-real",
			Description: "target org_node_key 库：children 等价 SQL",
			SQL:         targetOrgChildrenSQL,
			Args:        []any{heavyTenant, heavyRoot.OrgNodeKey, asOfDate},
			TenantUUID:  heavyTenant,
			Notes:       []string{"target bootstrap 当前仅覆盖 org 纯 schema；无 person/setid/staffing 关联表"},
		},
		{
			Key:         "org-details",
			Stage:       "target-real",
			Description: "target org_node_key 库：详情页等价 SQL（不含 person 侧 manager join）",
			SQL:         targetOrgDetailsSQL,
			Args:        []any{heavyTenant, heavyDetails.OrgNodeKey, asOfDate},
			TenantUUID:  heavyTenant,
			Notes:       []string{"manager/person 关联未纳入当前 target bootstrap"},
		},
		{
			Key:         "org-search",
			Stage:       "target-real",
			Description: "target org_node_key 库：搜索候选等价 SQL",
			SQL:         targetOrgSearchSQL,
			Args:        []any{heavyTenant, heavySearch, asOfDate, 8},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "org-subtree-filter",
			Stage:       "target-real",
			Description: "target org_node_key 库：node_path <@ 子树过滤",
			SQL:         targetOrgSubtreeFilterSQL,
			Args:        []any{heavyTenant, asOfDate, heavySubtree.NodePath},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "org-ancestor-chain",
			Stage:       "target-real",
			Description: "target org_node_key 库：path_node_keys 祖先链展开",
			SQL:         targetOrgAncestorChainSQL,
			Args:        []any{heavyTenant, heavyDetails.OrgNodeKey, asOfDate},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "org-full-name-rebuild",
			Stage:       "target-real",
			Description: "target org_node_key 库：full_name_path 更新 SQL",
			SQL:         targetOrgFullNamePathSQL,
			Args:        []any{heavyTenant, heavySubtree.NodePath, asOfDate},
			TenantUUID:  heavyTenant,
		},
		{
			Key:         "org-move",
			Stage:       "target-real",
			Description: "target org_node_key 库：move 子树版本切分与重写 SQL",
			SQL:         targetOrgMoveSQL,
			Args:        []any{heavyTenant, heavyMoveTarget.OrgNodeKey, heavyMoveNewParent.OrgNodeKey, samples.Heavy.MoveEffectiveDate},
			TenantUUID:  heavyTenant,
		},
	}
}

func captureOrgunitStoplineExplain(ctx context.Context, conn *pgx.Conn, outputDir string, spec orgunitStoplineCaptureSpec) (orgunitStoplineExplainRecord, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return orgunitStoplineExplainRecord{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if strings.TrimSpace(spec.TenantUUID) != "" {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, spec.TenantUUID); err != nil {
			return orgunitStoplineExplainRecord{}, err
		}
	}

	explainSQL := "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) " + spec.SQL
	var raw any
	if err := tx.QueryRow(ctx, explainSQL, spec.Args...).Scan(&raw); err != nil {
		return orgunitStoplineExplainRecord{}, err
	}
	rawBytes := normalizeExplainPayload(raw)
	metrics, err := summarizeOrgunitExplain(rawBytes)
	if err != nil {
		return orgunitStoplineExplainRecord{}, err
	}

	fileName := fmt.Sprintf("%s-%s.explain.json", spec.Stage, spec.Key)
	filePath := filepath.Join(outputDir, fileName)
	var normalized any
	if err := json.Unmarshal(rawBytes, &normalized); err != nil {
		return orgunitStoplineExplainRecord{}, err
	}
	if err := writeJSONFile(filePath, normalized); err != nil {
		return orgunitStoplineExplainRecord{}, err
	}

	return orgunitStoplineExplainRecord{
		Key:         spec.Key,
		Stage:       spec.Stage,
		Description: spec.Description,
		SQL:         strings.TrimSpace(spec.SQL),
		Args:        append([]any(nil), spec.Args...),
		TenantUUID:  spec.TenantUUID,
		Notes:       append([]string(nil), spec.Notes...),
		Metrics:     metrics,
		RawJSONFile: fileName,
	}, nil
}

func summarizeOrgunitExplain(raw []byte) (orgunitStoplineExplainMetrics, error) {
	var envelope orgunitStoplineExplainEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return orgunitStoplineExplainMetrics{}, err
	}
	if len(envelope) == 0 {
		return orgunitStoplineExplainMetrics{}, fmt.Errorf("empty explain payload")
	}
	root := envelope[0]
	plan, _ := root["Plan"].(map[string]any)
	return orgunitStoplineExplainMetrics{
		PlanningTimeMS:      asFloat64(root["Planning Time"]),
		ExecutionTimeMS:     asFloat64(root["Execution Time"]),
		PlanRows:            asFloat64(plan["Plan Rows"]),
		ActualRows:          asFloat64(plan["Actual Rows"]),
		SharedHitBlocks:     asInt64(plan["Shared Hit Blocks"]),
		SharedReadBlocks:    asInt64(plan["Shared Read Blocks"]),
		SharedDirtiedBlocks: asInt64(plan["Shared Dirtied Blocks"]),
		SharedWrittenBlocks: asInt64(plan["Shared Written Blocks"]),
	}, nil
}

func renderOrgunitStoplineMarkdown(report orgunitStoplineReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# DEV-PLAN-320 Stopline Explain Report\n\n")
	fmt.Fprintf(&b, "- captured_at: `%s`\n", report.CapturedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- as_of_date: `%s`\n\n", report.AsOfDate)

	fmt.Fprintf(&b, "## Samples\n\n")
	fmt.Fprintf(&b, "- heavy tenant: `%s` (`%s`, `%s`)\n", report.Samples.Heavy.Tenant.TenantUUID, report.Samples.Heavy.Tenant.Name, report.Samples.Heavy.Tenant.Hostname)
	fmt.Fprintf(&b, "- heavy root: `%s` / `%s`\n", report.Samples.Heavy.Root.OrgCode, report.Samples.Heavy.Root.OrgNodeKey)
	fmt.Fprintf(&b, "- heavy details target: `%s` / `%s`\n", report.Samples.Heavy.DetailsTarget.OrgCode, report.Samples.Heavy.DetailsTarget.OrgNodeKey)
	fmt.Fprintf(&b, "- heavy subtree filter: `%s` / `%s`\n", report.Samples.Heavy.SubtreeFilter.OrgCode, report.Samples.Heavy.SubtreeFilter.NodePath)
	fmt.Fprintf(&b, "- heavy move: `%s` -> `%s` at `%s`\n", report.Samples.Heavy.MoveTarget.OrgCode, report.Samples.Heavy.MoveNewParent.OrgCode, report.Samples.Heavy.MoveEffectiveDate)
	fmt.Fprintf(&b, "- chain tenant: `%s` (`%s`, `%s`)\n", report.Samples.Chain.Tenant.TenantUUID, report.Samples.Chain.Tenant.Name, report.Samples.Chain.Tenant.Hostname)
	fmt.Fprintf(&b, "- chain business unit: `%s` setid=`%s`\n\n", report.Samples.Chain.BusinessUnit.OrgCode, report.Samples.Chain.BusinessUnit.SetID)

	byStage := make(map[string][]orgunitStoplineExplainRecord)
	for _, result := range report.Results {
		byStage[result.Stage] = append(byStage[result.Stage], result)
	}
	stages := make([]string, 0, len(byStage))
	for stage := range byStage {
		stages = append(stages, stage)
	}
	sort.Strings(stages)
	for _, stage := range stages {
		sort.Slice(byStage[stage], func(i, j int) bool { return byStage[stage][i].Key < byStage[stage][j].Key })
		fmt.Fprintf(&b, "## %s\n\n", stage)
		fmt.Fprintf(&b, "| key | execution_ms | planning_ms | shared_hit | shared_read | shared_dirtied | shared_written | explain |\n")
		fmt.Fprintf(&b, "| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n")
		for _, result := range byStage[stage] {
			fmt.Fprintf(
				&b,
				"| `%s` | %.3f | %.3f | %d | %d | %d | %d | `%s` |\n",
				result.Key,
				result.Metrics.ExecutionTimeMS,
				result.Metrics.PlanningTimeMS,
				result.Metrics.SharedHitBlocks,
				result.Metrics.SharedReadBlocks,
				result.Metrics.SharedDirtiedBlocks,
				result.Metrics.SharedWrittenBlocks,
				result.RawJSONFile,
			)
			if len(result.Notes) > 0 {
				fmt.Fprintf(&b, "\n说明：%s\n\n", strings.Join(result.Notes, "；"))
			}
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "## Notes\n\n")
	fmt.Fprintf(&b, "1. `target-real` 当前只覆盖 `orgunit` 新 schema；SetID / Staffing / Person 相关 post-cutover explain 尚未纳入 dedicated target bootstrap。\n")
	fmt.Fprintf(&b, "2. `target-shadow` 使用 stopline shadow 表承载 SetID / Staffing 当前态样本，并通过 `org_code -> org_node_key` 映射导入 dedicated target；该证据仅用于 stopline 对比，不等同于 consumer runtime 已完成 cutover。\n")
	fmt.Fprintf(&b, "3. `org-move` 与 `org-full-name-rebuild` 均在事务内执行 `EXPLAIN (ANALYZE, BUFFERS)`，采集后由调用侧回滚。\n")
	fmt.Fprintf(&b, "4. 原始 explain JSON 见同目录 `*.explain.json`。\n")
	return b.String()
}

func queryTenantSample(ctx context.Context, conn *pgx.Conn, sql string, asOfDate string) (orgunitStoplineTenantSample, error) {
	var sample orgunitStoplineTenantSample
	if err := conn.QueryRow(ctx, sql, asOfDate).Scan(&sample.TenantUUID, &sample.Hostname, &sample.Name); err != nil {
		if err == pgx.ErrNoRows {
			return orgunitStoplineTenantSample{}, nil
		}
		return orgunitStoplineTenantSample{}, err
	}
	return sample, nil
}

func queryHeavyNodes(ctx context.Context, conn *pgx.Conn, tenantUUID string, asOfDate string) ([]orgunitStoplineNodeSample, error) {
	rows, err := conn.Query(ctx, orgunitStoplineHeavyNodesSQL, tenantUUID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []orgunitStoplineNodeSample
	for rows.Next() {
		var item orgunitStoplineNodeSample
		if err := rows.Scan(&item.OrgID, &item.ParentOrgID, &item.OrgCode, &item.Name, &item.NodePath, &item.ParentOrgCode, &item.SubtreeSize); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func queryChainBusinessUnit(ctx context.Context, conn *pgx.Conn, tenantUUID string, asOfDate string) (orgunitStoplineNodeSample, error) {
	var item orgunitStoplineNodeSample
	if err := conn.QueryRow(ctx, orgunitStoplineChainBusinessUnitSQL, tenantUUID, asOfDate).Scan(&item.OrgID, &item.OrgCode, &item.Name, &item.SetID); err != nil {
		if err == pgx.ErrNoRows {
			return orgunitStoplineNodeSample{}, nil
		}
		return orgunitStoplineNodeSample{}, err
	}
	return item, nil
}

func queryChainPosition(ctx context.Context, conn *pgx.Conn, tenantUUID string, asOfDate string, orgID int) (orgunitStoplineNodeSample, error) {
	var item orgunitStoplineNodeSample
	if err := conn.QueryRow(ctx, orgunitStoplineChainPositionSQL, tenantUUID, asOfDate, orgID).Scan(&item.PositionUUID, &item.OrgUnitID, &item.Name, &item.JobCatalogSetID, &item.EffectiveDate); err != nil {
		if err == pgx.ErrNoRows {
			return orgunitStoplineNodeSample{}, nil
		}
		return orgunitStoplineNodeSample{}, err
	}
	item.PositionCount = 1
	return item, nil
}

func fillTargetKeys(ctx context.Context, targetConn *pgx.Conn, samples *orgunitStoplineSamples) error {
	type mapping struct {
		tenantUUID string
		codes      []string
		set        func(string, string, string)
	}
	targetMappings := []mapping{
		{
			tenantUUID: samples.Heavy.Tenant.TenantUUID,
			codes: []string{
				samples.Heavy.Root.OrgCode,
				samples.Heavy.DetailsTarget.OrgCode,
				samples.Heavy.SubtreeFilter.OrgCode,
				samples.Heavy.MoveTarget.OrgCode,
				samples.Heavy.MoveNewParent.OrgCode,
			},
			set: func(orgCode string, orgNodeKey string, nodePath string) {
				switch orgCode {
				case samples.Heavy.Root.OrgCode:
					samples.Heavy.Root.OrgNodeKey = orgNodeKey
				case samples.Heavy.DetailsTarget.OrgCode:
					samples.Heavy.DetailsTarget.OrgNodeKey = orgNodeKey
				case samples.Heavy.SubtreeFilter.OrgCode:
					samples.Heavy.SubtreeFilter.OrgNodeKey = orgNodeKey
					samples.Heavy.SubtreeFilter.NodePath = nodePath
				case samples.Heavy.MoveTarget.OrgCode:
					samples.Heavy.MoveTarget.OrgNodeKey = orgNodeKey
				case samples.Heavy.MoveNewParent.OrgCode:
					samples.Heavy.MoveNewParent.OrgNodeKey = orgNodeKey
				}
			},
		},
		{
			tenantUUID: samples.Chain.Tenant.TenantUUID,
			codes: []string{
				samples.Chain.BusinessUnit.OrgCode,
			},
			set: func(orgCode string, orgNodeKey string, nodePath string) {
				if orgCode == samples.Chain.BusinessUnit.OrgCode {
					samples.Chain.BusinessUnit.OrgNodeKey = orgNodeKey
					samples.Chain.BusinessUnit.NodePath = nodePath
				}
			},
		},
	}
	for _, m := range targetMappings {
		codeSet := make(map[string]struct{}, len(m.codes))
		for _, code := range m.codes {
			codeSet[code] = struct{}{}
		}
		rows, err := targetConn.Query(ctx, `
SELECT c.org_code, c.org_node_key::text, v.node_path::text
FROM orgunit.org_unit_codes c
JOIN orgunit.org_unit_versions v
  ON v.tenant_uuid = c.tenant_uuid
 AND v.org_node_key = c.org_node_key
WHERE c.tenant_uuid = $1::uuid
  AND v.validity @> $2::date
`, m.tenantUUID, samples.AsOfDate)
		if err != nil {
			return err
		}
		for rows.Next() {
			var orgCode, orgNodeKey, nodePath string
			if err := rows.Scan(&orgCode, &orgNodeKey, &nodePath); err != nil {
				rows.Close()
				return err
			}
			if _, ok := codeSet[orgCode]; ok {
				m.set(orgCode, orgNodeKey, nodePath)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	}
	return nil
}

func prepareTargetStoplineShadowData(ctx context.Context, sourceConn *pgx.Conn, targetConn *pgx.Conn, samples orgunitStoplineSamples) error {
	tenantUUID := strings.TrimSpace(samples.Chain.Tenant.TenantUUID)
	if tenantUUID == "" {
		return nil
	}

	if _, err := targetConn.Exec(ctx, targetStoplineShadowBootstrapSQL); err != nil {
		return err
	}

	codeMap, err := loadTargetOrgCodeMap(ctx, targetConn, tenantUUID, samples.AsOfDate)
	if err != nil {
		return err
	}

	bindings, err := querySourceStoplineBindings(ctx, sourceConn, tenantUUID, samples.AsOfDate)
	if err != nil {
		return err
	}
	positions, err := querySourceStoplinePositions(ctx, sourceConn, tenantUUID, samples.AsOfDate)
	if err != nil {
		return err
	}

	tx, err := targetConn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `DELETE FROM stopline.setid_binding_versions WHERE tenant_uuid = $1::uuid;`, tenantUUID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM stopline.position_versions WHERE tenant_uuid = $1::uuid;`, tenantUUID); err != nil {
		return err
	}

	for _, row := range bindings {
		orgNodeKey, ok := codeMap[row.OrgCode]
		if !ok {
			return fmt.Errorf("target stopline shadow missing org_code mapping for setid binding tenant=%s org_code=%s", tenantUUID, row.OrgCode)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO stopline.setid_binding_versions (
  tenant_uuid,
  org_node_key,
  setid,
  validity
)
VALUES (
  $1::uuid,
  $2::char(8),
  $3::text,
  daterange($4::date, $5::date, '[)')
);
`, tenantUUID, orgNodeKey, row.SetID, row.ValidFrom, nullableDateValue(row.ValidTo)); err != nil {
			return err
		}
	}

	for _, row := range positions {
		orgNodeKey, ok := codeMap[row.OrgCode]
		if !ok {
			return fmt.Errorf("target stopline shadow missing org_code mapping for staffing position tenant=%s org_code=%s", tenantUUID, row.OrgCode)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO stopline.position_versions (
  tenant_uuid,
  position_uuid,
  org_node_key,
  jobcatalog_setid,
  name,
  validity
)
VALUES (
  $1::uuid,
  $2::uuid,
  $3::char(8),
  NULLIF($4::text, ''),
  $5::text,
  daterange($6::date, $7::date, '[)')
);
`, tenantUUID, row.PositionUUID, orgNodeKey, row.JobCatalogSetID, row.Name, row.ValidFrom, nullableDateValue(row.ValidTo)); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func loadTargetOrgCodeMap(ctx context.Context, targetConn *pgx.Conn, tenantUUID string, asOfDate string) (map[string]string, error) {
	rows, err := targetConn.Query(ctx, `
SELECT c.org_code, c.org_node_key::text
FROM orgunit.org_unit_codes c
JOIN orgunit.org_unit_versions v
  ON v.tenant_uuid = c.tenant_uuid
 AND v.org_node_key = c.org_node_key
WHERE c.tenant_uuid = $1::uuid
  AND v.validity @> $2::date
`, tenantUUID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var orgCode string
		var orgNodeKey string
		if err := rows.Scan(&orgCode, &orgNodeKey); err != nil {
			return nil, err
		}
		out[strings.TrimSpace(orgCode)] = strings.TrimSpace(orgNodeKey)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func querySourceStoplineBindings(ctx context.Context, sourceConn *pgx.Conn, tenantUUID string, asOfDate string) ([]orgunitStoplineShadowBindingRow, error) {
	rows, err := sourceConn.Query(ctx, `
SELECT
  c.org_code,
  b.setid,
  lower(b.validity)::text AS valid_from,
  COALESCE(upper(b.validity)::text, '') AS valid_to
FROM orgunit.setid_binding_versions b
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = b.tenant_uuid
 AND c.org_id = b.org_id
WHERE b.tenant_uuid = $1::uuid
  AND b.validity @> $2::date
ORDER BY c.org_code ASC, lower(b.validity) ASC
`, tenantUUID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []orgunitStoplineShadowBindingRow
	for rows.Next() {
		var item orgunitStoplineShadowBindingRow
		if err := rows.Scan(&item.OrgCode, &item.SetID, &item.ValidFrom, &item.ValidTo); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func querySourceStoplinePositions(ctx context.Context, sourceConn *pgx.Conn, tenantUUID string, asOfDate string) ([]orgunitStoplineShadowPositionRow, error) {
	rows, err := sourceConn.Query(ctx, `
SELECT
  pv.position_uuid::text,
  c.org_code,
  COALESCE(pv.jobcatalog_setid, '') AS jobcatalog_setid,
  COALESCE(pv.name, '') AS position_name,
  lower(pv.validity)::text AS valid_from,
  COALESCE(upper(pv.validity)::text, '') AS valid_to
FROM staffing.position_versions pv
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = pv.tenant_uuid
 AND c.org_id = pv.org_unit_id
WHERE pv.tenant_uuid = $1::uuid
  AND pv.validity @> $2::date
ORDER BY lower(pv.validity) ASC, pv.position_uuid ASC
`, tenantUUID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []orgunitStoplineShadowPositionRow
	for rows.Next() {
		var item orgunitStoplineShadowPositionRow
		if err := rows.Scan(
			&item.PositionUUID,
			&item.OrgCode,
			&item.JobCatalogSetID,
			&item.Name,
			&item.ValidFrom,
			&item.ValidTo,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func firstHeavyLeaf(nodes []orgunitStoplineNodeSample, avoidCode string) orgunitStoplineNodeSample {
	for _, node := range nodes {
		if node.OrgCode == avoidCode {
			continue
		}
		if node.SubtreeSize <= 1 {
			return node
		}
	}
	return orgunitStoplineNodeSample{}
}

func firstHeavySubtree(nodes []orgunitStoplineNodeSample) orgunitStoplineNodeSample {
	for _, node := range nodes {
		if node.ParentOrgID != 0 && node.SubtreeSize > 1 {
			return node
		}
	}
	return orgunitStoplineNodeSample{}
}

func pickHeavyMoveNodes(nodes []orgunitStoplineNodeSample, rootOrgID int) (orgunitStoplineNodeSample, orgunitStoplineNodeSample) {
	subtree := orgunitStoplineNodeSample{}
	for _, node := range nodes {
		if node.ParentOrgID != 0 && node.SubtreeSize > 1 {
			subtree = node
			break
		}
	}
	if subtree.OrgID == 0 {
		return orgunitStoplineNodeSample{}, orgunitStoplineNodeSample{}
	}
	for _, candidate := range nodes {
		if candidate.ParentOrgID != rootOrgID {
			continue
		}
		if candidate.OrgID == subtree.OrgID {
			continue
		}
		if strings.HasPrefix(candidate.NodePath, subtree.NodePath+".") || strings.HasPrefix(subtree.NodePath, candidate.NodePath+".") {
			continue
		}
		return subtree, candidate
	}
	return orgunitStoplineNodeSample{}, orgunitStoplineNodeSample{}
}

func findSearchQuery(nodes []orgunitStoplineNodeSample, preferred string) string {
	preferred = strings.TrimSpace(preferred)
	if preferred == "" {
		return ""
	}
	for _, node := range nodes {
		if strings.Contains(strings.TrimSpace(node.Name), preferred) {
			return preferred
		}
	}
	return ""
}

func nextDay(asOfDate string) string {
	parsed, err := time.Parse("2006-01-02", asOfDate)
	if err != nil {
		return asOfDate
	}
	return parsed.Add(24 * time.Hour).Format("2006-01-02")
}

func normalizeExplainPayload(raw any) []byte {
	switch value := raw.(type) {
	case string:
		return []byte(value)
	case []byte:
		return value
	default:
		data, _ := json.Marshal(value)
		return data
	}
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func asFloat64(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		out, _ := typed.Float64()
		return out
	default:
		return 0
	}
}

func asInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		out, _ := typed.Int64()
		return out
	default:
		return 0
	}
}

func firstNonEmpty(items []string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func nullableDateValue(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
