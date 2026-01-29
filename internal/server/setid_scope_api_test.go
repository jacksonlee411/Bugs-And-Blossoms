package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type scopeAPIStore struct {
	listScopePackagesFn        func(context.Context, string, string) ([]ScopePackage, error)
	createScopePackageFn       func(context.Context, string, string, string, string, string, string, string) (ScopePackage, error)
	disableScopePackageFn      func(context.Context, string, string, string, string) (ScopePackage, error)
	createScopeSubscriptionFn  func(context.Context, string, string, string, string, string, string, string, string) (ScopeSubscription, error)
	getScopeSubscriptionFn     func(context.Context, string, string, string, string) (ScopeSubscription, error)
	createGlobalScopePackageFn func(context.Context, string, string, string, string, string, string, string) (ScopePackage, error)
	listGlobalScopePackagesFn  func(context.Context, string) ([]ScopePackage, error)
}

func (s scopeAPIStore) EnsureBootstrap(context.Context, string, string) error { return nil }
func (s scopeAPIStore) ListSetIDs(context.Context, string) ([]SetID, error)   { return nil, nil }
func (s scopeAPIStore) ListGlobalSetIDs(context.Context) ([]SetID, error)     { return nil, nil }
func (s scopeAPIStore) CreateSetID(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s scopeAPIStore) ListSetIDBindings(context.Context, string, string) ([]SetIDBindingRow, error) {
	return nil, nil
}
func (s scopeAPIStore) BindSetID(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s scopeAPIStore) CreateGlobalSetID(context.Context, string, string, string, string) error {
	return nil
}
func (s scopeAPIStore) ListScopeCodes(context.Context, string) ([]ScopeCode, error) {
	return nil, nil
}
func (s scopeAPIStore) CreateScopePackage(ctx context.Context, tenantID string, scopeCode string, packageCode string, name string, effectiveDate string, requestID string, initiatorID string) (ScopePackage, error) {
	if s.createScopePackageFn == nil {
		return ScopePackage{}, nil
	}
	return s.createScopePackageFn(ctx, tenantID, scopeCode, packageCode, name, effectiveDate, requestID, initiatorID)
}
func (s scopeAPIStore) DisableScopePackage(ctx context.Context, tenantID string, packageID string, requestID string, initiatorID string) (ScopePackage, error) {
	if s.disableScopePackageFn == nil {
		return ScopePackage{}, nil
	}
	return s.disableScopePackageFn(ctx, tenantID, packageID, requestID, initiatorID)
}
func (s scopeAPIStore) ListScopePackages(ctx context.Context, tenantID string, scopeCode string) ([]ScopePackage, error) {
	if s.listScopePackagesFn == nil {
		return nil, nil
	}
	return s.listScopePackagesFn(ctx, tenantID, scopeCode)
}
func (s scopeAPIStore) CreateScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, packageID string, packageOwner string, effectiveDate string, requestID string, initiatorID string) (ScopeSubscription, error) {
	if s.createScopeSubscriptionFn == nil {
		return ScopeSubscription{}, nil
	}
	return s.createScopeSubscriptionFn(ctx, tenantID, setID, scopeCode, packageID, packageOwner, effectiveDate, requestID, initiatorID)
}
func (s scopeAPIStore) GetScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, asOfDate string) (ScopeSubscription, error) {
	if s.getScopeSubscriptionFn == nil {
		return ScopeSubscription{}, nil
	}
	return s.getScopeSubscriptionFn(ctx, tenantID, setID, scopeCode, asOfDate)
}
func (s scopeAPIStore) CreateGlobalScopePackage(ctx context.Context, scopeCode string, packageCode string, name string, effectiveDate string, requestID string, initiatorID string, actorScope string) (ScopePackage, error) {
	if s.createGlobalScopePackageFn == nil {
		return ScopePackage{}, nil
	}
	return s.createGlobalScopePackageFn(ctx, scopeCode, packageCode, name, effectiveDate, requestID, initiatorID, actorScope)
}
func (s scopeAPIStore) ListGlobalScopePackages(ctx context.Context, scopeCode string) ([]ScopePackage, error) {
	if s.listGlobalScopePackagesFn == nil {
		return nil, nil
	}
	return s.listGlobalScopePackagesFn(ctx, scopeCode)
}

func TestHandleScopePackagesAPI_Get(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-packages?scope_code=jobcatalog", nil)
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("empty rows", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-packages?scope_code=jobcatalog", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			listScopePackagesFn: func(context.Context, string, string) ([]ScopePackage, error) {
				return nil, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) == "" {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("actor scope optional", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-packages?scope_code=jobcatalog", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			listScopePackagesFn: func(context.Context, string, string) ([]ScopePackage, error) {
				return []ScopePackage{}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("scope_code required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-packages", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-packages?scope_code=jobcatalog", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			listScopePackagesFn: func(context.Context, string, string) ([]ScopePackage, error) {
				return nil, errors.New("SCOPE_CODE_INVALID")
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "SCOPE_CODE_INVALID") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-packages?scope_code=jobcatalog", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			listScopePackagesFn: func(context.Context, string, string) ([]ScopePackage, error) {
				return []ScopePackage{{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: "PKG1", Status: "active"}}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		var got []ScopePackage
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode err=%v", err)
		}
		if len(got) != 1 || got[0].PackageID != "p1" {
			t.Fatalf("unexpected payload: %+v", got)
		}
	})
}

func TestHandleScopePackagesAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/orgunit/api/scope-packages", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleScopePackagesAPI(rec, req, scopeAPIStore{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleScopePackagesAPI_Post(t *testing.T) {
	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid request", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"","name":"","request_id":""}`)
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid effective date", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","name":"Pkg","effective_date":"bad","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("reserved code", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","package_code":"DEFLT","name":"Pkg","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid code", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","package_code":"bad-code","name":"Pkg","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("create error", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			createScopePackageFn: func(context.Context, string, string, string, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{}, errors.New("PACKAGE_CODE_DUPLICATE")
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","package_code":"","name":"Pkg","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			createScopePackageFn: func(context.Context, string, string, string, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: "PKG1", Status: "active"}, nil
			},
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "p1") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}

func TestHandleScopePackageDisableAPI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages/p1/disable", strings.NewReader(`{"request_id":"r1"}`))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-packages/p1/disable", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages/p1/disable", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing request id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages/p1/disable", strings.NewReader(`{"request_id":""}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages/p1/disable", strings.NewReader(`{"request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{
			disableScopePackageFn: func(context.Context, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{}, errors.New("PACKAGE_NOT_FOUND")
			},
		})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-packages/p1/disable", strings.NewReader(`{"request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{
			disableScopePackageFn: func(context.Context, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{PackageID: "p1", Status: "disabled"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "disabled") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}

func TestHandleScopeSubscriptionsAPI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-subscriptions?setid=S2601&scope_code=jobcatalog&as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get missing params", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-subscriptions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get invalid date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-subscriptions?setid=S2601&scope_code=jobcatalog&as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-subscriptions?setid=S2601&scope_code=jobcatalog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{
			getScopeSubscriptionFn: func(context.Context, string, string, string, string) (ScopeSubscription, error) {
				return ScopeSubscription{}, errors.New("SCOPE_SUBSCRIPTION_MISSING")
			},
		})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-subscriptions?setid=S2601&scope_code=jobcatalog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{
			getScopeSubscriptionFn: func(context.Context, string, string, string, string) (ScopeSubscription, error) {
				return ScopeSubscription{SetID: "S2601", ScopeCode: "jobcatalog", PackageID: "p1", PackageOwner: "tenant", EffectiveDate: "2026-01-01", EndDate: ""}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "S2601") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("get default as_of", func(t *testing.T) {
		var gotAsOf string
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-subscriptions?setid=S2601&scope_code=jobcatalog", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{
			getScopeSubscriptionFn: func(_ context.Context, _ string, _ string, _ string, asOf string) (ScopeSubscription, error) {
				gotAsOf = strings.TrimSpace(asOf)
				return ScopeSubscription{SetID: "S2601", ScopeCode: "jobcatalog"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if gotAsOf == "" {
			t.Fatalf("missing as_of")
		}
	})

	t.Run("post bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-subscriptions", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post missing fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-subscriptions", strings.NewReader(`{"setid":""}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-subscriptions", strings.NewReader(`{"setid":"S2601","scope_code":"jobcatalog","package_id":"p1","package_owner":"tenant","effective_date":"bad","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post owner invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-subscriptions", strings.NewReader(`{"setid":"S2601","scope_code":"jobcatalog","package_id":"p1","package_owner":"nope","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-subscriptions", strings.NewReader(`{"setid":"S2601","scope_code":"jobcatalog","package_id":"p1","package_owner":"tenant","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{
			createScopeSubscriptionFn: func(context.Context, string, string, string, string, string, string, string, string) (ScopeSubscription, error) {
				return ScopeSubscription{}, errors.New("SUBSCRIPTION_OVERLAP")
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/scope-subscriptions", strings.NewReader(`{"setid":"S2601","scope_code":"jobcatalog","package_id":"p1","package_owner":"tenant","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{
			createScopeSubscriptionFn: func(context.Context, string, string, string, string, string, string, string, string) (ScopeSubscription, error) {
				return ScopeSubscription{SetID: "S2601", ScopeCode: "jobcatalog", PackageID: "p1", PackageOwner: "tenant", EffectiveDate: "2026-01-01"}, nil
			},
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleScopeSubscriptionsAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/orgunit/api/scope-subscriptions", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleGlobalScopePackagesAPI(t *testing.T) {
	t.Run("get actor scope required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/global-scope-packages?scope_code=jobcatalog", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get scope_code required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/global-scope-packages", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/global-scope-packages?scope_code=jobcatalog", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{
			listGlobalScopePackagesFn: func(context.Context, string) ([]ScopePackage, error) {
				return nil, errors.New("SCOPE_CODE_INVALID")
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgunit/api/global-scope-packages?scope_code=jobcatalog", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{
			listGlobalScopePackagesFn: func(context.Context, string) ([]ScopePackage, error) {
				return []ScopePackage{{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: "PKG1"}}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","request_id":"r1"}`))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-scope-packages", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-scope-packages", strings.NewReader(`{"scope_code":"","name":"","request_id":""}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","effective_date":"bad","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post reserved code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"DEFLT","name":"Pkg","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"bad-code","name":"Pkg","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post actor scope required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","request_id":"r1"}`))
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{
			createGlobalScopePackageFn: func(context.Context, string, string, string, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{}, errors.New("PACKAGE_CODE_DUPLICATE")
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","request_id":"r1"}`))
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{
			createGlobalScopePackageFn: func(context.Context, string, string, string, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: "PKG1", Status: "active"}, nil
			},
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post generate code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"","name":"Pkg","request_id":"r1"}`))
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{
			createGlobalScopePackageFn: func(_ context.Context, _ string, packageCode string, _ string, _ string, _ string, _ string, _ string) (ScopePackage, error) {
				if !strings.HasPrefix(packageCode, "PKG_") {
					t.Fatalf("unexpected code=%q", packageCode)
				}
				if !packageCodePattern.MatchString(packageCode) {
					t.Fatalf("invalid code=%q", packageCode)
				}
				return ScopePackage{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: packageCode, Status: "active"}, nil
			},
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleGlobalScopePackagesAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/orgunit/api/global-scope-packages", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWriteScopeAPIError_StatusMapping(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/orgunit/api/scope-packages", nil)

	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: errors.New("PACKAGE_NOT_FOUND"), status: http.StatusNotFound},
		{name: "conflict", err: errors.New("PACKAGE_CODE_DUPLICATE"), status: http.StatusConflict},
		{name: "unprocessable", err: errors.New("PACKAGE_CODE_RESERVED"), status: http.StatusUnprocessableEntity},
		{name: "stable", err: errors.New("CODE_123"), status: http.StatusUnprocessableEntity},
		{name: "bad request", err: newBadRequestError("bad"), status: http.StatusBadRequest},
		{name: "pg invalid", err: &pgconn.PgError{Code: "22P02", Message: "invalid"}, status: http.StatusBadRequest},
		{name: "default", err: errors.New(""), status: http.StatusInternalServerError},
	} {
		rec := httptest.NewRecorder()
		writeScopeAPIError(rec, req, tc.err, "fallback")
		if rec.Code != tc.status {
			t.Fatalf("%s status=%d", tc.name, rec.Code)
		}
		if tc.name == "default" && !strings.Contains(rec.Body.String(), "fallback") {
			t.Fatalf("missing default code: %q", rec.Body.String())
		}
	}
}

func TestScopeAPIHelpers(t *testing.T) {
	if got := parseScopePackageID("/orgunit/api/scope-packages/p1/disable"); got != "p1" {
		t.Fatalf("got=%q", got)
	}
	if got := parseScopePackageID("/orgunit/api/scope-packages"); got != "" {
		t.Fatalf("expected empty, got=%q", got)
	}

	code := generatePackageCode()
	if !strings.HasPrefix(code, "PKG_") {
		t.Fatalf("unexpected code=%q", code)
	}
	if ok, _ := regexp.MatchString(`^PKG_[A-Z0-9]{1,8}$`, code); !ok {
		t.Fatalf("unexpected code=%q", code)
	}
}
