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
)

type fakeOrgUnitWriteService struct {
	writeFn func(ctx context.Context, tenantID string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error)
}

func (s fakeOrgUnitWriteService) Write(ctx context.Context, tenantID string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
	if s.writeFn != nil {
		return s.writeFn(ctx, tenantID, req)
	}
	return orgunitservices.OrgUnitWriteResult{}, nil
}

func (s fakeOrgUnitWriteService) Create(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, nil
}
func (s fakeOrgUnitWriteService) Rename(context.Context, string, orgunitservices.RenameOrgUnitRequest) error {
	return nil
}
func (s fakeOrgUnitWriteService) Move(context.Context, string, orgunitservices.MoveOrgUnitRequest) error {
	return nil
}
func (s fakeOrgUnitWriteService) Disable(context.Context, string, orgunitservices.DisableOrgUnitRequest) error {
	return nil
}
func (s fakeOrgUnitWriteService) Enable(context.Context, string, orgunitservices.EnableOrgUnitRequest) error {
	return nil
}
func (s fakeOrgUnitWriteService) SetBusinessUnit(context.Context, string, orgunitservices.SetBusinessUnitRequest) error {
	return nil
}
func (s fakeOrgUnitWriteService) Correct(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, nil
}
func (s fakeOrgUnitWriteService) CorrectStatus(context.Context, string, orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, nil
}
func (s fakeOrgUnitWriteService) RescindRecord(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, nil
}
func (s fakeOrgUnitWriteService) RescindOrg(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, nil
}

func TestHandleOrgUnitsWriteAPI_BasicValidation(t *testing.T) {
	svc := fakeOrgUnitWriteService{}

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(`{}`))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("basic write accepted", func(t *testing.T) {
		body := `{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","request_id":"r1","patch":{"name":"X"}}`
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("forbid ext_labels_snapshot in request", func(t *testing.T) {
		body := `{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","request_id":"r1","patch":{"name":"X","ext_labels_snapshot":{"x":"y"}}}`
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code == http.StatusOK {
			t.Fatalf("expected error, got ok: %s", rec.Body.String())
		}
	})

	t.Run("unknown field rejected by DisallowUnknownFields", func(t *testing.T) {
		body := `{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","request_id":"r1","patch":{"name":"X"},"x":1}`
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleOrgUnitsWriteAPI_ResultAndErrorMapping(t *testing.T) {
	body := `{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","request_id":"r1","patch":{"name":"Root A"}}`

	t.Run("service nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("success envelope", func(t *testing.T) {
		svc := fakeOrgUnitWriteService{
			writeFn: func(_ context.Context, tenantID string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
				if tenantID != "t1" || req.Intent != "create_org" || req.OrgCode != "ROOT" || req.RequestID != "r1" {
					t.Fatalf("req=%+v tenant=%s", req, tenantID)
				}
				return orgunitservices.OrgUnitWriteResult{
					OrgCode:       "ROOT",
					EffectiveDate: "2026-01-01",
					EventType:     "CREATE",
					EventUUID:     "evt-1",
					Fields:        map[string]any{"name": "Root A"},
				}, nil
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if resp["event_type"] != "CREATE" || resp["event_uuid"] != "evt-1" {
			t.Fatalf("resp=%v", resp)
		}
	})

	t.Run("bad json from service is mapped", func(t *testing.T) {
		svc := fakeOrgUnitWriteService{
			writeFn: func(context.Context, string, orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
				return orgunitservices.OrgUnitWriteResult{}, errOrgUnitBadJSON
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("stable code maps to 400", func(t *testing.T) {
		svc := fakeOrgUnitWriteService{
			writeFn: func(context.Context, string, orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
				return orgunitservices.OrgUnitWriteResult{}, errors.New("PATCH_FIELD_NOT_ALLOWED")
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleOrgUnitsWriteAPI_CreateOrgSkipsNewOrgScopeCheckButChecksParent(t *testing.T) {
	store := newOrgUnitMemoryStore()
	ctx := context.Background()
	root, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "ROOT", "Root", "", true)
	if err != nil {
		t.Fatalf("create root err=%v", err)
	}
	other, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "OTHER", "Other", "", true)
	if err != nil {
		t.Fatalf("create other err=%v", err)
	}
	runtime := &orgUnitScopeRuntimeStub{scopes: []principalOrgScope{{
		OrgNodeKey:         root.ID,
		IncludeDescendants: true,
	}}}
	svc := fakeOrgUnitWriteService{
		writeFn: func(_ context.Context, tenantID string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
			if tenantID != "t1" || req.Intent != "create_org" || req.OrgCode != "NEW" {
				t.Fatalf("tenant=%s req=%+v", tenantID, req)
			}
			if req.Patch.ParentOrgCode == nil || *req.Patch.ParentOrgCode != "ROOT" {
				t.Fatalf("parent patch=%+v", req.Patch.ParentOrgCode)
			}
			return orgunitservices.OrgUnitWriteResult{
				OrgCode:       req.OrgCode,
				EffectiveDate: req.EffectiveDate,
				EventType:     "CREATE",
				EventUUID:     "evt-new",
			}, nil
		},
	}

	body := `{"intent":"create_org","org_code":"NEW","effective_date":"2026-01-01","request_id":"r1","patch":{"name":"New","parent_org_code":"ROOT"}}`
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
	req = req.WithContext(withPrincipal(withTenant(req.Context(), Tenant{ID: "t1"}), Principal{ID: "principal-a"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsWriteAPI(rec, req, svc, orgUnitScopeDeps{store: store, runtime: runtime})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	body = `{"intent":"create_org","org_code":"NEW2","effective_date":"2026-01-01","request_id":"r2","patch":{"name":"New 2","parent_org_code":"OTHER"}}`
	req = httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
	req = req.WithContext(withPrincipal(withTenant(req.Context(), Tenant{ID: "t1"}), Principal{ID: "principal-a"}))
	rec = httptest.NewRecorder()
	handleOrgUnitsWriteAPI(rec, req, svc, orgUnitScopeDeps{store: store, runtime: runtime})
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "authz_scope_forbidden") {
		t.Fatalf("status=%d body=%s other=%s", rec.Code, rec.Body.String(), other.ID)
	}
}

func TestHandleOrgUnitsWriteAPI_InvalidEffectiveDateReturns400BeforeScopeCheck(t *testing.T) {
	store := newOrgUnitMemoryStore()
	ctx := context.Background()
	root, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "ROOT", "Root", "", true)
	if err != nil {
		t.Fatalf("create root err=%v", err)
	}
	runtime := &orgUnitScopeRuntimeStub{scopes: []principalOrgScope{{
		OrgNodeKey:         root.ID,
		IncludeDescendants: true,
	}}}
	svc := fakeOrgUnitWriteService{
		writeFn: func(context.Context, string, orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
			t.Fatal("write should not be called when effective_date is invalid")
			return orgunitservices.OrgUnitWriteResult{}, nil
		},
	}

	body := `{"intent":"correct","org_code":"ROOT","effective_date":"bad","target_effective_date":"2026-01-01","request_id":"r1","patch":{"name":"Root 2"}}`
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
	req = req.WithContext(withPrincipal(withTenant(req.Context(), Tenant{ID: "t1"}), Principal{ID: "principal-a"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsWriteAPI(rec, req, svc, orgUnitScopeDeps{store: store, runtime: runtime})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), orgUnitErrEffectiveDate) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleOrgUnitsWriteAPI_CorrectScopesTargetByTargetEffectiveDateAndParentByEffectiveDate(t *testing.T) {
	targetKey, err := encodeOrgNodeKeyFromID(10000001)
	if err != nil {
		t.Fatalf("target key err=%v", err)
	}
	parentKey, err := encodeOrgNodeKeyFromID(10000002)
	if err != nil {
		t.Fatalf("parent key err=%v", err)
	}
	store := &resolveOrgCodeStore{
		resolveID: 10000001,
		resolveCodesByNodeKey: map[string]string{
			targetKey: "TARGET",
			parentKey: "PARENT",
		},
		listChildren: []OrgUnitChild{{
			OrgID:      10000002,
			OrgNodeKey: parentKey,
			OrgCode:    "PARENT",
			Name:       "Parent",
			Status:     orgUnitListStatusActive,
		}},
	}
	runtime := &orgUnitScopeRuntimeStub{scopes: []principalOrgScope{{
		OrgNodeKey:         targetKey,
		IncludeDescendants: true,
	}}}
	svc := fakeOrgUnitWriteService{
		writeFn: func(_ context.Context, tenantID string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
			if tenantID != "t1" || req.EffectiveDate != "2026-02-01" || req.TargetEffectiveDate != "2026-01-01" {
				t.Fatalf("tenant=%s req=%+v", tenantID, req)
			}
			return orgunitservices.OrgUnitWriteResult{
				OrgCode:       req.OrgCode,
				EffectiveDate: req.EffectiveDate,
				EventType:     "CORRECT_EVENT",
				EventUUID:     "evt-correct",
			}, nil
		},
	}

	body := `{"intent":"correct","org_code":"TARGET","effective_date":"2026-02-01","target_effective_date":"2026-01-01","request_id":"r1","patch":{"parent_org_code":"PARENT"}}`
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
	req = req.WithContext(withPrincipal(withTenant(req.Context(), Tenant{ID: "t1"}), Principal{ID: "principal-a"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsWriteAPI(rec, req, svc, orgUnitScopeDeps{store: store, runtime: runtime})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !sawOrgUnitDetailsLookup(store, targetKey, "2026-01-01") {
		t.Fatalf("target scope lookup missing target_effective_date; keys=%v asOf=%v", store.detailsByNodeKeyArgs, store.detailsByNodeKeyAsOfArgs)
	}
	if !sawOrgUnitDetailsLookup(store, parentKey, "2026-02-01") {
		t.Fatalf("parent scope lookup missing effective_date; keys=%v asOf=%v", store.detailsByNodeKeyArgs, store.detailsByNodeKeyAsOfArgs)
	}
}

func sawOrgUnitDetailsLookup(store *resolveOrgCodeStore, orgNodeKey string, asOf string) bool {
	for idx, key := range store.detailsByNodeKeyArgs {
		if key == orgNodeKey && idx < len(store.detailsByNodeKeyAsOfArgs) && store.detailsByNodeKeyAsOfArgs[idx] == asOf {
			return true
		}
	}
	return false
}
