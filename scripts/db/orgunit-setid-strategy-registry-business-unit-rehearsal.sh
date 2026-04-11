#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage: orgunit-setid-strategy-registry-business-unit-rehearsal.sh --source-url URL --as-of YYYY-MM-DD [options]

options:
  --case NAME          pass | unresolved | ambiguous | all (default: all)
  --base-name PREFIX   database name prefix (default: orgunit_setid_registry_bu_rehearsal)
  --work-dir PATH      artifact directory (default: .local/orgunit-setid-strategy-registry-business-unit-rehearsal)

notes:
  - source-url must be an owner/bypass-RLS connection with createdb privilege.
  - script clones source-real into a dedicated rehearsal/source database and seeds 1 valid business_unit strategy row.
  - each case runs against an isolated fresh target database.
EOF
  exit 2
}

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

source_url=""
as_of=""
case_name="all"
base_name="orgunit_setid_registry_bu_rehearsal"
work_dir=".local/orgunit-setid-strategy-registry-business-unit-rehearsal"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source-url)
      source_url="${2:-}"
      shift 2
      ;;
    --as-of)
      as_of="${2:-}"
      shift 2
      ;;
    --case)
      case_name="${2:-}"
      shift 2
      ;;
    --base-name)
      base_name="${2:-}"
      shift 2
      ;;
    --work-dir)
      work_dir="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "[setid-registry-business-unit-rehearsal] unknown argument: $1" >&2
      usage
      ;;
  esac
done

if [[ -z "$source_url" || -z "$as_of" ]]; then
  usage
fi

case "$case_name" in
  pass|unresolved|ambiguous|all) ;;
  *)
    echo "[setid-registry-business-unit-rehearsal] invalid --case: $case_name" >&2
    usage
    ;;
esac

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
run_id="${timestamp,,}"
mkdir -p "$work_dir"

dbtool=(go run ./cmd/dbtool)

run_db_admin() {
  local tmp
  tmp="$(mktemp --suffix=.go)"
 cat > "$tmp" <<'EOF'
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	if len(os.Args) < 2 {
		panic("missing mode")
	}
	mode := os.Args[1]
	switch mode {
	case "pick-source":
		if len(os.Args) != 4 {
			panic("usage: pick-source <url> <as-of>")
		}
		if err := pickSource(os.Args[2], os.Args[3]); err != nil {
			panic(err)
		}
	case "clone-db":
		if len(os.Args) != 4 {
			panic("usage: clone-db <source-url> <target-db>")
		}
		targetURL, err := cloneDatabase(os.Args[2], os.Args[3])
		if err != nil {
			panic(err)
		}
		fmt.Println(targetURL)
	case "create-db":
		if len(os.Args) != 4 {
			panic("usage: create-db <source-url> <target-db>")
		}
		targetURL, err := createDatabase(os.Args[2], os.Args[3], "")
		if err != nil {
			panic(err)
		}
		fmt.Println(targetURL)
	case "seed-source":
		if len(os.Args) != 6 {
			panic("usage: seed-source <url> <as-of> <suffix> <business-unit-node-key>")
		}
		if err := seedSource(os.Args[2], os.Args[3], os.Args[4], os.Args[5]); err != nil {
			panic(err)
		}
	case "resolve-target-node-key":
		if len(os.Args) != 5 {
			panic("usage: resolve-target-node-key <url> <tenant-uuid> <org-code>")
		}
		orgNodeKey, err := resolveTargetNodeKey(os.Args[2], os.Args[3], os.Args[4])
		if err != nil {
			panic(err)
		}
		fmt.Println(orgNodeKey)
	case "mutate-target":
		if len(os.Args) != 7 {
			panic("usage: mutate-target <url> <mode> <tenant-uuid> <org-code> <as-of>")
		}
		if err := mutateTarget(os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6]); err != nil {
			panic(err)
		}
	default:
		panic("unknown mode: " + mode)
	}
}

func cloneDatabase(sourceURL string, targetDB string) (string, error) {
	sourceDB, _, err := databaseURLs(sourceURL)
	if err != nil {
		return "", err
	}
	if _, err := createDatabase(sourceURL, targetDB, sourceDB); err != nil {
		return "", err
	}
	return replaceDatabase(sourceURL, targetDB)
}

func createDatabase(sourceURL string, targetDB string, templateDB string) (string, error) {
	_, adminURL, err := databaseURLs(sourceURL)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())

	quotedTarget := pgx.Identifier{targetDB}.Sanitize()
	if _, err := conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", quotedTarget)); err != nil {
		return "", err
	}
	createSQL := fmt.Sprintf("CREATE DATABASE %s", quotedTarget)
	if strings.TrimSpace(templateDB) != "" {
		createSQL += fmt.Sprintf(" TEMPLATE %s", pgx.Identifier{templateDB}.Sanitize())
	}
	if _, err := conn.Exec(ctx, createSQL); err != nil {
		return "", err
	}
	return replaceDatabase(sourceURL, targetDB)
}

func pickSource(sourceURL string, asOf string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, sourceURL)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	orgVersionColumns, err := loadTableColumns(ctx, conn, "orgunit", "org_unit_versions")
	if err != nil {
		return err
	}
	orgCodeColumns, err := loadTableColumns(ctx, conn, "orgunit", "org_unit_codes")
	if err != nil {
		return err
	}

	sourceNodeKeyExpr := "''"
	switch {
	case hasColumn(orgVersionColumns, "org_node_key"):
		sourceNodeKeyExpr = "v.org_node_key::text"
	case hasColumn(orgCodeColumns, "org_node_key"):
		sourceNodeKeyExpr = "c.org_node_key::text"
	}

	query := fmt.Sprintf(`
SELECT
  v.tenant_uuid::text,
  c.org_code,
  c.org_id::text,
  %s AS source_org_node_key
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_id = v.org_id
WHERE v.validity @> $1::date
  AND v.is_business_unit = true
ORDER BY v.tenant_uuid, c.org_code
LIMIT 1;
`, sourceNodeKeyExpr)

	var tenantUUID string
	var orgCode string
	var orgID string
	var orgNodeKey string
	if err := conn.QueryRow(ctx, query, asOf).Scan(&tenantUUID, &orgCode, &orgID, &orgNodeKey); err != nil {
		return fmt.Errorf("select current business unit seed row: %w", err)
	}

	lines := []string{
		"TENANT_UUID=" + strings.TrimSpace(tenantUUID),
		"ORG_CODE=" + strings.TrimSpace(orgCode),
		"ORG_ID=" + strings.TrimSpace(orgID),
		"SOURCE_ORG_NODE_KEY=" + strings.TrimSpace(orgNodeKey),
	}
	fmt.Println(strings.Join(lines, "\n"))
	return nil
}

func seedSource(sourceURL string, asOf string, suffix string, businessUnitNodeKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, sourceURL)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	registryColumns, err := loadTableColumns(ctx, conn, "orgunit", "setid_strategy_registry")
	if err != nil {
		return err
	}
	orgVersionColumns, err := loadTableColumns(ctx, conn, "orgunit", "org_unit_versions")
	if err != nil {
		return err
	}
	orgCodeColumns, err := loadTableColumns(ctx, conn, "orgunit", "org_unit_codes")
	if err != nil {
		return err
	}

	if !hasColumn(registryColumns, "business_unit_id") && !hasColumn(registryColumns, "business_unit_node_key") {
		return fmt.Errorf("setid_strategy_registry missing business_unit_id/business_unit_node_key")
	}
	if !hasColumn(orgCodeColumns, "org_id") {
		return fmt.Errorf("org_unit_codes missing org_id")
	}

	sourceNodeKeyExpr := "''"
	switch {
	case hasColumn(orgVersionColumns, "org_node_key"):
		sourceNodeKeyExpr = "v.org_node_key::text"
	case hasColumn(orgCodeColumns, "org_node_key"):
		sourceNodeKeyExpr = "c.org_node_key::text"
	}

	query := fmt.Sprintf(`
SELECT
  v.tenant_uuid::text,
  c.org_code,
  c.org_id::text,
  %s AS source_org_node_key
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_id = v.org_id
WHERE v.validity @> $1::date
  AND v.is_business_unit = true
ORDER BY v.tenant_uuid, c.org_code
LIMIT 1;
`, sourceNodeKeyExpr)

	var tenantUUID string
	var orgCode string
	var orgID string
	var currentOrgNodeKey string
	if err := conn.QueryRow(ctx, query, asOf).Scan(&tenantUUID, &orgCode, &orgID, &currentOrgNodeKey); err != nil {
		return fmt.Errorf("select current business unit seed row: %w", err)
	}

	orgNodeKey := strings.TrimSpace(businessUnitNodeKey)

	capabilityKey := "orgunit.rehearsal_" + suffix + ".field_policy"
	fieldKey := "rehearsal_" + suffix
	nowUTC := time.Now().UTC().Format(time.RFC3339)

	columnName := "business_unit_id"
	columnValue := orgID
	if hasColumn(registryColumns, "business_unit_node_key") {
		columnName = "business_unit_node_key"
		columnValue = orgNodeKey
		if columnValue == "" {
			return fmt.Errorf("seed source requires predicted target org_node_key when source schema already uses business_unit_node_key")
		}
	}

	insertSQL := fmt.Sprintf(`
INSERT INTO orgunit.setid_strategy_registry (
  tenant_uuid,
  capability_key,
  owner_module,
  field_key,
  personalization_mode,
  org_applicability,
  %s,
  required,
  visible,
  maintainable,
  default_rule_ref,
  default_value,
  allowed_value_codes,
  priority,
  priority_mode,
  local_override_mode,
  explain_required,
  is_stable,
  change_policy,
  effective_date,
  end_date,
  updated_at
) VALUES (
  $1::uuid,
  $2::text,
  'orgunit',
  $3::text,
  'setid',
  'business_unit',
  $4::text,
  false,
  true,
  true,
  NULL,
  NULL,
  NULL,
  100,
  'blend_custom_first',
  'allow',
  false,
  true,
  'plan_required',
  $5::date,
  NULL,
  $6::timestamptz
);
`, columnName)

	if _, err := conn.Exec(ctx, insertSQL, tenantUUID, capabilityKey, fieldKey, columnValue, asOf, nowUTC); err != nil {
		return fmt.Errorf("seed source business_unit row: %w", err)
	}

	lines := []string{
		"TENANT_UUID=" + strings.TrimSpace(tenantUUID),
		"ORG_CODE=" + strings.TrimSpace(orgCode),
		"ORG_ID=" + strings.TrimSpace(orgID),
		"SOURCE_ORG_NODE_KEY=" + strings.TrimSpace(currentOrgNodeKey),
		"SEEDED_BUSINESS_UNIT_NODE_KEY=" + strings.TrimSpace(orgNodeKey),
		"CAPABILITY_KEY=" + capabilityKey,
		"FIELD_KEY=" + fieldKey,
	}
	fmt.Println(strings.Join(lines, "\n"))
	return nil
}

func resolveTargetNodeKey(targetURL string, tenantUUID string, orgCode string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, targetURL)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())

	var orgNodeKey string
	if err := conn.QueryRow(ctx, `
SELECT org_node_key::text
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid
  AND org_code = $2::text;
`, tenantUUID, orgCode).Scan(&orgNodeKey); err != nil {
		return "", fmt.Errorf("resolve target org_node_key by org_code: %w", err)
	}
	return strings.TrimSpace(orgNodeKey), nil
}

func mutateTarget(targetURL string, mode string, tenantUUID string, orgCode string, asOf string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, targetURL)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	var orgNodeKey string
	if err := conn.QueryRow(ctx, `
SELECT org_node_key::text
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid
  AND org_code = $2::text;
`, tenantUUID, orgCode).Scan(&orgNodeKey); err != nil {
		return fmt.Errorf("resolve target org_node_key by org_code: %w", err)
	}

	switch mode {
	case "unresolved":
		cmdTag, err := conn.Exec(ctx, `
DELETE FROM orgunit.org_unit_versions
WHERE tenant_uuid = $1::uuid
  AND org_node_key = $2::char(8)
  AND validity @> $3::date;
`, tenantUUID, orgNodeKey, asOf)
		if err != nil {
			return err
		}
		if cmdTag.RowsAffected() == 0 {
			return fmt.Errorf("unresolved mutation affected 0 rows")
		}
	case "ambiguous":
		if _, err := conn.Exec(ctx, `ALTER TABLE orgunit.org_unit_versions DROP CONSTRAINT IF EXISTS org_unit_versions_no_overlap;`); err != nil {
			return err
		}
		cmdTag, err := conn.Exec(ctx, `
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
FROM orgunit.org_unit_versions
WHERE tenant_uuid = $1::uuid
  AND org_node_key = $2::char(8)
  AND validity @> $3::date
LIMIT 1;
`, tenantUUID, orgNodeKey, asOf)
		if err != nil {
			return err
		}
		if cmdTag.RowsAffected() != 1 {
			return fmt.Errorf("ambiguous mutation affected %d rows want 1", cmdTag.RowsAffected())
		}
	default:
		return fmt.Errorf("unknown target mutation mode: %s", mode)
	}
	return nil
}

func databaseURLs(input string) (string, string, error) {
	parsed, err := url.Parse(input)
	if err != nil {
		return "", "", err
	}
	sourceDB := strings.TrimPrefix(parsed.Path, "/")
	if strings.TrimSpace(sourceDB) == "" {
		return "", "", fmt.Errorf("source url missing database name")
	}
	adminURL, err := replaceDatabase(input, "postgres")
	if err != nil {
		return "", "", err
	}
	return sourceDB, adminURL, nil
}

func replaceDatabase(input string, dbName string) (string, error) {
	parsed, err := url.Parse(input)
	if err != nil {
		return "", err
	}
	parsed.Path = "/" + dbName
	return parsed.String(), nil
}

func loadTableColumns(ctx context.Context, conn *pgx.Conn, schema string, table string) (map[string]struct{}, error) {
	rows, err := conn.Query(ctx, `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = $1
  AND table_name = $2;
`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, err
		}
		out[strings.TrimSpace(columnName)] = struct{}{}
	}
	return out, rows.Err()
}

func hasOrgunitFunction(ctx context.Context, conn *pgx.Conn, functionName string) (bool, error) {
	var exists bool
	if err := conn.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM pg_proc p
  JOIN pg_namespace n ON n.oid = p.pronamespace
  WHERE n.nspname = 'orgunit'
    AND p.proname = $1
);
`, functionName).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func hasColumn(columns map[string]struct{}, column string) bool {
	_, ok := columns[column]
	return ok
}
EOF
  go run "$tmp" "$@"
  rm -f "$tmp"
}

mask_url() {
  printf '%s\n' "$1" | sed 's#://[^:]*:[^@]*@#://***:***@#'
}

declare -a selected_cases
if [[ "$case_name" == "all" ]]; then
  selected_cases=(pass unresolved ambiguous)
else
  selected_cases=("$case_name")
fi

source_clone_db="${base_name}_${run_id}_source"
if ! source_clone_url="$(run_db_admin clone-db "$source_url" "$source_clone_db")"; then
  echo "[setid-registry-business-unit-rehearsal] clone-db failed" >&2
  exit 1
fi
echo "[setid-registry-business-unit-rehearsal] cloned source db=$(mask_url "$source_clone_url")"

if ! pick_output="$(run_db_admin pick-source "$source_clone_url" "$as_of")"; then
  echo "[setid-registry-business-unit-rehearsal] pick-source failed" >&2
  exit 1
fi
while IFS='=' read -r key value; do
  case "$key" in
    TENANT_UUID) tenant_uuid="$value" ;;
    ORG_CODE) org_code="$value" ;;
    ORG_ID) org_id="$value" ;;
    SOURCE_ORG_NODE_KEY) source_org_node_key="$value" ;;
  esac
done <<< "$pick_output"

if [[ -z "${tenant_uuid:-}" || -z "${org_code:-}" || -z "${org_id:-}" ]]; then
  echo "[setid-registry-business-unit-rehearsal] failed to parse picked source output" >&2
  exit 1
fi

org_snapshot="$work_dir/orgunit-snapshot-${run_id}.json"
registry_snapshot="$work_dir/setid-strategy-registry-${run_id}.json"

echo "[setid-registry-business-unit-rehearsal] export source org snapshot"
"${dbtool[@]}" orgunit-snapshot-export \
  --url "$source_clone_url" \
  --as-of "$as_of" \
  --output "$org_snapshot"

echo "[setid-registry-business-unit-rehearsal] check source org snapshot"
"${dbtool[@]}" orgunit-snapshot-check \
  --input "$org_snapshot"

probe_db="${base_name}_${run_id}_probe"
probe_url="$(run_db_admin create-db "$source_url" "$probe_db")"
echo "[setid-registry-business-unit-rehearsal] probe target=$(mask_url "$probe_url")"

"${dbtool[@]}" orgunit-snapshot-bootstrap-target \
  --url "$probe_url" \
  --include-setid-strategy-registry

"${dbtool[@]}" orgunit-snapshot-import \
  --url "$probe_url" \
  --input "$org_snapshot"

"${dbtool[@]}" orgunit-snapshot-verify \
  --url "$probe_url" \
  --input "$org_snapshot"

if ! predicted_target_org_node_key="$(run_db_admin resolve-target-node-key "$probe_url" "$tenant_uuid" "$org_code")"; then
  echo "[setid-registry-business-unit-rehearsal] resolve-target-node-key failed" >&2
  exit 1
fi

if ! seed_output="$(run_db_admin seed-source "$source_clone_url" "$as_of" "$run_id" "$predicted_target_org_node_key")"; then
  echo "[setid-registry-business-unit-rehearsal] seed-source failed" >&2
  exit 1
fi
while IFS='=' read -r key value; do
  case "$key" in
    TENANT_UUID) tenant_uuid="$value" ;;
    ORG_CODE) org_code="$value" ;;
    ORG_ID) org_id="$value" ;;
    SOURCE_ORG_NODE_KEY) source_org_node_key="$value" ;;
    SEEDED_BUSINESS_UNIT_NODE_KEY) seeded_business_unit_node_key="$value" ;;
    CAPABILITY_KEY) capability_key="$value" ;;
    FIELD_KEY) field_key="$value" ;;
  esac
done <<< "$seed_output"

if [[ -z "${tenant_uuid:-}" || -z "${org_code:-}" || -z "${capability_key:-}" || -z "${field_key:-}" ]]; then
  echo "[setid-registry-business-unit-rehearsal] failed to parse seed output" >&2
  exit 1
fi

echo "[setid-registry-business-unit-rehearsal] export source registry snapshot"
"${dbtool[@]}" orgunit-setid-strategy-registry-export \
  --url "$source_clone_url" \
  --as-of "$as_of" \
  --output "$registry_snapshot"

echo "[setid-registry-business-unit-rehearsal] check source registry snapshot"
"${dbtool[@]}" orgunit-setid-strategy-registry-check \
  --input "$registry_snapshot"

echo "[setid-registry-business-unit-rehearsal] seed summary tenant=$tenant_uuid org_code=$org_code org_id=$org_id source_org_node_key=${source_org_node_key:-} predicted_target_org_node_key=${predicted_target_org_node_key:-} seeded_business_unit_node_key=${seeded_business_unit_node_key:-} capability_key=$capability_key field_key=$field_key"

for current_case in "${selected_cases[@]}"; do
  target_db="${base_name}_${run_id}_${current_case}"
  target_url="$(run_db_admin create-db "$source_url" "$target_db")"
  echo "[setid-registry-business-unit-rehearsal] case=$current_case target=$(mask_url "$target_url")"

  "${dbtool[@]}" orgunit-snapshot-bootstrap-target \
    --url "$target_url" \
    --include-setid-strategy-registry

  "${dbtool[@]}" orgunit-snapshot-import \
    --url "$target_url" \
    --input "$org_snapshot"

  "${dbtool[@]}" orgunit-snapshot-verify \
    --url "$target_url" \
    --input "$org_snapshot"

  if [[ "$current_case" == "unresolved" || "$current_case" == "ambiguous" ]]; then
    echo "[setid-registry-business-unit-rehearsal] mutate target case=$current_case"
    run_db_admin mutate-target "$target_url" "$current_case" "$tenant_uuid" "$org_code" "$as_of" >/dev/null
  fi

  if [[ "$current_case" == "pass" ]]; then
    "${dbtool[@]}" orgunit-setid-strategy-registry-import \
      --url "$target_url" \
      --input "$registry_snapshot"

    "${dbtool[@]}" orgunit-setid-strategy-registry-verify \
      --url "$target_url" \
      --input "$registry_snapshot"

    "${dbtool[@]}" orgunit-setid-strategy-registry-validate \
      --url "$target_url" \
      --as-of "$as_of"

    echo "[setid-registry-business-unit-rehearsal] case=pass OK"
    continue
  fi

  set +e
  import_output="$("${dbtool[@]}" orgunit-setid-strategy-registry-import \
    --url "$target_url" \
    --input "$registry_snapshot" 2>&1)"
  import_status=$?
  set -e

  if [[ "$import_status" -eq 0 ]]; then
    echo "[setid-registry-business-unit-rehearsal] case=$current_case expected import failure but succeeded" >&2
    exit 1
  fi

  expected_fragment="count=0 want=1"
  if [[ "$current_case" == "ambiguous" ]]; then
    expected_fragment="count=2 want=1"
  fi

  if [[ "$import_output" != *"$expected_fragment"* ]]; then
    echo "[setid-registry-business-unit-rehearsal] case=$current_case unexpected failure output:" >&2
    printf '%s\n' "$import_output" >&2
    exit 1
  fi

  echo "[setid-registry-business-unit-rehearsal] case=$current_case expected stopline observed fragment=$expected_fragment"
done

echo "[setid-registry-business-unit-rehearsal] OK run_id=$run_id work_dir=$work_dir"
