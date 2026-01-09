package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

type TimeProfileVersion struct {
	Name                        string
	LifecycleStatus             string
	EffectiveDate               string
	ShiftStartLocal             string
	ShiftEndLocal               string
	LateToleranceMinutes        int
	EarlyLeaveToleranceMinutes  int
	OvertimeMinMinutes          int
	OvertimeRoundingMode        string
	OvertimeRoundingUnitMinutes int
	LastEventDBID               int64
}

type HolidayDayOverride struct {
	DayDate       string
	DayType       string
	HolidayCode   string
	Note          string
	LastEventDBID int64
}

type AttendanceConfigStore interface {
	GetTimeProfileAsOf(ctx context.Context, tenantID string, asOfDate string) (TimeProfileVersion, bool, error)
	ListTimeProfileVersions(ctx context.Context, tenantID string, limit int) ([]TimeProfileVersion, error)
	UpsertTimeProfile(ctx context.Context, tenantID string, initiatorID string, effectiveDate string, payload map[string]any) error

	ListHolidayDayOverrides(ctx context.Context, tenantID string, fromDate string, toDate string, limit int) ([]HolidayDayOverride, error)
	SetHolidayDayOverride(ctx context.Context, tenantID string, initiatorID string, dayDate string, payload map[string]any) error
	ClearHolidayDayOverride(ctx context.Context, tenantID string, initiatorID string, dayDate string) error
}

func (s *staffingPGStore) GetTimeProfileAsOf(ctx context.Context, tenantID string, asOfDate string) (TimeProfileVersion, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TimeProfileVersion{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return TimeProfileVersion{}, false, err
	}

	var out TimeProfileVersion
	err = tx.QueryRow(ctx, `
SELECT
  COALESCE(name, '') AS name,
  lifecycle_status,
  lower(validity)::text AS effective_date,
  to_char(shift_start_local, 'HH24:MI') AS shift_start_local,
  to_char(shift_end_local, 'HH24:MI') AS shift_end_local,
  late_tolerance_minutes,
  early_leave_tolerance_minutes,
  overtime_min_minutes,
  overtime_rounding_mode,
  overtime_rounding_unit_minutes,
  last_event_id
FROM staffing.time_profile_versions
WHERE tenant_id = $1::uuid
  AND lifecycle_status = 'active'
  AND validity @> $2::date
ORDER BY lower(validity) DESC, id DESC
LIMIT 1
`, tenantID, asOfDate).Scan(
		&out.Name,
		&out.LifecycleStatus,
		&out.EffectiveDate,
		&out.ShiftStartLocal,
		&out.ShiftEndLocal,
		&out.LateToleranceMinutes,
		&out.EarlyLeaveToleranceMinutes,
		&out.OvertimeMinMinutes,
		&out.OvertimeRoundingMode,
		&out.OvertimeRoundingUnitMinutes,
		&out.LastEventDBID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.Commit(ctx); err != nil {
				return TimeProfileVersion{}, false, err
			}
			return TimeProfileVersion{}, false, nil
		}
		return TimeProfileVersion{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return TimeProfileVersion{}, false, err
	}
	return out, true, nil
}

func (s *staffingPGStore) ListTimeProfileVersions(ctx context.Context, tenantID string, limit int) ([]TimeProfileVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	rows, err := tx.Query(ctx, `
SELECT
  COALESCE(name, '') AS name,
  lifecycle_status,
  lower(validity)::text AS effective_date,
  to_char(shift_start_local, 'HH24:MI') AS shift_start_local,
  to_char(shift_end_local, 'HH24:MI') AS shift_end_local,
  late_tolerance_minutes,
  early_leave_tolerance_minutes,
  overtime_min_minutes,
  overtime_rounding_mode,
  overtime_rounding_unit_minutes,
  last_event_id
FROM staffing.time_profile_versions
WHERE tenant_id = $1::uuid
ORDER BY lower(validity) DESC, id DESC
LIMIT $2
`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TimeProfileVersion
	for rows.Next() {
		var v TimeProfileVersion
		if err := rows.Scan(
			&v.Name,
			&v.LifecycleStatus,
			&v.EffectiveDate,
			&v.ShiftStartLocal,
			&v.ShiftEndLocal,
			&v.LateToleranceMinutes,
			&v.EarlyLeaveToleranceMinutes,
			&v.OvertimeMinMinutes,
			&v.OvertimeRoundingMode,
			&v.OvertimeRoundingUnitMinutes,
			&v.LastEventDBID,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *staffingPGStore) UpsertTimeProfile(ctx context.Context, tenantID string, initiatorID string, effectiveDate string, payload map[string]any) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return errors.New("effective_date is required")
	}

	var hasAny bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM staffing.time_profile_events WHERE tenant_id=$1::uuid);`, tenantID).Scan(&hasAny); err != nil {
		return err
	}
	eventType := "UPDATE"
	if !hasAny {
		eventType = "CREATE"
	}

	payloadJSON, _ := json.Marshal(payload)
	if _, err := tx.Exec(ctx, `
SELECT staffing.submit_time_profile_event(
  gen_random_uuid(),
  $1::uuid,
  $2::text,
  $3::date,
  $4::jsonb,
  gen_random_uuid()::text,
  $5::uuid
);
`, tenantID, eventType, effectiveDate, payloadJSON, initiatorID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *staffingPGStore) ListHolidayDayOverrides(ctx context.Context, tenantID string, fromDate string, toDate string, limit int) ([]HolidayDayOverride, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 2000
	}
	if limit > 5000 {
		limit = 5000
	}

	rows, err := tx.Query(ctx, `
SELECT
  day_date::text,
  day_type,
  COALESCE(holiday_code, ''),
  COALESCE(note, ''),
  last_event_id
FROM staffing.holiday_days
WHERE tenant_id = $1::uuid
  AND day_date >= $2::date
  AND day_date < $3::date
ORDER BY day_date ASC
LIMIT $4
`, tenantID, fromDate, toDate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HolidayDayOverride
	for rows.Next() {
		var d HolidayDayOverride
		if err := rows.Scan(&d.DayDate, &d.DayType, &d.HolidayCode, &d.Note, &d.LastEventDBID); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *staffingPGStore) SetHolidayDayOverride(ctx context.Context, tenantID string, initiatorID string, dayDate string, payload map[string]any) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	dayDate = strings.TrimSpace(dayDate)
	if dayDate == "" {
		return errors.New("day_date is required")
	}

	payloadJSON, _ := json.Marshal(payload)
	if _, err := tx.Exec(ctx, `
SELECT staffing.submit_holiday_day_event(
  gen_random_uuid(),
  $1::uuid,
  $2::date,
  'SET',
  $3::jsonb,
  gen_random_uuid()::text,
  $4::uuid
);
`, tenantID, dayDate, payloadJSON, initiatorID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *staffingPGStore) ClearHolidayDayOverride(ctx context.Context, tenantID string, initiatorID string, dayDate string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	dayDate = strings.TrimSpace(dayDate)
	if dayDate == "" {
		return errors.New("day_date is required")
	}

	if _, err := tx.Exec(ctx, `
SELECT staffing.submit_holiday_day_event(
  gen_random_uuid(),
  $1::uuid,
  $2::date,
  'CLEAR',
  '{}'::jsonb,
  gen_random_uuid()::text,
  $3::uuid
);
`, tenantID, dayDate, initiatorID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *staffingMemoryStore) GetTimeProfileAsOf(_ context.Context, _ string, _ string) (TimeProfileVersion, bool, error) {
	return TimeProfileVersion{}, false, nil
}

func (s *staffingMemoryStore) ListTimeProfileVersions(_ context.Context, _ string, _ int) ([]TimeProfileVersion, error) {
	return nil, nil
}

func (s *staffingMemoryStore) UpsertTimeProfile(_ context.Context, _ string, _ string, _ string, _ map[string]any) error {
	return nil
}

func (s *staffingMemoryStore) ListHolidayDayOverrides(_ context.Context, _ string, _ string, _ string, _ int) ([]HolidayDayOverride, error) {
	return nil, nil
}

func (s *staffingMemoryStore) SetHolidayDayOverride(_ context.Context, _ string, _ string, _ string, _ map[string]any) error {
	return nil
}

func (s *staffingMemoryStore) ClearHolidayDayOverride(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
