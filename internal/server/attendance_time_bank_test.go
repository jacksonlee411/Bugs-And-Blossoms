package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeTimeBankCycleStore struct {
	getFn func(ctx context.Context, tenantID string, personUUID string, month string) (TimeBankCycle, bool, error)
}

func (s fakeTimeBankCycleStore) GetTimeBankCycleForMonth(ctx context.Context, tenantID string, personUUID string, month string) (TimeBankCycle, bool, error) {
	return s.getFn(ctx, tenantID, personUUID, month)
}

func TestMonthStartEndDates(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		_, _, err := monthStartEndDates("")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		_, _, err := monthStartEndDates("BAD")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		start, end, err := monthStartEndDates(" 2026-02 ")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if start != "2026-02-01" {
			t.Fatalf("start=%s", start)
		}
		if end != "2026-02-28" {
			t.Fatalf("end=%s", end)
		}
	})
}

func TestRenderAttendanceTimeBank(t *testing.T) {
	t.Parallel()

	tenant := Tenant{ID: "t1", Name: "Tenant 1"}

	t.Run("pick person", func(t *testing.T) {
		t.Parallel()

		out := renderAttendanceTimeBank(nil, false, nil, []Person{{UUID: "p1", Pernr: "1", DisplayName: "Alice"}}, tenant, "2026-01-01", "", "2026-01", "2026-01-01", "2026-01-31", "")
		if !strings.Contains(out, "(pick a person)") {
			t.Fatalf("body=%q", out)
		}
	})

	t.Run("cycle missing (person not in list)", func(t *testing.T) {
		t.Parallel()

		out := renderAttendanceTimeBank(nil, false, nil, nil, tenant, "2026-01-01", "p1", "2026-01", "2026-01-01", "2026-01-31", "")
		if !strings.Contains(out, "Person: <code>p1</code>") {
			t.Fatalf("body=%q", out)
		}
		if !strings.Contains(out, "(no cycle computed yet)") {
			t.Fatalf("body=%q", out)
		}
		if !strings.Contains(out, "(no daily results)") {
			t.Fatalf("body=%q", out)
		}
	})

	t.Run("cycle found + results", func(t *testing.T) {
		t.Parallel()

		tmLocal := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("Z", 8*60*60))
		cycle := TimeBankCycle{
			PersonUUID:         "p1",
			CycleType:          "MONTH",
			CycleStartDate:     "2026-01-01",
			CycleEndDate:       "2026-01-31",
			RulesetVersion:     "v1",
			WorkedMinutesTotal: 480,
			OvertimeMinutes200: 120,
			CompEarnedMinutes:  120,
			ComputedAt:         tmLocal,
			CreatedAt:          tmLocal,
			UpdatedAt:          tmLocal,
		}

		restDay := "RESTDAY"
		results := []DailyAttendanceResult{
			{PersonUUID: "p1", WorkDate: "2026-01-05", DayType: &restDay, Status: "OK", WorkedMinutes: 480, OvertimeMinutes200: 120, ComputedAt: tmLocal},
			{PersonUUID: "p1", WorkDate: "2026-01-06", Status: "OK", WorkedMinutes: 480, ComputedAt: tmLocal},
		}

		out := renderAttendanceTimeBank(&cycle, true, results, []Person{{UUID: "p1", Pernr: "1", DisplayName: "Alice"}}, tenant, "2026-01-15", "p1", "2026-01", "2026-01-01", "2026-01-31", "")
		if !strings.Contains(out, "Comp Earned Minutes") {
			t.Fatalf("body=%q", out)
		}
		if !strings.Contains(out, "/org/attendance-daily-results/p1/2026-01-05") {
			t.Fatalf("body=%q", out)
		}
		if !strings.Contains(out, "RESTDAY") {
			t.Fatalf("body=%q", out)
		}
	})
}

func TestHandleAttendanceTimeBank(t *testing.T) {
	t.Parallel()

	people := []Person{{UUID: "p1", Pernr: "1", DisplayName: "Alice"}}
	tenant := Tenant{ID: "t1", Name: "Tenant 1"}

	t.Run("tenant missing", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-time-bank?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleAttendanceTimeBank(rec, req, fakeTimeBankCycleStore{}, fakeDailyAttendanceResultStore{}, fakePersonStore{persons: people})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("as_of missing redirects", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-time-bank", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceTimeBank(rec, req, fakeTimeBankCycleStore{}, fakeDailyAttendanceResultStore{}, fakePersonStore{persons: people})
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "as_of=") {
			t.Fatalf("location=%q", loc)
		}
	})

	t.Run("persons error", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-time-bank?as_of=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceTimeBank(rec, req, fakeTimeBankCycleStore{}, fakeDailyAttendanceResultStore{}, fakePersonStore{listPersonsErr: errors.New("boom")})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "boom") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("month invalid", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-time-bank?as_of=2026-01-01&month=BAD", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceTimeBank(rec, req, fakeTimeBankCycleStore{}, fakeDailyAttendanceResultStore{}, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "month 无效") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("person empty does not call stores", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-time-bank?as_of=2026-02-15", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()

		cycleStore := fakeTimeBankCycleStore{
			getFn: func(context.Context, string, string, string) (TimeBankCycle, bool, error) {
				t.Fatal("unexpected cycle store call")
				return TimeBankCycle{}, false, nil
			},
		}
		dailyStore := fakeDailyAttendanceResultStore{
			listPersonFn: func(context.Context, string, string, string, string, int) ([]DailyAttendanceResult, error) {
				t.Fatal("unexpected daily results store call")
				return nil, nil
			},
		}

		handleAttendanceTimeBank(rec, req, cycleStore, dailyStore, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "(pick a person)") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("cycle + daily results errors merge", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-time-bank?as_of=2026-02-15&person_uuid=p1", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()

		var gotMonth string
		cycleStore := fakeTimeBankCycleStore{
			getFn: func(_ context.Context, _ string, _ string, month string) (TimeBankCycle, bool, error) {
				gotMonth = month
				return TimeBankCycle{}, false, errors.New("cycle fail")
			},
		}
		dailyStore := fakeDailyAttendanceResultStore{
			listPersonFn: func(context.Context, string, string, string, string, int) ([]DailyAttendanceResult, error) {
				return nil, errors.New("results fail")
			},
		}

		handleAttendanceTimeBank(rec, req, cycleStore, dailyStore, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if gotMonth != "2026-02" {
			t.Fatalf("month=%q", gotMonth)
		}
		if !strings.Contains(rec.Body.String(), "cycle fail") || !strings.Contains(rec.Body.String(), "results fail") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("cycle found + daily results filtered", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-time-bank?as_of=2026-02-15&person_uuid=p1&month=2026-02", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()

		cycleStore := fakeTimeBankCycleStore{
			getFn: func(context.Context, string, string, string) (TimeBankCycle, bool, error) {
				return TimeBankCycle{CycleType: "MONTH", CompEarnedMinutes: 60, ComputedAt: time.Unix(1, 0).UTC()}, true, nil
			},
		}
		dailyStore := fakeDailyAttendanceResultStore{
			listPersonFn: func(context.Context, string, string, string, string, int) ([]DailyAttendanceResult, error) {
				return []DailyAttendanceResult{
					{PersonUUID: "p1", WorkDate: "2026-02-01", Status: "OK", ComputedAt: time.Unix(1, 0).UTC()},
					{PersonUUID: "p1", WorkDate: "2026-02-02", Status: "OK", WorkedMinutes: 1, ComputedAt: time.Unix(1, 0).UTC()},
				}, nil
			},
		}

		handleAttendanceTimeBank(rec, req, cycleStore, dailyStore, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Comp Earned Minutes") {
			t.Fatalf("body=%q", body)
		}
		if strings.Contains(body, "/org/attendance-daily-results/p1/2026-02-01") {
			t.Fatalf("expected zero-contribution day filtered out: body=%q", body)
		}
		if !strings.Contains(body, "2026-02-02") {
			t.Fatalf("expected contributing day: body=%q", body)
		}
	})
}
