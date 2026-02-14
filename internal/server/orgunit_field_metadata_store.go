package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type orgUnitTenantFieldConfig struct {
	FieldKey         string
	ValueType        string
	DataSourceType   string
	DataSourceConfig json.RawMessage
	PhysicalCol      string
	EnabledOn        string
	DisabledOn       *string
	UpdatedAt        time.Time
}

type orgUnitVersionExtSnapshot struct {
	VersionValues  map[string]any
	VersionLabels  map[string]string
	EventLabels    map[string]string
	LastEventID    int64
	HasVersionData bool
}

type orgUnitMutationTargetEvent struct {
	EffectiveEventType string
	RawEventType       string
	HasEffective       bool
	HasRaw             bool
}

var orgUnitExtPhysicalColRe = regexp.MustCompile(`^ext_(str|int|uuid|bool|date)_[0-9]{2}$`)

var errOrgUnitExtQueryFieldNotAllowed = errors.New("org_ext_query_field_not_allowed")

func (s *orgUnitPGStore) ListTenantFieldConfigs(ctx context.Context, tenantID string) ([]orgUnitTenantFieldConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
SELECT
  field_key,
  value_type,
  data_source_type,
  data_source_config,
  physical_col,
  enabled_on::text,
  CASE WHEN disabled_on IS NULL THEN NULL ELSE disabled_on::text END AS disabled_on,
  updated_at
FROM orgunit.tenant_field_configs
WHERE tenant_uuid = $1::uuid
ORDER BY field_key ASC
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]orgUnitTenantFieldConfig, 0)
	for rows.Next() {
		item, scanErr := scanOrgUnitTenantFieldConfig(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *orgUnitPGStore) ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	items, err := listEnabledTenantFieldConfigsAsOfTx(ctx, tx, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func listEnabledTenantFieldConfigsAsOfTx(ctx context.Context, tx pgx.Tx, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error) {
	rows, err := tx.Query(ctx, `
SELECT
  field_key,
  value_type,
  data_source_type,
  data_source_config,
  physical_col,
  enabled_on::text,
  CASE WHEN disabled_on IS NULL THEN NULL ELSE disabled_on::text END AS disabled_on,
  updated_at
FROM orgunit.tenant_field_configs
WHERE tenant_uuid = $1::uuid
  AND enabled_on <= $2::date
  AND (disabled_on IS NULL OR $2::date < disabled_on)
ORDER BY field_key ASC
`, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]orgUnitTenantFieldConfig, 0)
	for rows.Next() {
		item, scanErr := scanOrgUnitTenantFieldConfig(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *orgUnitPGStore) GetEnabledTenantFieldConfigAsOf(ctx context.Context, tenantID string, fieldKey string, asOf string) (orgUnitTenantFieldConfig, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}

	item, ok, err := getEnabledTenantFieldConfigAsOfTx(ctx, tx, tenantID, fieldKey, asOf)
	if err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}
	return item, ok, nil
}

func getEnabledTenantFieldConfigAsOfTx(ctx context.Context, tx pgx.Tx, tenantID string, fieldKey string, asOf string) (orgUnitTenantFieldConfig, bool, error) {
	var cfg orgUnitTenantFieldConfig
	var rawConfig []byte
	var disabledOn *string
	err := tx.QueryRow(ctx, `
SELECT
  field_key,
  value_type,
  data_source_type,
  data_source_config,
  physical_col,
  enabled_on::text,
  CASE WHEN disabled_on IS NULL THEN NULL ELSE disabled_on::text END AS disabled_on,
  updated_at
FROM orgunit.tenant_field_configs
WHERE tenant_uuid = $1::uuid
  AND field_key = $2::text
  AND enabled_on <= $3::date
  AND (disabled_on IS NULL OR $3::date < disabled_on)
LIMIT 1
`, tenantID, fieldKey, asOf).Scan(
		&cfg.FieldKey,
		&cfg.ValueType,
		&cfg.DataSourceType,
		&rawConfig,
		&cfg.PhysicalCol,
		&cfg.EnabledOn,
		&disabledOn,
		&cfg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orgUnitTenantFieldConfig{}, false, nil
		}
		return orgUnitTenantFieldConfig{}, false, err
	}
	cfg.DataSourceConfig = cloneRawJSON(rawConfig)
	cfg.DisabledOn = cloneOptionalString(disabledOn)
	return cfg, true, nil
}

func (s *orgUnitPGStore) EnableTenantFieldConfig(
	ctx context.Context,
	tenantID string,
	fieldKey string,
	valueType string,
	dataSourceType string,
	dataSourceConfig json.RawMessage,
	enabledOn string,
	requestCode string,
	initiatorUUID string,
) (orgUnitTenantFieldConfig, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}

	wasRetry, err := tenantFieldConfigRequestExistsTx(ctx, tx, tenantID, requestCode, "ENABLE")
	if err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}

	if _, err := tx.Exec(ctx, `
SELECT orgunit.enable_tenant_field_config(
  $1::uuid,
  $2::text,
  $3::text,
  $4::date,
  $5::text,
  $6::jsonb,
  $7::text,
  $8::uuid
)
`, tenantID, fieldKey, valueType, enabledOn, dataSourceType, dataSourceConfig, requestCode, initiatorUUID); err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}

	cfg, err := getTenantFieldConfigByKeyTx(ctx, tx, tenantID, fieldKey)
	if err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}
	return cfg, wasRetry, nil
}

func (s *orgUnitPGStore) DisableTenantFieldConfig(
	ctx context.Context,
	tenantID string,
	fieldKey string,
	disabledOn string,
	requestCode string,
	initiatorUUID string,
) (orgUnitTenantFieldConfig, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}

	wasRetry, err := tenantFieldConfigRequestExistsTx(ctx, tx, tenantID, requestCode, "DISABLE")
	if err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}

	if _, err := tx.Exec(ctx, `
SELECT orgunit.disable_tenant_field_config(
  $1::uuid,
  $2::text,
  $3::date,
  $4::text,
  $5::uuid
)
`, tenantID, fieldKey, disabledOn, requestCode, initiatorUUID); err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}

	cfg, err := getTenantFieldConfigByKeyTx(ctx, tx, tenantID, fieldKey)
	if err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return orgUnitTenantFieldConfig{}, false, err
	}
	return cfg, wasRetry, nil
}

func tenantFieldConfigRequestExistsTx(ctx context.Context, tx pgx.Tx, tenantID string, requestCode string, expectedType string) (bool, error) {
	var eventType string
	err := tx.QueryRow(ctx, `
SELECT event_type
FROM orgunit.tenant_field_config_events
WHERE tenant_uuid = $1::uuid
  AND request_code = $2::text
LIMIT 1
`, tenantID, requestCode).Scan(&eventType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(eventType), expectedType), nil
}

func getTenantFieldConfigByKeyTx(ctx context.Context, tx pgx.Tx, tenantID string, fieldKey string) (orgUnitTenantFieldConfig, error) {
	var cfg orgUnitTenantFieldConfig
	var rawConfig []byte
	var disabledOn *string
	err := tx.QueryRow(ctx, `
SELECT
  field_key,
  value_type,
  data_source_type,
  data_source_config,
  physical_col,
  enabled_on::text,
  CASE WHEN disabled_on IS NULL THEN NULL ELSE disabled_on::text END AS disabled_on,
  updated_at
FROM orgunit.tenant_field_configs
WHERE tenant_uuid = $1::uuid
  AND field_key = $2::text
LIMIT 1
`, tenantID, fieldKey).Scan(
		&cfg.FieldKey,
		&cfg.ValueType,
		&cfg.DataSourceType,
		&rawConfig,
		&cfg.PhysicalCol,
		&cfg.EnabledOn,
		&disabledOn,
		&cfg.UpdatedAt,
	)
	if err != nil {
		return orgUnitTenantFieldConfig{}, err
	}
	cfg.DataSourceConfig = cloneRawJSON(rawConfig)
	cfg.DisabledOn = cloneOptionalString(disabledOn)
	return cfg, nil
}

func (s *orgUnitPGStore) GetOrgUnitVersionExtSnapshot(ctx context.Context, tenantID string, orgID int, asOf string) (orgUnitVersionExtSnapshot, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return orgUnitVersionExtSnapshot{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return orgUnitVersionExtSnapshot{}, err
	}

	var versionJSON []byte
	var labelsJSON []byte
	var lastEventID int64
	if err := tx.QueryRow(ctx, `
SELECT to_jsonb(v), COALESCE(v.ext_labels_snapshot, {}::jsonb), v.last_event_id
FROM orgunit.org_unit_versions v
WHERE v.tenant_uuid = $1::uuid
  AND v.org_id = $2::int
  AND v.validity @> $3::date
ORDER BY lower(v.validity) DESC
LIMIT 1
`, tenantID, orgID, asOf).Scan(&versionJSON, &labelsJSON, &lastEventID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orgUnitVersionExtSnapshot{}, errOrgUnitNotFound
		}
		return orgUnitVersionExtSnapshot{}, err
	}

	versionMap := make(map[string]any)
	if len(versionJSON) > 0 {
		if err := json.Unmarshal(versionJSON, &versionMap); err != nil {
			return orgUnitVersionExtSnapshot{}, err
		}
	}

	versionLabels, err := decodeStringMap(labelsJSON)
	if err != nil {
		return orgUnitVersionExtSnapshot{}, err
	}

	eventLabels := map[string]string{}
	if lastEventID > 0 {
		var payload []byte
		if err := tx.QueryRow(ctx, `
SELECT payload
FROM orgunit.org_events_effective
WHERE tenant_uuid = $1::uuid
  AND id = $2::bigint
LIMIT 1
`, tenantID, lastEventID).Scan(&payload); err == nil {
			eventLabels, _ = decodeExtLabelsFromPayload(payload)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return orgUnitVersionExtSnapshot{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return orgUnitVersionExtSnapshot{}, err
	}

	return orgUnitVersionExtSnapshot{
		VersionValues:  versionMap,
		VersionLabels:  versionLabels,
		EventLabels:    eventLabels,
		LastEventID:    lastEventID,
		HasVersionData: true,
	}, nil
}

func (s *orgUnitPGStore) ResolveMutationTargetEvent(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitMutationTargetEvent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return orgUnitMutationTargetEvent{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return orgUnitMutationTargetEvent{}, err
	}

	result := orgUnitMutationTargetEvent{}
	var eventUUID string
	err = tx.QueryRow(ctx, `
SELECT event_uuid::text, event_type
FROM orgunit.org_events_effective
WHERE tenant_uuid = $1::uuid
  AND org_id = $2::int
  AND effective_date = $3::date
ORDER BY id DESC
LIMIT 1
`, tenantID, orgID, effectiveDate).Scan(&eventUUID, &result.EffectiveEventType)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return orgUnitMutationTargetEvent{}, err
		}
	} else {
		result.HasEffective = true
	}

	if result.HasEffective {
		err = tx.QueryRow(ctx, `
SELECT event_type
FROM orgunit.org_events
WHERE tenant_uuid = $1::uuid
  AND event_uuid = $2::uuid
LIMIT 1
`, tenantID, eventUUID).Scan(&result.RawEventType)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return orgUnitMutationTargetEvent{}, err
			}
		} else {
			result.HasRaw = true
		}
	}

	if !result.HasRaw {
		err = tx.QueryRow(ctx, `
	SELECT event_type
	FROM orgunit.org_events
	WHERE tenant_uuid = $1::uuid
	  AND org_id = $2::int
	  AND effective_date = $3::date
	  AND event_type IN ('CREATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT')
	ORDER BY id DESC
	LIMIT 1
	`, tenantID, orgID, effectiveDate).Scan(&result.RawEventType)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return orgUnitMutationTargetEvent{}, err
			}
		} else {
			result.HasRaw = true
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return orgUnitMutationTargetEvent{}, err
	}

	result.EffectiveEventType = strings.TrimSpace(result.EffectiveEventType)
	result.RawEventType = strings.TrimSpace(result.RawEventType)
	if result.RawEventType == "" {
		result.HasRaw = false
	}
	if result.EffectiveEventType == "" {
		result.HasEffective = false
	}
	return result, nil
}

func (s *orgUnitPGStore) EvaluateRescindOrgDenyReasons(ctx context.Context, tenantID string, orgID int) ([]string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	deny := make([]string, 0)

	var rootOrgID *int
	if err := tx.QueryRow(ctx, `
SELECT root_org_id
FROM orgunit.org_trees
WHERE tenant_uuid = $1::uuid
LIMIT 1
`, tenantID).Scan(&rootOrgID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if rootOrgID != nil && *rootOrgID == orgID {
		deny = append(deny, orgUnitErrRootDeleteForbidden)
	}

	var nodePath *string
	if err := tx.QueryRow(ctx, `
SELECT node_path::text
FROM orgunit.org_unit_versions
WHERE tenant_uuid = $1::uuid
  AND org_id = $2::int
ORDER BY lower(validity) DESC
LIMIT 1
`, tenantID, orgID).Scan(&nodePath); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	if nodePath != nil && strings.TrimSpace(*nodePath) != "" {
		var hasChildren bool
		if err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM orgunit.org_unit_versions c
  WHERE c.tenant_uuid = $1::uuid
    AND c.node_path <@ $2::ltree
    AND c.org_id <> $3::int
  LIMIT 1
)
`, tenantID, *nodePath, orgID).Scan(&hasChildren); err != nil {
			return nil, err
		}
		if hasChildren {
			deny = append(deny, orgUnitErrHasChildrenCannotDelete)
		}
	}

	var hasDependencies bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM orgunit.setid_binding_versions b
  WHERE b.tenant_uuid = $1::uuid
    AND b.org_id = $2::int
  LIMIT 1
)
`, tenantID, orgID).Scan(&hasDependencies); err != nil {
		return nil, err
	}
	if hasDependencies {
		deny = append(deny, orgUnitErrHasDependenciesCannotDelete)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return dedupDenyReasons(deny), nil
}

func (s *orgUnitPGStore) ListOrgUnitsPage(ctx context.Context, tenantID string, req orgUnitListPageRequest) ([]orgUnitListItem, int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, 0, err
	}

	where := make([]string, 0, 8)
	where = append(where, `v.tenant_uuid = $1::uuid`, `v.validity @> $2::date`)
	args := make([]any, 0, 10)
	args = append(args, tenantID, req.AsOf)
	argPos := 3

	if req.ParentID != nil {
		where = append(where, fmt.Sprintf("v.parent_id = $%d::int", argPos))
		args = append(args, *req.ParentID)
		argPos++
	} else {
		where = append(where, "v.parent_id IS NULL")
	}

	if !req.IncludeDisabled {
		where = append(where, `v.status = 'active'`)
	}
	if req.Status == orgUnitListStatusActive {
		where = append(where, `v.status = 'active'`)
	}
	if req.Status == orgUnitListStatusDisabled {
		where = append(where, `v.status = 'disabled'`)
	}

	keyword := strings.TrimSpace(req.Keyword)
	if keyword != "" {
		where = append(where, fmt.Sprintf(`(c.org_code ILIKE %% || $%d::text || %% OR v.name ILIKE %% || $%d::text || %%)`, argPos, argPos))
		args = append(args, keyword)
		argPos++
	}

	if req.ExtFilterFieldKey != "" {
		cfg, ok, err := getEnabledTenantFieldConfigAsOfTx(ctx, tx, tenantID, req.ExtFilterFieldKey, req.AsOf)
		if err != nil {
			return nil, 0, err
		}
		if !ok {
			return nil, 0, errOrgUnitExtQueryFieldNotAllowed
		}
		def, ok := lookupOrgUnitFieldDefinition(req.ExtFilterFieldKey)
		if !ok || !def.AllowFilter || !orgUnitExtPhysicalColRe.MatchString(cfg.PhysicalCol) {
			return nil, 0, errOrgUnitExtQueryFieldNotAllowed
		}
		parsedValue, parseErr := parseOrgUnitExtQueryValue(cfg.ValueType, req.ExtFilterValue)
		if parseErr != nil {
			return nil, 0, errOrgUnitExtQueryFieldNotAllowed
		}
		where = append(where, fmt.Sprintf("v.%s = $%d", quoteSQLIdentifier(cfg.PhysicalCol), argPos))
		args = append(args, parsedValue)
		argPos++
	}

	var extSortConfig *orgUnitTenantFieldConfig
	if req.ExtSortFieldKey != "" {
		cfg, ok, err := getEnabledTenantFieldConfigAsOfTx(ctx, tx, tenantID, req.ExtSortFieldKey, req.AsOf)
		if err != nil {
			return nil, 0, err
		}
		if !ok {
			return nil, 0, errOrgUnitExtQueryFieldNotAllowed
		}
		def, ok := lookupOrgUnitFieldDefinition(req.ExtSortFieldKey)
		if !ok || !def.AllowSort || !orgUnitExtPhysicalColRe.MatchString(cfg.PhysicalCol) {
			return nil, 0, errOrgUnitExtQueryFieldNotAllowed
		}
		extSortConfig = &cfg
	}

	whereSQL := strings.Join(where, "\n  AND ")

	countSQL := `
SELECT COUNT(*)
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_id = v.org_id
WHERE ` + whereSQL

	var total int
	if err := tx.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	sortExpr := "c.org_code"
	switch req.SortField {
	case orgUnitListSortName:
		sortExpr = "v.name"
	case orgUnitListSortStatus:
		sortExpr = "v.status"
	case orgUnitListSortCode:
		sortExpr = "c.org_code"
	}
	if extSortConfig != nil {
		sortExpr = "v." + quoteSQLIdentifier(extSortConfig.PhysicalCol)
	}

	sortOrder := strings.ToUpper(strings.TrimSpace(req.SortOrder))
	if sortOrder != "DESC" {
		sortOrder = "ASC"
	}

	selectCols := `
SELECT
  c.org_code,
  v.name,
  v.status,
  v.is_business_unit`
	if req.ParentID != nil {
		selectCols += fmt.Sprintf(`,
  EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions ch
     WHERE ch.tenant_uuid = v.tenant_uuid
       AND ch.parent_id = v.org_id
       AND ch.validity @> $2::date
       AND ch.status = 'active'
     LIMIT 1
   ) AS has_children`)
	}

	listSQL := selectCols + `
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = v.tenant_uuid
 AND c.org_id = v.org_id
WHERE ` + whereSQL + `
ORDER BY ` + sortExpr + ` ` + sortOrder + `, c.org_code ASC`

	queryArgs := append([]any(nil), args...)
	if req.Limit > 0 {
		listSQL += fmt.Sprintf("\nLIMIT $%d OFFSET $%d", argPos, argPos+1)
		queryArgs = append(queryArgs, req.Limit, req.Offset)
	}

	rows, err := tx.Query(ctx, listSQL, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]orgUnitListItem, 0)
	for rows.Next() {
		item := orgUnitListItem{}
		var status string
		var isBusinessUnit bool
		if req.ParentID != nil {
			var hasChildren bool
			if err := rows.Scan(&item.OrgCode, &item.Name, &status, &isBusinessUnit, &hasChildren); err != nil {
				return nil, 0, err
			}
			item.HasChildren = &hasChildren
		} else {
			if err := rows.Scan(&item.OrgCode, &item.Name, &status, &isBusinessUnit); err != nil {
				return nil, 0, err
			}
		}
		if strings.TrimSpace(status) == "" {
			status = orgUnitListStatusActive
		}
		item.Status = status
		item.IsBusinessUnit = &isBusinessUnit
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func scanOrgUnitTenantFieldConfig(row interface {
	Scan(dest ...any) error
}) (orgUnitTenantFieldConfig, error) {
	var item orgUnitTenantFieldConfig
	var rawConfig []byte
	var disabledOn *string
	if err := row.Scan(
		&item.FieldKey,
		&item.ValueType,
		&item.DataSourceType,
		&rawConfig,
		&item.PhysicalCol,
		&item.EnabledOn,
		&disabledOn,
		&item.UpdatedAt,
	); err != nil {
		return orgUnitTenantFieldConfig{}, err
	}
	item.DataSourceConfig = cloneRawJSON(rawConfig)
	item.DisabledOn = cloneOptionalString(disabledOn)
	return item, nil
}

func cloneRawJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return json.RawMessage(out)
}

func cloneOptionalString(in *string) *string {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func decodeStringMap(raw []byte) (map[string]string, error) {
	result := map[string]string{}
	if len(raw) == 0 {
		return result, nil
	}
	var tmp map[string]any
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return nil, err
	}
	for key, value := range tmp {
		if str, ok := value.(string); ok {
			result[key] = str
		}
	}
	return result, nil
}

func decodeExtLabelsFromPayload(payload []byte) (map[string]string, error) {
	out := map[string]string{}
	if len(payload) == 0 {
		return out, nil
	}
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	rawLabels, ok := body["ext_labels_snapshot"]
	if !ok {
		return out, nil
	}
	labelsMap, ok := rawLabels.(map[string]any)
	if !ok {
		return out, nil
	}
	for key, value := range labelsMap {
		if text, ok := value.(string); ok {
			out[key] = text
		}
	}
	return out, nil
}

func parseOrgUnitExtQueryValue(valueType string, raw string) (any, error) {
	valueType = strings.ToLower(strings.TrimSpace(valueType))
	input := strings.TrimSpace(raw)

	switch valueType {
	case "text":
		return input, nil
	case "int":
		parsed, err := strconv.Atoi(input)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case "uuid":
		parsed, err := uuid.Parse(input)
		if err != nil {
			return nil, err
		}
		return parsed.String(), nil
	case "bool":
		switch strings.ToLower(input) {
		case "true", "1":
			return true, nil
		case "false", "0":
			return false, nil
		default:
			return nil, errors.New("bool invalid")
		}
	case "date":
		if _, err := time.Parse("2006-01-02", input); err != nil {
			return nil, err
		}
		return input, nil
	default:
		return nil, errors.New("value_type unsupported")
	}
}

func quoteSQLIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func orgUnitFieldConfigEnabledAsOf(cfg orgUnitTenantFieldConfig, asOf string) bool {
	if strings.TrimSpace(asOf) == "" {
		return false
	}
	enabledOn := strings.TrimSpace(cfg.EnabledOn)
	if enabledOn == "" || enabledOn > asOf {
		return false
	}
	if cfg.DisabledOn == nil {
		return true
	}
	disabledOn := strings.TrimSpace(*cfg.DisabledOn)
	if disabledOn == "" {
		return true
	}
	return asOf < disabledOn
}

func dedupDenyReasons(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		key := strings.TrimSpace(item)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return denyReasonPriority(out[i]) < denyReasonPriority(out[j])
	})
	return out
}

func denyReasonPriority(code string) int {
	switch code {
	case "FORBIDDEN":
		return 10
	case orgUnitErrEventNotFound:
		return 20
	case orgUnitErrEventRescinded:
		return 30
	case orgUnitErrRootDeleteForbidden:
		return 40
	case orgUnitErrHasChildrenCannotDelete:
		return 50
	case orgUnitErrHasDependenciesCannotDelete:
		return 60
	case orgUnitErrStatusCorrectionUnsupported:
		return 70
	default:
		return 100
	}
}
