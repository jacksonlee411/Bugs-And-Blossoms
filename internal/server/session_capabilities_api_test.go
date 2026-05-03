package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type sessionCapabilitiesAuthorizerStub struct {
	allowed     map[string]bool
	notEnforced bool
	err         error
}

func (a sessionCapabilitiesAuthorizerStub) CapabilitiesForPrincipal(context.Context, string, string) ([]string, error) {
	if a.err != nil {
		return nil, a.err
	}
	if a.notEnforced {
		return nil, nil
	}
	out := make([]string, 0, len(a.allowed))
	for key, allowed := range a.allowed {
		if allowed {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (a sessionCapabilitiesAuthorizerStub) AuthorizePrincipal(context.Context, string, string, string, string) (bool, error) {
	return false, nil
}

func (a sessionCapabilitiesAuthorizerStub) OrgScopesForPrincipal(context.Context, string, string, string) ([]principalOrgScope, error) {
	return nil, nil
}

func (a sessionCapabilitiesAuthorizerStub) ListRoleDefinitions(context.Context, string) ([]authzRoleDefinition, error) {
	return nil, nil
}

func (a sessionCapabilitiesAuthorizerStub) GetRoleDefinition(context.Context, string, string) (authzRoleDefinition, bool, error) {
	return authzRoleDefinition{}, false, nil
}

func (a sessionCapabilitiesAuthorizerStub) CreateRoleDefinition(context.Context, string, saveAuthzRoleDefinitionInput) (authzRoleDefinition, error) {
	return authzRoleDefinition{}, nil
}

func (a sessionCapabilitiesAuthorizerStub) UpdateRoleDefinition(context.Context, string, string, saveAuthzRoleDefinitionInput) (authzRoleDefinition, error) {
	return authzRoleDefinition{}, nil
}

func (a sessionCapabilitiesAuthorizerStub) GetPrincipalAssignment(context.Context, string, string) (principalAuthzAssignment, bool, error) {
	return principalAuthzAssignment{}, false, nil
}

func (a sessionCapabilitiesAuthorizerStub) ReplacePrincipalAssignment(context.Context, string, string, replacePrincipalAssignmentInput) (principalAuthzAssignment, error) {
	return principalAuthzAssignment{}, nil
}

func TestHandleSessionCapabilitiesAPI_ReturnsAllowedCanonicalKeys(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/iam/api/me/capabilities", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a", Domain: "localhost", Name: "Tenant A"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "tenant-a", RoleSlug: authz.RoleTenantViewer, Status: "active"}))
	rec := httptest.NewRecorder()

	handleSessionCapabilitiesAPI(rec, req, sessionCapabilitiesAuthorizerStub{
		allowed: map[string]bool{
			"cubebox.conversations:read": true,
			"cubebox.conversations:use":  true,
			"orgunit.orgunits:read":      true,
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload sessionCapabilitiesResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"cubebox.conversations:read",
		"cubebox.conversations:use",
		"orgunit.orgunits:read",
	}
	if len(payload.AuthzCapabilityKeys) != len(want) {
		t.Fatalf("authz_capability_keys=%v", payload.AuthzCapabilityKeys)
	}
	for i := range want {
		if payload.AuthzCapabilityKeys[i] != want[i] {
			t.Fatalf("authz_capability_keys=%v", payload.AuthzCapabilityKeys)
		}
	}
}

func TestHandleSessionCapabilitiesAPI_NonEnforcedModeDoesNotGrantCapabilities(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/iam/api/me/capabilities", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a", Domain: "localhost", Name: "Tenant A"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "tenant-a", RoleSlug: authz.RoleTenantViewer, Status: "active"}))
	rec := httptest.NewRecorder()

	handleSessionCapabilitiesAPI(rec, req, sessionCapabilitiesAuthorizerStub{notEnforced: true})

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload sessionCapabilitiesResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.AuthzCapabilityKeys) != 0 {
		t.Fatalf("authz_capability_keys=%v", payload.AuthzCapabilityKeys)
	}
}

func TestHandleSessionCapabilitiesAPI_MissingPrincipal(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/iam/api/me/capabilities", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a", Domain: "localhost", Name: "Tenant A"}))
	rec := httptest.NewRecorder()

	handleSessionCapabilitiesAPI(rec, req, sessionCapabilitiesAuthorizerStub{})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSessionCapabilitiesAPI_AuthorizerError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/iam/api/me/capabilities", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a", Domain: "localhost", Name: "Tenant A"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "tenant-a", RoleSlug: authz.RoleTenantAdmin, Status: "active"}))
	rec := httptest.NewRecorder()

	handleSessionCapabilitiesAPI(rec, req, sessionCapabilitiesAuthorizerStub{err: errors.New("boom")})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
