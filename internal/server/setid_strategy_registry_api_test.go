package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeStrategyRegistryItem_Defaults(t *testing.T) {
	item := normalizeStrategyRegistryItem(setIDStrategyRegistryUpsertAPIRequest{
		CapabilityKey:       " Staffing.Assignment_Create.Field_Policy ",
		OwnerModule:         " Staffing ",
		FieldKey:            " Field_X ",
		PersonalizationMode: " SETID ",
		OrgLevel:            " TENANT ",
		Priority:            0,
		ChangePolicy:        "",
		EffectiveDate:       "2026-01-01",
	})
	if item.CapabilityKey != "staffing.assignment_create.field_policy" {
		t.Fatalf("capability_key=%q", item.CapabilityKey)
	}
	if item.FieldKey != "field_x" {
		t.Fatalf("field_key=%q", item.FieldKey)
	}
	if item.Priority != 100 {
		t.Fatalf("priority=%d", item.Priority)
	}
	if item.ChangePolicy != "plan_required" {
		t.Fatalf("change_policy=%q", item.ChangePolicy)
	}
	if item.BusinessUnitID != "" {
		t.Fatalf("business_unit_id=%q", item.BusinessUnitID)
	}
	if item.UpdatedAt == "" {
		t.Fatal("expected updated_at")
	}
}

func TestValidateStrategyRegistryItem(t *testing.T) {
	valid := setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelBusinessUnit,
		BusinessUnitID:      "10000001",
		Required:            true,
		Visible:             true,
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
	}
	cases := []struct {
		name   string
		item   setIDStrategyRegistryItem
		status int
		code   string
	}{
		{name: "missing required", item: setIDStrategyRegistryItem{}, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "capability invalid", item: func() setIDStrategyRegistryItem {
			it := valid
			it.CapabilityKey = "bad"
			return it
		}(), status: http.StatusBadRequest, code: "invalid_capability_key"},
		{name: "field invalid", item: func() setIDStrategyRegistryItem {
			it := valid
			it.FieldKey = "bad-field"
			return it
		}(), status: http.StatusBadRequest, code: "invalid_field_key"},
		{name: "mode invalid", item: func() setIDStrategyRegistryItem {
			it := valid
			it.PersonalizationMode = "bad"
			return it
		}(), status: http.StatusUnprocessableEntity, code: "personalization_mode_invalid"},
		{name: "org level invalid", item: func() setIDStrategyRegistryItem {
			it := valid
			it.OrgLevel = "bad"
			return it
		}(), status: http.StatusUnprocessableEntity, code: "org_level_invalid"},
		{name: "business unit required", item: func() setIDStrategyRegistryItem {
			it := valid
			it.BusinessUnitID = ""
			return it
		}(), status: http.StatusBadRequest, code: "invalid_business_unit_id"},
		{name: "business unit invalid", item: func() setIDStrategyRegistryItem {
			it := valid
			it.BusinessUnitID = "bad"
			return it
		}(), status: http.StatusBadRequest, code: "invalid_business_unit_id"},
		{name: "scope package scope required", item: func() setIDStrategyRegistryItem {
			it := valid
			it.PersonalizationMode = personalizationModeScopePackage
			it.ScopeCode = ""
			return it
		}(), status: http.StatusBadRequest, code: "invalid_scope_code"},
		{name: "explain required", item: func() setIDStrategyRegistryItem {
			it := valid
			it.ExplainRequired = false
			return it
		}(), status: http.StatusUnprocessableEntity, code: explainRequiredCode},
		{name: "field policy conflict", item: func() setIDStrategyRegistryItem {
			it := valid
			it.Required = true
			it.Visible = false
			return it
		}(), status: http.StatusUnprocessableEntity, code: fieldPolicyConflictCode},
		{name: "effective invalid", item: func() setIDStrategyRegistryItem {
			it := valid
			it.EffectiveDate = "bad"
			return it
		}(), status: http.StatusBadRequest, code: "invalid_effective_date"},
		{name: "end date invalid", item: func() setIDStrategyRegistryItem {
			it := valid
			it.EndDate = "bad"
			return it
		}(), status: http.StatusBadRequest, code: "invalid_end_date"},
		{name: "end date conflict", item: func() setIDStrategyRegistryItem {
			it := valid
			it.EndDate = "2026-01-01"
			return it
		}(), status: http.StatusUnprocessableEntity, code: fieldPolicyConflictCode},
		{name: "ok tenant", item: func() setIDStrategyRegistryItem {
			it := valid
			it.OrgLevel = orgLevelTenant
			it.BusinessUnitID = ""
			return it
		}(), status: 0, code: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, code, _ := validateStrategyRegistryItem(tc.item)
			if status != tc.status || code != tc.code {
				t.Fatalf("status=%d code=%q", status, code)
			}
		})
	}
}

func TestSetIDStrategyRegistryRuntime_UpsertListResolve(t *testing.T) {
	runtime := newSetIDStrategyRegistryRuntime()

	tenantItem := setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelTenant,
		Required:            false,
		Visible:             true,
		DefaultValue:        "b2",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            100,
	}
	buItem := setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelBusinessUnit,
		BusinessUnitID:      "10000001",
		Required:            true,
		Visible:             true,
		DefaultValue:        "a1",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	}
	_, updated := runtime.upsert("t1", tenantItem)
	if updated {
		t.Fatal("expected create")
	}
	_, updated = runtime.upsert("t1", buItem)
	if updated {
		t.Fatal("expected create")
	}
	updatedItem := buItem
	updatedItem.DefaultValue = "a2"
	_, updated = runtime.upsert("t1", updatedItem)
	if !updated {
		t.Fatal("expected update")
	}

	if _, err := runtime.list("t1", "", "", "bad"); err == nil {
		t.Fatal("expected error")
	}
	items, err := runtime.list("t1", "", "", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len=%d", len(items))
	}
	none, err := runtime.list("t1", "staffing.assignment_create.field_policy", "field_x", "2025-12-31")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(none) != 0 {
		t.Fatalf("len=%d", len(none))
	}

	decision, err := runtime.resolveFieldDecision("t1", "staffing.assignment_create.field_policy", "field_x", "10000001", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !decision.Required || decision.ResolvedDefaultVal != "a2" {
		t.Fatalf("decision=%+v", decision)
	}

	tenantDecision, err := runtime.resolveFieldDecision("t1", "staffing.assignment_create.field_policy", "field_x", "10000002", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if tenantDecision.Required || tenantDecision.ResolvedDefaultVal != "b2" {
		t.Fatalf("decision=%+v", tenantDecision)
	}

	if _, err := runtime.resolveFieldDecision("t1", "staffing.assignment_create.field_policy", "missing", "10000001", "2026-01-01"); err == nil || err.Error() != fieldPolicyMissingCode {
		t.Fatalf("err=%v", err)
	}

	runtime.byTenant["t1"] = append(runtime.byTenant["t1"], setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_conflict",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelBusinessUnit,
		BusinessUnitID:      "10000001",
		Required:            true,
		Visible:             false,
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            300,
	})
	if _, err := runtime.resolveFieldDecision("t1", "staffing.assignment_create.field_policy", "field_conflict", "10000001", "2026-01-01"); err == nil || err.Error() != fieldPolicyConflictCode {
		t.Fatalf("err=%v", err)
	}

	runtime.byTenant["t1"] = append(runtime.byTenant["t1"], setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelTenant,
		Required:            false,
		Visible:             true,
		ExplainRequired:     true,
		EffectiveDate:       "bad",
		Priority:            10,
	})
	runtime.byTenant["t1"] = append(runtime.byTenant["t1"], setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelTenant,
		Required:            false,
		Visible:             true,
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		EndDate:             "2026-01-01",
		Priority:            10,
	})
	runtime.byTenant["t1"] = append(runtime.byTenant["t1"], setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelTenant,
		Required:            false,
		Visible:             true,
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		EndDate:             "bad",
		Priority:            10,
	})
	items, err = runtime.list("t1", "staffing.assignment_create.field_policy", "field_x", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(items) < 2 {
		t.Fatalf("expected filtered rows, got=%d", len(items))
	}
}

func TestSetIDStrategyRegistryRuntime_ResolveFieldDecisionBranches(t *testing.T) {
	runtime := newSetIDStrategyRegistryRuntime()

	_, _ = runtime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_cap",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelTenant,
		Required:            false,
		Visible:             true,
		DefaultValue:        "ok",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            10,
	})

	rows, err := runtime.list("t1", "staffing.assignment_update.field_policy", "field_cap", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty rows, got=%d", len(rows))
	}

	if _, err := runtime.resolveFieldDecision("t1", "staffing.assignment_create.field_policy", "field_cap", "10000001", "bad"); err == nil || err.Error() != "invalid as_of" {
		t.Fatalf("err=%v", err)
	}

	runtime.byTenant["t1"] = append(runtime.byTenant["t1"], setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_unknown_org",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            "unknown",
		Required:            false,
		Visible:             true,
		DefaultValue:        "ok",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            50,
	})
	if _, err := runtime.resolveFieldDecision("t1", "staffing.assignment_create.field_policy", "field_unknown_org", "10000001", "2026-01-01"); err == nil || err.Error() != fieldPolicyMissingCode {
		t.Fatalf("err=%v", err)
	}

	_, _ = runtime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_missing_default",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelBusinessUnit,
		BusinessUnitID:      "10000001",
		Required:            false,
		Visible:             true,
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            100,
	})
	if _, err := runtime.resolveFieldDecision("t1", "staffing.assignment_create.field_policy", "field_missing_default", "10000001", "2026-01-01"); err == nil || err.Error() != fieldDefaultRuleMissingCode {
		t.Fatalf("err=%v", err)
	}
}

func TestSetIDStrategyRegistryRuntime_BUFieldVarianceAcceptance(t *testing.T) {
	runtime := newSetIDStrategyRegistryRuntime()

	_, _ = runtime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelBusinessUnit,
		BusinessUnitID:      "10000001",
		Required:            true,
		Visible:             true,
		DefaultRuleRef:      "rule://a1",
		DefaultValue:        "a1",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})
	_, _ = runtime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelBusinessUnit,
		BusinessUnitID:      "10000002",
		Required:            false,
		Visible:             false,
		DefaultRuleRef:      "rule://b2",
		DefaultValue:        "b2",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})

	buA, err := runtime.resolveFieldDecision("t1", "staffing.assignment_create.field_policy", "field_x", "10000001", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !buA.Required || !buA.Visible || buA.ResolvedDefaultVal != "a1" {
		t.Fatalf("unexpected buA decision: %+v", buA)
	}

	buB, err := runtime.resolveFieldDecision("t1", "staffing.assignment_create.field_policy", "field_x", "10000002", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if buB.Required || buB.Visible || buB.ResolvedDefaultVal != "b2" {
		t.Fatalf("unexpected buB decision: %+v", buB)
	}
}

func TestHandleSetIDStrategyRegistryAPI(t *testing.T) {
	resetSetIDStrategyRegistryRuntimeForTest()
	t.Cleanup(resetSetIDStrategyRegistryRuntimeForTest)

	req := httptest.NewRequest(http.MethodGet, "/org/api/setid-strategy-registry?as_of=2026-01-01", nil)
	rec := httptest.NewRecorder()
	handleSetIDStrategyRegistryAPI(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}

	methodReq := httptest.NewRequest(http.MethodPut, "/org/api/setid-strategy-registry", nil)
	methodReq = methodReq.WithContext(withTenant(methodReq.Context(), Tenant{ID: "t1"}))
	methodRec := httptest.NewRecorder()
	handleSetIDStrategyRegistryAPI(methodRec, methodReq)
	if methodRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", methodRec.Code)
	}

	missingAsOfReq := httptest.NewRequest(http.MethodGet, "/org/api/setid-strategy-registry", nil)
	missingAsOfReq = missingAsOfReq.WithContext(withTenant(missingAsOfReq.Context(), Tenant{ID: "t1"}))
	missingAsOfRec := httptest.NewRecorder()
	handleSetIDStrategyRegistryAPI(missingAsOfRec, missingAsOfReq)
	if missingAsOfRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", missingAsOfRec.Code)
	}

	invalidAsOfReq := httptest.NewRequest(http.MethodGet, "/org/api/setid-strategy-registry?as_of=bad", nil)
	invalidAsOfReq = invalidAsOfReq.WithContext(withTenant(invalidAsOfReq.Context(), Tenant{ID: "t1"}))
	invalidAsOfRec := httptest.NewRecorder()
	handleSetIDStrategyRegistryAPI(invalidAsOfRec, invalidAsOfReq)
	if invalidAsOfRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", invalidAsOfRec.Code)
	}

	badJSONReq := httptest.NewRequest(http.MethodPost, "/org/api/setid-strategy-registry", strings.NewReader("{"))
	badJSONReq = badJSONReq.WithContext(withTenant(badJSONReq.Context(), Tenant{ID: "t1"}))
	badJSONRec := httptest.NewRecorder()
	handleSetIDStrategyRegistryAPI(badJSONRec, badJSONReq)
	if badJSONRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", badJSONRec.Code)
	}

	missingRequestIDReq := httptest.NewRequest(http.MethodPost, "/org/api/setid-strategy-registry", bytes.NewBufferString(`{"capability_key":"a.b","owner_module":"a","field_key":"field_x","personalization_mode":"tenant_only","org_level":"tenant","effective_date":"2026-01-01","request_id":""}`))
	missingRequestIDReq = missingRequestIDReq.WithContext(withTenant(missingRequestIDReq.Context(), Tenant{ID: "t1"}))
	missingRequestIDRec := httptest.NewRecorder()
	handleSetIDStrategyRegistryAPI(missingRequestIDRec, missingRequestIDReq)
	if missingRequestIDRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", missingRequestIDRec.Code)
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/org/api/setid-strategy-registry", bytes.NewBufferString(`{"capability_key":"staffing.assignment_create.field_policy","owner_module":"staffing","field_key":"field_x","personalization_mode":"bad","org_level":"tenant","effective_date":"2026-01-01","request_id":"r1"}`))
	invalidReq = invalidReq.WithContext(withTenant(invalidReq.Context(), Tenant{ID: "t1"}))
	invalidRec := httptest.NewRecorder()
	handleSetIDStrategyRegistryAPI(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", invalidRec.Code)
	}

	createBody := `{"capability_key":"staffing.assignment_create.field_policy","owner_module":"staffing","field_key":"field_x","personalization_mode":"setid","org_level":"business_unit","business_unit_id":"10000001","required":true,"visible":true,"default_rule_ref":"rule://a1","default_value":"a1","priority":200,"explain_required":true,"is_stable":true,"change_policy":"plan_required","effective_date":"2026-01-01","request_id":"r2"}`
	createReq := httptest.NewRequest(http.MethodPost, "/org/api/setid-strategy-registry", bytes.NewBufferString(createBody))
	createReq = createReq.WithContext(withTenant(createReq.Context(), Tenant{ID: "t1"}))
	createRec := httptest.NewRecorder()
	handleSetIDStrategyRegistryAPI(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), `"capability_key":"staffing.assignment_create.field_policy"`) {
		t.Fatalf("unexpected body: %q", createRec.Body.String())
	}

	updateBody := `{"capability_key":"staffing.assignment_create.field_policy","owner_module":"staffing","field_key":"field_x","personalization_mode":"setid","org_level":"business_unit","business_unit_id":"10000001","required":false,"visible":true,"default_value":"a2","priority":220,"explain_required":true,"is_stable":true,"change_policy":"plan_required","effective_date":"2026-01-01","request_id":"r3"}`
	updateReq := httptest.NewRequest(http.MethodPost, "/org/api/setid-strategy-registry", bytes.NewBufferString(updateBody))
	updateReq = updateReq.WithContext(withTenant(updateReq.Context(), Tenant{ID: "t1"}))
	updateRec := httptest.NewRecorder()
	handleSetIDStrategyRegistryAPI(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", updateRec.Code, updateRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/org/api/setid-strategy-registry?as_of=2026-01-01&capability_key=staffing.assignment_create.field_policy&field_key=field_x", nil)
	listReq = listReq.WithContext(withTenant(listReq.Context(), Tenant{ID: "t1"}))
	listRec := httptest.NewRecorder()
	handleSetIDStrategyRegistryAPI(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"items":[`) {
		t.Fatalf("unexpected body: %q", listRec.Body.String())
	}
}
