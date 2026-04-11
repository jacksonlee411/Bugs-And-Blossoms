package main

import (
	"strings"
	"testing"
)

func TestValidateSetIDStrategyRegistrySnapshot(t *testing.T) {
	t.Run("valid business unit row with org code", func(t *testing.T) {
		snapshot := setIDStrategyRegistrySnapshotFile{
			Version:  setIDStrategyRegistrySnapshotVersion,
			AsOfDate: "2026-04-10",
			RowCount: 1,
			Rows: []setIDStrategyRegistrySnapshotRow{
				{
					TenantUUID:          "00000000-0000-0000-0000-00000000000a",
					CapabilityKey:       "orgunit.write.field_policy",
					OwnerModule:         "orgunit",
					FieldKey:            "location_code",
					PersonalizationMode: "setid",
					OrgApplicability:    "business_unit",
					BusinessUnitOrgCode: "ROOT-A",
					Required:            true,
					Visible:             true,
					Maintainable:        true,
					Priority:            100,
					PriorityMode:        "blend_custom_first",
					LocalOverrideMode:   "allow",
					ExplainRequired:     true,
					IsStable:            true,
					ChangePolicy:        "plan_required",
					EffectiveDate:       "2026-04-01",
					UpdatedAt:           "2026-04-10T10:30:00Z",
				},
			},
		}
		if err := validateSetIDStrategyRegistrySnapshot(snapshot); err != nil {
			t.Fatalf("expected valid snapshot, got err=%v", err)
		}
	})

	t.Run("tenant row must not carry business unit refs", func(t *testing.T) {
		snapshot := setIDStrategyRegistrySnapshotFile{
			Version:  setIDStrategyRegistrySnapshotVersion,
			AsOfDate: "2026-04-10",
			RowCount: 1,
			Rows: []setIDStrategyRegistrySnapshotRow{
				{
					TenantUUID:          "00000000-0000-0000-0000-00000000000a",
					CapabilityKey:       "orgunit.write.field_policy",
					OwnerModule:         "orgunit",
					FieldKey:            "location_code",
					PersonalizationMode: "tenant_only",
					OrgApplicability:    "tenant",
					BusinessUnitOrgCode: "ROOT-A",
					Priority:            100,
					PriorityMode:        "blend_custom_first",
					LocalOverrideMode:   "allow",
					ChangePolicy:        "plan_required",
					EffectiveDate:       "2026-04-01",
					UpdatedAt:           "2026-04-10T10:30:00Z",
				},
			},
		}
		if err := validateSetIDStrategyRegistrySnapshot(snapshot); err == nil {
			t.Fatalf("expected tenant-row validation error")
		}
	})

	t.Run("business unit row requires org code or node key", func(t *testing.T) {
		snapshot := setIDStrategyRegistrySnapshotFile{
			Version:  setIDStrategyRegistrySnapshotVersion,
			AsOfDate: "2026-04-10",
			RowCount: 1,
			Rows: []setIDStrategyRegistrySnapshotRow{
				{
					TenantUUID:          "00000000-0000-0000-0000-00000000000a",
					CapabilityKey:       "orgunit.write.field_policy",
					OwnerModule:         "orgunit",
					FieldKey:            "location_code",
					PersonalizationMode: "setid",
					OrgApplicability:    "business_unit",
					Priority:            100,
					PriorityMode:        "blend_custom_first",
					LocalOverrideMode:   "allow",
					ExplainRequired:     true,
					ChangePolicy:        "plan_required",
					EffectiveDate:       "2026-04-01",
					UpdatedAt:           "2026-04-10T10:30:00Z",
				},
			},
		}
		if err := validateSetIDStrategyRegistrySnapshot(snapshot); err == nil {
			t.Fatalf("expected missing business unit ref validation error")
		}
	})
}

func TestEqualSetIDStrategyRegistrySnapshotRow(t *testing.T) {
	left := setIDStrategyRegistrySnapshotRow{
		TenantUUID:          "00000000-0000-0000-0000-00000000000a",
		CapabilityKey:       "orgunit.write.field_policy",
		OwnerModule:         "orgunit",
		FieldKey:            "location_code",
		PersonalizationMode: "setid",
		OrgApplicability:    "business_unit",
		BusinessUnitOrgCode: "ROOT-A",
		BusinessUnitNodeKey: "ABCDEFGH",
		Required:            true,
		Visible:             true,
		Maintainable:        true,
		AllowedValueCodes:   []string{"A", "B"},
		Priority:            100,
		PriorityMode:        "blend_custom_first",
		LocalOverrideMode:   "allow",
		ExplainRequired:     true,
		IsStable:            true,
		ChangePolicy:        "plan_required",
		EffectiveDate:       "2026-04-01",
		UpdatedAt:           "2026-04-10T10:30:00Z",
	}
	right := setIDStrategyRegistrySnapshotRow{
		TenantUUID:          "00000000-0000-0000-0000-00000000000a",
		CapabilityKey:       "orgunit.write.field_policy",
		OwnerModule:         "orgunit",
		FieldKey:            "location_code",
		PersonalizationMode: "setid",
		OrgApplicability:    "business_unit",
		BusinessUnitOrgCode: "ROOT-A",
		BusinessUnitNodeKey: "ABCDEFGH",
		Required:            true,
		Visible:             true,
		Maintainable:        true,
		AllowedValueCodes:   []string{"A", "B"},
		Priority:            100,
		PriorityMode:        "blend_custom_first",
		LocalOverrideMode:   "allow",
		ExplainRequired:     true,
		IsStable:            true,
		ChangePolicy:        "plan_required",
		EffectiveDate:       "2026-04-01",
		UpdatedAt:           "2026-04-10T10:30:00Z",
	}
	if !equalSetIDStrategyRegistrySnapshotRow(left, right) {
		t.Fatalf("expected rows to be equal")
	}
	right.BusinessUnitNodeKey = "ABCDEFGJ"
	if equalSetIDStrategyRegistrySnapshotRow(left, right) {
		t.Fatalf("expected rows to differ after node key change")
	}
}

func TestValidateSetIDStrategyRegistryTargetLayout(t *testing.T) {
	t.Run("accepts clean target schema", func(t *testing.T) {
		err := validateSetIDStrategyRegistryTargetLayout(setIDStrategyRegistryTargetLayout{
			SchemaState: setIDStrategyRegistrySchemaState{
				Columns: map[string]struct{}{
					"business_unit_node_key": {},
				},
				ConstraintDefs: map[string]string{
					"setid_strategy_registry_business_unit_node_key_applicability_check": "CHECK ((((org_applicability = 'tenant'::text) AND (business_unit_node_key = ''::text)) OR ((org_applicability = 'business_unit'::text) AND orgunit.is_valid_org_node_key(btrim(business_unit_node_key)))))",
				},
				OrgUnitVersionsColumns: map[string]struct{}{
					"org_node_key": {},
				},
			},
			OrgUnitCodesColumns: map[string]struct{}{
				"org_node_key": {},
			},
		})
		if err != nil {
			t.Fatalf("expected clean target layout, got err=%v", err)
		}
	})

	t.Run("blocks legacy target schema leftovers", func(t *testing.T) {
		err := validateSetIDStrategyRegistryTargetLayout(setIDStrategyRegistryTargetLayout{
			SchemaState: setIDStrategyRegistrySchemaState{
				Columns: map[string]struct{}{
					"business_unit_node_key": {},
					"business_unit_id":       {},
				},
				ConstraintDefs: map[string]string{
					"setid_strategy_registry_business_unit_applicability_check": "CHECK (((org_applicability = 'tenant'::text) AND (business_unit_id = ''::text)) OR ((org_applicability = 'business_unit'::text) AND (business_unit_id ~ '^[0-9]{8}$'::text)))",
				},
				OrgUnitVersionsColumns: map[string]struct{}{
					"org_node_key": {},
				},
			},
			OrgUnitCodesColumns: map[string]struct{}{
				"org_node_key": {},
			},
		})
		if err == nil {
			t.Fatalf("expected legacy target layout to be blocked")
		}
		for _, want := range []string{
			"schema_old_business_unit_id_present",
			"schema_old_constraint_present",
			"schema_legacy_regex_present",
			"schema_node_key_constraint_missing",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("expected err=%q to contain %q", err.Error(), want)
			}
		}
	})
}

func TestValidateSetIDStrategyRegistryTargetTenantRowCount(t *testing.T) {
	if err := validateSetIDStrategyRegistryTargetTenantRowCount("tenant-a", 0); err != nil {
		t.Fatalf("expected empty target tenant to pass, got err=%v", err)
	}
	err := validateSetIDStrategyRegistryTargetTenantRowCount("tenant-a", 2)
	if err == nil {
		t.Fatalf("expected non-empty target tenant to fail")
	}
	if !strings.Contains(err.Error(), "fresh target only") {
		t.Fatalf("expected fresh-target guidance, got err=%q", err.Error())
	}
}
