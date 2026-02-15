package services

import (
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
		if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{EffectiveDate: stringPtr("bad")}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
			t.Fatalf("expected EFFECTIVE_DATE_INVALID, got %v", err)
		}
	})

	t.Run("effective-date correction mode allows only effective_date", func(t *testing.T) {
		if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{EffectiveDate: stringPtr("2026-01-02")}); err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
	})

	t.Run("effective-date correction mode rejects other fields", func(t *testing.T) {
		if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{
			EffectiveDate: stringPtr("2026-01-02"),
			Name:          stringPtr("X"),
		}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("effective-date correction mode rejects ext payload", func(t *testing.T) {
		if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{
			EffectiveDate: stringPtr("2026-01-02"),
			Ext:           map[string]any{"org_type": "DEPARTMENT"},
		}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("disallowed core field", func(t *testing.T) {
		limited := OrgUnitMutationPolicyDecision{AllowedFields: []string{"effective_date"}}
		if err := ValidatePatch("2026-01-01", limited, OrgUnitCorrectionPatch{Name: stringPtr("X")}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("disallowed parent_org_code", func(t *testing.T) {
		limited := OrgUnitMutationPolicyDecision{AllowedFields: []string{"effective_date"}}
		if err := ValidatePatch("2026-01-01", limited, OrgUnitCorrectionPatch{ParentOrgCode: stringPtr("ROOT")}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("disallowed is_business_unit", func(t *testing.T) {
		limited := OrgUnitMutationPolicyDecision{AllowedFields: []string{"effective_date"}}
		if err := ValidatePatch("2026-01-01", limited, OrgUnitCorrectionPatch{IsBusinessUnit: boolPtr(true)}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("disallowed manager_pernr", func(t *testing.T) {
		limited := OrgUnitMutationPolicyDecision{AllowedFields: []string{"effective_date"}}
		if err := ValidatePatch("2026-01-01", limited, OrgUnitCorrectionPatch{ManagerPernr: stringPtr("1001")}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("disallowed effective_date field", func(t *testing.T) {
		limited := OrgUnitMutationPolicyDecision{AllowedFields: []string{"name"}}
		if err := ValidatePatch("2026-01-01", limited, OrgUnitCorrectionPatch{EffectiveDate: stringPtr("2026-01-01")}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
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
		"ORG_EVENT_NOT_FOUND":                      20,
		"ORG_EVENT_RESCINDED":                      30,
		"ORG_ROOT_DELETE_FORBIDDEN":                40,
		"ORG_HAS_CHILDREN_CANNOT_DELETE":           50,
		"ORG_HAS_DEPENDENCIES_CANNOT_DELETE":       60,
		"ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET": 70,
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

	// Changing effective_date must be exclusive.
	if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{
		EffectiveDate: stringPtr("2026-01-02"),
		Name:          stringPtr("New"),
	}); err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
		t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
	}

	// Allowed in normal mode.
	if err := ValidatePatch("2026-01-01", decision, OrgUnitCorrectionPatch{
		EffectiveDate: stringPtr("2026-01-01"),
		Name:          stringPtr("New"),
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
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ","
		}
		out += item
	}
	return out
}
