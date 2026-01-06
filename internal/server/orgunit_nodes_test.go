package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestOrgUnitMemoryStore(t *testing.T) {
	s := newOrgUnitMemoryStore()
	s.now = func() time.Time { return time.Unix(123, 0).UTC() }

	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "Hello World", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, "Hello"); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len=%d", len(nodes))
	}
	if nodes[0].Name != "Hello" {
		t.Fatalf("name=%q", nodes[0].Name)
	}
	if nodes[0].CreatedAt != time.Unix(123, 0).UTC() {
		t.Fatalf("created_at=%s", nodes[0].CreatedAt)
	}
}

func TestOrgUnitMemoryStore_RenameNodeCurrent_Errors(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "", "B"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "nope", "B"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOrgUnitMemoryStore_MoveDisableNodeCurrent(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "nope", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "nope"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID); err != nil {
		t.Fatalf("err=%v", err)
	}

	nodes, err := s.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("len=%d", len(nodes))
	}
}

func TestHandleOrgNodes_MissingTenant(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
	handleOrgNodes(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_GET_HX(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "")

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=2026-01-06", nil)
	req.Header.Set("HX-Request", "true")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); body == "" || bytes.Contains(rec.Body.Bytes(), []byte("<!doctype html>")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "")

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "A") {
		t.Fatalf("unexpected body: %q", body)
	}
}

type errStore struct{}

func (errStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return nil, errBoom{}
}
func (errStore) CreateNodeCurrent(context.Context, string, string, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, errBoom{}
}
func (errStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return errBoom{}
}
func (errStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return errBoom{}
}
func (errStore) DisableNodeCurrent(context.Context, string, string, string) error { return errBoom{} }

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

type emptyErr struct{}

func (emptyErr) Error() string { return "" }

type emptyErrStore struct{}

func (emptyErrStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return nil, emptyErr{}
}
func (emptyErrStore) CreateNodeCurrent(context.Context, string, string, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, emptyErr{}
}
func (emptyErrStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return emptyErr{}
}
func (emptyErrStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return emptyErr{}
}
func (emptyErrStore) DisableNodeCurrent(context.Context, string, string, string) error {
	return emptyErr{}
}

func TestHandleOrgNodes_GET_StoreError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("boom")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_BadAsOf(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=bad", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("as_of 无效")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_POST_BadForm(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("%zz")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_POST_BadForm_MergesEmptyStoreError(t *testing.T) {
	body := bytes.NewBufferString("%zz")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, emptyErrStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "bad form") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_EmptyName(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", bytes.NewBufferString("name="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("name is required")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_POST_SuccessRedirect(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("name=A&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/org/nodes?as_of=2026-01-06" {
		t.Fatalf("location=%q", loc)
	}
}

type writeSpyStore struct {
	createCalled  int
	renameCalled  int
	moveCalled    int
	disableCalled int

	argsCreate  []string
	argsRename  []string
	argsMove    []string
	argsDisable []string

	err error
}

func (s *writeSpyStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return []OrgUnitNode{{ID: "c1", Name: "C"}}, nil
}
func (s *writeSpyStore) CreateNodeCurrent(_ context.Context, tenantID string, effectiveDate string, name string, parentID string) (OrgUnitNode, error) {
	s.createCalled++
	s.argsCreate = []string{tenantID, effectiveDate, name, parentID}
	if s.err != nil {
		return OrgUnitNode{}, s.err
	}
	return OrgUnitNode{ID: "u1", Name: name, CreatedAt: time.Unix(1, 0).UTC()}, nil
}
func (s *writeSpyStore) RenameNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, newName string) error {
	s.renameCalled++
	s.argsRename = []string{tenantID, effectiveDate, orgID, newName}
	return s.err
}
func (s *writeSpyStore) MoveNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, newParentID string) error {
	s.moveCalled++
	s.argsMove = []string{tenantID, effectiveDate, orgID, newParentID}
	return s.err
}
func (s *writeSpyStore) DisableNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string) error {
	s.disableCalled++
	s.argsDisable = []string{tenantID, effectiveDate, orgID}
	return s.err
}

type asOfSpyStore struct {
	gotAsOf string
}

func (s *asOfSpyStore) ListNodesCurrent(_ context.Context, _ string, asOf string) ([]OrgUnitNode, error) {
	s.gotAsOf = asOf
	return []OrgUnitNode{{ID: "c1", Name: "C"}}, nil
}
func (s *asOfSpyStore) CreateNodeCurrent(context.Context, string, string, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}
func (s *asOfSpyStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (s *asOfSpyStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (s *asOfSpyStore) DisableNodeCurrent(context.Context, string, string, string) error { return nil }

func TestHandleOrgNodes_POST_Rename_UsesStore(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_id=u1&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.renameCalled != 1 {
		t.Fatalf("renameCalled=%d", store.renameCalled)
	}
	if got := strings.Join(store.argsRename, "|"); got != "t1|2026-01-05|u1|New" {
		t.Fatalf("args=%q", got)
	}
	if loc := rec.Header().Get("Location"); loc != "/org/nodes?as_of=2026-01-05" {
		t.Fatalf("location=%q", loc)
	}
}

func TestHandleOrgNodes_POST_Rename_DefaultsEffectiveDateToAsOf(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_id=u1&new_name=New")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.Join(store.argsRename, "|"); got != "t1|2026-01-06|u1|New" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_Rename_Error_ShowsErrorAndNodes(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("action=rename&org_id=u1&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") || !strings.Contains(bodyOut, "C") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Create_BadEffectiveDate(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("name=A&effective_date=bad")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.createCalled != 0 {
		t.Fatalf("createCalled=%d", store.createCalled)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "effective_date 无效") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Create_Error_ShowsError(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("name=A&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Move_UsesStore(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=move&org_id=u1&new_parent_id=p1&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.moveCalled != 1 {
		t.Fatalf("moveCalled=%d", store.moveCalled)
	}
	if got := strings.Join(store.argsMove, "|"); got != "t1|2026-01-05|u1|p1" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_Disable_UsesStore(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=disable&org_id=u1&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.disableCalled != 1 {
		t.Fatalf("disableCalled=%d", store.disableCalled)
	}
	if got := strings.Join(store.argsDisable, "|"); got != "t1|2026-01-05|u1" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_Disable_BadEffectiveDate(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=disable&org_id=u1&effective_date=bad")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.disableCalled != 0 {
		t.Fatalf("disableCalled=%d", store.disableCalled)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("effective_date 无效")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Move_Error_ShowsErrorAndNodes(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("action=move&org_id=u1&new_parent_id=p1&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") || !strings.Contains(bodyOut, "C") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Disable_Error_ShowsErrorAndNodes(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("action=disable&org_id=u1&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") || !strings.Contains(bodyOut, "C") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_MergesErrorHints(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("name=")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=bad", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "name is required；as_of 无效") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_DefaultsEffectiveDateToAsOf(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("name=A")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.Join(store.argsCreate, "|"); got != "t1|2026-01-06|A|" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_Rename_MissingOrgID(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_id=&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.renameCalled != 0 {
		t.Fatalf("renameCalled=%d", store.renameCalled)
	}
}

func TestHandleOrgNodes_POST_Rename_MissingNewName(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_id=u1&new_name=&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.renameCalled != 0 {
		t.Fatalf("renameCalled=%d", store.renameCalled)
	}
}

func TestHandleOrgNodes_GET_DefaultAsOf_UsesToday(t *testing.T) {
	store := &asOfSpyStore{}
	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.gotAsOf != time.Now().UTC().Format("2006-01-02") {
		t.Fatalf("asOf=%q", store.gotAsOf)
	}
}

func TestHandleOrgNodes_MethodNotAllowed(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPut, "/org/nodes", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRenderOrgNodes(t *testing.T) {
	out := renderOrgNodes(nil, Tenant{Name: "T"}, "", "2026-01-06")
	if out == "" {
		t.Fatal("expected output")
	}
	out2 := renderOrgNodes([]OrgUnitNode{{ID: "1", Name: "N"}}, Tenant{Name: "T"}, "err", "2026-01-06")
	if out2 == "" {
		t.Fatal("expected output")
	}
}

func TestOrgUnitPGStore_ListNodesCurrent_AndCreateCurrent(t *testing.T) {
	pool := &fakeBeginner{}
	store := &orgUnitPGStore{pool: pool}

	nodes, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ID != "n1" {
		t.Fatalf("nodes=%+v", nodes)
	}

	createdCurrent, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "Current", "")
	if err != nil {
		t.Fatal(err)
	}
	if createdCurrent.ID != "u1" || createdCurrent.Name != "Current" || !createdCurrent.CreatedAt.Equal(time.Unix(789, 0).UTC()) {
		t.Fatalf("created=%+v", createdCurrent)
	}

	createdWithParent, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "CurrentWithParent", "p1")
	if err != nil {
		t.Fatal(err)
	}
	if createdWithParent.ID == "" {
		t.Fatal("expected id")
	}
}

func TestOrgUnitPGStore_ListNodesCurrent_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{scanErr: errors.New("scan")}}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows_err", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{err: errors.New("rows")}}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit")}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_CreateNodeCurrent_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("effective_date_required", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("org_id_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event_id_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"u1"}}
			tx.row2 = &stubRow{err: errors.New("row2")}
			return tx, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit_exec", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
			tx.row = &stubRow{vals: []any{"u1"}}
			tx.row2 = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("transaction_time_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"u1"}}
			tx.row2 = &stubRow{vals: []any{"e1"}}
			tx.row3 = &stubRow{err: errors.New("row3")}
			return tx, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"u1"}}
			tx.row2 = &stubRow{vals: []any{"e1"}}
			tx.row3 = &stubRow{vals: []any{time.Unix(1, 0).UTC()}}
			return tx, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_RenameMoveDisableCurrent(t *testing.T) {
	t.Run("rename_success", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", "New"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("rename_errors", func(t *testing.T) {
		t.Run("begin", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin")
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("set_config", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec")}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("effective_date_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "", "u1", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("org_id_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("new_name_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("event_id_scan", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{rowErr: errors.New("row")}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("submit_exec", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("commit", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{commitErr: errors.New("commit")}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
	})

	t.Run("move_success", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", ""); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("move_success_with_parent", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", "p1"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("move_errors", func(t *testing.T) {
		t.Run("begin", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin")
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("set_config", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec")}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("effective_date_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "", "u1", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("org_id_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("event_id_scan", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{rowErr: errors.New("row")}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("submit_exec", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("commit", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{commitErr: errors.New("commit")}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "u1", ""); err == nil {
				t.Fatal("expected error")
			}
		})
	})

	t.Run("disable_success", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "u1"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("disable_errors", func(t *testing.T) {
		t.Run("begin", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin")
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "u1"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("set_config", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec")}, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "u1"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("effective_date_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "", "u1"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("org_id_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("event_id_scan", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{rowErr: errors.New("row")}, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "u1"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("submit_exec", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "u1"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("commit", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{commitErr: errors.New("commit")}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "u1"); err == nil {
				t.Fatal("expected error")
			}
		})
	})
}
