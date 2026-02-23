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

	"github.com/jackc/pgx/v5/pgconn"
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
	body := bytes.NewBufferString(`{"org_code":"","effective_date":"","request_id":""}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_BothOrgIDAndCode(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"10000001","org_code":"A1","effective_date":"2026-01-01","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_InvalidEffectiveDate(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"bad","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, &resolveOrgCodeStore{resolveID: 10000001})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgUnitIDProvided(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"123","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_StoreError(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, errOrgUnitStore{err: errBoom{}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

type resolveOrgCodeStore struct {
	resolveID  int
	resolveErr error

	setErr  error
	setArgs []string

	listNodes    []OrgUnitNode
	listNodesErr error

	listChildren    []OrgUnitChild
	listChildrenErr error

	resolveCodes    map[int]string
	resolveCodesErr error

	getNodeDetails    OrgUnitNodeDetails
	getNodeDetailsErr error

	searchNodeResult OrgUnitSearchResult
	searchNodeErr    error

	listNodeVersions    []OrgUnitNodeVersion
	listNodeVersionsErr error

	auditEvents    []OrgUnitNodeAuditEvent
	auditEventsErr error
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
func (s *resolveOrgCodeStore) SetBusinessUnitCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, _ bool, requestID string) error {
	s.setArgs = []string{tenantID, effectiveDate, orgID, requestID}
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
	if s.resolveCodesErr != nil {
		return nil, s.resolveCodesErr
	}
	if s.resolveCodes != nil {
		return s.resolveCodes, nil
	}
	return map[int]string{}, nil
}
func (s *resolveOrgCodeStore) ListChildren(context.Context, string, int, string) ([]OrgUnitChild, error) {
	if s.listChildrenErr != nil {
		return nil, s.listChildrenErr
	}
	return append([]OrgUnitChild(nil), s.listChildren...), nil
}
func (s *resolveOrgCodeStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	if s.getNodeDetailsErr != nil {
		return OrgUnitNodeDetails{}, s.getNodeDetailsErr
	}
	return s.getNodeDetails, nil
}
func (s *resolveOrgCodeStore) SearchNode(context.Context, string, string, string) (OrgUnitSearchResult, error) {
	if s.searchNodeErr != nil {
		return OrgUnitSearchResult{}, s.searchNodeErr
	}
	return s.searchNodeResult, nil
}
func (s *resolveOrgCodeStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return []OrgUnitSearchCandidate{}, nil
}
func (s *resolveOrgCodeStore) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	if s.listNodeVersionsErr != nil {
		return nil, s.listNodeVersionsErr
	}
	return append([]OrgUnitNodeVersion(nil), s.listNodeVersions...), nil
}
func (s *resolveOrgCodeStore) ListNodeAuditEvents(context.Context, string, int, int) ([]OrgUnitNodeAuditEvent, error) {
	if s.auditEventsErr != nil {
		return nil, s.auditEventsErr
	}
	return append([]OrgUnitNodeAuditEvent(nil), s.auditEvents...), nil
}
func (s *resolveOrgCodeStore) MaxEffectiveDateOnOrBefore(context.Context, string, string) (string, bool, error) {
	return "", false, nil
}
func (s *resolveOrgCodeStore) MinEffectiveDate(context.Context, string) (string, bool, error) {
	return "", false, nil
}

type orgUnitListPageReaderStore struct {
	*resolveOrgCodeStore
	items       []orgUnitListItem
	total       int
	err         error
	capturedReq orgUnitListPageRequest
}

func (s *orgUnitListPageReaderStore) ListOrgUnitsPage(_ context.Context, _ string, req orgUnitListPageRequest) ([]orgUnitListItem, int, error) {
	s.capturedReq = req
	if s.err != nil {
		return nil, 0, s.err
	}
	return append([]orgUnitListItem(nil), s.items...), s.total, nil
}

type orgUnitDetailsExtStoreStub struct {
	*resolveOrgCodeStore
	cfgs        []orgUnitTenantFieldConfig
	cfgErr      error
	snapshot    orgUnitVersionExtSnapshot
	snapshotErr error
}

func (s orgUnitDetailsExtStoreStub) ListEnabledTenantFieldConfigsAsOf(_ context.Context, _ string, _ string) ([]orgUnitTenantFieldConfig, error) {
	if s.cfgErr != nil {
		return nil, s.cfgErr
	}
	return append([]orgUnitTenantFieldConfig(nil), s.cfgs...), nil
}

func (s orgUnitDetailsExtStoreStub) GetOrgUnitVersionExtSnapshot(_ context.Context, _ string, _ int, _ string) (orgUnitVersionExtSnapshot, error) {
	if s.snapshotErr != nil {
		return orgUnitVersionExtSnapshot{}, s.snapshotErr
	}
	return s.snapshot, nil
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeInvalid(t *testing.T) {
	store := &resolveOrgCodeStore{}
	body := bytes.NewBufferString(`{"org_code":"bad\u007f","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
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
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
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
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
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
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
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
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
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
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
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

	body := bytes.NewBufferString(`{"org_code":"` + created.OrgCode + `","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
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

func TestHandleOrgUnitsBusinessUnitAPI_WriteServicePath(t *testing.T) {
	var captured orgunitservices.SetBusinessUnitRequest
	svc := orgUnitWriteServiceStub{
		setBusinessUnitFn: func(_ context.Context, tenantID string, req orgunitservices.SetBusinessUnitRequest) error {
			if tenantID != "t1" {
				t.Fatalf("tenant=%s", tenantID)
			}
			captured = req
			return nil
		},
	}

	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"ext":{"org_type":"DEPARTMENT"}}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if captured.OrgCode != "A1" || captured.EffectiveDate != "2026-01-01" || !captured.IsBusinessUnit {
		t.Fatalf("captured=%+v", captured)
	}
	if captured.Ext["org_type"] != "DEPARTMENT" {
		t.Fatalf("ext=%v", captured.Ext)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_WriteServiceRejectsExtLabelsSnapshot(t *testing.T) {
	svc := orgUnitWriteServiceStub{}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"ext_labels_snapshot":{"org_type":"Department"}}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_WriteServiceRejectsUnknownField(t *testing.T) {
	svc := orgUnitWriteServiceStub{}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"req-unknown","unknown":1}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_WriteServiceValidationAndErrorMapping(t *testing.T) {
	t.Run("effective date required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", bytes.NewBufferString(`{"org_code":"A1","effective_date":"   ","is_business_unit":true}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsBusinessUnitAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("org code required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", bytes.NewBufferString(`{"org_unit_id":"10000001","effective_date":"2026-01-01","is_business_unit":true}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsBusinessUnitAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("service error mapped", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{
			setBusinessUnitFn: func(context.Context, string, orgunitservices.SetBusinessUnitRequest) error {
				return errors.New("ORG_INVALID_ARGUMENT")
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsBusinessUnitAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleOrgUnitsBusinessUnitAPI_DependencyMissing(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, struct{}{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
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

func TestHandleOrgUnitsAPI_ListServerModeChildren(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID: 10000001,
		listChildren: []OrgUnitChild{
			{OrgID: 10000002, OrgCode: "A002", Name: "Finance", Status: "active", IsBusinessUnit: false, HasChildren: true},
			{OrgID: 10000003, OrgCode: "A003", Name: "Admin", Status: "disabled", IsBusinessUnit: true, HasChildren: false},
			{OrgID: 10000004, OrgCode: "B001", Name: "Budget", Status: "active", IsBusinessUnit: false, HasChildren: false},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001&include_disabled=1&mode=grid&status=inactive&sort=name&order=asc&page=0&size=10", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Page     int               `json:"page"`
		Size     int               `json:"size"`
		Total    int               `json:"total"`
		OrgUnits []orgUnitListItem `json:"org_units"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if payload.Page != 0 || payload.Size != 10 || payload.Total != 1 {
		t.Fatalf("unexpected page payload: %+v", payload)
	}
	if len(payload.OrgUnits) != 1 || payload.OrgUnits[0].OrgCode != "A003" {
		t.Fatalf("unexpected rows: %+v", payload.OrgUnits)
	}
}

func TestHandleOrgUnitsAPI_ListServerModePagingAndSort(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID: 10000001,
		listChildren: []OrgUnitChild{
			{OrgID: 10000002, OrgCode: "A002", Name: "Finance", Status: "active"},
			{OrgID: 10000003, OrgCode: "A003", Name: "Admin", Status: "active"},
			{OrgID: 10000004, OrgCode: "B001", Name: "Budget", Status: "active"},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001&mode=grid&q=A&sort=code&order=desc&page=0&size=2", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Total    int               `json:"total"`
		OrgUnits []orgUnitListItem `json:"org_units"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if payload.Total != 2 {
		t.Fatalf("total=%d", payload.Total)
	}
	if len(payload.OrgUnits) != 2 {
		t.Fatalf("len=%d", len(payload.OrgUnits))
	}
	if payload.OrgUnits[0].OrgCode != "A003" || payload.OrgUnits[1].OrgCode != "A002" {
		t.Fatalf("unexpected order: %+v", payload.OrgUnits)
	}
}

func TestHandleOrgUnitsAPI_ListServerModeInvalidQuery(t *testing.T) {
	tests := []string{
		"/org/api/org-units?as_of=2026-01-01&mode=grid&order=desc",
		"/org/api/org-units?as_of=2026-01-01&mode=grid&sort=foo",
		"/org/api/org-units?as_of=2026-01-01&mode=grid&page=-1",
		"/org/api/org-units?as_of=2026-01-01&mode=grid&size=0",
	}
	for _, rawURL := range tests {
		req := httptest.NewRequest(http.MethodGet, rawURL, nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("url=%s status=%d body=%s", rawURL, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleOrgUnitsAPI_ListServerModeNotFoundAndError(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		store := &resolveOrgCodeStore{
			resolveID:       10000001,
			listChildrenErr: errOrgUnitNotFound,
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001&mode=grid&page=0&size=10", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAPI(rec, req, store, nil)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("internal error", func(t *testing.T) {
		store := &resolveOrgCodeStore{
			listNodesErr: errBoom{},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&mode=grid&page=0&size=10", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAPI(rec, req, store, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestParseOrgUnitListQueryOptions(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantAny bool
		wantErr bool
		check   func(t *testing.T, opts orgUnitListQueryOptions)
	}{
		{
			name:    "empty",
			rawURL:  "/org/api/org-units",
			wantAny: false,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.Page != orgUnitListDefaultPage || opts.PageSize != orgUnitListDefaultPageSize {
					t.Fatalf("unexpected defaults: %+v", opts)
				}
			},
		},
		{
			name:    "mode status and sort",
			rawURL:  "/org/api/org-units?mode=grid&status=inactive&sort=name&order=desc&page=2&size=25&q=Root",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.Status != orgUnitListStatusDisabled {
					t.Fatalf("status=%s", opts.Status)
				}
				if opts.SortField != orgUnitListSortName || opts.SortOrder != orgUnitListSortOrderDesc {
					t.Fatalf("sort=%+v", opts)
				}
				if !opts.Paginate || opts.Page != 2 || opts.PageSize != 25 {
					t.Fatalf("paginate=%+v", opts)
				}
				if opts.Keyword != "Root" {
					t.Fatalf("keyword=%q", opts.Keyword)
				}
			},
		},
		{
			name:    "status empty and disabled",
			rawURL:  "/org/api/org-units?status=&mode=grid",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.Status != "" {
					t.Fatalf("status=%q", opts.Status)
				}
			},
		},
		{
			name:    "status disabled alias",
			rawURL:  "/org/api/org-units?status=disabled",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.Status != orgUnitListStatusDisabled {
					t.Fatalf("status=%q", opts.Status)
				}
			},
		},
		{
			name:    "status active",
			rawURL:  "/org/api/org-units?status=active",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.Status != orgUnitListStatusActive {
					t.Fatalf("status=%q", opts.Status)
				}
			},
		},
		{
			name:    "sort default order",
			rawURL:  "/org/api/org-units?sort=code",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.SortOrder != orgUnitListSortOrderAsc {
					t.Fatalf("sort order=%s", opts.SortOrder)
				}
			},
		},
		{
			name:    "page only",
			rawURL:  "/org/api/org-units?page=1",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if !opts.Paginate || opts.Page != 1 || opts.PageSize != orgUnitListDefaultPageSize {
					t.Fatalf("opts=%+v", opts)
				}
			},
		},
		{
			name:    "size only",
			rawURL:  "/org/api/org-units?size=50",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if !opts.Paginate || opts.Page != orgUnitListDefaultPage || opts.PageSize != 50 {
					t.Fatalf("opts=%+v", opts)
				}
			},
		},
		{name: "status invalid", rawURL: "/org/api/org-units?status=bad", wantErr: true},
		{name: "sort invalid", rawURL: "/org/api/org-units?sort=bad", wantErr: true},
		{name: "sort empty", rawURL: "/org/api/org-units?sort=", wantErr: true},
		{name: "order without sort", rawURL: "/org/api/org-units?order=asc", wantErr: true},
		{name: "order invalid", rawURL: "/org/api/org-units?sort=code&order=bad", wantErr: true},
		{name: "order empty", rawURL: "/org/api/org-units?sort=code&order=", wantErr: true},
		{name: "page empty", rawURL: "/org/api/org-units?page=", wantErr: true},
		{name: "page invalid", rawURL: "/org/api/org-units?page=-1", wantErr: true},
		{name: "size empty", rawURL: "/org/api/org-units?size=", wantErr: true},
		{name: "size too large", rawURL: "/org/api/org-units?size=9999", wantErr: true},
		{name: "ext sort missing key", rawURL: "/org/api/org-units?sort=ext:", wantErr: true},
		{name: "ext filter missing key", rawURL: "/org/api/org-units?ext_filter_value=DEPARTMENT", wantErr: true},
		{name: "ext filter missing value", rawURL: "/org/api/org-units?ext_filter_field_key=org_type", wantErr: true},
		{name: "ext filter empty field key", rawURL: "/org/api/org-units?ext_filter_field_key=&ext_filter_value=DEPARTMENT", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.rawURL, nil)
			opts, hasAny, err := parseOrgUnitListQueryOptions(req.URL.Query())
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err=%v", err)
			}
			if hasAny != tt.wantAny {
				t.Fatalf("hasAny=%v want=%v", hasAny, tt.wantAny)
			}
			if tt.check != nil {
				tt.check(t, opts)
			}
		})
	}
}

func TestFilterOrgUnitListItems(t *testing.T) {
	items := []orgUnitListItem{
		{OrgCode: "A001", Name: "Root Org", Status: ""},
		{OrgCode: "B002", Name: "Beta Team", Status: "active"},
		{OrgCode: "C003", Name: "Closed Team", Status: "disabled"},
	}

	all := filterOrgUnitListItems(items, "", "")
	if len(all) != 3 {
		t.Fatalf("len=%d", len(all))
	}

	byStatus := filterOrgUnitListItems(items, "", "active")
	if len(byStatus) != 2 {
		t.Fatalf("status len=%d", len(byStatus))
	}

	byKeyword := filterOrgUnitListItems(items, "closed", "")
	if len(byKeyword) != 1 || byKeyword[0].OrgCode != "C003" {
		t.Fatalf("keyword rows=%+v", byKeyword)
	}
}

func TestSortOrgUnitListItems(t *testing.T) {
	items := []orgUnitListItem{
		{OrgCode: "B002", Name: "Beta", Status: "disabled"},
		{OrgCode: "A001", Name: "Alpha", Status: "active"},
		{OrgCode: "C003", Name: "Alpha", Status: "active"},
	}

	sortOrgUnitListItems(items, orgUnitListSortName, orgUnitListSortOrderAsc)
	if items[0].OrgCode != "A001" || items[1].OrgCode != "C003" {
		t.Fatalf("name asc order=%+v", items)
	}

	sortOrgUnitListItems(items, orgUnitListSortStatus, orgUnitListSortOrderDesc)
	if items[0].Status != "disabled" {
		t.Fatalf("status desc order=%+v", items)
	}

	sortOrgUnitListItems(items, "", "")
	if len(items) != 3 {
		t.Fatalf("unexpected rows=%+v", items)
	}
}

func TestListOrgUnitListPage(t *testing.T) {
	t.Run("pager store", func(t *testing.T) {
		store := &orgUnitListPageReaderStore{
			resolveOrgCodeStore: &resolveOrgCodeStore{},
			items:               []orgUnitListItem{{OrgCode: "A001", Name: "Root", Status: "active"}},
			total:               1,
		}
		items, total, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf: "2026-01-01",
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if total != 1 || len(items) != 1 || items[0].OrgCode != "A001" {
			t.Fatalf("items=%+v total=%d", items, total)
		}
	})

	t.Run("children and paging", func(t *testing.T) {
		parentID := 10000001
		store := &resolveOrgCodeStore{
			listChildren: []OrgUnitChild{
				{OrgID: 10000002, OrgCode: "A002", Name: "Two", Status: "", IsBusinessUnit: true, HasChildren: true},
				{OrgID: 10000003, OrgCode: "A003", Name: "Three", Status: "disabled"},
			},
		}
		items, total, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf:      "2026-01-01",
			ParentID:  &parentID,
			SortField: orgUnitListSortCode,
			SortOrder: orgUnitListSortOrderAsc,
			Limit:     1,
			Offset:    -1,
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if total != 2 || len(items) != 1 {
			t.Fatalf("items=%+v total=%d", items, total)
		}
		if items[0].Status != "active" {
			t.Fatalf("status=%q", items[0].Status)
		}
		if items[0].IsBusinessUnit == nil || !*items[0].IsBusinessUnit {
			t.Fatalf("is_business_unit missing")
		}
		if items[0].HasChildren == nil || !*items[0].HasChildren {
			t.Fatalf("has_children missing")
		}

		empty, totalAfter, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf:     "2026-01-01",
			ParentID: &parentID,
			Limit:    1,
			Offset:   5,
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if totalAfter != 2 || len(empty) != 0 {
			t.Fatalf("unexpected empty=%+v total=%d", empty, totalAfter)
		}

		clamped, totalClamped, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf:     "2026-01-01",
			ParentID: &parentID,
			Limit:    5,
			Offset:   0,
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if totalClamped != 2 || len(clamped) != 2 {
			t.Fatalf("unexpected clamped=%+v total=%d", clamped, totalClamped)
		}
	})

	t.Run("roots and errors", func(t *testing.T) {
		store := &resolveOrgCodeStore{
			listNodes: []OrgUnitNode{
				{OrgCode: "Z009", Name: "Zero", Status: "", HasChildren: true},
				{OrgCode: "B002", Name: "Beta", Status: "disabled", HasChildren: false},
				{OrgCode: "A001", Name: "Alpha", Status: "active", HasChildren: false},
			},
		}
		items, total, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf:    "2026-01-01",
			Keyword: "Alpha",
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if total != 1 || len(items) != 1 || items[0].OrgCode != "A001" {
			t.Fatalf("items=%+v total=%d", items, total)
		}

		roots, totalRoots, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf: "2026-01-01",
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if totalRoots != 3 || len(roots) != 3 || roots[0].Status != orgUnitListStatusActive {
			t.Fatalf("roots=%+v total=%d", roots, totalRoots)
		}
		if roots[0].HasChildren == nil || !*roots[0].HasChildren {
			t.Fatalf("root has_children missing: %+v", roots[0])
		}

		errStore := &resolveOrgCodeStore{listNodesErr: errBoom{}}
		if _, _, err := listOrgUnitListPage(context.Background(), errStore, "t1", orgUnitListPageRequest{AsOf: "2026-01-01"}); err == nil {
			t.Fatalf("expected list nodes error")
		}

		childErrStore := &resolveOrgCodeStore{listChildrenErr: errBoom{}}
		parentID := 10000001
		if _, _, err := listOrgUnitListPage(context.Background(), childErrStore, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ParentID: &parentID}); err == nil {
			t.Fatalf("expected list children error")
		}
	})
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
		listNodes: []OrgUnitNode{{ID: "10000001", OrgCode: "A001", Name: "Root", HasChildren: true}},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	var payload struct {
		OrgUnits []orgUnitListItem `json:"org_units"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if len(payload.OrgUnits) != 1 || payload.OrgUnits[0].HasChildren == nil || !*payload.OrgUnits[0].HasChildren {
		t.Fatalf("unexpected roots payload: %+v", payload.OrgUnits)
	}
}

func TestHandleOrgUnitsDetailsAPI_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatalf("create err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code="+created.OrgCode+"&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsDetailsAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), created.OrgCode) {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsDetailsAPI_BasicErrors(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=bad%7F", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsDetailsAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001", nil)
	req2 = req2.WithContext(withTenant(req2.Context(), Tenant{ID: "t1", Name: "T"}))
	rec2 := httptest.NewRecorder()
	handleOrgUnitsDetailsAPI(rec2, req2, newOrgUnitMemoryStore())
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec2.Code)
	}
}

func TestHandleOrgUnitsVersionsAPI_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatalf("create err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/versions?org_code="+created.OrgCode, nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsVersionsAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "2026-01-01") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsAuditAPI_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatalf("create err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?org_code="+created.OrgCode+"&limit=1", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAuditAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"events\"") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsSearchAPI_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatalf("create err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=A001&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsSearchAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"target_org_code\":\"A001\"") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsSearchAPI_BasicErrors(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsSearchAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=A001&as_of=2026-01-01", nil)
	req2 = req2.WithContext(withTenant(req2.Context(), Tenant{ID: "t1", Name: "T"}))
	rec2 := httptest.NewRecorder()
	handleOrgUnitsSearchAPI(rec2, req2, newOrgUnitMemoryStore())
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec2.Code)
	}
}

func TestHandleOrgUnitsDetailsAPI_ErrorBranches(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitsDetailsAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/details?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsDetailsAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001&as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsDetailsAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("org_code required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsDetailsAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsDetailsAPI(rec, req, &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeInvalid})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve internal error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsDetailsAPI(rec, req, &resolveOrgCodeStore{resolveErr: errBoom{}})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("details not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsDetailsAPI(rec, req, &resolveOrgCodeStore{
			resolveID:         10000001,
			getNodeDetailsErr: errOrgUnitNotFound,
		})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("details internal error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsDetailsAPI(rec, req, &resolveOrgCodeStore{
			resolveID:         10000001,
			getNodeDetailsErr: errBoom{},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("ext store missing interface", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsDetailsAPI(rec, req, &resolveOrgCodeStore{
			resolveID: 10000001,
			getNodeDetails: OrgUnitNodeDetails{
				OrgID:   10000001,
				OrgCode: "A001",
				Name:    "Root",
				Status:  "active",
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ext fields not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		store := &orgUnitDetailsExtStoreStub{
			resolveOrgCodeStore: &resolveOrgCodeStore{
				resolveID: 10000001,
				getNodeDetails: OrgUnitNodeDetails{
					OrgID:   10000001,
					OrgCode: "A001",
					Name:    "Root",
					Status:  "active",
				},
			},
			cfgs:        []orgUnitTenantFieldConfig{{FieldKey: "org_type", PhysicalCol: "ext_str_01"}},
			snapshotErr: errOrgUnitNotFound,
		}
		handleOrgUnitsDetailsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ext fields internal error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		store := &orgUnitDetailsExtStoreStub{
			resolveOrgCodeStore: &resolveOrgCodeStore{
				resolveID: 10000001,
				getNodeDetails: OrgUnitNodeDetails{
					OrgID:   10000001,
					OrgCode: "A001",
					Name:    "Root",
					Status:  "active",
				},
			},
			cfgErr: errors.New("cfg"),
		}
		handleOrgUnitsDetailsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleOrgUnitsVersionsAPI_ErrorBranches(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/versions?org_code=A001", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitsVersionsAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/versions?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsVersionsAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("org_code required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/versions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsVersionsAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("org_code invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/versions?org_code=bad%7F", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsVersionsAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/versions?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsVersionsAPI(rec, req, &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeInvalid})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/versions?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsVersionsAPI(rec, req, &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeNotFound})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve internal error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/versions?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsVersionsAPI(rec, req, &resolveOrgCodeStore{resolveErr: errBoom{}})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("versions not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/versions?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsVersionsAPI(rec, req, &resolveOrgCodeStore{
			resolveID:           10000001,
			listNodeVersionsErr: errOrgUnitNotFound,
		})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("versions internal error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/versions?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsVersionsAPI(rec, req, &resolveOrgCodeStore{
			resolveID:           10000001,
			listNodeVersionsErr: errBoom{},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleOrgUnitsAuditAPI_ErrorBranches(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?org_code=A001", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitsAuditAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/audit?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAuditAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("org_code required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAuditAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("org_code invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?org_code=bad%7F", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAuditAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAuditAPI(rec, req, &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeInvalid})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAuditAPI(rec, req, &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeNotFound})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve internal error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAuditAPI(rec, req, &resolveOrgCodeStore{resolveErr: errBoom{}})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("audit not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAuditAPI(rec, req, &resolveOrgCodeStore{
			resolveID:      10000001,
			auditEventsErr: errOrgUnitNotFound,
		})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("audit internal error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAuditAPI(rec, req, &resolveOrgCodeStore{
			resolveID:      10000001,
			auditEventsErr: errBoom{},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleOrgUnitsAuditAPI_HasMore(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?org_code=A001&limit=1", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAuditAPI(rec, req, &resolveOrgCodeStore{
		resolveID: 10000001,
		auditEvents: []OrgUnitNodeAuditEvent{
			{EventID: 1, EventUUID: "e1", EventType: "RENAME", EffectiveDate: "2026-01-01"},
			{EventID: 2, EventUUID: "e2", EventType: "MOVE", EffectiveDate: "2026-01-02"},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"has_more":true`) {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsSearchAPI_ErrorBranches(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=A001", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitsSearchAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/search?query=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsSearchAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=A001&as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsSearchAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("search internal error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=A001&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsSearchAPI(rec, req, &resolveOrgCodeStore{searchNodeErr: errBoom{}})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve path org codes error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=A001&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsSearchAPI(rec, req, &resolveOrgCodeStore{
			searchNodeResult: OrgUnitSearchResult{
				TargetOrgID:   10000001,
				TargetOrgCode: "A001",
				TargetName:    "Root",
				PathOrgIDs:    []int{10000001},
				TreeAsOf:      "2026-01-01",
			},
			resolveCodesErr: errBoom{},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
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

func TestHandleOrgUnitsAPI_CreateRejectsExtLabelsSnapshot(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		createFn: func(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			t.Fatalf("create should not be called when ext_labels_snapshot is provided")
			return orgunittypes.OrgUnitResult{}, nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", strings.NewReader(`{"org_code":"A001","name":"Root","effective_date":"2026-01-01","ext_labels_snapshot":{"org_type":"Department"}}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
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
			if req.Ext["org_type"] != "DEPARTMENT" {
				t.Fatalf("ext=%v", req.Ext)
			}
			return nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","new_name":"New","ext":{"org_type":"DEPARTMENT"}}`)
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
			if req.Ext["org_type"] != "DEPARTMENT" {
				t.Fatalf("ext=%v", req.Ext)
			}
			return nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","new_parent_org_code":"A0001","effective_date":"2026-01-01","ext":{"org_type":"DEPARTMENT"}}`)
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
			if req.Ext["org_type"] != "DEPARTMENT" {
				t.Fatalf("ext=%v", req.Ext)
			}
			return nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","ext":{"org_type":"DEPARTMENT"}}`)
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

func TestHandleOrgUnitsEnableAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		enableFn: func(_ context.Context, _ string, req orgunitservices.EnableOrgUnitRequest) error {
			called = true
			if req.OrgCode == "" {
				t.Fatalf("expected org code")
			}
			if req.Ext["org_type"] != "DEPARTMENT" {
				t.Fatalf("ext=%v", req.Ext)
			}
			return nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","ext":{"org_type":"DEPARTMENT"}}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/enable", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsEnableAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !called {
		t.Fatalf("expected enable call")
	}
}

func TestHandleOrgUnitsEnableAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/enable", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsEnableAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAppendActionAPIs_RejectExtLabelsSnapshot(t *testing.T) {
	tests := []struct {
		name string
		url  string
		body string
		call func(http.ResponseWriter, *http.Request, orgunitservices.OrgUnitWriteService)
	}{
		{
			name: "rename",
			url:  "/org/api/org-units/rename",
			body: `{"org_code":"A001","new_name":"New","effective_date":"2026-01-01","ext_labels_snapshot":{"org_type":"Department"}}`,
			call: handleOrgUnitsRenameAPI,
		},
		{
			name: "move",
			url:  "/org/api/org-units/move",
			body: `{"org_code":"A001","new_parent_org_code":"A0001","effective_date":"2026-01-01","ext_labels_snapshot":{"org_type":"Department"}}`,
			call: handleOrgUnitsMoveAPI,
		},
		{
			name: "disable",
			url:  "/org/api/org-units/disable",
			body: `{"org_code":"A001","effective_date":"2026-01-01","ext_labels_snapshot":{"org_type":"Department"}}`,
			call: handleOrgUnitsDisableAPI,
		},
		{
			name: "enable",
			url:  "/org/api/org-units/enable",
			body: `{"org_code":"A001","effective_date":"2026-01-01","ext_labels_snapshot":{"org_type":"Department"}}`,
			call: handleOrgUnitsEnableAPI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.url, strings.NewReader(tt.body))
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			tt.call(rec, req, orgUnitWriteServiceStub{})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
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
			if req.Patch.Ext["org_type"] != "DEPARTMENT" {
				t.Fatalf("expected ext payload forwarded, got=%v", req.Patch.Ext)
			}
			return orgunittypes.OrgUnitResult{
				OrgCode:       "A001",
				EffectiveDate: "2026-01-01",
				Fields:        map[string]any{"name": "New Name"},
			}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","patch":{"name":"New Name","ext":{"org_type":"DEPARTMENT"}},"request_id":"r1"}`)
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

func TestHandleOrgUnitsCorrectionsAPI_RejectClientExtLabelsSnapshot(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		correctFn: func(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			t.Fatalf("service should not be called")
			return orgunittypes.OrgUnitResult{}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","patch":{"ext":{"org_type":"DEPARTMENT"},"ext_labels_snapshot":{"org_type":"Department"}},"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), orgUnitErrPatchFieldNotAllowed) {
		t.Fatalf("body=%q", rec.Body.String())
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

func TestHandleOrgUnitsStatusCorrectionsAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		correctStatusFn: func(_ context.Context, tenantID string, req orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			called = true
			if tenantID != "t1" {
				t.Fatalf("tenant=%s", tenantID)
			}
			if req.OrgCode != "A001" || req.TargetEffectiveDate != "2026-01-01" || req.TargetStatus != "disabled" || req.RequestID != "r1" {
				t.Fatalf("unexpected request: %+v", req)
			}
			return orgunittypes.OrgUnitResult{OrgCode: "A001", EffectiveDate: "2026-01-01", Fields: map[string]any{"target_status": "disabled"}}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","target_status":"disabled","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/status-corrections", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsStatusCorrectionsAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatalf("expected correct status call")
	}
	if !strings.Contains(rec.Body.String(), "A001") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsStatusCorrectionsAPI_BasicErrors(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/status-corrections", strings.NewReader(`{"org_code":"A001"}`))
		rec := httptest.NewRecorder()
		handleOrgUnitsStatusCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/status-corrections", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsStatusCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/status-corrections", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsStatusCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("service missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/status-corrections", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","target_status":"disabled","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsStatusCorrectionsAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("status correction unsupported", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{correctStatusFn: func(context.Context, string, orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/status-corrections", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","target_status":"disabled","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsStatusCorrectionsAPI(rec, req, svc)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleOrgUnitsRescindsAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		rescindRecordFn: func(_ context.Context, tenantID string, req orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			called = true
			if tenantID != "t1" {
				t.Fatalf("tenant=%s", tenantID)
			}
			if req.OrgCode != "A001" || req.TargetEffectiveDate != "2026-01-01" || req.RequestID != "r1" || req.Reason != "bad" {
				t.Fatalf("unexpected request: %+v", req)
			}
			return orgunittypes.OrgUnitResult{OrgCode: "A001", EffectiveDate: "2026-01-01"}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","request_id":"r1","reason":"bad"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRescindsAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatalf("expected rescind call")
	}
	if !strings.Contains(rec.Body.String(), "RESCIND_EVENT") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsRescindsAPI_BasicErrors(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", strings.NewReader(`{"org_code":"A001"}`))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/rescinds", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("service missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", strings.NewReader(`{"org_code":"A001"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{rescindRecordFn: func(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_EVENT_NOT_FOUND")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","request_id":"r1","reason":"bad"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, svc)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("request id conflict", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{rescindRecordFn: func(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_REQUEST_ID_CONFLICT")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","request_id":"r1","reason":"bad"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, svc)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

}

func TestHandleOrgUnitsRescindsOrgAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		rescindOrgFn: func(_ context.Context, tenantID string, req orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			called = true
			if tenantID != "t1" {
				t.Fatalf("tenant=%s", tenantID)
			}
			if req.OrgCode != "A001" || req.RequestID != "r2" || req.Reason != "bad-org" {
				t.Fatalf("unexpected request: %+v", req)
			}
			return orgunittypes.OrgUnitResult{OrgCode: "A001", Fields: map[string]any{"rescinded_events": int64(3)}}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","request_id":"r2","reason":"bad-org"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRescindsOrgAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatalf("expected rescind org call")
	}
	if !strings.Contains(rec.Body.String(), "\"rescinded_events\":3") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsRescindsOrgAPI_RescindedEventsFieldTypes(t *testing.T) {
	cases := []struct {
		name  string
		field any
		want  string
	}{
		{name: "int", field: 3, want: `"rescinded_events":3`},
		{name: "int32", field: int32(4), want: `"rescinded_events":4`},
		{name: "int64", field: int64(5), want: `"rescinded_events":5`},
		{name: "float64", field: float64(6), want: `"rescinded_events":6`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := orgUnitWriteServiceStub{
				rescindOrgFn: func(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
					return orgunittypes.OrgUnitResult{OrgCode: "A001", Fields: map[string]any{"rescinded_events": tc.field}}, nil
				},
			}
			req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001","request_id":"r2","reason":"bad-org"}`))
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			handleOrgUnitsRescindsOrgAPI(rec, req, svc)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("body=%q", rec.Body.String())
			}
		})
	}
}

func TestHandleOrgUnitsRescindsOrgAPI_BasicErrors(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001"}`))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/rescinds/org", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("service missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{rescindOrgFn: func(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_ROOT_DELETE_FORBIDDEN")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001","request_id":"r2","reason":"bad-org"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, svc)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("has children conflict", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{rescindOrgFn: func(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_HAS_CHILDREN_CANNOT_DELETE")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001","request_id":"r2","reason":"bad-org"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, svc)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("has dependencies conflict", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{rescindOrgFn: func(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_HAS_DEPENDENCIES_CANNOT_DELETE")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001","request_id":"r2","reason":"bad-org"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, svc)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestWriteOrgUnitServiceError_StatusMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"not_found", errors.New("ORG_CODE_NOT_FOUND"), http.StatusNotFound},
		{"bad_request_id", errors.New("ORG_CODE_INVALID"), http.StatusBadRequest},
		{"conflict", errors.New("EVENT_DATE_CONFLICT"), http.StatusConflict},
		{"request_id_conflict", errors.New("ORG_REQUEST_ID_CONFLICT"), http.StatusConflict},
		{"request_id_conflict_pg_message", &pgconn.PgError{Message: "ORG_REQUEST_ID_CONFLICT"}, http.StatusConflict},
		{"status_correction_unsupported", errors.New("ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET"), http.StatusConflict},
		{"orgunit_codes_write_forbidden_pg_message", &pgconn.PgError{Message: "ORGUNIT_CODES_WRITE_FORBIDDEN"}, http.StatusUnprocessableEntity},
		{"field_policy_version_required", errors.New("FIELD_POLICY_VERSION_REQUIRED"), http.StatusBadRequest},
		{"field_policy_version_stale", errors.New("FIELD_POLICY_VERSION_STALE"), http.StatusConflict},
		{"high_risk_reorder", errors.New("ORG_HIGH_RISK_REORDER_FORBIDDEN"), http.StatusConflict},
		{"root_delete_forbidden", errors.New("ORG_ROOT_DELETE_FORBIDDEN"), http.StatusConflict},
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

func TestOrgUnitAPIStatusForCode_Branches(t *testing.T) {
	tests := []struct {
		code   string
		status int
		ok     bool
	}{
		{code: orgUnitErrCodeInvalid, status: http.StatusBadRequest, ok: true},
		{code: orgUnitErrCodeNotFound, status: http.StatusNotFound, ok: true},
		{code: orgUnitErrRequestIDConflict, status: http.StatusConflict, ok: true},
		{code: orgUnitErrFieldPolicyMissing, status: http.StatusUnprocessableEntity, ok: true},
		{code: "UNKNOWN_CODE", status: 0, ok: false},
	}
	for _, tt := range tests {
		status, ok := orgUnitAPIStatusForCode(tt.code)
		if status != tt.status || ok != tt.ok {
			t.Fatalf("code=%s status=%d ok=%v", tt.code, status, ok)
		}
	}
}

func TestWriteOrgUnitServiceError_UsesStablePgMessageCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", nil)
	rec := httptest.NewRecorder()

	writeOrgUnitServiceError(rec, req, &pgconn.PgError{Message: "ORGUNIT_CODES_WRITE_FORBIDDEN", Code: "P0001"}, "orgunit_correct_failed")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got := payload["code"]; got != "ORGUNIT_CODES_WRITE_FORBIDDEN" {
		t.Fatalf("code=%v", got)
	}
}

func TestWriteOrgUnitServiceError_BadRequestStableUnknownCodePreserved(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", nil)
	rec := httptest.NewRecorder()

	writeOrgUnitServiceError(rec, req, newBadRequestError("SOME_DB_CODE"), "orgunit_correct_failed")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got := payload["code"]; got != "SOME_DB_CODE" {
		t.Fatalf("code=%v", got)
	}
	if got := payload["message"]; got != "Some DB code." {
		t.Fatalf("message=%v", got)
	}
}

func TestWriteOrgUnitServiceError_BlankCodeFallsBackToDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", nil)
	rec := httptest.NewRecorder()

	writeOrgUnitServiceError(rec, req, errors.New("   "), "orgunit_correct_failed")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got := payload["code"]; got != "orgunit_correct_failed" {
		t.Fatalf("code=%v", got)
	}
	if got := payload["message"]; got != "Orgunit correct failed." {
		t.Fatalf("message=%v", got)
	}
}

type orgUnitWriteServiceStub struct {
	writeFn           func(context.Context, string, orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error)
	createFn          func(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
	renameFn          func(context.Context, string, orgunitservices.RenameOrgUnitRequest) error
	moveFn            func(context.Context, string, orgunitservices.MoveOrgUnitRequest) error
	disableFn         func(context.Context, string, orgunitservices.DisableOrgUnitRequest) error
	enableFn          func(context.Context, string, orgunitservices.EnableOrgUnitRequest) error
	setBusinessUnitFn func(context.Context, string, orgunitservices.SetBusinessUnitRequest) error
	correctFn         func(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
	correctStatusFn   func(context.Context, string, orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
	rescindRecordFn   func(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
	rescindOrgFn      func(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
}

func (s orgUnitWriteServiceStub) Write(ctx context.Context, tenantID string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
	if s.writeFn == nil {
		return orgunitservices.OrgUnitWriteResult{}, nil
	}
	return s.writeFn(ctx, tenantID, req)
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

func (s orgUnitWriteServiceStub) Enable(ctx context.Context, tenantID string, req orgunitservices.EnableOrgUnitRequest) error {
	if s.enableFn == nil {
		return nil
	}
	return s.enableFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) SetBusinessUnit(ctx context.Context, tenantID string, req orgunitservices.SetBusinessUnitRequest) error {
	if s.setBusinessUnitFn == nil {
		return nil
	}
	return s.setBusinessUnitFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) Correct(ctx context.Context, tenantID string, req orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	if s.correctFn == nil {
		return orgunittypes.OrgUnitResult{}, nil
	}
	return s.correctFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) CorrectStatus(ctx context.Context, tenantID string, req orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	if s.correctStatusFn == nil {
		return orgunittypes.OrgUnitResult{}, nil
	}
	return s.correctStatusFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) RescindRecord(ctx context.Context, tenantID string, req orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	if s.rescindRecordFn == nil {
		return orgunittypes.OrgUnitResult{}, nil
	}
	return s.rescindRecordFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) RescindOrg(ctx context.Context, tenantID string, req orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	if s.rescindOrgFn == nil {
		return orgunittypes.OrgUnitResult{}, nil
	}
	return s.rescindOrgFn(ctx, tenantID, req)
}
