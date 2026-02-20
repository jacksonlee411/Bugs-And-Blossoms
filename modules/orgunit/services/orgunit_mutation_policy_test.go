package services

import (
	"sort"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
)

func TestResolvePolicy_CorrectEvent_CoreMatrixAndExtMerge(t *testing.T) {
	tests := []struct {
		target types.OrgUnitEventType
		want   []string
	}{
		{target: types.OrgUnitEventCreate, want: []string{"effective_date", "is_business_unit", "manager_pernr", "name", "parent_org_code"}},
		{target: types.OrgUnitEventRename, want: []string{"effective_date", "name"}},
		{target: types.OrgUnitEventMove, want: []string{"effective_date", "parent_org_code"}},
		{target: types.OrgUnitEventSetBusinessUnit, want: []string{"effective_date", "is_business_unit"}},
		{target: types.OrgUnitEventDisable, want: []string{"effective_date"}},
		{target: types.OrgUnitEventEnable, want: []string{"effective_date"}},
	}
	for _, tt := range tests {
		target := tt.target
		decision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
			ActionKind:               OrgUnitActionCorrectEvent,
			EmittedEventType:         OrgUnitEmittedCorrectEvent,
			TargetEffectiveEventType: &target,
		}, OrgUnitMutationPolicyFacts{CanAdmin: true})
		if err != nil {
			t.Fatalf("target=%s err=%v", tt.target, err)
		}
		if got := join(decision.AllowedFields); got != join(tt.want) {
			t.Fatalf("target=%s allowed=%v want=%v", tt.target, decision.AllowedFields, tt.want)
		}
	}

	target := types.OrgUnitEventRename
	decision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:               OrgUnitActionCorrectEvent,
		EmittedEventType:         OrgUnitEmittedCorrectEvent,
		TargetEffectiveEventType: &target,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:            true,
		EnabledExtFieldKeys: []string{"org_type", "short_name"},
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if join(decision.AllowedFields) != "effective_date,name,org_type,short_name" {
		t.Fatalf("allowed=%v", decision.AllowedFields)
	}
	if decision.FieldPayloadKeys["org_type"] != "ext.org_type" {
		t.Fatalf("ext mapping=%q", decision.FieldPayloadKeys["org_type"])
	}
	if decision.FieldPayloadKeys["name"] != "name" {
		t.Fatalf("core mapping=%q", decision.FieldPayloadKeys["name"])
	}
}

func TestResolvePolicy_CorrectStatus_DenyReasonsOrder(t *testing.T) {
	target := types.OrgUnitEventRename
	decision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:               OrgUnitActionCorrectStatus,
		EmittedEventType:         OrgUnitEmittedCorrectStatus,
		TargetEffectiveEventType: &target,
	}, OrgUnitMutationPolicyFacts{CanAdmin: false})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision.Enabled {
		t.Fatalf("expected disabled")
	}
	if join(decision.DenyReasons) != "FORBIDDEN,ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET" {
		t.Fatalf("deny=%v", decision.DenyReasons)
	}
}

func TestResolvePolicy_RescindOrg_DenyReasonsMergeAndOrder(t *testing.T) {
	decision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionRescindOrg,
		EmittedEventType: OrgUnitEmittedRescindOrg,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:              false,
		RescindOrgDenyReasons: []string{"ORG_ROOT_DELETE_FORBIDDEN"},
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision.Enabled {
		t.Fatalf("expected disabled")
	}
	if join(decision.DenyReasons) != "FORBIDDEN,ORG_ROOT_DELETE_FORBIDDEN" {
		t.Fatalf("deny=%v", decision.DenyReasons)
	}
}

func TestResolvePolicy_CorrectStatus_SupportedTarget(t *testing.T) {
	target := types.OrgUnitEventEnable
	decision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:               OrgUnitActionCorrectStatus,
		EmittedEventType:         OrgUnitEmittedCorrectStatus,
		TargetEffectiveEventType: &target,
	}, OrgUnitMutationPolicyFacts{CanAdmin: true})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !decision.Enabled {
		t.Fatalf("expected enabled")
	}
	if join(decision.AllowedTargetStatuses) != "active,disabled" {
		t.Fatalf("statuses=%v", decision.AllowedTargetStatuses)
	}
	if len(decision.DenyReasons) != 0 {
		t.Fatalf("deny=%v", decision.DenyReasons)
	}

	decision, err = ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:               OrgUnitActionCorrectStatus,
		EmittedEventType:         OrgUnitEmittedCorrectStatus,
		TargetEffectiveEventType: &target,
	}, OrgUnitMutationPolicyFacts{CanAdmin: false})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision.Enabled {
		t.Fatalf("expected disabled")
	}
	if join(decision.DenyReasons) != "FORBIDDEN" {
		t.Fatalf("deny=%v", decision.DenyReasons)
	}
}

func TestResolvePolicy_RescindEvent(t *testing.T) {
	target := types.OrgUnitEventRename
	decision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:               OrgUnitActionRescindEvent,
		EmittedEventType:         OrgUnitEmittedRescindEvent,
		TargetEffectiveEventType: &target,
	}, OrgUnitMutationPolicyFacts{CanAdmin: true})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !decision.Enabled {
		t.Fatalf("expected enabled")
	}
	if len(decision.DenyReasons) != 0 {
		t.Fatalf("deny=%v", decision.DenyReasons)
	}

	decision, err = ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:               OrgUnitActionRescindEvent,
		EmittedEventType:         OrgUnitEmittedRescindEvent,
		TargetEffectiveEventType: &target,
	}, OrgUnitMutationPolicyFacts{CanAdmin: false})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision.Enabled {
		t.Fatalf("expected disabled")
	}
	if join(decision.DenyReasons) != "FORBIDDEN" {
		t.Fatalf("deny=%v", decision.DenyReasons)
	}
}

func TestResolvePolicy_InvalidKey(t *testing.T) {
	target := types.OrgUnitEventCreate
	if _, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:               OrgUnitActionCorrectEvent,
		EmittedEventType:         OrgUnitEmittedCorrectStatus,
		TargetEffectiveEventType: &target,
	}, OrgUnitMutationPolicyFacts{CanAdmin: true}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolvePolicy_Append_Create(t *testing.T) {
	decision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionCreate,
		EmittedEventType: OrgUnitEmittedCreate,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:            true,
		TreeInitialized:     true,
		OrgAlreadyExists:    false,
		CreateAsRoot:        false,
		EnabledExtFieldKeys: []string{"org_type"},
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !decision.Enabled {
		t.Fatalf("expected enabled")
	}
	if join(decision.AllowedFields) != "effective_date,is_business_unit,manager_pernr,name,org_code,org_type,parent_org_code" {
		t.Fatalf("allowed=%v", decision.AllowedFields)
	}
	if join(keysOf(decision.FieldPayloadKeys)) != join(decision.AllowedFields) {
		t.Fatalf("field_payload_keys keys mismatch")
	}
	if decision.FieldPayloadKeys["name"] != "name" {
		t.Fatalf("name mapping=%q", decision.FieldPayloadKeys["name"])
	}
	if decision.FieldPayloadKeys["org_type"] != "ext.org_type" {
		t.Fatalf("ext mapping=%q", decision.FieldPayloadKeys["org_type"])
	}

	disabled, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionCreate,
		EmittedEventType: OrgUnitEmittedCreate,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:         true,
		TreeInitialized:  true,
		OrgAlreadyExists: true,
		CreateAsRoot:     false,
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if disabled.Enabled {
		t.Fatalf("expected disabled")
	}
	if join(disabled.DenyReasons) != "ORG_ALREADY_EXISTS" {
		t.Fatalf("deny=%v", disabled.DenyReasons)
	}
	if len(disabled.AllowedFields) != 0 || len(disabled.FieldPayloadKeys) != 0 {
		t.Fatalf("expected fail-closed fields")
	}

	treeNotInit, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionCreate,
		EmittedEventType: OrgUnitEmittedCreate,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:        true,
		TreeInitialized: false,
		CreateAsRoot:    false,
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if join(treeNotInit.DenyReasons) != "ORG_TREE_NOT_INITIALIZED" {
		t.Fatalf("deny=%v", treeNotInit.DenyReasons)
	}

	rootAlreadyExists, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionCreate,
		EmittedEventType: OrgUnitEmittedCreate,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:        true,
		TreeInitialized: true,
		CreateAsRoot:    true,
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if join(rootAlreadyExists.DenyReasons) != "ORG_ROOT_ALREADY_EXISTS" {
		t.Fatalf("deny=%v", rootAlreadyExists.DenyReasons)
	}
}

func TestResolvePolicy_Append_EventUpdate(t *testing.T) {
	rename, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedRename,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:            true,
		TreeInitialized:     true,
		TargetExistsAsOf:    true,
		IsRoot:              false,
		EnabledExtFieldKeys: []string{"org_type"},
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !rename.Enabled {
		t.Fatalf("expected enabled")
	}
	if join(rename.AllowedFields) != "effective_date,name,org_type" {
		t.Fatalf("allowed=%v", rename.AllowedFields)
	}
	if rename.FieldPayloadKeys["name"] != "new_name" {
		t.Fatalf("name mapping=%q", rename.FieldPayloadKeys["name"])
	}

	moveRoot, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedMove,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:         true,
		TreeInitialized:  true,
		TargetExistsAsOf: true,
		IsRoot:           true,
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if moveRoot.Enabled {
		t.Fatalf("expected disabled")
	}
	if join(moveRoot.DenyReasons) != "ORG_ROOT_CANNOT_BE_MOVED" {
		t.Fatalf("deny=%v", moveRoot.DenyReasons)
	}
	if len(moveRoot.AllowedFields) != 0 || len(moveRoot.FieldPayloadKeys) != 0 {
		t.Fatalf("expected fail-closed fields")
	}

	missing, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedDisable,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:         true,
		TreeInitialized:  true,
		TargetExistsAsOf: false,
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if join(missing.DenyReasons) != "ORG_NOT_FOUND_AS_OF" {
		t.Fatalf("deny=%v", missing.DenyReasons)
	}

	treeNotInitialized, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedDisable,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:         true,
		TreeInitialized:  false,
		TargetExistsAsOf: true,
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if join(treeNotInitialized.DenyReasons) != "ORG_TREE_NOT_INITIALIZED" {
		t.Fatalf("deny=%v", treeNotInitialized.DenyReasons)
	}

	forbidden, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedEnable,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:         false,
		TreeInitialized:  true,
		TargetExistsAsOf: true,
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if join(forbidden.DenyReasons) != "FORBIDDEN" {
		t.Fatalf("deny=%v", forbidden.DenyReasons)
	}

	setBU, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedSetBusinessUnit,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:         true,
		TreeInitialized:  true,
		TargetExistsAsOf: true,
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !setBU.Enabled {
		t.Fatalf("expected enabled")
	}
	if join(setBU.AllowedFields) != "effective_date,is_business_unit" {
		t.Fatalf("allowed=%v", setBU.AllowedFields)
	}

	if _, err := ResolvePolicy(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedEventType("NO_SUCH"),
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:         true,
		TreeInitialized:  true,
		TargetExistsAsOf: true,
	}); err == nil {
		t.Fatalf("expected invalid key error")
	}
}

func TestAllowedFields_ReturnsCopy(t *testing.T) {
	decision := OrgUnitMutationPolicyDecision{AllowedFields: []string{"a"}}
	got := AllowedFields(decision)
	got[0] = "mutated"
	if decision.AllowedFields[0] != "a" {
		t.Fatalf("expected defensive copy, got=%v", decision.AllowedFields)
	}
}

func TestValidatePatch_CoversBranches(t *testing.T) {
	decision := OrgUnitMutationPolicyDecision{AllowedFields: []string{"effective_date", "name", "parent_org_code", "is_business_unit", "manager_pernr", "org_type"}}

	t.Run("invalid effective_date", func(t *testing.T) {
		if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{EffectiveDate: new("bad")}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
			t.Fatalf("expected EFFECTIVE_DATE_INVALID, got %v", err)
		}
	})

	t.Run("effective-date correction mode allows only effective_date", func(t *testing.T) {
		if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{EffectiveDate: new("2026-01-02")}); err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
	})

	t.Run("effective-date correction mode allows other fields", func(t *testing.T) {
		if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{
			EffectiveDate: new("2026-01-02"),
			Name:          new("X"),
		}); err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
	})

	t.Run("effective-date correction mode allows ext payload", func(t *testing.T) {
		if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{
			EffectiveDate: new("2026-01-02"),
			Ext:           map[string]any{"org_type": "DEPARTMENT"},
		}); err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
	})

	t.Run("disallowed core field", func(t *testing.T) {
		limited := OrgUnitMutationPolicyDecision{AllowedFields: []string{"effective_date"}}
		if err := ValidatePatch("2026-01-01", limited, OrgUnitCorrectionPatch{Name: new("X")}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("disallowed parent_org_code", func(t *testing.T) {
		limited := OrgUnitMutationPolicyDecision{AllowedFields: []string{"effective_date"}}
		if err := ValidatePatch("2026-01-01", limited, OrgUnitCorrectionPatch{ParentOrgCode: new("ROOT")}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("disallowed is_business_unit", func(t *testing.T) {
		limited := OrgUnitMutationPolicyDecision{AllowedFields: []string{"effective_date"}}
		if err := ValidatePatch("2026-01-01", limited, OrgUnitCorrectionPatch{IsBusinessUnit: new(true)}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("disallowed manager_pernr", func(t *testing.T) {
		limited := OrgUnitMutationPolicyDecision{AllowedFields: []string{"effective_date"}}
		if err := ValidatePatch("2026-01-01", limited, OrgUnitCorrectionPatch{ManagerPernr: new("1001")}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("disallowed effective_date field", func(t *testing.T) {
		limited := OrgUnitMutationPolicyDecision{AllowedFields: []string{"name"}}
		if err := ValidatePatch("2026-01-01", limited, OrgUnitCorrectionPatch{EffectiveDate: new("2026-01-01")}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("blank ext key", func(t *testing.T) {
		if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{Ext: map[string]any{" ": "x"}}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("allowed ext key", func(t *testing.T) {
		if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{Ext: map[string]any{"org_type": "DEPARTMENT"}}); err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
	})

	t.Run("disallowed ext key", func(t *testing.T) {
		limited := OrgUnitMutationPolicyDecision{AllowedFields: []string{"effective_date"}}
		if err := ValidatePatch("2026-01-01", limited, OrgUnitCorrectionPatch{Ext: map[string]any{"org_type": "DEPARTMENT"}}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})
}

func TestPolicyHelpers_NormalizeMergeAndDenyReasonPriority(t *testing.T) {
	if got := normalizeFieldKeys(nil); len(got) != 0 {
		t.Fatalf("got=%v", got)
	}
	keys := normalizeFieldKeys([]string{"", " a ", "a", "b"})
	if join(keys) != "a,b" {
		t.Fatalf("keys=%v", keys)
	}
	keys = normalizeFieldKeys([]string{"name", "a"})
	if join(keys) != "a" {
		t.Fatalf("reserved keys should be filtered, got=%v", keys)
	}
	keys = normalizeFieldKeys([]string{"ext", "ext_labels_snapshot", "manager_pernr", "x"})
	if join(keys) != "x" {
		t.Fatalf("reserved keys should be filtered, got=%v", keys)
	}

	if !isReservedExtFieldKey("org_code") || !isReservedExtFieldKey("ext") || !isReservedExtFieldKey("ext_labels_snapshot") {
		t.Fatalf("expected reserved keys")
	}
	if isReservedExtFieldKey("org_type") {
		t.Fatalf("org_type should not be reserved")
	}

	if fields, ok := allowedAppendCoreFieldsForEmittedEvent(OrgUnitEmittedCreate); !ok || join(fields) != "effective_date,is_business_unit,manager_pernr,name,org_code,parent_org_code" {
		t.Fatalf("create fields=%v ok=%v", fields, ok)
	}

	merged := mergeAndSortKeys([]string{"", "b", "a", "a"}, []string{"c", "b", ""})
	if join(merged) != "a,b,c" {
		t.Fatalf("merged=%v", merged)
	}

	if got := dedupAndSortDenyReasons(nil); len(got) != 0 {
		t.Fatalf("got=%v", got)
	}
	reasons := dedupAndSortDenyReasons([]string{"", "ORG_EVENT_NOT_FOUND", "FORBIDDEN", "FORBIDDEN", "UNKNOWN"})
	if join(reasons) != "FORBIDDEN,ORG_EVENT_NOT_FOUND,UNKNOWN" {
		t.Fatalf("reasons=%v", reasons)
	}

	// Cover all priority switch branches.
	want := map[string]int{
		"FORBIDDEN":                                10,
		"ORG_TREE_NOT_INITIALIZED":                 20,
		"ORG_NOT_FOUND_AS_OF":                      30,
		"ORG_ROOT_CANNOT_BE_MOVED":                 40,
		"ORG_ALREADY_EXISTS":                       50,
		"ORG_ROOT_ALREADY_EXISTS":                  60,
		"ORG_EVENT_NOT_FOUND":                      70,
		"ORG_EVENT_RESCINDED":                      80,
		"ORG_ROOT_DELETE_FORBIDDEN":                90,
		"ORG_HAS_CHILDREN_CANNOT_DELETE":           91,
		"ORG_HAS_DEPENDENCIES_CANNOT_DELETE":       92,
		"ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET": 93,
		"UNKNOWN": 100,
	}
	for code, exp := range want {
		if got := denyReasonPriority(code); got != exp {
			t.Fatalf("code=%s got=%d exp=%d", code, got, exp)
		}
	}
}

func TestValidatePatch_EffectiveDateCorrectionModeAndAllowedFields(t *testing.T) {
	decision := OrgUnitMutationPolicyDecision{
		AllowedFields: []string{"effective_date", "name", "org_type"},
	}

	// Changing effective_date can be submitted together with other fields (DEV-PLAN-108).
	if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{
		EffectiveDate: new("2026-01-02"),
		Name:          new("New"),
	}); err != nil {
		t.Fatalf("unexpected err=%v", err)
	}

	// Allowed in normal mode.
	if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{
		EffectiveDate: new("2026-01-01"),
		Name:          new("New"),
	}); err != nil {
		t.Fatalf("unexpected err=%v", err)
	}

	// Ext field not allowed.
	if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{
		Ext: map[string]any{"missing": "x"},
	}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
		t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
	}
}

func join(items []string) string {
	var out strings.Builder
	for i, item := range items {
		if i > 0 {
			out.WriteString(",")
		}
		out.WriteString(item)
	}
	return out.String()
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Sort for stable comparison.
	sort.Strings(out)
	return out
}
