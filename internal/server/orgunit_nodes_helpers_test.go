package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type treeAsOfStoreStub struct {
	*orgUnitMemoryStore
	maxFn func(ctx context.Context, tenantID, asOfDate string) (string, bool, error)
	minFn func(ctx context.Context, tenantID string) (string, bool, error)
}

func (s *treeAsOfStoreStub) MaxEffectiveDateOnOrBefore(ctx context.Context, tenantID, asOfDate string) (string, bool, error) {
	if s.maxFn != nil {
		return s.maxFn(ctx, tenantID, asOfDate)
	}
	return "", false, nil
}

func (s *treeAsOfStoreStub) MinEffectiveDate(ctx context.Context, tenantID string) (string, bool, error) {
	if s.minFn != nil {
		return s.minFn(ctx, tenantID)
	}
	return "", false, nil
}

func TestRejectDeprecatedAsOf(t *testing.T) {
	t.Run("deprecated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=2026-01-06", nil)
		rec := httptest.NewRecorder()
		ok := rejectDeprecatedAsOf(rec, req)
		if ok {
			t.Fatal("expected deprecated as_of to be rejected")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "deprecated as_of") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes?tree_as_of=2026-01-06", nil)
		rec := httptest.NewRecorder()
		ok := rejectDeprecatedAsOf(rec, req)
		if !ok {
			t.Fatal("expected request to be allowed")
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestRequireTreeAsOf(t *testing.T) {
	t.Run("deprecated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=2026-01-06", nil)
		rec := httptest.NewRecorder()
		_, ok := requireTreeAsOf(rec, req)
		if ok {
			t.Fatal("expected deprecated as_of to be rejected")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
		rec := httptest.NewRecorder()
		_, ok := requireTreeAsOf(rec, req)
		if ok {
			t.Fatal("expected missing tree_as_of to fail")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes?tree_as_of=bad", nil)
		rec := httptest.NewRecorder()
		_, ok := requireTreeAsOf(rec, req)
		if ok {
			t.Fatal("expected invalid tree_as_of to fail")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes?tree_as_of=2026-01-06", nil)
		rec := httptest.NewRecorder()
		value, ok := requireTreeAsOf(rec, req)
		if !ok {
			t.Fatal("expected valid tree_as_of")
		}
		if value != "2026-01-06" {
			t.Fatalf("value=%q", value)
		}
	})
}

func TestResolveTreeAsOfForPage(t *testing.T) {
	t.Run("valid query returns", func(t *testing.T) {
		store := &treeAsOfStoreStub{orgUnitMemoryStore: newOrgUnitMemoryStore()}
		store.maxFn = func(context.Context, string, string) (string, bool, error) {
			t.Fatal("unexpected store call")
			return "", false, nil
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes?tree_as_of=2026-01-06", nil)
		rec := httptest.NewRecorder()
		value, ok := resolveTreeAsOfForPage(rec, req, store, "t1")
		if !ok || value != "2026-01-06" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid query redirects", func(t *testing.T) {
		store := &treeAsOfStoreStub{orgUnitMemoryStore: newOrgUnitMemoryStore()}
		store.maxFn = func(context.Context, string, string) (string, bool, error) {
			return "2026-01-05", true, nil
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes?tree_as_of=bad", nil)
		rec := httptest.NewRecorder()
		value, ok := resolveTreeAsOfForPage(rec, req, store, "t1")
		if ok || value != "" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "tree_as_of=2026-01-05") {
			t.Fatalf("location=%q", loc)
		}
	})

	t.Run("store error", func(t *testing.T) {
		store := &treeAsOfStoreStub{orgUnitMemoryStore: newOrgUnitMemoryStore()}
		store.maxFn = func(context.Context, string, string) (string, bool, error) {
			return "", false, errors.New("boom")
		}
		req := httptest.NewRequest(http.MethodPost, "/org/nodes", nil)
		rec := httptest.NewRecorder()
		_, ok := resolveTreeAsOfForPage(rec, req, store, "t1")
		if ok {
			t.Fatal("expected store error")
		}
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("min error", func(t *testing.T) {
		store := &treeAsOfStoreStub{orgUnitMemoryStore: newOrgUnitMemoryStore()}
		store.maxFn = func(context.Context, string, string) (string, bool, error) {
			return "", false, nil
		}
		store.minFn = func(context.Context, string) (string, bool, error) {
			return "", false, errors.New("boom")
		}
		req := httptest.NewRequest(http.MethodPost, "/org/nodes", nil)
		rec := httptest.NewRecorder()
		_, ok := resolveTreeAsOfForPage(rec, req, store, "t1")
		if ok {
			t.Fatal("expected min error")
		}
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("fallback to min on POST", func(t *testing.T) {
		store := &treeAsOfStoreStub{orgUnitMemoryStore: newOrgUnitMemoryStore()}
		store.maxFn = func(context.Context, string, string) (string, bool, error) {
			return "", false, nil
		}
		store.minFn = func(context.Context, string) (string, bool, error) {
			return "2026-01-01", true, nil
		}
		req := httptest.NewRequest(http.MethodPost, "/org/nodes", nil)
		rec := httptest.NewRecorder()
		value, ok := resolveTreeAsOfForPage(rec, req, store, "t1")
		if !ok || value != "2026-01-01" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("fallback to system day redirects", func(t *testing.T) {
		store := &treeAsOfStoreStub{orgUnitMemoryStore: newOrgUnitMemoryStore()}
		store.maxFn = func(context.Context, string, string) (string, bool, error) {
			return "", false, nil
		}
		store.minFn = func(context.Context, string) (string, bool, error) {
			return "", false, nil
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
		rec := httptest.NewRecorder()
		_, ok := resolveTreeAsOfForPage(rec, req, store, "t1")
		if ok {
			t.Fatal("expected redirect")
		}
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
		expected := "tree_as_of=" + currentUTCDateString()
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, expected) {
			t.Fatalf("location=%q expected=%q", loc, expected)
		}
	})
}

func TestTreeAsOfFromForm(t *testing.T) {
	t.Run("deprecated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", nil)
		rec := httptest.NewRecorder()
		_, ok := treeAsOfFromForm(rec, req)
		if ok {
			t.Fatal("expected deprecated as_of to be rejected")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/nodes", nil)
		rec := httptest.NewRecorder()
		_, ok := treeAsOfFromForm(rec, req)
		if ok {
			t.Fatal("expected missing tree_as_of to fail")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		body := strings.NewReader("tree_as_of=bad")
		req := httptest.NewRequest(http.MethodPost, "/org/nodes", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		_, ok := treeAsOfFromForm(rec, req)
		if ok {
			t.Fatal("expected invalid tree_as_of to fail")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("from form", func(t *testing.T) {
		body := strings.NewReader("tree_as_of=2026-01-06")
		req := httptest.NewRequest(http.MethodPost, "/org/nodes", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		value, ok := treeAsOfFromForm(rec, req)
		if !ok || value != "2026-01-06" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
	})

	t.Run("from query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", nil)
		rec := httptest.NewRecorder()
		value, ok := treeAsOfFromForm(rec, req)
		if !ok || value != "2026-01-06" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
	})
}

func TestParseOptionalTreeAsOf(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
		rec := httptest.NewRecorder()
		value, ok := parseOptionalTreeAsOf(rec, req)
		if !ok || value != "" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes?tree_as_of=bad", nil)
		rec := httptest.NewRecorder()
		_, ok := parseOptionalTreeAsOf(rec, req)
		if ok {
			t.Fatal("expected invalid tree_as_of to fail")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes?tree_as_of=2026-01-06", nil)
		rec := httptest.NewRecorder()
		value, ok := parseOptionalTreeAsOf(rec, req)
		if !ok || value != "2026-01-06" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
	})
}
