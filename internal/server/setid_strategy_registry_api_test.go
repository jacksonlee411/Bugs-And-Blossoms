package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type setIDStrategyRegistryRows struct {
	idx     int
	rows    [][]any
	scanErr error
	err     error
}

type setIDStrategyRegistryStoreStub struct {
	upsertFn               func(context.Context, string, setIDStrategyRegistryItem) (setIDStrategyRegistryItem, bool, error)
	listFn                 func(context.Context, string, string, string, string) ([]setIDStrategyRegistryItem, error)
	resolveFieldDecisionFn func(context.Context, string, string, string, string, string) (setIDFieldDecision, error)
}

func (s setIDStrategyRegistryStoreStub) upsert(ctx context.Context, tenantID string, item setIDStrategyRegistryItem) (setIDStrategyRegistryItem, bool, error) {
	if s.upsertFn == nil {
		return item, false, nil
	}
	return s.upsertFn(ctx, tenantID, item)
}

func (s setIDStrategyRegistryStoreStub) list(ctx context.Context, tenantID string, capabilityKey string, fieldKey string, asOf string) ([]setIDStrategyRegistryItem, error) {
	if s.listFn == nil {
		return nil, nil
	}
	return s.listFn(ctx, tenantID, capabilityKey, fieldKey, asOf)
}

func (s setIDStrategyRegistryStoreStub) resolveFieldDecision(ctx context.Context, tenantID string, capabilityKey string, fieldKey string, businessUnitID string, asOf string) (setIDFieldDecision, error) {
	if s.resolveFieldDecisionFn == nil {
		return setIDFieldDecision{}, errors.New(fieldPolicyMissingCode)
	}
	return s.resolveFieldDecisionFn(ctx, tenantID, capabilityKey, fieldKey, businessUnitID, asOf)
}

func (r *setIDStrategyRegistryRows) Close()                        {}
func (r *setIDStrategyRegistryRows) Err() error                    { return r.err }
func (r *setIDStrategyRegistryRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *setIDStrategyRegistryRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *setIDStrategyRegistryRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *setIDStrategyRegistryRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.rows[r.idx-1]
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *bool:
			*d = row[i].(bool)
		case *int:
			*d = row[i].(int)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}
func (r *setIDStrategyRegistryRows) Values() ([]any, error) { return nil, nil }
func (r *setIDStrategyRegistryRows) RawValues() [][]byte    { return nil }
func (r *setIDStrategyRegistryRows) Conn() *pgx.Conn        { return nil }

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
		{name: "capability context token forbidden", item: func() setIDStrategyRegistryItem {
			it := valid
			it.CapabilityKey = "staffing.assignment_create.bu_a"
			return it
		}(), status: http.StatusUnprocessableEntity, code: "invalid_capability_key_context"},
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

func TestContainsCapabilityContextToken(t *testing.T) {
	cases := []struct {
		name string
		key  string
		want bool
	}{
		{name: "no context token", key: "staffing.assignment_create.field_policy", want: false},
		{name: "empty segment ignored", key: "staffing..assignment", want: false},
		{name: "exact token", key: "staffing.tenant.field_policy", want: true},
		{name: "prefixed token", key: "staffing.scope_cn.field_policy", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsCapabilityContextToken(tc.key); got != tc.want {
				t.Fatalf("got=%v want=%v", got, tc.want)
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

	contextMismatchReq := httptest.NewRequest(http.MethodPost, "/org/api/setid-strategy-registry", bytes.NewBufferString(`{"capability_key":"staffing.assignment_create.field_policy","owner_module":"staffing","field_key":"field_x","personalization_mode":"setid","org_level":"business_unit","business_unit_id":"10000001","required":true,"visible":true,"default_rule_ref":"rule://a1","default_value":"a1","priority":200,"explain_required":true,"is_stable":true,"change_policy":"plan_required","effective_date":"2026-01-01","request_id":"r-mismatch"}`))
	contextMismatchReq.Header.Set("X-Actor-Scope", "saas")
	contextMismatchReq = contextMismatchReq.WithContext(withTenant(contextMismatchReq.Context(), Tenant{ID: "t1"}))
	contextMismatchRec := httptest.NewRecorder()
	handleSetIDStrategyRegistryAPI(contextMismatchRec, contextMismatchReq)
	if contextMismatchRec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", contextMismatchRec.Code, contextMismatchRec.Body.String())
	}
	if !strings.Contains(contextMismatchRec.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("unexpected body=%q", contextMismatchRec.Body.String())
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

func TestHandleSetIDStrategyRegistryAPI_StoreErrorBranches(t *testing.T) {
	resetSetIDStrategyRegistryRuntimeForTest()
	t.Cleanup(resetSetIDStrategyRegistryRuntimeForTest)

	t.Run("list internal error", func(t *testing.T) {
		useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
			listFn: func(context.Context, string, string, string, string) ([]setIDStrategyRegistryItem, error) {
				return nil, errors.New("boom")
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/org/api/setid-strategy-registry?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDStrategyRegistryAPI(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "setid_strategy_registry_list_failed") {
			t.Fatalf("unexpected body=%q", rec.Body.String())
		}
	})

	t.Run("upsert internal error", func(t *testing.T) {
		useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
			upsertFn: func(context.Context, string, setIDStrategyRegistryItem) (setIDStrategyRegistryItem, bool, error) {
				return setIDStrategyRegistryItem{}, false, errors.New("boom")
			},
		})
		body := `{"capability_key":"staffing.assignment_create.field_policy","owner_module":"staffing","field_key":"field_x","personalization_mode":"setid","org_level":"business_unit","business_unit_id":"10000001","required":true,"visible":true,"default_rule_ref":"rule://a1","default_value":"a1","priority":200,"explain_required":true,"is_stable":true,"change_policy":"plan_required","effective_date":"2026-01-01","request_id":"r2"}`
		req := httptest.NewRequest(http.MethodPost, "/org/api/setid-strategy-registry", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleSetIDStrategyRegistryAPI(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "setid_strategy_registry_upsert_failed") {
			t.Fatalf("unexpected body=%q", rec.Body.String())
		}
	})
}

func TestUseSetIDStrategyRegistryStore(t *testing.T) {
	resetSetIDStrategyRegistryRuntimeForTest()
	t.Cleanup(resetSetIDStrategyRegistryRuntimeForTest)

	if defaultSetIDStrategyRegistryStore == nil {
		t.Fatal("expected default store")
	}
	useSetIDStrategyRegistryStore(nil)
	if defaultSetIDStrategyRegistryStore == nil {
		t.Fatal("expected runtime store fallback")
	}

	custom := &setIDStrategyRegistryRuntimeStore{runtime: newSetIDStrategyRegistryRuntime()}
	useSetIDStrategyRegistryStore(custom)
	if defaultSetIDStrategyRegistryStore != custom {
		t.Fatal("expected custom store")
	}
}

func TestSetIDStrategyRegistryPGStore(t *testing.T) {
	item := setIDStrategyRegistryItem{
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
		Priority:            200,
		ExplainRequired:     true,
		IsStable:            true,
		ChangePolicy:        "plan_required",
		EffectiveDate:       "2026-01-01",
		UpdatedAt:           "2026-01-01T00:00:00Z",
	}
	t.Run("new store nil", func(t *testing.T) {
		if got := newSetIDStrategyRegistryPGStore(nil); got != nil {
			t.Fatal("expected nil store")
		}
	})
	t.Run("upsert begin error", func(t *testing.T) {
		store := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin fail")
		}))
		if _, _, err := store.upsert(context.Background(), "t1", item); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("upsert set tenant error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("boom"), execErrAt: 1, row: &stubRow{vals: []any{false}}}
		store := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, _, err := store.upsert(context.Background(), "t1", item); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("upsert exists scan error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("boom")}
		store := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, _, err := store.upsert(context.Background(), "t1", item); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("upsert exec error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{false}}, execErr: errors.New("boom"), execErrAt: 2}
		store := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, _, err := store.upsert(context.Background(), "t1", item); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("upsert create and update", func(t *testing.T) {
		txCreate := &stubTx{row: &stubRow{vals: []any{false}}}
		storeCreate := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return txCreate, nil }))
		saved, updated, err := storeCreate.upsert(context.Background(), "t1", item)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if updated {
			t.Fatal("expected create")
		}
		if saved.CapabilityKey != item.CapabilityKey {
			t.Fatalf("saved=%+v", saved)
		}

		txUpdate := &stubTx{row: &stubRow{vals: []any{true}}}
		storeUpdate := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return txUpdate, nil }))
		_, updated, err = storeUpdate.upsert(context.Background(), "t1", item)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !updated {
			t.Fatal("expected update")
		}
	})
	t.Run("upsert with end_date", func(t *testing.T) {
		itemWithEndDate := item
		itemWithEndDate.EndDate = "2026-02-01"
		tx := &stubTx{row: &stubRow{vals: []any{false}}}
		store := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, _, err := store.upsert(context.Background(), "t1", itemWithEndDate); err != nil {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("list invalid as_of", func(t *testing.T) {
		store := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		if _, err := store.list(context.Background(), "t1", "", "", "bad"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("list begin/query/scan/rows error", func(t *testing.T) {
		storeBeginErr := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin fail")
		}))
		if _, err := storeBeginErr.list(context.Background(), "t1", "", "", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}

		txQueryErr := &stubTx{queryErr: errors.New("boom"), queryErrAt: 1}
		storeQueryErr := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return txQueryErr, nil }))
		if _, err := storeQueryErr.list(context.Background(), "t1", "", "", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}

		txScanErr := &stubTx{
			rows: &setIDStrategyRegistryRows{
				rows:    [][]any{{"staffing.assignment_create.field_policy", "staffing", "field_x", "setid", "business_unit", "10000001", true, true, "rule://a1", "a1", 200, true, true, "plan_required", "2026-01-01", "", "2026-01-01T00:00:00Z"}},
				scanErr: errors.New("scan fail"),
			},
		}
		storeScanErr := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return txScanErr, nil }))
		if _, err := storeScanErr.list(context.Background(), "t1", "", "", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}

		txRowsErr := &stubTx{
			rows: &setIDStrategyRegistryRows{
				rows: [][]any{},
				err:  errors.New("rows fail"),
			},
		}
		storeRowsErr := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return txRowsErr, nil }))
		if _, err := storeRowsErr.list(context.Background(), "t1", "", "", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("list and resolve success/missing", func(t *testing.T) {
		rows := &setIDStrategyRegistryRows{
			rows: [][]any{
				{"staffing.assignment_create.field_policy", "staffing", "field_x", "setid", "business_unit", "10000001", true, true, "rule://a1", "a1", 200, true, true, "plan_required", "2026-01-01", "", "2026-01-01T00:00:00Z"},
			},
		}
		tx := &stubTx{rows: rows}
		store := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		items, err := store.list(context.Background(), "t1", "staffing.assignment_create.field_policy", "field_x", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(items) != 1 || items[0].DefaultValue != "a1" {
			t.Fatalf("items=%+v", items)
		}

		rowsForResolve := &setIDStrategyRegistryRows{
			rows: [][]any{
				{"staffing.assignment_create.field_policy", "staffing", "field_x", "setid", "business_unit", "10000001", true, true, "rule://a1", "a1", 200, true, true, "plan_required", "2026-01-01", "", "2026-01-01T00:00:00Z"},
			},
		}
		resolveStore := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{rows: rowsForResolve}, nil }))
		decision, err := resolveStore.resolveFieldDecision(context.Background(), "t1", "staffing.assignment_create.field_policy", "field_x", "10000001", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !decision.Required || decision.ResolvedDefaultVal != "a1" {
			t.Fatalf("decision=%+v", decision)
		}

		emptyStore := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &setIDStrategyRegistryRows{rows: [][]any{}}}, nil
		}))
		if _, err := emptyStore.resolveFieldDecision(context.Background(), "t1", "staffing.assignment_create.field_policy", "field_x", "10000001", "2026-01-01"); err == nil || err.Error() != fieldPolicyMissingCode {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("resolve list error", func(t *testing.T) {
		store := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		if _, err := store.resolveFieldDecision(context.Background(), "t1", "staffing.assignment_create.field_policy", "field_x", "10000001", "bad"); err == nil {
			t.Fatal("expected error")
		}
	})
}
