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
func (s errSetIDStore) ListSetIDBindings(context.Context, string, string) ([]SetIDBindingRow, error) {
	return nil, s.err
}
func (s errSetIDStore) BindSetID(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s errSetIDStore) CreateGlobalSetID(context.Context, string, string, string, string) error {
	return s.err
}

type partialSetIDStore struct {
	listSetErr   error
	createSetErr error
	listBindErr  error
	bindErr      error
}

func (s partialSetIDStore) EnsureBootstrap(context.Context, string, string) error { return nil }
func (s partialSetIDStore) ListSetIDs(context.Context, string) ([]SetID, error) {
	if s.listSetErr != nil {
		return nil, s.listSetErr
	}
	return []SetID{{SetID: "DEFLT", Name: "Default", Status: "active"}}, nil
}
func (s partialSetIDStore) CreateSetID(context.Context, string, string, string, string, string) error {
	return s.createSetErr
}
func (s partialSetIDStore) ListSetIDBindings(context.Context, string, string) ([]SetIDBindingRow, error) {
	if s.listBindErr != nil {
		return nil, s.listBindErr
	}
	return []SetIDBindingRow{{OrgUnitID: "org1", SetID: "DEFLT", ValidFrom: "2026-01-01"}}, nil
}
func (s partialSetIDStore) BindSetID(context.Context, string, string, string, string, string, string) error {
	return s.bindErr
}
func (s partialSetIDStore) CreateGlobalSetID(context.Context, string, string, string, string) error {
	return nil
}

type errOrgUnitStore struct{ err error }

func (s errOrgUnitStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return nil, s.err
}
func (s errOrgUnitStore) CreateNodeCurrent(context.Context, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, s.err
}
func (s errOrgUnitStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return s.err
}
func (s errOrgUnitStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return s.err
}
func (s errOrgUnitStore) DisableNodeCurrent(context.Context, string, string, string) error {
	return s.err
}
func (s errOrgUnitStore) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return s.err
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

func newTestOrgStore() OrgUnitStore {
	return newOrgUnitMemoryStore()
}

func TestHandleSetID_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_MissingAsOfRedirects(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()

	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
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

	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_Get(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
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
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_Post_BindSetID(t *testing.T) {
	store := newSetIDMemoryStore()

	createSetID := url.Values{}
	createSetID.Set("action", "create_setid")
	createSetID.Set("setid", "B0001")
	createSetID.Set("name", "Default B")
	reqS := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(createSetID.Encode()))
	reqS.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqS = reqS.WithContext(withTenant(reqS.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recS := httptest.NewRecorder()
	handleSetID(recS, reqS, store, newTestOrgStore())
	if recS.Code != http.StatusSeeOther {
		t.Fatalf("create setid status=%d", recS.Code)
	}

	bind := url.Values{}
	bind.Set("action", "bind_setid")
	bind.Set("org_unit_id", "org1")
	bind.Set("setid", "B0001")
	bind.Set("effective_date", "2026-01-01")
	reqB := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(bind.Encode()))
	reqB.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqB = reqB.WithContext(withTenant(reqB.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recB := httptest.NewRecorder()
	handleSetID(recB, reqB, store, newTestOrgStore())
	if recB.Code != http.StatusSeeOther {
		t.Fatalf("bind setid status=%d", recB.Code)
	}
}

func TestHandleSetID_Post_ParseFormError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_MergeMsgBranches(t *testing.T) {
	newBadFormReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader("%"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	}

	recSet := httptest.NewRecorder()
	handleSetID(recSet, newBadFormReq(), partialSetIDStore{listSetErr: errors.New("boom")}, newTestOrgStore())
	if recSet.Code != http.StatusOK {
		t.Fatalf("status=%d", recSet.Code)
	}
	if body := recSet.Body.String(); !strings.Contains(body, "bad form") || !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}

	recSetEmpty := httptest.NewRecorder()
	handleSetID(recSetEmpty, newBadFormReq(), partialSetIDStore{listSetErr: emptyErr{}}, newTestOrgStore())
	if recSetEmpty.Code != http.StatusOK {
		t.Fatalf("status=%d", recSetEmpty.Code)
	}
	if body := recSetEmpty.Body.String(); !strings.Contains(body, "bad form") {
		t.Fatalf("unexpected body: %q", body)
	}

	recBind := httptest.NewRecorder()
	handleSetID(recBind, newBadFormReq(), partialSetIDStore{listBindErr: errors.New("bind boom")}, newTestOrgStore())
	if recBind.Code != http.StatusOK {
		t.Fatalf("status=%d", recBind.Code)
	}
	if body := recBind.Body.String(); !strings.Contains(body, "bad form") || !strings.Contains(body, "bind boom") {
		t.Fatalf("unexpected body: %q", body)
	}

	recOrg := httptest.NewRecorder()
	handleSetID(recOrg, newBadFormReq(), partialSetIDStore{}, errOrgUnitStore{err: errors.New("org boom")})
	if recOrg.Code != http.StatusOK {
		t.Fatalf("status=%d", recOrg.Code)
	}
	if body := recOrg.Body.String(); !strings.Contains(body, "bad form") || !strings.Contains(body, "org boom") {
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
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
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
	handleSetID(rec, req, store, newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	form2 := url.Values{}
	form2.Set("action", "bind_setid")
	req2 := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2 = req2.WithContext(withTenant(req2.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec2 := httptest.NewRecorder()
	handleSetID(rec2, req2, store, newTestOrgStore())
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d", rec2.Code)
	}
}

func TestHandleSetID_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_EnsureBootstrapError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, errSetIDStore{err: errors.New("boom")}, newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
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
	handleSetID(recSet, reqSet, partialSetIDStore{createSetErr: errors.New("boom")}, newTestOrgStore())
	if recSet.Code != http.StatusOK {
		t.Fatalf("status=%d", recSet.Code)
	}

	bind := url.Values{}
	bind.Set("action", "bind_setid")
	bind.Set("org_unit_id", "org1")
	bind.Set("setid", "DEFLT")
	reqBind := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(bind.Encode()))
	reqBind.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqBind = reqBind.WithContext(withTenant(reqBind.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recBind := httptest.NewRecorder()
	handleSetID(recBind, reqBind, partialSetIDStore{bindErr: errors.New("boom")}, newTestOrgStore())
	if recBind.Code != http.StatusOK {
		t.Fatalf("status=%d", recBind.Code)
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
	if err := s.BindSetID(context.Background(), "t1", "", "2026-01-01", "A0001", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.BindSetID(context.Background(), "t1", "org1", "2026-01-01", "", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.BindSetID(context.Background(), "t1", "org1", "2026-01-01", "NOPE", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.BindSetID(context.Background(), "t1", "org1", "2026-01-01", "A0001", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestSetIDMemoryStore_ListSortsWithMultipleItems(t *testing.T) {
	s := newSetIDMemoryStore().(*setidMemoryStore)
	if err := s.EnsureBootstrap(context.Background(), "t1", "i1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.CreateSetID(context.Background(), "t1", "B0001", "B", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.CreateSetID(context.Background(), "t1", "A0001", "A", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.BindSetID(context.Background(), "t1", "org-b", "2026-01-01", "B0001", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.BindSetID(context.Background(), "t1", "org-a", "2026-01-01", "A0001", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	setids, err := s.ListSetIDs(context.Background(), "t1")
	if err != nil || len(setids) < 2 {
		t.Fatalf("len=%d err=%v", len(setids), err)
	}
	if setids[0].SetID != "A0001" {
		t.Fatalf("unexpected first setid=%q", setids[0].SetID)
	}

	bindings, err := s.ListSetIDBindings(context.Background(), "t1", "2026-01-01")
	if err != nil || len(bindings) < 2 {
		t.Fatalf("len=%d err=%v", len(bindings), err)
	}
	if bindings[0].OrgUnitID != "org-a" {
		t.Fatalf("unexpected first binding org=%q", bindings[0].OrgUnitID)
	}
}

func TestSetIDMemoryStore_CreateGlobalSetID(t *testing.T) {
	s := newSetIDMemoryStore().(*setidMemoryStore)
	if err := s.CreateGlobalSetID(context.Background(), "", "", "", "saas"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.CreateGlobalSetID(context.Background(), "Shared", "", "", "tenant"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.CreateGlobalSetID(context.Background(), "Shared", "", "", "saas"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if s.globalSetIDName != "Shared" {
		t.Fatalf("name=%q", s.globalSetIDName)
	}
}

func TestRenderSetIDPage_SkipsDisabledOptions(t *testing.T) {
	html := renderSetIDPage(
		[]SetID{{SetID: "SHARE", Name: "Shared", Status: "active"}, {SetID: "A0001", Name: "A", Status: "disabled"}},
		[]SetIDBindingRow{{OrgUnitID: "org1", SetID: "SHARE", ValidFrom: "2026-01-01"}},
		[]OrgUnitNode{{ID: "org1", Name: "BU 1", IsBusinessUnit: true}, {ID: "org2", Name: "BU 0", IsBusinessUnit: true}},
		Tenant{Name: "T"},
		"2026-01-07",
		"",
	)
	if !strings.Contains(html, "option value=\"SHARE\"") {
		t.Fatalf("unexpected html: %q", html)
	}
	if strings.Contains(html, "option value=\"A0001\"") {
		t.Fatalf("unexpected html: %q", html)
	}
}

func TestRenderSetIDPage_NoBusinessUnits(t *testing.T) {
	html := renderSetIDPage(
		[]SetID{{SetID: "DEFLT", Name: "Default", Status: "active"}},
		nil,
		[]OrgUnitNode{{ID: "org1", Name: "Org 1", IsBusinessUnit: false}},
		Tenant{Name: "T"},
		"2026-01-07",
		"",
	)
	if !strings.Contains(html, "(no business units)") {
		t.Fatalf("unexpected html: %q", html)
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
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_OrgStoreMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "orgunit store missing") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_BindInvalidEffectiveDate(t *testing.T) {
	form := url.Values{}
	form.Set("action", "bind_setid")
	form.Set("org_unit_id", "org1")
	form.Set("setid", "DEFLT")
	form.Set("effective_date", "bad")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "effective_date 无效") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
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

func TestSetIDPGStore_EnsureBootstrap_GlobalShare(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{row: &stubRow{vals: []any{"gt1"}}}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("global begin error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return nil, errors.New("begin fail")
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global tenant id error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{rowErr: errors.New("row fail")}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global set_config current_tenant error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{
				row:       &stubRow{vals: []any{"gt1"}},
				execErr:   errors.New("exec fail"),
				execErrAt: 1,
			}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global set_config actor_scope error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{
				row:       &stubRow{vals: []any{"gt1"}},
				execErr:   errors.New("exec fail"),
				execErrAt: 2,
			}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global set_config allow_share_read error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{
				row:       &stubRow{vals: []any{"gt1"}},
				execErr:   errors.New("exec fail"),
				execErrAt: 3,
			}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global submit error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{
				row:       &stubRow{vals: []any{"gt1"}},
				execErr:   errors.New("exec fail"),
				execErrAt: 4,
			}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global commit error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{
				row:       &stubRow{vals: []any{"gt1"}},
				commitErr: errors.New("commit fail"),
			}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSetIDPGStore_ListSetIDs(t *testing.T) {
	txTenant := &stubTx{
		rows: &tableRows{rows: [][]any{
			{"A0001", "A", "active"},
		}},
	}
	txGlobal := &stubTx{
		row: &stubRow{vals: []any{"gt1"}},
		rows: &tableRows{rows: [][]any{
			{"SHARE", "Shared", "active"},
		}},
	}
	var calls int
	s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		calls++
		if calls == 1 {
			return txTenant, nil
		}
		return txGlobal, nil
	})}

	if got, err := s.ListSetIDs(context.Background(), "t1"); err != nil || len(got) != 2 {
		t.Fatalf("len=%d err=%v", len(got), err)
	}

	txQueryErr := &stubTx{queryErr: errors.New("query fail")}
	sQueryErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txQueryErr, nil })}
	if _, err := sQueryErr.ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txGlobalQueryErr := &stubTx{
		row:      &stubRow{vals: []any{"gt1"}},
		queryErr: errors.New("global query fail"),
	}
	calls = 0
	sGlobalQueryErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		calls++
		if calls == 1 {
			return &stubTx{rows: &tableRows{rows: [][]any{{"A0001", "A", "active"}}}}, nil
		}
		return txGlobalQueryErr, nil
	})}
	if _, err := sGlobalQueryErr.ListSetIDs(context.Background(), "t1"); err == nil {
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
}

func TestSetIDPGStore_ListSetIDs_GlobalErrors(t *testing.T) {
	tenantTx := func() *stubTx {
		return &stubTx{rows: &tableRows{rows: [][]any{{"A0001", "A", "active"}}}}
	}
	makeStore := func(globalTx pgx.Tx, globalErr error) *setidPGStore {
		var calls int
		return &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return tenantTx(), nil
			}
			if globalErr != nil {
				return nil, globalErr
			}
			return globalTx, nil
		})}
	}

	if _, err := makeStore(nil, errors.New("begin fail")).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{rowErr: errors.New("row fail")}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{
		row:       &stubRow{vals: []any{"gt1"}},
		execErr:   errors.New("exec fail"),
		execErrAt: 1,
	}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{
		row:       &stubRow{vals: []any{"gt1"}},
		execErr:   errors.New("exec fail"),
		execErrAt: 2,
	}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{
		row:  &stubRow{vals: []any{"gt1"}},
		rows: &scanErrRows{},
	}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{
		row:  &stubRow{vals: []any{"gt1"}},
		rows: &tableRows{rows: [][]any{}, err: errors.New("rows err")},
	}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{
		row:       &stubRow{vals: []any{"gt1"}},
		rows:      &tableRows{rows: [][]any{}},
		commitErr: errors.New("commit fail"),
	}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDPGStore_ListSetIDBindings(t *testing.T) {
	tx := &stubTx{
		rows: &tableRows{rows: [][]any{
			{"org1", "SHARE", "2026-01-01", ""},
		}},
	}
	s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}

	rows, err := s.ListSetIDBindings(context.Background(), "t1", "2026-01-01")
	if err != nil || len(rows) != 1 {
		t.Fatalf("len=%d err=%v", len(rows), err)
	}
	if rows[0].OrgUnitID != "org1" {
		t.Fatalf("unexpected org=%q", rows[0].OrgUnitID)
	}

	txQueryErr := &stubTx{queryErr: errors.New("query fail")}
	sQueryErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txQueryErr, nil })}
	if _, err := sQueryErr.ListSetIDBindings(context.Background(), "t1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	txErrScan := &stubTx{rows: &scanErrRows{}}
	sErrScan := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txErrScan, nil })}
	if _, err := sErrScan.ListSetIDBindings(context.Background(), "t1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	txErrRows := &stubTx{rows: &tableRows{rows: [][]any{}, err: errors.New("rows err")}}
	sErrRows := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txErrRows, nil })}
	if _, err := sErrRows.ListSetIDBindings(context.Background(), "t1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	txCommitErr := &stubTx{commitErr: errors.New("commit fail"), rows: &tableRows{rows: [][]any{}}}
	sCommitErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txCommitErr, nil })}
	if _, err := sCommitErr.ListSetIDBindings(context.Background(), "t1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDPGStore_CreateSetID_Errors(t *testing.T) {
	tx1 := &stubTx{execErr: errors.New("exec fail"), execErrAt: 1}
	s1 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx1, nil })}
	if err := s1.CreateSetID(context.Background(), "t1", "A0001", "A", "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx2 := &stubTx{execErr: errors.New("exec fail"), execErrAt: 2}
	s2 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx2, nil })}
	if err := s2.CreateSetID(context.Background(), "t1", "A0001", "A", "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDPGStore_BindSetID_Errors(t *testing.T) {
	tx2 := &stubTx{execErr: errors.New("exec fail"), execErrAt: 2}
	s2 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx2, nil })}
	if err := s2.BindSetID(context.Background(), "t1", "org1", "2026-01-01", "SHARE", "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx3 := &stubTx{execErr: errors.New("exec fail"), execErrAt: 3}
	s3 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3, nil })}
	if err := s3.BindSetID(context.Background(), "t1", "org1", "2026-01-01", "SHARE", "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDPGStore_CreateGlobalSetID(t *testing.T) {
	sBeginErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin fail")
	})}
	if err := sBeginErr.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txRowErr := &stubTx{rowErr: errors.New("row fail")}
	sRowErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txRowErr, nil })}
	if err := sRowErr.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txExecErr := &stubTx{row: &stubRow{vals: []any{"gt1"}}, execErr: errors.New("exec fail"), execErrAt: 1}
	sExecErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txExecErr, nil })}
	if err := sExecErr.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txExecErr2 := &stubTx{row: &stubRow{vals: []any{"gt1"}}, execErr: errors.New("exec fail"), execErrAt: 2}
	sExecErr2 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txExecErr2, nil })}
	if err := sExecErr2.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txExecErr3 := &stubTx{row: &stubRow{vals: []any{"gt1"}}, execErr: errors.New("exec fail"), execErrAt: 3}
	sExecErr3 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txExecErr3, nil })}
	if err := sExecErr3.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txCommitErr := &stubTx{row: &stubRow{vals: []any{"gt1"}}, commitErr: errors.New("commit fail")}
	sCommitErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txCommitErr, nil })}
	if err := sCommitErr.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txOK := &stubTx{row: &stubRow{vals: []any{"gt1"}}}
	sOK := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txOK, nil })}
	if err := sOK.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err != nil {
		t.Fatalf("err=%v", err)
	}
}
