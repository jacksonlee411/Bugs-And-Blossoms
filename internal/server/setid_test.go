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

type errSetIDStore struct{ err error }

func (s errSetIDStore) EnsureBootstrap(context.Context, string, string) error { return s.err }
func (s errSetIDStore) ListSetIDs(context.Context, string) ([]SetID, error)   { return nil, s.err }
func (s errSetIDStore) CreateSetID(context.Context, string, string, string, string, string) error {
	return s.err
}
func (s errSetIDStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	return nil, s.err
}
func (s errSetIDStore) CreateBusinessUnit(context.Context, string, string, string, string, string) error {
	return s.err
}
func (s errSetIDStore) ListMappings(context.Context, string, string) ([]SetIDMappingRow, error) {
	return nil, s.err
}
func (s errSetIDStore) PutMappings(context.Context, string, string, map[string]string, string, string) error {
	return s.err
}

type partialSetIDStore struct {
	listSetErr   error
	createSetErr error
	listBUErr    error
	createBUErr  error
	listMapErr   error
	putMapErr    error
}

func (s partialSetIDStore) EnsureBootstrap(context.Context, string, string) error { return nil }
func (s partialSetIDStore) ListSetIDs(context.Context, string) ([]SetID, error) {
	if s.listSetErr != nil {
		return nil, s.listSetErr
	}
	return []SetID{{SetID: "SHARE", Name: "Shared", Status: "active"}}, nil
}
func (s partialSetIDStore) CreateSetID(context.Context, string, string, string, string, string) error {
	return s.createSetErr
}
func (s partialSetIDStore) ListBusinessUnits(context.Context, string) ([]BusinessUnit, error) {
	if s.listBUErr != nil {
		return nil, s.listBUErr
	}
	return []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}, nil
}
func (s partialSetIDStore) CreateBusinessUnit(context.Context, string, string, string, string, string) error {
	return s.createBUErr
}
func (s partialSetIDStore) ListMappings(context.Context, string, string) ([]SetIDMappingRow, error) {
	if s.listMapErr != nil {
		return nil, s.listMapErr
	}
	return []SetIDMappingRow{{BusinessUnitID: "BU000", SetID: "SHARE"}}, nil
}
func (s partialSetIDStore) PutMappings(context.Context, string, string, map[string]string, string, string) error {
	return s.putMapErr
}

type tableRows struct {
	idx  int
	rows [][]any
	err  error
}

func (r *tableRows) Close()                        {}
func (r *tableRows) Err() error                    { return r.err }
func (r *tableRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *tableRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *tableRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *tableRows) Scan(dest ...any) error {
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
func (r *tableRows) Values() ([]any, error) { return nil, nil }
func (r *tableRows) RawValues() [][]byte    { return nil }
func (r *tableRows) Conn() *pgx.Conn        { return nil }

type scanErrRows struct {
	next bool
}

func (r *scanErrRows) Close()                        {}
func (r *scanErrRows) Err() error                    { return nil }
func (r *scanErrRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *scanErrRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *scanErrRows) Next() bool {
	if r.next {
		return false
	}
	r.next = true
	return true
}
func (r *scanErrRows) Scan(...any) error { return errors.New("scan fail") }
func (r *scanErrRows) Values() ([]any, error) {
	return nil, nil
}
func (r *scanErrRows) RawValues() [][]byte { return nil }
func (r *scanErrRows) Conn() *pgx.Conn     { return nil }

func TestStrconvQuote(t *testing.T) {
	got := strconvQuote(`a"b\c`)
	if got != `"a\"b\\c"` {
		t.Fatalf("got=%q", got)
	}
}

func TestHandleSetID_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_MissingAsOfRedirects(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()

	handleSetID(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "as_of=") {
		t.Fatalf("location=%q", rec.Header().Get("Location"))
	}
}

func TestHandleSetID_InvalidAsOfReturns400(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=BAD", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()

	handleSetID(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_Get(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "SetID Governance") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleSetID_Post_CreateSetID(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_setid")
	form.Set("setid", "A0001")
	form.Set("name", "Default A")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_Post_CreateBU_And_SaveMappings(t *testing.T) {
	store := newSetIDMemoryStore()

	createBU := url.Values{}
	createBU.Set("action", "create_bu")
	createBU.Set("business_unit_id", "BU001")
	createBU.Set("name", "BU 1")
	reqBU := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(createBU.Encode()))
	reqBU.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqBU = reqBU.WithContext(withTenant(reqBU.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recBU := httptest.NewRecorder()
	handleSetID(recBU, reqBU, store)
	if recBU.Code != http.StatusSeeOther {
		t.Fatalf("create bu status=%d", recBU.Code)
	}

	createSetID := url.Values{}
	createSetID.Set("action", "create_setid")
	createSetID.Set("setid", "B0001")
	createSetID.Set("name", "Default B")
	reqS := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(createSetID.Encode()))
	reqS.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqS = reqS.WithContext(withTenant(reqS.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recS := httptest.NewRecorder()
	handleSetID(recS, reqS, store)
	if recS.Code != http.StatusSeeOther {
		t.Fatalf("create setid status=%d", recS.Code)
	}

	save := url.Values{}
	save.Set("action", "save_mappings")
	save.Set("map_BU000", "SHARE")
	save.Set("map_BU001", "B0001")
	reqM := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(save.Encode()))
	reqM.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqM = reqM.WithContext(withTenant(reqM.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recM := httptest.NewRecorder()
	handleSetID(recM, reqM, store)
	if recM.Code != http.StatusSeeOther {
		t.Fatalf("save mappings status=%d", recM.Code)
	}
}

func TestHandleSetID_Post_ParseFormError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_MergeMsgBranches(t *testing.T) {
	reqEmpty := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader("%"))
	reqEmpty.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqEmpty = reqEmpty.WithContext(withTenant(reqEmpty.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recEmpty := httptest.NewRecorder()
	handleSetID(recEmpty, reqEmpty, partialSetIDStore{listSetErr: errors.New("")})
	if recEmpty.Code != http.StatusOK {
		t.Fatalf("status=%d", recEmpty.Code)
	}
	if body := recEmpty.Body.String(); !strings.Contains(body, "bad form") {
		t.Fatalf("unexpected body: %q", body)
	}

	reqBoom := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader("%"))
	reqBoom.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqBoom = reqBoom.WithContext(withTenant(reqBoom.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recBoom := httptest.NewRecorder()
	handleSetID(recBoom, reqBoom, partialSetIDStore{listSetErr: errors.New("boom")})
	if recBoom.Code != http.StatusOK {
		t.Fatalf("status=%d", recBoom.Code)
	}
	if body := recBoom.Body.String(); !strings.Contains(body, "bad form") || !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleSetID_Post_UnknownAction(t *testing.T) {
	form := url.Values{}
	form.Set("action", "nope")
	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_Post_MissingFields(t *testing.T) {
	store := newSetIDMemoryStore()

	form := url.Values{}
	form.Set("action", "create_setid")
	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	form2 := url.Values{}
	form2.Set("action", "create_bu")
	req2 := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2 = req2.WithContext(withTenant(req2.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec2 := httptest.NewRecorder()
	handleSetID(rec2, req2, store)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d", rec2.Code)
	}

	form3 := url.Values{}
	form3.Set("action", "save_mappings")
	req3 := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form3.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req3 = req3.WithContext(withTenant(req3.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec3 := httptest.NewRecorder()
	handleSetID(rec3, req3, store)
	if rec3.Code != http.StatusOK {
		t.Fatalf("status=%d", rec3.Code)
	}
}

func TestHandleSetID_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_EnsureBootstrapError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, errSetIDStore{err: errors.New("boom")})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestSetIDMemoryStore_Errors(t *testing.T) {
	s := newSetIDMemoryStore().(*setidMemoryStore)
	if err := s.CreateSetID(context.Background(), "t1", "", "n", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.CreateSetID(context.Background(), "t1", "SHARE", "n", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.CreateSetID(context.Background(), "t1", "A0001", "n", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.CreateSetID(context.Background(), "t1", "A0001", "n", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.CreateBusinessUnit(context.Background(), "t1", "BU001", "n", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.CreateBusinessUnit(context.Background(), "t1", "BU001", "n", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.CreateBusinessUnit(context.Background(), "t1", "", "n", "", ""); err == nil {
		t.Fatal("expected error")
	}
	sFresh := newSetIDMemoryStore().(*setidMemoryStore)
	if err := sFresh.PutMappings(context.Background(), "t1", "jobcatalog", map[string]string{"bu001": "a0001"}, "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestRenderSetIDPage_SkipsDisabledOptions(t *testing.T) {
	html := renderSetIDPage(
		[]SetID{{SetID: "SHARE", Name: "Shared", Status: "active"}, {SetID: "A0001", Name: "A", Status: "disabled"}},
		[]BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}},
		[]SetIDMappingRow{{BusinessUnitID: "BU000", SetID: "SHARE"}},
		Tenant{Name: "T"},
		"2026-01-07",
		"",
	)
	if !strings.Contains(html, "SHARE") {
		t.Fatalf("unexpected html: %q", html)
	}
}

func TestSetIDMemoryStore_ListSortsWithMultipleItems(t *testing.T) {
	s := newSetIDMemoryStore().(*setidMemoryStore)
	if err := s.EnsureBootstrap(context.Background(), "t1", "i1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.CreateSetID(context.Background(), "t1", "A0001", "A", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.CreateSetID(context.Background(), "t1", "B0001", "B", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.CreateBusinessUnit(context.Background(), "t1", "BU001", "BU 1", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.PutMappings(context.Background(), "t1", "jobcatalog", map[string]string{"BU000": "A0001", "BU001": "B0001"}, "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	setids, err := s.ListSetIDs(context.Background(), "t1")
	if err != nil || len(setids) < 2 {
		t.Fatalf("len=%d err=%v", len(setids), err)
	}
	if setids[0].SetID != "A0001" {
		t.Fatalf("unexpected first setid=%q", setids[0].SetID)
	}

	bus, err := s.ListBusinessUnits(context.Background(), "t1")
	if err != nil || len(bus) != 2 {
		t.Fatalf("len=%d err=%v", len(bus), err)
	}
	if bus[0].BusinessUnitID != "BU000" {
		t.Fatalf("unexpected first bu=%q", bus[0].BusinessUnitID)
	}

	mappings, err := s.ListMappings(context.Background(), "t1", "jobcatalog")
	if err != nil || len(mappings) != 2 {
		t.Fatalf("len=%d err=%v", len(mappings), err)
	}
	if mappings[0].BusinessUnitID != "BU000" {
		t.Fatalf("unexpected first mapping bu=%q", mappings[0].BusinessUnitID)
	}
}

func TestHandleSetID_Post_DefaultAction(t *testing.T) {
	form := url.Values{}
	form.Set("setid", "A0001")
	form.Set("name", "Default A")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_ListAndWriteErrors(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))

	recSet := httptest.NewRecorder()
	handleSetID(recSet, req, partialSetIDStore{listSetErr: errors.New("boom")})
	if recSet.Code != http.StatusOK {
		t.Fatalf("status=%d", recSet.Code)
	}

	recBU := httptest.NewRecorder()
	handleSetID(recBU, req, partialSetIDStore{listBUErr: errors.New("boom")})
	if recBU.Code != http.StatusOK {
		t.Fatalf("status=%d", recBU.Code)
	}

	recMap := httptest.NewRecorder()
	handleSetID(recMap, req, partialSetIDStore{listMapErr: errors.New("boom")})
	if recMap.Code != http.StatusOK {
		t.Fatalf("status=%d", recMap.Code)
	}
}

func TestHandleSetID_Post_WriteErrors(t *testing.T) {
	createSet := url.Values{}
	createSet.Set("action", "create_setid")
	createSet.Set("setid", "A0001")
	createSet.Set("name", "A")
	reqSet := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(createSet.Encode()))
	reqSet.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqSet = reqSet.WithContext(withTenant(reqSet.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recSet := httptest.NewRecorder()
	handleSetID(recSet, reqSet, partialSetIDStore{createSetErr: errors.New("boom")})
	if recSet.Code != http.StatusOK {
		t.Fatalf("status=%d", recSet.Code)
	}

	createBU := url.Values{}
	createBU.Set("action", "create_bu")
	createBU.Set("business_unit_id", "BU001")
	createBU.Set("name", "BU 1")
	reqBU := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(createBU.Encode()))
	reqBU.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqBU = reqBU.WithContext(withTenant(reqBU.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recBU := httptest.NewRecorder()
	handleSetID(recBU, reqBU, partialSetIDStore{createBUErr: errors.New("boom")})
	if recBU.Code != http.StatusOK {
		t.Fatalf("status=%d", recBU.Code)
	}

	save := url.Values{}
	save.Set("action", "save_mappings")
	save.Set("map_BU000", "SHARE")
	reqM := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(save.Encode()))
	reqM.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqM = reqM.WithContext(withTenant(reqM.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recM := httptest.NewRecorder()
	handleSetID(recM, reqM, partialSetIDStore{putMapErr: errors.New("boom")})
	if recM.Code != http.StatusOK {
		t.Fatalf("status=%d", recM.Code)
	}
}

func TestSetIDPGStore_WithTx_Errors(t *testing.T) {
	s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin fail")
	})}
	if err := s.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx := &stubTx{execErr: errors.New("set_config fail"), execErrAt: 1}
	s2 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	if err := s2.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx2 := &stubTx{commitErr: errors.New("commit fail")}
	s3 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx2, nil })}
	if err := s3.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDPGStore_ListAndWrite(t *testing.T) {
	tx := &stubTx{
		rows: &tableRows{rows: [][]any{
			{"SHARE", "Shared", "active"},
			{"A0001", "A", "active"},
		}},
	}
	s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}

	if got, err := s.ListSetIDs(context.Background(), "t1"); err != nil || len(got) != 2 {
		t.Fatalf("len=%d err=%v", len(got), err)
	}

	txQueryErr := &stubTx{queryErr: errors.New("query fail")}
	sQueryErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txQueryErr, nil })}
	if _, err := sQueryErr.ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txErrScan := &stubTx{rows: &scanErrRows{}}
	sErrScan := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txErrScan, nil })}
	if _, err := sErrScan.ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txErrRows := &stubTx{rows: &tableRows{rows: [][]any{}, err: errors.New("rows err")}}
	sErrRows := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txErrRows, nil })}
	if _, err := sErrRows.ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txCommitErr := &stubTx{commitErr: errors.New("commit fail"), rows: &tableRows{rows: [][]any{}}}
	sCommitErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txCommitErr, nil })}
	if _, err := sCommitErr.ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txBU := &stubTx{
		rows: &tableRows{rows: [][]any{
			{"BU000", "Default BU", "active"},
		}},
	}
	sBU := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txBU, nil })}
	if bus, err := sBU.ListBusinessUnits(context.Background(), "t1"); err != nil || len(bus) != 1 {
		t.Fatalf("len=%d err=%v", len(bus), err)
	}

	txBUQueryErr := &stubTx{queryErr: errors.New("query fail")}
	sBUQueryErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txBUQueryErr, nil })}
	if _, err := sBUQueryErr.ListBusinessUnits(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txBUScanErr := &stubTx{rows: &scanErrRows{}}
	sBUScanErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txBUScanErr, nil })}
	if _, err := sBUScanErr.ListBusinessUnits(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txM := &stubTx{
		rows: &tableRows{rows: [][]any{
			{"BU000", "SHARE"},
		}},
	}
	sM := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txM, nil })}
	if rows, err := sM.ListMappings(context.Background(), "t1", "jobcatalog"); err != nil || len(rows) != 1 {
		t.Fatalf("len=%d err=%v", len(rows), err)
	}

	txMQueryErr := &stubTx{queryErr: errors.New("query fail")}
	sMQueryErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txMQueryErr, nil })}
	if _, err := sMQueryErr.ListMappings(context.Background(), "t1", "jobcatalog"); err == nil {
		t.Fatal("expected error")
	}

	txMScanErr := &stubTx{rows: &scanErrRows{}}
	sMScanErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txMScanErr, nil })}
	if _, err := sMScanErr.ListMappings(context.Background(), "t1", "jobcatalog"); err == nil {
		t.Fatal("expected error")
	}

	tx2 := &stubTx{execErr: errors.New("exec fail"), execErrAt: 2}
	s2 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx2, nil })}
	if err := s2.CreateSetID(context.Background(), "t1", `A"1`, "Name", "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx3 := &stubTx{execErr: errors.New("exec fail"), execErrAt: 2}
	s3 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3, nil })}
	if err := s3.CreateBusinessUnit(context.Background(), "t1", "BU001", "BU", "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx4 := &stubTx{execErr: errors.New("exec fail"), execErrAt: 2}
	s4 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4, nil })}
	if err := s4.PutMappings(context.Background(), "t1", "jobcatalog", map[string]string{`BU"01`: `A\\1`}, "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx5 := &stubTx{}
	s5 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx5, nil })}
	if err := s5.PutMappings(context.Background(), "t1", "jobcatalog", map[string]string{}, "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	txOK := &stubTx{}
	sOK := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txOK, nil })}
	if err := sOK.PutMappings(context.Background(), "t1", "jobcatalog", map[string]string{"BU000": "SHARE"}, "r1", "p1"); err != nil {
		t.Fatalf("err=%v", err)
	}

	txOK2 := &stubTx{}
	sOK2 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txOK2, nil })}
	if err := sOK2.PutMappings(context.Background(), "t1", "jobcatalog", map[string]string{"BU000": "SHARE", "BU001": "A0001"}, "r1", "p1"); err != nil {
		t.Fatalf("err=%v", err)
	}
}
