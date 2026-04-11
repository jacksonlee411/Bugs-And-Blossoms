package main

import "testing"

func TestValidateSetIDStrategyRegistrySchemaState(t *testing.T) {
	t.Run("accepts node_key schema state", func(t *testing.T) {
		issues := validateSetIDStrategyRegistrySchemaState(setIDStrategyRegistrySchemaState{
			Columns: map[string]struct{}{
				"business_unit_node_key": {},
				"resolved_setid":        {},
			},
			ConstraintDefs: map[string]string{
				"setid_strategy_registry_business_unit_node_key_applicability_ch": "CHECK ((((org_applicability = 'tenant'::text) AND (business_unit_node_key = ''::text)) OR ((org_applicability = 'business_unit'::text) AND orgunit.is_valid_org_node_key(btrim(business_unit_node_key)))))",
				"setid_strategy_registry_resolved_setid_format_check":            "CHECK ((btrim(resolved_setid) = ''::text) OR (btrim(resolved_setid) ~ '^[A-Z0-9]{5}$'::text))",
				"setid_strategy_registry_scope_shape_check":                      "CHECK ((((org_applicability = 'tenant'::text) AND (btrim(business_unit_node_key) = ''::text) AND (btrim(resolved_setid) = ''::text)) OR ((org_applicability = 'business_unit'::text) AND orgunit.is_valid_org_node_key(btrim(business_unit_node_key)) AND (btrim(resolved_setid) ~ '^[A-Z0-9]{5}$'::text))))",
			},
			IndexDefs: map[string]string{
				"setid_strategy_registry_key_unique_idx": "CREATE UNIQUE INDEX setid_strategy_registry_key_unique_idx ON orgunit.setid_strategy_registry USING btree (tenant_uuid, capability_key, field_key, org_applicability, resolved_setid, business_unit_node_key, effective_date)",
			},
			OrgUnitVersionsColumns: map[string]struct{}{
				"org_node_key": {},
			},
		})
		if len(issues) != 0 {
			t.Fatalf("expected no issues, got=%v", issues)
		}
	})

	t.Run("flags legacy schema leftovers", func(t *testing.T) {
		issues := validateSetIDStrategyRegistrySchemaState(setIDStrategyRegistrySchemaState{
			Columns: map[string]struct{}{
				"business_unit_id": {},
			},
			ConstraintDefs: map[string]string{
				"setid_strategy_registry_business_unit_applicability_check": "CHECK (((org_applicability = 'tenant'::text) AND (business_unit_id = ''::text)) OR ((org_applicability = 'business_unit'::text) AND (business_unit_id ~ '^[0-9]{8}$'::text)))",
			},
			OrgUnitVersionsColumns: map[string]struct{}{},
		})
		if len(issues) < 4 {
			t.Fatalf("expected multiple issues, got=%v", issues)
		}
	})
}

func TestValidateSetIDStrategyRegistryRows(t *testing.T) {
	current := map[setIDStrategyRegistryNodeKeyRef]int{
		{TenantUUID: "t1", NodeKey: "A2345678"}: 1,
		{TenantUUID: "t1", NodeKey: "B2345678"}: 2,
	}

	rows := []setIDStrategyRegistryValidationRow{
		{
			TenantUUID:          "t1",
			CapabilityKey:       "assistant.orgunit.create",
			FieldKey:            "business_unit_org_code",
			OrgApplicability:    "tenant",
			BusinessUnitNodeKey: "A2345678",
			EffectiveDate:       "2026-01-01",
		},
		{
			TenantUUID:          "t1",
			CapabilityKey:       "assistant.orgunit.create",
			FieldKey:            "business_unit_org_code",
			OrgApplicability:    "business_unit",
			BusinessUnitNodeKey: "",
			ResolvedSetID:       "A0001",
			EffectiveDate:       "2026-01-01",
		},
		{
			TenantUUID:          "t1",
			CapabilityKey:       "assistant.orgunit.create",
			FieldKey:            "business_unit_org_code",
			OrgApplicability:    "business_unit",
			BusinessUnitNodeKey: "bad",
			ResolvedSetID:       "A0001",
			EffectiveDate:       "2026-01-02",
		},
		{
			TenantUUID:          "t1",
			CapabilityKey:       "assistant.orgunit.create",
			FieldKey:            "business_unit_org_code",
			OrgApplicability:    "business_unit",
			BusinessUnitNodeKey: "C2345678",
			ResolvedSetID:       "A0001",
			EffectiveDate:       "2026-01-03",
		},
		{
			TenantUUID:          "t1",
			CapabilityKey:       "assistant.orgunit.create",
			FieldKey:            "business_unit_org_code",
			OrgApplicability:    "business_unit",
			BusinessUnitNodeKey: "B2345678",
			ResolvedSetID:       "A0001",
			EffectiveDate:       "2026-01-04",
		},
		{
			TenantUUID:          "t1",
			CapabilityKey:       "assistant.orgunit.create",
			FieldKey:            "business_unit_org_code",
			OrgApplicability:    "business_unit",
			BusinessUnitNodeKey: "A2345678",
			ResolvedSetID:       "A0001",
			EffectiveDate:       "2026-01-05",
		},
	}

	issues := validateSetIDStrategyRegistryRows(rows, current)
	if len(issues) != 5 {
		t.Fatalf("expected 5 issues, got=%d issues=%v", len(issues), issues)
	}
}
