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
	globalTenantID  = "00000000-0000-0000-0000-000000000000"

	dictEventCreated        = "DICT_VALUE_CREATED"
	dictEventLabelCorrected = "DICT_VALUE_LABEL_CORRECTED"
	dictEventDisabled       = "DICT_VALUE_DISABLED"
)

var (
	errDictCodeRequired         = errors.New("DICT_CODE_REQUIRED")
	errDictNotFound             = errors.New("DICT_NOT_FOUND")
	errDictValueCodeRequired    = errors.New("DICT_VALUE_CODE_REQUIRED")
	errDictValueLabelRequired   = errors.New("DICT_VALUE_LABEL_REQUIRED")
	errDictValueNotFoundAsOf    = errors.New("DICT_VALUE_NOT_FOUND_AS_OF")
	errDictValueConflict        = errors.New("DICT_VALUE_CONFLICT")
	errDictRequestCodeRequired  = errors.New("DICT_REQUEST_CODE_REQUIRED")
	errDictEffectiveDayRequired = errors.New("DICT_EFFECTIVE_DAY_REQUIRED")
)

type DictStore interface {
	dictpkg.Resolver
	ListDicts(ctx context.Context, tenantID string, asOf string) ([]DictItem, error)
	ListDictValues(ctx context.Context, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error)
	CreateDictValue(ctx context.Context, tenantID string, req DictCreateValueRequest) (DictValueItem, bool, error)
	DisableDictValue(ctx context.Context, tenantID string, req DictDisableValueRequest) (DictValueItem, bool, error)
	CorrectDictValue(ctx context.Context, tenantID string, req DictCorrectValueRequest) (DictValueItem, bool, error)
	ListDictValueAudit(ctx context.Context, tenantID string, dictCode string, code string, limit int) ([]DictValueAuditItem, error)
}

type DictItem struct {
	DictCode string
	Name     string
}

type DictValueItem struct {
	DictCode   string
	Code       string
	Label      string
	Status     string
	EnabledOn  string
	DisabledOn *string
	UpdatedAt  time.Time
}

type DictValueAuditItem struct {
	EventID        int64           `json:"event_id"`
	EventUUID      string          `json:"event_uuid"`
	DictCode       string          `json:"dict_code"`
	Code           string          `json:"code"`
	EventType      string          `json:"event_type"`
	EffectiveDay   string          `json:"effective_day"`
	RequestCode    string          `json:"request_code"`
	InitiatorUUID  string          `json:"initiator_uuid"`
	TxTime         time.Time       `json:"tx_time"`
	Payload        json.RawMessage `json:"payload"`
	BeforeSnapshot json.RawMessage `json:"before_snapshot"`
	AfterSnapshot  json.RawMessage `json:"after_snapshot"`
}

type DictCreateValueRequest struct {
	DictCode    string
	Code        string
	Label       string
	EnabledOn   string
	RequestCode string
	Initiator   string
}

type DictDisableValueRequest struct {
	DictCode    string
	Code        string
	DisabledOn  string
	RequestCode string
	Initiator   string
}

type DictCorrectValueRequest struct {
	DictCode      string
	Code          string
	Label         string
	CorrectionDay string
	RequestCode   string
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

	items, err := listDictsByTenant(ctx, tx, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 && tenantID != globalTenantID {
		items, err = listDictsByTenant(ctx, tx, globalTenantID, asOf)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func listDictsByTenant(ctx context.Context, tx pgx.Tx, tenantID string, asOf string) ([]DictItem, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT dict_code
FROM iam.dict_value_segments
WHERE tenant_uuid = $1::uuid
  AND enabled_on <= $2::date
  AND (disabled_on IS NULL OR $2::date < disabled_on)
ORDER BY dict_code ASC
`, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DictItem, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		items = append(items, DictItem{DictCode: code, Name: dictDisplayName(code)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
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

	values, err := listDictValuesByTenant(ctx, tx, tenantID, dictCode, asOf, keyword, limit, status)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 && tenantID != globalTenantID {
		values, err = listDictValuesByTenant(ctx, tx, globalTenantID, dictCode, asOf, keyword, limit, status)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return values, nil
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

	var label string
	label, ok, err := resolveValueLabelByTenant(ctx, tx, tenantID, asOf, dictCode, code)
	if err != nil {
		return "", false, err
	}
	if !ok && tenantID != globalTenantID {
		label, ok, err = resolveValueLabelByTenant(ctx, tx, globalTenantID, asOf, dictCode, code)
		if err != nil {
			return "", false, err
		}
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
	return s.submitValueEvent(ctx, tenantID, req.DictCode, req.Code, dictEventCreated, req.EnabledOn, map[string]any{"label": req.Label}, req.RequestCode, req.Initiator)
}

func (s *dictPGStore) DisableDictValue(ctx context.Context, tenantID string, req DictDisableValueRequest) (DictValueItem, bool, error) {
	return s.submitValueEvent(ctx, tenantID, req.DictCode, req.Code, dictEventDisabled, req.DisabledOn, map[string]any{}, req.RequestCode, req.Initiator)
}

func (s *dictPGStore) CorrectDictValue(ctx context.Context, tenantID string, req DictCorrectValueRequest) (DictValueItem, bool, error) {
	return s.submitValueEvent(ctx, tenantID, req.DictCode, req.Code, dictEventLabelCorrected, req.CorrectionDay, map[string]any{"label": req.Label}, req.RequestCode, req.Initiator)
}

func (s *dictPGStore) submitValueEvent(ctx context.Context, tenantID string, dictCode string, code string, eventType string, day string, payload map[string]any, requestCode string, initiator string) (DictValueItem, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DictValueItem{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
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
`, tenantID, dictCode, code, eventType, day, rawPayload, requestCode, initiator).Scan(&eventID, &wasRetry)
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

	rows, err := tx.Query(ctx, `
SELECT id, event_uuid::text, dict_code, code, event_type, effective_day::text, request_code, COALESCE(initiator_uuid::text, ''), tx_time, payload, before_snapshot, after_snapshot
FROM iam.dict_value_events
WHERE tenant_uuid = $1::uuid
  AND dict_code = $2::text
  AND code = $3::text
ORDER BY id DESC
LIMIT $4::int
`, tenantID, dictCode, code, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DictValueAuditItem, 0)
	for rows.Next() {
		var item DictValueAuditItem
		if err := rows.Scan(
			&item.EventID, &item.EventUUID, &item.DictCode, &item.Code, &item.EventType, &item.EffectiveDay, &item.RequestCode,
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
	values map[string][]DictValueItem
}

func newDictMemoryStore() DictStore {
	now := time.Unix(0, 0).UTC()
	defaultValues := []DictValueItem{
		{DictCode: dictCodeOrgType, Code: "10", Label: "部门", Status: "active", EnabledOn: "1970-01-01", UpdatedAt: now},
		{DictCode: dictCodeOrgType, Code: "20", Label: "单位", Status: "active", EnabledOn: "1970-01-01", UpdatedAt: now},
	}
	return &dictMemoryStore{
		values: map[string][]DictValueItem{
			globalTenantID:                         append([]DictValueItem(nil), defaultValues...),
			"00000000-0000-0000-0000-000000000001": append([]DictValueItem(nil), defaultValues...),
		},
	}
}

func (s *dictMemoryStore) ListDicts(_ context.Context, tenantID string, asOf string) ([]DictItem, error) {
	if strings.TrimSpace(asOf) == "" {
		return nil, errDictEffectiveDayRequired
	}
	for _, item := range s.valuesForTenant(tenantID) {
		if item.DictCode == dictCodeOrgType && item.EnabledOn <= asOf {
			return []DictItem{{DictCode: dictCodeOrgType, Name: dictDisplayName(dictCodeOrgType)}}, nil
		}
	}
	return []DictItem{}, nil
}

func (s *dictMemoryStore) ListDictValues(_ context.Context, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error) {
	_ = status
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	out := make([]DictValueItem, 0)
	for _, item := range s.valuesForTenant(tenantID) {
		if item.DictCode != dictCode {
			continue
		}
		if item.EnabledOn > asOf {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.Code), keyword) && !strings.Contains(strings.ToLower(item.Label), keyword) {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Code < out[j].Code })
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
	for _, item := range s.valuesForTenant(tenantID) {
		if item.DictCode == dictCode && item.Code == code && item.EnabledOn <= asOf {
			return item.Label, true, nil
		}
	}
	return "", false, nil
}

func (s *dictMemoryStore) ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
	values, _ := s.ListDictValues(ctx, tenantID, dictCode, asOf, keyword, limit, "active")
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

func (s *dictMemoryStore) ListDictValueAudit(context.Context, string, string, string, int) ([]DictValueAuditItem, error) {
	return []DictValueAuditItem{}, nil
}

func (s *dictMemoryStore) valuesForTenant(tenantID string) []DictValueItem {
	if items, ok := s.values[tenantID]; ok && len(items) > 0 {
		return items
	}
	return s.values[globalTenantID]
}

func supportedDictCode(dictCode string) bool {
	return strings.EqualFold(strings.TrimSpace(dictCode), dictCodeOrgType)
}

func dictDisplayName(dictCode string) string {
	switch strings.TrimSpace(strings.ToLower(dictCode)) {
	case dictCodeOrgType:
		return "Org Type"
	default:
		return dictCode
	}
}
