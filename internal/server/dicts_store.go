package server

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
)

const (
	dictCodeOrgType = "org_type"
	// 保留该常量用于历史测试数据与迁移工具识别；运行时读取已收敛为 tenant-only。
	globalTenantID = "00000000-0000-0000-0000-000000000000"

	dictEventCreated        = "DICT_VALUE_CREATED"
	dictEventLabelCorrected = "DICT_VALUE_LABEL_CORRECTED"
	dictEventDisabled       = "DICT_VALUE_DISABLED"

	dictValueEventCreated        = "DICT_VALUE_CREATED"
	dictValueEventLabelCorrected = "DICT_VALUE_LABEL_CORRECTED"
	dictValueEventDisabled       = "DICT_VALUE_DISABLED"

	dictRegistryEventCreated  = "DICT_CREATED"
	dictRegistryEventDisabled = "DICT_DISABLED"
)

var (
	errDictCodeRequired          = errors.New("DICT_CODE_REQUIRED")
	errDictCodeInvalid           = errors.New("DICT_CODE_INVALID")
	errDictNotFound              = errors.New("DICT_NOT_FOUND")
	errDictNameRequired          = errors.New("DICT_NAME_REQUIRED")
	errDictCodeConflict          = errors.New("DICT_CODE_CONFLICT")
	errDictDisabled              = errors.New("DICT_DISABLED")
	errDictDisabledOnRequired    = errors.New("DICT_DISABLED_ON_REQUIRED")
	errDictValueCodeRequired     = errors.New("DICT_VALUE_CODE_REQUIRED")
	errDictValueLabelRequired    = errors.New("DICT_VALUE_LABEL_REQUIRED")
	errDictValueNotFoundAsOf     = errors.New("DICT_VALUE_NOT_FOUND_AS_OF")
	errDictValueConflict         = errors.New("DICT_VALUE_CONFLICT")
	errDictValueDictDisabled     = errors.New("DICT_VALUE_DICT_DISABLED")
	errDictBaselineNotReady      = errors.New("DICT_BASELINE_NOT_READY")
	errDictRequestIDRequired     = errors.New("DICT_REQUEST_CODE_REQUIRED")
	errDictEffectiveDayRequired  = errors.New("DICT_EFFECTIVE_DAY_REQUIRED")
	errDictDisabledOnInvalidDate = errors.New("DICT_DISABLED_ON_INVALID")
)

type DictStore interface {
	dictpkg.Resolver
	ListDicts(ctx context.Context, tenantID string, asOf string) ([]DictItem, error)
	CreateDict(ctx context.Context, tenantID string, req DictCreateRequest) (DictItem, bool, error)
	DisableDict(ctx context.Context, tenantID string, req DictDisableRequest) (DictItem, bool, error)
	ListDictValues(ctx context.Context, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error)
	CreateDictValue(ctx context.Context, tenantID string, req DictCreateValueRequest) (DictValueItem, bool, error)
	DisableDictValue(ctx context.Context, tenantID string, req DictDisableValueRequest) (DictValueItem, bool, error)
	CorrectDictValue(ctx context.Context, tenantID string, req DictCorrectValueRequest) (DictValueItem, bool, error)
	ListDictValueAudit(ctx context.Context, tenantID string, dictCode string, code string, limit int) ([]DictValueAuditItem, error)
}

type DictItem struct {
	DictCode   string  `json:"dict_code"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	EnabledOn  string  `json:"enabled_on"`
	DisabledOn *string `json:"disabled_on"`
}

type DictValueItem struct {
	DictCode   string    `json:"dict_code"`
	Code       string    `json:"code"`
	Label      string    `json:"label"`
	Status     string    `json:"status"`
	EnabledOn  string    `json:"enabled_on"`
	DisabledOn *string   `json:"disabled_on"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type DictValueAuditItem struct {
	EventID        int64           `json:"event_id"`
	EventUUID      string          `json:"event_uuid"`
	DictCode       string          `json:"dict_code"`
	Code           string          `json:"code"`
	EventType      string          `json:"event_type"`
	EffectiveDay   string          `json:"effective_day"`
	RequestID      string          `json:"request_id"`
	InitiatorUUID  string          `json:"initiator_uuid"`
	TxTime         time.Time       `json:"tx_time"`
	Payload        json.RawMessage `json:"payload"`
	BeforeSnapshot json.RawMessage `json:"before_snapshot"`
	AfterSnapshot  json.RawMessage `json:"after_snapshot"`
}

type DictCreateRequest struct {
	DictCode  string
	Name      string
	EnabledOn string
	RequestID string
	Initiator string
}

type DictDisableRequest struct {
	DictCode   string
	DisabledOn string
	RequestID  string
	Initiator  string
}

type DictCreateValueRequest struct {
	DictCode  string
	Code      string
	Label     string
	EnabledOn string
	RequestID string
	Initiator string
}

type DictDisableValueRequest struct {
	DictCode   string
	Code       string
	DisabledOn string
	RequestID  string
	Initiator  string
}

type DictCorrectValueRequest struct {
	DictCode      string
	Code          string
	Label         string
	CorrectionDay string
	RequestID     string
	Initiator     string
}

type dictPGStore struct {
	pool pgBeginner
}

func newDictPGStore(pool pgBeginner) DictStore {
	return &dictPGStore{pool: pool}
}

func (s *dictPGStore) ListDicts(ctx context.Context, tenantID string, asOf string) ([]DictItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	items, err := listTenantDictsByAsOfTx(ctx, tx, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func listTenantDictsByAsOfTx(ctx context.Context, tx pgx.Tx, tenantID string, asOf string) ([]DictItem, error) {
	rows, err := tx.Query(ctx, `
SELECT dict_code, name, status, enabled_on::text, CASE WHEN disabled_on IS NULL THEN NULL ELSE disabled_on::text END AS disabled_on
FROM (
  SELECT
    d.dict_code,
    d.name,
    d.enabled_on,
    d.disabled_on,
    CASE
      WHEN d.enabled_on <= $2::date AND (d.disabled_on IS NULL OR $2::date < d.disabled_on) THEN 'active'
      ELSE 'inactive'
    END AS status,
    row_number() OVER (PARTITION BY d.dict_code ORDER BY d.enabled_on DESC) AS rn
  FROM iam.dicts d
  WHERE d.tenant_uuid = $1::uuid
    AND d.enabled_on <= $2::date
    AND (d.disabled_on IS NULL OR $2::date < d.disabled_on)
) merged
WHERE rn = 1
ORDER BY dict_code ASC
`, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DictItem, 0)
	for rows.Next() {
		var item DictItem
		var disabledOn *string
		if err := rows.Scan(&item.DictCode, &item.Name, &item.Status, &item.EnabledOn, &disabledOn); err != nil {
			return nil, err
		}
		item.DisabledOn = cloneOptionalString(disabledOn)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *dictPGStore) CreateDict(ctx context.Context, tenantID string, req DictCreateRequest) (DictItem, bool, error) {
	return s.submitDictEvent(ctx, tenantID, req.DictCode, dictRegistryEventCreated, req.EnabledOn, map[string]any{"name": req.Name}, req.RequestID, req.Initiator)
}

func (s *dictPGStore) DisableDict(ctx context.Context, tenantID string, req DictDisableRequest) (DictItem, bool, error) {
	return s.submitDictEvent(ctx, tenantID, req.DictCode, dictRegistryEventDisabled, req.DisabledOn, map[string]any{}, req.RequestID, req.Initiator)
}

func (s *dictPGStore) submitDictEvent(
	ctx context.Context,
	tenantID string,
	dictCode string,
	eventType string,
	day string,
	payload map[string]any,
	requestID string,
	initiator string,
) (DictItem, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DictItem{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return DictItem{}, false, err
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return DictItem{}, false, err
	}

	var eventID int64
	var wasRetry bool
	err = tx.QueryRow(ctx, `
SELECT event_id, was_retry
FROM iam.submit_dict_event($1::uuid, $2::text, $3::text, $4::date, $5::jsonb, $6::text, $7::uuid)
`, tenantID, dictCode, eventType, day, rawPayload, requestID, initiator).Scan(&eventID, &wasRetry)
	if err != nil {
		return DictItem{}, false, err
	}

	item, err := getDictFromEventTx(ctx, tx, tenantID, eventID)
	if err != nil {
		return DictItem{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return DictItem{}, false, err
	}
	return item, wasRetry, nil
}

func getDictFromEventTx(ctx context.Context, tx pgx.Tx, tenantID string, eventID int64) (DictItem, error) {
	var snapshot []byte
	err := tx.QueryRow(ctx, `
SELECT after_snapshot
FROM iam.dict_events
WHERE tenant_uuid = $1::uuid
  AND id = $2::bigint
`, tenantID, eventID).Scan(&snapshot)
	if err != nil {
		return DictItem{}, err
	}

	var payload struct {
		DictCode   string  `json:"dict_code"`
		Name       string  `json:"name"`
		Status     string  `json:"status"`
		EnabledOn  string  `json:"enabled_on"`
		DisabledOn *string `json:"disabled_on"`
	}
	if err := json.Unmarshal(snapshot, &payload); err != nil {
		return DictItem{}, err
	}
	return DictItem{
		DictCode:   payload.DictCode,
		Name:       payload.Name,
		Status:     payload.Status,
		EnabledOn:  payload.EnabledOn,
		DisabledOn: cloneOptionalString(payload.DisabledOn),
	}, nil
}

func (s *dictPGStore) ListDictValues(ctx context.Context, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	sourceTenantID, err := resolveDictSourceTenantAsOfTx(ctx, tx, tenantID, dictCode, asOf)
	if err != nil {
		return nil, err
	}

	values, err := listDictValuesByTenant(ctx, tx, sourceTenantID, dictCode, asOf, keyword, limit, status)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return values, nil
}

func assertTenantBaselineReadyTx(ctx context.Context, tx pgx.Tx, tenantID string) error {
	var ready bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1
  FROM iam.dicts
  WHERE tenant_uuid = $1::uuid
    AND dict_code = $2::text
)
`, tenantID, dictCodeOrgType).Scan(&ready)
	if err != nil {
		return err
	}
	if !ready {
		return errDictBaselineNotReady
	}
	return nil
}

// 兼容历史调用点：运行时 tenant-only，命中时始终返回当前租户。
func resolveDictSourceTenantAsOfTx(ctx context.Context, tx pgx.Tx, tenantID string, dictCode string, asOf string) (string, error) {
	var sourceTenant string
	err := tx.QueryRow(ctx, `
SELECT tenant_uuid::text
FROM iam.dicts
WHERE tenant_uuid = $1::uuid
  AND dict_code = $2::text
  AND enabled_on <= $3::date
  AND (disabled_on IS NULL OR $3::date < disabled_on)
LIMIT 1
`, tenantID, dictCode, asOf).Scan(&sourceTenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errDictNotFound
		}
		return "", err
	}
	return sourceTenant, nil
}

// 兼容历史调用点：运行时 tenant-only，命中时始终返回当前租户。
func resolveDictSourceTenantTx(ctx context.Context, tx pgx.Tx, tenantID string, dictCode string) (string, error) {
	var sourceTenant string
	err := tx.QueryRow(ctx, `
SELECT tenant_uuid::text
FROM iam.dicts
WHERE tenant_uuid = $1::uuid
  AND dict_code = $2::text
LIMIT 1
`, tenantID, dictCode).Scan(&sourceTenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errDictNotFound
		}
		return "", err
	}
	return sourceTenant, nil
}

func listDictValuesByTenant(ctx context.Context, tx pgx.Tx, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error) {
	rows, err := tx.Query(ctx, `
SELECT
  dict_code,
  code,
  label,
  CASE
    WHEN enabled_on <= $3::date AND (disabled_on IS NULL OR $3::date < disabled_on) THEN 'active'
    ELSE 'inactive'
  END AS current_status,
  enabled_on::text,
  CASE WHEN disabled_on IS NULL THEN NULL ELSE disabled_on::text END AS disabled_on,
  updated_at
FROM iam.dict_value_segments
WHERE tenant_uuid = $1::uuid
  AND dict_code = $2::text
  AND ($4::text = '' OR code ILIKE ('%' || $4::text || '%') OR label ILIKE ('%' || $4::text || '%'))
  AND (
    $5::text = 'all'
    OR ($5::text = 'active' AND enabled_on <= $3::date AND (disabled_on IS NULL OR $3::date < disabled_on))
    OR ($5::text = 'inactive' AND NOT (enabled_on <= $3::date AND (disabled_on IS NULL OR $3::date < disabled_on)))
  )
ORDER BY code ASC, enabled_on DESC
LIMIT $6::int
`, tenantID, dictCode, asOf, keyword, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DictValueItem, 0)
	for rows.Next() {
		var item DictValueItem
		var disabled *string
		if err := rows.Scan(&item.DictCode, &item.Code, &item.Label, &item.Status, &item.EnabledOn, &disabled, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.DisabledOn = cloneOptionalString(disabled)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *dictPGStore) ResolveValueLabel(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", false, err
	}

	sourceTenantID, err := resolveDictSourceTenantAsOfTx(ctx, tx, tenantID, dictCode, asOf)
	if err != nil {
		if errors.Is(err, errDictNotFound) {
			return "", false, nil
		}
		return "", false, err
	}

	label, ok, err := resolveValueLabelByTenant(ctx, tx, sourceTenantID, asOf, dictCode, code)
	if err != nil {
		return "", false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", false, err
	}
	return label, ok, nil
}

func resolveValueLabelByTenant(ctx context.Context, tx pgx.Tx, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	var label string
	err := tx.QueryRow(ctx, `
SELECT label
FROM iam.dict_value_segments
WHERE tenant_uuid = $1::uuid
  AND dict_code = $2::text
  AND code = $3::text
  AND enabled_on <= $4::date
  AND (disabled_on IS NULL OR $4::date < disabled_on)
ORDER BY enabled_on DESC
LIMIT 1
`, tenantID, dictCode, code, asOf).Scan(&label)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return label, true, nil
}

func (s *dictPGStore) ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
	values, err := s.ListDictValues(ctx, tenantID, dictCode, asOf, keyword, limit, "active")
	if err != nil {
		return nil, err
	}
	out := make([]dictpkg.Option, 0, len(values))
	for _, item := range values {
		out = append(out, dictpkg.Option{
			Code:       item.Code,
			Label:      item.Label,
			Status:     item.Status,
			EnabledOn:  item.EnabledOn,
			DisabledOn: cloneOptionalString(item.DisabledOn),
			UpdatedAt:  item.UpdatedAt,
		})
	}
	return out, nil
}

func (s *dictPGStore) CreateDictValue(ctx context.Context, tenantID string, req DictCreateValueRequest) (DictValueItem, bool, error) {
	return s.submitValueEvent(ctx, tenantID, req.DictCode, req.Code, dictValueEventCreated, req.EnabledOn, map[string]any{"label": req.Label}, req.RequestID, req.Initiator)
}

func (s *dictPGStore) DisableDictValue(ctx context.Context, tenantID string, req DictDisableValueRequest) (DictValueItem, bool, error) {
	return s.submitValueEvent(ctx, tenantID, req.DictCode, req.Code, dictValueEventDisabled, req.DisabledOn, map[string]any{}, req.RequestID, req.Initiator)
}

func (s *dictPGStore) CorrectDictValue(ctx context.Context, tenantID string, req DictCorrectValueRequest) (DictValueItem, bool, error) {
	return s.submitValueEvent(ctx, tenantID, req.DictCode, req.Code, dictValueEventLabelCorrected, req.CorrectionDay, map[string]any{"label": req.Label}, req.RequestID, req.Initiator)
}

func (s *dictPGStore) submitValueEvent(ctx context.Context, tenantID string, dictCode string, code string, eventType string, day string, payload map[string]any, requestID string, initiator string) (DictValueItem, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DictValueItem{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return DictValueItem{}, false, err
	}
	if err := assertTenantBaselineReadyTx(ctx, tx, tenantID); err != nil {
		return DictValueItem{}, false, err
	}

	if err := assertTenantDictActiveAsOfTx(ctx, tx, tenantID, dictCode, day); err != nil {
		return DictValueItem{}, false, err
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return DictValueItem{}, false, err
	}

	var eventID int64
	var wasRetry bool
	err = tx.QueryRow(ctx, `
SELECT event_id, was_retry
FROM iam.submit_dict_value_event($1::uuid, $2::text, $3::text, $4::text, $5::date, $6::jsonb, $7::text, $8::uuid)
`, tenantID, dictCode, code, eventType, day, rawPayload, requestID, initiator).Scan(&eventID, &wasRetry)
	if err != nil {
		return DictValueItem{}, false, err
	}

	item, err := getDictValueFromEventTx(ctx, tx, tenantID, eventID)
	if err != nil {
		return DictValueItem{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DictValueItem{}, false, err
	}
	return item, wasRetry, nil
}

func assertTenantDictActiveAsOfTx(ctx context.Context, tx pgx.Tx, tenantID string, dictCode string, asOf string) error {
	var active bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM iam.dicts
  WHERE tenant_uuid = $1::uuid
    AND dict_code = $2::text
    AND enabled_on <= $3::date
    AND (disabled_on IS NULL OR $3::date < disabled_on)
)
`, tenantID, dictCode, asOf).Scan(&active)
	if err != nil {
		return err
	}
	if active {
		return nil
	}

	var existsAny bool
	err = tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM iam.dicts
  WHERE tenant_uuid = $1::uuid
    AND dict_code = $2::text
)
`, tenantID, dictCode).Scan(&existsAny)
	if err != nil {
		return err
	}
	if existsAny {
		return errDictValueDictDisabled
	}
	return errDictNotFound
}

func getDictValueFromEventTx(ctx context.Context, tx pgx.Tx, tenantID string, eventID int64) (DictValueItem, error) {
	var snapshot []byte
	var txTime time.Time
	err := tx.QueryRow(ctx, `
SELECT after_snapshot, tx_time
FROM iam.dict_value_events
WHERE tenant_uuid = $1::uuid
  AND id = $2::bigint
`, tenantID, eventID).Scan(&snapshot, &txTime)
	if err != nil {
		return DictValueItem{}, err
	}
	var payload struct {
		DictCode   string  `json:"dict_code"`
		Code       string  `json:"code"`
		Label      string  `json:"label"`
		Status     string  `json:"status"`
		EnabledOn  string  `json:"enabled_on"`
		DisabledOn *string `json:"disabled_on"`
	}
	if err := json.Unmarshal(snapshot, &payload); err != nil {
		return DictValueItem{}, err
	}
	return DictValueItem{
		DictCode:   payload.DictCode,
		Code:       payload.Code,
		Label:      payload.Label,
		Status:     payload.Status,
		EnabledOn:  payload.EnabledOn,
		DisabledOn: cloneOptionalString(payload.DisabledOn),
		UpdatedAt:  txTime,
	}, nil
}

func (s *dictPGStore) ListDictValueAudit(ctx context.Context, tenantID string, dictCode string, code string, limit int) ([]DictValueAuditItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	sourceTenantID, err := resolveDictSourceTenantTx(ctx, tx, tenantID, dictCode)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
SELECT id, event_uuid::text, dict_code, code, event_type, effective_day::text, request_id, COALESCE(initiator_uuid::text, ''), tx_time, payload, before_snapshot, after_snapshot
FROM iam.dict_value_events
WHERE tenant_uuid = $1::uuid
  AND dict_code = $2::text
  AND code = $3::text
ORDER BY id DESC
LIMIT $4::int
	`, sourceTenantID, dictCode, code, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DictValueAuditItem, 0)
	for rows.Next() {
		var item DictValueAuditItem
		if err := rows.Scan(
			&item.EventID, &item.EventUUID, &item.DictCode, &item.Code, &item.EventType, &item.EffectiveDay, &item.RequestID,
			&item.InitiatorUUID, &item.TxTime, &item.Payload, &item.BeforeSnapshot, &item.AfterSnapshot,
		); err != nil {
			return nil, err
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

type dictMemoryStore struct {
	dicts  map[string]map[string]DictItem
	values map[string][]DictValueItem
}

func newDictMemoryStore() DictStore {
	now := time.Unix(0, 0).UTC()
	defaultDict := DictItem{DictCode: dictCodeOrgType, Name: "Org Type", Status: "active", EnabledOn: "1970-01-01"}
	defaultValues := []DictValueItem{
		{DictCode: dictCodeOrgType, Code: "10", Label: "部门", Status: "active", EnabledOn: "1970-01-01", UpdatedAt: now},
		{DictCode: dictCodeOrgType, Code: "20", Label: "单位", Status: "active", EnabledOn: "1970-01-01", UpdatedAt: now},
	}
	return &dictMemoryStore{
		dicts: map[string]map[string]DictItem{
			globalTenantID:                         {dictCodeOrgType: defaultDict},
			"00000000-0000-0000-0000-000000000001": {dictCodeOrgType: defaultDict},
			"t1":                                   {dictCodeOrgType: defaultDict},
		},
		values: map[string][]DictValueItem{
			globalTenantID:                         append([]DictValueItem(nil), defaultValues...),
			"00000000-0000-0000-0000-000000000001": append([]DictValueItem(nil), defaultValues...),
			"t1":                                   append([]DictValueItem(nil), defaultValues...),
		},
	}
}

func (s *dictMemoryStore) ListDicts(_ context.Context, tenantID string, asOf string) ([]DictItem, error) {
	if strings.TrimSpace(asOf) == "" {
		return nil, errDictEffectiveDayRequired
	}
	tenantDicts := s.dictsByTenant(tenantID)

	out := make([]DictItem, 0)
	for _, item := range tenantDicts {
		if !dictActiveAsOf(item, asOf) {
			continue
		}
		cloned := cloneDictItem(item)
		cloned.Status = dictStatusAsOf(item, asOf)
		out = append(out, cloned)
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].DictCode < out[j].DictCode })
	return out, nil
}

func (s *dictMemoryStore) CreateDict(_ context.Context, tenantID string, req DictCreateRequest) (DictItem, bool, error) {
	code := strings.TrimSpace(strings.ToLower(req.DictCode))
	if code == "" {
		return DictItem{}, false, errDictCodeRequired
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return DictItem{}, false, errDictNameRequired
	}
	enabledOn := strings.TrimSpace(req.EnabledOn)
	if enabledOn == "" {
		return DictItem{}, false, errDictEffectiveDayRequired
	}
	if _, ok := s.dicts[tenantID]; !ok {
		s.dicts[tenantID] = map[string]DictItem{}
	}
	if _, exists := s.dicts[tenantID][code]; exists {
		return DictItem{}, false, errDictCodeConflict
	}
	item := DictItem{DictCode: code, Name: name, Status: "active", EnabledOn: enabledOn}
	s.dicts[tenantID][code] = item
	return item, false, nil
}

func (s *dictMemoryStore) DisableDict(_ context.Context, tenantID string, req DictDisableRequest) (DictItem, bool, error) {
	code := strings.TrimSpace(strings.ToLower(req.DictCode))
	if code == "" {
		return DictItem{}, false, errDictCodeRequired
	}
	disabledOn := strings.TrimSpace(req.DisabledOn)
	if disabledOn == "" {
		return DictItem{}, false, errDictDisabledOnRequired
	}
	items, ok := s.dicts[tenantID]
	if !ok {
		return DictItem{}, false, errDictNotFound
	}
	item, ok := items[code]
	if !ok {
		return DictItem{}, false, errDictNotFound
	}
	if disabledOn <= item.EnabledOn {
		return DictItem{}, false, errDictCodeConflict
	}
	if item.DisabledOn != nil && disabledOn >= *item.DisabledOn {
		return DictItem{}, false, errDictCodeConflict
	}
	item.DisabledOn = cloneOptionalString(&disabledOn)
	item.Status = "inactive"
	items[code] = item
	return cloneDictItem(item), false, nil
}

func (s *dictMemoryStore) ListDictValues(_ context.Context, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error) {
	sourceTenantID, ok := s.resolveSourceTenantAsOf(tenantID, dictCode, asOf)
	if !ok {
		return nil, errDictNotFound
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		status = "all"
	}

	out := make([]DictValueItem, 0)
	for _, item := range s.valuesForTenant(sourceTenantID) {
		if item.DictCode != dictCode {
			continue
		}
		currentStatus := valueStatusAsOf(item, asOf)
		if status != "all" && currentStatus != status {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.Code), keyword) && !strings.Contains(strings.ToLower(item.Label), keyword) {
			continue
		}
		cloned := item
		cloned.Status = currentStatus
		out = append(out, cloned)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Code == out[j].Code {
			return out[i].EnabledOn > out[j].EnabledOn
		}
		return out[i].Code < out[j].Code
	})
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *dictMemoryStore) ResolveValueLabel(_ context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	sourceTenantID, ok := s.resolveSourceTenantAsOf(tenantID, dictCode, asOf)
	if !ok {
		return "", false, nil
	}
	for _, item := range s.valuesForTenant(sourceTenantID) {
		if item.DictCode == dictCode && item.Code == code && valueStatusAsOf(item, asOf) == "active" {
			return item.Label, true, nil
		}
	}
	return "", false, nil
}

func (s *dictMemoryStore) ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
	values, err := s.ListDictValues(ctx, tenantID, dictCode, asOf, keyword, limit, "active")
	if err != nil {
		return nil, err
	}
	out := make([]dictpkg.Option, 0, len(values))
	for _, item := range values {
		out = append(out, dictpkg.Option{Code: item.Code, Label: item.Label, Status: item.Status, EnabledOn: item.EnabledOn, DisabledOn: item.DisabledOn, UpdatedAt: item.UpdatedAt})
	}
	return out, nil
}

func (s *dictMemoryStore) CreateDictValue(context.Context, string, DictCreateValueRequest) (DictValueItem, bool, error) {
	return DictValueItem{}, false, errDictValueConflict
}

func (s *dictMemoryStore) DisableDictValue(context.Context, string, DictDisableValueRequest) (DictValueItem, bool, error) {
	return DictValueItem{}, false, errDictValueConflict
}

func (s *dictMemoryStore) CorrectDictValue(context.Context, string, DictCorrectValueRequest) (DictValueItem, bool, error) {
	return DictValueItem{}, false, errDictValueConflict
}

func (s *dictMemoryStore) ListDictValueAudit(_ context.Context, tenantID string, dictCode string, code string, _ int) ([]DictValueAuditItem, error) {
	sourceTenantID, ok := s.resolveSourceTenant(tenantID, dictCode)
	if !ok {
		return nil, errDictNotFound
	}
	if sourceTenantID == "" || code == "" {
		return []DictValueAuditItem{}, nil
	}
	return []DictValueAuditItem{}, nil
}

func (s *dictMemoryStore) dictExists(tenantID string, dictCode string) bool {
	if _, ok := s.dicts[tenantID][dictCode]; ok {
		return true
	}
	return false
}

func (s *dictMemoryStore) dictExistsAsOf(tenantID string, dictCode string, asOf string) bool {
	if item, ok := s.dicts[tenantID][dictCode]; ok {
		return dictActiveAsOf(item, asOf)
	}
	return false
}

// 兼容历史测试：运行时 tenant-only，不再回退 global。
func (s *dictMemoryStore) resolveSourceTenantAsOf(tenantID string, dictCode string, asOf string) (string, bool) {
	if s.dictExistsAsOf(tenantID, dictCode, asOf) {
		return tenantID, true
	}
	return "", false
}

// 兼容历史测试：运行时 tenant-only，不再回退 global。
func (s *dictMemoryStore) resolveSourceTenant(tenantID string, dictCode string) (string, bool) {
	if s.dictExists(tenantID, dictCode) {
		return tenantID, true
	}
	return "", false
}

func (s *dictMemoryStore) valuesForTenant(tenantID string) []DictValueItem {
	if items, ok := s.values[tenantID]; ok {
		return items
	}
	return nil
}

func (s *dictMemoryStore) dictsByTenant(tenantID string) map[string]DictItem {
	items := map[string]DictItem{}
	for code, item := range s.dicts[tenantID] {
		items[code] = cloneDictItem(item)
	}
	return items
}

func cloneDictItem(item DictItem) DictItem {
	item.DisabledOn = cloneOptionalString(item.DisabledOn)
	return item
}

func dictActiveAsOf(item DictItem, asOf string) bool {
	if item.EnabledOn > asOf {
		return false
	}
	if item.DisabledOn != nil && asOf >= *item.DisabledOn {
		return false
	}
	return true
}

func dictStatusAsOf(item DictItem, asOf string) string {
	if dictActiveAsOf(item, asOf) {
		return "active"
	}
	return "inactive"
}

func valueStatusAsOf(item DictValueItem, asOf string) string {
	if item.EnabledOn <= asOf && (item.DisabledOn == nil || asOf < *item.DisabledOn) {
		return "active"
	}
	return "inactive"
}

func dictDisplayName(dictCode string) string {
	switch strings.TrimSpace(strings.ToLower(dictCode)) {
	case dictCodeOrgType:
		return "Org Type"
	default:
		return dictCode
	}
}
