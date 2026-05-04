package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type principalListStoreStub struct {
	items []Principal
	err   error
}

func (*principalListStoreStub) GetOrCreateTenantAdmin(context.Context, string) (Principal, error) {
	panic("unused")
}

func (*principalListStoreStub) UpsertFromKratos(context.Context, string, string, string, string, string) (Principal, error) {
	panic("unused")
}

func (*principalListStoreStub) GetByID(context.Context, string, string) (Principal, bool, error) {
	panic("unused")
}

func (s *principalListStoreStub) ListActive(context.Context, string) ([]Principal, error) {
	return append([]Principal(nil), s.items...), s.err
}

func TestHandlePrincipalAuthzAssignmentGetAPI_ListCandidatesWithoutPrincipalID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/iam/api/authz/user-assignments", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a", Domain: "localhost", Name: "Tenant A"}))
	rec := httptest.NewRecorder()

	handlePrincipalAuthzAssignmentGetAPI(rec, req, newMemoryAuthzRuntimeStore(), &principalListStoreStub{
		items: []Principal{
			{ID: "principal-a", TenantID: "tenant-a", Email: "user-a@example.invalid", DisplayName: "User A", Status: "active"},
			{ID: "principal-b", TenantID: "tenant-a", Email: "user-b@example.invalid", Status: "active"},
		},
	}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload principalAssignmentCandidatesResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Principals) != 2 {
		t.Fatalf("principals=%+v", payload.Principals)
	}
	if payload.Principals[0].PrincipalID != "principal-a" || payload.Principals[0].Email != "user-a@example.invalid" || payload.Principals[0].DisplayName != "User A" {
		t.Fatalf("principal[0]=%+v", payload.Principals[0])
	}
	if payload.Principals[1].PrincipalID != "principal-b" || payload.Principals[1].Email != "user-b@example.invalid" || payload.Principals[1].DisplayName != "" {
		t.Fatalf("principal[1]=%+v", payload.Principals[1])
	}
}

func TestHandlePrincipalAuthzAssignmentGetAPI_ReadsAssignmentWithPrincipalID(t *testing.T) {
	store := newMemoryAuthzRuntimeStore()
	_, err := store.CreateRoleDefinition(context.Background(), "tenant-a", saveAuthzRoleDefinitionInput{
		RoleSlug:            "dict-reader",
		Name:                "Dict Reader",
		AuthzCapabilityKeys: []string{"iam.dicts:read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := store.ReplacePrincipalAssignment(context.Background(), "tenant-a", "principal-a", replacePrincipalAssignmentInput{
		Roles:    []string{"dict-reader"},
		Revision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/iam/api/authz/user-assignments?principal_id=principal-a", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a", Domain: "localhost", Name: "Tenant A"}))
	rec := httptest.NewRecorder()

	handlePrincipalAuthzAssignmentGetAPI(rec, req, store, &principalListStoreStub{}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload principalAuthzAssignmentResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.PrincipalID != "principal-a" || payload.Revision != assignment.Revision || len(payload.Roles) != 1 {
		t.Fatalf("payload=%+v", payload)
	}
	if payload.Roles[0].RoleSlug != "dict-reader" || payload.Roles[0].DisplayName != "Dict Reader" {
		t.Fatalf("role=%+v", payload.Roles[0])
	}
}

func TestHandlePrincipalAuthzAssignmentPutAPI_ResolvesOrgCodeBeforeSave(t *testing.T) {
	store := newMemoryAuthzRuntimeStore()
	rootOrgNodeKey := mustOrgNodeKeyForTest(t, 10000000)
	if _, err := store.ReplacePrincipalAssignment(context.Background(), "tenant-a", "actor-a", replacePrincipalAssignmentInput{
		Roles:     []string{authz.RoleTenantAdmin},
		OrgScopes: []principalOrgScope{{OrgNodeKey: rootOrgNodeKey, IncludeDescendants: true}},
		Revision:  1,
	}); err != nil {
		t.Fatal(err)
	}
	reqBody := bytes.NewBufferString(`{
		"roles": [{"role_slug": "tenant-viewer"}],
		"org_scopes": [{"org_code": "flowers", "include_descendants": true}],
		"revision": 1
	}`)
	req := httptest.NewRequest(http.MethodPut, "/iam/api/authz/user-assignments/principal-b", reqBody)
	req = req.WithContext(withPrincipal(withTenant(req.Context(), Tenant{ID: "tenant-a", Domain: "localhost", Name: "Tenant A"}), Principal{
		ID:       "actor-a",
		TenantID: "tenant-a",
		Status:   "active",
	}))
	rec := httptest.NewRecorder()

	resolver := orgUnitCodeResolverStub{
		resolveOrgNodeKeyByCodeFn: func(_ context.Context, tenantID string, orgCode string) (string, error) {
			if tenantID != "tenant-a" || orgCode != "flowers" {
				t.Fatalf("resolve tenant=%q orgCode=%q", tenantID, orgCode)
			}
			return rootOrgNodeKey, nil
		},
		resolveOrgCodesByNodeKeysFn: func(_ context.Context, tenantID string, orgNodeKeys []string) (map[string]string, error) {
			if tenantID != "tenant-a" || len(orgNodeKeys) != 1 || orgNodeKeys[0] != rootOrgNodeKey {
				t.Fatalf("resolve codes tenant=%q keys=%v", tenantID, orgNodeKeys)
			}
			return map[string]string{rootOrgNodeKey: "FLOWERS"}, nil
		},
	}

	handlePrincipalAuthzAssignmentPutAPI(rec, req, store, resolver, store)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload principalAuthzAssignmentResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.PrincipalID != "principal-b" || payload.Revision != 2 || len(payload.Roles) != 1 || payload.Roles[0].RoleSlug != authz.RoleTenantViewer {
		t.Fatalf("payload=%+v", payload)
	}
	if len(payload.OrgScopes) != 1 {
		t.Fatalf("org scopes=%+v", payload.OrgScopes)
	}
	scope := payload.OrgScopes[0]
	if scope.OrgNodeKey != rootOrgNodeKey || scope.OrgCode != "FLOWERS" || !scope.IncludeDescendants {
		t.Fatalf("scope=%+v", scope)
	}
}

func TestHandlePrincipalAuthzAssignmentPutAPI_AllowsIAMOnlyAssignmentWithoutActorOrgScope(t *testing.T) {
	store := newMemoryAuthzRuntimeStore()
	if _, err := store.CreateRoleDefinition(context.Background(), "tenant-a", saveAuthzRoleDefinitionInput{
		RoleSlug:            "authz-delegator",
		Name:                "Authz Delegator",
		AuthzCapabilityKeys: []string{"iam.authz:admin"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReplacePrincipalAssignment(context.Background(), "tenant-a", "actor-a", replacePrincipalAssignmentInput{
		Roles:    []string{"authz-delegator"},
		Revision: 1,
	}); err != nil {
		t.Fatal(err)
	}
	reqBody := bytes.NewBufferString(`{
		"roles": [{"role_slug": "authz-delegator"}],
		"org_scopes": [],
		"revision": 1
	}`)
	req := httptest.NewRequest(http.MethodPut, "/iam/api/authz/user-assignments/principal-b", reqBody)
	req = req.WithContext(withPrincipal(withTenant(req.Context(), Tenant{ID: "tenant-a", Domain: "localhost", Name: "Tenant A"}), Principal{
		ID:       "actor-a",
		TenantID: "tenant-a",
		Status:   "active",
	}))
	rec := httptest.NewRecorder()

	handlePrincipalAuthzAssignmentPutAPI(rec, req, store, nil, store)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload principalAuthzAssignmentResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.PrincipalID != "principal-b" || payload.Revision != 2 || len(payload.Roles) != 1 || payload.Roles[0].RoleSlug != "authz-delegator" {
		t.Fatalf("payload=%+v", payload)
	}
	if len(payload.OrgScopes) != 0 {
		t.Fatalf("org scopes=%+v", payload.OrgScopes)
	}
}

func TestHandlePrincipalAuthzAssignmentPutAPI_FailsClosedWhenActorScopeDoesNotAllowRequestedOrg(t *testing.T) {
	store := newMemoryAuthzRuntimeStore()
	flowersOrgNodeKey := mustOrgNodeKeyForTest(t, 10000001)
	bugsOrgNodeKey := mustOrgNodeKeyForTest(t, 10000002)
	if _, err := store.ReplacePrincipalAssignment(context.Background(), "tenant-a", "actor-a", replacePrincipalAssignmentInput{
		Roles:     []string{authz.RoleTenantAdmin},
		OrgScopes: []principalOrgScope{{OrgNodeKey: flowersOrgNodeKey, IncludeDescendants: true}},
		Revision:  1,
	}); err != nil {
		t.Fatal(err)
	}
	reqBody := bytes.NewBufferString(`{
		"roles": [{"role_slug": "tenant-viewer"}],
		"org_scopes": [{"org_code": "bugs", "include_descendants": true}],
		"revision": 1
	}`)
	req := httptest.NewRequest(http.MethodPut, "/iam/api/authz/user-assignments/principal-b", reqBody)
	req = req.WithContext(withPrincipal(withTenant(req.Context(), Tenant{ID: "tenant-a", Domain: "localhost", Name: "Tenant A"}), Principal{
		ID:       "actor-a",
		TenantID: "tenant-a",
		Status:   "active",
	}))
	rec := httptest.NewRecorder()

	resolver := orgUnitCodeResolverStub{
		resolveOrgNodeKeyByCodeFn: func(_ context.Context, tenantID string, orgCode string) (string, error) {
			if tenantID != "tenant-a" || orgCode != "bugs" {
				t.Fatalf("resolve tenant=%q orgCode=%q", tenantID, orgCode)
			}
			return bugsOrgNodeKey, nil
		},
		resolveOrgCodesByNodeKeysFn: func(_ context.Context, _ string, _ []string) (map[string]string, error) {
			t.Fatal("scope check should fail before response org code hydration")
			return nil, nil
		},
	}

	handlePrincipalAuthzAssignmentPutAPI(rec, req, store, resolver, store)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != "authz_scope_forbidden" {
		t.Fatalf("payload=%+v", payload)
	}
}

func TestHandlePrincipalAuthzAssignmentPutAPI_HidesUnknownOrgCodeAsScopeForbidden(t *testing.T) {
	store := newMemoryAuthzRuntimeStore()
	rootOrgNodeKey := mustOrgNodeKeyForTest(t, 10000000)
	if _, err := store.ReplacePrincipalAssignment(context.Background(), "tenant-a", "actor-a", replacePrincipalAssignmentInput{
		Roles:     []string{authz.RoleTenantAdmin},
		OrgScopes: []principalOrgScope{{OrgNodeKey: rootOrgNodeKey, IncludeDescendants: true}},
		Revision:  1,
	}); err != nil {
		t.Fatal(err)
	}
	reqBody := bytes.NewBufferString(`{
		"roles": [{"role_slug": "tenant-viewer"}],
		"org_scopes": [{"org_code": "missing", "include_descendants": true}],
		"revision": 1
	}`)
	req := httptest.NewRequest(http.MethodPut, "/iam/api/authz/user-assignments/principal-b", reqBody)
	req = req.WithContext(withPrincipal(withTenant(req.Context(), Tenant{ID: "tenant-a", Domain: "localhost", Name: "Tenant A"}), Principal{
		ID:       "actor-a",
		TenantID: "tenant-a",
		Status:   "active",
	}))
	rec := httptest.NewRecorder()

	resolver := orgUnitCodeResolverStub{
		resolveOrgNodeKeyByCodeFn: func(_ context.Context, tenantID string, orgCode string) (string, error) {
			if tenantID != "tenant-a" || orgCode != "missing" {
				t.Fatalf("resolve tenant=%q orgCode=%q", tenantID, orgCode)
			}
			return "", orgunitpkg.ErrOrgCodeNotFound
		},
		resolveOrgCodesByNodeKeysFn: func(_ context.Context, _ string, _ []string) (map[string]string, error) {
			t.Fatal("scope check should fail before response org code hydration")
			return nil, nil
		},
	}

	handlePrincipalAuthzAssignmentPutAPI(rec, req, store, resolver, store)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != "authz_scope_forbidden" {
		t.Fatalf("payload=%+v", payload)
	}
}

func TestHandlePrincipalAuthzAssignmentGetAPI_FailsClosedWhenOrgCodeMissing(t *testing.T) {
	store := newMemoryAuthzRuntimeStore()
	rootOrgNodeKey := mustOrgNodeKeyForTest(t, 10000000)
	_, err := store.ReplacePrincipalAssignment(context.Background(), "tenant-a", "principal-a", replacePrincipalAssignmentInput{
		Roles:     []string{authz.RoleTenantViewer},
		OrgScopes: []principalOrgScope{{OrgNodeKey: rootOrgNodeKey, IncludeDescendants: true}},
		Revision:  1,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/iam/api/authz/user-assignments?principal_id=principal-a", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a", Domain: "localhost", Name: "Tenant A"}))
	rec := httptest.NewRecorder()

	resolver := orgUnitCodeResolverStub{
		resolveOrgCodesByNodeKeysFn: func(_ context.Context, tenantID string, orgNodeKeys []string) (map[string]string, error) {
			if tenantID != "tenant-a" || len(orgNodeKeys) != 1 || orgNodeKeys[0] != rootOrgNodeKey {
				t.Fatalf("resolve codes tenant=%q keys=%v", tenantID, orgNodeKeys)
			}
			return map[string]string{}, nil
		},
	}

	handlePrincipalAuthzAssignmentGetAPI(rec, req, store, &principalListStoreStub{}, resolver)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != "org_code_not_found" {
		t.Fatalf("payload=%+v", payload)
	}
}
