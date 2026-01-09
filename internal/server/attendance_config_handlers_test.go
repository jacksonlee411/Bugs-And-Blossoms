package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type stubAttendanceConfigStore struct {
	getTimeProfileAsOf      func(ctx context.Context, tenantID string, asOfDate string) (TimeProfileVersion, bool, error)
	listTimeProfileVersions func(ctx context.Context, tenantID string, limit int) ([]TimeProfileVersion, error)
	upsertTimeProfile       func(ctx context.Context, tenantID string, initiatorID string, effectiveDate string, payload map[string]any) error

	listHolidayDayOverrides func(ctx context.Context, tenantID string, fromDate string, toDate string, limit int) ([]HolidayDayOverride, error)
	setHolidayDayOverride   func(ctx context.Context, tenantID string, initiatorID string, dayDate string, payload map[string]any) error
	clearHolidayDayOverride func(ctx context.Context, tenantID string, initiatorID string, dayDate string) error
}

func (s stubAttendanceConfigStore) GetTimeProfileAsOf(ctx context.Context, tenantID string, asOfDate string) (TimeProfileVersion, bool, error) {
	if s.getTimeProfileAsOf == nil {
		return TimeProfileVersion{}, false, nil
	}
	return s.getTimeProfileAsOf(ctx, tenantID, asOfDate)
}
func (s stubAttendanceConfigStore) ListTimeProfileVersions(ctx context.Context, tenantID string, limit int) ([]TimeProfileVersion, error) {
	if s.listTimeProfileVersions == nil {
		return nil, nil
	}
	return s.listTimeProfileVersions(ctx, tenantID, limit)
}
func (s stubAttendanceConfigStore) UpsertTimeProfile(ctx context.Context, tenantID string, initiatorID string, effectiveDate string, payload map[string]any) error {
	if s.upsertTimeProfile == nil {
		return nil
	}
	return s.upsertTimeProfile(ctx, tenantID, initiatorID, effectiveDate, payload)
}
func (s stubAttendanceConfigStore) ListHolidayDayOverrides(ctx context.Context, tenantID string, fromDate string, toDate string, limit int) ([]HolidayDayOverride, error) {
	if s.listHolidayDayOverrides == nil {
		return nil, nil
	}
	return s.listHolidayDayOverrides(ctx, tenantID, fromDate, toDate, limit)
}
func (s stubAttendanceConfigStore) SetHolidayDayOverride(ctx context.Context, tenantID string, initiatorID string, dayDate string, payload map[string]any) error {
	if s.setHolidayDayOverride == nil {
		return nil
	}
	return s.setHolidayDayOverride(ctx, tenantID, initiatorID, dayDate, payload)
}
func (s stubAttendanceConfigStore) ClearHolidayDayOverride(ctx context.Context, tenantID string, initiatorID string, dayDate string) error {
	if s.clearHolidayDayOverride == nil {
		return nil
	}
	return s.clearHolidayDayOverride(ctx, tenantID, initiatorID, dayDate)
}

func hxReq(method, target string, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("HX-Request", "true")
	return req
}

func withTenantAndPrincipal(req *http.Request, includePrincipal bool) *http.Request {
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	if includePrincipal {
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
	}
	return req
}

func TestAttendanceTimeProfileHandlers(t *testing.T) {
	store := stubAttendanceConfigStore{
		getTimeProfileAsOf: func(context.Context, string, string) (TimeProfileVersion, bool, error) {
			return TimeProfileVersion{
				Name:                        "Default",
				LifecycleStatus:             "active",
				EffectiveDate:               "2026-01-01",
				ShiftStartLocal:             "09:00",
				ShiftEndLocal:               "18:00",
				LateToleranceMinutes:        0,
				EarlyLeaveToleranceMinutes:  0,
				OvertimeMinMinutes:          0,
				OvertimeRoundingMode:        "NONE",
				OvertimeRoundingUnitMinutes: 0,
				LastEventDBID:               1,
			}, true, nil
		},
		listTimeProfileVersions: func(context.Context, string, int) ([]TimeProfileVersion, error) {
			return []TimeProfileVersion{{EffectiveDate: "2026-01-01", ShiftStartLocal: "09:00", ShiftEndLocal: "18:00", LastEventDBID: 1}}, nil
		},
	}

	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleAttendanceTimeProfile(rec, hxReq(http.MethodGet, "/org/attendance-time-profile?as_of=2026-01-01", ""), store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("as_of missing redirects", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-time-profile", ""), false)
		handleAttendanceTimeProfile(rec, req, store)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-time-profile?as_of=bad", ""), false)
		handleAttendanceTimeProfile(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("GET with current and versions", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-time-profile?as_of=2026-01-01", ""), false)
		handleAttendanceTimeProfile(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Attendance / TimeProfile") {
			t.Fatalf("body=%q", rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "Versions") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("GET without current and without versions", func(t *testing.T) {
		emptyStore := stubAttendanceConfigStore{
			getTimeProfileAsOf: func(context.Context, string, string) (TimeProfileVersion, bool, error) {
				return TimeProfileVersion{}, false, nil
			},
			listTimeProfileVersions: func(context.Context, string, int) ([]TimeProfileVersion, error) {
				return nil, nil
			},
		}
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-time-profile?as_of=2026-01-01", ""), false)
		handleAttendanceTimeProfile(rec, req, emptyStore)
		if rec.Code != http.StatusOK ||
			!strings.Contains(rec.Body.String(), "(no active time profile as-of)") ||
			!strings.Contains(rec.Body.String(), "(no versions)") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET store error (current)", func(t *testing.T) {
		errStore := store
		errStore.getTimeProfileAsOf = func(context.Context, string, string) (TimeProfileVersion, bool, error) {
			return TimeProfileVersion{}, false, errors.New("db err")
		}
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-time-profile?as_of=2026-01-01", ""), false)
		handleAttendanceTimeProfile(rec, req, errStore)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "db err") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET store error (versions)", func(t *testing.T) {
		errStore := store
		errStore.listTimeProfileVersions = func(context.Context, string, int) ([]TimeProfileVersion, error) {
			return nil, errors.New("db err")
		}
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-time-profile?as_of=2026-01-01", ""), false)
		handleAttendanceTimeProfile(rec, req, errStore)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "db err") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST bad form", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-time-profile?as_of=2026-01-01", "op=%ZZ"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceTimeProfile(rec, req, store)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "bad form") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST principal missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-time-profile?as_of=2026-01-01", "op=save&effective_date=2026-01-01&shift_start_local=09:00&shift_end_local=18:00"), false)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceTimeProfile(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST unsupported op", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-time-profile?as_of=2026-01-01", "op=nope"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceTimeProfile(rec, req, store)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "unsupported op") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	postCases := []struct {
		name    string
		body    string
		wantSub string
	}{
		{"effective_date required", "op=save&shift_start_local=09:00&shift_end_local=18:00", "effective_date is required"},
		{"effective_date invalid", "op=save&effective_date=2026-13-01&shift_start_local=09:00&shift_end_local=18:00", "effective_date 无效"},
		{"shift required", "op=save&effective_date=2026-01-01", "shift_start_local and shift_end_local are required"},
		{"shift_start invalid", "op=save&effective_date=2026-01-01&shift_start_local=nope&shift_end_local=18:00", "shift_start_local 无效"},
		{"shift_end invalid", "op=save&effective_date=2026-01-01&shift_start_local=09:00&shift_end_local=x", "shift_end_local 无效"},
		{"shift_end <= start", "op=save&effective_date=2026-01-01&shift_start_local=09:00&shift_end_local=08:00", "shift_end_local must be"},
		{"late tol invalid", "op=save&effective_date=2026-01-01&shift_start_local=09:00&shift_end_local=18:00&late_tolerance_minutes=-1", "late_tolerance_minutes 无效"},
		{"early tol invalid", "op=save&effective_date=2026-01-01&shift_start_local=09:00&shift_end_local=18:00&early_leave_tolerance_minutes=-1", "early_leave_tolerance_minutes 无效"},
		{"ot min invalid", "op=save&effective_date=2026-01-01&shift_start_local=09:00&shift_end_local=18:00&overtime_min_minutes=x", "overtime_min_minutes 无效"},
		{"ot unit invalid", "op=save&effective_date=2026-01-01&shift_start_local=09:00&shift_end_local=18:00&overtime_rounding_unit_minutes=x", "overtime_rounding_unit_minutes 无效"},
		{"ot mode invalid", "op=save&effective_date=2026-01-01&shift_start_local=09:00&shift_end_local=18:00&overtime_rounding_mode=BAD", "overtime_rounding_mode must be NONE|FLOOR|CEIL|NEAREST"},
	}
	for _, tc := range postCases {
		t.Run("POST validation: "+tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-time-profile?as_of=2026-01-01", tc.body), true)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			handleAttendanceTimeProfile(rec, req, store)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.wantSub) {
				t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}

	t.Run("POST store error", func(t *testing.T) {
		errStore := store
		errStore.upsertTimeProfile = func(context.Context, string, string, string, map[string]any) error {
			return errors.New("db err")
		}
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-time-profile?as_of=2026-01-01", "op=save&effective_date=2026-01-01&shift_start_local=09:00&shift_end_local=18:00"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceTimeProfile(rec, req, errStore)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "db err") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST success redirect", func(t *testing.T) {
		okStore := store
		okStore.upsertTimeProfile = func(context.Context, string, string, string, map[string]any) error { return nil }
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-time-profile?as_of=2026-01-01", "op=save&effective_date=2026-01-01&shift_start_local=09:00&shift_end_local=18:00"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceTimeProfile(rec, req, okStore)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST success with name", func(t *testing.T) {
		okStore := store
		okStore.upsertTimeProfile = func(context.Context, string, string, string, map[string]any) error { return nil }
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-time-profile?as_of=2026-01-01", "op=save&effective_date=2026-01-01&shift_start_local=09:00&shift_end_local=18:00&name=Default"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceTimeProfile(rec, req, okStore)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPut, "/org/attendance-time-profile?as_of=2026-01-01", ""), true)
		handleAttendanceTimeProfile(rec, req, store)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestAttendanceHolidayCalendarHandlers(t *testing.T) {
	store := stubAttendanceConfigStore{
		listHolidayDayOverrides: func(context.Context, string, string, string, int) ([]HolidayDayOverride, error) { return nil, nil },
	}

	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleAttendanceHolidayCalendar(rec, hxReq(http.MethodGet, "/org/attendance-holiday-calendar?as_of=2026-01-01", ""), store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("as_of missing redirects", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-holiday-calendar", ""), false)
		handleAttendanceHolidayCalendar(rec, req, store)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-holiday-calendar?as_of=bad", ""), false)
		handleAttendanceHolidayCalendar(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("month invalid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=bad", ""), false)
		handleAttendanceHolidayCalendar(rec, req, store)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "month 无效") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET default month", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-holiday-calendar?as_of=2026-01-01", ""), false)
		handleAttendanceHolidayCalendar(rec, req, store)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Import (CSV)") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET with override", func(t *testing.T) {
		ovStore := store
		ovStore.listHolidayDayOverrides = func(context.Context, string, string, string, int) ([]HolidayDayOverride, error) {
			return []HolidayDayOverride{{DayDate: "2026-01-01", DayType: "LEGAL_HOLIDAY", HolidayCode: "NY", Note: "n", LastEventDBID: 1}}, nil
		}
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", ""), false)
		handleAttendanceHolidayCalendar(rec, req, ovStore)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "yes") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET list error", func(t *testing.T) {
		errStore := store
		errStore.listHolidayDayOverrides = func(context.Context, string, string, string, int) ([]HolidayDayOverride, error) {
			return nil, errors.New("db err")
		}
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodGet, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", ""), false)
		handleAttendanceHolidayCalendar(rec, req, errStore)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "db err") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST bad form", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=%ZZ"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, store)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "bad form") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST principal missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=day_set&day_date=2026-01-01&day_type=WORKDAY"), false)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST day_set validations", func(t *testing.T) {
		cases := []struct {
			body string
			sub  string
		}{
			{"op=day_set&day_type=WORKDAY", "day_date is required"},
			{"op=day_set&day_date=2026-13-01&day_type=WORKDAY", "day_date 无效"},
			{"op=day_set&day_date=2026-01-01&day_type=BAD", "day_type must be WORKDAY|RESTDAY|LEGAL_HOLIDAY"},
		}
		for _, tc := range cases {
			rec := httptest.NewRecorder()
			req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", tc.body), true)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			handleAttendanceHolidayCalendar(rec, req, store)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.sub) {
				t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("POST day_set store error", func(t *testing.T) {
		errStore := store
		errStore.setHolidayDayOverride = func(context.Context, string, string, string, map[string]any) error {
			return errors.New("db err")
		}
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=day_set&day_date=2026-01-01&day_type=WORKDAY"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, errStore)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "db err") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST day_set success redirect", func(t *testing.T) {
		okStore := store
		okStore.setHolidayDayOverride = func(context.Context, string, string, string, map[string]any) error { return nil }
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=day_set&day_date=2026-01-01&day_type=WORKDAY"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, okStore)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST day_set with holiday_code and note", func(t *testing.T) {
		okStore := store
		okStore.setHolidayDayOverride = func(context.Context, string, string, string, map[string]any) error { return nil }
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=day_set&day_date=2026-01-01&day_type=LEGAL_HOLIDAY&holiday_code=NY&note=n"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, okStore)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST day_clear validations", func(t *testing.T) {
		cases := []struct {
			body string
			sub  string
		}{
			{"op=day_clear", "day_date is required"},
			{"op=day_clear&day_date=2026-13-01", "day_date 无效"},
		}
		for _, tc := range cases {
			rec := httptest.NewRecorder()
			req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", tc.body), true)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			handleAttendanceHolidayCalendar(rec, req, store)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.sub) {
				t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("POST day_clear store error", func(t *testing.T) {
		errStore := store
		errStore.clearHolidayDayOverride = func(context.Context, string, string, string) error {
			return errors.New("db err")
		}
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=day_clear&day_date=2026-01-01"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, errStore)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "db err") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST day_clear success redirect", func(t *testing.T) {
		okStore := store
		okStore.clearHolidayDayOverride = func(context.Context, string, string, string) error { return nil }
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=day_clear&day_date=2026-01-01"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, okStore)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST import_csv too large", func(t *testing.T) {
		rec := httptest.NewRecorder()
		csv := strings.Repeat("a", 256*1024+1)
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=import_csv&csv="+csv), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, store)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "csv too large") {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST import_csv empty", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=import_csv&csv="), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, store)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "csv is required") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST import_csv line validations", func(t *testing.T) {
		cases := []struct {
			csv string
			sub string
		}{
			{"2026-01-01", "expected 2-4 columns"},
			{",WORKDAY", "day_date is required"},
			{"bad,WORKDAY", "invalid day_date"},
			{"2026-01-01,BAD", "invalid day_type"},
		}
		for _, tc := range cases {
			rec := httptest.NewRecorder()
			req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=import_csv&csv="+url.QueryEscape(tc.csv)), true)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			handleAttendanceHolidayCalendar(rec, req, store)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.sub) {
				t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("POST import_csv store error", func(t *testing.T) {
		errStore := store
		errStore.setHolidayDayOverride = func(context.Context, string, string, string, map[string]any) error {
			return errors.New("db err")
		}
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=import_csv&csv=2026-01-01,WORKDAY"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, errStore)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "line 1") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST import_csv success redirect", func(t *testing.T) {
		okStore := store
		okStore.setHolidayDayOverride = func(context.Context, string, string, string, map[string]any) error { return nil }
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=import_csv&csv=2026-01-01,WORKDAY,NY,note"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, okStore)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST unsupported op", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", "op=nope"), true)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handleAttendanceHolidayCalendar(rec, req, store)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "unsupported op") {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withTenantAndPrincipal(hxReq(http.MethodPut, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", ""), true)
		handleAttendanceHolidayCalendar(rec, req, store)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestParseOptionalNonNegInt_Positive(t *testing.T) {
	n, err := parseOptionalNonNegInt("5")
	if err != nil || n != 5 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestRenderAttendanceTimeProfile_SelectRoundingMode(t *testing.T) {
	out := renderAttendanceTimeProfile(
		Tenant{ID: "t1"},
		"2026-01-01",
		map[string]string{"overtime_rounding_mode": "CEIL"},
		nil,
		"",
		nil,
	)
	if !strings.Contains(out, `<option value="CEIL" selected>CEIL</option>`) {
		t.Fatalf("out=%q", out)
	}
}
