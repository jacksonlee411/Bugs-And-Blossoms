package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type TimePunch struct {
	EventID         string          `json:"event_id"`
	PersonUUID      string          `json:"person_uuid"`
	PunchTime       time.Time       `json:"punch_time"`
	PunchType       string          `json:"punch_type"`
	SourceProvider  string          `json:"source_provider"`
	Payload         json.RawMessage `json:"payload"`
	TransactionTime time.Time       `json:"transaction_time"`
}

type TimePunchStore interface {
	ListTimePunchesForPerson(ctx context.Context, tenantID string, personUUID string, fromUTC time.Time, toUTC time.Time, limit int) ([]TimePunch, error)
	SubmitTimePunch(ctx context.Context, tenantID string, initiatorID string, p submitTimePunchParams) (TimePunch, error)
	ImportTimePunches(ctx context.Context, tenantID string, initiatorID string, events []submitTimePunchParams) error
}

type DailyAttendanceResult struct {
	PersonUUID             string     `json:"person_uuid"`
	WorkDate               string     `json:"work_date"`
	RulesetVersion         string     `json:"ruleset_version"`
	Status                 string     `json:"status"`
	Flags                  []string   `json:"flags"`
	FirstInTime            *time.Time `json:"first_in_time"`
	LastOutTime            *time.Time `json:"last_out_time"`
	WorkedMinutes          int        `json:"worked_minutes"`
	LateMinutes            int        `json:"late_minutes"`
	EarlyLeaveMinutes      int        `json:"early_leave_minutes"`
	InputPunchCount        int        `json:"input_punch_count"`
	InputMaxPunchEventDBID *int64     `json:"input_max_punch_event_db_id"`
	InputMaxPunchTime      *time.Time `json:"input_max_punch_time"`
	ComputedAt             time.Time  `json:"computed_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type DailyAttendanceResultStore interface {
	ListDailyAttendanceResultsForDate(ctx context.Context, tenantID string, workDate string, limit int) ([]DailyAttendanceResult, error)
	GetDailyAttendanceResult(ctx context.Context, tenantID string, personUUID string, workDate string) (DailyAttendanceResult, bool, error)
	ListDailyAttendanceResultsForPerson(ctx context.Context, tenantID string, personUUID string, fromDate string, toDate string, limit int) ([]DailyAttendanceResult, error)
}

type submitTimePunchParams struct {
	EventID          string
	PersonUUID       string
	PunchTime        time.Time
	PunchType        string
	SourceProvider   string
	Payload          json.RawMessage
	SourceRawPayload json.RawMessage
	DeviceInfo       json.RawMessage
}

func (s *staffingPGStore) ListTimePunchesForPerson(ctx context.Context, tenantID string, personUUID string, fromUTC time.Time, toUTC time.Time, limit int) ([]TimePunch, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	rows, err := tx.Query(ctx, `
SELECT
  event_id::text,
  person_uuid::text,
  punch_time,
  punch_type,
  source_provider,
  payload,
  transaction_time
FROM staffing.time_punch_events
WHERE tenant_id = $1::uuid
  AND person_uuid = $2::uuid
  AND punch_time >= $3
  AND punch_time < $4
ORDER BY punch_time DESC, id DESC
LIMIT $5
`, tenantID, personUUID, fromUTC, toUTC, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TimePunch
	for rows.Next() {
		var p TimePunch
		var payload []byte
		if err := rows.Scan(&p.EventID, &p.PersonUUID, &p.PunchTime, &p.PunchType, &p.SourceProvider, &payload, &p.TransactionTime); err != nil {
			return nil, err
		}
		p.PunchTime = p.PunchTime.UTC()
		p.TransactionTime = p.TransactionTime.UTC()
		p.Payload = json.RawMessage(payload)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *staffingPGStore) SubmitTimePunch(ctx context.Context, tenantID string, initiatorID string, p submitTimePunchParams) (TimePunch, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TimePunch{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return TimePunch{}, err
	}

	if p.EventID == "" {
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&p.EventID); err != nil {
			return TimePunch{}, err
		}
	}

	p.PersonUUID = strings.TrimSpace(p.PersonUUID)
	if p.PersonUUID == "" {
		return TimePunch{}, errors.New("person_uuid is required")
	}

	p.PunchType = strings.ToUpper(strings.TrimSpace(p.PunchType))
	if p.PunchType == "" {
		return TimePunch{}, errors.New("punch_type is required")
	}

	p.SourceProvider = strings.ToUpper(strings.TrimSpace(p.SourceProvider))
	if p.SourceProvider == "" {
		p.SourceProvider = "MANUAL"
	}

	payload := json.RawMessage(p.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	sourceRaw := json.RawMessage(p.SourceRawPayload)
	if len(sourceRaw) == 0 {
		sourceRaw = json.RawMessage(`{}`)
	}
	deviceInfo := json.RawMessage(p.DeviceInfo)
	if len(deviceInfo) == 0 {
		deviceInfo = json.RawMessage(`{}`)
	}

	requestID := p.EventID

	var eventDBID int64
	if err := tx.QueryRow(ctx, `
SELECT staffing.submit_time_punch_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::timestamptz,
  $5::text,
  $6::text,
  $7::jsonb,
  $8::jsonb,
  $9::jsonb,
  $10::text,
  $11::uuid
)
`, p.EventID, tenantID, p.PersonUUID, p.PunchTime.UTC(), p.PunchType, p.SourceProvider, []byte(payload), []byte(sourceRaw), []byte(deviceInfo), requestID, initiatorID).Scan(&eventDBID); err != nil {
		return TimePunch{}, err
	}

	var out TimePunch
	var payloadOut []byte
	if err := tx.QueryRow(ctx, `
SELECT event_id::text, person_uuid::text, punch_time, punch_type, source_provider, payload, transaction_time
FROM staffing.time_punch_events
WHERE id = $1
`, eventDBID).Scan(&out.EventID, &out.PersonUUID, &out.PunchTime, &out.PunchType, &out.SourceProvider, &payloadOut, &out.TransactionTime); err != nil {
		return TimePunch{}, err
	}
	out.PunchTime = out.PunchTime.UTC()
	out.TransactionTime = out.TransactionTime.UTC()
	out.Payload = json.RawMessage(payloadOut)

	if err := tx.Commit(ctx); err != nil {
		return TimePunch{}, err
	}
	return out, nil
}

func (s *staffingPGStore) ImportTimePunches(ctx context.Context, tenantID string, initiatorID string, events []submitTimePunchParams) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	for i, e := range events {
		if e.EventID == "" {
			if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&e.EventID); err != nil {
				return fmt.Errorf("line %d: %w", i+1, err)
			}
		}
		e.PersonUUID = strings.TrimSpace(e.PersonUUID)
		if e.PersonUUID == "" {
			return fmt.Errorf("line %d: person_uuid is required", i+1)
		}
		e.PunchType = strings.ToUpper(strings.TrimSpace(e.PunchType))
		e.SourceProvider = strings.ToUpper(strings.TrimSpace(e.SourceProvider))
		if e.SourceProvider == "" {
			e.SourceProvider = "IMPORT"
		}

		payload := json.RawMessage(e.Payload)
		if len(payload) == 0 {
			payload = json.RawMessage(`{}`)
		}
		sourceRaw := json.RawMessage(e.SourceRawPayload)
		if len(sourceRaw) == 0 {
			sourceRaw = json.RawMessage(`{}`)
		}
		deviceInfo := json.RawMessage(e.DeviceInfo)
		if len(deviceInfo) == 0 {
			deviceInfo = json.RawMessage(`{}`)
		}

		requestID := e.EventID

		var id int64
		if err := tx.QueryRow(ctx, `
SELECT staffing.submit_time_punch_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::timestamptz,
  $5::text,
  $6::text,
  $7::jsonb,
  $8::jsonb,
  $9::jsonb,
  $10::text,
  $11::uuid
)
`, e.EventID, tenantID, e.PersonUUID, e.PunchTime.UTC(), e.PunchType, e.SourceProvider, []byte(payload), []byte(sourceRaw), []byte(deviceInfo), requestID, initiatorID).Scan(&id); err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *staffingPGStore) ListDailyAttendanceResultsForDate(ctx context.Context, tenantID string, workDate string, limit int) ([]DailyAttendanceResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	rows, err := tx.Query(ctx, `
SELECT
  person_uuid::text,
  work_date::text,
  ruleset_version,
  status,
  flags,
  first_in_time,
  last_out_time,
  worked_minutes,
  late_minutes,
  early_leave_minutes,
  input_punch_count,
  input_max_punch_event_db_id,
  input_max_punch_time,
  computed_at,
  created_at,
  updated_at
FROM staffing.daily_attendance_results
WHERE tenant_id = $1::uuid
  AND work_date = $2::date
ORDER BY person_uuid::text ASC
LIMIT $3
`, tenantID, workDate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DailyAttendanceResult
	for rows.Next() {
		var r DailyAttendanceResult
		var firstIn *time.Time
		var lastOut *time.Time
		var inputMaxPunchEventDBID *int64
		var inputMaxPunchTime *time.Time
		if err := rows.Scan(
			&r.PersonUUID,
			&r.WorkDate,
			&r.RulesetVersion,
			&r.Status,
			&r.Flags,
			&firstIn,
			&lastOut,
			&r.WorkedMinutes,
			&r.LateMinutes,
			&r.EarlyLeaveMinutes,
			&r.InputPunchCount,
			&inputMaxPunchEventDBID,
			&inputMaxPunchTime,
			&r.ComputedAt,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if firstIn != nil {
			tm := firstIn.UTC()
			r.FirstInTime = &tm
		}
		if lastOut != nil {
			tm := lastOut.UTC()
			r.LastOutTime = &tm
		}
		r.InputMaxPunchEventDBID = inputMaxPunchEventDBID
		if inputMaxPunchTime != nil {
			tm := inputMaxPunchTime.UTC()
			r.InputMaxPunchTime = &tm
		}
		r.ComputedAt = r.ComputedAt.UTC()
		r.CreatedAt = r.CreatedAt.UTC()
		r.UpdatedAt = r.UpdatedAt.UTC()

		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *staffingPGStore) GetDailyAttendanceResult(ctx context.Context, tenantID string, personUUID string, workDate string) (DailyAttendanceResult, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DailyAttendanceResult{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return DailyAttendanceResult{}, false, err
	}

	var out DailyAttendanceResult
	var firstIn *time.Time
	var lastOut *time.Time
	var inputMaxPunchEventDBID *int64
	var inputMaxPunchTime *time.Time
	err = tx.QueryRow(ctx, `
SELECT
  person_uuid::text,
  work_date::text,
  ruleset_version,
  status,
  flags,
  first_in_time,
  last_out_time,
  worked_minutes,
  late_minutes,
  early_leave_minutes,
  input_punch_count,
  input_max_punch_event_db_id,
  input_max_punch_time,
  computed_at,
  created_at,
  updated_at
FROM staffing.daily_attendance_results
WHERE tenant_id = $1::uuid
  AND person_uuid = $2::uuid
  AND work_date = $3::date
`, tenantID, personUUID, workDate).Scan(
		&out.PersonUUID,
		&out.WorkDate,
		&out.RulesetVersion,
		&out.Status,
		&out.Flags,
		&firstIn,
		&lastOut,
		&out.WorkedMinutes,
		&out.LateMinutes,
		&out.EarlyLeaveMinutes,
		&out.InputPunchCount,
		&inputMaxPunchEventDBID,
		&inputMaxPunchTime,
		&out.ComputedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DailyAttendanceResult{}, false, nil
		}
		return DailyAttendanceResult{}, false, err
	}

	if firstIn != nil {
		tm := firstIn.UTC()
		out.FirstInTime = &tm
	}
	if lastOut != nil {
		tm := lastOut.UTC()
		out.LastOutTime = &tm
	}
	out.InputMaxPunchEventDBID = inputMaxPunchEventDBID
	if inputMaxPunchTime != nil {
		tm := inputMaxPunchTime.UTC()
		out.InputMaxPunchTime = &tm
	}
	out.ComputedAt = out.ComputedAt.UTC()
	out.CreatedAt = out.CreatedAt.UTC()
	out.UpdatedAt = out.UpdatedAt.UTC()

	if err := tx.Commit(ctx); err != nil {
		return DailyAttendanceResult{}, false, err
	}
	return out, true, nil
}

func (s *staffingPGStore) ListDailyAttendanceResultsForPerson(ctx context.Context, tenantID string, personUUID string, fromDate string, toDate string, limit int) ([]DailyAttendanceResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	rows, err := tx.Query(ctx, `
SELECT
  person_uuid::text,
  work_date::text,
  ruleset_version,
  status,
  flags,
  first_in_time,
  last_out_time,
  worked_minutes,
  late_minutes,
  early_leave_minutes,
  input_punch_count,
  input_max_punch_event_db_id,
  input_max_punch_time,
  computed_at,
  created_at,
  updated_at
FROM staffing.daily_attendance_results
WHERE tenant_id = $1::uuid
  AND person_uuid = $2::uuid
  AND work_date >= $3::date
  AND work_date <= $4::date
ORDER BY work_date DESC
LIMIT $5
`, tenantID, personUUID, fromDate, toDate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DailyAttendanceResult
	for rows.Next() {
		var r DailyAttendanceResult
		var firstIn *time.Time
		var lastOut *time.Time
		var inputMaxPunchEventDBID *int64
		var inputMaxPunchTime *time.Time
		if err := rows.Scan(
			&r.PersonUUID,
			&r.WorkDate,
			&r.RulesetVersion,
			&r.Status,
			&r.Flags,
			&firstIn,
			&lastOut,
			&r.WorkedMinutes,
			&r.LateMinutes,
			&r.EarlyLeaveMinutes,
			&r.InputPunchCount,
			&inputMaxPunchEventDBID,
			&inputMaxPunchTime,
			&r.ComputedAt,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if firstIn != nil {
			tm := firstIn.UTC()
			r.FirstInTime = &tm
		}
		if lastOut != nil {
			tm := lastOut.UTC()
			r.LastOutTime = &tm
		}
		r.InputMaxPunchEventDBID = inputMaxPunchEventDBID
		if inputMaxPunchTime != nil {
			tm := inputMaxPunchTime.UTC()
			r.InputMaxPunchTime = &tm
		}
		r.ComputedAt = r.ComputedAt.UTC()
		r.CreatedAt = r.CreatedAt.UTC()
		r.UpdatedAt = r.UpdatedAt.UTC()

		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func isSTAFFING_IDEMPOTENCY_REUSED(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "STAFFING_IDEMPOTENCY_REUSED")
}

func (s *staffingMemoryStore) ListTimePunchesForPerson(_ context.Context, tenantID string, personUUID string, fromUTC time.Time, toUTC time.Time, limit int) ([]TimePunch, error) {
	byPerson := s.punches[tenantID]
	if byPerson == nil {
		return nil, nil
	}
	all := byPerson[personUUID]
	if len(all) == 0 {
		return nil, nil
	}

	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	out := make([]TimePunch, 0, min(limit, len(all)))
	for _, p := range all {
		if p.PunchTime.Before(fromUTC) || !p.PunchTime.Before(toUTC) {
			continue
		}
		out = append(out, p)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *staffingMemoryStore) SubmitTimePunch(_ context.Context, tenantID string, initiatorID string, p submitTimePunchParams) (TimePunch, error) {
	if strings.TrimSpace(initiatorID) == "" {
		return TimePunch{}, errors.New("initiator_id is required")
	}

	p.PersonUUID = strings.TrimSpace(p.PersonUUID)
	if p.PersonUUID == "" {
		return TimePunch{}, errors.New("person_uuid is required")
	}

	p.PunchType = strings.ToUpper(strings.TrimSpace(p.PunchType))
	if p.PunchType == "" {
		return TimePunch{}, errors.New("punch_type is required")
	}
	if p.PunchType != "IN" && p.PunchType != "OUT" {
		return TimePunch{}, errors.New("unsupported punch_type")
	}

	p.SourceProvider = strings.ToUpper(strings.TrimSpace(p.SourceProvider))
	if p.SourceProvider == "" {
		p.SourceProvider = "MANUAL"
	}
	if p.SourceProvider != "MANUAL" && p.SourceProvider != "IMPORT" {
		return TimePunch{}, errors.New("unsupported source_provider")
	}

	if p.EventID == "" {
		p.EventID = "ev-" + fmt.Sprint(time.Now().UnixNano())
	}

	payload := json.RawMessage(p.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	out := TimePunch{
		EventID:         p.EventID,
		PersonUUID:      p.PersonUUID,
		PunchTime:       p.PunchTime.UTC(),
		PunchType:       p.PunchType,
		SourceProvider:  p.SourceProvider,
		Payload:         payload,
		TransactionTime: time.Now().UTC(),
	}

	if s.punches[tenantID] == nil {
		s.punches[tenantID] = make(map[string][]TimePunch)
	}
	s.punches[tenantID][p.PersonUUID] = append([]TimePunch{out}, s.punches[tenantID][p.PersonUUID]...)

	return out, nil
}

func (s *staffingMemoryStore) ImportTimePunches(ctx context.Context, tenantID string, initiatorID string, events []submitTimePunchParams) error {
	for _, e := range events {
		if _, err := s.SubmitTimePunch(ctx, tenantID, initiatorID, e); err != nil {
			return err
		}
	}
	return nil
}

func (s *staffingMemoryStore) ListDailyAttendanceResultsForDate(context.Context, string, string, int) ([]DailyAttendanceResult, error) {
	return nil, nil
}

func (s *staffingMemoryStore) GetDailyAttendanceResult(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
	return DailyAttendanceResult{}, false, nil
}

func (s *staffingMemoryStore) ListDailyAttendanceResultsForPerson(context.Context, string, string, string, string, int) ([]DailyAttendanceResult, error) {
	return nil, nil
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
