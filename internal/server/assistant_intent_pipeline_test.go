package server

import (
	"context"
	"os"
	"testing"
)

func TestAssistantModelGatewayResolveIntentFallbackAndValidation(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	t.Setenv("DEEPSEEK_API_KEY", "dummy")

	t.Setenv("ASSISTANT_MODEL_CONFIG_JSON", `{"provider_routing":{"strategy":"priority_failover","fallback_enabled":true},"providers":[{"name":"openai","enabled":true,"model":"gpt-5-codex","endpoint":"https://api.openai.com/v1","timeout_ms":200,"retries":0,"priority":10,"key_ref":"OPENAI_API_KEY"}]}`)
	gateway, err := newAssistantModelGateway()
	if err != nil {
		t.Fatalf("new gateway err=%v", err)
	}
	_, errs := gateway.applyConfig(assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{
				Name:      "openai",
				Enabled:   true,
				Model:     "timeout-model",
				Endpoint:  "https://api.openai.com/v1",
				TimeoutMS: 200,
				Retries:   0,
				Priority:  10,
				KeyRef:    "OPENAI_API_KEY",
			},
			{
				Name:      "deepseek",
				Enabled:   true,
				Model:     "deepseek-chat",
				Endpoint:  "https://api.deepseek.com",
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
	gateway.adapters = map[string]assistantProviderAdapter{
		"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			return nil, errAssistantModelTimeout
		}),
		"deepseek": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			return []byte(`{"action":"create_orgunit","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01"}`), nil
		}),
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
			Endpoint:  "https://api.unknown.example",
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

func TestAssistantStrictDecodeIntentExpandedFields(t *testing.T) {
	intent, err := assistantStrictDecodeIntent([]byte(`{"action":"move_orgunit","org_code":"FLOWER-C","effective_date":"2026-04-01","new_parent_ref_text":"共享服务中心"}`))
	if err != nil {
		t.Fatalf("strict decode err=%v", err)
	}
	if intent.Action != assistantIntentMoveOrgUnit || intent.OrgCode != "FLOWER-C" || intent.NewParentRefText != "共享服务中心" {
		t.Fatalf("unexpected intent=%+v", intent)
	}
}

func TestAssistantIntentPipelineExpandedActions(t *testing.T) {
	moveIntent := assistantIntentSpec{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01", NewParentRefText: "共享服务中心"}
	movePlan := assistantBuildPlan(moveIntent)
	if movePlan.ActionID != assistantIntentMoveOrgUnit || movePlan.CommitAdapterKey != "orgunit_move_v1" {
		t.Fatalf("unexpected move plan=%+v", movePlan)
	}
	moveSkill, moveDelta := assistantCompileIntentToPlans(moveIntent, "FLOWER-B")
	if len(moveSkill.SelectedSkills) != 1 || moveSkill.SelectedSkills[0] != "org.orgunit_move" {
		t.Fatalf("unexpected move skill=%+v", moveSkill)
	}
	if moveDelta.CapabilityKey != "org.orgunit_write.field_policy" || len(moveDelta.Changes) == 0 {
		t.Fatalf("unexpected move delta=%+v", moveDelta)
	}
	moveDryRun := assistantBuildDryRun(moveIntent, []assistantCandidate{{CandidateID: "FLOWER-B", CandidateCode: "FLOWER-B"}}, "FLOWER-B")
	if len(moveDryRun.ValidationErrors) != 0 {
		t.Fatalf("unexpected move validation=%v", moveDryRun.ValidationErrors)
	}

	correctIntent := assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, OrgCode: "FLOWER-C", TargetEffectiveDate: "2026-01-01", NewName: "运营中心"}
	if errs := assistantIntentValidationErrors(correctIntent); len(errs) != 0 {
		t.Fatalf("unexpected correct errs=%v", errs)
	}
	missingMove := assistantIntentSpec{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01"}
	if errs := assistantIntentValidationErrors(missingMove); len(errs) == 0 || errs[0] != "missing_new_parent_ref_text" {
		t.Fatalf("unexpected missing move errs=%v", errs)
	}
}
