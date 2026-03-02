package server

import (
	"context"
	"errors"
	"testing"
)

func TestAssistantIntentPipeline_Branches(t *testing.T) {
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})

	if _, err := svc.resolveIntent(context.Background(), "t1", "c1", "DROP TABLE orgunit"); !errors.Is(err, errAssistantPlanBoundaryViolation) {
		t.Fatalf("unexpected err=%v", err)
	}

	svc.modelGateway = nil
	if resolved, err := svc.resolveIntent(context.Background(), "t1", "c1", "  "); err != nil || resolved.ProviderName != "builtin" {
		t.Fatalf("unexpected resolved=%+v err=%v", resolved, err)
	}
	if resolved, err := svc.resolveIntent(context.Background(), "t1", "c1", "随便聊聊"); err != nil || resolved.ProviderName != "builtin" {
		t.Fatalf("unexpected resolved=%+v err=%v", resolved, err)
	}

	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1000, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}},
		adapters: map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			return []byte(`{"action":"create_orgunit"}`), nil
		})},
	}
	if _, err := svc.resolveIntent(context.Background(), "t1", "c1", "新建一个部门"); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("unexpected err=%v", err)
	}

	intent := assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}
	plan := assistantBuildPlan(intent)
	plan.SkillExecutionPlan = assistantSkillExecutionPlan{SelectedSkills: []string{"s1"}}
	plan.ConfigDeltaPlan = assistantConfigDeltaPlan{CapabilityKey: plan.CapabilityKey}
	dry := assistantBuildDryRun(intent, nil, "")
	if err := assistantAnnotateIntentPlan("t", "c", "输入", &intent, &plan, &dry); err != nil {
		t.Fatalf("annotate err=%v", err)
	}

	badPlan := plan
	badPlan.CompilerContractVersion = "broken"
	if !assistantTurnContractVersionMismatchedForCreate(intent, badPlan) {
		t.Fatal("expected mismatch for bad compiler version")
	}

	if !assistantTurnContractVersionMismatchedForCreate(assistantIntentSpec{IntentSchemaVersion: "v0"}, plan) {
		t.Fatal("expected mismatch for bad intent version")
	}
	if assistantTurnContractVersionMismatched(nil) {
		t.Fatal("nil turn should not mismatch")
	}
	if assistantTurnContractVersionMismatched(&assistantTurn{Intent: assistantIntentSpec{}, Plan: assistantPlanSummary{}}) {
		t.Fatal("empty version should not force mismatch")
	}
}
