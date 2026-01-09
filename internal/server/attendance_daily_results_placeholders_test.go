package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAttendanceDailyResultsPlaceholderHandlers_Coverage(t *testing.T) {
	t.Parallel()

	tenant := Tenant{ID: "t1", Name: "Tenant 1"}

	t.Run("list tenant missing", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsPlaceholder(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list missing as_of redirects", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultsPlaceholder(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "as_of=") {
			t.Fatalf("missing redirect as_of: %q", loc)
		}
	})

	t.Run("detail tenant missing", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/person-101/2026-01-01?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetailPlaceholder(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail missing as_of redirects", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/person-101/2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetailPlaceholder(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail bad path", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/bad?as_of=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetailPlaceholder(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail ok", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-daily-results/person-101/2026-01-01?as_of=2026-01-01", nil).WithContext(withTenant(t.Context(), tenant))
		rec := httptest.NewRecorder()
		handleAttendanceDailyResultDetailPlaceholder(rec, req)
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

		out := renderAttendanceDailyResultsDetailPlaceholder(tenant, "2026-01-01", "person-101", "2026-01-01", "bad path")
		if !strings.Contains(out, "bad path") {
			t.Fatalf("expected errMsg in html: %q", out)
		}
	})
}
