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
	"github.com/jackc/pgx/v5/pgconn"
)

func TestOrgUnitMemoryStore(t *testing.T) {
	s := newOrgUnitMemoryStore()
	s.now = func() time.Time { return time.Unix(123, 0).UTC() }

	created, err := s.CreateNode(context.Background(), "t1", "Hello World")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenameNodeV4(context.Background(), "t1", "2026-01-06", created.ID, "Hello"); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.ListNodes(context.Background(), "t1")
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

func TestOrgUnitMemoryStore_RenameNodeV4_Errors(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNode(context.Background(), "t1", "A")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.RenameNodeV4(context.Background(), "t1", "2026-01-06", "", "B"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.RenameNodeV4(context.Background(), "t1", "2026-01-06", created.ID, ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.RenameNodeV4(context.Background(), "t1", "2026-01-06", "nope", "B"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOrgUnitMemoryStore_MoveDisableNodeV4(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNode(context.Background(), "t1", "A")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.MoveNodeV4(context.Background(), "t1", "2026-01-06", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.MoveNodeV4(context.Background(), "t1", "2026-01-06", "nope", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.MoveNodeV4(context.Background(), "t1", "2026-01-06", created.ID, ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	if err := s.DisableNodeV4(context.Background(), "t1", "2026-01-06", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableNodeV4(context.Background(), "t1", "2026-01-06", "nope"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableNodeV4(context.Background(), "t1", "2026-01-06", created.ID); err != nil {
		t.Fatalf("err=%v", err)
	}

	nodes, err := s.ListNodes(context.Background(), "t1")
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
	_, _ = store.CreateNode(context.Background(), "t1", "A")

	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
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

func TestHandleOrgNodes_GET_ReadLegacy_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNode(context.Background(), "t1", "A")

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=legacy", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("Read: <code>legacy</code>")) || !bytes.Contains([]byte(body), []byte("A")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_ReadLegacy_StoreError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=legacy", nil)
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

func TestHandleOrgNodes_POST_BadForm(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("%zz")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_POST_EmptyName(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes", bytes.NewBufferString("name="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

type errStore struct{}

func (errStore) ListNodes(context.Context, string) ([]OrgUnitNode, error) { return nil, errBoom{} }
func (errStore) CreateNode(context.Context, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, errBoom{}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

func TestHandleOrgNodes_GET_StoreError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_POST_CreateError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=legacy", bytes.NewBufferString("name=A"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

func TestHandleOrgNodes_POST_SuccessRedirect(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", bytes.NewBufferString("name=A"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/org/nodes?read=v4&as_of=2026-01-06" {
		t.Fatalf("location=%q", loc)
	}
}

func TestHandleOrgNodes_POST_SuccessRedirect_PreservesQuery(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=legacy&foo=bar", bytes.NewBufferString("name=A"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/org/nodes?read=legacy&foo=bar" {
		t.Fatalf("location=%q", loc)
	}
}

type v4WriteSpyStore struct {
	v4Called      int
	args          []string
	renameCalled  int
	renameArgs    []string
	moveCalled    int
	moveArgs      []string
	disableCalled int
	disableArgs   []string
	err           error
	v4Nodes       []OrgUnitNode
}

func (s *v4WriteSpyStore) ListNodes(context.Context, string) ([]OrgUnitNode, error) { return nil, nil }
func (s *v4WriteSpyStore) CreateNode(context.Context, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}
func (s *v4WriteSpyStore) ListNodesV4(context.Context, string, string) ([]OrgUnitNode, error) {
	return s.v4Nodes, nil
}
func (s *v4WriteSpyStore) CreateNodeV4(_ context.Context, tenantID string, effectiveDate string, name string, parentID string) (OrgUnitNode, error) {
	s.v4Called++
	s.args = []string{tenantID, effectiveDate, name, parentID}
	if s.err != nil {
		return OrgUnitNode{}, s.err
	}
	return OrgUnitNode{ID: "u1", Name: name, CreatedAt: time.Unix(1, 0).UTC()}, nil
}

func (s *v4WriteSpyStore) RenameNodeV4(_ context.Context, tenantID string, effectiveDate string, orgID string, newName string) error {
	s.renameCalled++
	s.renameArgs = []string{tenantID, effectiveDate, orgID, newName}
	if s.err != nil {
		return s.err
	}
	return nil
}

func (s *v4WriteSpyStore) MoveNodeV4(_ context.Context, tenantID string, effectiveDate string, orgID string, newParentID string) error {
	s.moveCalled++
	s.moveArgs = []string{tenantID, effectiveDate, orgID, newParentID}
	if s.err != nil {
		return s.err
	}
	return nil
}

func (s *v4WriteSpyStore) DisableNodeV4(_ context.Context, tenantID string, effectiveDate string, orgID string) error {
	s.disableCalled++
	s.disableArgs = []string{tenantID, effectiveDate, orgID}
	if s.err != nil {
		return s.err
	}
	return nil
}

type v4ReadOnlyStore struct {
	nodes []OrgUnitNode
}

func (s v4ReadOnlyStore) ListNodes(context.Context, string) ([]OrgUnitNode, error) { return nil, nil }
func (s v4ReadOnlyStore) CreateNode(context.Context, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}
func (s v4ReadOnlyStore) ListNodesV4(context.Context, string, string) ([]OrgUnitNode, error) {
	return s.nodes, nil
}

type legacyOnlyStore struct {
	inner *orgUnitMemoryStore
}

func (s legacyOnlyStore) ListNodes(ctx context.Context, tenantID string) ([]OrgUnitNode, error) {
	return s.inner.ListNodes(ctx, tenantID)
}
func (s legacyOnlyStore) CreateNode(ctx context.Context, tenantID string, name string) (OrgUnitNode, error) {
	return s.inner.CreateNode(ctx, tenantID, name)
}

func TestHandleOrgNodes_POST_ReadV4_UsesV4Writer(t *testing.T) {
	store := &v4WriteSpyStore{}
	body := bytes.NewBufferString("name=Hello&effective_date=2026-01-05&parent_id=u0")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.v4Called != 1 {
		t.Fatalf("v4Called=%d", store.v4Called)
	}
	if store.renameCalled != 0 {
		t.Fatalf("renameCalled=%d", store.renameCalled)
	}
	if store.moveCalled != 0 {
		t.Fatalf("moveCalled=%d", store.moveCalled)
	}
	if store.disableCalled != 0 {
		t.Fatalf("disableCalled=%d", store.disableCalled)
	}
	if got := strings.Join(store.args, "|"); got != "t1|2026-01-05|Hello|u0" {
		t.Fatalf("args=%q", got)
	}
	if loc := rec.Header().Get("Location"); loc != "/org/nodes?read=v4&as_of=2026-01-05" {
		t.Fatalf("location=%q", loc)
	}
}

func TestHandleOrgNodes_POST_ReadV4_DefaultsEffectiveDateToAsOf(t *testing.T) {
	store := &v4WriteSpyStore{}
	body := bytes.NewBufferString("name=Hello")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.Join(store.args, "|"); got != "t1|2026-01-06|Hello|" {
		t.Fatalf("args=%q", got)
	}
}

type v4CreateOnlyStore struct{}

func (v4CreateOnlyStore) ListNodes(context.Context, string) ([]OrgUnitNode, error) { return nil, nil }
func (v4CreateOnlyStore) CreateNode(context.Context, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}
func (v4CreateOnlyStore) ListNodesV4(context.Context, string, string) ([]OrgUnitNode, error) {
	return []OrgUnitNode{{ID: "v1", Name: "V"}}, nil
}
func (v4CreateOnlyStore) CreateNodeV4(context.Context, string, string, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}

func TestHandleOrgNodes_POST_ReadV4_Rename_UsesRenamer(t *testing.T) {
	store := &v4WriteSpyStore{}
	body := bytes.NewBufferString("action=rename&org_id=u1&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
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
	if got := strings.Join(store.renameArgs, "|"); got != "t1|2026-01-05|u1|New" {
		t.Fatalf("args=%q", got)
	}
	if loc := rec.Header().Get("Location"); loc != "/org/nodes?read=v4&as_of=2026-01-05" {
		t.Fatalf("location=%q", loc)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Rename_DefaultsEffectiveDateToAsOf(t *testing.T) {
	store := &v4WriteSpyStore{}
	body := bytes.NewBufferString("action=rename&org_id=u1&new_name=New")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.Join(store.renameArgs, "|"); got != "t1|2026-01-06|u1|New" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Rename_BadEffectiveDate(t *testing.T) {
	store := &v4WriteSpyStore{v4Nodes: []OrgUnitNode{{ID: "v1", Name: "V"}}}
	body := bytes.NewBufferString("action=rename&org_id=u1&new_name=New&effective_date=bad")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
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
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("effective_date 无效")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Rename_Error(t *testing.T) {
	store := &v4WriteSpyStore{
		err:     errors.New("boom"),
		v4Nodes: []OrgUnitNode{{ID: "v1", Name: "V"}},
	}
	body := bytes.NewBufferString("action=rename&org_id=u1&new_name=New&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.renameCalled != 1 {
		t.Fatalf("renameCalled=%d", store.renameCalled)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("boom")) || !bytes.Contains([]byte(bodyOut), []byte("V")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Rename_MissingOrgID(t *testing.T) {
	store := &v4WriteSpyStore{v4Nodes: []OrgUnitNode{{ID: "v1", Name: "V"}}}
	body := bytes.NewBufferString("action=rename&org_id=&new_name=New&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
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
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("org_id is required")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Rename_MissingNewName(t *testing.T) {
	store := &v4WriteSpyStore{v4Nodes: []OrgUnitNode{{ID: "v1", Name: "V"}}}
	body := bytes.NewBufferString("action=rename&org_id=u1&new_name=&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
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
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("new_name is required")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Rename_MissingRenamer(t *testing.T) {
	store := v4CreateOnlyStore{}
	body := bytes.NewBufferString("action=rename&org_id=u1&new_name=New&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("v4 renamer 未配置")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadLegacy_RenameRejected(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNode(context.Background(), "t1", "A")
	body := bytes.NewBufferString("action=rename&org_id=mem-a&new_name=B&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=legacy&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("rename 仅支持 v4 模式")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Move_UsesMover(t *testing.T) {
	store := &v4WriteSpyStore{}
	body := bytes.NewBufferString("action=move&org_id=u1&new_parent_id=u0&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
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
	if got := strings.Join(store.moveArgs, "|"); got != "t1|2026-01-05|u1|u0" {
		t.Fatalf("args=%q", got)
	}
	if loc := rec.Header().Get("Location"); loc != "/org/nodes?read=v4&as_of=2026-01-05" {
		t.Fatalf("location=%q", loc)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Move_DefaultsEffectiveDateToAsOf(t *testing.T) {
	store := &v4WriteSpyStore{}
	body := bytes.NewBufferString("action=move&org_id=u1&new_parent_id=")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.Join(store.moveArgs, "|"); got != "t1|2026-01-06|u1|" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Move_BadEffectiveDate(t *testing.T) {
	store := &v4WriteSpyStore{v4Nodes: []OrgUnitNode{{ID: "v1", Name: "V"}}}
	body := bytes.NewBufferString("action=move&org_id=u1&effective_date=bad")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.moveCalled != 0 {
		t.Fatalf("moveCalled=%d", store.moveCalled)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("effective_date 无效")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Move_Error(t *testing.T) {
	store := &v4WriteSpyStore{
		err:     errors.New("boom"),
		v4Nodes: []OrgUnitNode{{ID: "v1", Name: "V"}},
	}
	body := bytes.NewBufferString("action=move&org_id=u1&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.moveCalled != 1 {
		t.Fatalf("moveCalled=%d", store.moveCalled)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("boom")) || !bytes.Contains([]byte(bodyOut), []byte("V")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Move_MissingMover(t *testing.T) {
	store := v4CreateOnlyStore{}
	body := bytes.NewBufferString("action=move&org_id=u1&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("v4 mover 未配置")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadLegacy_MoveRejected(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNode(context.Background(), "t1", "A")
	body := bytes.NewBufferString("action=move&org_id=mem-a&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=legacy&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("move 仅支持 v4 模式")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Disable_UsesDisabler(t *testing.T) {
	store := &v4WriteSpyStore{}
	body := bytes.NewBufferString("action=disable&org_id=u1&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
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
	if got := strings.Join(store.disableArgs, "|"); got != "t1|2026-01-05|u1" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Disable_DefaultsEffectiveDateToAsOf(t *testing.T) {
	store := &v4WriteSpyStore{}
	body := bytes.NewBufferString("action=disable&org_id=u1")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.Join(store.disableArgs, "|"); got != "t1|2026-01-06|u1" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Disable_BadEffectiveDate(t *testing.T) {
	store := &v4WriteSpyStore{v4Nodes: []OrgUnitNode{{ID: "v1", Name: "V"}}}
	body := bytes.NewBufferString("action=disable&org_id=u1&effective_date=bad")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
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

func TestHandleOrgNodes_POST_ReadV4_Disable_Error(t *testing.T) {
	store := &v4WriteSpyStore{
		err:     errors.New("boom"),
		v4Nodes: []OrgUnitNode{{ID: "v1", Name: "V"}},
	}
	body := bytes.NewBufferString("action=disable&org_id=u1&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.disableCalled != 1 {
		t.Fatalf("disableCalled=%d", store.disableCalled)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("boom")) || !bytes.Contains([]byte(bodyOut), []byte("V")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_Disable_MissingDisabler(t *testing.T) {
	store := v4CreateOnlyStore{}
	body := bytes.NewBufferString("action=disable&org_id=u1&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("v4 disabler 未配置")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadLegacy_DisableRejected(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNode(context.Background(), "t1", "A")
	body := bytes.NewBufferString("action=disable&org_id=mem-a&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=legacy&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("disable 仅支持 v4 模式")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_DefaultReadV4_UsesAsOfNow(t *testing.T) {
	store := &v4WriteSpyStore{}
	body := bytes.NewBufferString("name=Hello")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/org/nodes?read=v4&as_of=") {
		t.Fatalf("location=%q", loc)
	}
	asOf := strings.TrimPrefix(loc, "/org/nodes?read=v4&as_of=")
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		t.Fatalf("as_of=%q err=%v", asOf, err)
	}
}

func TestHandleOrgNodes_POST_ReadV4_BadEffectiveDate(t *testing.T) {
	store := &v4WriteSpyStore{}
	body := bytes.NewBufferString("name=Hello&effective_date=bad")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.v4Called != 0 {
		t.Fatalf("v4Called=%d", store.v4Called)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("effective_date 无效")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_MissingWriter(t *testing.T) {
	store := v4ReadOnlyStore{nodes: []OrgUnitNode{{ID: "v1", Name: "V"}}}
	body := bytes.NewBufferString("name=Hello&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("v4 writer 未配置")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_CreateError(t *testing.T) {
	store := &v4WriteSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("name=Hello&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.v4Called != 1 {
		t.Fatalf("v4Called=%d", store.v4Called)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("boom")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_ReadV4_CreateError_WithV4Nodes(t *testing.T) {
	store := &v4WriteSpyStore{
		err:     errors.New("boom"),
		v4Nodes: []OrgUnitNode{{ID: "v1", Name: "V"}},
	}
	body := bytes.NewBufferString("name=Hello&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?read=v4&as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("boom")) || !bytes.Contains([]byte(bodyOut), []byte("V")) {
		t.Fatalf("unexpected body: %q", bodyOut)
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
	out := renderOrgNodes(nil, Tenant{Name: "T"}, "", "legacy", "2026-01-06")
	if out == "" {
		t.Fatal("expected output")
	}
	out2 := renderOrgNodes([]OrgUnitNode{{ID: "1", Name: "N"}}, Tenant{Name: "T"}, "err", "legacy", "2026-01-06")
	if out2 == "" {
		t.Fatal("expected output")
	}
}

func TestOrgUnitPGStore_ListAndCreate(t *testing.T) {
	pool := &fakeBeginner{}
	store := &orgUnitPGStore{pool: pool}

	nodes, err := store.ListNodes(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ID != "n1" {
		t.Fatalf("nodes=%+v", nodes)
	}

	nodesV4, err := store.ListNodesV4(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodesV4) != 1 || nodesV4[0].ID != "n1" {
		t.Fatalf("nodes=%+v", nodesV4)
	}

	created, err := store.CreateNode(context.Background(), "t1", "A")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "n2" || created.Name != "A" {
		t.Fatalf("created=%+v", created)
	}

	createdV4, err := store.CreateNodeV4(context.Background(), "t1", "2026-01-06", "V4", "")
	if err != nil {
		t.Fatal(err)
	}
	if createdV4.ID != "u1" || createdV4.Name != "V4" || !createdV4.CreatedAt.Equal(time.Unix(789, 0).UTC()) {
		t.Fatalf("created=%+v", createdV4)
	}

	createdV4Child, err := store.CreateNodeV4(context.Background(), "t1", "2026-01-06", "V4 Child", "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if createdV4Child.ID != "u1" || createdV4Child.Name != "V4 Child" || !createdV4Child.CreatedAt.Equal(time.Unix(789, 0).UTC()) {
		t.Fatalf("created=%+v", createdV4Child)
	}
}

type v4SpyStore struct {
	legacyCalled int
	v4Called     int

	v4Nodes     []OrgUnitNode
	v4Err       error
	legacyNodes []OrgUnitNode
	legacyErr   error
}

func (s *v4SpyStore) ListNodes(context.Context, string) ([]OrgUnitNode, error) {
	s.legacyCalled++
	return s.legacyNodes, s.legacyErr
}

func (s *v4SpyStore) ListNodesV4(context.Context, string, string) ([]OrgUnitNode, error) {
	s.v4Called++
	return s.v4Nodes, s.v4Err
}

func (s *v4SpyStore) CreateNode(context.Context, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}

func TestHandleOrgNodes_GET_ReadV4_UsesV4WhenAvailable(t *testing.T) {
	store := &v4SpyStore{
		v4Nodes:     []OrgUnitNode{{ID: "v1", Name: "V"}},
		legacyNodes: []OrgUnitNode{{ID: "l1", Name: "L"}},
	}

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=v4&as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.v4Called != 1 || store.legacyCalled != 0 {
		t.Fatalf("calls v4=%d legacy=%d", store.v4Called, store.legacyCalled)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("V")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_DefaultReadV4(t *testing.T) {
	store := &v4SpyStore{
		v4Nodes:     []OrgUnitNode{{ID: "v1", Name: "V"}},
		legacyNodes: []OrgUnitNode{{ID: "l1", Name: "L"}},
	}

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.v4Called != 1 || store.legacyCalled != 0 {
		t.Fatalf("calls v4=%d legacy=%d", store.v4Called, store.legacyCalled)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("V")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_ReadInvalidDefaultsToV4(t *testing.T) {
	store := &v4SpyStore{
		v4Nodes:     []OrgUnitNode{{ID: "v1", Name: "V"}},
		legacyNodes: []OrgUnitNode{{ID: "l1", Name: "L"}},
	}

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=nope&as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.v4Called != 1 || store.legacyCalled != 0 {
		t.Fatalf("calls v4=%d legacy=%d", store.v4Called, store.legacyCalled)
	}
}

func TestHandleOrgNodes_GET_ReadV4_FallbackOnError(t *testing.T) {
	store := &v4SpyStore{
		v4Err:       errors.New("v4 boom"),
		legacyNodes: []OrgUnitNode{{ID: "l1", Name: "L"}},
	}

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=v4&as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.v4Called != 1 || store.legacyCalled != 1 {
		t.Fatalf("calls v4=%d legacy=%d", store.v4Called, store.legacyCalled)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("v4 读取失败")) || !bytes.Contains([]byte(body), []byte("L")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_ReadV4_FallbackOnEmpty(t *testing.T) {
	store := &v4SpyStore{
		v4Nodes:     nil,
		legacyNodes: []OrgUnitNode{{ID: "l1", Name: "L"}},
	}

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=v4&as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.v4Called != 1 || store.legacyCalled != 1 {
		t.Fatalf("calls v4=%d legacy=%d", store.v4Called, store.legacyCalled)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("v4 快照为空")) || !bytes.Contains([]byte(body), []byte("L")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_ReadV4_BadAsOf(t *testing.T) {
	store := &v4SpyStore{
		v4Nodes:     []OrgUnitNode{{ID: "v1", Name: "V"}},
		legacyNodes: []OrgUnitNode{{ID: "l1", Name: "L"}},
	}

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=v4&as_of=bad", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.v4Called != 0 || store.legacyCalled != 1 {
		t.Fatalf("calls v4=%d legacy=%d", store.v4Called, store.legacyCalled)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("as_of 无效")) || !bytes.Contains([]byte(body), []byte("L")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_ReadV4_BadAsOf_LegacyError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=v4&as_of=bad", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("as_of 无效，且 legacy 读取失败")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_ReadV4_StoreWithoutV4_LegacyError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=v4&as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("v4 reader 未配置，且 legacy 读取失败")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_ReadV4_StoreWithoutV4_FallbackSuccess(t *testing.T) {
	mem := newOrgUnitMemoryStore()
	_, _ = mem.CreateNode(context.Background(), "t1", "L")
	store := legacyOnlyStore{inner: mem}

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=v4&as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("v4 reader 未配置，已回退到 legacy")) || !bytes.Contains([]byte(body), []byte("L")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

type v4ErrStore struct {
	legacyErr error
	v4Err     error
}

func (s v4ErrStore) ListNodes(context.Context, string) ([]OrgUnitNode, error) {
	return nil, s.legacyErr
}
func (s v4ErrStore) CreateNode(context.Context, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}
func (s v4ErrStore) ListNodesV4(context.Context, string, string) ([]OrgUnitNode, error) {
	return nil, s.v4Err
}

func TestHandleOrgNodes_GET_ReadV4_V4Error_LegacyError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=v4&as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, v4ErrStore{legacyErr: errors.New("legacy boom"), v4Err: errors.New("v4 boom")})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("v4 读取失败: v4 boom；且 legacy 读取失败: legacy boom")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

type v4EmptyLegacyErrStore struct{}

func (v4EmptyLegacyErrStore) ListNodes(context.Context, string) ([]OrgUnitNode, error) {
	return nil, errors.New("legacy boom")
}
func (v4EmptyLegacyErrStore) CreateNode(context.Context, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}
func (v4EmptyLegacyErrStore) ListNodesV4(context.Context, string, string) ([]OrgUnitNode, error) {
	return nil, nil
}

func TestHandleOrgNodes_GET_ReadV4_Empty_LegacyError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?read=v4&as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, v4EmptyLegacyErrStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("legacy 读取失败: legacy boom")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestOrgUnitPGStore_ListNodes_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				queryErr: errors.New("query"),
			}, nil
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{scanErr: errors.New("scan")}}, nil
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows_err", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{err: errors.New("rows")}}, nil
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit")}, nil
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_ListNodesV4_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		_, err := store.ListNodesV4(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		_, err := store.ListNodesV4(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		})}
		_, err := store.ListNodesV4(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{scanErr: errors.New("scan")}}, nil
		})}
		_, err := store.ListNodesV4(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows_err", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{err: errors.New("rows")}}, nil
		})}
		_, err := store.ListNodesV4(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit")}, nil
		})}
		_, err := store.ListNodesV4(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_CreateNode_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.CreateNode(context.Background(), "t1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.CreateNode(context.Background(), "t1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit_scan", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.CreateNode(context.Background(), "t1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("created_at_scan", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"n1"}}
			tx.row2Err = errors.New("row2")
			return tx, nil
		}))
		_, err := store.CreateNode(context.Background(), "t1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"n1"}}
			tx.row2 = &stubRow{vals: []any{time.Unix(1, 0).UTC()}}
			return tx, nil
		}))
		_, err := store.CreateNode(context.Background(), "t1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_CreateNodeV4_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		_, err := store.CreateNodeV4(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		_, err := store.CreateNodeV4(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("effective_date_required", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		_, err := store.CreateNodeV4(context.Background(), "t1", "", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("org_id_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		})}
		_, err := store.CreateNodeV4(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event_id_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"u1"}}
			tx.row2Err = errors.New("row2")
			return tx, nil
		})}
		_, err := store.CreateNodeV4(context.Background(), "t1", "2026-01-06", "A", "")
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
		_, err := store.CreateNodeV4(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("tx_time_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"u1"}}
			tx.row2 = &stubRow{vals: []any{"e1"}}
			tx.row3Err = errors.New("row3")
			return tx, nil
		})}
		_, err := store.CreateNodeV4(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"u1"}}
			tx.row2 = &stubRow{vals: []any{"e1"}}
			tx.row3 = &stubRow{vals: []any{time.Unix(2, 0).UTC()}}
			return tx, nil
		})}
		_, err := store.CreateNodeV4(context.Background(), "t1", "2026-01-06", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_RenameNodeV4_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		err := store.RenameNodeV4(context.Background(), "t1", "2026-01-06", "u1", "New")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		err := store.RenameNodeV4(context.Background(), "t1", "2026-01-06", "u1", "New")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("effective_date_required", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		err := store.RenameNodeV4(context.Background(), "t1", "", "u1", "New")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("org_id_required", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		err := store.RenameNodeV4(context.Background(), "t1", "2026-01-06", "", "New")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("new_name_required", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		err := store.RenameNodeV4(context.Background(), "t1", "2026-01-06", "u1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event_id_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		})}
		err := store.RenameNodeV4(context.Background(), "t1", "2026-01-06", "u1", "New")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit_exec", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		err := store.RenameNodeV4(context.Background(), "t1", "2026-01-06", "u1", "New")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		err := store.RenameNodeV4(context.Background(), "t1", "2026-01-06", "u1", "New")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_RenameNodeV4_Success(t *testing.T) {
	store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{"e1"}}
		return tx, nil
	})}
	if err := store.RenameNodeV4(context.Background(), "t1", "2026-01-06", "u1", "New"); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitPGStore_MoveNodeV4_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		err := store.MoveNodeV4(context.Background(), "t1", "2026-01-06", "u1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		err := store.MoveNodeV4(context.Background(), "t1", "2026-01-06", "u1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("effective_date_required", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		err := store.MoveNodeV4(context.Background(), "t1", "", "u1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("org_id_required", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		err := store.MoveNodeV4(context.Background(), "t1", "2026-01-06", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event_id_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		})}
		err := store.MoveNodeV4(context.Background(), "t1", "2026-01-06", "u1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit_exec", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		err := store.MoveNodeV4(context.Background(), "t1", "2026-01-06", "u1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		err := store.MoveNodeV4(context.Background(), "t1", "2026-01-06", "u1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_MoveNodeV4_Success(t *testing.T) {
	store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{"e1"}}
		return tx, nil
	})}
	if err := store.MoveNodeV4(context.Background(), "t1", "2026-01-06", "u1", "u0"); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitPGStore_DisableNodeV4_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		err := store.DisableNodeV4(context.Background(), "t1", "2026-01-06", "u1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		err := store.DisableNodeV4(context.Background(), "t1", "2026-01-06", "u1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("effective_date_required", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		err := store.DisableNodeV4(context.Background(), "t1", "", "u1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("org_id_required", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		err := store.DisableNodeV4(context.Background(), "t1", "2026-01-06", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event_id_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		})}
		err := store.DisableNodeV4(context.Background(), "t1", "2026-01-06", "u1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit_exec", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		err := store.DisableNodeV4(context.Background(), "t1", "2026-01-06", "u1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		err := store.DisableNodeV4(context.Background(), "t1", "2026-01-06", "u1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_DisableNodeV4_Success(t *testing.T) {
	store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{"e1"}}
		return tx, nil
	})}
	if err := store.DisableNodeV4(context.Background(), "t1", "2026-01-06", "u1"); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestNewHandlerWithOptions_BadDBURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://%zz")
	t.Setenv("ALLOWLIST_PATH", "../../config/routing/allowlist.yaml")
	t.Setenv("TENANTS_PATH", "../../config/tenants.yaml")

	_, err := NewHandlerWithOptions(HandlerOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

type beginnerFunc func(ctx context.Context) (pgx.Tx, error)

func (f beginnerFunc) Begin(ctx context.Context) (pgx.Tx, error) { return f(ctx) }

type stubTx struct {
	execErr   error
	execErrAt int
	execN     int
	queryErr  error
	commitErr error
	rowErr    error
	row2Err   error
	row3Err   error

	rows *stubRows
	row  pgx.Row
	row2 pgx.Row
	row3 pgx.Row
}

func (t *stubTx) Begin(ctx context.Context) (pgx.Tx, error) { return t, nil }
func (t *stubTx) Commit(context.Context) error              { return t.commitErr }
func (t *stubTx) Rollback(context.Context) error            { return nil }
func (t *stubTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *stubTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return fakeBatchResults{} }
func (t *stubTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *stubTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *stubTx) Conn() *pgx.Conn { return nil }

func (t *stubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	t.execN++
	if t.execErr != nil {
		at := t.execErrAt
		if at == 0 {
			at = 1
		}
		if t.execN == at {
			return pgconn.CommandTag{}, t.execErr
		}
	}
	return pgconn.CommandTag{}, nil
}

func (t *stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if t.queryErr != nil {
		return nil, t.queryErr
	}
	if t.rows != nil {
		return t.rows, nil
	}
	return &fakeRows{}, nil
}

func (t *stubTx) QueryRow(context.Context, string, ...any) pgx.Row {
	if t.rowErr != nil {
		return &stubRow{err: t.rowErr}
	}
	if t.row != nil {
		r := t.row
		t.row = nil
		return r
	}
	if t.row2Err != nil {
		return &stubRow{err: t.row2Err}
	}
	if t.row2 != nil {
		r := t.row2
		t.row2 = nil
		return r
	}
	if t.row3Err != nil {
		return &stubRow{err: t.row3Err}
	}
	if t.row3 != nil {
		return t.row3
	}
	return fakeRow{}
}

type stubRows struct {
	empty   bool
	nextN   int
	scanErr error
	err     error
}

func (r *stubRows) Close()                        {}
func (r *stubRows) Err() error                    { return r.err }
func (r *stubRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *stubRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *stubRows) Next() bool {
	if r.empty {
		return false
	}
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *stubRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	return (&fakeRows{}).Scan(dest...)
}
func (r *stubRows) Values() ([]any, error) { return nil, nil }
func (r *stubRows) RawValues() [][]byte    { return nil }
func (r *stubRows) Conn() *pgx.Conn        { return nil }

type stubRow struct {
	vals []any
	err  error
}

func (r *stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return fakeRow{vals: r.vals}.Scan(dest...)
}

type fakeBeginner struct {
	beginCount int
}

func (b *fakeBeginner) Begin(context.Context) (pgx.Tx, error) {
	b.beginCount++
	return &fakeTx{beginCount: b.beginCount}, nil
}

type fakeTx struct {
	beginCount int
	uuidN      int
	committed  bool
	rolled     bool
}

func (t *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &fakeRows{idx: 0}, nil
}

func (t *fakeTx) QueryRow(ctx context.Context, q string, args ...any) pgx.Row {
	if strings.Contains(q, "gen_random_uuid") {
		t.uuidN++
		switch t.uuidN {
		case 1:
			return fakeRow{vals: []any{"u1"}}
		default:
			return fakeRow{vals: []any{"e1"}}
		}
	}
	if strings.Contains(q, "submit_orgunit_event") {
		return fakeRow{vals: []any{"n2"}}
	}
	if strings.Contains(q, "FROM orgunit.nodes") {
		return fakeRow{vals: []any{time.Unix(456, 0).UTC()}}
	}
	if strings.Contains(q, "FROM orgunit.org_events") {
		return fakeRow{vals: []any{time.Unix(789, 0).UTC()}}
	}
	return &stubRow{err: errors.New("unexpected QueryRow")}
}

func (t *fakeTx) Commit(context.Context) error   { t.committed = true; return nil }
func (t *fakeTx) Rollback(context.Context) error { t.rolled = true; return nil }

func (t *fakeTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return fakeBatchResults{} }
func (t *fakeTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *fakeTx) Conn() *pgx.Conn { return nil }

type fakeBatchResults struct{}

func (fakeBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (fakeBatchResults) Query() (pgx.Rows, error)         { return &fakeRows{}, nil }
func (fakeBatchResults) QueryRow() pgx.Row                { return fakeRow{} }
func (fakeBatchResults) Close() error                     { return nil }

type fakeRows struct {
	idx int
}

func (r *fakeRows) Close()                        {}
func (r *fakeRows) Err() error                    { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *fakeRows) Next() bool {
	if r.idx > 0 {
		return false
	}
	r.idx++
	return true
}
func (r *fakeRows) Scan(dest ...any) error {
	*(dest[0].(*string)) = "n1"
	*(dest[1].(*string)) = "Node"
	*(dest[2].(*time.Time)) = time.Unix(123, 0).UTC()
	return nil
}
func (r *fakeRows) Values() ([]any, error) { return nil, nil }
func (r *fakeRows) RawValues() [][]byte    { return nil }
func (r *fakeRows) Conn() *pgx.Conn        { return nil }

type fakeRow struct {
	vals []any
}

func (r fakeRow) Scan(dest ...any) error {
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = r.vals[i].(string)
		case *time.Time:
			*d = r.vals[i].(time.Time)
		}
	}
	return nil
}
