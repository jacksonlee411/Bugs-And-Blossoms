package server

import (
	"context"
	"os"
	"testing"
)

func TestAssistantModelGatewayResolveIntentFallbackAndValidation(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")

	gateway := newAssistantModelGateway()
	_, errs := gateway.applyConfig(assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-4o-mini",
				Endpoint:  "simulate://timeout",
				TimeoutMS: 200,
				Retries:   0,
				Priority:  10,
				KeyRef:    "OPENAI_API_KEY",
			},
			{
				Name:      "deepseek",
				Enabled:   true,
				Model:     "deepseek-chat",
				Endpoint:  "builtin://deepseek",
				TimeoutMS: 200,
				Retries:   0,
				Priority:  20,
				KeyRef:    "DEEPSEEK_API_KEY",
			},
		},
	})
	if len(errs) != 0 {
		t.Fatalf("apply config errs=%v", errs)
	}

	resolved, err := gateway.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。"})
	if err != nil {
		t.Fatalf("resolve intent err=%v", err)
	}
	if resolved.ProviderName != "deepseek" {
		t.Fatalf("provider=%s", resolved.ProviderName)
	}
	if resolved.Intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("intent action=%s", resolved.Intent.Action)
	}

	gateway.mu.Lock()
	gateway.config = assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{{
			Name:      "openai",
			Enabled:   true,
			Model:     "gpt-4o-mini",
			Endpoint:  "https://example.invalid/v1/chat",
			TimeoutMS: 300,
			Retries:   0,
			Priority:  10,
			KeyRef:    "OPENAI_API_KEY",
		}},
	}
	gateway.mu.Unlock()
	if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
		t.Fatalf("unset env: %v", err)
	}
	_, err = gateway.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "新建一个部门"})
	if err != errAssistantModelSecretMissing {
		t.Fatalf("want secret missing err, got=%v", err)
	}

	gateway.mu.Lock()
	gateway.config = assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{{
			Name:      "unknown",
			Enabled:   true,
			Model:     "x",
			Endpoint:  "builtin://unknown",
			TimeoutMS: 100,
			Retries:   0,
			Priority:  1,
			KeyRef:    "UNKNOWN_KEY",
		}},
	}
	gateway.mu.Unlock()
	_, err = gateway.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "新建一个部门"})
	if err != errAssistantModelConfigInvalid {
		t.Fatalf("want config invalid err, got=%v", err)
	}

	if !errorsIsAny(errAssistantModelTimeout, errAssistantModelTimeout, errAssistantModelRateLimited) {
		t.Fatal("errorsIsAny should match explicit target")
	}
}

func TestAssistantIntentPipelineDeterminismAndContracts(t *testing.T) {
	intent := assistantIntentSpec{
		Action:        assistantIntentCreateOrgUnit,
		EntityName:    "运营部",
		EffectiveDate: "2026-01-01",
	}
	plan := assistantBuildPlan(intent)
	skillPlan, deltaPlan := assistantCompileIntentToPlans(intent, "")
	plan.SkillExecutionPlan = skillPlan
	plan.ConfigDeltaPlan = deltaPlan
	dryRun := assistantBuildDryRun(intent, nil, "")

	if err := assistantAnnotateIntentPlan("tenant-a", "conv-a", "新建运营部", &intent, &plan, &dryRun); err != nil {
		t.Fatalf("annotate err=%v", err)
	}
	if intent.IntentSchemaVersion == "" || intent.ContextHash == "" || intent.IntentHash == "" {
		t.Fatalf("intent annotations missing: %+v", intent)
	}
	if plan.CompilerContractVersion == "" || plan.CapabilityMapVersion == "" || plan.SkillManifestDigest == "" {
		t.Fatalf("plan annotations missing: %+v", plan)
	}
	if dryRun.PlanHash == "" {
		t.Fatal("plan_hash should not be empty")
	}

	hashA := assistantPlanHash(intent, plan, dryRun)
	hashB := assistantPlanHash(intent, plan, dryRun)
	if hashA == "" || hashA != hashB {
		t.Fatalf("plan hash unstable: %q != %q", hashA, hashB)
	}

	turn := &assistantTurn{Intent: intent, Plan: plan}
	if assistantTurnContractVersionMismatched(turn) {
		t.Fatal("contract should match after annotation")
	}
	turn.Plan.CompilerContractVersion = "assistant.compiler.v0"
	if !assistantTurnContractVersionMismatched(turn) {
		t.Fatal("contract mismatch should be detected")
	}

	if err := assistantAnnotateIntentPlan("tenant-a", "conv-a", "新建运营部", nil, &plan, &dryRun); err != errAssistantPlanDeterminismViolation {
		t.Fatalf("want determinism violation, got=%v", err)
	}

	if got := assistantCanonicalHash(map[string]any{"bad": func() {}}); got != "" {
		t.Fatalf("marshal failure should return empty hash, got=%q", got)
	}
	if got := assistantSkillManifestDigest(nil); got != "" {
		t.Fatalf("empty skill digest should be empty, got=%q", got)
	}
}

func TestAssistantStrictDecodeIntent(t *testing.T) {
	intent, err := assistantStrictDecodeIntent([]byte(`{"action":"create_orgunit"}`))
	if err != nil {
		t.Fatalf("strict decode err=%v", err)
	}
	if intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("action=%s", intent.Action)
	}

	if _, err := assistantStrictDecodeIntent([]byte(`{"action":"create_orgunit","extra":1}`)); err == nil {
		t.Fatal("unknown field should fail strict decode")
	}

	if _, err := assistantStrictDecodeIntent([]byte(`{"action":"create_orgunit"}{}`)); err != errAssistantPlanSchemaConstrainedDecodeFailed {
		t.Fatalf("want constrained decode failed, got=%v", err)
	}
}
