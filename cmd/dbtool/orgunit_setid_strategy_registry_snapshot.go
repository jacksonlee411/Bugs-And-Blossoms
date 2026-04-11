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
)

const setIDStrategyRegistrySnapshotVersion = "dev-plan-320-setid-strategy-registry-v1"

type setIDStrategyRegistrySnapshotFile struct {
	Version    string                             `json:"version"`
	AsOfDate   string                             `json:"as_of_date"`
	ExportedAt time.Time                          `json:"exported_at"`
	RowCount   int                                `json:"row_count"`
	Rows       []setIDStrategyRegistrySnapshotRow `json:"rows"`
}

type setIDStrategyRegistrySnapshotRow struct {
	TenantUUID              string   `json:"tenant_uuid"`
	CapabilityKey           string   `json:"capability_key"`
	OwnerModule             string   `json:"owner_module"`
	FieldKey                string   `json:"field_key"`
	PersonalizationMode     string   `json:"personalization_mode"`
	OrgApplicability        string   `json:"org_applicability"`
	BusinessUnitSourceValue string   `json:"business_unit_source_value,omitempty"`
	BusinessUnitOrgCode     string   `json:"business_unit_org_code,omitempty"`
	BusinessUnitNodeKey     string   `json:"business_unit_node_key,omitempty"`
	Required                bool     `json:"required"`
	Visible                 bool     `json:"visible"`
	Maintainable            bool     `json:"maintainable"`
	DefaultRuleRef          string   `json:"default_rule_ref,omitempty"`
	DefaultValue            string   `json:"default_value,omitempty"`
	AllowedValueCodes       []string `json:"allowed_value_codes,omitempty"`
	Priority                int      `json:"priority"`
	PriorityMode            string   `json:"priority_mode"`
	LocalOverrideMode       string   `json:"local_override_mode"`
	ExplainRequired         bool     `json:"explain_required"`
	IsStable                bool     `json:"is_stable"`
	ChangePolicy            string   `json:"change_policy"`
	EffectiveDate           string   `json:"effective_date"`
	EndDate                 string   `json:"end_date,omitempty"`
	UpdatedAt               string   `json:"updated_at"`
}

type setIDStrategyRegistryExportLayout struct {
	RegistrySourceColumn string
	HasOrgCodeOrgID      bool
	HasOrgCodeNodeKey    bool
	HasDecodeFunction    bool
}

type setIDStrategyRegistryTargetLayout struct {
	SchemaState         setIDStrategyRegistrySchemaState
	OrgUnitCodesColumns map[string]struct{}
}

func orgunitSetIDStrategyRegistryExport(args []string) {
	fs := flag.NewFlagSet("orgunit-setid-strategy-registry-export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var url string
	var asOf string
	var output string
	fs.StringVar(&url, "url", "", "postgres connection string")
	fs.StringVar(&asOf, "as-of", "", "snapshot as-of date (YYYY-MM-DD)")
	fs.StringVar(&output, "output", "", "output json file")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if url == "" {
		fatalf("missing --url")
	}
	if strings.TrimSpace(asOf) == "" {
		fatalf("missing --as-of")
	}
	if output == "" {
		fatalf("missing --output")
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

	rows, err := exportSetIDStrategyRegistrySnapshotRows(ctx, conn, asOf)
	if err != nil {
		fatal(err)
	}
	snapshot := setIDStrategyRegistrySnapshotFile{
		Version:    setIDStrategyRegistrySnapshotVersion,
		AsOfDate:   asOf,
		ExportedAt: time.Now().UTC(),
		RowCount:   len(rows),
		Rows:       rows,
	}
	if err := validateSetIDStrategyRegistrySnapshot(snapshot); err != nil {
		fatal(err)
	}
	if err := writeSetIDStrategyRegistrySnapshot(output, snapshot); err != nil {
		fatal(err)
	}
	fmt.Printf("[orgunit-setid-strategy-registry-export] OK rows=%d business_unit_rows=%d output=%s as_of=%s\n",
		len(snapshot.Rows),
		countSetIDStrategyRegistryBusinessUnitRows(snapshot.Rows),
		output,
		snapshot.AsOfDate,
	)
}

func orgunitSetIDStrategyRegistryCheck(args []string) {
	fs := flag.NewFlagSet("orgunit-setid-strategy-registry-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var input string
	fs.StringVar(&input, "input", "", "snapshot json file")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if input == "" {
		fatalf("missing --input")
	}

	snapshot, err := readSetIDStrategyRegistrySnapshot(input)
	if err != nil {
		fatal(err)
	}
	if err := validateSetIDStrategyRegistrySnapshot(snapshot); err != nil {
		fatal(err)
	}
	fmt.Printf("[orgunit-setid-strategy-registry-check] OK rows=%d business_unit_rows=%d input=%s as_of=%s\n",
		len(snapshot.Rows),
		countSetIDStrategyRegistryBusinessUnitRows(snapshot.Rows),
		input,
		snapshot.AsOfDate,
	)
}

func orgunitSetIDStrategyRegistryImport(args []string) {
	fs := flag.NewFlagSet("orgunit-setid-strategy-registry-import", flag.ContinueOnError)
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

	snapshot, err := readSetIDStrategyRegistrySnapshot(input)
	if err != nil {
		fatal(err)
	}
	if err := validateSetIDStrategyRegistrySnapshot(snapshot); err != nil {
		fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	targetLayout, err := loadSetIDStrategyRegistryTargetLayout(ctx, conn)
	if err != nil {
		fatal(err)
	}
	if err := validateSetIDStrategyRegistryTargetLayout(targetLayout); err != nil {
		fatal(err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	for _, tenantUUID := range setIDStrategyRegistrySnapshotTenantIDs(snapshot) {
		rows := setIDStrategyRegistrySnapshotRowsForTenant(snapshot.Rows, tenantUUID)
		if err := importSetIDStrategyRegistrySnapshotTenant(ctx, tx, snapshot.AsOfDate, tenantUUID, rows); err != nil {
			fatal(err)
		}
		if err := verifySetIDStrategyRegistrySnapshotTenant(ctx, tx, snapshot.AsOfDate, tenantUUID, rows); err != nil {
			fatal(err)
		}
	}

	if dryRun {
		if err := tx.Rollback(ctx); err != nil {
			fatal(err)
		}
		fmt.Printf("[orgunit-setid-strategy-registry-import] DRY-RUN OK tenants=%d rows=%d input=%s\n",
			len(setIDStrategyRegistrySnapshotTenantIDs(snapshot)),
			len(snapshot.Rows),
			input,
		)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		fatal(err)
	}
	fmt.Printf("[orgunit-setid-strategy-registry-import] OK tenants=%d rows=%d input=%s\n",
		len(setIDStrategyRegistrySnapshotTenantIDs(snapshot)),
		len(snapshot.Rows),
		input,
	)
}

func orgunitSetIDStrategyRegistryVerify(args []string) {
	fs := flag.NewFlagSet("orgunit-setid-strategy-registry-verify", flag.ContinueOnError)
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

	snapshot, err := readSetIDStrategyRegistrySnapshot(input)
	if err != nil {
		fatal(err)
	}
	if err := validateSetIDStrategyRegistrySnapshot(snapshot); err != nil {
		fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	targetLayout, err := loadSetIDStrategyRegistryTargetLayout(ctx, conn)
	if err != nil {
		fatal(err)
	}
	if err := validateSetIDStrategyRegistryTargetLayout(targetLayout); err != nil {
		fatal(err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	for _, tenantUUID := range setIDStrategyRegistrySnapshotTenantIDs(snapshot) {
		rows := setIDStrategyRegistrySnapshotRowsForTenant(snapshot.Rows, tenantUUID)
		if err := verifySetIDStrategyRegistrySnapshotTenant(ctx, tx, snapshot.AsOfDate, tenantUUID, rows); err != nil {
			fatal(err)
		}
	}

	fmt.Printf("[orgunit-setid-strategy-registry-verify] OK tenants=%d rows=%d input=%s\n",
		len(setIDStrategyRegistrySnapshotTenantIDs(snapshot)),
		len(snapshot.Rows),
		input,
	)
}

func exportSetIDStrategyRegistrySnapshotRows(ctx context.Context, conn *pgx.Conn, asOf string) ([]setIDStrategyRegistrySnapshotRow, error) {
	layout, err := detectSetIDStrategyRegistryExportLayout(ctx, conn)
	if err != nil {
		return nil, err
	}

	sourceExpr := "r." + layout.RegistrySourceColumn
	nodeKeyExpr := "''"
	switch {
	case layout.RegistrySourceColumn == "business_unit_node_key":
		nodeKeyExpr = fmt.Sprintf("CASE WHEN r.org_applicability = 'business_unit' THEN btrim(%s) ELSE '' END", sourceExpr)
	case layout.HasOrgCodeNodeKey:
		nodeKeyExpr = fmt.Sprintf("CASE WHEN r.org_applicability = 'business_unit' AND orgunit.is_valid_org_node_key(btrim(%s)) THEN btrim(%s) ELSE COALESCE(c.org_node_key::text, '') END", sourceExpr, sourceExpr)
	default:
		nodeKeyExpr = fmt.Sprintf("CASE WHEN r.org_applicability = 'business_unit' AND orgunit.is_valid_org_node_key(btrim(%s)) THEN btrim(%s) ELSE '' END", sourceExpr, sourceExpr)
	}

	joinPredicates := []string{"c.tenant_uuid = r.tenant_uuid"}
	if layout.HasOrgCodeNodeKey {
		joinPredicates = append(joinPredicates, fmt.Sprintf("(orgunit.is_valid_org_node_key(btrim(%s)) AND c.org_node_key = btrim(%s)::char(8))", sourceExpr, sourceExpr))
	}
	if layout.HasOrgCodeOrgID {
		numericMatch := fmt.Sprintf("(%s ~ '^[0-9]+$' AND c.org_id = %s::int)", sourceExpr, sourceExpr)
		joinPredicates = append(joinPredicates, numericMatch)
		if layout.HasDecodeFunction {
			decodedMatch := fmt.Sprintf("(orgunit.is_valid_org_node_key(btrim(%s)) AND c.org_id = orgunit.decode_org_node_key(btrim(%s)::char(8))::int)", sourceExpr, sourceExpr)
			joinPredicates = append(joinPredicates, decodedMatch)
		}
	}
	if len(joinPredicates) <= 1 {
		return nil, fmt.Errorf("setid strategy registry export unsupported: orgunit.org_unit_codes missing compatible key columns")
	}
	joinClause := "LEFT JOIN orgunit.org_unit_codes c ON " + joinPredicates[0] + " AND (" + strings.Join(joinPredicates[1:], " OR ") + ")"

	query := fmt.Sprintf(`
SELECT
  r.tenant_uuid::text,
  r.capability_key,
  r.owner_module,
  r.field_key,
  r.personalization_mode,
  r.org_applicability,
  CASE WHEN r.org_applicability = 'business_unit' THEN btrim(%s) ELSE '' END AS business_unit_source_value,
  CASE WHEN r.org_applicability = 'business_unit' THEN COALESCE(c.org_code, '') ELSE '' END AS business_unit_org_code,
  %s AS business_unit_node_key,
  r.required,
  r.visible,
  r.maintainable,
  COALESCE(r.default_rule_ref, ''),
  COALESCE(r.default_value, ''),
  COALESCE(r.allowed_value_codes, '[]'::jsonb)::text,
  r.priority,
  r.priority_mode,
  r.local_override_mode,
  r.explain_required,
  r.is_stable,
  r.change_policy,
  r.effective_date::text,
  COALESCE(r.end_date::text, ''),
  to_char(r.updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM orgunit.setid_strategy_registry r
%s
WHERE r.effective_date <= $1::date
  AND (r.end_date IS NULL OR r.end_date > $1::date)
ORDER BY r.tenant_uuid, r.capability_key, r.field_key, r.org_applicability, business_unit_org_code, business_unit_node_key, r.effective_date;
`, sourceExpr, nodeKeyExpr, joinClause)

	rows, err := conn.Query(ctx, query, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]setIDStrategyRegistrySnapshotRow, 0)
	for rows.Next() {
		var row setIDStrategyRegistrySnapshotRow
		var allowedValueCodesRaw string
		if err := rows.Scan(
			&row.TenantUUID,
			&row.CapabilityKey,
			&row.OwnerModule,
			&row.FieldKey,
			&row.PersonalizationMode,
			&row.OrgApplicability,
			&row.BusinessUnitSourceValue,
			&row.BusinessUnitOrgCode,
			&row.BusinessUnitNodeKey,
			&row.Required,
			&row.Visible,
			&row.Maintainable,
			&row.DefaultRuleRef,
			&row.DefaultValue,
			&allowedValueCodesRaw,
			&row.Priority,
			&row.PriorityMode,
			&row.LocalOverrideMode,
			&row.ExplainRequired,
			&row.IsStable,
			&row.ChangePolicy,
			&row.EffectiveDate,
			&row.EndDate,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if strings.TrimSpace(allowedValueCodesRaw) != "" {
			if err := json.Unmarshal([]byte(allowedValueCodesRaw), &row.AllowedValueCodes); err != nil {
				return nil, err
			}
		}
		out = append(out, normalizeSetIDStrategyRegistrySnapshotRow(row))
	}
	return out, rows.Err()
}

func detectSetIDStrategyRegistryExportLayout(ctx context.Context, conn *pgx.Conn) (setIDStrategyRegistryExportLayout, error) {
	registryColumns, err := loadTableColumns(ctx, conn, "orgunit", "setid_strategy_registry")
	if err != nil {
		return setIDStrategyRegistryExportLayout{}, err
	}
	orgCodeColumns, err := loadTableColumns(ctx, conn, "orgunit", "org_unit_codes")
	if err != nil {
		return setIDStrategyRegistryExportLayout{}, err
	}
	hasDecodeFunction, err := hasOrgunitFunction(ctx, conn, "decode_org_node_key")
	if err != nil {
		return setIDStrategyRegistryExportLayout{}, err
	}

	layout := setIDStrategyRegistryExportLayout{
		HasOrgCodeOrgID:   hasColumn(orgCodeColumns, "org_id"),
		HasOrgCodeNodeKey: hasColumn(orgCodeColumns, "org_node_key"),
		HasDecodeFunction: hasDecodeFunction,
	}
	switch {
	case hasColumn(registryColumns, "business_unit_node_key"):
		layout.RegistrySourceColumn = "business_unit_node_key"
	case hasColumn(registryColumns, "business_unit_id"):
		layout.RegistrySourceColumn = "business_unit_id"
	default:
		return layout, fmt.Errorf("setid strategy registry export unsupported: missing business_unit_node_key/business_unit_id")
	}
	return layout, nil
}

func loadSetIDStrategyRegistryTargetLayout(ctx context.Context, conn *pgx.Conn) (setIDStrategyRegistryTargetLayout, error) {
	schemaState, err := loadSetIDStrategyRegistrySchemaState(ctx, conn)
	if err != nil {
		return setIDStrategyRegistryTargetLayout{}, err
	}
	orgUnitCodesColumns, err := loadTableColumns(ctx, conn, "orgunit", "org_unit_codes")
	if err != nil {
		return setIDStrategyRegistryTargetLayout{}, err
	}
	return setIDStrategyRegistryTargetLayout{
		SchemaState:         schemaState,
		OrgUnitCodesColumns: orgUnitCodesColumns,
	}, nil
}

func validateSetIDStrategyRegistryTargetLayout(layout setIDStrategyRegistryTargetLayout) error {
	issues := validateSetIDStrategyRegistrySchemaState(layout.SchemaState)
	if len(issues) > 0 {
		var critical []string
		for _, issue := range issues {
			if isCriticalSetIDStrategyRegistryTargetLayoutIssue(issue.Code) {
				critical = append(critical, issue.Code)
			}
		}
		if len(critical) > 0 {
			return fmt.Errorf("setid strategy registry target layout invalid: %s", strings.Join(critical, ", "))
		}
	}
	if !hasColumn(layout.OrgUnitCodesColumns, "org_node_key") {
		return fmt.Errorf("setid strategy registry target layout invalid: orgunit.org_unit_codes missing org_node_key")
	}
	return nil
}

func isCriticalSetIDStrategyRegistryTargetLayoutIssue(code string) bool {
	switch code {
	case "schema_missing_business_unit_node_key",
		"schema_old_business_unit_id_present",
		"target_org_node_key_schema_missing",
		"schema_old_constraint_present",
		"schema_legacy_regex_present",
		"schema_node_key_constraint_missing":
		return true
	default:
		return false
	}
}

func importSetIDStrategyRegistrySnapshotTenant(ctx context.Context, tx pgx.Tx, asOf string, tenantUUID string, rows []setIDStrategyRegistrySnapshotRow) error {
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantUUID); err != nil {
		return err
	}
	if err := ensureSetIDStrategyRegistryTargetTenantEmpty(ctx, tx, tenantUUID); err != nil {
		return err
	}
	resolvedRows, err := resolveSetIDStrategyRegistrySnapshotRowsForTarget(ctx, tx, asOf, tenantUUID, rows)
	if err != nil {
		return err
	}
	for _, row := range resolvedRows {
		allowedValueCodesJSON := any(nil)
		if len(row.AllowedValueCodes) > 0 {
			raw, err := json.Marshal(row.AllowedValueCodes)
			if err != nil {
				return err
			}
			allowedValueCodesJSON = string(raw)
		}
		endDate := nullableString(row.EndDate)
		if _, err := tx.Exec(ctx, `
INSERT INTO orgunit.setid_strategy_registry (
  tenant_uuid,
  capability_key,
  owner_module,
  field_key,
  personalization_mode,
  org_applicability,
  business_unit_node_key,
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
  $3::text,
  $4::text,
  $5::text,
  $6::text,
  $7::text,
  $8::boolean,
  $9::boolean,
  $10::boolean,
  NULLIF($11::text, ''),
  NULLIF($12::text, ''),
  $13::jsonb,
  $14::integer,
  $15::text,
  $16::text,
  $17::boolean,
  $18::boolean,
  $19::text,
  $20::date,
  $21::date,
  $22::timestamptz
);
`, row.TenantUUID, row.CapabilityKey, row.OwnerModule, row.FieldKey, row.PersonalizationMode, row.OrgApplicability, row.BusinessUnitNodeKey, row.Required, row.Visible, row.Maintainable, row.DefaultRuleRef, row.DefaultValue, allowedValueCodesJSON, row.Priority, row.PriorityMode, row.LocalOverrideMode, row.ExplainRequired, row.IsStable, row.ChangePolicy, row.EffectiveDate, endDate, row.UpdatedAt); err != nil {
			return err
		}
	}
	return nil
}

func ensureSetIDStrategyRegistryTargetTenantEmpty(ctx context.Context, tx pgx.Tx, tenantUUID string) error {
	var rowCount int
	if err := tx.QueryRow(ctx, `
SELECT count(*)::int
FROM orgunit.setid_strategy_registry
WHERE tenant_uuid = $1::uuid;
`, tenantUUID).Scan(&rowCount); err != nil {
		return err
	}
	return validateSetIDStrategyRegistryTargetTenantRowCount(tenantUUID, rowCount)
}

func validateSetIDStrategyRegistryTargetTenantRowCount(tenantUUID string, rowCount int) error {
	if rowCount == 0 {
		return nil
	}
	return fmt.Errorf(
		"tenant %s target setid strategy registry is not empty: existing_rows=%d (DEV-PLAN-320 fresh target only)",
		tenantUUID,
		rowCount,
	)
}

func verifySetIDStrategyRegistrySnapshotTenant(ctx context.Context, tx pgx.Tx, asOf string, tenantUUID string, expectedRows []setIDStrategyRegistrySnapshotRow) error {
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantUUID); err != nil {
		return err
	}
	resolvedExpected, err := resolveSetIDStrategyRegistrySnapshotRowsForTarget(ctx, tx, asOf, tenantUUID, expectedRows)
	if err != nil {
		return err
	}
	actualRows, err := loadSetIDStrategyRegistryTargetRows(ctx, tx, asOf, tenantUUID)
	if err != nil {
		return err
	}
	if len(actualRows) != len(resolvedExpected) {
		return fmt.Errorf("tenant %s setid strategy registry row_count mismatch: got %d want %d", tenantUUID, len(actualRows), len(resolvedExpected))
	}
	sortSetIDStrategyRegistrySnapshotRows(actualRows)
	sortSetIDStrategyRegistrySnapshotRows(resolvedExpected)
	for idx := range resolvedExpected {
		if !equalSetIDStrategyRegistrySnapshotRow(actualRows[idx], resolvedExpected[idx]) {
			return fmt.Errorf(
				"tenant %s setid strategy registry row mismatch at %d: got=%s want=%s",
				tenantUUID,
				idx,
				setIDStrategyRegistrySnapshotComparableKey(actualRows[idx]),
				setIDStrategyRegistrySnapshotComparableKey(resolvedExpected[idx]),
			)
		}
	}
	return nil
}

func loadSetIDStrategyRegistryTargetRows(ctx context.Context, tx pgx.Tx, asOf string, tenantUUID string) ([]setIDStrategyRegistrySnapshotRow, error) {
	rows, err := tx.Query(ctx, `
SELECT
  r.tenant_uuid::text,
  r.capability_key,
  r.owner_module,
  r.field_key,
  r.personalization_mode,
  r.org_applicability,
  '' AS business_unit_source_value,
  CASE WHEN r.org_applicability = 'business_unit' THEN COALESCE(c.org_code, '') ELSE '' END AS business_unit_org_code,
  CASE WHEN r.org_applicability = 'business_unit' THEN r.business_unit_node_key ELSE '' END AS business_unit_node_key,
  r.required,
  r.visible,
  r.maintainable,
  COALESCE(r.default_rule_ref, ''),
  COALESCE(r.default_value, ''),
  COALESCE(r.allowed_value_codes, '[]'::jsonb)::text,
  r.priority,
  r.priority_mode,
  r.local_override_mode,
  r.explain_required,
  r.is_stable,
  r.change_policy,
  r.effective_date::text,
  COALESCE(r.end_date::text, ''),
  to_char(r.updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM orgunit.setid_strategy_registry r
LEFT JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = r.tenant_uuid
 AND c.org_node_key = NULLIF(r.business_unit_node_key, '')::char(8)
WHERE r.tenant_uuid = $1::uuid
  AND r.effective_date <= $2::date
  AND (r.end_date IS NULL OR r.end_date > $2::date)
ORDER BY r.capability_key, r.field_key, r.org_applicability, business_unit_org_code, business_unit_node_key, r.effective_date;
`, tenantUUID, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]setIDStrategyRegistrySnapshotRow, 0)
	for rows.Next() {
		var row setIDStrategyRegistrySnapshotRow
		var allowedValueCodesRaw string
		if err := rows.Scan(
			&row.TenantUUID,
			&row.CapabilityKey,
			&row.OwnerModule,
			&row.FieldKey,
			&row.PersonalizationMode,
			&row.OrgApplicability,
			&row.BusinessUnitSourceValue,
			&row.BusinessUnitOrgCode,
			&row.BusinessUnitNodeKey,
			&row.Required,
			&row.Visible,
			&row.Maintainable,
			&row.DefaultRuleRef,
			&row.DefaultValue,
			&allowedValueCodesRaw,
			&row.Priority,
			&row.PriorityMode,
			&row.LocalOverrideMode,
			&row.ExplainRequired,
			&row.IsStable,
			&row.ChangePolicy,
			&row.EffectiveDate,
			&row.EndDate,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if strings.TrimSpace(allowedValueCodesRaw) != "" {
			if err := json.Unmarshal([]byte(allowedValueCodesRaw), &row.AllowedValueCodes); err != nil {
				return nil, err
			}
		}
		out = append(out, normalizeSetIDStrategyRegistrySnapshotRow(row))
	}
	return out, rows.Err()
}

func resolveSetIDStrategyRegistrySnapshotRowsForTarget(ctx context.Context, tx pgx.Tx, asOf string, tenantUUID string, rows []setIDStrategyRegistrySnapshotRow) ([]setIDStrategyRegistrySnapshotRow, error) {
	codeToNodeKey := make(map[string]string)
	nodeKeyToCode := make(map[string]string)
	out := make([]setIDStrategyRegistrySnapshotRow, 0, len(rows))
	for _, raw := range rows {
		row := normalizeSetIDStrategyRegistrySnapshotRow(raw)
		if row.TenantUUID != tenantUUID {
			return nil, fmt.Errorf("tenant mismatch in snapshot rows: row tenant=%s want=%s", row.TenantUUID, tenantUUID)
		}
		if row.OrgApplicability == "tenant" {
			row.BusinessUnitOrgCode = ""
			row.BusinessUnitNodeKey = ""
			out = append(out, row)
			continue
		}

		nodeKey := row.BusinessUnitNodeKey
		orgCode := row.BusinessUnitOrgCode
		switch {
		case nodeKey != "" && orgCode != "":
			resolvedNodeKey, err := lookupSetIDStrategyRegistryTargetOrgNodeKeyByCode(ctx, tx, tenantUUID, orgCode)
			if err != nil {
				return nil, err
			}
			if resolvedNodeKey != nodeKey {
				return nil, fmt.Errorf("tenant %s org_code %s resolves to org_node_key %s but snapshot has %s", tenantUUID, orgCode, resolvedNodeKey, nodeKey)
			}
		case nodeKey != "":
			if cachedOrgCode, ok := nodeKeyToCode[nodeKey]; ok {
				orgCode = cachedOrgCode
			} else {
				resolvedOrgCode, err := lookupSetIDStrategyRegistryTargetOrgCodeByNodeKey(ctx, tx, tenantUUID, nodeKey)
				if err != nil {
					return nil, err
				}
				orgCode = resolvedOrgCode
				nodeKeyToCode[nodeKey] = orgCode
			}
		case orgCode != "":
			if cachedNodeKey, ok := codeToNodeKey[orgCode]; ok {
				nodeKey = cachedNodeKey
			} else {
				resolvedNodeKey, err := lookupSetIDStrategyRegistryTargetOrgNodeKeyByCode(ctx, tx, tenantUUID, orgCode)
				if err != nil {
					return nil, err
				}
				nodeKey = resolvedNodeKey
				codeToNodeKey[orgCode] = nodeKey
			}
		default:
			return nil, fmt.Errorf("tenant %s business_unit row missing business_unit_org_code/business_unit_node_key", tenantUUID)
		}

		currentCount, err := countCurrentSetIDStrategyRegistryTargetOrgNodeKey(ctx, tx, tenantUUID, nodeKey, asOf)
		if err != nil {
			return nil, err
		}
		if currentCount != 1 {
			return nil, fmt.Errorf("tenant %s org_node_key %s current target mapping count=%d want=1", tenantUUID, nodeKey, currentCount)
		}
		if orgCode != "" {
			codeToNodeKey[orgCode] = nodeKey
		}
		if nodeKey != "" && orgCode != "" {
			nodeKeyToCode[nodeKey] = orgCode
		}
		row.BusinessUnitOrgCode = orgCode
		row.BusinessUnitNodeKey = nodeKey
		out = append(out, row)
	}
	sortSetIDStrategyRegistrySnapshotRows(out)
	return out, nil
}

func lookupSetIDStrategyRegistryTargetOrgNodeKeyByCode(ctx context.Context, tx pgx.Tx, tenantUUID string, orgCode string) (string, error) {
	var orgNodeKey string
	if err := tx.QueryRow(ctx, `
SELECT org_node_key::text
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid
  AND org_code = $2::text;
`, tenantUUID, orgCode).Scan(&orgNodeKey); err != nil {
		return "", fmt.Errorf("tenant %s org_code %s resolve org_node_key: %w", tenantUUID, orgCode, err)
	}
	return strings.TrimSpace(orgNodeKey), nil
}

func lookupSetIDStrategyRegistryTargetOrgCodeByNodeKey(ctx context.Context, tx pgx.Tx, tenantUUID string, orgNodeKey string) (string, error) {
	var orgCode string
	if err := tx.QueryRow(ctx, `
SELECT org_code
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid
  AND org_node_key = $2::char(8);
`, tenantUUID, orgNodeKey).Scan(&orgCode); err != nil {
		return "", fmt.Errorf("tenant %s org_node_key %s resolve org_code: %w", tenantUUID, orgNodeKey, err)
	}
	return strings.TrimSpace(orgCode), nil
}

func countCurrentSetIDStrategyRegistryTargetOrgNodeKey(ctx context.Context, tx pgx.Tx, tenantUUID string, orgNodeKey string, asOf string) (int, error) {
	var count int
	if err := tx.QueryRow(ctx, `
SELECT count(*)::int
FROM orgunit.org_unit_versions
WHERE tenant_uuid = $1::uuid
  AND org_node_key = $2::char(8)
  AND validity @> $3::date;
`, tenantUUID, orgNodeKey, asOf).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func readSetIDStrategyRegistrySnapshot(path string) (setIDStrategyRegistrySnapshotFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return setIDStrategyRegistrySnapshotFile{}, err
	}
	var snapshot setIDStrategyRegistrySnapshotFile
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return setIDStrategyRegistrySnapshotFile{}, err
	}
	return snapshot, nil
}

func writeSetIDStrategyRegistrySnapshot(path string, snapshot setIDStrategyRegistrySnapshotFile) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func validateSetIDStrategyRegistrySnapshot(snapshot setIDStrategyRegistrySnapshotFile) error {
	if strings.TrimSpace(snapshot.Version) != setIDStrategyRegistrySnapshotVersion {
		return fmt.Errorf("setid strategy registry snapshot version mismatch: got %q want %q", snapshot.Version, setIDStrategyRegistrySnapshotVersion)
	}
	asOf := strings.TrimSpace(snapshot.AsOfDate)
	if asOf == "" {
		return fmt.Errorf("setid strategy registry snapshot as_of_date is required")
	}
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		return fmt.Errorf("setid strategy registry snapshot as_of_date invalid: %w", err)
	}
	if snapshot.RowCount != len(snapshot.Rows) {
		return fmt.Errorf("setid strategy registry snapshot row_count mismatch: got %d want %d", snapshot.RowCount, len(snapshot.Rows))
	}

	var errs []string
	seen := make(map[string]struct{}, len(snapshot.Rows))
	for idx, raw := range snapshot.Rows {
		row := normalizeSetIDStrategyRegistrySnapshotRow(raw)
		if err := validateSetIDStrategyRegistrySnapshotRow(row); err != nil {
			errs = append(errs, fmt.Sprintf("row[%d]: %v", idx, err))
			continue
		}
		dupKey := setIDStrategyRegistrySnapshotComparableKey(row)
		if _, ok := seen[dupKey]; ok {
			errs = append(errs, fmt.Sprintf("row[%d]: duplicate active row key %s", idx, dupKey))
			continue
		}
		seen[dupKey] = struct{}{}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func validateSetIDStrategyRegistrySnapshotRow(row setIDStrategyRegistrySnapshotRow) error {
	if row.TenantUUID == "" {
		return fmt.Errorf("tenant_uuid is required")
	}
	if row.CapabilityKey == "" || row.OwnerModule == "" || row.FieldKey == "" || row.PersonalizationMode == "" || row.OrgApplicability == "" || row.EffectiveDate == "" {
		return fmt.Errorf("capability_key/owner_module/field_key/personalization_mode/org_applicability/effective_date required")
	}
	if _, err := time.Parse("2006-01-02", row.EffectiveDate); err != nil {
		return fmt.Errorf("effective_date invalid: %w", err)
	}
	if row.EndDate != "" {
		endDate, err := time.Parse("2006-01-02", row.EndDate)
		if err != nil {
			return fmt.Errorf("end_date invalid: %w", err)
		}
		effectiveDate, _ := time.Parse("2006-01-02", row.EffectiveDate)
		if !endDate.After(effectiveDate) {
			return fmt.Errorf("end_date must be after effective_date")
		}
	}
	if row.UpdatedAt == "" {
		return fmt.Errorf("updated_at is required")
	}
	if _, err := time.Parse(time.RFC3339, row.UpdatedAt); err != nil {
		return fmt.Errorf("updated_at invalid: %w", err)
	}
	switch row.OrgApplicability {
	case "tenant":
		if row.BusinessUnitOrgCode != "" || row.BusinessUnitNodeKey != "" {
			return fmt.Errorf("tenant row must not carry business_unit_org_code/business_unit_node_key")
		}
	case "business_unit":
		if row.BusinessUnitOrgCode == "" && row.BusinessUnitNodeKey == "" {
			return fmt.Errorf("business_unit row must include business_unit_org_code or business_unit_node_key")
		}
		if row.BusinessUnitNodeKey != "" && !orgNodeKeyPattern.MatchString(row.BusinessUnitNodeKey) {
			return fmt.Errorf("business_unit_node_key invalid")
		}
	default:
		return fmt.Errorf("org_applicability invalid: %s", row.OrgApplicability)
	}
	return nil
}

func normalizeSetIDStrategyRegistrySnapshotRow(row setIDStrategyRegistrySnapshotRow) setIDStrategyRegistrySnapshotRow {
	row.TenantUUID = strings.TrimSpace(row.TenantUUID)
	row.CapabilityKey = strings.ToLower(strings.TrimSpace(row.CapabilityKey))
	row.OwnerModule = strings.ToLower(strings.TrimSpace(row.OwnerModule))
	row.FieldKey = strings.ToLower(strings.TrimSpace(row.FieldKey))
	row.PersonalizationMode = strings.ToLower(strings.TrimSpace(row.PersonalizationMode))
	row.OrgApplicability = strings.ToLower(strings.TrimSpace(row.OrgApplicability))
	row.BusinessUnitSourceValue = strings.TrimSpace(row.BusinessUnitSourceValue)
	row.BusinessUnitOrgCode = strings.TrimSpace(row.BusinessUnitOrgCode)
	row.BusinessUnitNodeKey = strings.TrimSpace(row.BusinessUnitNodeKey)
	row.DefaultRuleRef = strings.TrimSpace(row.DefaultRuleRef)
	row.DefaultValue = strings.TrimSpace(row.DefaultValue)
	row.PriorityMode = strings.ToLower(strings.TrimSpace(row.PriorityMode))
	row.LocalOverrideMode = strings.ToLower(strings.TrimSpace(row.LocalOverrideMode))
	row.ChangePolicy = strings.ToLower(strings.TrimSpace(row.ChangePolicy))
	row.EffectiveDate = strings.TrimSpace(row.EffectiveDate)
	row.EndDate = strings.TrimSpace(row.EndDate)
	row.UpdatedAt = strings.TrimSpace(row.UpdatedAt)
	row.AllowedValueCodes = normalizeSetIDStrategyRegistryAllowedValueCodes(row.AllowedValueCodes)
	return row
}

func normalizeSetIDStrategyRegistryAllowedValueCodes(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func setIDStrategyRegistrySnapshotComparableKey(row setIDStrategyRegistrySnapshotRow) string {
	row = normalizeSetIDStrategyRegistrySnapshotRow(row)
	return strings.Join([]string{
		row.TenantUUID,
		row.CapabilityKey,
		row.FieldKey,
		row.OrgApplicability,
		row.BusinessUnitOrgCode,
		row.BusinessUnitNodeKey,
		row.EffectiveDate,
		row.EndDate,
		row.UpdatedAt,
	}, "|")
}

func sortSetIDStrategyRegistrySnapshotRows(rows []setIDStrategyRegistrySnapshotRow) {
	sort.Slice(rows, func(i, j int) bool {
		left := normalizeSetIDStrategyRegistrySnapshotRow(rows[i])
		right := normalizeSetIDStrategyRegistrySnapshotRow(rows[j])
		return setIDStrategyRegistrySnapshotComparableKey(left) < setIDStrategyRegistrySnapshotComparableKey(right)
	})
}

func equalSetIDStrategyRegistrySnapshotRow(left setIDStrategyRegistrySnapshotRow, right setIDStrategyRegistrySnapshotRow) bool {
	left = normalizeSetIDStrategyRegistrySnapshotRow(left)
	right = normalizeSetIDStrategyRegistrySnapshotRow(right)
	return left.TenantUUID == right.TenantUUID &&
		left.CapabilityKey == right.CapabilityKey &&
		left.OwnerModule == right.OwnerModule &&
		left.FieldKey == right.FieldKey &&
		left.PersonalizationMode == right.PersonalizationMode &&
		left.OrgApplicability == right.OrgApplicability &&
		left.BusinessUnitOrgCode == right.BusinessUnitOrgCode &&
		left.BusinessUnitNodeKey == right.BusinessUnitNodeKey &&
		left.Required == right.Required &&
		left.Visible == right.Visible &&
		left.Maintainable == right.Maintainable &&
		left.DefaultRuleRef == right.DefaultRuleRef &&
		left.DefaultValue == right.DefaultValue &&
		equalStringSlices(left.AllowedValueCodes, right.AllowedValueCodes) &&
		left.Priority == right.Priority &&
		left.PriorityMode == right.PriorityMode &&
		left.LocalOverrideMode == right.LocalOverrideMode &&
		left.ExplainRequired == right.ExplainRequired &&
		left.IsStable == right.IsStable &&
		left.ChangePolicy == right.ChangePolicy &&
		left.EffectiveDate == right.EffectiveDate &&
		left.EndDate == right.EndDate &&
		left.UpdatedAt == right.UpdatedAt
}

func setIDStrategyRegistrySnapshotTenantIDs(snapshot setIDStrategyRegistrySnapshotFile) []string {
	seen := make(map[string]struct{}, len(snapshot.Rows))
	out := make([]string, 0, len(snapshot.Rows))
	for _, row := range snapshot.Rows {
		tenantUUID := strings.TrimSpace(row.TenantUUID)
		if tenantUUID == "" {
			continue
		}
		if _, ok := seen[tenantUUID]; ok {
			continue
		}
		seen[tenantUUID] = struct{}{}
		out = append(out, tenantUUID)
	}
	sort.Strings(out)
	return out
}

func setIDStrategyRegistrySnapshotRowsForTenant(rows []setIDStrategyRegistrySnapshotRow, tenantUUID string) []setIDStrategyRegistrySnapshotRow {
	out := make([]setIDStrategyRegistrySnapshotRow, 0)
	for _, row := range rows {
		if strings.TrimSpace(row.TenantUUID) != tenantUUID {
			continue
		}
		out = append(out, normalizeSetIDStrategyRegistrySnapshotRow(row))
	}
	sortSetIDStrategyRegistrySnapshotRows(out)
	return out
}

func countSetIDStrategyRegistryBusinessUnitRows(rows []setIDStrategyRegistrySnapshotRow) int {
	count := 0
	for _, row := range rows {
		if strings.TrimSpace(row.OrgApplicability) == "business_unit" {
			count++
		}
	}
	return count
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
