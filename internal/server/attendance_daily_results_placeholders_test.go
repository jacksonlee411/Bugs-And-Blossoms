package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeDailyAttendanceResultStore struct {
	listDateFn     func(ctx context.Context, tenantID string, workDate string, limit int) ([]DailyAttendanceResult, error)
	getFn          func(ctx context.Context, tenantID string, personUUID string, workDate string) (DailyAttendanceResult, bool, error)
	listPersonFn   func(ctx context.Context, tenantID string, personUUID string, fromDate string, toDate string, limit int) ([]DailyAttendanceResult, error)
	auditFn        func(ctx context.Context, tenantID string, personUUID string, workDate string) (AttendanceTimeProfileForWorkDate, []TimePunchWithVoid, error)
	recalcEventsFn func(ctx context.Context, tenantID string, personUUID string, workDate string, limit int) ([]AttendanceRecalcEvent, error)
	submitVoidFn   func(ctx context.Context, tenantID string, initiatorID string, p SubmitTimePunchVoidParams) (TimePunchVoidResult, error)
	submitRecalcFn func(ctx context.Context, tenantID string, initiatorID string, p SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error)
}

func (s fakeDailyAttendanceResultStore) ListDailyAttendanceResultsForDate(ctx context.Context, tenantID string, workDate string, limit int) ([]DailyAttendanceResult, error) {
	return s.listDateFn(ctx, tenantID, workDate, limit)
}

func (s fakeDailyAttendanceResultStore) GetDailyAttendanceResult(ctx context.Context, tenantID string, personUUID string, workDate string) (DailyAttendanceResult, bool, error) {
	return s.getFn(ctx, tenantID, personUUID, workDate)
}

func (s fakeDailyAttendanceResultStore) ListDailyAttendanceResultsForPerson(ctx context.Context, tenantID string, personUUID string, fromDate string, toDate string, limit int) ([]DailyAttendanceResult, error) {
	return s.listPersonFn(ctx, tenantID, personUUID, fromDate, toDate, limit)
}

func (s fakeDailyAttendanceResultStore) GetAttendanceTimeProfileAndPunchesForWorkDate(ctx context.Context, tenantID string, personUUID string, workDate string) (AttendanceTimeProfileForWorkDate, []TimePunchWithVoid, error) {
	return s.auditFn(ctx, tenantID, personUUID, workDate)
}

func (s fakeDailyAttendanceResultStore) ListAttendanceRecalcEventsForWorkDate(ctx context.Context, tenantID string, personUUID string, workDate string, limit int) ([]AttendanceRecalcEvent, error) {
	return s.recalcEventsFn(ctx, tenantID, personUUID, workDate, limit)
}

func (s fakeDailyAttendanceResultStore) SubmitTimePunchVoid(ctx context.Context, tenantID string, initiatorID string, p SubmitTimePunchVoidParams) (TimePunchVoidResult, error) {
	return s.submitVoidFn(ctx, tenantID, initiatorID, p)
}

func (s fakeDailyAttendanceResultStore) SubmitAttendanceRecalc(ctx context.Context, tenantID string, initiatorID string, p SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
	return s.submitRecalcFn(ctx, tenantID, initiatorID, p)
}

func TestAttendanceDailyResultsHandlers_Coverage(t *testing.T) {
	t.Parallel()

	tenant := Tenant{ID: "t1", Name: "Tenant 1"}
	store := fakeDailyAttendanceResultStore{
		listDateFn: func(context.Context, string, string, int) ([]DailyAttendanceResult, error) { return nil, nil },
		getFn: func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{}, false, nil
		},
		listPersonFn: func(context.Context, string, string, string, string, int) ([]DailyAttendanceResult, error) {
			return nil, nil
		},
		auditFn: func(context.Context, string, string, string) (AttendanceTimeProfileForWorkDate, []TimePunchWithVoid, error) {
			return AttendanceTimeProfileForWorkDate{}, nil, nil
		},
		recalcEventsFn: func(context.Context, string, string, string, int) ([]AttendanceRecalcEvent, error) {
			return nil, nil
		},
		submitVoidFn: func(context.Context, string, string, SubmitTimePunchVoidParams) (TimePunchVoidResult, error) {
			return TimePunchVoidResult{}, nil
		},
		submitRecalcFn: func(context.Context, string, string, SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
			return AttendanceRecalcResult{}, nil
		},
	}
	personStore := fakePersonStore{}

	t.Run("list tenant missing", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResults(rec, req, store, personStore)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list missing as_of redirects", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResults(rec, req, store, personStore)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "as_of=") {
			t.Fatalf("missing redirect as_of: %q", loc)
		}
	})

	t.Run("list invalid work_date", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results?as_of=2026-01-01&work_date=BAD", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResults(rec, req, store, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "work_date") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("list persons error", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results?as_of=2026-01-01&work_date=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResults(rec, req, store, fakePersonStore{listPersonsErr: errors.New("boom")})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "boom") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("list store error", func(t *testing.T) {
		t.Parallel()

		storeErr := store
		storeErr.listDateFn = func(context.Context, string, string, int) ([]DailyAttendanceResult, error) {
			return nil, errors.New("list fail")
		}

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results?as_of=2026-01-01&work_date=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResults(rec, req, storeErr, fakePersonStore{persons: []Person{{UUID: "p1", Pernr: "1", DisplayName: "Alice"}}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "list fail") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("list ok (with results)", func(t *testing.T) {
		t.Parallel()

		loc := time.FixedZone("X", 8*60*60)
		firstIn := time.Date(2026, 1, 1, 9, 0, 0, 0, loc)
		lastOut := time.Date(2026, 1, 1, 18, 0, 0, 0, loc)
		dayType := "WORKDAY"

		storeOK := store
		storeOK.listDateFn = func(context.Context, string, string, int) ([]DailyAttendanceResult, error) {
			return []DailyAttendanceResult{{
				PersonUUID:         "p1",
				WorkDate:           "2026-01-01",
				RulesetVersion:     "R1",
				DayType:            &dayType,
				Status:             "PRESENT",
				Flags:              []string{"LATE"},
				FirstInTime:        &firstIn,
				LastOutTime:        &lastOut,
				ScheduledMinutes:   540,
				WorkedMinutes:      480,
				OvertimeMinutes150: 10,
				InputPunchCount:    2,
				ComputedAt:         time.Unix(1, 0).UTC(),
			}}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results?as_of=2026-01-01&work_date=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResults(rec, req, storeOK, fakePersonStore{persons: []Person{{UUID: "p1", Pernr: "1", DisplayName: "Alice"}}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Attendance / Daily Results") || !strings.Contains(rec.Body.String(), "PRESENT") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("detail tenant missing", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/person-101/2026-01-01?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, store, personStore)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail missing as_of redirects", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/person-101/2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, store, personStore)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail bad path", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/bad?as_of=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, store, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail store error", func(t *testing.T) {
		t.Parallel()

		storeErr := store
		storeErr.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{}, false, errors.New("get fail")
		}

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/person-101/2026-01-01?as_of=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeErr, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "get fail") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("detail found ok", func(t *testing.T) {
		t.Parallel()

		firstIn := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
		lastOut := time.Date(2026, 1, 1, 18, 0, 0, 0, time.UTC)
		maxID := int64(123)
		maxPunchTime := time.Date(2026, 1, 1, 18, 0, 0, 0, time.UTC)
		timeProfileLastEventID := int64(1001)
		holidayDayLastEventID := int64(2001)
		dayType := "WORKDAY"
		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{
				PersonUUID:             "p1",
				WorkDate:               "2026-01-01",
				RulesetVersion:         "R1",
				DayType:                &dayType,
				Status:                 "PRESENT",
				Flags:                  []string{"LATE"},
				FirstInTime:            &firstIn,
				LastOutTime:            &lastOut,
				ScheduledMinutes:       540,
				InputMaxPunchEventDBID: &maxID,
				InputMaxPunchTime:      &maxPunchTime,
				WorkedMinutes:          480,
				OvertimeMinutes150:     10,
				InputPunchCount:        2,
				TimeProfileLastEventID: &timeProfileLastEventID,
				HolidayDayLastEventID:  &holidayDayLastEventID,
				ComputedAt:             time.Unix(1, 0).UTC(),
			}, true, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, fakePersonStore{persons: []Person{{UUID: "p1", Pernr: "1", DisplayName: "Alice"}}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Summary") || !strings.Contains(rec.Body.String(), "PRESENT") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("detail invalid work_date", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/person-101/BAD?as_of=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, store, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("parse coverage", func(t *testing.T) {
		t.Parallel()

		if _, _, ok := parseAttendanceDailyResultsDetailPath(""); ok {
			t.Fatal("expected not ok")
		}
		if _, _, ok := parseAttendanceDailyResultsDetailPath("/org/other/person/2026-01-01"); ok {
			t.Fatal("expected not ok")
		}
		if _, _, ok := parseAttendanceDailyResultsDetailPath("/org/attendance-daily-results/person/"); ok {
			t.Fatal("expected not ok")
		}
		if _, _, ok := parseAttendanceDailyResultsDetailPath("/org/attendance-daily-results//2026-01-01"); ok {
			t.Fatal("expected not ok")
		}
		if _, _, ok := parseAttendanceDailyResultsDetailPath("/org/attendance-daily-results/person-101/"); ok {
			t.Fatal("expected not ok")
		}
		p, d, ok := parseAttendanceDailyResultsDetailPath("/org/attendance-daily-results/person-101/2026-01-01")
		if !ok || p != "person-101" || d != "2026-01-01" {
			t.Fatalf("unexpected parse: ok=%v person=%q date=%q", ok, p, d)
		}
	})

	t.Run("render errMsg branch", func(t *testing.T) {
		t.Parallel()

		out := renderAttendanceDailyResultsDetail(tenant, "2026-01-01", "person-101", "2026-01-01", nil, Person{}, nil, nil, nil, "bad path")
		if !strings.Contains(out, "bad path") {
			t.Fatalf("expected errMsg in html: %q", out)
		}
	})

	t.Run("render detail back link without work_date", func(t *testing.T) {
		t.Parallel()

		out := renderAttendanceDailyResultsDetail(tenant, "2026-01-01", "person-101", "", nil, Person{}, nil, nil, nil, "")
		if strings.Contains(out, "work_date=") {
			t.Fatalf("unexpected work_date param: %q", out)
		}
	})

	t.Run("render list workDate empty branch", func(t *testing.T) {
		t.Parallel()

		out := renderAttendanceDailyResults(nil, nil, tenant, "2026-01-01", "", "")
		if !strings.Contains(out, "pick a work date") {
			t.Fatalf("out=%q", out)
		}
	})

	t.Run("api tenant missing", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1", nil)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api principal missing", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1", nil).
			WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api missing person_uuid", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api default date range (no from/to)", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		var resp attendanceDailyResultsAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if resp.PersonUUID != "p1" || resp.FromDate == "" || resp.FromDate != resp.ToDate {
			t.Fatalf("resp=%+v", resp)
		}
	})

	t.Run("api invalid from_date", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1&from_date=BAD", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api invalid to_date", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1&from_date=2026-01-01&to_date=BAD", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api invalid date range", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1&from_date=2026-01-02&to_date=2026-01-01", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api invalid limit", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1&limit=BAD", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api from_date empty uses to_date", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1&to_date=2026-01-01", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api to_date empty uses from_date", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1&from_date=2026-01-01", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api limit clamp > 2000", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1&from_date=2026-01-01&to_date=2026-01-01&limit=999999", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api store error", func(t *testing.T) {
		t.Parallel()

		storeErr := store
		storeErr.listPersonFn = func(context.Context, string, string, string, string, int) ([]DailyAttendanceResult, error) {
			return nil, errors.New("list fail")
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1&from_date=2026-01-01&to_date=2026-01-01", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, storeErr)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api ok + json", func(t *testing.T) {
		t.Parallel()

		dayType := "WORKDAY"
		storeOK := store
		storeOK.listPersonFn = func(context.Context, string, string, string, string, int) ([]DailyAttendanceResult, error) {
			return []DailyAttendanceResult{{PersonUUID: "p1", WorkDate: "2026-01-01", DayType: &dayType, Status: "PRESENT", ScheduledMinutes: 540}}, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1&from_date=2026-01-01&to_date=2026-01-01&limit=0", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, storeOK)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		var resp attendanceDailyResultsAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if resp.PersonUUID != "p1" || len(resp.Results) != 1 {
			t.Fatalf("resp=%+v", resp)
		}
	})

	t.Run("api ok with explicit limit", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.listPersonFn = func(context.Context, string, string, string, string, int) ([]DailyAttendanceResult, error) {
			return nil, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1&from_date=2026-01-01&to_date=2026-01-01&limit=10", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, storeOK)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("api method not allowed", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-daily-results?person_uuid=p1", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsAPI(rec, req, store)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail found ok (audit with punches + recalc events)", func(t *testing.T) {
		t.Parallel()

		firstIn := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
		lastOut := time.Date(2026, 1, 1, 18, 0, 0, 0, time.UTC)
		maxID := int64(123)
		maxPunchTime := time.Date(2026, 1, 1, 18, 0, 0, 0, time.UTC)
		timeProfileLastEventID := int64(1001)
		holidayDayLastEventID := int64(2001)
		dayType := "WORKDAY"

		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{
				PersonUUID:             "p1",
				WorkDate:               "2026-01-01",
				RulesetVersion:         "R1",
				DayType:                &dayType,
				Status:                 "PRESENT",
				Flags:                  []string{"LATE"},
				FirstInTime:            &firstIn,
				LastOutTime:            &lastOut,
				ScheduledMinutes:       540,
				InputMaxPunchEventDBID: &maxID,
				InputMaxPunchTime:      &maxPunchTime,
				WorkedMinutes:          480,
				OvertimeMinutes150:     10,
				InputPunchCount:        2,
				TimeProfileLastEventID: &timeProfileLastEventID,
				HolidayDayLastEventID:  &holidayDayLastEventID,
				ComputedAt:             time.Unix(1, 0).UTC(),
			}, true, nil
		}

		voidDBID := int64(99)
		voidEventID := "void-1"
		voidCreatedAt := time.Date(2026, 1, 2, 1, 2, 3, 0, time.UTC)
		storeOK.auditFn = func(context.Context, string, string, string) (AttendanceTimeProfileForWorkDate, []TimePunchWithVoid, error) {
			tp := AttendanceTimeProfileForWorkDate{
				ShiftStartLocal:        "09:00:00",
				ShiftEndLocal:          "18:00:00",
				TimeProfileLastEventID: 123,
				WindowStart:            time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				WindowEnd:              time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			}
			return tp, []TimePunchWithVoid{
				{
					EventDBID:       1,
					EventID:         "punch-1",
					PersonUUID:      "p1",
					PunchTime:       time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC),
					PunchType:       "IN",
					SourceProvider:  "MANUAL",
					Payload:         json.RawMessage(`{}`),
					TransactionTime: time.Date(2026, 1, 1, 1, 1, 0, 0, time.UTC),
				},
				{
					EventDBID:       2,
					EventID:         "punch-2",
					PersonUUID:      "p1",
					PunchTime:       time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC),
					PunchType:       "OUT",
					SourceProvider:  "IMPORT",
					Payload:         json.RawMessage(`{}`),
					TransactionTime: time.Date(2026, 1, 1, 9, 1, 0, 0, time.UTC),
					VoidDBID:        &voidDBID,
					VoidEventID:     &voidEventID,
					VoidCreatedAt:   &voidCreatedAt,
					VoidPayload:     json.RawMessage(`{"reason":"mistake"}`),
				},
			}, nil
		}
		storeOK.recalcEventsFn = func(context.Context, string, string, string, int) ([]AttendanceRecalcEvent, error) {
			return []AttendanceRecalcEvent{{
				DBID:       1,
				EventID:    "recalc-1",
				PersonUUID: "p1",
				FromDate:   "2026-01-01",
				ToDate:     "2026-01-02",
				Payload:    json.RawMessage(`{"source":"test"}`),
				CreatedAt:  time.Date(2026, 1, 2, 2, 0, 0, 0, time.UTC),
			}}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, fakePersonStore{persons: []Person{{UUID: "p1", Pernr: "1", DisplayName: "Alice"}}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Corrections") || !strings.Contains(rec.Body.String(), "Audit") || !strings.Contains(rec.Body.String(), "VOIDED") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("detail audit load errors are shown", func(t *testing.T) {
		t.Parallel()

		storeErr := store
		storeErr.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT", ComputedAt: time.Unix(1, 0).UTC()}, true, nil
		}
		storeErr.auditFn = func(context.Context, string, string, string) (AttendanceTimeProfileForWorkDate, []TimePunchWithVoid, error) {
			return AttendanceTimeProfileForWorkDate{}, nil, errors.New("audit boom")
		}
		storeErr.recalcEventsFn = func(context.Context, string, string, string, int) ([]AttendanceRecalcEvent, error) {
			return nil, errors.New("recalc boom")
		}

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeErr, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "audit load failed") || !strings.Contains(rec.Body.String(), "audit recalc load failed") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("detail POST bad form", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT", ComputedAt: time.Unix(1, 0).UTC()}, true, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=void_punch&target_punch_event_id=%zz")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "bad form") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("detail POST principal missing", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=void_punch&target_punch_event_id=t1")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, store, personStore)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail POST invalid op", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=unknown")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, store, personStore)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail method not allowed", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		req := httptest.NewRequest(http.MethodPut, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, store, personStore)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail POST void punch missing target", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT", ComputedAt: time.Unix(1, 0).UTC()}, true, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=void_punch")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail POST void punch submit error", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT", ComputedAt: time.Unix(1, 0).UTC()}, true, nil
		}
		storeOK.submitVoidFn = func(context.Context, string, string, SubmitTimePunchVoidParams) (TimePunchVoidResult, error) {
			return TimePunchVoidResult{}, errors.New("boom")
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=void_punch&target_punch_event_id=t1")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "boom") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("detail POST void punch ok redirects", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		var got SubmitTimePunchVoidParams
		storeOK.submitVoidFn = func(_ context.Context, _ string, _ string, p SubmitTimePunchVoidParams) (TimePunchVoidResult, error) {
			got = p
			return TimePunchVoidResult{DBID: 1, EventID: "e1", TargetPunchEventID: "t1"}, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=void_punch&target_punch_event_id=t1&reason=mistake")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if got.TargetPunchEventID != "t1" {
			t.Fatalf("got target=%q", got.TargetPunchEventID)
		}
		var payload map[string]any
		if err := json.Unmarshal(got.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["source"] != "ui" || payload["reason"] != "mistake" {
			t.Fatalf("payload=%v", payload)
		}
	})

	t.Run("detail POST recalc day ok redirects", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		var got SubmitAttendanceRecalcParams
		storeOK.submitRecalcFn = func(_ context.Context, _ string, _ string, p SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
			got = p
			return AttendanceRecalcResult{DBID: 1, EventID: "e1", PersonUUID: "p1", FromDate: "2026-01-01", ToDate: "2026-01-01"}, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=recalc_day&reason=fix")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		var payload map[string]any
		if err := json.Unmarshal(got.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["source"] != "ui" || payload["reason"] != "fix" {
			t.Fatalf("payload=%v", payload)
		}
	})

	t.Run("detail POST recalc day submit error", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT", ComputedAt: time.Unix(1, 0).UTC()}, true, nil
		}
		storeOK.submitRecalcFn = func(context.Context, string, string, SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
			return AttendanceRecalcResult{}, errors.New("boom")
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=recalc_day")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "boom") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("detail POST recalc range missing date", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT", ComputedAt: time.Unix(1, 0).UTC()}, true, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=recalc_range")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail POST recalc range invalid from_date", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT", ComputedAt: time.Unix(1, 0).UTC()}, true, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=recalc_range&from_date=BAD&to_date=2026-01-01")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail POST recalc range invalid to_date", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT", ComputedAt: time.Unix(1, 0).UTC()}, true, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=recalc_range&from_date=2026-01-01&to_date=BAD")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail POST recalc range invalid range", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT", ComputedAt: time.Unix(1, 0).UTC()}, true, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=recalc_range&from_date=2026-01-02&to_date=2026-01-01")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail POST recalc range too large", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT", ComputedAt: time.Unix(1, 0).UTC()}, true, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=recalc_range&from_date=2026-01-01&to_date=2026-02-01")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail POST recalc range ok redirects", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		var got SubmitAttendanceRecalcParams
		storeOK.submitRecalcFn = func(_ context.Context, _ string, _ string, p SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
			got = p
			return AttendanceRecalcResult{DBID: 1, EventID: "e1", PersonUUID: "p1", FromDate: "2026-01-01", ToDate: "2026-01-02"}, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=recalc_range&from_date=2026-01-01&to_date=2026-01-02&reason=fix")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		var payload map[string]any
		if err := json.Unmarshal(got.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["source"] != "ui" || payload["reason"] != "fix" {
			t.Fatalf("payload=%v", payload)
		}
	})

	t.Run("detail POST recalc range submit error", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT", ComputedAt: time.Unix(1, 0).UTC()}, true, nil
		}
		storeOK.submitRecalcFn = func(context.Context, string, string, SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
			return AttendanceRecalcResult{}, errors.New("boom")
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-daily-results/p1/2026-01-01?as_of=2026-01-01", strings.NewReader("op=recalc_range&from_date=2026-01-01&to_date=2026-01-02")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetail(rec, req, storeOK, personStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "boom") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("internal api punch void tenant missing", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punch-voids", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		handleAttendancePunchVoidsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api punch void principal missing", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punch-voids", strings.NewReader(`{}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendancePunchVoidsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api punch void method not allowed", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-punch-voids", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendancePunchVoidsAPI(rec, req, store)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api punch void bad json", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punch-voids", strings.NewReader("{")).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendancePunchVoidsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api punch void missing target", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punch-voids", strings.NewReader(`{"target_punch_event_id":""}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendancePunchVoidsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api punch void idempotency reused", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.submitVoidFn = func(context.Context, string, string, SubmitTimePunchVoidParams) (TimePunchVoidResult, error) {
			return TimePunchVoidResult{}, errors.New("STAFFING_IDEMPOTENCY_REUSED: boom")
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punch-voids", strings.NewReader(`{"target_punch_event_id":"t1"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendancePunchVoidsAPI(rec, req, storeOK)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api punch void target not found", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.submitVoidFn = func(context.Context, string, string, SubmitTimePunchVoidParams) (TimePunchVoidResult, error) {
			return TimePunchVoidResult{}, errors.New("STAFFING_TIME_PUNCH_EVENT_NOT_FOUND: boom")
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punch-voids", strings.NewReader(`{"target_punch_event_id":"t1"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendancePunchVoidsAPI(rec, req, storeOK)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api punch void submit failed", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.submitVoidFn = func(context.Context, string, string, SubmitTimePunchVoidParams) (TimePunchVoidResult, error) {
			return TimePunchVoidResult{}, errors.New("boom")
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punch-voids", strings.NewReader(`{"target_punch_event_id":"t1"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendancePunchVoidsAPI(rec, req, storeOK)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api punch void ok", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.submitVoidFn = func(context.Context, string, string, SubmitTimePunchVoidParams) (TimePunchVoidResult, error) {
			return TimePunchVoidResult{DBID: 1, EventID: "e1", TargetPunchEventID: "t1"}, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punch-voids", strings.NewReader(`{"target_punch_event_id":"t1"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendancePunchVoidsAPI(rec, req, storeOK)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc tenant missing", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc principal missing", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc method not allowed", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-recalc", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, store)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc bad json", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader("{")).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc missing person_uuid", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{"from_date":"2026-01-01"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc missing date range", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{"person_uuid":"p1"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc defaults from_date to to_date", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.submitRecalcFn = func(_ context.Context, _ string, _ string, p SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
			if p.FromDate != p.ToDate || p.FromDate != "2026-01-01" {
				t.Fatalf("from=%q to=%q", p.FromDate, p.ToDate)
			}
			return AttendanceRecalcResult{DBID: 1, EventID: "e1", PersonUUID: p.PersonUUID, FromDate: p.FromDate, ToDate: p.ToDate}, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{"person_uuid":"p1","to_date":"2026-01-01"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, storeOK)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc defaults to_date to from_date", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.submitRecalcFn = func(_ context.Context, _ string, _ string, p SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
			if p.FromDate != p.ToDate || p.ToDate != "2026-01-01" {
				t.Fatalf("from=%q to=%q", p.FromDate, p.ToDate)
			}
			return AttendanceRecalcResult{DBID: 1, EventID: "e1", PersonUUID: p.PersonUUID, FromDate: p.FromDate, ToDate: p.ToDate}, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{"person_uuid":"p1","from_date":"2026-01-01"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, storeOK)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc invalid from_date", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{"person_uuid":"p1","from_date":"BAD","to_date":"2026-01-01"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc invalid to_date", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{"person_uuid":"p1","from_date":"2026-01-01","to_date":"BAD"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc invalid range", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{"person_uuid":"p1","from_date":"2026-01-02","to_date":"2026-01-01"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc range too large", func(t *testing.T) {
		t.Parallel()

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{"person_uuid":"p1","from_date":"2026-01-01","to_date":"2026-02-01"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc idempotency reused", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.submitRecalcFn = func(context.Context, string, string, SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
			return AttendanceRecalcResult{}, errors.New("STAFFING_IDEMPOTENCY_REUSED: boom")
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{"person_uuid":"p1","from_date":"2026-01-01","to_date":"2026-01-01"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, storeOK)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc submit failed", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.submitRecalcFn = func(context.Context, string, string, SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
			return AttendanceRecalcResult{}, errors.New("boom")
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{"person_uuid":"p1","from_date":"2026-01-01","to_date":"2026-01-01"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, storeOK)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("internal api recalc ok", func(t *testing.T) {
		t.Parallel()

		storeOK := store
		storeOK.submitRecalcFn = func(context.Context, string, string, SubmitAttendanceRecalcParams) (AttendanceRecalcResult, error) {
			return AttendanceRecalcResult{DBID: 1, EventID: "e1", PersonUUID: "p1", FromDate: "2026-01-01", ToDate: "2026-01-01"}, nil
		}

		ctx := withTenant(t.Context(), tenant)
		ctx = withPrincipal(ctx, Principal{ID: "i1", TenantID: tenant.ID, RoleSlug: "tenant-admin", Status: "active"})
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-recalc", strings.NewReader(`{"person_uuid":"p1","from_date":"2026-01-01","to_date":"2026-01-01"}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceRecalcAPI(rec, req, storeOK)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}
