package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

func TestHandleAuthzCapabilitiesAPI_DefaultFiltersCoveredAssignableTenantCapabilities(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/iam/api/authz/capabilities", nil)
	rec := httptest.NewRecorder()

	handleAuthzCapabilitiesAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload authzCapabilitiesResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.RegistryRev != authz.RegistryRevision {
		t.Fatalf("registry_rev=%q", payload.RegistryRev)
	}
	if len(payload.Capabilities) == 0 {
		t.Fatal("expected capabilities")
	}
	for _, item := range payload.Capabilities {
		if !item.Assignable || item.Surface != authz.CapabilitySurfaceTenantAPI || item.Status != authz.CapabilityStatusEnabled || !item.Covered {
			t.Fatalf("unexpected capability option: %+v", item)
		}
		if item.Key == "iam.session:admin" || item.Key == "superadmin.tenants:read" {
			t.Fatalf("non-candidate key leaked: %+v", item)
		}
	}
}

func TestHandleAuthzCapabilitiesAPI_SearchAndMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/iam/api/authz/capabilities?q=dict&owner_module=iam", nil)
	rec := httptest.NewRecorder()

	handleAuthzCapabilitiesAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload authzCapabilitiesResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Capabilities) == 0 {
		t.Fatal("expected dict capabilities")
	}
	for _, item := range payload.Capabilities {
		if item.OwnerModule != "iam" {
			t.Fatalf("unexpected owner module: %+v", item)
		}
	}

	badReq := httptest.NewRequest(http.MethodPost, "/iam/api/authz/capabilities", nil)
	badRec := httptest.NewRecorder()
	handleAuthzCapabilitiesAPI(badRec, badReq)
	if badRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", badRec.Code)
	}
}

func TestHandleAuthzCapabilitiesAPI_RejectsDiagnosticParameters(t *testing.T) {
	for _, target := range []string{
		"/iam/api/authz/capabilities?include_disabled=true",
		"/iam/api/authz/capabilities?include_uncovered=true",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()

		handleAuthzCapabilitiesAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("target=%s status=%d body=%s", target, rec.Code, rec.Body.String())
		}
	}
}
