package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/uuidv7"
)

const orgunitSnapshotVersion = "dev-plan-320-org-node-key-cutover-v1"

type orgunitSnapshotFile struct {
	Version    string                  `json:"version"`
	AsOfDate   string                  `json:"as_of_date"`
	ExportedAt time.Time               `json:"exported_at"`
	Tenants    []orgunitSnapshotTenant `json:"tenants"`
}

type orgunitSnapshotTenant struct {
	TenantUUID string                `json:"tenant_uuid"`
	NodeCount  int                   `json:"node_count"`
	RootCount  int                   `json:"root_count"`
	Nodes      []orgunitSnapshotNode `json:"nodes"`
}

type orgunitSnapshotNode struct {
	OrgCode        string `json:"org_code"`
	ParentOrgCode  string `json:"parent_org_code,omitempty"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	IsBusinessUnit bool   `json:"is_business_unit"`
	ManagerUUID    string `json:"manager_uuid,omitempty"`
	FullNamePath   string `json:"full_name_path,omitempty"`
}

type importedOrgunitNode struct {
	orgunitSnapshotNode
	OrgNodeKey       string
	ParentOrgNodeKey string
	NodePath         string
	PathNodeKeys     []string
}

func orgunitSnapshotExport(args []string) {
	fs := flag.NewFlagSet("orgunit-snapshot-export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var url string
	var asOfDate string
	var output string
	fs.StringVar(&url, "url", "", "postgres connection string")
	fs.StringVar(&asOfDate, "as-of", "", "snapshot as-of date (YYYY-MM-DD)")
	fs.StringVar(&output, "output", "", "output json file")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if url == "" {
		fatalf("missing --url")
	}
	if asOfDate == "" {
		fatalf("missing --as-of")
	}
	if output == "" {
		fatalf("missing --output")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(ctx, `
WITH current_versions AS (
  SELECT
    v.tenant_uuid::text AS tenant_uuid,
    c.org_code,
    COALESCE(pc.org_code, '') AS parent_org_code,
    v.name,
    v.status,
    v.is_business_unit,
    COALESCE(v.manager_uuid::text, '') AS manager_uuid,
    COALESCE(v.full_name_path, '') AS full_name_path
  FROM orgunit.org_unit_versions v
  JOIN orgunit.org_unit_codes c
    ON c.tenant_uuid = v.tenant_uuid
   AND c.org_id = v.org_id
  LEFT JOIN orgunit.org_unit_codes pc
    ON pc.tenant_uuid = v.tenant_uuid
   AND pc.org_id = v.parent_id
  WHERE v.validity @> $1::date
)
SELECT
  tenant_uuid,
  org_code,
  parent_org_code,
  name,
  status,
  is_business_unit,
  manager_uuid,
  full_name_path
FROM current_versions
ORDER BY tenant_uuid ASC, org_code ASC;
`, asOfDate)
	if err != nil {
		fatal(err)
	}
	defer rows.Close()

	byTenant := make(map[string][]orgunitSnapshotNode)
	for rows.Next() {
		var tenantUUID string
		var node orgunitSnapshotNode
		if err := rows.Scan(
			&tenantUUID,
			&node.OrgCode,
			&node.ParentOrgCode,
			&node.Name,
			&node.Status,
			&node.IsBusinessUnit,
			&node.ManagerUUID,
			&node.FullNamePath,
		); err != nil {
			fatal(err)
		}
		node.OrgCode = strings.TrimSpace(node.OrgCode)
		node.ParentOrgCode = strings.TrimSpace(node.ParentOrgCode)
		node.Name = strings.TrimSpace(node.Name)
		node.Status = strings.TrimSpace(node.Status)
		node.ManagerUUID = strings.TrimSpace(node.ManagerUUID)
		node.FullNamePath = strings.TrimSpace(node.FullNamePath)
		byTenant[tenantUUID] = append(byTenant[tenantUUID], node)
	}
	if err := rows.Err(); err != nil {
		fatal(err)
	}

	tenantIDs := make([]string, 0, len(byTenant))
	for tenantUUID := range byTenant {
		tenantIDs = append(tenantIDs, tenantUUID)
	}
	sort.Strings(tenantIDs)

	snapshot := orgunitSnapshotFile{
		Version:    orgunitSnapshotVersion,
		AsOfDate:   asOfDate,
		ExportedAt: time.Now().UTC(),
		Tenants:    make([]orgunitSnapshotTenant, 0, len(tenantIDs)),
	}
	for _, tenantUUID := range tenantIDs {
		nodes := append([]orgunitSnapshotNode(nil), byTenant[tenantUUID]...)
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].OrgCode < nodes[j].OrgCode
		})
		rootCount := 0
		for _, node := range nodes {
			if strings.TrimSpace(node.ParentOrgCode) == "" {
				rootCount++
			}
		}
		snapshot.Tenants = append(snapshot.Tenants, orgunitSnapshotTenant{
			TenantUUID: tenantUUID,
			NodeCount:  len(nodes),
			RootCount:  rootCount,
			Nodes:      nodes,
		})
	}

	if err := validateOrgunitSnapshot(snapshot); err != nil {
		fatal(err)
	}
	if err := writeOrgunitSnapshot(output, snapshot); err != nil {
		fatal(err)
	}

	fmt.Printf("[orgunit-snapshot-export] OK tenants=%d output=%s as_of=%s\n", len(snapshot.Tenants), output, snapshot.AsOfDate)
}

func orgunitSnapshotCheck(args []string) {
	fs := flag.NewFlagSet("orgunit-snapshot-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var input string
	fs.StringVar(&input, "input", "", "snapshot json file")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if input == "" {
		fatalf("missing --input")
	}

	snapshot, err := readOrgunitSnapshot(input)
	if err != nil {
		fatal(err)
	}
	if err := validateOrgunitSnapshot(snapshot); err != nil {
		fatal(err)
	}

	totalNodes := 0
	for _, tenant := range snapshot.Tenants {
		totalNodes += tenant.NodeCount
	}
	fmt.Printf("[orgunit-snapshot-check] OK tenants=%d nodes=%d as_of=%s input=%s\n", len(snapshot.Tenants), totalNodes, snapshot.AsOfDate, input)
}

func orgunitSnapshotImport(args []string) {
	fs := flag.NewFlagSet("orgunit-snapshot-import", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var url string
	var input string
	var dryRun bool
	fs.StringVar(&url, "url", "", "postgres connection string")
	fs.StringVar(&input, "input", "", "snapshot json file")
	fs.BoolVar(&dryRun, "dry-run", false, "import inside a transaction and roll back")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if url == "" {
		fatalf("missing --url")
	}
	if input == "" {
		fatalf("missing --input")
	}

	snapshot, err := readOrgunitSnapshot(input)
	if err != nil {
		fatal(err)
	}
	if err := validateOrgunitSnapshot(snapshot); err != nil {
		fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	tx, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	for _, tenant := range snapshot.Tenants {
		if err := importOrgunitSnapshotTenant(ctx, tx, snapshot.AsOfDate, tenant); err != nil {
			fatal(err)
		}
		if err := verifyOrgunitSnapshotTenant(ctx, tx, snapshot.AsOfDate, tenant); err != nil {
			fatal(err)
		}
	}

	if dryRun {
		if err := tx.Rollback(ctx); err != nil {
			fatal(err)
		}
		fmt.Printf("[orgunit-snapshot-import] DRY-RUN OK tenants=%d input=%s\n", len(snapshot.Tenants), input)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		fatal(err)
	}
	fmt.Printf("[orgunit-snapshot-import] OK tenants=%d input=%s\n", len(snapshot.Tenants), input)
}

func orgunitSnapshotVerify(args []string) {
	fs := flag.NewFlagSet("orgunit-snapshot-verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var url string
	var input string
	fs.StringVar(&url, "url", "", "postgres connection string")
	fs.StringVar(&input, "input", "", "snapshot json file")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if url == "" {
		fatalf("missing --url")
	}
	if input == "" {
		fatalf("missing --input")
	}

	snapshot, err := readOrgunitSnapshot(input)
	if err != nil {
		fatal(err)
	}
	if err := validateOrgunitSnapshot(snapshot); err != nil {
		fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	tx, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	for _, tenant := range snapshot.Tenants {
		if err := verifyOrgunitSnapshotTenant(ctx, tx, snapshot.AsOfDate, tenant); err != nil {
			fatal(err)
		}
	}
	fmt.Printf("[orgunit-snapshot-verify] OK tenants=%d input=%s\n", len(snapshot.Tenants), input)
}

func readOrgunitSnapshot(path string) (orgunitSnapshotFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return orgunitSnapshotFile{}, err
	}
	var snapshot orgunitSnapshotFile
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return orgunitSnapshotFile{}, err
	}
	return snapshot, nil
}

func writeOrgunitSnapshot(path string, snapshot orgunitSnapshotFile) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func validateOrgunitSnapshot(snapshot orgunitSnapshotFile) error {
	if strings.TrimSpace(snapshot.Version) != orgunitSnapshotVersion {
		return fmt.Errorf("snapshot version mismatch: got %q want %q", snapshot.Version, orgunitSnapshotVersion)
	}
	if strings.TrimSpace(snapshot.AsOfDate) == "" {
		return fmt.Errorf("snapshot as_of_date is required")
	}
	if len(snapshot.Tenants) == 0 {
		return fmt.Errorf("snapshot tenants is empty")
	}

	var errs []string
	seenTenants := make(map[string]struct{}, len(snapshot.Tenants))
	for _, tenant := range snapshot.Tenants {
		tenantUUID := strings.TrimSpace(tenant.TenantUUID)
		if tenantUUID == "" {
			errs = append(errs, "tenant_uuid is required")
			continue
		}
		if _, ok := seenTenants[tenantUUID]; ok {
			errs = append(errs, fmt.Sprintf("tenant %s duplicated", tenantUUID))
			continue
		}
		seenTenants[tenantUUID] = struct{}{}

		if err := validateOrgunitSnapshotTenant(tenant); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func validateOrgunitSnapshotTenant(tenant orgunitSnapshotTenant) error {
	if len(tenant.Nodes) == 0 {
		return fmt.Errorf("tenant %s has empty nodes", tenant.TenantUUID)
	}
	if tenant.NodeCount != len(tenant.Nodes) {
		return fmt.Errorf("tenant %s node_count mismatch: got %d want %d", tenant.TenantUUID, tenant.NodeCount, len(tenant.Nodes))
	}

	byCode := make(map[string]orgunitSnapshotNode, len(tenant.Nodes))
	rootCount := 0
	for _, node := range tenant.Nodes {
		orgCode := strings.TrimSpace(node.OrgCode)
		if orgCode == "" {
			return fmt.Errorf("tenant %s has empty org_code", tenant.TenantUUID)
		}
		if _, exists := byCode[orgCode]; exists {
			return fmt.Errorf("tenant %s has duplicate org_code: %s", tenant.TenantUUID, orgCode)
		}
		if strings.TrimSpace(node.Name) == "" {
			return fmt.Errorf("tenant %s node %s has empty name", tenant.TenantUUID, orgCode)
		}
		status := normalizeOrgunitSnapshotStatus(node.Status)
		if status == "" {
			return fmt.Errorf("tenant %s node %s has invalid status %q", tenant.TenantUUID, orgCode, node.Status)
		}
		if strings.TrimSpace(node.ParentOrgCode) == "" {
			rootCount++
		}
		normalized := node
		normalized.OrgCode = orgCode
		normalized.ParentOrgCode = strings.TrimSpace(node.ParentOrgCode)
		normalized.Name = strings.TrimSpace(node.Name)
		normalized.Status = status
		normalized.ManagerUUID = strings.TrimSpace(node.ManagerUUID)
		normalized.FullNamePath = strings.TrimSpace(node.FullNamePath)
		byCode[orgCode] = normalized
	}
	if tenant.RootCount != rootCount {
		return fmt.Errorf("tenant %s root_count mismatch: got %d want %d", tenant.TenantUUID, tenant.RootCount, rootCount)
	}
	if rootCount != 1 {
		return fmt.Errorf("tenant %s root_count must be exactly 1, got %d", tenant.TenantUUID, rootCount)
	}

	for _, node := range byCode {
		parentCode := strings.TrimSpace(node.ParentOrgCode)
		if parentCode == "" {
			continue
		}
		if parentCode == node.OrgCode {
			return fmt.Errorf("tenant %s node %s cannot parent itself", tenant.TenantUUID, node.OrgCode)
		}
		if _, ok := byCode[parentCode]; !ok {
			return fmt.Errorf("tenant %s node %s references missing parent %s", tenant.TenantUUID, node.OrgCode, parentCode)
		}
	}

	if _, err := orderOrgunitSnapshotNodes(tenant.Nodes); err != nil {
		return fmt.Errorf("tenant %s invalid tree: %w", tenant.TenantUUID, err)
	}
	return nil
}

func normalizeOrgunitSnapshotStatus(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "active":
		return "active"
	case "disabled", "inactive":
		return "disabled"
	default:
		return ""
	}
}

func orderOrgunitSnapshotNodes(nodes []orgunitSnapshotNode) ([]orgunitSnapshotNode, error) {
	byCode := make(map[string]orgunitSnapshotNode, len(nodes))
	children := make(map[string][]string, len(nodes))
	inDegree := make(map[string]int, len(nodes))
	for _, node := range nodes {
		orgCode := strings.TrimSpace(node.OrgCode)
		normalized := node
		normalized.OrgCode = orgCode
		normalized.ParentOrgCode = strings.TrimSpace(node.ParentOrgCode)
		normalized.Name = strings.TrimSpace(node.Name)
		normalized.Status = normalizeOrgunitSnapshotStatus(node.Status)
		normalized.ManagerUUID = strings.TrimSpace(node.ManagerUUID)
		normalized.FullNamePath = strings.TrimSpace(node.FullNamePath)
		byCode[orgCode] = normalized
		inDegree[orgCode] = 0
	}
	for _, node := range byCode {
		parentCode := node.ParentOrgCode
		if parentCode == "" {
			continue
		}
		children[parentCode] = append(children[parentCode], node.OrgCode)
		inDegree[node.OrgCode]++
	}
	for parentCode := range children {
		sort.Strings(children[parentCode])
	}

	queue := make([]string, 0)
	for orgCode, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, orgCode)
		}
	}
	sort.Strings(queue)

	ordered := make([]orgunitSnapshotNode, 0, len(nodes))
	for len(queue) > 0 {
		orgCode := queue[0]
		queue = queue[1:]
		ordered = append(ordered, byCode[orgCode])
		for _, childCode := range children[orgCode] {
			inDegree[childCode]--
			if inDegree[childCode] == 0 {
				queue = append(queue, childCode)
				sort.Strings(queue)
			}
		}
	}
	if len(ordered) != len(nodes) {
		return nil, fmt.Errorf("cycle detected")
	}
	return ordered, nil
}

func importOrgunitSnapshotTenant(ctx context.Context, tx pgx.Tx, asOfDate string, tenant orgunitSnapshotTenant) error {
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenant.TenantUUID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM orgunit.org_unit_versions WHERE tenant_uuid = $1::uuid;`, tenant.TenantUUID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM orgunit.org_unit_codes WHERE tenant_uuid = $1::uuid;`, tenant.TenantUUID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM orgunit.org_events WHERE tenant_uuid = $1::uuid;`, tenant.TenantUUID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM orgunit.org_trees WHERE tenant_uuid = $1::uuid;`, tenant.TenantUUID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM orgunit.org_node_key_registry WHERE tenant_uuid = $1::uuid;`, tenant.TenantUUID); err != nil {
		return err
	}

	ordered, err := orderOrgunitSnapshotNodes(tenant.Nodes)
	if err != nil {
		return err
	}

	importedByCode := make(map[string]importedOrgunitNode, len(ordered))
	for _, node := range ordered {
		var orgNodeKey string
		if err := tx.QueryRow(ctx, `SELECT orgunit.allocate_org_node_key($1::uuid)::text;`, tenant.TenantUUID).Scan(&orgNodeKey); err != nil {
			return err
		}

		parentCode := strings.TrimSpace(node.ParentOrgCode)
		parentKey := ""
		parentPath := ""
		parentFullName := ""
		if parentCode != "" {
			parent, ok := importedByCode[parentCode]
			if !ok {
				return fmt.Errorf("tenant %s node %s missing imported parent %s", tenant.TenantUUID, node.OrgCode, parentCode)
			}
			parentKey = parent.OrgNodeKey
			parentPath = parent.NodePath
			parentFullName = parent.FullNamePath
		}

		nodePath := orgNodeKey
		fullNamePath := strings.TrimSpace(node.Name)
		if parentPath != "" {
			nodePath = parentPath + "." + orgNodeKey
		}
		if parentFullName != "" {
			fullNamePath = parentFullName + " / " + strings.TrimSpace(node.Name)
		}

		afterSnapshot := map[string]any{
			"org_node_key":        orgNodeKey,
			"name":                strings.TrimSpace(node.Name),
			"status":              normalizeOrgunitSnapshotStatus(node.Status),
			"parent_org_node_key": nullableString(parentKey),
			"node_path":           nodePath,
			"validity":            fmt.Sprintf("[%s,)", asOfDate),
			"full_name_path":      fullNamePath,
			"is_business_unit":    node.IsBusinessUnit,
		}
		if strings.TrimSpace(node.ManagerUUID) != "" {
			afterSnapshot["manager_uuid"] = strings.TrimSpace(node.ManagerUUID)
		}

		payload := map[string]any{
			"name":             strings.TrimSpace(node.Name),
			"status":           normalizeOrgunitSnapshotStatus(node.Status),
			"is_business_unit": node.IsBusinessUnit,
			"cutover_source":   "DEV-PLAN-320",
		}
		if parentKey != "" {
			payload["parent_org_node_key"] = parentKey
		}
		if strings.TrimSpace(node.ManagerUUID) != "" {
			payload["manager_uuid"] = strings.TrimSpace(node.ManagerUUID)
		}

		eventUUID, err := uuidv7.NewString()
		if err != nil {
			return err
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		afterSnapshotJSON, err := json.Marshal(afterSnapshot)
		if err != nil {
			return err
		}
		requestID := fmt.Sprintf("dbtool-orgunit-snapshot-import:%s:%s", tenant.TenantUUID, node.OrgCode)

		var eventID int64
		if err := tx.QueryRow(ctx, `
INSERT INTO orgunit.org_events (
  event_uuid,
  tenant_uuid,
  org_node_key,
  event_type,
  effective_date,
  payload,
  request_id,
  initiator_uuid,
  before_snapshot,
  after_snapshot
)
VALUES (
  $1::uuid,
  $2::uuid,
  $3::char(8),
  'CREATE',
  $4::date,
  $5::jsonb,
  $6::text,
  $2::uuid,
  NULL,
  $7::jsonb
)
RETURNING id;
`, eventUUID, tenant.TenantUUID, orgNodeKey, asOfDate, payloadJSON, requestID, afterSnapshotJSON).Scan(&eventID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
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
VALUES (
  $1::uuid,
  $2::char(8),
  NULLIF($3::text, '')::char(8),
  $4::ltree,
  daterange($5::date, NULL, '[)'),
  $6::text,
  $7::text,
  $8::text,
  $9::boolean,
  NULLIF($10::text, '')::uuid,
  $11::bigint
);
`, tenant.TenantUUID, orgNodeKey, parentKey, nodePath, asOfDate, strings.TrimSpace(node.Name), fullNamePath, normalizeOrgunitSnapshotStatus(node.Status), node.IsBusinessUnit, strings.TrimSpace(node.ManagerUUID), eventID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
INSERT INTO orgunit.org_unit_codes (
  tenant_uuid,
  org_node_key,
  org_code
)
VALUES ($1::uuid, $2::char(8), $3::text);
`, tenant.TenantUUID, orgNodeKey, strings.TrimSpace(node.OrgCode)); err != nil {
			return err
		}

		importedByCode[node.OrgCode] = importedOrgunitNode{
			orgunitSnapshotNode: node,
			OrgNodeKey:          orgNodeKey,
			ParentOrgNodeKey:    parentKey,
			NodePath:            nodePath,
			PathNodeKeys:        splitNodePath(nodePath),
		}
	}

	rootCode := ""
	for _, node := range ordered {
		if strings.TrimSpace(node.ParentOrgCode) == "" {
			rootCode = node.OrgCode
			break
		}
	}
	root, ok := importedByCode[rootCode]
	if !ok {
		return fmt.Errorf("tenant %s root node not imported", tenant.TenantUUID)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO orgunit.org_trees (tenant_uuid, root_org_node_key)
VALUES ($1::uuid, $2::char(8));
`, tenant.TenantUUID, root.OrgNodeKey); err != nil {
		return err
	}
	return nil
}

func verifyOrgunitSnapshotTenant(ctx context.Context, tx pgx.Tx, asOfDate string, tenant orgunitSnapshotTenant) error {
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenant.TenantUUID); err != nil {
		return err
	}

	type dbNode struct {
		OrgNodeKey       string
		OrgCode          string
		ParentOrgNodeKey string
		ParentOrgCode    string
		Name             string
		Status           string
		IsBusinessUnit   bool
		ManagerUUID      string
		FullNamePath     string
		NodePath         string
		PathNodeKeys     []string
	}

	rows, err := tx.Query(ctx, `
SELECT
  v.org_node_key::text,
  c.org_code,
  COALESCE(v.parent_org_node_key::text, ''),
  COALESCE(pc.org_code, ''),
  v.name,
  v.status,
  v.is_business_unit,
  COALESCE(v.manager_uuid::text, ''),
  COALESCE(v.full_name_path, ''),
  v.node_path::text,
  COALESCE(v.path_node_keys, ARRAY[]::text[])
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_node_key = v.org_node_key
LEFT JOIN orgunit.org_unit_codes pc
  ON pc.tenant_uuid = v.tenant_uuid
 AND pc.org_node_key = v.parent_org_node_key
WHERE v.tenant_uuid = $1::uuid
  AND v.validity @> $2::date
ORDER BY c.org_code ASC;
`, tenant.TenantUUID, asOfDate)
	if err != nil {
		return err
	}
	defer rows.Close()

	dbNodes := make(map[string]dbNode, tenant.NodeCount)
	for rows.Next() {
		var node dbNode
		if err := rows.Scan(
			&node.OrgNodeKey,
			&node.OrgCode,
			&node.ParentOrgNodeKey,
			&node.ParentOrgCode,
			&node.Name,
			&node.Status,
			&node.IsBusinessUnit,
			&node.ManagerUUID,
			&node.FullNamePath,
			&node.NodePath,
			&node.PathNodeKeys,
		); err != nil {
			return err
		}
		dbNodes[node.OrgCode] = node
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(dbNodes) != tenant.NodeCount {
		return fmt.Errorf("tenant %s imported node_count mismatch: got %d want %d", tenant.TenantUUID, len(dbNodes), tenant.NodeCount)
	}

	rootCodes := make([]string, 0, 1)
	for _, node := range tenant.Nodes {
		dbNode, ok := dbNodes[node.OrgCode]
		if !ok {
			return fmt.Errorf("tenant %s missing imported org_code %s", tenant.TenantUUID, node.OrgCode)
		}
		if strings.TrimSpace(dbNode.ParentOrgCode) != strings.TrimSpace(node.ParentOrgCode) {
			return fmt.Errorf("tenant %s org_code %s parent mismatch: got %q want %q", tenant.TenantUUID, node.OrgCode, dbNode.ParentOrgCode, node.ParentOrgCode)
		}
		if strings.TrimSpace(dbNode.Name) != strings.TrimSpace(node.Name) {
			return fmt.Errorf("tenant %s org_code %s name mismatch: got %q want %q", tenant.TenantUUID, node.OrgCode, dbNode.Name, node.Name)
		}
		if normalizeOrgunitSnapshotStatus(dbNode.Status) != normalizeOrgunitSnapshotStatus(node.Status) {
			return fmt.Errorf("tenant %s org_code %s status mismatch: got %q want %q", tenant.TenantUUID, node.OrgCode, dbNode.Status, node.Status)
		}
		if dbNode.IsBusinessUnit != node.IsBusinessUnit {
			return fmt.Errorf("tenant %s org_code %s is_business_unit mismatch", tenant.TenantUUID, node.OrgCode)
		}
		if strings.TrimSpace(dbNode.ManagerUUID) != strings.TrimSpace(node.ManagerUUID) {
			return fmt.Errorf("tenant %s org_code %s manager_uuid mismatch: got %q want %q", tenant.TenantUUID, node.OrgCode, dbNode.ManagerUUID, node.ManagerUUID)
		}
		if strings.TrimSpace(node.FullNamePath) != "" && strings.TrimSpace(dbNode.FullNamePath) != strings.TrimSpace(node.FullNamePath) {
			return fmt.Errorf("tenant %s org_code %s full_name_path mismatch: got %q want %q", tenant.TenantUUID, node.OrgCode, dbNode.FullNamePath, node.FullNamePath)
		}

		expectedPathKeys := splitNodePath(dbNode.NodePath)
		if !equalStringSlices(dbNode.PathNodeKeys, expectedPathKeys) {
			return fmt.Errorf("tenant %s org_code %s path_node_keys mismatch: got %v want %v", tenant.TenantUUID, node.OrgCode, dbNode.PathNodeKeys, expectedPathKeys)
		}
		if dbNode.ParentOrgCode == "" {
			rootCodes = append(rootCodes, dbNode.OrgCode)
		}
	}
	if len(rootCodes) != 1 {
		return fmt.Errorf("tenant %s imported root count mismatch: got %d want 1", tenant.TenantUUID, len(rootCodes))
	}

	var rootOrgCode string
	if err := tx.QueryRow(ctx, `
SELECT c.org_code
FROM orgunit.org_trees t
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = t.tenant_uuid
 AND c.org_node_key = t.root_org_node_key
WHERE t.tenant_uuid = $1::uuid;
`, tenant.TenantUUID).Scan(&rootOrgCode); err != nil {
		return err
	}
	if rootOrgCode != rootCodes[0] {
		return fmt.Errorf("tenant %s root_org_code mismatch: got %q want %q", tenant.TenantUUID, rootOrgCode, rootCodes[0])
	}
	return nil
}

func splitNodePath(nodePath string) []string {
	trimmed := strings.TrimSpace(nodePath)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func equalStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func nullableString(input string) any {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	return input
}
