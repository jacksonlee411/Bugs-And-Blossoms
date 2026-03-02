package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	orgunitports "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitMemoryStoreWithWriteStore struct {
	*orgUnitMemoryStore
}

func (s orgUnitMemoryStoreWithWriteStore) SubmitEvent(context.Context, string, string, *int, string, string, json.RawMessage, string, string) (int64, error) {
	return 1, nil
}

func (s orgUnitMemoryStoreWithWriteStore) SubmitCorrection(context.Context, string, int, string, json.RawMessage, string, string) (string, error) {
	return "", nil
}

func (s orgUnitMemoryStoreWithWriteStore) SubmitStatusCorrection(context.Context, string, int, string, string, string, string) (string, error) {
	return "", nil
}

func (s orgUnitMemoryStoreWithWriteStore) SubmitRescindEvent(context.Context, string, int, string, string, string, string) (string, error) {
	return "", nil
}

func (s orgUnitMemoryStoreWithWriteStore) SubmitRescindOrg(context.Context, string, int, string, string, string) (int, error) {
	return 0, nil
}

func (s orgUnitMemoryStoreWithWriteStore) FindEventByUUID(context.Context, string, string) (orgunittypes.OrgUnitEvent, error) {
	return orgunittypes.OrgUnitEvent{}, orgunitports.ErrOrgEventNotFound
}

func (s orgUnitMemoryStoreWithWriteStore) FindEventByEffectiveDate(context.Context, string, int, string) (orgunittypes.OrgUnitEvent, error) {
	return orgunittypes.OrgUnitEvent{}, orgunitports.ErrOrgEventNotFound
}

func (s orgUnitMemoryStoreWithWriteStore) ListEnabledTenantFieldConfigsAsOf(context.Context, string, string) ([]orgunittypes.TenantFieldConfig, error) {
	return nil, nil
}

func (s orgUnitMemoryStoreWithWriteStore) FindPersonByPernr(context.Context, string, string) (orgunittypes.Person, error) {
	return orgunittypes.Person{}, errors.New("person not found")
}

func TestNewHandlerWithOptions_AssistantWriteServiceFallbackForStores(t *testing.T) {
	wd := mustGetwd(t)
	t.Setenv("ALLOWLIST_PATH", mustAllowlistPathFromWd(t, wd))
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{KratosIdentityID: "00000000-0000-0000-0000-000000000001", Email: "tenant-admin@example.invalid", RoleSlug: "tenant-admin"}},
		OrgUnitStore:     &orgUnitPGStore{},
	})
	if err != nil {
		t.Fatalf("new handler failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/health", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status=%d body=%s", rec.Code, rec.Body.String())
	}

	memoryWithWrite := orgUnitMemoryStoreWithWriteStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}
	h, err = NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{KratosIdentityID: "00000000-0000-0000-0000-000000000001", Email: "tenant-admin@example.invalid", RoleSlug: "tenant-admin"}},
		OrgUnitStore:     memoryWithWrite,
	})
	if err != nil {
		t.Fatalf("new handler with write store failed: %v", err)
	}

	sid := loginAsTenantAdminForAssistantTests(t, h)
	conv := createAssistantConversationForTest(t, h, sid)
	getReq := httptest.NewRequest(http.MethodGet, "http://localhost/internal/assistant/conversations/"+conv.ConversationID, nil)
	getReq.Host = "localhost"
	getReq.AddCookie(sid)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get conversation status=%d body=%s", getRec.Code, getRec.Body.String())
	}
}

func TestOrgUnitFieldPolicyAPIItemFromStore(t *testing.T) {
	disabledOn := "2026-02-01"
	defaultExpr := "org_code == 'FLOWER'"
	updatedAt := time.Now().UTC().Truncate(time.Second)
	item := orgUnitFieldPolicyAPIItemFromStore(orgUnitTenantFieldPolicy{
		FieldKey:        "name",
		ScopeType:       "tenant",
		ScopeKey:        "*",
		Maintainable:    true,
		DefaultMode:     "CEL",
		DefaultRuleExpr: &defaultExpr,
		EnabledOn:       "2026-01-01",
		DisabledOn:      &disabledOn,
		UpdatedAt:       updatedAt,
	})
	if item.FieldKey != "name" || item.ScopeType != "tenant" || item.ScopeKey != "*" {
		t.Fatalf("unexpected basic fields: %+v", item)
	}
	if item.DefaultRuleExpr == nil || *item.DefaultRuleExpr != defaultExpr {
		t.Fatalf("unexpected default rule expr: %+v", item.DefaultRuleExpr)
	}
	if item.DisabledOn == nil || *item.DisabledOn != disabledOn {
		t.Fatalf("unexpected disabled_on: %+v", item.DisabledOn)
	}
	if !item.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected updated_at: %s != %s", item.UpdatedAt, updatedAt)
	}
}

func TestHandleOrgUnitFieldOptionsAPI_ResolveOrgIDInvalidBranch(t *testing.T) {
	base := newOrgUnitMemoryStore()
	store := orgUnitStoreWithEnabledFieldConfig{
		OrgUnitStore: orgUnitMemoryStoreResolveOrgIDErr{orgUnitMemoryStore: base, err: orgunitpkg.ErrOrgCodeInvalid},
		cfg:          orgUnitTenantFieldConfig{FieldKey: "org_code", ValueType: "text", DataSourceType: "PLAIN"},
		ok:           true,
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-options?field_key=org_code&org_code=FLOWER-A", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleOrgUnitFieldOptionsAPI(rec, req, store)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
