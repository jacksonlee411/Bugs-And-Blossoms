package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitMemoryStoreWithSetID struct {
	*orgUnitMemoryStore
	setID string
	err   error
}

func (s orgUnitMemoryStoreWithSetID) ResolveSetID(_ context.Context, _ string, _ string, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.setID, nil
}

type resolveOrgIDErrStore struct {
	*orgUnitMemoryStore
	err error
}

func (s resolveOrgIDErrStore) ResolveOrgID(context.Context, string, string) (int, error) {
	return 0, s.err
}

type jobStoreListProfilesErr struct {
	JobCatalogStore
	err error
}

func (s jobStoreListProfilesErr) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, s.err
}

func TestHandlePositionsOptionsAPI_Branches(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options", nil)
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, newOrgUnitMemoryStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions:options", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, newOrgUnitMemoryStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=bad&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, newOrgUnitMemoryStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("as_of required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, newOrgUnitMemoryStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing org_code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, newOrgUnitMemoryStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("org_code invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=2026-01-01&org_code=bad%7F", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, newOrgUnitMemoryStore(), newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve org code invalid", func(t *testing.T) {
		store := resolveOrgIDErrStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), err: orgunitpkg.ErrOrgCodeInvalid}
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=2026-01-01&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, store, newJobCatalogMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("resolve org code not found", func(t *testing.T) {
		store := resolveOrgIDErrStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), err: orgunitpkg.ErrOrgCodeNotFound}
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=2026-01-01&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, store, newJobCatalogMemoryStore())
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("resolve org code error", func(t *testing.T) {
		store := resolveOrgIDErrStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), err: errors.New("boom")}
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=2026-01-01&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, store, newJobCatalogMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("setid resolver missing", func(t *testing.T) {
		// orgStoreStub implements OrgUnitStore but does not implement orgUnitSetIDResolver.
		orgStore := orgStoreStub{}
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=2026-01-01&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, orgStore, newJobCatalogMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("resolve setid error", func(t *testing.T) {
		orgStore := orgUnitMemoryStoreWithSetID{orgUnitMemoryStore: newOrgUnitMemoryStore(), err: errors.New("boom")}
		if _, err := orgStore.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Org", "", true); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=2026-01-01&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, orgStore, newJobCatalogMemoryStore())
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("setid missing", func(t *testing.T) {
		orgStore := orgUnitMemoryStoreWithSetID{orgUnitMemoryStore: newOrgUnitMemoryStore(), setID: ""}
		if _, err := orgStore.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Org", "", true); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=2026-01-01&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, orgStore, newJobCatalogMemoryStore())
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("jobcatalog store missing", func(t *testing.T) {
		orgStore := orgUnitMemoryStoreWithSetID{orgUnitMemoryStore: newOrgUnitMemoryStore(), setID: "S1"}
		if _, err := orgStore.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Org", "", true); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=2026-01-01&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, orgStore, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("list job profiles error", func(t *testing.T) {
		orgStore := orgUnitMemoryStoreWithSetID{orgUnitMemoryStore: newOrgUnitMemoryStore(), setID: "S1"}
		if _, err := orgStore.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Org", "", true); err != nil {
			t.Fatal(err)
		}
		jobStore := jobStoreListProfilesErr{JobCatalogStore: newJobCatalogMemoryStore(), err: errors.New("boom")}
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=2026-01-01&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, orgStore, jobStore)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("ok", func(t *testing.T) {
		orgStore := orgUnitMemoryStoreWithSetID{orgUnitMemoryStore: newOrgUnitMemoryStore(), setID: "S1"}
		if _, err := orgStore.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "a001", "Org", "", true); err != nil {
			t.Fatal(err)
		}
		jobStore := newJobCatalogMemoryStore().(*jobcatalogMemoryStore)
		if err := jobStore.CreateJobProfile(context.Background(), "t1", "S1", "2026-01-01", "JP1", "Profile 1", "", []string{"F1"}, "F1"); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/org/api/positions:options?as_of=2026-01-01&org_code=a001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositionsOptionsAPI(rec, req, orgStore, jobStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}

		var out staffingPositionsOptionsAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.OrgCode != "A001" {
			t.Fatalf("org_code=%q", out.OrgCode)
		}
		if out.JobCatalogSetID != "S1" {
			t.Fatalf("setid=%q", out.JobCatalogSetID)
		}
		if out.AsOf != "2026-01-01" {
			t.Fatalf("as_of=%q", out.AsOf)
		}
		if len(out.JobProfiles) != 1 {
			t.Fatalf("job_profiles=%+v", out.JobProfiles)
		}
	})
}
