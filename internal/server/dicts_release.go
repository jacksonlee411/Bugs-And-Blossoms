package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	errDictReleaseIDRequired     = errors.New("DICT_RELEASE_ID_REQUIRED")
	errDictReleaseSourceInvalid  = errors.New("DICT_RELEASE_SOURCE_INVALID")
	errDictReleaseTargetRequired = errors.New("DICT_RELEASE_TARGET_REQUIRED")
	errDictReleasePayloadInvalid = errors.New("DICT_RELEASE_PAYLOAD_INVALID")
)

type DictBaselineReleaseRequest struct {
	SourceTenantID string
	TargetTenantID string
	AsOf           string
	ReleaseID      string
	RequestID      string
	Operator       string
	Initiator      string
	MaxConflicts   int
}

type DictBaselineReleaseResult struct {
	TaskID             string    `json:"task_id"`
	ReleaseID          string    `json:"release_id"`
	RequestID          string    `json:"request_id"`
	SourceTenantID     string    `json:"source_tenant_id"`
	TargetTenantID     string    `json:"target_tenant_id"`
	AsOf               string    `json:"as_of"`
	Status             string    `json:"status"`
	DictEventsTotal    int       `json:"dict_events_total"`
	DictEventsApplied  int       `json:"dict_events_applied"`
	DictEventsRetried  int       `json:"dict_events_retried"`
	ValueEventsTotal   int       `json:"value_events_total"`
	ValueEventsApplied int       `json:"value_events_applied"`
	ValueEventsRetried int       `json:"value_events_retried"`
	StartedAt          time.Time `json:"started_at"`
	FinishedAt         time.Time `json:"finished_at"`
}

type DictBaselineReleasePreview struct {
	ReleaseID               string                        `json:"release_id"`
	SourceTenantID          string                        `json:"source_tenant_id"`
	TargetTenantID          string                        `json:"target_tenant_id"`
	AsOf                    string                        `json:"as_of"`
	SourceDictCount         int                           `json:"source_dict_count"`
	SourceValueCount        int                           `json:"source_value_count"`
	TargetDictCount         int                           `json:"target_dict_count"`
	TargetValueCount        int                           `json:"target_value_count"`
	MissingDictCount        int                           `json:"missing_dict_count"`
	DictNameMismatchCount   int                           `json:"dict_name_mismatch_count"`
	MissingValueCount       int                           `json:"missing_value_count"`
	ValueLabelMismatchCount int                           `json:"value_label_mismatch_count"`
	Conflicts               []DictBaselineReleaseConflict `json:"conflicts"`
}

type DictBaselineReleaseConflict struct {
	Kind        string `json:"kind"`
	DictCode    string `json:"dict_code"`
	Code        string `json:"code,omitempty"`
	SourceValue string `json:"source_value,omitempty"`
	TargetValue string `json:"target_value,omitempty"`
}

type DictBaselineReleaseStore interface {
	PreviewBaseline(ctx context.Context, req DictBaselineReleaseRequest) (DictBaselineReleasePreview, error)
	PublishBaseline(ctx context.Context, req DictBaselineReleaseRequest) (DictBaselineReleaseResult, error)
}

type dictReleaseSourceEvent struct {
	ID           int64
	DictCode     string
	Code         string
	EventType    string
	EffectiveDay string
	RequestID    string
	Payload      json.RawMessage
}

func (s *dictPGStore) PreviewBaseline(ctx context.Context, req DictBaselineReleaseRequest) (DictBaselineReleasePreview, error) {
	req = normalizeDictBaselineReleaseRequest(req)
	if err := validateDictBaselineReleaseRequest(req, false); err != nil {
		return DictBaselineReleasePreview{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DictBaselineReleasePreview{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	sourceDicts, sourceValues, err := loadReleaseSnapshotTx(ctx, tx, req.SourceTenantID, req.AsOf)
	if err != nil {
		return DictBaselineReleasePreview{}, err
	}
	targetDicts, targetValues, err := loadReleaseSnapshotTx(ctx, tx, req.TargetTenantID, req.AsOf)
	if err != nil {
		return DictBaselineReleasePreview{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DictBaselineReleasePreview{}, err
	}

	preview := DictBaselineReleasePreview{
		ReleaseID:        req.ReleaseID,
		SourceTenantID:   req.SourceTenantID,
		TargetTenantID:   req.TargetTenantID,
		AsOf:             req.AsOf,
		SourceDictCount:  len(sourceDicts),
		SourceValueCount: len(sourceValues),
		TargetDictCount:  len(targetDicts),
		TargetValueCount: len(targetValues),
		Conflicts:        make([]DictBaselineReleaseConflict, 0),
	}

	limit := req.MaxConflicts
	if limit <= 0 {
		limit = 200
	}

	for dictCode, sourceName := range sourceDicts {
		targetName, ok := targetDicts[dictCode]
		if !ok {
			preview.MissingDictCount++
			appendReleaseConflict(&preview.Conflicts, limit, DictBaselineReleaseConflict{
				Kind:        "dict_missing",
				DictCode:    dictCode,
				SourceValue: sourceName,
			})
			continue
		}
		if targetName != sourceName {
			preview.DictNameMismatchCount++
			appendReleaseConflict(&preview.Conflicts, limit, DictBaselineReleaseConflict{
				Kind:        "dict_name_mismatch",
				DictCode:    dictCode,
				SourceValue: sourceName,
				TargetValue: targetName,
			})
		}
	}

	for key, sourceLabel := range sourceValues {
		targetLabel, ok := targetValues[key]
		dictCode, code := splitDictValueKey(key)
		if !ok {
			preview.MissingValueCount++
			appendReleaseConflict(&preview.Conflicts, limit, DictBaselineReleaseConflict{
				Kind:        "value_missing",
				DictCode:    dictCode,
				Code:        code,
				SourceValue: sourceLabel,
			})
			continue
		}
		if targetLabel != sourceLabel {
			preview.ValueLabelMismatchCount++
			appendReleaseConflict(&preview.Conflicts, limit, DictBaselineReleaseConflict{
				Kind:        "value_label_mismatch",
				DictCode:    dictCode,
				Code:        code,
				SourceValue: sourceLabel,
				TargetValue: targetLabel,
			})
		}
	}
	return preview, nil
}

func (s *dictPGStore) PublishBaseline(ctx context.Context, req DictBaselineReleaseRequest) (DictBaselineReleaseResult, error) {
	req = normalizeDictBaselineReleaseRequest(req)
	if err := validateDictBaselineReleaseRequest(req, true); err != nil {
		return DictBaselineReleaseResult{}, err
	}

	result := DictBaselineReleaseResult{
		TaskID:         dictBaselineReleaseTaskID(req.ReleaseID, req.TargetTenantID, req.AsOf),
		ReleaseID:      req.ReleaseID,
		RequestID:      req.RequestID,
		SourceTenantID: req.SourceTenantID,
		TargetTenantID: req.TargetTenantID,
		AsOf:           req.AsOf,
		Status:         "running",
		StartedAt:      time.Now().UTC(),
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DictBaselineReleaseResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	dictEvents, valueEvents, err := loadReleaseSourceEventsTx(ctx, tx, req.SourceTenantID, req.AsOf)
	if err != nil {
		return DictBaselineReleaseResult{}, err
	}
	if len(dictEvents) == 0 && len(valueEvents) == 0 {
		return DictBaselineReleaseResult{}, errDictBaselineNotReady
	}

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, req.TargetTenantID); err != nil {
		return DictBaselineReleaseResult{}, err
	}

	for _, rec := range dictEvents {
		payload, err := withReleaseMetadata(rec.Payload, req, rec.ID, rec.RequestID)
		if err != nil {
			return DictBaselineReleaseResult{}, err
		}
		requestID := dictBaselineReleaseRequestCode(req.RequestID, "dict", rec.ID)
		wasRetry, err := submitDictReleaseEventTx(ctx, tx, req.TargetTenantID, rec.DictCode, rec.EventType, rec.EffectiveDay, payload, requestID, req.Initiator)
		if err != nil {
			return DictBaselineReleaseResult{}, err
		}
		result.DictEventsTotal++
		if wasRetry {
			result.DictEventsRetried++
		} else {
			result.DictEventsApplied++
		}
	}

	for _, rec := range valueEvents {
		payload, err := withReleaseMetadata(rec.Payload, req, rec.ID, rec.RequestID)
		if err != nil {
			return DictBaselineReleaseResult{}, err
		}
		requestID := dictBaselineReleaseRequestCode(req.RequestID, "value", rec.ID)
		wasRetry, err := submitDictValueReleaseEventTx(ctx, tx, req.TargetTenantID, rec.DictCode, rec.Code, rec.EventType, rec.EffectiveDay, payload, requestID, req.Initiator)
		if err != nil {
			return DictBaselineReleaseResult{}, err
		}
		result.ValueEventsTotal++
		if wasRetry {
			result.ValueEventsRetried++
		} else {
			result.ValueEventsApplied++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return DictBaselineReleaseResult{}, err
	}

	result.Status = "succeeded"
	result.FinishedAt = time.Now().UTC()
	return result, nil
}

func loadReleaseSourceEventsTx(ctx context.Context, tx pgx.Tx, sourceTenantID string, asOf string) ([]dictReleaseSourceEvent, []dictReleaseSourceEvent, error) {
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, sourceTenantID); err != nil {
		return nil, nil, err
	}

	dictRows, err := tx.Query(ctx, `
SELECT id, dict_code, event_type, effective_day::text, request_id, payload
FROM iam.dict_events
WHERE tenant_uuid = $1::uuid
  AND effective_day <= $2::date
ORDER BY id ASC
`, sourceTenantID, asOf)
	if err != nil {
		return nil, nil, err
	}
	defer dictRows.Close()

	dictEvents := make([]dictReleaseSourceEvent, 0)
	for dictRows.Next() {
		var rec dictReleaseSourceEvent
		if err := dictRows.Scan(&rec.ID, &rec.DictCode, &rec.EventType, &rec.EffectiveDay, &rec.RequestID, &rec.Payload); err != nil {
			return nil, nil, err
		}
		dictEvents = append(dictEvents, rec)
	}
	if err := dictRows.Err(); err != nil {
		return nil, nil, err
	}

	valueRows, err := tx.Query(ctx, `
SELECT id, dict_code, code, event_type, effective_day::text, request_id, payload
FROM iam.dict_value_events
WHERE tenant_uuid = $1::uuid
  AND effective_day <= $2::date
ORDER BY id ASC
`, sourceTenantID, asOf)
	if err != nil {
		return nil, nil, err
	}
	defer valueRows.Close()

	valueEvents := make([]dictReleaseSourceEvent, 0)
	for valueRows.Next() {
		var rec dictReleaseSourceEvent
		if err := valueRows.Scan(&rec.ID, &rec.DictCode, &rec.Code, &rec.EventType, &rec.EffectiveDay, &rec.RequestID, &rec.Payload); err != nil {
			return nil, nil, err
		}
		valueEvents = append(valueEvents, rec)
	}
	if err := valueRows.Err(); err != nil {
		return nil, nil, err
	}

	return dictEvents, valueEvents, nil
}

func loadReleaseSnapshotTx(ctx context.Context, tx pgx.Tx, tenantID string, asOf string) (map[string]string, map[string]string, error) {
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, nil, err
	}

	dictRows, err := tx.Query(ctx, `
SELECT dict_code, name
FROM iam.dicts
WHERE tenant_uuid = $1::uuid
  AND enabled_on <= $2::date
  AND (disabled_on IS NULL OR $2::date < disabled_on)
`, tenantID, asOf)
	if err != nil {
		return nil, nil, err
	}
	defer dictRows.Close()

	dicts := make(map[string]string)
	for dictRows.Next() {
		var dictCode string
		var name string
		if err := dictRows.Scan(&dictCode, &name); err != nil {
			return nil, nil, err
		}
		dicts[dictCode] = name
	}
	if err := dictRows.Err(); err != nil {
		return nil, nil, err
	}

	valueRows, err := tx.Query(ctx, `
SELECT dict_code, code, label
FROM iam.dict_value_segments
WHERE tenant_uuid = $1::uuid
  AND enabled_on <= $2::date
  AND (disabled_on IS NULL OR $2::date < disabled_on)
`, tenantID, asOf)
	if err != nil {
		return nil, nil, err
	}
	defer valueRows.Close()

	values := make(map[string]string)
	for valueRows.Next() {
		var dictCode string
		var code string
		var label string
		if err := valueRows.Scan(&dictCode, &code, &label); err != nil {
			return nil, nil, err
		}
		values[joinDictValueKey(dictCode, code)] = label
	}
	if err := valueRows.Err(); err != nil {
		return nil, nil, err
	}

	return dicts, values, nil
}

func withReleaseMetadata(raw json.RawMessage, req DictBaselineReleaseRequest, sourceEventID int64, sourceRequestID string) ([]byte, error) {
	var payload map[string]any
	if len(raw) == 0 {
		payload = map[string]any{}
	} else if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, errDictReleasePayloadInvalid
	}
	payload["release"] = map[string]any{
		"release_id":        req.ReleaseID,
		"source_tenant_id":  req.SourceTenantID,
		"target_tenant_id":  req.TargetTenantID,
		"source_event_id":   sourceEventID,
		"source_request_id": sourceRequestID,
		"operator":          req.Operator,
		"as_of":             req.AsOf,
	}
	return json.Marshal(payload)
}

func submitDictReleaseEventTx(
	ctx context.Context,
	tx pgx.Tx,
	targetTenantID string,
	dictCode string,
	eventType string,
	effectiveDay string,
	payload []byte,
	requestID string,
	initiator string,
) (bool, error) {
	var eventID int64
	var wasRetry bool
	err := tx.QueryRow(ctx, `
SELECT event_id, was_retry
FROM iam.submit_dict_event($1::uuid, $2::text, $3::text, $4::date, $5::jsonb, $6::text, $7::uuid)
`, targetTenantID, dictCode, eventType, effectiveDay, payload, requestID, initiator).Scan(&eventID, &wasRetry)
	if err != nil {
		return false, err
	}
	_ = eventID
	return wasRetry, nil
}

func submitDictValueReleaseEventTx(
	ctx context.Context,
	tx pgx.Tx,
	targetTenantID string,
	dictCode string,
	code string,
	eventType string,
	effectiveDay string,
	payload []byte,
	requestID string,
	initiator string,
) (bool, error) {
	var eventID int64
	var wasRetry bool
	err := tx.QueryRow(ctx, `
SELECT event_id, was_retry
FROM iam.submit_dict_value_event($1::uuid, $2::text, $3::text, $4::text, $5::date, $6::jsonb, $7::text, $8::uuid)
`, targetTenantID, dictCode, code, eventType, effectiveDay, payload, requestID, initiator).Scan(&eventID, &wasRetry)
	if err != nil {
		return false, err
	}
	_ = eventID
	return wasRetry, nil
}

func normalizeDictBaselineReleaseRequest(req DictBaselineReleaseRequest) DictBaselineReleaseRequest {
	req.SourceTenantID = strings.TrimSpace(req.SourceTenantID)
	if req.SourceTenantID == "" {
		req.SourceTenantID = globalTenantID
	}
	req.TargetTenantID = strings.TrimSpace(req.TargetTenantID)
	req.AsOf = strings.TrimSpace(req.AsOf)
	req.ReleaseID = strings.TrimSpace(req.ReleaseID)
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.Operator = strings.TrimSpace(req.Operator)
	req.Initiator = strings.TrimSpace(req.Initiator)
	return req
}

func validateDictBaselineReleaseRequest(req DictBaselineReleaseRequest, requireRequestID bool) error {
	if req.ReleaseID == "" {
		return errDictReleaseIDRequired
	}
	if req.TargetTenantID == "" {
		return errDictReleaseTargetRequired
	}
	if !isDate(req.AsOf) {
		return errDictEffectiveDayRequired
	}
	if requireRequestID && req.RequestID == "" {
		return errDictRequestIDRequired
	}
	if _, err := uuid.Parse(req.SourceTenantID); err != nil {
		return errDictReleaseSourceInvalid
	}
	if _, err := uuid.Parse(req.TargetTenantID); err != nil {
		return errDictReleaseTargetRequired
	}
	return nil
}

func dictBaselineReleaseRequestCode(base string, kind string, sourceEventID int64) string {
	return fmt.Sprintf("%s#%s#%d", base, kind, sourceEventID)
}

func dictBaselineReleaseTaskID(releaseID string, targetTenantID string, asOf string) string {
	compactTenant := strings.ReplaceAll(targetTenantID, "-", "")
	return fmt.Sprintf("dict-release:%s:%s:%s", releaseID, compactTenant, asOf)
}

func appendReleaseConflict(conflicts *[]DictBaselineReleaseConflict, max int, item DictBaselineReleaseConflict) {
	if len(*conflicts) >= max {
		return
	}
	*conflicts = append(*conflicts, item)
}

func joinDictValueKey(dictCode string, code string) string {
	return dictCode + "|" + code
}

func splitDictValueKey(key string) (string, string) {
	idx := strings.Index(key, "|")
	if idx < 0 {
		return key, ""
	}
	return key[:idx], key[idx+1:]
}
