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
func (s errJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s errJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
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
func (s partialJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s partialJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
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
func (s createErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
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
	form.Set("code", "JC1")
	form.Set("name", "Group1")
	form.Set("description", "")

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
	form.Set("code", "JC1")
	form.Set("name", "Group1")
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
	form.Set("code", "")
	form.Set("name", "")
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
	form.Set("code", "JC1")
	form.Set("name", "Group1")
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
func (s createLevelErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s createLevelErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
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
func (s levelsListErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s levelsListErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", s.err
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
func (resolvedFallbackJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (resolvedFallbackJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, string, error) {
	return nil, "SHARE", nil
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

	_ = renderJobCatalog(nil, nil, []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}, Tenant{Name: "T"}, "BU000", "", "2026-01-01", "")
	_ = renderJobCatalog([]JobFamilyGroup{{ID: "g1", Code: "C", Name: "N", IsActive: true, EffectiveDay: "2026-01-01"}}, nil, []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}, Tenant{Name: "T"}, "BU000", "err", "2026-01-01", "SHARE")

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
	form.Set("code", "JC1")
	form.Set("name", "Group1")

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
	form.Set("code", "JC1")
	form.Set("name", "Group1")
	form.Set("description", "")
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
