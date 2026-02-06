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

	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestHandleOrgUnitsBusinessUnitAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", nil)
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/set-business-unit", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_InvalidRequest(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"","effective_date":"","request_code":""}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_BothOrgIDAndCode(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"10000001","org_code":"A1","effective_date":"2026-01-01","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_InvalidEffectiveDate(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"bad","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, &resolveOrgCodeStore{resolveID: 10000001})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgUnitIDProvided(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"123","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_StoreError(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, errOrgUnitStore{err: errBoom{}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

type resolveOrgCodeStore struct {
	resolveID       int
	resolveErr      error
	setErr          error
	setArgs         []string
	listNodes       []OrgUnitNode
	listNodesErr    error
	listChildren    []OrgUnitChild
	listChildrenErr error
}

func (s *resolveOrgCodeStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	if s.listNodesErr != nil {
		return nil, s.listNodesErr
	}
	return append([]OrgUnitNode(nil), s.listNodes...), nil
}
func (s *resolveOrgCodeStore) CreateNodeCurrent(context.Context, string, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}
func (s *resolveOrgCodeStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (s *resolveOrgCodeStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (s *resolveOrgCodeStore) DisableNodeCurrent(context.Context, string, string, string) error {
	return nil
}
func (s *resolveOrgCodeStore) SetBusinessUnitCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, _ bool, requestCode string) error {
	s.setArgs = []string{tenantID, effectiveDate, orgID, requestCode}
	return s.setErr
}
func (s *resolveOrgCodeStore) ResolveOrgID(context.Context, string, string) (int, error) {
	if s.resolveErr != nil {
		return 0, s.resolveErr
	}
	return s.resolveID, nil
}
func (s *resolveOrgCodeStore) ResolveOrgCode(context.Context, string, int) (string, error) {
	return "", nil
}
func (s *resolveOrgCodeStore) ResolveOrgCodes(context.Context, string, []int) (map[int]string, error) {
	return map[int]string{}, nil
}
func (s *resolveOrgCodeStore) ListChildren(context.Context, string, int, string) ([]OrgUnitChild, error) {
	if s.listChildrenErr != nil {
		return nil, s.listChildrenErr
	}
	return append([]OrgUnitChild(nil), s.listChildren...), nil
}
func (s *resolveOrgCodeStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	return OrgUnitNodeDetails{}, nil
}
func (s *resolveOrgCodeStore) SearchNode(context.Context, string, string, string) (OrgUnitSearchResult, error) {
	return OrgUnitSearchResult{}, nil
}
func (s *resolveOrgCodeStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return []OrgUnitSearchCandidate{}, nil
}
func (s *resolveOrgCodeStore) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	return []OrgUnitNodeVersion{}, nil
}
func (s *resolveOrgCodeStore) MaxEffectiveDateOnOrBefore(context.Context, string, string) (string, bool, error) {
	return "", false, nil
}
func (s *resolveOrgCodeStore) MinEffectiveDate(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeInvalid(t *testing.T) {
	store := &resolveOrgCodeStore{}
	body := bytes.NewBufferString(`{"org_code":"bad\u007f","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeNotFound(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeNotFound}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeResolveInvalid(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeInvalid}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeResolveError(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: errBoom{}}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_SetBusinessUnitError(t *testing.T) {
	store := &resolveOrgCodeStore{resolveID: 10000001, setErr: errBoom{}}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeSuccess(t *testing.T) {
	store := &resolveOrgCodeStore{resolveID: 10000001}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if len(store.setArgs) != 4 || store.setArgs[2] != "10000001" {
		t.Fatalf("unexpected set args: %+v", store.setArgs)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "ORG1", "Org1", "", false)
	if err != nil {
		t.Fatalf("err=%v", err)
	}

	body := bytes.NewBufferString(`{"org_code":"` + created.OrgCode + `","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), created.OrgCode) {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgUnitsAPI_ListRoots(t *testing.T) {
	store := &resolveOrgCodeStore{
		listNodes: []OrgUnitNode{{ID: "10000001", OrgCode: "A001", Name: "Root", IsBusinessUnit: true}},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		OrgUnits []map[string]any `json:"org_units"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if len(payload.OrgUnits) != 1 {
		t.Fatalf("expected 1 org unit, got %d", len(payload.OrgUnits))
	}
}

func TestHandleOrgUnitsAPI_PostMissingService(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", strings.NewReader(`{}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_CreateSuccess(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		createFn: func(_ context.Context, tenantID string, req orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			if tenantID != "t1" {
				return orgunittypes.OrgUnitResult{}, errors.New("bad tenant")
			}
			return orgunittypes.OrgUnitResult{
				OrgCode:       "A001",
				EffectiveDate: req.EffectiveDate,
				Fields:        map[string]any{"name": req.Name},
			}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","name":"Root","effective_date":"2026-01-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), svc)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "A001") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgUnitsRenameAPI_NotFound(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		renameFn: func(context.Context, string, orgunitservices.RenameOrgUnitRequest) error {
			return errors.New("ORG_CODE_NOT_FOUND")
		},
	}
	body := strings.NewReader(`{"org_code":"A001","new_name":"New","effective_date":"2026-01-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rename", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRenameAPI(rec, req, svc)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleOrgUnitsAPI_InvalidAsOf(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=bad", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListParentInvalidOrgCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=bad%7F", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListParentNotFound(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeNotFound}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListParentResolveInvalid(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeInvalid}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListParentResolveError(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: errBoom{}}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListChildrenNotFound(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID:       10000001,
		listChildrenErr: errOrgUnitNotFound,
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListChildrenError(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID:       10000001,
		listChildrenErr: errBoom{},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListChildrenSuccess(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID:    10000001,
		listChildren: []OrgUnitChild{{OrgID: 10000002, OrgCode: "A002", Name: "Child", HasChildren: true}},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "A002") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgUnitsAPI_ListNodesError(t *testing.T) {
	store := &resolveOrgCodeStore{listNodesErr: errBoom{}}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/org/api/org-units", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units", nil)
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_DefaultAsOf(t *testing.T) {
	store := &resolveOrgCodeStore{
		listNodes: []OrgUnitNode{{ID: "10000001", OrgCode: "A001", Name: "Root"}},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsRenameAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rename", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRenameAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsRenameAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/rename", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRenameAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsRenameAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rename", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handleOrgUnitsRenameAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsRenameAPI_ServiceMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rename", strings.NewReader(`{}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRenameAPI(rec, req, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_BadJSON(t *testing.T) {
	svc := orgUnitWriteServiceStub{}
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_CreateError(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		createFn: func(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_CODE_INVALID")
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", strings.NewReader(`{"org_code":"bad","name":"Root","effective_date":"2026-01-01"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsRenameAPI_DefaultEffectiveDate(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		renameFn: func(_ context.Context, _ string, req orgunitservices.RenameOrgUnitRequest) error {
			called = true
			if req.EffectiveDate == "" {
				t.Fatalf("expected default effective_date")
			}
			return nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","new_name":"New"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rename", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRenameAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !called {
		t.Fatalf("expected rename call")
	}
}

func TestHandleOrgUnitsMoveAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		moveFn: func(_ context.Context, _ string, req orgunitservices.MoveOrgUnitRequest) error {
			called = true
			if req.NewParentOrgCode == "" {
				t.Fatalf("expected parent org code")
			}
			return nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","new_parent_org_code":"A0001","effective_date":"2026-01-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/move", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsMoveAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !called {
		t.Fatalf("expected move call")
	}
}

func TestHandleOrgUnitsMoveAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/move", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsMoveAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsDisableAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		disableFn: func(_ context.Context, _ string, req orgunitservices.DisableOrgUnitRequest) error {
			called = true
			if req.OrgCode == "" {
				t.Fatalf("expected org code")
			}
			return nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/disable", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsDisableAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !called {
		t.Fatalf("expected disable call")
	}
}

func TestHandleOrgUnitsDisableAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/disable", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsDisableAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsCorrectionsAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		correctFn: func(_ context.Context, tenantID string, req orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			called = true
			if tenantID != "t1" {
				t.Fatalf("unexpected tenant: %s", tenantID)
			}
			if req.TargetEffectiveDate != "2026-01-01" {
				t.Fatalf("unexpected effective date: %s", req.TargetEffectiveDate)
			}
			if req.RequestID != "r1" {
				t.Fatalf("unexpected request_id: %s", req.RequestID)
			}
			if req.Patch.Name == nil || *req.Patch.Name != "New Name" {
				t.Fatalf("expected patch name")
			}
			return orgunittypes.OrgUnitResult{
				OrgCode:       "A001",
				EffectiveDate: "2026-01-01",
				Fields:        map[string]any{"name": "New Name"},
			}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","patch":{"name":"New Name"},"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatalf("expected correct call")
	}
	if !strings.Contains(rec.Body.String(), "A001") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgUnitsCorrectionsAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", strings.NewReader(`{"org_code":"A001"}`))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsCorrectionsAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/corrections", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsCorrectionsAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsCorrectionsAPI_MissingService(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","patch":{"name":"New"},"request_id":"r1"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsCorrectionsAPI_NotFound(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		correctFn: func(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_EVENT_NOT_FOUND")
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","patch":{"name":"New"},"request_id":"r1"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, svc)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWriteOrgUnitServiceError_StatusMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"not_found", errors.New("ORG_CODE_NOT_FOUND"), http.StatusNotFound},
		{"bad_request_code", errors.New("ORG_CODE_INVALID"), http.StatusBadRequest},
		{"conflict", errors.New("EVENT_DATE_CONFLICT"), http.StatusConflict},
		{"bad_request_msg", newBadRequestError("name is required"), http.StatusBadRequest},
		{"stable_unknown", errors.New("SOME_DB_CODE"), http.StatusUnprocessableEntity},
		{"unknown", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", nil)
			rec := httptest.NewRecorder()
			writeOrgUnitServiceError(rec, req, tt.err, "orgunit_failed")
			if rec.Code != tt.status {
				t.Fatalf("status=%d", rec.Code)
			}
		})
	}
}

type orgUnitWriteServiceStub struct {
	createFn  func(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
	renameFn  func(context.Context, string, orgunitservices.RenameOrgUnitRequest) error
	moveFn    func(context.Context, string, orgunitservices.MoveOrgUnitRequest) error
	disableFn func(context.Context, string, orgunitservices.DisableOrgUnitRequest) error
	correctFn func(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
}

func (s orgUnitWriteServiceStub) Create(ctx context.Context, tenantID string, req orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	if s.createFn == nil {
		return orgunittypes.OrgUnitResult{}, nil
	}
	return s.createFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) Rename(ctx context.Context, tenantID string, req orgunitservices.RenameOrgUnitRequest) error {
	if s.renameFn == nil {
		return nil
	}
	return s.renameFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) Move(ctx context.Context, tenantID string, req orgunitservices.MoveOrgUnitRequest) error {
	if s.moveFn == nil {
		return nil
	}
	return s.moveFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) Disable(ctx context.Context, tenantID string, req orgunitservices.DisableOrgUnitRequest) error {
	if s.disableFn == nil {
		return nil
	}
	return s.disableFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) SetBusinessUnit(context.Context, string, orgunitservices.SetBusinessUnitRequest) error {
	return nil
}

func (s orgUnitWriteServiceStub) Correct(ctx context.Context, tenantID string, req orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	if s.correctFn == nil {
		return orgunittypes.OrgUnitResult{}, nil
	}
	return s.correctFn(ctx, tenantID, req)
}
