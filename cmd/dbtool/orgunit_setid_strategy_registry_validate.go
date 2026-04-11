package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var orgNodeKeyPattern = regexp.MustCompile(`^[ABCDEFGHJKLMNPQRSTUVWXYZ][ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{7}$`)
var resolvedSetIDPattern = regexp.MustCompile(`^[A-Z0-9]{5}$`)

type setIDStrategyRegistrySchemaState struct {
	Columns                map[string]struct{}
	ConstraintDefs         map[string]string
	IndexDefs              map[string]string
	OrgUnitVersionsColumns map[string]struct{}
}

type setIDStrategyRegistryValidationRow struct {
	TenantUUID          string
	CapabilityKey       string
	FieldKey            string
	OrgApplicability    string
	BusinessUnitNodeKey string
	ResolvedSetID       string
	EffectiveDate       string
}

type setIDStrategyRegistryNodeKeyRef struct {
	TenantUUID string
	NodeKey    string
}

type setIDStrategyRegistryValidationIssue struct {
	Code                string
	TenantUUID          string
	CapabilityKey       string
	FieldKey            string
	OrgApplicability    string
	BusinessUnitNodeKey string
	ResolvedSetID       string
	EffectiveDate       string
	Detail              string
}

func orgunitSetIDStrategyRegistryValidate(args []string) {
	fs := flag.NewFlagSet("orgunit-setid-strategy-registry-validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var url string
	var asOf string
	fs.StringVar(&url, "url", "", "postgres connection string")
	fs.StringVar(&asOf, "as-of", "", "current-state effective day (YYYY-MM-DD)")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if url == "" {
		fatalf("missing --url")
	}
	if strings.TrimSpace(asOf) == "" {
		fatalf("missing --as-of")
	}
	asOf = strings.TrimSpace(asOf)
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		fatalf("invalid --as-of: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	schemaState, err := loadSetIDStrategyRegistrySchemaState(ctx, conn)
	if err != nil {
		fatal(err)
	}

	issues := validateSetIDStrategyRegistrySchemaState(schemaState)
	if hasCriticalSetIDStrategyRegistrySchemaIssue(issues) {
		printSetIDStrategyRegistryValidationIssues(os.Stderr, issues)
		fatalf("orgunit-setid-strategy-registry-validate: issues=%d (see stderr)", len(issues))
	}
	rows, err := loadSetIDStrategyRegistryValidationRows(ctx, conn)
	if err != nil {
		fatal(err)
	}

	currentNodeKeyCounts, err := loadCurrentOrgNodeKeyCounts(ctx, conn, asOf)
	if err != nil {
		fatal(err)
	}
	issues = append(issues, validateSetIDStrategyRegistryRows(rows, currentNodeKeyCounts)...)

	if len(issues) > 0 {
		printSetIDStrategyRegistryValidationIssues(os.Stderr, issues)
		fatalf("orgunit-setid-strategy-registry-validate: issues=%d (see stderr)", len(issues))
	}

	businessUnitRows := 0
	for _, row := range rows {
		if row.OrgApplicability == "business_unit" {
			businessUnitRows++
		}
	}
	fmt.Printf("[orgunit-setid-strategy-registry-validate] OK rows=%d business_unit_rows=%d as_of=%s\n", len(rows), businessUnitRows, asOf)
}

func loadSetIDStrategyRegistrySchemaState(ctx context.Context, conn *pgx.Conn) (setIDStrategyRegistrySchemaState, error) {
	state := setIDStrategyRegistrySchemaState{
		Columns:                make(map[string]struct{}),
		ConstraintDefs:         make(map[string]string),
		IndexDefs:              make(map[string]string),
		OrgUnitVersionsColumns: make(map[string]struct{}),
	}

	columnRows, err := conn.Query(ctx, `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = 'orgunit'
  AND table_name = 'setid_strategy_registry'
`)
	if err != nil {
		return state, err
	}
	defer columnRows.Close()
	for columnRows.Next() {
		var columnName string
		if err := columnRows.Scan(&columnName); err != nil {
			return state, err
		}
		state.Columns[strings.TrimSpace(columnName)] = struct{}{}
	}
	if err := columnRows.Err(); err != nil {
		return state, err
	}

	constraintRows, err := conn.Query(ctx, `
SELECT c.conname, pg_get_constraintdef(c.oid)
FROM pg_constraint c
JOIN pg_class t ON t.oid = c.conrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
WHERE n.nspname = 'orgunit'
  AND t.relname = 'setid_strategy_registry'
`)
	if err != nil {
		return state, err
	}
	defer constraintRows.Close()
	for constraintRows.Next() {
		var name string
		var def string
		if err := constraintRows.Scan(&name, &def); err != nil {
			return state, err
		}
		state.ConstraintDefs[strings.TrimSpace(name)] = strings.TrimSpace(def)
	}
	if err := constraintRows.Err(); err != nil {
		return state, err
	}

	indexRows, err := conn.Query(ctx, `
SELECT indexname, indexdef
FROM pg_indexes
WHERE schemaname = 'orgunit'
  AND tablename = 'setid_strategy_registry'
`)
	if err != nil {
		return state, err
	}
	defer indexRows.Close()
	for indexRows.Next() {
		var name string
		var def string
		if err := indexRows.Scan(&name, &def); err != nil {
			return state, err
		}
		state.IndexDefs[strings.TrimSpace(name)] = strings.TrimSpace(def)
	}
	if err := indexRows.Err(); err != nil {
		return state, err
	}

	orgNodeKeyColumnRows, err := conn.Query(ctx, `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = 'orgunit'
  AND table_name = 'org_unit_versions'
`)
	if err != nil {
		return state, err
	}
	defer orgNodeKeyColumnRows.Close()
	for orgNodeKeyColumnRows.Next() {
		var columnName string
		if err := orgNodeKeyColumnRows.Scan(&columnName); err != nil {
			return state, err
		}
		state.OrgUnitVersionsColumns[strings.TrimSpace(columnName)] = struct{}{}
	}
	return state, orgNodeKeyColumnRows.Err()
}

func loadSetIDStrategyRegistryValidationRows(ctx context.Context, conn *pgx.Conn) ([]setIDStrategyRegistryValidationRow, error) {
	rows, err := conn.Query(ctx, `
SELECT
  tenant_uuid::text,
  capability_key,
  field_key,
  org_applicability,
  business_unit_node_key,
  resolved_setid,
  effective_date::text
FROM orgunit.setid_strategy_registry
ORDER BY tenant_uuid, capability_key, field_key, org_applicability, resolved_setid, business_unit_node_key, effective_date
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []setIDStrategyRegistryValidationRow
	for rows.Next() {
		var row setIDStrategyRegistryValidationRow
		if err := rows.Scan(
			&row.TenantUUID,
			&row.CapabilityKey,
			&row.FieldKey,
			&row.OrgApplicability,
			&row.BusinessUnitNodeKey,
			&row.ResolvedSetID,
			&row.EffectiveDate,
		); err != nil {
			return nil, err
		}
		row.TenantUUID = strings.TrimSpace(row.TenantUUID)
		row.CapabilityKey = strings.TrimSpace(row.CapabilityKey)
		row.FieldKey = strings.TrimSpace(row.FieldKey)
		row.OrgApplicability = strings.TrimSpace(row.OrgApplicability)
		row.BusinessUnitNodeKey = strings.TrimSpace(row.BusinessUnitNodeKey)
		row.ResolvedSetID = strings.ToUpper(strings.TrimSpace(row.ResolvedSetID))
		row.EffectiveDate = strings.TrimSpace(row.EffectiveDate)
		out = append(out, row)
	}
	return out, rows.Err()
}

func loadCurrentOrgNodeKeyCounts(ctx context.Context, conn *pgx.Conn, asOf string) (map[setIDStrategyRegistryNodeKeyRef]int, error) {
	rows, err := conn.Query(ctx, `
SELECT tenant_uuid::text, org_node_key::text, count(*)::int
FROM orgunit.org_unit_versions
WHERE validity @> $1::date
GROUP BY tenant_uuid, org_node_key
`, asOf)
	if err != nil {
		return nil, fmt.Errorf("load current org_node_key mapping: %w", err)
	}
	defer rows.Close()

	out := make(map[setIDStrategyRegistryNodeKeyRef]int)
	for rows.Next() {
		var tenantUUID string
		var nodeKey string
		var count int
		if err := rows.Scan(&tenantUUID, &nodeKey, &count); err != nil {
			return nil, err
		}
		out[setIDStrategyRegistryNodeKeyRef{
			TenantUUID: strings.TrimSpace(tenantUUID),
			NodeKey:    strings.TrimSpace(nodeKey),
		}] = count
	}
	return out, rows.Err()
}

func validateSetIDStrategyRegistrySchemaState(state setIDStrategyRegistrySchemaState) []setIDStrategyRegistryValidationIssue {
	var issues []setIDStrategyRegistryValidationIssue

	if _, ok := state.Columns["business_unit_node_key"]; !ok {
		issues = append(issues, setIDStrategyRegistryValidationIssue{
			Code:   "schema_missing_business_unit_node_key",
			Detail: "orgunit.setid_strategy_registry missing column business_unit_node_key",
		})
	}
	if _, ok := state.Columns["resolved_setid"]; !ok {
		issues = append(issues, setIDStrategyRegistryValidationIssue{
			Code:   "schema_missing_resolved_setid",
			Detail: "orgunit.setid_strategy_registry missing column resolved_setid",
		})
	}
	if _, ok := state.Columns["business_unit_id"]; ok {
		issues = append(issues, setIDStrategyRegistryValidationIssue{
			Code:   "schema_old_business_unit_id_present",
			Detail: "orgunit.setid_strategy_registry still exposes legacy column business_unit_id",
		})
	}
	if _, ok := state.OrgUnitVersionsColumns["org_node_key"]; !ok {
		issues = append(issues, setIDStrategyRegistryValidationIssue{
			Code:   "target_org_node_key_schema_missing",
			Detail: "orgunit.org_unit_versions missing org_node_key; target org-node-key schema not bootstrapped",
		})
	}

	hasNodeKeyConstraint := false
	hasResolvedSetIDFormatConstraint := false
	hasResolvedSetIDShapeConstraint := false
	for name, def := range state.ConstraintDefs {
		if strings.Contains(name, "business_unit_applicability_check") && !strings.Contains(name, "node_key") {
			issues = append(issues, setIDStrategyRegistryValidationIssue{
				Code:   "schema_old_constraint_present",
				Detail: fmt.Sprintf("legacy business_unit applicability constraint still present: %s", name),
			})
		}
		if strings.Contains(def, "^[0-9]{8}$") {
			issues = append(issues, setIDStrategyRegistryValidationIssue{
				Code:   "schema_legacy_regex_present",
				Detail: fmt.Sprintf("legacy numeric regex still present in constraint %s", name),
			})
		}
		if strings.Contains(def, "business_unit_node_key") && strings.Contains(def, "orgunit.is_valid_org_node_key") {
			hasNodeKeyConstraint = true
		}
		if strings.Contains(def, "resolved_setid") && strings.Contains(def, "^[A-Z0-9]{5}$") {
			hasResolvedSetIDFormatConstraint = true
		}
		if strings.Contains(def, "resolved_setid") && strings.Contains(def, "business_unit_node_key") && strings.Contains(def, "orgunit.is_valid_org_node_key") {
			hasResolvedSetIDShapeConstraint = true
		}
	}
	if !hasNodeKeyConstraint {
		issues = append(issues, setIDStrategyRegistryValidationIssue{
			Code:   "schema_node_key_constraint_missing",
			Detail: "business_unit_node_key applicability constraint using orgunit.is_valid_org_node_key is missing",
		})
	}
	if !hasResolvedSetIDFormatConstraint {
		issues = append(issues, setIDStrategyRegistryValidationIssue{
			Code:   "schema_resolved_setid_format_constraint_missing",
			Detail: "resolved_setid format constraint is missing",
		})
	}
	if !hasResolvedSetIDShapeConstraint {
		issues = append(issues, setIDStrategyRegistryValidationIssue{
			Code:   "schema_resolved_setid_shape_constraint_missing",
			Detail: "resolved_setid scope-shape constraint is missing",
		})
	}
	if indexDef, ok := state.IndexDefs["setid_strategy_registry_key_unique_idx"]; !ok {
		issues = append(issues, setIDStrategyRegistryValidationIssue{
			Code:   "schema_resolved_setid_unique_index_missing",
			Detail: "setid_strategy_registry_key_unique_idx is missing",
		})
	} else if !strings.Contains(indexDef, "resolved_setid") {
		issues = append(issues, setIDStrategyRegistryValidationIssue{
			Code:   "schema_resolved_setid_unique_index_missing",
			Detail: "setid_strategy_registry_key_unique_idx does not include resolved_setid",
		})
	}
	return issues
}

func hasCriticalSetIDStrategyRegistrySchemaIssue(issues []setIDStrategyRegistryValidationIssue) bool {
	for _, issue := range issues {
		switch issue.Code {
		case "schema_missing_business_unit_node_key",
			"schema_missing_resolved_setid",
			"schema_resolved_setid_shape_constraint_missing",
			"schema_resolved_setid_unique_index_missing",
			"target_org_node_key_schema_missing":
			return true
		}
	}
	return false
}

func validateSetIDStrategyRegistryRows(rows []setIDStrategyRegistryValidationRow, currentNodeKeyCounts map[setIDStrategyRegistryNodeKeyRef]int) []setIDStrategyRegistryValidationIssue {
	var issues []setIDStrategyRegistryValidationIssue
	for _, row := range rows {
		nodeKey := strings.TrimSpace(row.BusinessUnitNodeKey)
		resolvedSetID := strings.ToUpper(strings.TrimSpace(row.ResolvedSetID))
		if resolvedSetID != "" && !resolvedSetIDPattern.MatchString(resolvedSetID) {
			issues = append(issues, issueForRegistryRow("resolved_setid_invalid", row, "resolved_setid must be empty wildcard or 5-char uppercase exact value"))
		}
		switch row.OrgApplicability {
		case "tenant":
			if nodeKey != "" {
				issues = append(issues, issueForRegistryRow("tenant_scope_node_key_not_empty", row, "tenant scope must keep business_unit_node_key empty"))
			}
		case "business_unit":
			if nodeKey == "" {
				issues = append(issues, issueForRegistryRow("business_unit_node_key_required", row, "business_unit scope requires business_unit_node_key"))
				continue
			}
			if resolvedSetID == "" {
				issues = append(issues, issueForRegistryRow("business_unit_resolved_setid_required", row, "business_unit scope requires exact resolved_setid"))
				continue
			}
			if !isValidOrgNodeKey(nodeKey) {
				issues = append(issues, issueForRegistryRow("business_unit_node_key_invalid", row, "business_unit_node_key is not a valid org_node_key"))
				continue
			}
			count := currentNodeKeyCounts[setIDStrategyRegistryNodeKeyRef{TenantUUID: row.TenantUUID, NodeKey: nodeKey}]
			if count == 0 {
				issues = append(issues, issueForRegistryRow("business_unit_node_key_unresolved", row, "business_unit_node_key does not resolve in current target state"))
			} else if count > 1 {
				issues = append(issues, issueForRegistryRow("business_unit_node_key_ambiguous", row, fmt.Sprintf("business_unit_node_key resolves to %d current rows", count)))
			}
		}
	}
	return issues
}

func issueForRegistryRow(code string, row setIDStrategyRegistryValidationRow, detail string) setIDStrategyRegistryValidationIssue {
	return setIDStrategyRegistryValidationIssue{
		Code:                code,
		TenantUUID:          row.TenantUUID,
		CapabilityKey:       row.CapabilityKey,
		FieldKey:            row.FieldKey,
		OrgApplicability:    row.OrgApplicability,
		BusinessUnitNodeKey: row.BusinessUnitNodeKey,
		ResolvedSetID:       row.ResolvedSetID,
		EffectiveDate:       row.EffectiveDate,
		Detail:              detail,
	}
}

func printSetIDStrategyRegistryValidationIssues(out *os.File, issues []setIDStrategyRegistryValidationIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		left := issues[i]
		right := issues[j]
		return strings.Join([]string{
			left.Code,
			left.TenantUUID,
			left.CapabilityKey,
			left.FieldKey,
			left.OrgApplicability,
			left.BusinessUnitNodeKey,
			left.ResolvedSetID,
			left.EffectiveDate,
		}, "|") < strings.Join([]string{
			right.Code,
			right.TenantUUID,
			right.CapabilityKey,
			right.FieldKey,
			right.OrgApplicability,
			right.BusinessUnitNodeKey,
			right.ResolvedSetID,
			right.EffectiveDate,
		}, "|")
	})
	for _, issue := range issues {
		fmt.Fprintf(out, "[orgunit-setid-strategy-registry-validate] issue=%s tenant=%s capability=%s field=%s applicability=%s node_key=%s resolved_setid=%s effective_date=%s detail=%s\n",
			issue.Code,
			emptyDash(issue.TenantUUID),
			emptyDash(issue.CapabilityKey),
			emptyDash(issue.FieldKey),
			emptyDash(issue.OrgApplicability),
			emptyDash(issue.BusinessUnitNodeKey),
			emptyDash(issue.ResolvedSetID),
			emptyDash(issue.EffectiveDate),
			issue.Detail,
		)
	}
}

func isValidOrgNodeKey(input string) bool {
	return orgNodeKeyPattern.MatchString(strings.TrimSpace(input))
}

func emptyDash(input string) string {
	if strings.TrimSpace(input) == "" {
		return "-"
	}
	return strings.TrimSpace(input)
}
