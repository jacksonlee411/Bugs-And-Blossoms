package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCapabilityCatalogEntryForCapabilityKey(t *testing.T) {
	entry, ok := capabilityCatalogEntryForCapabilityKey(orgUnitCreateFieldPolicyCapabilityKey)
	if !ok {
		t.Fatal("expected catalog entry")
	}
	if entry.OwnerModule != "orgunit" || entry.Module != "orgunit" {
		t.Fatalf("entry=%+v", entry)
	}
	if entry.TargetObject != "orgunit" || entry.Surface != "create_dialog" || entry.Intent != "create_org" {
		t.Fatalf("entry=%+v", entry)
	}
	if _, ok := capabilityCatalogEntryForCapabilityKey("unknown.key"); ok {
		t.Fatal("expected unknown key not found")
	}
}

func TestCapabilityCatalog_UniqueObjectIntentMapping(t *testing.T) {
	seen := make(map[string]string)
	for _, entry := range capabilityCatalogEntries {
		key := entry.TargetObject + "|" + entry.Surface + "|" + entry.Intent
		if existing, ok := seen[key]; ok && existing != entry.CapabilityKey {
			t.Fatalf("duplicate object/surface/intent mapping key=%s existing=%s new=%s", key, existing, entry.CapabilityKey)
		}
		seen[key] = entry.CapabilityKey
		if entry.Module != entry.OwnerModule {
			t.Fatalf("module mismatch entry=%+v", entry)
		}
	}
}

func TestCapabilityCatalog_CoversCapabilityDefinitionsAndRouteBindings(t *testing.T) {
	for _, definition := range capabilityDefinitions {
		if _, ok := capabilityCatalogEntryForCapabilityKey(definition.CapabilityKey); !ok {
			t.Fatalf("missing catalog entry for capability=%q", definition.CapabilityKey)
		}
	}
	for _, binding := range capabilityRouteBindings {
		if _, ok := capabilityCatalogEntryForCapabilityKey(binding.CapabilityKey); !ok {
			t.Fatalf("missing catalog entry for route binding capability=%q method=%s path=%s", binding.CapabilityKey, binding.Method, binding.Path)
		}
	}
}

func TestListCapabilityCatalog_FilterByIntent(t *testing.T) {
	items := listCapabilityCatalog(capabilityCatalogFilter{
		OwnerModule:  "orgunit",
		TargetObject: "orgunit",
		Surface:      "details_dialog",
		Intent:       "correct",
	})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got=%d", len(items))
	}
	if items[0].CapabilityKey != orgUnitCorrectFieldPolicyCapabilityKey {
		t.Fatalf("capability=%q", items[0].CapabilityKey)
	}
}

func TestListCapabilityCatalog_FilterByCapabilityKey(t *testing.T) {
	items := listCapabilityCatalog(capabilityCatalogFilter{CapabilityKey: orgUnitCreateFieldPolicyCapabilityKey})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got=%d", len(items))
	}
	if items[0].CapabilityKey != orgUnitCreateFieldPolicyCapabilityKey {
		t.Fatalf("capability=%q", items[0].CapabilityKey)
	}
	if got := listCapabilityCatalog(capabilityCatalogFilter{CapabilityKey: "unknown.capability"}); len(got) != 0 {
		t.Fatalf("expected empty result, got=%d", len(got))
	}
}

func TestBuildCapabilityCatalogEntries_Branches(t *testing.T) {
	entries := buildCapabilityCatalogEntries(
		[]capabilityDefinition{
			{CapabilityKey: "", OwnerModule: "orgunit", Status: "active"},
			{CapabilityKey: "org.orgunit_create.field_policy", OwnerModule: "orgunit", Status: "active"},
			{CapabilityKey: "org.orgunit_create.field_policy", OwnerModule: "orgunit", Status: "active"},
			{CapabilityKey: "org.orgunit_add_version.field_policy", OwnerModule: "orgunit", Status: "active"},
			{CapabilityKey: "org.not_in_metadata.field_policy", OwnerModule: "orgunit", Status: "active"},
		},
		[]capabilityRouteBinding{
			{CapabilityKey: "", RouteClass: "internal_api", Action: "noop"},
			{CapabilityKey: "org.orgunit_create.field_policy", RouteClass: "internal_api", Action: "z"},
			{CapabilityKey: "org.orgunit_create.field_policy", RouteClass: "internal_api", Action: "a"},
			{CapabilityKey: "org.orgunit_create.field_policy", RouteClass: "internal_api", Action: "a"},
		},
		map[string]capabilityCatalogMetadata{
			"org.orgunit_create.field_policy": {TargetObject: "orgunit", Surface: "create_dialog", Intent: "create_org"},
			"org.orgunit_add_version.field_policy": {
				// 与 create 使用同一 object/surface/intent，用于覆盖 fail-closed 冲突分支。
				TargetObject: "orgunit", Surface: "create_dialog", Intent: "create_org",
			},
		},
	)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got=%d", len(entries))
	}
	if entries[0].CapabilityKey != "org.orgunit_create.field_policy" {
		t.Fatalf("entries[0]=%+v", entries[0])
	}
	if len(entries[0].Actions) != 2 || entries[0].Actions[0] != "a" || entries[0].Actions[1] != "z" {
		t.Fatalf("actions=%v", entries[0].Actions)
	}
}

func TestParseCapabilityCatalogFilter_ModuleFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/internal/capabilities/catalog?module=orgunit&target_object=orgunit", nil)
	filter, ok := parseCapabilityCatalogFilter(req)
	if !ok {
		t.Fatal("expected filter parse ok")
	}
	if filter.OwnerModule != "orgunit" {
		t.Fatalf("owner_module=%q", filter.OwnerModule)
	}
}

func TestHandleCapabilityCatalogAPI(t *testing.T) {
	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal/capabilities/catalog", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleCapabilityCatalogAPI(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/capabilities/catalog", nil)
		rec := httptest.NewRecorder()
		handleCapabilityCatalogAPI(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("module mismatch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/capabilities/catalog?module=orgunit&owner_module=staffing", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleCapabilityCatalogAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("success filtered", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/capabilities/catalog?owner_module=orgunit&target_object=orgunit&surface=api_write&intent=write_all", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleCapabilityCatalogAPI(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload capabilityCatalogResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if len(payload.Items) != 1 {
			t.Fatalf("items=%d", len(payload.Items))
		}
		if payload.Items[0].CapabilityKey != orgUnitWriteFieldPolicyCapabilityKey {
			t.Fatalf("capability=%q", payload.Items[0].CapabilityKey)
		}
	})
}

func TestHandleCapabilityCatalogByIntentAPI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/internal/capabilities/catalog:by-intent?owner_module=orgunit&target_object=orgunit&surface=create_dialog&intent=create_org", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleCapabilityCatalogByIntentAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload capabilityCatalogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json err=%v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("items=%d", len(payload.Items))
	}
	if payload.Items[0].CapabilityKey != orgUnitCreateFieldPolicyCapabilityKey {
		t.Fatalf("capability=%q", payload.Items[0].CapabilityKey)
	}
}

func TestHandleCapabilityCatalogByIntentAPI_ErrorBranches(t *testing.T) {
	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal/capabilities/catalog:by-intent", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleCapabilityCatalogByIntentAPI(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/capabilities/catalog:by-intent", nil)
		rec := httptest.NewRecorder()
		handleCapabilityCatalogByIntentAPI(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("module mismatch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/capabilities/catalog:by-intent?module=orgunit&owner_module=staffing", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleCapabilityCatalogByIntentAPI(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}
