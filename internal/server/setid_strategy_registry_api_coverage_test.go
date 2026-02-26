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
)

func TestCollectCapabilityResolutionItems_BaselineError(t *testing.T) {
	items, err := collectCapabilityResolutionItems(func(queryCapabilityKey string) ([]setIDStrategyRegistryItem, error) {
		if queryCapabilityKey == orgUnitWriteFieldPolicyCapabilityKey {
			return nil, errors.New("baseline list failed")
		}
		return []setIDStrategyRegistryItem{}, nil
	}, orgUnitCreateFieldPolicyCapabilityKey)
	if err == nil || !strings.Contains(err.Error(), "baseline list failed") {
		t.Fatalf("items=%v err=%v", items, err)
	}
}

func TestStrategySourceTypeForCapabilityKey_Baseline(t *testing.T) {
	if got := strategySourceTypeForCapabilityKey(orgUnitWriteFieldPolicyCapabilityKey); got != strategySourceBaseline {
		t.Fatalf("source_type=%q", got)
	}
}

func TestValidateStrategyRegistryItem_DefinitionAndCatalogBranches(t *testing.T) {
	base := setIDStrategyRegistryItem{
		CapabilityKey:       orgUnitCreateFieldPolicyCapabilityKey,
		OwnerModule:         "orgunit",
		FieldKey:            "d_org_type",
		PersonalizationMode: personalizationModeSetID,
		OrgApplicability:    orgApplicabilityBusinessUnit,
		BusinessUnitID:      "10000001",
		Required:            true,
		Visible:             true,
		Maintainable:        true,
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
	}

	t.Run("definition missing", func(t *testing.T) {
		item := base
		item.CapabilityKey = "org.unknown.field_policy"
		status, code, _ := validateStrategyRegistryItem(item)
		if status != http.StatusUnprocessableEntity || code != "invalid_request" {
			t.Fatalf("status=%d code=%q", status, code)
		}
	})

	t.Run("definition owner mismatch", func(t *testing.T) {
		item := base
		item.OwnerModule = "staffing"
		status, code, _ := validateStrategyRegistryItem(item)
		if status != http.StatusUnprocessableEntity || code != "invalid_request" {
			t.Fatalf("status=%d code=%q", status, code)
		}
	})

	t.Run("catalog owner mismatch", func(t *testing.T) {
		origCatalog := capabilityCatalogByCapabilityKey
		capabilityCatalogByCapabilityKey = map[string]capabilityCatalogEntry{
			orgUnitCreateFieldPolicyCapabilityKey: {
				CapabilityKey: orgUnitCreateFieldPolicyCapabilityKey,
				OwnerModule:   "staffing",
			},
		}
		t.Cleanup(func() {
			capabilityCatalogByCapabilityKey = origCatalog
		})

		status, code, _ := validateStrategyRegistryItem(base)
		if status != http.StatusUnprocessableEntity || code != "invalid_request" {
			t.Fatalf("status=%d code=%q", status, code)
		}
	})
}

func TestValidateStrategyRegistryDisableRequest_DefinitionAndCatalogBranches(t *testing.T) {
	base := setIDStrategyRegistryDisableRequest{
		CapabilityKey:    orgUnitCreateFieldPolicyCapabilityKey,
		FieldKey:         "d_org_type",
		OrgApplicability: orgApplicabilityBusinessUnit,
		BusinessUnitID:   "10000001",
		EffectiveDate:    "2026-01-01",
		DisableAsOf:      "2026-01-02",
	}

	t.Run("definition missing", func(t *testing.T) {
		req := base
		req.CapabilityKey = "org.unknown.field_policy"
		status, code, _ := validateStrategyRegistryDisableRequest(req)
		if status != http.StatusUnprocessableEntity || code != "invalid_request" {
			t.Fatalf("status=%d code=%q", status, code)
		}
	})

	t.Run("catalog missing", func(t *testing.T) {
		origCatalog := capabilityCatalogByCapabilityKey
		capabilityCatalogByCapabilityKey = map[string]capabilityCatalogEntry{}
		t.Cleanup(func() {
			capabilityCatalogByCapabilityKey = origCatalog
		})

		status, code, _ := validateStrategyRegistryDisableRequest(base)
		if status != http.StatusUnprocessableEntity || code != "invalid_request" {
			t.Fatalf("status=%d code=%q", status, code)
		}
	})
}

func TestResolveFieldDecisionFromItems_EmptyCapabilityAndFieldMismatch(t *testing.T) {
	if _, err := resolveFieldDecisionFromItems(nil, "", "d_org_type", ""); err == nil || err.Error() != fieldPolicyMissingCode {
		t.Fatalf("err=%v", err)
	}

	decision, found, err := resolveCapabilityBucketDecision([]setIDStrategyRegistryItem{
		{
			CapabilityKey:    orgUnitCreateFieldPolicyCapabilityKey,
			FieldKey:         "another_field",
			OrgApplicability: orgApplicabilityTenant,
			Required:         true,
			Visible:          true,
			Maintainable:     true,
			EffectiveDate:    "2026-01-01",
		},
	}, orgUnitCreateFieldPolicyCapabilityKey, "d_org_type", orgApplicabilityTenant, "", strategySourceIntentOverride)
	if err != nil || found {
		t.Fatalf("decision=%+v found=%v err=%v", decision, found, err)
	}
}

func TestFieldDecisionSemanticallyEqual_DiffBranches(t *testing.T) {
	a := setIDFieldDecision{
		Required:           true,
		Visible:            true,
		Maintainable:       true,
		DefaultRuleRef:     "rule://a",
		ResolvedDefaultVal: "11",
		AllowedValueCodes:  []string{"11"},
	}
	b := a

	b.Required = false
	if fieldDecisionSemanticallyEqual(a, b) {
		t.Fatal("expected required diff not equal")
	}

	b = a
	b.DefaultRuleRef = "rule://b"
	if fieldDecisionSemanticallyEqual(a, b) {
		t.Fatal("expected default_rule_ref diff not equal")
	}

	b = a
	b.AllowedValueCodes = []string{"11", "12"}
	if fieldDecisionSemanticallyEqual(a, b) {
		t.Fatal("expected allowed_value_codes length diff not equal")
	}
}

func TestMergeStrategyItemsWithUpsert_Replace(t *testing.T) {
	oldItem := setIDStrategyRegistryItem{
		CapabilityKey:    orgUnitCreateFieldPolicyCapabilityKey,
		FieldKey:         "d_org_type",
		OrgApplicability: orgApplicabilityTenant,
		EffectiveDate:    "2026-01-01",
		DefaultValue:     "11",
	}
	newItem := oldItem
	newItem.DefaultValue = "12"

	merged := mergeStrategyItemsWithUpsert([]setIDStrategyRegistryItem{oldItem}, newItem)
	if len(merged) != 1 {
		t.Fatalf("len=%d", len(merged))
	}
	if merged[0].DefaultValue != "12" {
		t.Fatalf("item=%+v", merged[0])
	}
}

func TestEnsureStrategyResolvableAfterDisable_IntentUsesBaselineCandidate(t *testing.T) {
	err := ensureStrategyResolvableAfterDisable([]setIDStrategyRegistryItem{
		{
			CapabilityKey:    orgUnitWriteFieldPolicyCapabilityKey,
			FieldKey:         "d_org_type",
			OrgApplicability: orgApplicabilityTenant,
			Required:         true,
			Visible:          true,
			Maintainable:     true,
			DefaultValue:     "11",
			EffectiveDate:    "2026-01-01",
		},
	}, setIDStrategyRegistryDisableRequest{
		CapabilityKey:    orgUnitCreateFieldPolicyCapabilityKey,
		FieldKey:         "d_org_type",
		OrgApplicability: orgApplicabilityTenant,
		EffectiveDate:    "2026-01-01",
		DisableAsOf:      "2026-01-02",
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestSetIDStrategyRegistryPGStore_Disable_IntentIncludesBaselineCandidate(t *testing.T) {
	targetRow := []any{
		orgUnitCreateFieldPolicyCapabilityKey,
		"orgunit",
		"d_org_type",
		"setid",
		orgApplicabilityTenant,
		"",
		true,
		true,
		true,
		"",
		"11",
		`["11"]`,
		100,
		true,
		true,
		"plan_required",
		"2026-01-01",
		"",
		"2026-01-01T00:00:00Z",
	}
	baselineRow := []any{
		orgUnitWriteFieldPolicyCapabilityKey,
		"orgunit",
		"d_org_type",
		"setid",
		orgApplicabilityTenant,
		"",
		true,
		true,
		true,
		"",
		"11",
		`["11"]`,
		100,
		true,
		true,
		"plan_required",
		"2026-01-01",
		"",
		"2026-01-01T00:00:00Z",
	}

	store := newSetIDStrategyRegistryPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{
			row:  &stubRow{vals: targetRow},
			rows: &setIDStrategyRegistryRows{rows: [][]any{baselineRow}},
		}, nil
	}))

	saved, changed, err := store.disable(context.Background(), "00000000-0000-0000-0000-000000000001", setIDStrategyRegistryDisableRequest{
		CapabilityKey:    orgUnitCreateFieldPolicyCapabilityKey,
		FieldKey:         "d_org_type",
		OrgApplicability: orgApplicabilityTenant,
		EffectiveDate:    "2026-01-01",
		DisableAsOf:      "2026-01-02",
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}
	if saved.EndDate != "2026-01-02" {
		t.Fatalf("end_date=%q", saved.EndDate)
	}
}

func TestIsRedundantIntentOverride_ErrorAndNoBaselineBranches(t *testing.T) {
	previous := defaultSetIDStrategyRegistryStore
	t.Cleanup(func() {
		useSetIDStrategyRegistryStore(previous)
	})

	intentItem := setIDStrategyRegistryItem{
		CapabilityKey:    orgUnitCreateFieldPolicyCapabilityKey,
		FieldKey:         "d_org_type",
		OrgApplicability: orgApplicabilityTenant,
		Required:         true,
		Visible:          true,
		Maintainable:     true,
		DefaultValue:     "11",
		EffectiveDate:    "2026-01-01",
	}

	t.Run("list error", func(t *testing.T) {
		useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
			listFn: func(context.Context, string, string, string, string) ([]setIDStrategyRegistryItem, error) {
				return nil, errors.New("boom")
			},
		})
		if _, err := isRedundantIntentOverride(context.Background(), "t1", intentItem); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("override decision error", func(t *testing.T) {
		item := intentItem
		item.Required = true
		item.Visible = false
		useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
			listFn: func(context.Context, string, string, string, string) ([]setIDStrategyRegistryItem, error) {
				return nil, nil
			},
		})
		if _, err := isRedundantIntentOverride(context.Background(), "t1", item); err == nil || err.Error() != fieldPolicyConflictCode {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("baseline decision error", func(t *testing.T) {
		useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
			listFn: func(_ context.Context, _ string, capabilityKey string, _ string, _ string) ([]setIDStrategyRegistryItem, error) {
				if capabilityKey == orgUnitWriteFieldPolicyCapabilityKey {
					return []setIDStrategyRegistryItem{
						{
							CapabilityKey:    orgUnitWriteFieldPolicyCapabilityKey,
							FieldKey:         "d_org_type",
							OrgApplicability: orgApplicabilityTenant,
							Required:         true,
							Visible:          false,
							Maintainable:     true,
							EffectiveDate:    "2026-01-01",
						},
					}, nil
				}
				return nil, nil
			},
		})
		if _, err := isRedundantIntentOverride(context.Background(), "t1", intentItem); err == nil || err.Error() != fieldPolicyConflictCode {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("baseline missing", func(t *testing.T) {
		useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
			listFn: func(context.Context, string, string, string, string) ([]setIDStrategyRegistryItem, error) {
				return nil, nil
			},
		})
		redundant, err := isRedundantIntentOverride(context.Background(), "t1", intentItem)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if redundant {
			t.Fatal("expected redundant=false when baseline missing")
		}
	})
}

func TestHandleSetIDStrategyRegistryAPI_RedundantCheckError(t *testing.T) {
	previous := defaultSetIDStrategyRegistryStore
	t.Cleanup(func() {
		useSetIDStrategyRegistryStore(previous)
	})
	useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
		listFn: func(context.Context, string, string, string, string) ([]setIDStrategyRegistryItem, error) {
			return nil, errors.New("boom")
		},
	})

	body := `{"capability_key":"org.orgunit_create.field_policy","owner_module":"orgunit","field_key":"d_org_type","personalization_mode":"setid","org_applicability":"tenant","required":true,"visible":true,"maintainable":true,"default_value":"11","allowed_value_codes":["11"],"priority":120,"explain_required":true,"is_stable":true,"change_policy":"plan_required","effective_date":"2026-01-01","end_date":"2026-01-02","request_id":"r-redundant-check-err"}`
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
}
