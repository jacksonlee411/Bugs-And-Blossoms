package fieldpolicy

import "testing"

func TestResolvePrefersFirstNonEmptyBucket(t *testing.T) {
	decision, err := Resolve(
		PolicyContext{
			CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
			FieldKey:            "d_org_type",
			ResolvedSetID:       "B1000",
			BusinessUnitNodeKey: "10000001",
		},
		OrgUnitWriteFieldPolicyCapabilityKey,
		[]Record{
			{
				PolicyID:            "baseline-bu",
				CapabilityKey:       OrgUnitWriteFieldPolicyCapabilityKey,
				FieldKey:            "d_org_type",
				OrgApplicability:    OrgApplicabilityBusinessUnit,
				ResolvedSetID:       "B1000",
				BusinessUnitNodeKey: "10000001",
				Required:            true,
				Visible:             true,
				Maintainable:        true,
				DefaultValue:        "baseline-bu",
				Priority:            300,
				EffectiveDate:       "2026-01-01",
				CreatedAt:           "2026-01-01T00:00:00Z",
			},
			{
				PolicyID:         "intent-setid",
				CapabilityKey:    OrgUnitCreateFieldPolicyCapabilityKey,
				FieldKey:         "d_org_type",
				OrgApplicability: OrgApplicabilityTenant,
				ResolvedSetID:    "B1000",
				Required:         true,
				Visible:          true,
				Maintainable:     true,
				DefaultValue:     "intent-setid",
				Priority:         200,
				EffectiveDate:    "2026-01-01",
				CreatedAt:        "2026-01-01T00:00:00Z",
			},
			{
				PolicyID:            "intent-bu",
				CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
				FieldKey:            "d_org_type",
				OrgApplicability:    OrgApplicabilityBusinessUnit,
				ResolvedSetID:       "B1000",
				BusinessUnitNodeKey: "10000001",
				Required:            true,
				Visible:             true,
				Maintainable:        true,
				DefaultValue:        "intent-bu",
				Priority:            100,
				EffectiveDate:       "2026-01-01",
				CreatedAt:           "2026-01-01T00:00:00Z",
			},
		},
	)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision.MatchedBucket != BucketIntentSetIDBusinessUnitExact {
		t.Fatalf("matched_bucket=%q", decision.MatchedBucket)
	}
	if decision.PrimaryPolicyID != "intent-bu" {
		t.Fatalf("primary_policy_id=%q", decision.PrimaryPolicyID)
	}
	if decision.SourceType != SourceTypeIntentOverride {
		t.Fatalf("source_type=%q", decision.SourceType)
	}
	if decision.ResolvedDefaultVal != "intent-bu" {
		t.Fatalf("resolved_default_value=%q", decision.ResolvedDefaultVal)
	}
}

func TestResolveModeMatrix(t *testing.T) {
	baseRecords := []Record{
		{
			PolicyID:            "local",
			CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
			FieldKey:            "d_org_type",
			OrgApplicability:    OrgApplicabilityBusinessUnit,
			ResolvedSetID:       "B1000",
			BusinessUnitNodeKey: "10000001",
			Required:            true,
			Visible:             true,
			Maintainable:        true,
			AllowedValueCodes:   []string{"11", "12"},
			DefaultValue:        "11",
			Priority:            200,
			EffectiveDate:       "2026-01-01",
			CreatedAt:           "2026-01-02T00:00:00Z",
		},
		{
			PolicyID:          "fallback-setid",
			CapabilityKey:     OrgUnitCreateFieldPolicyCapabilityKey,
			FieldKey:          "d_org_type",
			OrgApplicability:  OrgApplicabilityTenant,
			ResolvedSetID:     "B1000",
			Required:          true,
			Visible:           true,
			Maintainable:      true,
			AllowedValueCodes: []string{"12", "13"},
			DefaultValue:      "12",
			Priority:          100,
			EffectiveDate:     "2026-01-01",
			CreatedAt:         "2026-01-01T00:00:00Z",
		},
		{
			PolicyID:          "fallback-wildcard",
			CapabilityKey:     OrgUnitWriteFieldPolicyCapabilityKey,
			FieldKey:          "d_org_type",
			OrgApplicability:  OrgApplicabilityTenant,
			Required:          true,
			Visible:           true,
			Maintainable:      true,
			AllowedValueCodes: []string{"13", "14"},
			DefaultValue:      "13",
			Priority:          50,
			EffectiveDate:     "2026-01-01",
			CreatedAt:         "2025-12-31T00:00:00Z",
		},
	}
	ctx := PolicyContext{
		CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
		FieldKey:            "d_org_type",
		ResolvedSetID:       "B1000",
		BusinessUnitNodeKey: "10000001",
	}

	t.Run("blend custom first allow", func(t *testing.T) {
		records := append([]Record(nil), baseRecords...)
		records[0].PriorityMode = PriorityModeBlendCustomFirst
		records[0].LocalOverrideMode = LocalOverrideModeAllow
		decision, err := Resolve(ctx, OrgUnitWriteFieldPolicyCapabilityKey, records)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got := decision.AllowedValueCodes; len(got) != 4 || got[0] != "11" || got[1] != "12" || got[2] != "13" || got[3] != "14" {
			t.Fatalf("allowed_value_codes=%v", got)
		}
		if len(decision.WinnerPolicyIDs) != 3 {
			t.Fatalf("winner_policy_ids=%v", decision.WinnerPolicyIDs)
		}
	})

	t.Run("blend custom first no override", func(t *testing.T) {
		records := append([]Record(nil), baseRecords...)
		records[0].PriorityMode = PriorityModeBlendCustomFirst
		records[0].LocalOverrideMode = LocalOverrideModeNoOverride
		decision, err := Resolve(ctx, OrgUnitWriteFieldPolicyCapabilityKey, records)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got := decision.AllowedValueCodes; len(got) != 4 || got[0] != "12" || got[1] != "13" || got[2] != "14" || got[3] != "11" {
			t.Fatalf("allowed_value_codes=%v", got)
		}
	})

	t.Run("blend deflt first no local", func(t *testing.T) {
		records := append([]Record(nil), baseRecords...)
		records[0].PriorityMode = PriorityModeBlendDefltFirst
		records[0].LocalOverrideMode = LocalOverrideModeNoLocal
		decision, err := Resolve(ctx, OrgUnitWriteFieldPolicyCapabilityKey, records)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got := decision.AllowedValueCodes; len(got) != 3 || got[0] != "12" || got[1] != "13" || got[2] != "14" {
			t.Fatalf("allowed_value_codes=%v", got)
		}
	})

	t.Run("deflt unsubscribed allow", func(t *testing.T) {
		records := append([]Record(nil), baseRecords...)
		records[0].PriorityMode = PriorityModeDefltUnsubscribed
		records[0].LocalOverrideMode = LocalOverrideModeAllow
		decision, err := Resolve(ctx, OrgUnitWriteFieldPolicyCapabilityKey, records)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got := decision.AllowedValueCodes; len(got) != 2 || got[0] != "11" || got[1] != "12" {
			t.Fatalf("allowed_value_codes=%v", got)
		}
	})

	t.Run("illegal combination", func(t *testing.T) {
		records := append([]Record(nil), baseRecords...)
		records[0].PriorityMode = PriorityModeDefltUnsubscribed
		records[0].LocalOverrideMode = LocalOverrideModeNoLocal
		if _, err := Resolve(ctx, OrgUnitWriteFieldPolicyCapabilityKey, records); err == nil || err.Error() != ErrorPolicyModeCombination {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestResolveIncludesTrace(t *testing.T) {
	decision, err := Resolve(
		PolicyContext{
			CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
			FieldKey:            "d_org_type",
			ResolvedSetID:       "B1000",
			BusinessUnitNodeKey: "10000001",
		},
		OrgUnitWriteFieldPolicyCapabilityKey,
		[]Record{
			{
				PolicyID:            "local",
				CapabilityKey:       OrgUnitCreateFieldPolicyCapabilityKey,
				FieldKey:            "d_org_type",
				OrgApplicability:    OrgApplicabilityBusinessUnit,
				ResolvedSetID:       "B1000",
				BusinessUnitNodeKey: "10000001",
				Required:            true,
				Visible:             true,
				Maintainable:        true,
				AllowedValueCodes:   []string{"11"},
				DefaultValue:        "11",
				Priority:            100,
				EffectiveDate:       "2026-01-01",
				CreatedAt:           "2026-01-01T00:00:00Z",
			},
			{
				PolicyID:          "fallback",
				CapabilityKey:     OrgUnitWriteFieldPolicyCapabilityKey,
				FieldKey:          "d_org_type",
				OrgApplicability:  OrgApplicabilityTenant,
				Required:          true,
				Visible:           true,
				Maintainable:      true,
				AllowedValueCodes: []string{"12"},
				DefaultValue:      "12",
				Priority:          50,
				EffectiveDate:     "2026-01-01",
				CreatedAt:         "2025-12-31T00:00:00Z",
			},
		},
	)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(decision.ResolutionTrace) < 2 {
		t.Fatalf("resolution_trace=%v", decision.ResolutionTrace)
	}
	if decision.PrimaryPolicyID != "local" {
		t.Fatalf("primary_policy_id=%q", decision.PrimaryPolicyID)
	}
	if len(decision.MatchedPolicyIDs) != 1 || decision.MatchedPolicyIDs[0] != "local" {
		t.Fatalf("matched_policy_ids=%v", decision.MatchedPolicyIDs)
	}
}
