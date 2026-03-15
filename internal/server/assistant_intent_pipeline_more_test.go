package server

import (
	"context"
	"errors"
	"strings"
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
			return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create"}`), nil
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
			return []byte(`{"action":"plan_only","route_kind":"knowledge_qa","intent_id":"knowledge.general_qa"}`), nil
		})},
	}
	resolvedIntent, err = svc.resolveIntent(context.Background(), "t1", "c1", "在 AI治理办公室 下新建 人力资源部239A补全")
	if err != nil {
		t.Fatalf("unexpected plan_only resolve err=%v", err)
	}
	if resolvedIntent.Intent.Action != assistantIntentPlanOnly {
		t.Fatalf("expected model plan_only preserved, got=%s", resolvedIntent.Intent.Action)
	}

	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1000, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}},
		adapters: map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"AI治理办公室","entity_name":"人力资源部239A补全","effective_date":"2026-03-09"}`), nil
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

	svc.modelGateway = &assistantModelGateway{
		config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1000, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}},
		adapters: map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create"}`), nil
		})},
	}
	resolvedIntent, err = svc.resolveIntent(context.Background(), "t1", "c1", "在 AI治理办公室 下新建 人力资源部239A补全")
	if err != nil {
		t.Fatalf("unexpected explicit-slot preserve err=%v", err)
	}
	if resolvedIntent.Intent.ParentRefText != "" || resolvedIntent.Intent.EntityName != "" {
		t.Fatalf("expected no local slot supplementation, got=%+v", resolvedIntent.Intent)
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
					return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","effective_date":"2026-01-01"}`), nil
				}
				return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01"}`), nil
			}),
		},
	}
	resolved, err := svc.resolveIntent(context.Background(), "t1", "c1", "在鲜花组织之下，新建一个部门，成立日期是2026年1月1日。")
	if err != nil {
		t.Fatalf("resolve intent err=%v", err)
	}
	if attempt != 1 {
		t.Fatalf("expected semantic core to accept partial intent without retry, attempts=%d", attempt)
	}
	if resolved.Intent.Action != assistantIntentCreateOrgUnit || resolved.Intent.ParentRefText != "鲜花组织" || resolved.Intent.EntityName != "" || resolved.Intent.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected intent=%+v", resolved.Intent)
	}
	if got := assistantIntentValidationErrors(resolved.Intent); len(got) != 1 || got[0] != "missing_entity_name" {
		t.Fatalf("expected missing_entity_name, got=%v", got)
	}
}

func TestAssistantIntentPipeline_MergesPendingTurnContextForMissingFields(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	attempt := 0
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
					return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","effective_date":"2026-01-01","user_visible_reply":"请补充部门名称。","next_question":"请告诉我部门名称。","readiness":"need_more_info"}`), nil
				}
				return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01","user_visible_reply":"我已补齐草案，请确认。","readiness":"ready_for_confirm"}`), nil
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
	if first.Phase != assistantPhaseAwaitMissingFields {
		t.Fatalf("expected await_missing_fields, got=%q", first.Phase)
	}
	if first.ReplyNLG == nil || strings.TrimSpace(first.ReplyNLG.Text) == "" {
		t.Fatalf("expected semantic reply seeded on first turn, got=%+v", first.ReplyNLG)
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
		t.Fatalf("unexpected model-owned intent=%+v", merged.Intent)
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
				return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create"}`), nil
			}),
		},
	}
	resolved, err := svc.resolveIntent(context.Background(), "t1", "c1", "在鲜花组织之下，新建一个名为运营部的部门")
	if err != nil {
		t.Fatalf("unexpected err=%v", err)
	}
	if attempt != 1 || resolved.Intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("unexpected resolved=%+v attempts=%d", resolved, attempt)
	}
}

func TestAssistantIntentPipeline_FailsClosedWithoutSemanticRetryOnInvalidFirstPass(t *testing.T) {
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
				return []byte(`{"action":"unsupported_action","route_kind":"business_action","intent_id":"org.unsupported_action"}`), nil
			}),
		},
	}

	if _, err := svc.resolveIntent(context.Background(), "tenant-1", "conv-1", "创建一个部门"); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected invalid semantic contract failure, got=%v", err)
	}
	if attempt != 1 {
		t.Fatalf("expected no semantic retry after invalid first pass, attempts=%d", attempt)
	}
}

func TestAssistantIntentPipeline_DoesNotRetryEvenIfSecondSemanticPassWouldSucceed(t *testing.T) {
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
					return []byte(`{"action":"unsupported_action","route_kind":"business_action","intent_id":"org.unsupported_action"}`), nil
				}
				return []byte(`{"action":"plan_only","route_kind":"knowledge_qa","intent_id":"knowledge.general_qa"}`), nil
			}),
		},
	}

	if _, err := svc.resolveIntent(context.Background(), "tenant-1", "conv-1", "系统有哪些功能"); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected invalid semantic contract failure, got=%v", err)
	}
	if attempt != 1 {
		t.Fatalf("expected no semantic retry after invalid first pass, attempts=%d", attempt)
	}
}

func TestAssistantIntentPipeline_FailsClosedOnStrictDecodeFailure(t *testing.T) {
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

	if _, err := svc.resolveIntent(context.Background(), "tenant-1", "conv-1", "在鲜花组织之下，新建一个名为运营部的部门。"); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected strict decode failure, got=%v", err)
	}
}

func TestAssistantIntentPipeline_LocalTemporalHelpers(t *testing.T) {
	t.Run("extract explicit temporal hints", func(t *testing.T) {
		if got := assistantExtractExplicitTemporalHints("2026-01-01"); got.EffectiveDate != "2026-01-01" || got.TargetEffectiveDate != "2026-01-01" {
			t.Fatalf("unexpected iso hints=%+v", got)
		}
		if got := assistantExtractExplicitTemporalHints("2026年1月2日"); got.EffectiveDate != "2026-01-02" || got.TargetEffectiveDate != "2026-01-02" {
			t.Fatalf("unexpected cn hints=%+v", got)
		}
		if got := assistantExtractExplicitTemporalHints("补充名称"); got.EffectiveDate != "" || got.TargetEffectiveDate != "" {
			t.Fatalf("unexpected empty hints=%+v", got)
		}
	})
}

func TestAssistantIntentPipeline_RequiresSemanticRouteContractForCreate(t *testing.T) {
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

	_, err := svc.resolveIntent(context.Background(), "tenant-1", "conv-1", "在鲜花组织之下，新建一个部门，成立日期是2026-01-01")
	if !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected missing route contract to fail closed, got=%v", err)
	}
	if attempt != 1 {
		t.Fatalf("expected no semantic retry for missing route contract, got=%d", attempt)
	}
}

func TestAssistantIntentPipeline_FirstSemanticPassNoLongerSupplementsExplicitSlots(t *testing.T) {
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
					return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create"}`), nil
				}
				return []byte(`{"action":"plan_only","route_kind":"knowledge_qa","intent_id":"knowledge.general_qa"}`), nil
			}),
		},
	}
	resolved, err := svc.resolveIntent(context.Background(), "tenant-1", "conv-1", "在 AI治理办公室 下新建 人力资源部239A补全")
	if err != nil {
		t.Fatalf("resolve intent err=%v", err)
	}
	if attempt != 1 {
		t.Fatalf("expected first semantic result accepted without retry, got=%d", attempt)
	}
	if resolved.Intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("unexpected action=%+v", resolved.Intent)
	}
	if resolved.Intent.ParentRefText != "" || resolved.Intent.EntityName != "" {
		t.Fatalf("unexpected intent=%+v", resolved.Intent)
	}
}

func TestAssistantIntentPipeline_UnsupportedActionFailsClosed(t *testing.T) {
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
				return []byte(`{"action":"unsupported_action","route_kind":"business_action","intent_id":"org.unsupported_action","effective_date":"2026-01-01"}`), nil
			}),
		},
	}

	if _, err := svc.resolveIntent(context.Background(), "tenant-1", "conv-1", "在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01"); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected unsupported action to fail closed, got=%v", err)
	}
}
