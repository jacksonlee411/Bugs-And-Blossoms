package persistence

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
	DictCodeOrgType = "org_type"
	// 保留该常量用于历史测试数据与迁移工具识别；运行时读取已收敛为 tenant-only。
	GlobalTenantID = "00000000-0000-0000-0000-000000000000"

	DictEventCreated        = "DICT_VALUE_CREATED"
	DictEventLabelCorrected = "DICT_VALUE_LABEL_CORRECTED"
	DictEventDisabled       = "DICT_VALUE_DISABLED"

	DictValueEventCreated        = "DICT_VALUE_CREATED"
	DictValueEventLabelCorrected = "DICT_VALUE_LABEL_CORRECTED"
	DictValueEventDisabled       = "DICT_VALUE_DISABLED"

	DictRegistryEventCreated  = "DICT_CREATED"
	DictRegistryEventDisabled = "DICT_DISABLED"

	DictOptionSetIDDeflt       = "DEFLT"
	DictOptionSetIDSourceDeflt = "deflt"
)

var (
	ErrDictCodeRequired          = errors.New("DICT_CODE_REQUIRED")
	ErrDictCodeInvalid           = errors.New("DICT_CODE_INVALID")
	ErrDictNotFound              = errors.New("DICT_NOT_FOUND")
	ErrDictNameRequired          = errors.New("DICT_NAME_REQUIRED")
	ErrDictCodeConflict          = errors.New("DICT_CODE_CONFLICT")
	ErrDictDisabled              = errors.New("DICT_DISABLED")
	ErrDictDisabledOnRequired    = errors.New("DICT_DISABLED_ON_REQUIRED")
	ErrDictValueCodeRequired     = errors.New("DICT_VALUE_CODE_REQUIRED")
	ErrDictValueLabelRequired    = errors.New("DICT_VALUE_LABEL_REQUIRED")
	ErrDictValueNotFoundAsOf     = errors.New("DICT_VALUE_NOT_FOUND_AS_OF")
	ErrDictValueConflict         = errors.New("DICT_VALUE_CONFLICT")
	ErrDictValueDictDisabled     = errors.New("DICT_VALUE_DICT_DISABLED")
	ErrDictBaselineNotReady      = errors.New("DICT_BASELINE_NOT_READY")
	ErrDictRequestIDRequired     = errors.New("DICT_REQUEST_CODE_REQUIRED")
	ErrDictEffectiveDayRequired  = errors.New("DICT_EFFECTIVE_DAY_REQUIRED")
	ErrDictDisabledOnInvalidDate = errors.New("DICT_DISABLED_ON_INVALID")
)

type PGBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

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
	DictCode    string    `json:"dict_code"`
	Code        string    `json:"code"`
	Label       string    `json:"label"`
	SetID       string    `json:"setid,omitempty"`
	SetIDSource string    `json:"setid_source,omitempty"`
	Status      string    `json:"status"`
	EnabledOn   string    `json:"enabled_on"`
	DisabledOn  *string   `json:"disabled_on"`
	UpdatedAt   time.Time `json:"updated_at"`
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

type PGStore struct {
	Pool PGBeginner
}

func NewPGStore(pool PGBeginner) *PGStore {
	return &PGStore{Pool: pool}
}

func (s *PGStore) ListDicts(ctx context.Context, tenantID string, asOf string) ([]DictItem, error) {
	tx, err := s.Pool.Begin(ctx)
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
		item.DisabledOn = CloneOptionalString(disabledOn)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PGStore) CreateDict(ctx context.Context, tenantID string, req DictCreateRequest) (DictItem, bool, error) {
	return SubmitDictEvent(ctx, s.Pool, tenantID, req.DictCode, DictRegistryEventCreated, req.EnabledOn, map[string]any{"name": req.Name}, req.RequestID, req.Initiator)
}

func (s *PGStore) DisableDict(ctx context.Context, tenantID string, req DictDisableRequest) (DictItem, bool, error) {
	return SubmitDictEvent(ctx, s.Pool, tenantID, req.DictCode, DictRegistryEventDisabled, req.DisabledOn, map[string]any{}, req.RequestID, req.Initiator)
}

func SubmitDictEvent(
	ctx context.Context,
	pool PGBeginner,
	tenantID string,
	dictCode string,
	eventType string,
	day string,
	payload map[string]any,
	requestID string,
	initiator string,
) (DictItem, bool, error) {
	tx, err := pool.Begin(ctx)
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

	item, err := GetDictFromEventTx(ctx, tx, tenantID, eventID)
	if err != nil {
		return DictItem{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return DictItem{}, false, err
	}
	return item, wasRetry, nil
}

func GetDictFromEventTx(ctx context.Context, tx pgx.Tx, tenantID string, eventID int64) (DictItem, error) {
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
		DisabledOn: CloneOptionalString(payload.DisabledOn),
	}, nil
}

func (s *PGStore) ListDictValues(ctx context.Context, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	sourceTenantID, err := ResolveDictSourceTenantAsOfTx(ctx, tx, tenantID, dictCode, asOf)
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

func AssertTenantBaselineReadyTx(ctx context.Context, tx pgx.Tx, tenantID string) error {
	var ready bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1
  FROM iam.dicts
  WHERE tenant_uuid = $1::uuid
    AND dict_code = $2::text
)
`, tenantID, DictCodeOrgType).Scan(&ready)
	if err != nil {
		return err
	}
	if !ready {
		return ErrDictBaselineNotReady
	}
	return nil
}

// 兼容历史调用点：运行时 tenant-only，命中时始终返回当前租户。
func ResolveDictSourceTenantAsOfTx(ctx context.Context, tx pgx.Tx, tenantID string, dictCode string, asOf string) (string, error) {
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
			return "", ErrDictNotFound
		}
		return "", err
	}
	return sourceTenant, nil
}

// 兼容历史调用点：运行时 tenant-only，命中时始终返回当前租户。
func ResolveDictSourceTenantTx(ctx context.Context, tx pgx.Tx, tenantID string, dictCode string) (string, error) {
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
			return "", ErrDictNotFound
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
		item.DisabledOn = CloneOptionalString(disabled)
		item.SetID = DictOptionSetIDDeflt
		item.SetIDSource = DictOptionSetIDSourceDeflt
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PGStore) ResolveValueLabel(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", false, err
	}

	sourceTenantID, err := ResolveDictSourceTenantAsOfTx(ctx, tx, tenantID, dictCode, asOf)
	if err != nil {
		if errors.Is(err, ErrDictNotFound) {
			return "", false, nil
		}
		return "", false, err
	}

	label, ok, err := ResolveValueLabelByTenant(ctx, tx, sourceTenantID, asOf, dictCode, code)
	if err != nil {
		return "", false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", false, err
	}
	return label, ok, nil
}

func ResolveValueLabelByTenant(ctx context.Context, tx pgx.Tx, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
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

func (s *PGStore) ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
	values, err := s.ListDictValues(ctx, tenantID, dictCode, asOf, keyword, limit, "active")
	if err != nil {
		return nil, err
	}
	out := make([]dictpkg.Option, 0, len(values))
	for _, item := range values {
		out = append(out, dictpkg.Option{
			Code:        item.Code,
			Label:       item.Label,
			SetID:       DictOptionSetIDDeflt,
			SetIDSource: DictOptionSetIDSourceDeflt,
			Status:      item.Status,
			EnabledOn:   item.EnabledOn,
			DisabledOn:  CloneOptionalString(item.DisabledOn),
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return out, nil
}

func (s *PGStore) CreateDictValue(ctx context.Context, tenantID string, req DictCreateValueRequest) (DictValueItem, bool, error) {
	return SubmitValueEvent(ctx, s.Pool, tenantID, req.DictCode, req.Code, DictValueEventCreated, req.EnabledOn, map[string]any{"label": req.Label}, req.RequestID, req.Initiator)
}

func (s *PGStore) DisableDictValue(ctx context.Context, tenantID string, req DictDisableValueRequest) (DictValueItem, bool, error) {
	return SubmitValueEvent(ctx, s.Pool, tenantID, req.DictCode, req.Code, DictValueEventDisabled, req.DisabledOn, map[string]any{}, req.RequestID, req.Initiator)
}

func (s *PGStore) CorrectDictValue(ctx context.Context, tenantID string, req DictCorrectValueRequest) (DictValueItem, bool, error) {
	return SubmitValueEvent(ctx, s.Pool, tenantID, req.DictCode, req.Code, DictValueEventLabelCorrected, req.CorrectionDay, map[string]any{"label": req.Label}, req.RequestID, req.Initiator)
}

func SubmitValueEvent(ctx context.Context, pool PGBeginner, tenantID string, dictCode string, code string, eventType string, day string, payload map[string]any, requestID string, initiator string) (DictValueItem, bool, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return DictValueItem{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return DictValueItem{}, false, err
	}
	if err := AssertTenantBaselineReadyTx(ctx, tx, tenantID); err != nil {
		return DictValueItem{}, false, err
	}

	if err := AssertTenantDictActiveAsOfTx(ctx, tx, tenantID, dictCode, day); err != nil {
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

	item, err := GetDictValueFromEventTx(ctx, tx, tenantID, eventID)
	if err != nil {
		return DictValueItem{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DictValueItem{}, false, err
	}
	return item, wasRetry, nil
}

func AssertTenantDictActiveAsOfTx(ctx context.Context, tx pgx.Tx, tenantID string, dictCode string, asOf string) error {
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
		return ErrDictValueDictDisabled
	}
	return ErrDictNotFound
}

func GetDictValueFromEventTx(ctx context.Context, tx pgx.Tx, tenantID string, eventID int64) (DictValueItem, error) {
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
		DictCode:    payload.DictCode,
		Code:        payload.Code,
		Label:       payload.Label,
		SetID:       DictOptionSetIDDeflt,
		SetIDSource: DictOptionSetIDSourceDeflt,
		Status:      payload.Status,
		EnabledOn:   payload.EnabledOn,
		DisabledOn:  CloneOptionalString(payload.DisabledOn),
		UpdatedAt:   txTime,
	}, nil
}

func (s *PGStore) ListDictValueAudit(ctx context.Context, tenantID string, dictCode string, code string, limit int) ([]DictValueAuditItem, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	sourceTenantID, err := ResolveDictSourceTenantTx(ctx, tx, tenantID, dictCode)
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

type MemoryStore struct {
	Dicts  map[string]map[string]DictItem
	Values map[string][]DictValueItem
}

func NewMemoryStore() *MemoryStore {
	now := time.Unix(0, 0).UTC()
	defaultDict := DictItem{DictCode: DictCodeOrgType, Name: "Org Type", Status: "active", EnabledOn: "1970-01-01"}
	defaultValues := []DictValueItem{
		{DictCode: DictCodeOrgType, Code: "10", Label: "部门", SetID: DictOptionSetIDDeflt, SetIDSource: DictOptionSetIDSourceDeflt, Status: "active", EnabledOn: "1970-01-01", UpdatedAt: now},
		{DictCode: DictCodeOrgType, Code: "20", Label: "单位", SetID: DictOptionSetIDDeflt, SetIDSource: DictOptionSetIDSourceDeflt, Status: "active", EnabledOn: "1970-01-01", UpdatedAt: now},
	}
	return &MemoryStore{
		Dicts: map[string]map[string]DictItem{
			GlobalTenantID:                         {DictCodeOrgType: defaultDict},
			"00000000-0000-0000-0000-000000000001": {DictCodeOrgType: defaultDict},
			"t1":                                   {DictCodeOrgType: defaultDict},
		},
		Values: map[string][]DictValueItem{
			GlobalTenantID:                         append([]DictValueItem(nil), defaultValues...),
			"00000000-0000-0000-0000-000000000001": append([]DictValueItem(nil), defaultValues...),
			"t1":                                   append([]DictValueItem(nil), defaultValues...),
		},
	}
}

func (s *MemoryStore) ListDicts(_ context.Context, tenantID string, asOf string) ([]DictItem, error) {
	if strings.TrimSpace(asOf) == "" {
		return nil, ErrDictEffectiveDayRequired
	}
	tenantDicts := s.dictsByTenant(tenantID)

	out := make([]DictItem, 0)
	for _, item := range tenantDicts {
		if !DictActiveAsOf(item, asOf) {
			continue
		}
		cloned := CloneDictItem(item)
		cloned.Status = DictStatusAsOf(item, asOf)
		out = append(out, cloned)
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].DictCode < out[j].DictCode })
	return out, nil
}

func (s *MemoryStore) CreateDict(_ context.Context, tenantID string, req DictCreateRequest) (DictItem, bool, error) {
	code := strings.TrimSpace(strings.ToLower(req.DictCode))
	if code == "" {
		return DictItem{}, false, ErrDictCodeRequired
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return DictItem{}, false, ErrDictNameRequired
	}
	enabledOn := strings.TrimSpace(req.EnabledOn)
	if enabledOn == "" {
		return DictItem{}, false, ErrDictEffectiveDayRequired
	}
	if _, ok := s.Dicts[tenantID]; !ok {
		s.Dicts[tenantID] = map[string]DictItem{}
	}
	if _, exists := s.Dicts[tenantID][code]; exists {
		return DictItem{}, false, ErrDictCodeConflict
	}
	item := DictItem{DictCode: code, Name: name, Status: "active", EnabledOn: enabledOn}
	s.Dicts[tenantID][code] = item
	return item, false, nil
}

func (s *MemoryStore) DisableDict(_ context.Context, tenantID string, req DictDisableRequest) (DictItem, bool, error) {
	code := strings.TrimSpace(strings.ToLower(req.DictCode))
	if code == "" {
		return DictItem{}, false, ErrDictCodeRequired
	}
	disabledOn := strings.TrimSpace(req.DisabledOn)
	if disabledOn == "" {
		return DictItem{}, false, ErrDictDisabledOnRequired
	}
	items, ok := s.Dicts[tenantID]
	if !ok {
		return DictItem{}, false, ErrDictNotFound
	}
	item, ok := items[code]
	if !ok {
		return DictItem{}, false, ErrDictNotFound
	}
	if disabledOn <= item.EnabledOn {
		return DictItem{}, false, ErrDictCodeConflict
	}
	if item.DisabledOn != nil && disabledOn >= *item.DisabledOn {
		return DictItem{}, false, ErrDictCodeConflict
	}
	item.DisabledOn = CloneOptionalString(&disabledOn)
	item.Status = "inactive"
	items[code] = item
	return CloneDictItem(item), false, nil
}

func (s *MemoryStore) ListDictValues(_ context.Context, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error) {
	sourceTenantID, ok := s.resolveSourceTenantAsOf(tenantID, dictCode, asOf)
	if !ok {
		return nil, ErrDictNotFound
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
		currentStatus := ValueStatusAsOf(item, asOf)
		if status != "all" && currentStatus != status {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.Code), keyword) && !strings.Contains(strings.ToLower(item.Label), keyword) {
			continue
		}
		cloned := item
		cloned.Status = currentStatus
		if strings.TrimSpace(cloned.SetID) == "" {
			cloned.SetID = DictOptionSetIDDeflt
		}
		if strings.TrimSpace(cloned.SetIDSource) == "" {
			cloned.SetIDSource = DictOptionSetIDSourceDeflt
		}
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

func (s *MemoryStore) ResolveValueLabel(_ context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	sourceTenantID, ok := s.resolveSourceTenantAsOf(tenantID, dictCode, asOf)
	if !ok {
		return "", false, nil
	}
	for _, item := range s.valuesForTenant(sourceTenantID) {
		if item.DictCode == dictCode && item.Code == code && ValueStatusAsOf(item, asOf) == "active" {
			return item.Label, true, nil
		}
	}
	return "", false, nil
}

func (s *MemoryStore) ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
	values, err := s.ListDictValues(ctx, tenantID, dictCode, asOf, keyword, limit, "active")
	if err != nil {
		return nil, err
	}
	out := make([]dictpkg.Option, 0, len(values))
	for _, item := range values {
		out = append(out, dictpkg.Option{
			Code:        item.Code,
			Label:       item.Label,
			SetID:       DictOptionSetIDDeflt,
			SetIDSource: DictOptionSetIDSourceDeflt,
			Status:      item.Status,
			EnabledOn:   item.EnabledOn,
			DisabledOn:  item.DisabledOn,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return out, nil
}

func (s *MemoryStore) CreateDictValue(context.Context, string, DictCreateValueRequest) (DictValueItem, bool, error) {
	return DictValueItem{}, false, ErrDictValueConflict
}

func (s *MemoryStore) DisableDictValue(context.Context, string, DictDisableValueRequest) (DictValueItem, bool, error) {
	return DictValueItem{}, false, ErrDictValueConflict
}

func (s *MemoryStore) CorrectDictValue(context.Context, string, DictCorrectValueRequest) (DictValueItem, bool, error) {
	return DictValueItem{}, false, ErrDictValueConflict
}

func (s *MemoryStore) ListDictValueAudit(_ context.Context, tenantID string, dictCode string, code string, _ int) ([]DictValueAuditItem, error) {
	sourceTenantID, ok := s.resolveSourceTenant(tenantID, dictCode)
	if !ok {
		return nil, ErrDictNotFound
	}
	if sourceTenantID == "" || code == "" {
		return []DictValueAuditItem{}, nil
	}
	return []DictValueAuditItem{}, nil
}

func (s *MemoryStore) dictExists(tenantID string, dictCode string) bool {
	if _, ok := s.Dicts[tenantID][dictCode]; ok {
		return true
	}
	return false
}

func (s *MemoryStore) dictExistsAsOf(tenantID string, dictCode string, asOf string) bool {
	if item, ok := s.Dicts[tenantID][dictCode]; ok {
		return DictActiveAsOf(item, asOf)
	}
	return false
}

// 兼容历史测试：运行时 tenant-only，不再回退 global。
func (s *MemoryStore) resolveSourceTenantAsOf(tenantID string, dictCode string, asOf string) (string, bool) {
	if s.dictExistsAsOf(tenantID, dictCode, asOf) {
		return tenantID, true
	}
	return "", false
}

// 兼容历史测试：运行时 tenant-only，不再回退 global。
func (s *MemoryStore) resolveSourceTenant(tenantID string, dictCode string) (string, bool) {
	if s.dictExists(tenantID, dictCode) {
		return tenantID, true
	}
	return "", false
}

func (s *MemoryStore) valuesForTenant(tenantID string) []DictValueItem {
	if items, ok := s.Values[tenantID]; ok {
		return items
	}
	return nil
}

func (s *MemoryStore) dictsByTenant(tenantID string) map[string]DictItem {
	items := map[string]DictItem{}
	for code, item := range s.Dicts[tenantID] {
		items[code] = CloneDictItem(item)
	}
	return items
}

func CloneDictItem(item DictItem) DictItem {
	item.DisabledOn = CloneOptionalString(item.DisabledOn)
	return item
}

func DictActiveAsOf(item DictItem, asOf string) bool {
	if item.EnabledOn > asOf {
		return false
	}
	if item.DisabledOn != nil && asOf >= *item.DisabledOn {
		return false
	}
	return true
}

func DictStatusAsOf(item DictItem, asOf string) string {
	if DictActiveAsOf(item, asOf) {
		return "active"
	}
	return "inactive"
}

func ValueStatusAsOf(item DictValueItem, asOf string) string {
	if item.EnabledOn <= asOf && (item.DisabledOn == nil || asOf < *item.DisabledOn) {
		return "active"
	}
	return "inactive"
}

func DictDisplayName(dictCode string) string {
	switch strings.TrimSpace(strings.ToLower(dictCode)) {
	case DictCodeOrgType:
		return "Org Type"
	default:
		return dictCode
	}
}

func CloneOptionalString(in *string) *string {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
