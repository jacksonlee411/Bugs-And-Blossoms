package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type scopeUIStore struct {
	listScopeCodesFn          func(context.Context, string) ([]ScopeCode, error)
	listScopePackagesFn       func(context.Context, string, string) ([]ScopePackage, error)
	getScopeSubscriptionFn    func(context.Context, string, string, string, string) (ScopeSubscription, error)
	createScopeSubscriptionFn func(context.Context, string, string, string, string, string, string, string, string) (ScopeSubscription, error)
}

func (s scopeUIStore) EnsureBootstrap(context.Context, string, string) error { return nil }
func (s scopeUIStore) ListSetIDs(context.Context, string) ([]SetID, error)   { return nil, nil }
func (s scopeUIStore) ListGlobalSetIDs(context.Context) ([]SetID, error)     { return nil, nil }
func (s scopeUIStore) CreateSetID(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s scopeUIStore) ListSetIDBindings(context.Context, string, string) ([]SetIDBindingRow, error) {
	return nil, nil
}
func (s scopeUIStore) BindSetID(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s scopeUIStore) CreateGlobalSetID(context.Context, string, string, string, string) error {
	return nil
}
func (s scopeUIStore) ListScopeCodes(ctx context.Context, tenantID string) ([]ScopeCode, error) {
	if s.listScopeCodesFn == nil {
		return nil, nil
	}
	return s.listScopeCodesFn(ctx, tenantID)
}
func (s scopeUIStore) CreateScopePackage(context.Context, string, string, string, string, string, string, string) (ScopePackage, error) {
	return ScopePackage{}, nil
}
func (s scopeUIStore) DisableScopePackage(context.Context, string, string, string, string) (ScopePackage, error) {
	return ScopePackage{}, nil
}
func (s scopeUIStore) ListScopePackages(ctx context.Context, tenantID string, scopeCode string) ([]ScopePackage, error) {
	if s.listScopePackagesFn == nil {
		return nil, nil
	}
	return s.listScopePackagesFn(ctx, tenantID, scopeCode)
}
func (s scopeUIStore) CreateScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, packageID string, packageOwner string, effectiveDate string, requestID string, initiatorID string) (ScopeSubscription, error) {
	if s.createScopeSubscriptionFn == nil {
		return ScopeSubscription{}, nil
	}
	return s.createScopeSubscriptionFn(ctx, tenantID, setID, scopeCode, packageID, packageOwner, effectiveDate, requestID, initiatorID)
}
func (s scopeUIStore) GetScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, asOfDate string) (ScopeSubscription, error) {
	if s.getScopeSubscriptionFn == nil {
		return ScopeSubscription{}, nil
	}
	return s.getScopeSubscriptionFn(ctx, tenantID, setID, scopeCode, asOfDate)
}
func (s scopeUIStore) CreateGlobalScopePackage(context.Context, string, string, string, string, string, string, string) (ScopePackage, error) {
	return ScopePackage{}, nil
}
func (s scopeUIStore) ListGlobalScopePackages(context.Context, string) ([]ScopePackage, error) {
	return nil, nil
}

func TestHandleSetIDScopeSubscriptionsUI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/setids/S2601/scope-subscriptions?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/setids/S2601", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/setids/S2601/scope-subscriptions?as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("default as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/setids/S2601/scope-subscriptions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{
			listScopeCodesFn: func(context.Context, string) ([]ScopeCode, error) {
				return []ScopeCode{}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list scope error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/setids/S2601/scope-subscriptions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{
			listScopeCodesFn: func(context.Context, string) ([]ScopeCode, error) {
				return nil, errors.New("list scope fail")
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "list scope fail") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("get success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/setids/S2601/scope-subscriptions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{
			listScopeCodesFn: func(context.Context, string) ([]ScopeCode, error) {
				return []ScopeCode{
					{ScopeCode: "jobcatalog", ShareMode: "tenant-only"},
					{ScopeCode: "orgunit_geo_admin", ShareMode: "shared-only"},
				}, nil
			},
			listScopePackagesFn: func(context.Context, string, string) ([]ScopePackage, error) {
				return []ScopePackage{{PackageID: "p1", PackageCode: "PKG1"}}, nil
			},
			getScopeSubscriptionFn: func(_ context.Context, _ string, _ string, scopeCode string, _ string) (ScopeSubscription, error) {
				if scopeCode == "jobcatalog" {
					return ScopeSubscription{ScopeCode: "jobcatalog", PackageID: "p1", PackageOwner: "tenant", EffectiveDate: "2026-01-01"}, nil
				}
				return ScopeSubscription{ScopeCode: scopeCode, PackageID: "gp1", PackageOwner: "global", EffectiveDate: "2026-01-01"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "(read-only)") {
			t.Fatalf("missing read-only marker")
		}
		if !strings.Contains(rec.Body.String(), "PKG1") {
			t.Fatalf("missing package code")
		}
	})

	t.Run("post parse form error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/setids/S2601/scope-subscriptions?as_of=2026-01-01", strings.NewReader("%"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{
			listScopeCodesFn: func(context.Context, string) ([]ScopeCode, error) {
				return []ScopeCode{}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "bad form") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("post missing fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/setids/S2601/scope-subscriptions?as_of=2026-01-01", strings.NewReader("scope_code=&package_id=&request_id="))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{
			listScopeCodesFn: func(context.Context, string) ([]ScopeCode, error) {
				return []ScopeCode{}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "scope_code/package_id/request_id required") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("post invalid effective date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/setids/S2601/scope-subscriptions?as_of=2026-01-01", strings.NewReader("scope_code=jobcatalog&package_id=p1&effective_date=bad&request_id=r1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{
			listScopeCodesFn: func(context.Context, string) ([]ScopeCode, error) {
				return []ScopeCode{}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "effective_date invalid") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("post create error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/setids/S2601/scope-subscriptions?as_of=2026-01-01", strings.NewReader("scope_code=jobcatalog&package_id=p1&effective_date=2026-01-01&request_id=r1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{
			listScopeCodesFn: func(context.Context, string) ([]ScopeCode, error) {
				return []ScopeCode{}, nil
			},
			createScopeSubscriptionFn: func(context.Context, string, string, string, string, string, string, string, string) (ScopeSubscription, error) {
				return ScopeSubscription{}, errors.New("SCOPE_SUBSCRIPTION_MISSING")
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "SCOPE_SUBSCRIPTION_MISSING") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("post success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/setids/S2601/scope-subscriptions?as_of=2026-01-01", strings.NewReader("scope_code=jobcatalog&package_id=p1&effective_date=2026-01-01&request_id=r1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{
			listScopeCodesFn: func(context.Context, string) ([]ScopeCode, error) {
				return []ScopeCode{{ScopeCode: "jobcatalog", ShareMode: "tenant-only"}}, nil
			},
			listScopePackagesFn: func(context.Context, string, string) ([]ScopePackage, error) {
				return []ScopePackage{{PackageID: "p1", PackageCode: "PKG1"}}, nil
			},
			getScopeSubscriptionFn: func(context.Context, string, string, string, string) (ScopeSubscription, error) {
				return ScopeSubscription{ScopeCode: "jobcatalog", PackageID: "p1", PackageOwner: "tenant", EffectiveDate: "2026-01-01"}, nil
			},
			createScopeSubscriptionFn: func(context.Context, string, string, string, string, string, string, string, string) (ScopeSubscription, error) {
				return ScopeSubscription{SetID: "S2601", ScopeCode: "jobcatalog", PackageID: "p1"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if rec.Header().Get("HX-Trigger") == "" {
			t.Fatal("missing HX-Trigger")
		}
	})
}

func TestHandleSetIDScopeSubscriptionsUI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/orgunit/setids/S2601/scope-subscriptions?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleSetIDScopeSubscriptionsUI(rec, req, scopeUIStore{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestScopeSubscriptionsUIHelpers(t *testing.T) {
	if got := parseScopeSubscriptionSetID("/orgunit/setids/S2601/scope-subscriptions"); got != "S2601" {
		t.Fatalf("got=%q", got)
	}
	if got := parseScopeSubscriptionSetID("/orgunit/setids/S2601"); got != "" {
		t.Fatalf("expected empty, got=%q", got)
	}
}
