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
	if _, err := svc.resolveIntent(context.Background(), "t1", "c1", "  "); !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("unexpected err=%v", err)
	}
	if _, err := svc.resolveIntent(context.Background(), "t1", "c1", "随便聊聊"); !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("unexpected err=%v", err)
	}
	if _, err := svc.resolveIntent(context.Background(), "t1", "c1", "在鲜花组织之下，新建一个名为运营部的部门"); !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("unexpected err=%v", err)
	}

	svc.modelGateway = &assistantModelGateway{config: assistantModelConfig{Providers: nil}}
	if _, err := svc.resolveIntent(context.Background(), "t1", "c1", "新建一个部门"); !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("unexpected err=%v", err)
	}

	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1000, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}},
		adapters: map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			return []byte(`{"action":"create_orgunit"}`), nil
		})},
	}
	resolvedIntent, err := svc.resolveIntent(context.Background(), "t1", "c1", "新建一个部门")
	if err != nil {
		t.Fatalf("unexpected err=%v", err)
	}
	if resolvedIntent.Intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("intent action=%s", resolvedIntent.Intent.Action)
	}
	if got := assistantIntentValidationErrors(resolvedIntent.Intent); len(got) == 0 {
		t.Fatalf("expected missing required fields, got=%v", got)
	}

	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1000, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}},
		adapters: map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			return []byte(`{"action":"plan_only"}`), nil
		})},
	}
	resolvedIntent, err = svc.resolveIntent(context.Background(), "t1", "c1", "在 AI治理办公室 下新建 人力资源部239A补全")
	if err != nil {
		t.Fatalf("unexpected plan_only upgrade err=%v", err)
	}
	if resolvedIntent.Intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("expected create_orgunit after upgrade, got=%s", resolvedIntent.Intent.Action)
	}
	if resolvedIntent.Intent.ParentRefText != "AI治理办公室" || resolvedIntent.Intent.EntityName != "人力资源部239A补全" {
		t.Fatalf("unexpected upgraded intent=%+v", resolvedIntent.Intent)
	}
	if got := assistantIntentValidationErrors(resolvedIntent.Intent); len(got) != 1 || got[0] != "missing_effective_date" {
		t.Fatalf("expected only missing_effective_date, got=%v", got)
	}

	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1000, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}},
		adapters: map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			return []byte(`{"action":"create_orgunit","parent_ref_text":"AI治理办公室","entity_name":"人力资源部239A补全","effective_date":"2026-03-09"}`), nil
		})},
	}
	resolvedIntent, err = svc.resolveIntent(context.Background(), "t1", "c1", "在 AI治理办公室 下新建 人力资源部239A补全")
	if err != nil {
		t.Fatalf("unexpected hallucinated date normalize err=%v", err)
	}
	if resolvedIntent.Intent.EffectiveDate != "" {
		t.Fatalf("expected hallucinated effective date cleared, got=%q", resolvedIntent.Intent.EffectiveDate)
	}
	if got := assistantIntentValidationErrors(resolvedIntent.Intent); len(got) != 1 || got[0] != "missing_effective_date" {
		t.Fatalf("expected missing_effective_date after clearing hallucinated date, got=%v", got)
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
	badPlan = plan
	badPlan.CapabilityMapVersion = "cap_map_v0"
	if !assistantTurnContractVersionMismatchedForCreate(intent, badPlan) {
		t.Fatal("expected mismatch for bad capability map version")
	}
	badPlan = plan
	badPlan.SkillManifestDigest = ""
	if !assistantTurnContractVersionMismatchedForCreate(intent, badPlan) {
		t.Fatal("expected mismatch for empty skill digest")
	}
	if assistantTurnContractVersionMismatched(nil) {
		t.Fatal("nil turn should not mismatch")
	}
	if assistantTurnContractVersionMismatched(&assistantTurn{Intent: assistantIntentSpec{}, Plan: assistantPlanSummary{}}) {
		t.Fatal("empty version should not force mismatch")
	}

	originalHashFn := assistantCanonicalHashFn
	originalPlanHashFn := assistantPlanHashFn
	defer func() {
		assistantCanonicalHashFn = originalHashFn
		assistantPlanHashFn = originalPlanHashFn
	}()

	intent2 := assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}
	plan2 := assistantBuildPlan(intent2)
	plan2.SkillExecutionPlan = assistantSkillExecutionPlan{SelectedSkills: []string{"s1"}}
	dry2 := assistantBuildDryRun(intent2, nil, "")

	assistantCanonicalHashFn = func(any) string { return "" }
	if err := assistantAnnotateIntentPlan("t", "c", "输入", &intent2, &plan2, &dry2); !errors.Is(err, errAssistantPlanDeterminismViolation) {
		t.Fatalf("unexpected err=%v", err)
	}

	call := 0
	assistantCanonicalHashFn = func(any) string {
		call++
		if call == 1 {
			return "context"
		}
		return ""
	}
	if err := assistantAnnotateIntentPlan("t", "c", "输入", &intent2, &plan2, &dry2); !errors.Is(err, errAssistantPlanDeterminismViolation) {
		t.Fatalf("unexpected err=%v", err)
	}

	assistantCanonicalHashFn = originalHashFn
	assistantPlanHashFn = func(assistantIntentSpec, assistantPlanSummary, assistantDryRunResult) string { return "" }
	if err := assistantAnnotateIntentPlan("t", "c", "输入", &intent2, &plan2, &dry2); !errors.Is(err, errAssistantPlanDeterminismViolation) {
		t.Fatalf("unexpected err=%v", err)
	}

	call = 0
	assistantPlanHashFn = func(assistantIntentSpec, assistantPlanSummary, assistantDryRunResult) string {
		call++
		if call == 1 {
			return "hash-a"
		}
		return "hash-b"
	}
	if err := assistantAnnotateIntentPlan("t", "c", "输入", &intent2, &plan2, &dry2); !errors.Is(err, errAssistantPlanDeterminismViolation) {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestAssistantIntentPipeline_RetryOnSchemaInvalid(t *testing.T) {
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	attempt := 0
	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{
				{
					Name:      "openai",
					Enabled:   true,
					Model:     "gpt-5-codex",
					Endpoint:  "https://api.openai.com/v1",
					TimeoutMS: 1000,
					Retries:   0,
					Priority:  1,
					KeyRef:    "OPENAI_API_KEY",
				},
			},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
				attempt++
				if attempt == 1 {
					return []byte(`{"action":"create_orgunit","parent_ref_text":"鲜花组织","effective_date":"2026-01-01"}`), nil
				}
				return []byte(`{"action":"create_orgunit","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01"}`), nil
			}),
		},
	}
	resolved, err := svc.resolveIntent(context.Background(), "t1", "c1", "在鲜花组织之下，新建一个部门，成立日期是2026年1月1日。")
	if err != nil {
		t.Fatalf("resolve intent err=%v", err)
	}
	if attempt != 2 {
		t.Fatalf("expected one retry, attempts=%d", attempt)
	}
	if resolved.Intent.Action != assistantIntentCreateOrgUnit || resolved.Intent.ParentRefText != "鲜花组织" || resolved.Intent.EntityName != "运营部" || resolved.Intent.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected intent=%+v", resolved.Intent)
	}
}

func TestAssistantIntentPipeline_MergesPendingTurnContextForMissingFields(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  "https://api.openai.com/v1",
				TimeoutMS: 1000,
				Retries:   0,
				Priority:  1,
				KeyRef:    "OPENAI_API_KEY",
			}},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
				return []byte(`{"choices":[{"message":{"content":"fallback"}}]}`), nil
			}),
		},
	}
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	conversation := svc.createConversation("tenant-1", principal)
	created, err := svc.createTurn(context.Background(), "tenant-1", principal, conversation.ConversationID, "在鲜花组织之下，新建一个部门，成立日期是2026-01-01")
	if err != nil {
		t.Fatalf("create first turn err=%v", err)
	}
	first := created.Turns[len(created.Turns)-1]
	if first.Phase != assistantPhaseIdle {
		t.Fatalf("expected idle, got=%q", first.Phase)
	}
	if !first.RouteDecision.ClarificationRequired {
		t.Fatalf("expected route clarification required, got=%+v", first.RouteDecision)
	}
	next, err := svc.createTurn(context.Background(), "tenant-1", principal, conversation.ConversationID, "名为运营部的部门")
	if err != nil {
		t.Fatalf("create follow-up turn err=%v", err)
	}
	if len(next.Turns) != 2 {
		t.Fatalf("expected 2 turns, got=%d", len(next.Turns))
	}
	merged := next.Turns[len(next.Turns)-1]
	if merged.Intent.ParentRefText != "鲜花组织" || merged.Intent.EntityName != "运营部" || merged.Intent.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected merged intent=%+v", merged.Intent)
	}
	if merged.Phase != assistantPhaseAwaitCommitConfirm {
		t.Fatalf("expected await_commit_confirm, got=%q", merged.Phase)
	}
	if merged.ResolvedCandidateID == "" {
		t.Fatal("expected candidate resolved from pending context")
	}
}

func TestAssistantIntentPipeline_ResolveIntentErrorBranches(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")

	var nilSvc *assistantConversationService
	if _, err := nilSvc.resolveIntent(context.Background(), "t1", "c1", "新建一个部门"); !errors.Is(err, errAssistantServiceMissing) {
		t.Fatalf("expected service missing, got=%v", err)
	}

	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	svc.gatewayErr = errAssistantRuntimeConfigInvalid
	if _, err := svc.resolveIntent(context.Background(), "t1", "c1", "新建一个部门"); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
		t.Fatalf("expected runtime config invalid, got=%v", err)
	}

	svc.gatewayErr = nil
	attempt := 0
	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{
				{
					Name:      "openai",
					Enabled:   true,
					Model:     "gpt-5-codex",
					Endpoint:  "https://api.openai.com/v1",
					TimeoutMS: 1000,
					Retries:   0,
					Priority:  1,
					KeyRef:    "OPENAI_API_KEY",
				},
			},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
				attempt++
				if attempt == 1 {
					return []byte(`{"action":"create_orgunit"}`), nil
				}
				return nil, errAssistantModelTimeout
			}),
		},
	}
	if _, err := svc.resolveIntent(context.Background(), "t1", "c1", "在鲜花组织之下，新建一个名为运营部的部门"); !errors.Is(err, errAssistantModelTimeout) {
		t.Fatalf("expected timeout on retry, got=%v", err)
	}
}

func TestAssistantIntentPipeline_FallbackToLocalIntentOnStrictDecodeFailure(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  "https://api.openai.com/v1",
				TimeoutMS: 1000,
				Retries:   0,
				Priority:  1,
				KeyRef:    "OPENAI_API_KEY",
			}},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
				return []byte(`{"choices":[{"message":{"content":"好的，我来帮你创建部门"}}]}`), nil
			}),
		},
	}

	resolved, err := svc.resolveIntent(context.Background(), "tenant-1", "conv-1", "在鲜花组织之下，新建一个名为运营部的部门。")
	if err != nil {
		t.Fatalf("resolve intent err=%v", err)
	}
	if resolved.ProviderName != "deterministic" {
		t.Fatalf("provider=%s", resolved.ProviderName)
	}
	if resolved.ModelName != "builtin-intent-extractor" {
		t.Fatalf("model=%s", resolved.ModelName)
	}
	if resolved.Intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("intent action=%s", resolved.Intent.Action)
	}
	if resolved.Intent.ParentRefText != "鲜花组织" || resolved.Intent.EntityName != "运营部" {
		t.Fatalf("unexpected intent=%+v", resolved.Intent)
	}
	if resolved.Intent.EffectiveDate != "" {
		t.Fatalf("expected missing effective date for follow-up, got=%q", resolved.Intent.EffectiveDate)
	}
}

func TestAssistantIntentPipeline_LocalFactHelpers(t *testing.T) {
	t.Run("overlay keeps non create action unchanged", func(t *testing.T) {
		intent := assistantIntentSpec{Action: assistantIntentPlanOnly, EffectiveDate: "2026-03-09"}
		local := assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}
		got := assistantOverlayExplicitIntentFacts(intent, local)
		if got != intent {
			t.Fatalf("unexpected overlay=%+v", got)
		}
	})

	t.Run("overlay fills missing parent entity and explicit date", func(t *testing.T) {
		intent := assistantIntentSpec{Action: assistantIntentCreateOrgUnit}
		local := assistantIntentSpec{ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}
		got := assistantOverlayExplicitIntentFacts(intent, local)
		if got.ParentRefText != "鲜花组织" || got.EntityName != "运营部" || got.EffectiveDate != "2026-01-01" {
			t.Fatalf("unexpected overlay=%+v", got)
		}
	})

	t.Run("overlay clears hallucinated date when user omitted it", func(t *testing.T) {
		intent := assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-03-09"}
		got := assistantOverlayExplicitIntentFacts(intent, assistantIntentSpec{})
		if got.EffectiveDate != "" {
			t.Fatalf("expected cleared date, got=%+v", got)
		}
	})

	t.Run("normalize upgrades plan only from local create intent", func(t *testing.T) {
		resolved, upgraded := assistantNormalizeResolvedIntentWithLocalFacts(
			assistantResolveIntentResult{Intent: assistantIntentSpec{Action: assistantIntentPlanOnly}},
			assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部"},
		)
		if !upgraded || resolved.Intent.Action != assistantIntentCreateOrgUnit || resolved.Intent.ParentRefText != "鲜花组织" {
			t.Fatalf("resolved=%+v upgraded=%v", resolved, upgraded)
		}
	})

	t.Run("normalize keeps provider create intent when no upgrade needed", func(t *testing.T) {
		resolved, upgraded := assistantNormalizeResolvedIntentWithLocalFacts(
			assistantResolveIntentResult{Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-03-09"}},
			assistantIntentSpec{EffectiveDate: "2026-03-25"},
		)
		if upgraded {
			t.Fatal("unexpected upgrade")
		}
		if resolved.Intent.EffectiveDate != "2026-03-25" {
			t.Fatalf("resolved=%+v", resolved)
		}
	})
}

func TestAssistantIntentPipeline_RetryInvalidThenFallbackLocal(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	attempt := 0
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  "https://api.openai.com/v1",
				TimeoutMS: 1000,
				Retries:   0,
				Priority:  1,
				KeyRef:    "OPENAI_API_KEY",
			}},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
				attempt++
				return []byte(`{"action":"create_orgunit","parent_ref_text":"鲜花组织","effective_date":"2026-01-01"}`), nil
			}),
		},
	}

	resolved, err := svc.resolveIntent(context.Background(), "tenant-1", "conv-1", "在鲜花组织之下，新建一个部门，成立日期是2026-01-01")
	if err != nil {
		t.Fatalf("resolve intent err=%v", err)
	}
	if attempt != 2 {
		t.Fatalf("expected two attempts, got=%d", attempt)
	}
	if resolved.ProviderName != "deterministic" || resolved.ModelName != "builtin-intent-extractor" {
		t.Fatalf("expected local fallback, got=%+v", resolved)
	}
	if resolved.Intent.Action != assistantIntentCreateOrgUnit || resolved.Intent.ParentRefText != "鲜花组织" || resolved.Intent.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected intent=%+v", resolved.Intent)
	}
}

func TestAssistantIntentPipeline_RetryPlanOnlyUpgradeAfterInvalidFirstPass(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	attempt := 0
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  "https://api.openai.com/v1",
				TimeoutMS: 1000,
				Retries:   0,
				Priority:  1,
				KeyRef:    "OPENAI_API_KEY",
			}},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
				attempt++
				if attempt == 1 {
					return []byte(`{"action":"create_orgunit"}`), nil
				}
				return []byte(`{"action":"plan_only"}`), nil
			}),
		},
	}
	resolved, err := svc.resolveIntent(context.Background(), "tenant-1", "conv-1", "在 AI治理办公室 下新建 人力资源部239A补全")
	if err != nil {
		t.Fatalf("resolve intent err=%v", err)
	}
	if attempt != 2 {
		t.Fatalf("expected two attempts, got=%d", attempt)
	}
	if resolved.Intent.Action != assistantIntentCreateOrgUnit || resolved.Intent.ParentRefText != "AI治理办公室" || resolved.Intent.EntityName != "人力资源部239A补全" {
		t.Fatalf("unexpected intent=%+v", resolved.Intent)
	}
}

func TestAssistantIntentPipeline_UnsupportedActionUpgradeFromLocalFacts(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  "https://api.openai.com/v1",
				TimeoutMS: 1000,
				Retries:   0,
				Priority:  1,
				KeyRef:    "OPENAI_API_KEY",
			}},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
				return []byte(`{"action":"unsupported_action","effective_date":"2026-01-01"}`), nil
			}),
		},
	}

	resolved, err := svc.resolveIntent(context.Background(), "tenant-1", "conv-1", "在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01")
	if err != nil {
		t.Fatalf("resolve intent err=%v", err)
	}
	if resolved.Intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("expected create_orgunit after unsupported action upgrade, got=%s", resolved.Intent.Action)
	}
	if resolved.Intent.ParentRefText != "AI治理办公室" || resolved.Intent.EntityName != "人力资源部2" || resolved.Intent.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected upgraded intent=%+v", resolved.Intent)
	}
	if resolved.ProviderName != "openai" || resolved.ModelName != "gpt-5-codex" {
		t.Fatalf("expected real provider metadata preserved, got=%+v", resolved)
	}
}
