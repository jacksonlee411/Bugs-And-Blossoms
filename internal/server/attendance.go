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
	DayType                *string    `json:"day_type"`
	Status                 string     `json:"status"`
	Flags                  []string   `json:"flags"`
	FirstInTime            *time.Time `json:"first_in_time"`
	LastOutTime            *time.Time `json:"last_out_time"`
	ScheduledMinutes       int        `json:"scheduled_minutes"`
	WorkedMinutes          int        `json:"worked_minutes"`
	OvertimeMinutes150     int        `json:"overtime_minutes_150"`
	OvertimeMinutes200     int        `json:"overtime_minutes_200"`
	OvertimeMinutes300     int        `json:"overtime_minutes_300"`
	LateMinutes            int        `json:"late_minutes"`
	EarlyLeaveMinutes      int        `json:"early_leave_minutes"`
	InputPunchCount        int        `json:"input_punch_count"`
	InputMaxPunchEventDBID *int64     `json:"input_max_punch_event_db_id"`
	InputMaxPunchTime      *time.Time `json:"input_max_punch_time"`
	TimeProfileLastEventID *int64     `json:"time_profile_last_event_id"`
	HolidayDayLastEventID  *int64     `json:"holiday_day_last_event_id"`
	ComputedAt             time.Time  `json:"computed_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type AttendanceTimeProfileForWorkDate struct {
	ShiftStartLocal            string    `json:"shift_start_local"`
	ShiftEndLocal              string    `json:"shift_end_local"`
	LateToleranceMinutes       int       `json:"late_tolerance_minutes"`
	EarlyLeaveToleranceMinutes int       `json:"early_leave_tolerance_minutes"`
	OvertimeMinMinutes         int       `json:"overtime_min_minutes"`
	OvertimeRoundingMode       string    `json:"overtime_rounding_mode"`
	OvertimeRoundingUnitMin    int       `json:"overtime_rounding_unit_minutes"`
	TimeProfileLastEventID     int64     `json:"time_profile_last_event_id"`
	ShiftStart                 time.Time `json:"shift_start"`
	ShiftEnd                   time.Time `json:"shift_end"`
	WindowStart                time.Time `json:"window_start"`
	WindowEnd                  time.Time `json:"window_end"`
}

type TimePunchWithVoid struct {
	EventDBID        int64           `json:"event_db_id"`
	EventID          string          `json:"event_id"`
	PersonUUID       string          `json:"person_uuid"`
	PunchTime        time.Time       `json:"punch_time"`
	PunchType        string          `json:"punch_type"`
	SourceProvider   string          `json:"source_provider"`
	Payload          json.RawMessage `json:"payload"`
	TransactionTime  time.Time       `json:"transaction_time"`
	VoidDBID         *int64          `json:"void_db_id,omitempty"`
	VoidEventID      *string         `json:"void_event_id,omitempty"`
	VoidPayload      json.RawMessage `json:"void_payload,omitempty"`
	VoidRequestID    *string         `json:"void_request_id,omitempty"`
	VoidInitiatorID  *string         `json:"void_initiator_id,omitempty"`
	VoidCreatedAt    *time.Time      `json:"void_created_at,omitempty"`
	VoidTxTime       *time.Time      `json:"void_transaction_time,omitempty"`
	TargetPunchDBID  *int64          `json:"target_punch_event_db_id,omitempty"`
	TargetPunchEvent *string         `json:"target_punch_event_id,omitempty"`
}

type AttendanceRecalcEvent struct {
	DBID            int64           `json:"id"`
	EventID         string          `json:"event_id"`
	PersonUUID      string          `json:"person_uuid"`
	FromDate        string          `json:"from_date"`
	ToDate          string          `json:"to_date"`
	Payload         json.RawMessage `json:"payload"`
	RequestID       string          `json:"request_id"`
	InitiatorID     string          `json:"initiator_id"`
	TransactionTime time.Time       `json:"transaction_time"`
	CreatedAt       time.Time       `json:"created_at"`
}

type SubmitTimePunchVoidParams struct {
	EventID            string
	TargetPunchEventID string
	Payload            json.RawMessage
}

type TimePunchVoidResult struct {
	DBID               int64  `json:"id"`
	EventID            string `json:"event_id"`
	TargetPunchEventID string `json:"target_punch_event_id"`
}

type SubmitAttendanceRecalcParams struct {
	EventID    string
	PersonUUID string
	FromDate   string
	ToDate     string
	Payload    json.RawMessage
}

type AttendanceRecalcResult struct {
	DBID       int64  `json:"id"`
	EventID    string `json:"event_id"`
	PersonUUID string `json:"person_uuid"`
	FromDate   string `json:"from_date"`
	ToDate     string `json:"to_date"`
}

type DailyAttendanceResultStore interface {
	ListDailyAttendanceResultsForDate(ctx context.Context, tenantID string, workDate string, limit int) ([]DailyAttendanceResult, error)
	GetDailyAttendanceResult(ctx context.Context, tenantID string, personUUID string, workDate string) (DailyAttendanceResult, bool, error)
	ListDailyAttendanceResultsForPerson(ctx context.Context, tenantID string, personUUID string, fromDate string, toDate string, limit int) ([]DailyAttendanceResult, error)
	GetAttendanceTimeProfileAndPunchesForWorkDate(ctx context.Context, tenantID string, personUUID string, workDate string) (AttendanceTimeProfileForWorkDate, []TimePunchWithVoid, error)
	ListAttendanceRecalcEventsForWorkDate(ctx context.Context, tenantID string, personUUID string, workDate string, limit int) ([]AttendanceRecalcEvent, error)
	SubmitTimePunchVoid(ctx context.Context, tenantID string, initiatorID string, p SubmitTimePunchVoidParams) (TimePunchVoidResult, error)
	SubmitAttendanceRecalc(ctx context.Context, tenantID string, initiatorID string, p SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error)
}

type TimeBankCycle struct {
	PersonUUID             string     `json:"person_uuid"`
	CycleType              string     `json:"cycle_type"`
	CycleStartDate         string     `json:"cycle_start_date"`
	CycleEndDate           string     `json:"cycle_end_date"`
	RulesetVersion         string     `json:"ruleset_version"`
	WorkedMinutesTotal     int        `json:"worked_minutes_total"`
	OvertimeMinutes150     int        `json:"overtime_minutes_150"`
	OvertimeMinutes200     int        `json:"overtime_minutes_200"`
	OvertimeMinutes300     int        `json:"overtime_minutes_300"`
	CompEarnedMinutes      int        `json:"comp_earned_minutes"`
	CompUsedMinutes        int        `json:"comp_used_minutes"`
	InputMaxPunchEventDBID *int64     `json:"input_max_punch_event_db_id"`
	InputMaxPunchTime      *time.Time `json:"input_max_punch_time"`
	ComputedAt             time.Time  `json:"computed_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type TimeBankCycleStore interface {
	GetTimeBankCycleForMonth(ctx context.Context, tenantID string, personUUID string, month string) (TimeBankCycle, bool, error)
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
  day_type,
  status,
  flags,
  first_in_time,
  last_out_time,
  scheduled_minutes,
  worked_minutes,
  overtime_minutes_150,
  overtime_minutes_200,
  overtime_minutes_300,
  late_minutes,
  early_leave_minutes,
  input_punch_count,
  input_max_punch_event_db_id,
  input_max_punch_time,
  time_profile_last_event_id,
  holiday_day_last_event_id,
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
		var dayType *string
		var firstIn *time.Time
		var lastOut *time.Time
		var inputMaxPunchEventDBID *int64
		var inputMaxPunchTime *time.Time
		var timeProfileLastEventID *int64
		var holidayDayLastEventID *int64
		if err := rows.Scan(
			&r.PersonUUID,
			&r.WorkDate,
			&r.RulesetVersion,
			&dayType,
			&r.Status,
			&r.Flags,
			&firstIn,
			&lastOut,
			&r.ScheduledMinutes,
			&r.WorkedMinutes,
			&r.OvertimeMinutes150,
			&r.OvertimeMinutes200,
			&r.OvertimeMinutes300,
			&r.LateMinutes,
			&r.EarlyLeaveMinutes,
			&r.InputPunchCount,
			&inputMaxPunchEventDBID,
			&inputMaxPunchTime,
			&timeProfileLastEventID,
			&holidayDayLastEventID,
			&r.ComputedAt,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}

		r.DayType = dayType
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
		r.TimeProfileLastEventID = timeProfileLastEventID
		r.HolidayDayLastEventID = holidayDayLastEventID
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
	var dayType *string
	var firstIn *time.Time
	var lastOut *time.Time
	var inputMaxPunchEventDBID *int64
	var inputMaxPunchTime *time.Time
	var timeProfileLastEventID *int64
	var holidayDayLastEventID *int64
	err = tx.QueryRow(ctx, `
SELECT
  person_uuid::text,
  work_date::text,
  ruleset_version,
  day_type,
  status,
  flags,
  first_in_time,
  last_out_time,
  scheduled_minutes,
  worked_minutes,
  overtime_minutes_150,
  overtime_minutes_200,
  overtime_minutes_300,
  late_minutes,
  early_leave_minutes,
  input_punch_count,
  input_max_punch_event_db_id,
  input_max_punch_time,
  time_profile_last_event_id,
  holiday_day_last_event_id,
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
		&dayType,
		&out.Status,
		&out.Flags,
		&firstIn,
		&lastOut,
		&out.ScheduledMinutes,
		&out.WorkedMinutes,
		&out.OvertimeMinutes150,
		&out.OvertimeMinutes200,
		&out.OvertimeMinutes300,
		&out.LateMinutes,
		&out.EarlyLeaveMinutes,
		&out.InputPunchCount,
		&inputMaxPunchEventDBID,
		&inputMaxPunchTime,
		&timeProfileLastEventID,
		&holidayDayLastEventID,
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

	out.DayType = dayType
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
	out.TimeProfileLastEventID = timeProfileLastEventID
	out.HolidayDayLastEventID = holidayDayLastEventID
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
  day_type,
  status,
  flags,
  first_in_time,
  last_out_time,
  scheduled_minutes,
  worked_minutes,
  overtime_minutes_150,
  overtime_minutes_200,
  overtime_minutes_300,
  late_minutes,
  early_leave_minutes,
  input_punch_count,
  input_max_punch_event_db_id,
  input_max_punch_time,
  time_profile_last_event_id,
  holiday_day_last_event_id,
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
		var dayType *string
		var firstIn *time.Time
		var lastOut *time.Time
		var inputMaxPunchEventDBID *int64
		var inputMaxPunchTime *time.Time
		var timeProfileLastEventID *int64
		var holidayDayLastEventID *int64
		if err := rows.Scan(
			&r.PersonUUID,
			&r.WorkDate,
			&r.RulesetVersion,
			&dayType,
			&r.Status,
			&r.Flags,
			&firstIn,
			&lastOut,
			&r.ScheduledMinutes,
			&r.WorkedMinutes,
			&r.OvertimeMinutes150,
			&r.OvertimeMinutes200,
			&r.OvertimeMinutes300,
			&r.LateMinutes,
			&r.EarlyLeaveMinutes,
			&r.InputPunchCount,
			&inputMaxPunchEventDBID,
			&inputMaxPunchTime,
			&timeProfileLastEventID,
			&holidayDayLastEventID,
			&r.ComputedAt,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}

		r.DayType = dayType
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
		r.TimeProfileLastEventID = timeProfileLastEventID
		r.HolidayDayLastEventID = holidayDayLastEventID
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

func (s *staffingPGStore) GetAttendanceTimeProfileAndPunchesForWorkDate(ctx context.Context, tenantID string, personUUID string, workDate string) (AttendanceTimeProfileForWorkDate, []TimePunchWithVoid, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AttendanceTimeProfileForWorkDate{}, nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return AttendanceTimeProfileForWorkDate{}, nil, err
	}

	personUUID = strings.TrimSpace(personUUID)
	if personUUID == "" {
		return AttendanceTimeProfileForWorkDate{}, nil, errors.New("person_uuid is required")
	}
	workDate = strings.TrimSpace(workDate)
	if workDate == "" {
		return AttendanceTimeProfileForWorkDate{}, nil, errors.New("work_date is required")
	}

	var tp AttendanceTimeProfileForWorkDate
	if err := tx.QueryRow(ctx, `
SELECT
  shift_start_local::text,
  shift_end_local::text,
  late_tolerance_minutes,
  early_leave_tolerance_minutes,
  overtime_min_minutes,
  overtime_rounding_mode,
  overtime_rounding_unit_minutes,
  time_profile_last_event_id,
  shift_start,
  shift_end,
  window_start,
  window_end
FROM staffing.get_time_profile_for_work_date($1::uuid, $2::date)
`, tenantID, workDate).Scan(
		&tp.ShiftStartLocal,
		&tp.ShiftEndLocal,
		&tp.LateToleranceMinutes,
		&tp.EarlyLeaveToleranceMinutes,
		&tp.OvertimeMinMinutes,
		&tp.OvertimeRoundingMode,
		&tp.OvertimeRoundingUnitMin,
		&tp.TimeProfileLastEventID,
		&tp.ShiftStart,
		&tp.ShiftEnd,
		&tp.WindowStart,
		&tp.WindowEnd,
	); err != nil {
		return AttendanceTimeProfileForWorkDate{}, nil, err
	}
	tp.ShiftStart = tp.ShiftStart.UTC()
	tp.ShiftEnd = tp.ShiftEnd.UTC()
	tp.WindowStart = tp.WindowStart.UTC()
	tp.WindowEnd = tp.WindowEnd.UTC()

	rows, err := tx.Query(ctx, `
SELECT
  e.id,
  e.event_id::text,
  e.person_uuid::text,
  e.punch_time,
  e.punch_type,
  e.source_provider,
  e.payload,
  e.transaction_time,
  v.id,
  v.event_id::text,
  v.payload,
  v.request_id,
  v.initiator_id::text,
  v.created_at,
  v.transaction_time,
  v.target_punch_event_db_id,
  v.target_punch_event_id::text
FROM staffing.time_punch_events e
LEFT JOIN staffing.time_punch_void_events v
  ON v.tenant_id = e.tenant_id
 AND v.target_punch_event_db_id = e.id
WHERE e.tenant_id = $1::uuid
  AND e.person_uuid = $2::uuid
  AND e.punch_time >= $3
  AND e.punch_time < $4
ORDER BY e.punch_time ASC, e.id ASC
`, tenantID, personUUID, tp.WindowStart, tp.WindowEnd)
	if err != nil {
		return AttendanceTimeProfileForWorkDate{}, nil, err
	}
	defer rows.Close()

	var punches []TimePunchWithVoid
	for rows.Next() {
		var p TimePunchWithVoid
		var payload []byte
		var txTime time.Time
		var voidDBID *int64
		var voidEventID *string
		var voidPayload []byte
		var voidRequestID *string
		var voidInitiatorID *string
		var voidCreatedAt *time.Time
		var voidTxTime *time.Time
		var targetPunchDBID *int64
		var targetPunchEventID *string
		if err := rows.Scan(
			&p.EventDBID,
			&p.EventID,
			&p.PersonUUID,
			&p.PunchTime,
			&p.PunchType,
			&p.SourceProvider,
			&payload,
			&txTime,
			&voidDBID,
			&voidEventID,
			&voidPayload,
			&voidRequestID,
			&voidInitiatorID,
			&voidCreatedAt,
			&voidTxTime,
			&targetPunchDBID,
			&targetPunchEventID,
		); err != nil {
			return AttendanceTimeProfileForWorkDate{}, nil, err
		}

		p.PunchTime = p.PunchTime.UTC()
		p.TransactionTime = txTime.UTC()
		p.Payload = json.RawMessage(payload)

		p.VoidDBID = voidDBID
		p.VoidEventID = voidEventID
		p.VoidPayload = json.RawMessage(voidPayload)
		p.VoidRequestID = voidRequestID
		p.VoidInitiatorID = voidInitiatorID
		if voidCreatedAt != nil {
			tm := voidCreatedAt.UTC()
			p.VoidCreatedAt = &tm
		}
		if voidTxTime != nil {
			tm := voidTxTime.UTC()
			p.VoidTxTime = &tm
		}
		p.TargetPunchDBID = targetPunchDBID
		p.TargetPunchEvent = targetPunchEventID

		punches = append(punches, p)
	}
	if err := rows.Err(); err != nil {
		return AttendanceTimeProfileForWorkDate{}, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AttendanceTimeProfileForWorkDate{}, nil, err
	}
	return tp, punches, nil
}

func (s *staffingPGStore) ListAttendanceRecalcEventsForWorkDate(ctx context.Context, tenantID string, personUUID string, workDate string, limit int) ([]AttendanceRecalcEvent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	personUUID = strings.TrimSpace(personUUID)
	if personUUID == "" {
		return nil, errors.New("person_uuid is required")
	}
	workDate = strings.TrimSpace(workDate)
	if workDate == "" {
		return nil, errors.New("work_date is required")
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	rows, err := tx.Query(ctx, `
SELECT
  id,
  event_id::text,
  person_uuid::text,
  from_date::text,
  to_date::text,
  payload,
  request_id,
  initiator_id::text,
  transaction_time,
  created_at
FROM staffing.attendance_recalc_events
WHERE tenant_id = $1::uuid
  AND person_uuid = $2::uuid
  AND from_date <= $3::date
  AND to_date >= $3::date
ORDER BY created_at DESC, id DESC
LIMIT $4
`, tenantID, personUUID, workDate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AttendanceRecalcEvent
	for rows.Next() {
		var e AttendanceRecalcEvent
		var payload []byte
		if err := rows.Scan(
			&e.DBID,
			&e.EventID,
			&e.PersonUUID,
			&e.FromDate,
			&e.ToDate,
			&payload,
			&e.RequestID,
			&e.InitiatorID,
			&e.TransactionTime,
			&e.CreatedAt,
		); err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(payload)
		e.TransactionTime = e.TransactionTime.UTC()
		e.CreatedAt = e.CreatedAt.UTC()
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *staffingPGStore) SubmitTimePunchVoid(ctx context.Context, tenantID string, initiatorID string, p SubmitTimePunchVoidParams) (TimePunchVoidResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TimePunchVoidResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return TimePunchVoidResult{}, err
	}

	if strings.TrimSpace(initiatorID) == "" {
		return TimePunchVoidResult{}, errors.New("initiator_id is required")
	}

	if p.EventID == "" {
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&p.EventID); err != nil {
			return TimePunchVoidResult{}, err
		}
	}

	p.TargetPunchEventID = strings.TrimSpace(p.TargetPunchEventID)
	if p.TargetPunchEventID == "" {
		return TimePunchVoidResult{}, errors.New("target_punch_event_id is required")
	}

	payload := json.RawMessage(p.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	requestID := p.EventID

	var id int64
	if err := tx.QueryRow(ctx, `
SELECT staffing.submit_time_punch_void_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::jsonb,
  $5::text,
  $6::uuid
)
`, p.EventID, tenantID, p.TargetPunchEventID, []byte(payload), requestID, initiatorID).Scan(&id); err != nil {
		return TimePunchVoidResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return TimePunchVoidResult{}, err
	}

	return TimePunchVoidResult{
		DBID:               id,
		EventID:            p.EventID,
		TargetPunchEventID: p.TargetPunchEventID,
	}, nil
}

func (s *staffingPGStore) SubmitAttendanceRecalc(ctx context.Context, tenantID string, initiatorID string, p SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AttendanceRecalcResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return AttendanceRecalcResult{}, err
	}

	if strings.TrimSpace(initiatorID) == "" {
		return AttendanceRecalcResult{}, errors.New("initiator_id is required")
	}

	if p.EventID == "" {
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&p.EventID); err != nil {
			return AttendanceRecalcResult{}, err
		}
	}

	p.PersonUUID = strings.TrimSpace(p.PersonUUID)
	if p.PersonUUID == "" {
		return AttendanceRecalcResult{}, errors.New("person_uuid is required")
	}
	p.FromDate = strings.TrimSpace(p.FromDate)
	p.ToDate = strings.TrimSpace(p.ToDate)
	if p.FromDate == "" || p.ToDate == "" {
		return AttendanceRecalcResult{}, errors.New("from_date/to_date is required")
	}

	payload := json.RawMessage(p.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	requestID := p.EventID

	var id int64
	if err := tx.QueryRow(ctx, `
SELECT staffing.submit_attendance_recalc_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::date,
  $5::date,
  $6::jsonb,
  $7::text,
  $8::uuid
)
`, p.EventID, tenantID, p.PersonUUID, p.FromDate, p.ToDate, []byte(payload), requestID, initiatorID).Scan(&id); err != nil {
		return AttendanceRecalcResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AttendanceRecalcResult{}, err
	}

	return AttendanceRecalcResult{
		DBID:       id,
		EventID:    p.EventID,
		PersonUUID: p.PersonUUID,
		FromDate:   p.FromDate,
		ToDate:     p.ToDate,
	}, nil
}

func (s *staffingPGStore) GetTimeBankCycleForMonth(ctx context.Context, tenantID string, personUUID string, month string) (TimeBankCycle, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TimeBankCycle{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return TimeBankCycle{}, false, err
	}

	personUUID = strings.TrimSpace(personUUID)
	if personUUID == "" {
		return TimeBankCycle{}, false, errors.New("person_uuid is required")
	}

	month = strings.TrimSpace(month)
	if month == "" {
		return TimeBankCycle{}, false, errors.New("month is required")
	}
	tm, err := time.Parse("2006-01", month)
	if err != nil {
		return TimeBankCycle{}, false, err
	}
	cycleStart := time.Date(tm.Year(), tm.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	var out TimeBankCycle
	var inputMaxPunchEventDBID *int64
	var inputMaxPunchTime *time.Time
	err = tx.QueryRow(ctx, `
SELECT
  person_uuid::text,
  cycle_type,
  cycle_start_date::text,
  cycle_end_date::text,
  ruleset_version,
  worked_minutes_total,
  overtime_minutes_150,
  overtime_minutes_200,
  overtime_minutes_300,
  comp_earned_minutes,
  comp_used_minutes,
  input_max_punch_event_db_id,
  input_max_punch_time,
  computed_at,
  created_at,
  updated_at
FROM staffing.time_bank_cycles
WHERE tenant_id = $1::uuid
  AND person_uuid = $2::uuid
  AND cycle_type = 'MONTH'
  AND cycle_start_date = $3::date
`, tenantID, personUUID, cycleStart).Scan(
		&out.PersonUUID,
		&out.CycleType,
		&out.CycleStartDate,
		&out.CycleEndDate,
		&out.RulesetVersion,
		&out.WorkedMinutesTotal,
		&out.OvertimeMinutes150,
		&out.OvertimeMinutes200,
		&out.OvertimeMinutes300,
		&out.CompEarnedMinutes,
		&out.CompUsedMinutes,
		&inputMaxPunchEventDBID,
		&inputMaxPunchTime,
		&out.ComputedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TimeBankCycle{}, false, nil
		}
		return TimeBankCycle{}, false, err
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
		return TimeBankCycle{}, false, err
	}
	return out, true, nil
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

func (s *staffingMemoryStore) GetAttendanceTimeProfileAndPunchesForWorkDate(context.Context, string, string, string) (AttendanceTimeProfileForWorkDate, []TimePunchWithVoid, error) {
	return AttendanceTimeProfileForWorkDate{}, nil, nil
}

func (s *staffingMemoryStore) ListAttendanceRecalcEventsForWorkDate(context.Context, string, string, string, int) ([]AttendanceRecalcEvent, error) {
	return nil, nil
}

func (s *staffingMemoryStore) SubmitTimePunchVoid(context.Context, string, string, SubmitTimePunchVoidParams) (TimePunchVoidResult, error) {
	return TimePunchVoidResult{}, nil
}

func (s *staffingMemoryStore) SubmitAttendanceRecalc(context.Context, string, string, SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
	return AttendanceRecalcResult{}, nil
}

func (s *staffingMemoryStore) GetTimeBankCycleForMonth(context.Context, string, string, string) (TimeBankCycle, bool, error) {
	return TimeBankCycle{}, false, nil
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
