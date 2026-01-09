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
	listDateFn   func(ctx context.Context, tenantID string, workDate string, limit int) ([]DailyAttendanceResult, error)
	getFn        func(ctx context.Context, tenantID string, personUUID string, workDate string) (DailyAttendanceResult, bool, error)
	listPersonFn func(ctx context.Context, tenantID string, personUUID string, fromDate string, toDate string, limit int) ([]DailyAttendanceResult, error)
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

		storeOK := store
		storeOK.listDateFn = func(context.Context, string, string, int) ([]DailyAttendanceResult, error) {
			return []DailyAttendanceResult{{
				PersonUUID:      "p1",
				WorkDate:        "2026-01-01",
				RulesetVersion:  "R1",
				Status:          "PRESENT",
				Flags:           []string{"LATE"},
				FirstInTime:     &firstIn,
				LastOutTime:     &lastOut,
				WorkedMinutes:   480,
				InputPunchCount: 2,
				ComputedAt:      time.Unix(1, 0).UTC(),
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
		storeOK := store
		storeOK.getFn = func(context.Context, string, string, string) (DailyAttendanceResult, bool, error) {
			return DailyAttendanceResult{
				PersonUUID:             "p1",
				WorkDate:               "2026-01-01",
				RulesetVersion:         "R1",
				Status:                 "PRESENT",
				Flags:                  []string{"LATE"},
				FirstInTime:            &firstIn,
				LastOutTime:            &lastOut,
				InputMaxPunchEventDBID: &maxID,
				InputMaxPunchTime:      &maxPunchTime,
				WorkedMinutes:          480,
				InputPunchCount:        2,
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

		out := renderAttendanceDailyResultsDetail(tenant, "2026-01-01", "person-101", "2026-01-01", nil, Person{}, "bad path")
		if !strings.Contains(out, "bad path") {
			t.Fatalf("expected errMsg in html: %q", out)
		}
	})

	t.Run("render detail back link without work_date", func(t *testing.T) {
		t.Parallel()

		out := renderAttendanceDailyResultsDetail(tenant, "2026-01-01", "person-101", "", nil, Person{}, "")
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
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-daily-results?person_uuid=p1&to_date=BAD", nil).WithContext(ctx)
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

		storeOK := store
		storeOK.listPersonFn = func(context.Context, string, string, string, string, int) ([]DailyAttendanceResult, error) {
			return []DailyAttendanceResult{{PersonUUID: "p1", WorkDate: "2026-01-01", Status: "PRESENT"}}, nil
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
}
