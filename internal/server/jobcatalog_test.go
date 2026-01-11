package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type jobcatalogRows struct {
	idx     int
	rows    [][]any
	scanErr error
	err     error
}

func (r *jobcatalogRows) Close()                        {}
func (r *jobcatalogRows) Err() error                    { return r.err }
func (r *jobcatalogRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *jobcatalogRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *jobcatalogRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *jobcatalogRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.rows[r.idx-1]
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *bool:
			*d = row[i].(bool)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}
func (r *jobcatalogRows) Values() ([]any, error) { return nil, nil }
func (r *jobcatalogRows) RawValues() [][]byte    { return nil }
func (r *jobcatalogRows) Conn() *pgx.Conn        { return nil }

type errJobCatalogStore struct {
	err error
}

func (s errJobCatalogStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return nil, s.err
}
func (s errJobCatalogStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "", s.err
}
func (s errJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s errJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "", s.err
}
func (s errJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return s.err
}
func (s errJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return s.err
}
func (s errJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "", s.err
}
func (s errJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s errJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "", s.err
}
func (s errJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return s.err
}
func (s errJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "", s.err
}

type partialJobCatalogStore struct {
	businessUnits []BusinessUnit
	listErr       error
}

func (s partialJobCatalogStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return append([]BusinessUnit(nil), s.businessUnits...), nil
}
func (s partialJobCatalogStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "SHARE", nil
}
func (s partialJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s partialJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "SHARE", s.listErr
}
func (s partialJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s partialJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s partialJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "SHARE", nil
}
func (s partialJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s partialJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", nil
}
func (s partialJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s partialJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "SHARE", nil
}

type createErrJobCatalogStore struct {
	businessUnits []BusinessUnit
	err           error
}

func (s createErrJobCatalogStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return append([]BusinessUnit(nil), s.businessUnits...), nil
}
func (s createErrJobCatalogStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "SHARE", nil
}
func (s createErrJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s createErrJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "SHARE", nil
}
func (s createErrJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s createErrJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s createErrJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "SHARE", nil
}
func (s createErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", nil
}
func (s createErrJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s createErrJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "SHARE", nil
}

func TestHandleJobCatalog_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/job-catalog", nil)
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, newJobCatalogMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_InvalidAsOf(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/job-catalog?as_of=bad", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, newJobCatalogMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_ListBUError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/job-catalog?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, errJobCatalogStore{err: errors.New("boom")})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_GetAndPost_Create(t *testing.T) {
	store := newJobCatalogMemoryStore()

	_, _ = store.ResolveSetID(context.Background(), "t1", "", "jobcatalog")
	_ = store.CreateJobFamilyGroup(context.Background(), "t1", "", "2026-01-01", "JC0", "G0", "")
	_, _, _ = store.ListJobFamilyGroups(context.Background(), "t1", "", "2026-01-01")

	reqGet := httptest.NewRequest(http.MethodGet, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", nil)
	reqGet = reqGet.WithContext(withTenant(reqGet.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recGet := httptest.NewRecorder()
	handleJobCatalog(recGet, reqGet, store)
	if recGet.Code != http.StatusOK {
		t.Fatalf("get status=%d", recGet.Code)
	}

	form := url.Values{}
	form.Set("action", "create_job_family_group")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_family_group_code", "JC1")
	form.Set("job_family_group_name", "Group1")
	form.Set("job_family_group_description", "")

	reqPost := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	reqPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqPost = reqPost.WithContext(withTenant(reqPost.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recPost := httptest.NewRecorder()
	handleJobCatalog(recPost, reqPost, store)
	if recPost.Code != http.StatusSeeOther {
		t.Fatalf("post status=%d", recPost.Code)
	}
}

func TestHandleJobCatalog_Post_BadForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?as_of=2026-01-01", strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, newJobCatalogMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_Post_UnknownAction(t *testing.T) {
	form := url.Values{}
	form.Set("action", "nope")
	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, newJobCatalogMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_Post_InvalidEffectiveDate(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_job_family_group")
	form.Set("effective_date", "bad")
	form.Set("business_unit_id", "BU000")
	form.Set("job_family_group_code", "JC1")
	form.Set("job_family_group_name", "Group1")
	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, newJobCatalogMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_Post_MissingCode(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_job_family_group")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_family_group_code", "")
	form.Set("job_family_group_name", "")
	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, newJobCatalogMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_Post_CreateError(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_job_family_group")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_family_group_code", "JC1")
	form.Set("job_family_group_name", "Group1")
	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, errJobCatalogStore{err: errors.New("boom")})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

type createLevelErrJobCatalogStore struct {
	businessUnits []BusinessUnit
	err           error
}

func (s createLevelErrJobCatalogStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return append([]BusinessUnit(nil), s.businessUnits...), nil
}
func (s createLevelErrJobCatalogStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "SHARE", nil
}
func (s createLevelErrJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createLevelErrJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "SHARE", nil
}
func (s createLevelErrJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s createLevelErrJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s createLevelErrJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "SHARE", nil
}
func (s createLevelErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s createLevelErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", nil
}
func (s createLevelErrJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s createLevelErrJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "SHARE", nil
}

type levelsListErrJobCatalogStore struct {
	businessUnits []BusinessUnit
	err           error
}

func (s levelsListErrJobCatalogStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return append([]BusinessUnit(nil), s.businessUnits...), nil
}
func (s levelsListErrJobCatalogStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "SHARE", nil
}
func (s levelsListErrJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s levelsListErrJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "SHARE", nil
}
func (s levelsListErrJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s levelsListErrJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s levelsListErrJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "SHARE", nil
}
func (s levelsListErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s levelsListErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", s.err
}
func (s levelsListErrJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s levelsListErrJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "SHARE", nil
}

func TestHandleJobCatalog_Post_CreateJobLevel_Success(t *testing.T) {
	store := newJobCatalogMemoryStore()

	form := url.Values{}
	form.Set("action", "create_job_level")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_level_code", "JL1")
	form.Set("job_level_name", "Level1")
	form.Set("job_level_description", "")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_Post_CreateJobLevel_MissingCode(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_job_level")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_level_code", "")
	form.Set("job_level_name", "")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, newJobCatalogMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_Post_CreateJobFamily_Success(t *testing.T) {
	store := newJobCatalogMemoryStore()

	form := url.Values{}
	form.Set("action", "create_job_family")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_family_code", "JF-BE")
	form.Set("job_family_name", "Backend")
	form.Set("job_family_group_code", "JFG-ENG")
	form.Set("job_family_description", "")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

type createJobFamilyErrStore struct {
	err error
}

func (s createJobFamilyErrStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}, nil
}
func (s createJobFamilyErrStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "SHARE", nil
}
func (s createJobFamilyErrStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createJobFamilyErrStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "SHARE", nil
}
func (s createJobFamilyErrStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return s.err
}
func (s createJobFamilyErrStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s createJobFamilyErrStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "SHARE", nil
}
func (s createJobFamilyErrStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createJobFamilyErrStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", nil
}
func (s createJobFamilyErrStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s createJobFamilyErrStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "SHARE", nil
}

func TestHandleJobCatalog_Post_CreateJobFamily_Error_ShowsPage(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_job_family")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_family_code", "JF-BE")
	form.Set("job_family_name", "Backend")
	form.Set("job_family_group_code", "JFG-ENG")
	form.Set("job_family_description", "")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, createJobFamilyErrStore{err: errors.New("boom")})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleJobCatalog_Post_CreateJobFamily_MissingGroup(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_job_family")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_family_code", "JF-BE")
	form.Set("job_family_name", "Backend")
	form.Set("job_family_group_code", "")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, newJobCatalogMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_Post_UpdateJobFamilyGroup_Success(t *testing.T) {
	store := newJobCatalogMemoryStore()
	_ = store.CreateJobFamily(context.Background(), "t1", "BU000", "2026-01-01", "JF-BE", "Backend", "", "JFG-ENG")

	form := url.Values{}
	form.Set("action", "update_job_family_group")
	form.Set("effective_date", "2026-02-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_family_code", "JF-BE")
	form.Set("job_family_group_code", "JFG-SALES")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_Post_UpdateJobFamilyGroup_MissingFields(t *testing.T) {
	form := url.Values{}
	form.Set("action", "update_job_family_group")
	form.Set("effective_date", "2026-02-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_family_code", "")
	form.Set("job_family_group_code", "")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, newJobCatalogMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

type updateJobFamilyGroupErrStore struct {
	err error
}

func (s updateJobFamilyGroupErrStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}, nil
}
func (s updateJobFamilyGroupErrStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "SHARE", nil
}
func (s updateJobFamilyGroupErrStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s updateJobFamilyGroupErrStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "SHARE", nil
}
func (s updateJobFamilyGroupErrStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s updateJobFamilyGroupErrStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return s.err
}
func (s updateJobFamilyGroupErrStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "SHARE", nil
}
func (s updateJobFamilyGroupErrStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s updateJobFamilyGroupErrStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", nil
}
func (s updateJobFamilyGroupErrStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s updateJobFamilyGroupErrStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "SHARE", nil
}

func TestHandleJobCatalog_Post_UpdateJobFamilyGroup_Error_ShowsPage(t *testing.T) {
	form := url.Values{}
	form.Set("action", "update_job_family_group")
	form.Set("effective_date", "2026-02-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_family_code", "JF-BE")
	form.Set("job_family_group_code", "JFG-SALES")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, updateJobFamilyGroupErrStore{err: errors.New("boom")})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleJobCatalog_Post_CreateJobProfile_Success(t *testing.T) {
	store := newJobCatalogMemoryStore()

	form := url.Values{}
	form.Set("action", "create_job_profile")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_profile_code", "JP-SWE")
	form.Set("job_profile_name", "Software Engineer")
	form.Set("job_profile_family_codes", "JF-BE,JF-FE")
	form.Set("job_profile_primary_family_code", "JF-BE")
	form.Set("job_profile_description", "")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

type createJobProfileErrStore struct {
	err error
}

func (s createJobProfileErrStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}, nil
}
func (s createJobProfileErrStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "SHARE", nil
}
func (s createJobProfileErrStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createJobProfileErrStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "SHARE", nil
}
func (s createJobProfileErrStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s createJobProfileErrStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s createJobProfileErrStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "SHARE", nil
}
func (s createJobProfileErrStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createJobProfileErrStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", nil
}
func (s createJobProfileErrStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return s.err
}
func (s createJobProfileErrStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "SHARE", nil
}

func TestHandleJobCatalog_Post_CreateJobProfile_Error_ShowsPage(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_job_profile")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_profile_code", "JP-SWE")
	form.Set("job_profile_name", "Software Engineer")
	form.Set("job_profile_family_codes", "JF-BE,JF-FE")
	form.Set("job_profile_primary_family_code", "JF-BE")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, createJobProfileErrStore{err: errors.New("boom")})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleJobCatalog_Post_CreateJobProfile_MissingFamilies(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_job_profile")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_profile_code", "JP-SWE")
	form.Set("job_profile_name", "Software Engineer")
	form.Set("job_profile_family_codes", "")
	form.Set("job_profile_primary_family_code", "")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, newJobCatalogMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

type listFamiliesErrJobCatalogStore struct {
	err error
}

func (s listFamiliesErrJobCatalogStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}, nil
}
func (s listFamiliesErrJobCatalogStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "SHARE", nil
}
func (s listFamiliesErrJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s listFamiliesErrJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "SHARE", nil
}
func (s listFamiliesErrJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s listFamiliesErrJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s listFamiliesErrJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "SHARE", s.err
}
func (s listFamiliesErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s listFamiliesErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", nil
}
func (s listFamiliesErrJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s listFamiliesErrJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "SHARE", nil
}

func TestHandleJobCatalog_Get_ListFamiliesError_ShowsError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/job-catalog?as_of=2026-01-01&business_unit_id=BU000", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, listFamiliesErrJobCatalogStore{err: errors.New("boom")})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}
}

type listProfilesErrJobCatalogStore struct {
	err error
}

func (s listProfilesErrJobCatalogStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}, nil
}
func (s listProfilesErrJobCatalogStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "SHARE", nil
}
func (s listProfilesErrJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s listProfilesErrJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "SHARE", nil
}
func (s listProfilesErrJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s listProfilesErrJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s listProfilesErrJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "SHARE", nil
}
func (s listProfilesErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s listProfilesErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", nil
}
func (s listProfilesErrJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s listProfilesErrJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "SHARE", s.err
}

func TestHandleJobCatalog_Get_ListProfilesError_ShowsError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/job-catalog?as_of=2026-01-01&business_unit_id=BU000", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, listProfilesErrJobCatalogStore{err: errors.New("boom")})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleJobCatalog_Post_CreateJobLevel_Error(t *testing.T) {
	bus := []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}
	store := createLevelErrJobCatalogStore{businessUnits: bus, err: errors.New("boom")}

	form := url.Values{}
	form.Set("action", "create_job_level")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_level_code", "JL1")
	form.Set("job_level_name", "Level1")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleJobCatalog_Get_ListJobLevelsError(t *testing.T) {
	bus := []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}
	store := levelsListErrJobCatalogStore{businessUnits: bus, err: errors.New("boom")}

	req := httptest.NewRequest(http.MethodGet, "/org/job-catalog?business_unit_id=BU000&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestJobCatalogPGStore_CreateAndListJobLevels(t *testing.T) {
	makeTx := func() *stubTx {
		return &stubTx{
			row:  &stubRow{vals: []any{"SHARE"}},
			row2: &stubRow{vals: []any{"level-id"}},
			row3: &stubRow{vals: []any{"event-id"}},
		}
	}

	store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return makeTx(), nil
	})).(*jobcatalogPGStore)

	if err := store.CreateJobLevel(context.Background(), "t1", "BU000", "2026-01-01", "JL1", "Level 1", "desc"); err != nil {
		t.Fatalf("err=%v", err)
	}

	store2 := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return makeTx(), nil
	})).(*jobcatalogPGStore)
	if err := store2.CreateJobLevel(context.Background(), "t1", "BU000", "2026-01-01", "JL1", "Level 1", ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	rows := &jobcatalogRows{rows: [][]any{{"id1", "JL1", "Level 1", true, "2026-01-01"}}}
	store3 := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{
			row:  &stubRow{vals: []any{"SHARE"}},
			rows: rows,
		}, nil
	})).(*jobcatalogPGStore)
	levels, setID, err := store3.ListJobLevels(context.Background(), "t1", "BU000", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if setID != "SHARE" {
		t.Fatalf("setid=%q", setID)
	}
	if len(levels) != 1 {
		t.Fatalf("levels=%d", len(levels))
	}
}

func TestJobCatalogPGStore_CreateAndListJobFamilies(t *testing.T) {
	makeTx := func() *stubTx {
		return &stubTx{
			row:  &stubRow{vals: []any{"SHARE"}},
			row2: &stubRow{vals: []any{"group-id"}},
			row3: &stubRow{vals: []any{"family-id"}},
			row4: &stubRow{vals: []any{"event-id"}},
		}
	}

	store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return makeTx(), nil
	})).(*jobcatalogPGStore)

	if err := store.CreateJobFamily(context.Background(), "t1", "BU000", "2026-01-01", "JF-BE", "Backend", "desc", "JFG-ENG"); err != nil {
		t.Fatalf("err=%v", err)
	}
	storeNullDesc := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return makeTx(), nil
	})).(*jobcatalogPGStore)
	if err := storeNullDesc.CreateJobFamily(context.Background(), "t1", "BU000", "2026-01-01", "JF-FE", "Frontend", "", "JFG-ENG"); err != nil {
		t.Fatalf("err=%v", err)
	}

	rows := &jobcatalogRows{rows: [][]any{{"id1", "JF-BE", "JFG-ENG", "Backend", true, "2026-01-01"}}}
	store2 := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{
			row:  &stubRow{vals: []any{"SHARE"}},
			rows: rows,
		}, nil
	})).(*jobcatalogPGStore)
	families, setID, err := store2.ListJobFamilies(context.Background(), "t1", "BU000", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if setID != "SHARE" {
		t.Fatalf("setid=%q", setID)
	}
	if len(families) != 1 {
		t.Fatalf("families=%d", len(families))
	}
}

func TestJobCatalogPGStore_CreateJobFamily_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "group_lookup_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, row2Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "family_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, row2: &stubRow{vals: []any{"group-id"}}, row3Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "event_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, row2: &stubRow{vals: []any{"group-id"}}, row3: &stubRow{vals: []any{"family-id"}}, row4Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "submit_exec_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, row2: &stubRow{vals: []any{"group-id"}}, row3: &stubRow{vals: []any{"family-id"}}, row4: &stubRow{vals: []any{"event-id"}}, execErr: errors.New("boom"), execErrAt: 3},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			err := store.CreateJobFamily(context.Background(), "t1", "BU000", "2026-01-01", "JF-BE", "Backend", "desc", "JFG-ENG")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_ListJobFamilies_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "query_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, queryErr: errors.New("boom"), queryErrAt: 1},
			wantError: true,
		},
		{
			name:      "scan_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: [][]any{{"id1", "JF-BE", "JFG-ENG", "Backend", true, "2026-01-01"}}, scanErr: errors.New("boom")}},
			wantError: true,
		},
		{
			name:      "rows_err",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: nil, err: errors.New("boom")}},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			_, _, err := store.ListJobFamilies(context.Background(), "t1", "BU000", "2026-01-01")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_UpdateJobFamilyGroup_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "group_lookup_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, row2Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "family_lookup_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, row2: &stubRow{vals: []any{"group-id"}}, row3Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "event_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, row2: &stubRow{vals: []any{"group-id"}}, row3: &stubRow{vals: []any{"family-id"}}, row4Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "submit_exec_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, row2: &stubRow{vals: []any{"group-id"}}, row3: &stubRow{vals: []any{"family-id"}}, row4: &stubRow{vals: []any{"event-id"}}, execErr: errors.New("boom"), execErrAt: 3},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			err := store.UpdateJobFamilyGroup(context.Background(), "t1", "BU000", "2026-02-01", "JF-BE", "JFG-SALES")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_CreateAndListJobProfiles(t *testing.T) {
	rows := &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}, {"JF-FE", "family2"}}}
	store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{
			row:  &stubRow{vals: []any{"SHARE"}},
			rows: rows,
			row2: &stubRow{vals: []any{"profile-id"}},
			row3: &stubRow{vals: []any{"event-id"}},
		}, nil
	})).(*jobcatalogPGStore)
	if err := store.CreateJobProfile(context.Background(), "t1", "BU000", "2026-01-01", "JP-SWE", "Software Engineer", "desc", []string{"JF-BE", "JF-FE"}, "JF-BE"); err != nil {
		t.Fatalf("err=%v", err)
	}
	rows2 := &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}, {"JF-FE", "family2"}}}
	storePrimaryNotInFamilies := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{
			row:  &stubRow{vals: []any{"SHARE"}},
			rows: rows2,
			row2: &stubRow{vals: []any{"profile-id"}},
			row3: &stubRow{vals: []any{"event-id"}},
		}, nil
	})).(*jobcatalogPGStore)
	if err := storePrimaryNotInFamilies.CreateJobProfile(context.Background(), "t1", "BU000", "2026-01-01", "JP-BAD", "Bad", "", []string{"JF-BE"}, "JF-FE"); err != nil {
		t.Fatalf("err=%v", err)
	}

	rowsProfiles := &jobcatalogRows{rows: [][]any{{"id1", "JP-SWE", "Software Engineer", true, "2026-01-01", "JF-BE,JF-FE", "JF-BE"}}}
	store2 := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{
			row:  &stubRow{vals: []any{"SHARE"}},
			rows: rowsProfiles,
		}, nil
	})).(*jobcatalogPGStore)
	profiles, setID, err := store2.ListJobProfiles(context.Background(), "t1", "BU000", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if setID != "SHARE" {
		t.Fatalf("setid=%q", setID)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles=%d", len(profiles))
	}
}

func TestJobCatalogPGStore_CreateJobProfile_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		families  []string
		primary   string
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "family_query_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, queryErr: errors.New("boom"), queryErrAt: 1},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "family_scan_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}, scanErr: errors.New("boom")}},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "family_rows_err",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: nil, err: errors.New("boom")}},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "family_missing",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}}},
			families:  []string{"JF-BE", "JF-FE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "primary_missing",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}}},
			families:  []string{"JF-BE"},
			primary:   "JF-FE",
			wantError: true,
		},
		{
			name:      "profile_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}}, row2Err: errors.New("boom")},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "event_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}}, row2: &stubRow{vals: []any{"profile-id"}}, row3Err: errors.New("boom")},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "submit_exec_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}}, row2: &stubRow{vals: []any{"profile-id"}}, row3: &stubRow{vals: []any{"event-id"}}, execErr: errors.New("boom"), execErrAt: 3},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			err := store.CreateJobProfile(context.Background(), "t1", "BU000", "2026-01-01", "JP-SWE", "Software Engineer", "desc", tc.families, tc.primary)
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_ListJobProfiles_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "query_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, queryErr: errors.New("boom"), queryErrAt: 1},
			wantError: true,
		},
		{
			name:      "scan_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: [][]any{{"id1", "JP-SWE", "Software Engineer", true, "2026-01-01", "JF-BE", "JF-BE"}}, scanErr: errors.New("boom")}},
			wantError: true,
		},
		{
			name:      "rows_err",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: nil, err: errors.New("boom")}},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			_, _, err := store.ListJobProfiles(context.Background(), "t1", "BU000", "2026-01-01")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_CreateJobLevel_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "level_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, row2Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "event_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, row2: &stubRow{vals: []any{"level-id"}}, row3Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "submit_exec_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, row2: &stubRow{vals: []any{"level-id"}}, row3: &stubRow{vals: []any{"event-id"}}, execErr: errors.New("boom"), execErrAt: 3},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			err := store.CreateJobLevel(context.Background(), "t1", "BU000", "2026-01-01", "JL1", "Level 1", "desc")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_ListJobLevels_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "query_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, queryErr: errors.New("boom"), queryErrAt: 1},
			wantError: true,
		},
		{
			name:      "scan_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: [][]any{{"id1", "JL1", "Level 1", true, "2026-01-01"}}, scanErr: errors.New("boom")}},
			wantError: true,
		},
		{
			name:      "rows_err",
			tx:        &stubTx{row: &stubRow{vals: []any{"SHARE"}}, rows: &jobcatalogRows{rows: nil, err: errors.New("boom")}},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			_, _, err := store.ListJobLevels(context.Background(), "t1", "BU000", "2026-01-01")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogMemoryStore_CreateAndListJobLevels_DefaultBU(t *testing.T) {
	store := newJobCatalogMemoryStore().(*jobcatalogMemoryStore)
	if err := store.CreateJobLevel(context.Background(), "t1", "", "2026-01-01", "JL1", "Level 1", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := store.CreateJobLevel(context.Background(), "t1", "", "2026-01-01", "JL2", "Level 2", ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	levels, resolved, err := store.ListJobLevels(context.Background(), "t1", "", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if resolved != "SHARE" {
		t.Fatalf("resolved=%q", resolved)
	}
	if len(levels) != 2 {
		t.Fatalf("levels=%d", len(levels))
	}
}

func TestJobCatalogMemoryStore_CreateAndListJobFamilies_DefaultBU(t *testing.T) {
	store := newJobCatalogMemoryStore().(*jobcatalogMemoryStore)
	if err := store.CreateJobFamily(context.Background(), "t1", "", "2026-01-01", "JF-BE", "Backend", "", "JFG-ENG"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := store.UpdateJobFamilyGroup(context.Background(), "t1", "", "2026-02-01", "JF-BE", "JFG-SALES"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := store.UpdateJobFamilyGroup(context.Background(), "t1", "", "2026-02-01", "NOPE", "JFG-SALES"); err != nil {
		t.Fatalf("err=%v", err)
	}

	families, resolved, err := store.ListJobFamilies(context.Background(), "t1", "", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if resolved != "SHARE" {
		t.Fatalf("resolved=%q", resolved)
	}
	if len(families) != 1 {
		t.Fatalf("families=%d", len(families))
	}
}

func TestJobCatalogMemoryStore_CreateAndListJobProfiles_DefaultBU(t *testing.T) {
	store := newJobCatalogMemoryStore().(*jobcatalogMemoryStore)
	if err := store.CreateJobProfile(context.Background(), "t1", "", "2026-01-01", "JP-SWE", "Software Engineer", "", []string{"JF-BE", "JF-FE"}, "JF-BE"); err != nil {
		t.Fatalf("err=%v", err)
	}
	profiles, resolved, err := store.ListJobProfiles(context.Background(), "t1", "", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if resolved != "SHARE" {
		t.Fatalf("resolved=%q", resolved)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles=%d", len(profiles))
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" a, ,b,  , c ")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("got=%v", got)
	}
}

func TestQuoteAll(t *testing.T) {
	got := quoteAll([]string{"x", "y"})
	if len(got) != 2 || got[0] != `"x"` || got[1] != `"y"` {
		t.Fatalf("got=%v", got)
	}
}

func TestRenderJobCatalog_RendersFamiliesAndProfiles(t *testing.T) {
	bus := []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}
	html := renderJobCatalog(
		[]JobFamilyGroup{
			{ID: "g1", Code: "JFG-ENG", Name: "Engineering", IsActive: true, EffectiveDay: "2026-01-01"},
			{ID: "g2", Code: "JFG-OFF", Name: "Off", IsActive: false, EffectiveDay: "2026-01-01"},
		},
		[]JobFamily{
			{ID: "f1", Code: "JF-BE", GroupCode: "JFG-ENG", Name: "Backend", IsActive: true, EffectiveDay: "2026-01-01"},
			{ID: "f2", Code: "JF-OFF", GroupCode: "JFG-OFF", Name: "Off", IsActive: false, EffectiveDay: "2026-01-01"},
		},
		[]JobLevel{
			{ID: "l1", Code: "JL-1", Name: "Level 1", IsActive: true, EffectiveDay: "2026-01-01"},
			{ID: "l2", Code: "JL-OFF", Name: "Off", IsActive: false, EffectiveDay: "2026-01-01"},
		},
		[]JobProfile{
			{ID: "p1", Code: "JP-SWE", Name: "Software Engineer", IsActive: true, EffectiveDay: "2026-01-01", FamilyCodesCSV: "JF-BE,JF-FE", PrimaryFamilyCode: "JF-BE"},
			{ID: "p2", Code: "JP-OFF", Name: "Off", IsActive: false, EffectiveDay: "2026-01-01", FamilyCodesCSV: "JF-OFF", PrimaryFamilyCode: "JF-OFF"},
		},
		bus,
		Tenant{Name: "T"},
		"BU000",
		"err",
		"2026-01-01",
		"SHARE",
	)
	if !strings.Contains(html, "Job Families") || !strings.Contains(html, "Job Profiles") {
		t.Fatalf("unexpected html")
	}
}

type resolvedFallbackJobCatalogStore struct{}

func (resolvedFallbackJobCatalogStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return []BusinessUnit{
		{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"},
		{BusinessUnitID: "BU001", Name: "Other BU", Status: "active"},
	}, nil
}
func (resolvedFallbackJobCatalogStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "SHARE", nil
}
func (resolvedFallbackJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (resolvedFallbackJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "", nil
}
func (resolvedFallbackJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (resolvedFallbackJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (resolvedFallbackJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "", nil
}
func (resolvedFallbackJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (resolvedFallbackJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", nil
}
func (resolvedFallbackJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (resolvedFallbackJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "", nil
}

func TestHandleJobCatalog_Get_ResolvedFallbackToLevels(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/job-catalog?as_of=2026-01-01&business_unit_id=BU001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, resolvedFallbackJobCatalogStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Resolved SetID") || !strings.Contains(body, "SHARE") {
		t.Fatalf("unexpected body: %q", body)
	}
}

type resolvedFallbackToProfilesJobCatalogStore struct{}

func (resolvedFallbackToProfilesJobCatalogStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return []BusinessUnit{
		{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"},
		{BusinessUnitID: "BU001", Name: "Other BU", Status: "active"},
	}, nil
}
func (resolvedFallbackToProfilesJobCatalogStore) ResolveSetID(context.Context, string, string, string) (string, error) {
	return "SHARE", nil
}
func (resolvedFallbackToProfilesJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (resolvedFallbackToProfilesJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, string, error) {
	return nil, "", nil
}
func (resolvedFallbackToProfilesJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (resolvedFallbackToProfilesJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (resolvedFallbackToProfilesJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, string, error) {
	return nil, "", nil
}
func (resolvedFallbackToProfilesJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (resolvedFallbackToProfilesJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "", nil
}
func (resolvedFallbackToProfilesJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (resolvedFallbackToProfilesJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, string, error) {
	return nil, "SHARE", nil
}

func TestHandleJobCatalog_Get_ResolvedFallbackToProfiles(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/job-catalog?as_of=2026-01-01&business_unit_id=BU001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, resolvedFallbackToProfilesJobCatalogStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Resolved SetID") || !strings.Contains(body, "SHARE") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleJobCatalog_Get_RendersJobLevels(t *testing.T) {
	store := newJobCatalogMemoryStore()
	_ = store.CreateJobLevel(context.Background(), "t1", "BU000", "2026-01-01", "JL1", "Level 1", "")

	req := httptest.NewRequest(http.MethodGet, "/org/job-catalog?as_of=2026-01-01&business_unit_id=BU000", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Job Levels") || !strings.Contains(body, "JL1") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleJobCatalog_DefaultsAndMethodNotAllowed(t *testing.T) {
	store := newJobCatalogMemoryStore()
	req := httptest.NewRequest(http.MethodGet, "/org/job-catalog?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	bus := []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}
	_ = renderJobCatalog(nil, nil, nil, nil, bus, Tenant{Name: "T"}, "BU000", "", "2026-01-01", "")
	_ = renderJobCatalog([]JobFamilyGroup{{ID: "g1", Code: "C", Name: "N", IsActive: true, EffectiveDay: "2026-01-01"}}, nil, nil, nil, bus, Tenant{Name: "T"}, "BU000", "err", "2026-01-01", "SHARE")

	req2 := httptest.NewRequest(http.MethodPut, "/org/job-catalog?as_of=2026-01-01", nil)
	req2 = req2.WithContext(withTenant(req2.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec2 := httptest.NewRecorder()
	handleJobCatalog(rec2, req2, store)
	if rec2.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec2.Code)
	}
}

func TestHandleJobCatalog_MergeMsgBranches(t *testing.T) {
	bus := []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}

	reqGet := httptest.NewRequest(http.MethodGet, "/org/job-catalog?as_of=2026-01-01&business_unit_id=BU000", nil)
	reqGet = reqGet.WithContext(withTenant(reqGet.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recGet := httptest.NewRecorder()
	handleJobCatalog(recGet, reqGet, partialJobCatalogStore{businessUnits: bus, listErr: errors.New("boom")})
	if recGet.Code != http.StatusOK {
		t.Fatalf("get status=%d", recGet.Code)
	}

	reqPost2 := httptest.NewRequest(http.MethodPost, "/org/job-catalog?as_of=2026-01-01", strings.NewReader("%"))
	reqPost2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqPost2 = reqPost2.WithContext(withTenant(reqPost2.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recPost2 := httptest.NewRecorder()
	handleJobCatalog(recPost2, reqPost2, partialJobCatalogStore{businessUnits: bus, listErr: errors.New("boom")})
	if recPost2.Code != http.StatusOK {
		t.Fatalf("post status=%d", recPost2.Code)
	}
	if body := recPost2.Body.String(); !strings.Contains(body, "bad form") || !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}

	reqPost := httptest.NewRequest(http.MethodPost, "/org/job-catalog?as_of=2026-01-01", strings.NewReader("%"))
	reqPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqPost = reqPost.WithContext(withTenant(reqPost.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recPost := httptest.NewRecorder()
	handleJobCatalog(recPost, reqPost, partialJobCatalogStore{businessUnits: bus, listErr: errors.New("")})
	if recPost.Code != http.StatusOK {
		t.Fatalf("post status=%d", recPost.Code)
	}
	if body := recPost.Body.String(); !strings.Contains(body, "bad form") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleJobCatalog_SortsActiveBUs(t *testing.T) {
	bus := []BusinessUnit{
		{BusinessUnitID: "BU002", Name: "BU 2", Status: "active"},
		{BusinessUnitID: "BU001", Name: "BU 1", Status: "active"},
	}

	req := httptest.NewRequest(http.MethodGet, "/org/job-catalog?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, partialJobCatalogStore{businessUnits: bus})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleJobCatalog_CreateJobFamilyGroupError_ShowsPage(t *testing.T) {
	bus := []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}

	form := url.Values{}
	form.Set("action", "create_job_family_group")
	form.Set("effective_date", "2026-01-01")
	form.Set("business_unit_id", "BU000")
	form.Set("job_family_group_code", "JC1")
	form.Set("job_family_group_name", "Group1")

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, createErrJobCatalogStore{businessUnits: bus, err: errors.New("boom")})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleJobCatalog_Post_DefaultActionAndEffectiveDate(t *testing.T) {
	store := newJobCatalogMemoryStore()
	reqPre := httptest.NewRequest(http.MethodGet, "/org/job-catalog?as_of=2026-01-01", nil)
	reqPre = reqPre.WithContext(withTenant(reqPre.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recPre := httptest.NewRecorder()
	handleJobCatalog(recPre, reqPre, store)
	if recPre.Code != http.StatusOK {
		t.Fatalf("pre status=%d", recPre.Code)
	}

	form := url.Values{}
	form.Set("effective_date", "")
	form.Set("business_unit_id", "")
	form.Set("job_family_group_code", "JC1")
	form.Set("job_family_group_name", "Group1")
	form.Set("job_family_group_description", "")
	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleJobCatalog(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestJobCatalogPGStore_WithTxAndMethods(t *testing.T) {
	beginErrStore := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin fail")
	})}
	if _, err := beginErrStore.ListBusinessUnits(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	tx := &stubTx{rows: &jobcatalogRows{rows: [][]any{{"BU000", "Default BU", "active"}}}}
	s := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	if bus, err := s.ListBusinessUnits(context.Background(), "t1"); err != nil || len(bus) != 1 {
		t.Fatalf("len=%d err=%v", len(bus), err)
	}

	txBootstrapErr := &stubTx{execErr: errors.New("bootstrap fail"), execErrAt: 2}
	sBootstrapErr := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txBootstrapErr, nil })}
	if _, err := sBootstrapErr.ListBusinessUnits(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txScanErr := &stubTx{rows: &scanErrRows{}}
	sScanErr := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txScanErr, nil })}
	if _, err := sScanErr.ListBusinessUnits(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	tx2 := &stubTx{row: fakeRow{vals: []any{"SHARE"}}}
	s2 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx2, nil })}
	if sid, err := s2.ResolveSetID(context.Background(), "t1", "BU000", "jobcatalog"); err != nil || sid != "SHARE" {
		t.Fatalf("sid=%q err=%v", sid, err)
	}

	tx3 := &stubTx{
		row:  fakeRow{vals: []any{"SHARE"}},
		row2: fakeRow{vals: []any{"g1"}},
		row3: fakeRow{vals: []any{"e1"}},
	}
	s3 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3, nil })}
	if err := s3.CreateJobFamilyGroup(context.Background(), "t1", "BU000", "2026-01-01", "JC1", "Group1", ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	tx3a := &stubTx{execErr: errors.New("bootstrap fail"), execErrAt: 2}
	s3a := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3a, nil })}
	if err := s3a.CreateJobFamilyGroup(context.Background(), "t1", "BU000", "2026-01-01", "JC1", "Group1", "desc"); err == nil {
		t.Fatal("expected error")
	}

	tx3ResolveErr := &stubTx{rowErr: errors.New("resolve fail")}
	s3ResolveErr := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3ResolveErr, nil })}
	if err := s3ResolveErr.CreateJobFamilyGroup(context.Background(), "t1", "BU000", "2026-01-01", "JC1", "Group1", "desc"); err == nil {
		t.Fatal("expected error")
	}

	tx3b := &stubTx{
		row:     fakeRow{vals: []any{"SHARE"}},
		row2:    fakeRow{vals: []any{"g1"}},
		row3Err: errors.New("uuid fail"),
	}
	s3b := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3b, nil })}
	if err := s3b.CreateJobFamilyGroup(context.Background(), "t1", "BU000", "2026-01-01", "JC1", "Group1", "desc"); err == nil {
		t.Fatal("expected error")
	}

	tx3c := &stubTx{
		execErr:   errors.New("exec fail"),
		execErrAt: 3,
		row:       fakeRow{vals: []any{"SHARE"}},
		row2:      fakeRow{vals: []any{"g1"}},
		row3:      fakeRow{vals: []any{"e1"}},
	}
	s3c := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3c, nil })}
	if err := s3c.CreateJobFamilyGroup(context.Background(), "t1", "BU000", "2026-01-01", "JC1", "Group1", "desc"); err == nil {
		t.Fatal("expected error")
	}

	tx4 := &stubTx{
		row: fakeRow{vals: []any{"SHARE"}},
		rows: &jobcatalogRows{rows: [][]any{
			{"g1", "JC1", "Group1", true, "2026-01-01"},
		}},
	}
	s4 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4, nil })}
	groups, resolved, err := s4.ListJobFamilyGroups(context.Background(), "t1", "BU000", "2026-01-01")
	if err != nil || resolved != "SHARE" || len(groups) != 1 {
		t.Fatalf("resolved=%q len=%d err=%v", resolved, len(groups), err)
	}

	tx4a := &stubTx{execErr: errors.New("bootstrap fail"), execErrAt: 2}
	s4a := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4a, nil })}
	if _, _, err := s4a.ListJobFamilyGroups(context.Background(), "t1", "BU000", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx4ResolveErr := &stubTx{rowErr: errors.New("resolve fail")}
	s4ResolveErr := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4ResolveErr, nil })}
	if _, _, err := s4ResolveErr.ListJobFamilyGroups(context.Background(), "t1", "BU000", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx4b := &stubTx{
		row:      fakeRow{vals: []any{"SHARE"}},
		queryErr: errors.New("query fail"),
	}
	s4b := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4b, nil })}
	if _, _, err := s4b.ListJobFamilyGroups(context.Background(), "t1", "BU000", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx4c := &stubTx{
		row:  fakeRow{vals: []any{"SHARE"}},
		rows: &scanErrRows{},
	}
	s4c := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4c, nil })}
	if _, _, err := s4c.ListJobFamilyGroups(context.Background(), "t1", "BU000", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
}

func TestJobCatalogPGStore_Errors(t *testing.T) {
	tx := &stubTx{queryErr: errors.New("query fail")}
	s := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	if _, err := s.ListBusinessUnits(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	tx2 := &stubTx{rowErr: errors.New("row fail")}
	s2 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx2, nil })}
	if _, err := s2.ResolveSetID(context.Background(), "t1", "BU000", "jobcatalog"); err == nil {
		t.Fatal("expected error")
	}

	tx3 := &stubTx{
		row:     fakeRow{vals: []any{"SHARE"}},
		row2Err: errors.New("uuid fail"),
	}
	s3 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3, nil })}
	if err := s3.CreateJobFamilyGroup(context.Background(), "t1", "BU000", "2026-01-01", "JC1", "Group1", ""); err == nil {
		t.Fatal("expected error")
	}

	tx4 := &stubTx{
		row:  fakeRow{vals: []any{"SHARE"}},
		rows: &jobcatalogRows{err: errors.New("rows err")},
	}
	s4 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4, nil })}
	if _, _, err := s4.ListJobFamilyGroups(context.Background(), "t1", "BU000", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx5 := &stubTx{execErr: errors.New("set_config fail"), execErrAt: 1}
	s5 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx5, nil })}
	if _, err := s5.ListBusinessUnits(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	tx6 := &stubTx{execErr: errors.New("bootstrap fail"), execErrAt: 2}
	s6 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx6, nil })}
	if _, err := s6.ResolveSetID(context.Background(), "t1", "BU000", "jobcatalog"); err == nil {
		t.Fatal("expected error")
	}

	tx7 := &stubTx{
		commitErr: errors.New("commit fail"),
		rows:      &jobcatalogRows{rows: [][]any{{"BU000", "Default BU", "active"}}},
	}
	s7 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx7, nil })}
	if _, err := s7.ListBusinessUnits(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}
}
