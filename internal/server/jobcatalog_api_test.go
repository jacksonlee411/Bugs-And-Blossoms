package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func tenantAdminAPIRequest(method string, path string, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
	return req
}

type jobCatalogListErrStore struct {
	JobCatalogStore
	groupsErr   error
	familiesErr error
	levelsErr   error
	profilesErr error
}

func (s jobCatalogListErrStore) ListJobFamilyGroups(ctx context.Context, tenantID string, setID string, asOf string) ([]JobFamilyGroup, error) {
	if s.groupsErr != nil {
		return nil, s.groupsErr
	}
	return s.JobCatalogStore.ListJobFamilyGroups(ctx, tenantID, setID, asOf)
}

func (s jobCatalogListErrStore) ListJobFamilies(ctx context.Context, tenantID string, setID string, asOf string) ([]JobFamily, error) {
	if s.familiesErr != nil {
		return nil, s.familiesErr
	}
	return s.JobCatalogStore.ListJobFamilies(ctx, tenantID, setID, asOf)
}

func (s jobCatalogListErrStore) ListJobLevels(ctx context.Context, tenantID string, setID string, asOf string) ([]JobLevel, error) {
	if s.levelsErr != nil {
		return nil, s.levelsErr
	}
	return s.JobCatalogStore.ListJobLevels(ctx, tenantID, setID, asOf)
}

func (s jobCatalogListErrStore) ListJobProfiles(ctx context.Context, tenantID string, setID string, asOf string) ([]JobProfile, error) {
	if s.profilesErr != nil {
		return nil, s.profilesErr
	}
	return s.JobCatalogStore.ListJobProfiles(ctx, tenantID, setID, asOf)
}

type jobCatalogResolvePkgWhitespaceErrStore struct{ JobCatalogStore }

func (jobCatalogResolvePkgWhitespaceErrStore) ResolveJobCatalogPackageByCode(context.Context, string, string, string) (JobCatalogPackage, error) {
	return JobCatalogPackage{}, errors.New(" ")
}

func TestHandleJobCatalogAPI_Branches(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/jobcatalog/api/catalog", nil)
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog", "{}")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=bad", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("setid and package mutually exclusive", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=PKG1&setid=S1", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("as_of required", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("no selection returns empty arrays", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out["tenant_id"] != "t1" {
			t.Fatalf("tenant_id=%v", out["tenant_id"])
		}
	})

	t.Run("package selection forbidden without principal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=PKG1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("view error code fallback when errMsg trims empty", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=PKG1", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), jobCatalogResolvePkgWhitespaceErrStore{JobCatalogStore: newJobCatalogMemoryStore()})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "jobcatalog_view_invalid") {
			t.Fatalf("unexpected body=%q", rec.Body.String())
		}
	})

	t.Run("list groups error", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=PKG1", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), jobCatalogListErrStore{JobCatalogStore: newJobCatalogMemoryStore(), groupsErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("list groups stable error", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=PKG1", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), jobCatalogListErrStore{JobCatalogStore: newJobCatalogMemoryStore(), groupsErr: errors.New("JOB_CATALOG_LIST_GROUPS_FAILED")})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("list families error", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=PKG1", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), jobCatalogListErrStore{JobCatalogStore: newJobCatalogMemoryStore(), familiesErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("list families stable error", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=PKG1", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), jobCatalogListErrStore{JobCatalogStore: newJobCatalogMemoryStore(), familiesErr: errors.New("JOB_CATALOG_LIST_FAMILIES_FAILED")})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("list levels error", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=PKG1", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), jobCatalogListErrStore{JobCatalogStore: newJobCatalogMemoryStore(), levelsErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("list levels stable error", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=PKG1", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), jobCatalogListErrStore{JobCatalogStore: newJobCatalogMemoryStore(), levelsErr: errors.New("JOB_CATALOG_LIST_LEVELS_FAILED")})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("list profiles error", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=PKG1", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), jobCatalogListErrStore{JobCatalogStore: newJobCatalogMemoryStore(), profilesErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("list profiles stable error", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=PKG1", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), jobCatalogListErrStore{JobCatalogStore: newJobCatalogMemoryStore(), profilesErr: errors.New("JOB_CATALOG_LIST_PROFILES_FAILED")})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("package selection success", func(t *testing.T) {
		store := newJobCatalogMemoryStore().(*jobcatalogMemoryStore)
		if err := store.CreateJobFamilyGroup(context.Background(), "t1", "PKG1", "2026-01-01", "G1", "Group 1", ""); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateJobFamily(context.Background(), "t1", "PKG1", "2026-01-01", "F1", "Family 1", "", "G1"); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateJobLevel(context.Background(), "t1", "PKG1", "2026-01-01", "L1", "Level 1", ""); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateJobProfile(context.Background(), "t1", "PKG1", "2026-01-01", "P1", "Profile 1", "", []string{"F1"}, "F1"); err != nil {
			t.Fatal(err)
		}

		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog?as_of=2026-01-01&package_code=pkg1", "")
		rec := httptest.NewRecorder()
		handleJobCatalogAPI(rec, req, defaultJobCatalogSetIDStore(), store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"package_code":"PKG1"`) || !strings.Contains(rec.Body.String(), `"job_family_group_code":"G1"`) {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}

type jobCatalogWriteErrStore struct {
	JobCatalogStore
	createGroupErr       error
	createFamilyErr      error
	updateFamilyGroupErr error
	createLevelErr       error
	createProfileErr     error
}

func (s jobCatalogWriteErrStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return s.createGroupErr
}

func (s jobCatalogWriteErrStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return s.createFamilyErr
}

func (s jobCatalogWriteErrStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return s.updateFamilyGroupErr
}

func (s jobCatalogWriteErrStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return s.createLevelErr
}

func (s jobCatalogWriteErrStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return s.createProfileErr
}

func TestHandleJobCatalogWriteAPI_Branches(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", nil)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodGet, "/jobcatalog/api/catalog/actions", "")
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", "{")
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid effective date", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"bad"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("package required", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"effective_date":"2026-01-01"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("view forbidden without principal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", bytes.NewBufferString(`{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_family_group","code":"G1","name":"Group 1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("view error code fallback when errMsg trims empty", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_family_group","code":"G1","name":"Group 1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), jobCatalogResolvePkgWhitespaceErrStore{JobCatalogStore: newJobCatalogMemoryStore()})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "jobcatalog_view_invalid") {
			t.Fatalf("unexpected body=%q", rec.Body.String())
		}
	})

	t.Run("create group invalid request", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_family_group","code":"","name":""}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("create group ok (default action)", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","code":"G1","name":"Group 1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("create group store error", func(t *testing.T) {
		store := jobCatalogWriteErrStore{JobCatalogStore: newJobCatalogMemoryStore(), createGroupErr: errors.New("boom")}
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","code":"G1","name":"Group 1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), store)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("create family invalid request", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_family","code":"F1","name":"","group_code":""}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("create family ok", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_family","code":"F1","name":"Family 1","group_code":"G1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("create family store error", func(t *testing.T) {
		store := jobCatalogWriteErrStore{JobCatalogStore: newJobCatalogMemoryStore(), createFamilyErr: errors.New("boom")}
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_family","code":"F1","name":"Family 1","group_code":"G1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), store)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("update family group invalid request", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"update_job_family_group","code":"","group_code":""}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("update family group ok", func(t *testing.T) {
		store := newJobCatalogMemoryStore().(*jobcatalogMemoryStore)
		if err := store.CreateJobFamily(context.Background(), "t1", "PKG1", "2026-01-01", "F1", "Family 1", "", "G0"); err != nil {
			t.Fatal(err)
		}
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"update_job_family_group","code":"F1","group_code":"G1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("update family group store error", func(t *testing.T) {
		store := jobCatalogWriteErrStore{JobCatalogStore: newJobCatalogMemoryStore(), updateFamilyGroupErr: errors.New("boom")}
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"update_job_family_group","code":"F1","group_code":"G1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), store)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("create level invalid request", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_level","code":"","name":""}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("create level ok", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_level","code":"L1","name":"Level 1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("create level store error", func(t *testing.T) {
		store := jobCatalogWriteErrStore{JobCatalogStore: newJobCatalogMemoryStore(), createLevelErr: errors.New("boom")}
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_level","code":"L1","name":"Level 1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), store)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("create profile invalid request", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_profile","code":"P1","name":"Profile 1","family_codes_csv":"","primary_family_code":""}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("create profile ok", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_profile","code":"P1","name":"Profile 1","family_codes_csv":"F1,F2","primary_family_code":"F1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("create profile store error", func(t *testing.T) {
		store := jobCatalogWriteErrStore{JobCatalogStore: newJobCatalogMemoryStore(), createProfileErr: errors.New("boom")}
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"create_job_profile","code":"P1","name":"Profile 1","family_codes_csv":"F1","primary_family_code":"F1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), store)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("default effective_date", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","code":"G1","name":"Group 1"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("unknown action", func(t *testing.T) {
		req := tenantAdminAPIRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", `{"package_code":"PKG1","effective_date":"2026-01-01","action":"nope"}`)
		rec := httptest.NewRecorder()
		handleJobCatalogWriteAPI(rec, req, defaultJobCatalogSetIDStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}
