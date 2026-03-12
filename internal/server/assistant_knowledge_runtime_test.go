package server

import (
	"strings"
	"testing"
)

func TestAssistantKnowledgeRuntime_LoadAndRoute(t *testing.T) {
	runtime, err := assistantLoadKnowledgeRuntime()
	if err != nil {
		t.Fatalf("load knowledge runtime err=%v", err)
	}
	if strings.TrimSpace(runtime.SnapshotDigest) == "" {
		t.Fatal("knowledge snapshot digest should not be empty")
	}
	if strings.TrimSpace(runtime.RouteCatalogVersion) == "" || strings.TrimSpace(runtime.ResolverContractVersion) == "" || strings.TrimSpace(runtime.ContextTemplateVersion) == "" {
		t.Fatalf("versions should not be empty runtime=%+v", runtime)
	}

	business := runtime.routeIntent("在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01", assistantIntentSpec{Action: assistantIntentCreateOrgUnit})
	if business.RouteKind != assistantRouteKindBusinessAction || business.IntentID != "org.orgunit_create" {
		t.Fatalf("unexpected business route intent=%+v", business)
	}

	qa := runtime.routeIntent("你好，想问下系统支持哪些功能？", assistantIntentSpec{Action: assistantIntentPlanOnly})
	if qa.RouteKind != assistantRouteKindKnowledgeQA && qa.RouteKind != assistantRouteKindChitchat {
		t.Fatalf("unexpected non-business route intent=%+v", qa)
	}

	unknown := runtime.routeIntent("随机输入无关键词", assistantIntentSpec{Action: assistantIntentPlanOnly})
	if unknown.RouteKind != assistantRouteKindUncertain {
		t.Fatalf("expected uncertain route, got=%+v", unknown)
	}
}

func TestAssistantKnowledgeRuntime_BuildPlanContextAndApply(t *testing.T) {
	runtime, err := assistantLoadKnowledgeRuntime()
	if err != nil {
		t.Fatalf("load knowledge runtime err=%v", err)
	}
	spec, ok := assistantLookupDefaultActionSpec(assistantIntentCreateOrgUnit)
	if !ok {
		t.Fatal("create_orgunit spec missing")
	}

	intent := assistantIntentSpec{Action: assistantIntentCreateOrgUnit, RouteKind: assistantRouteKindBusinessAction}
	turn := &assistantTurn{
		TurnID:     "turn_1",
		Phase:      assistantPhaseAwaitMissingFields,
		RequestID:  "req_1",
		TraceID:    "trace_1",
		DryRun:     assistantDryRunResult{ValidationErrors: []string{"missing_parent_ref_text"}},
		Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", Name: "鲜花组织"}},
	}
	ctx, err := runtime.buildPlanContextV1("tenant_1", "zh", intent, spec, turn)
	if err != nil {
		t.Fatalf("build plan context err=%v", err)
	}
	if !strings.Contains(ctx.ActionViewSummary, "创建") {
		t.Fatalf("unexpected action view summary=%q", ctx.ActionViewSummary)
	}

	plan := assistantBuildPlan(intent)
	dryRun := assistantDryRunResult{ValidationErrors: []string{"missing_parent_ref_text"}, Explain: "old"}
	assistantApplyPlanContextV1(&plan, &dryRun, intent, ctx)
	if !strings.Contains(dryRun.Explain, "上级组织") {
		t.Fatalf("expected knowledge guidance explain, got=%q", dryRun.Explain)
	}

	nonBusinessIntent := assistantIntentSpec{Action: assistantIntentPlanOnly, IntentID: "knowledge.general_qa", RouteKind: assistantRouteKindKnowledgeQA}
	nonBusinessCtx, err := runtime.buildPlanContextV1("tenant_1", "zh", nonBusinessIntent, assistantActionSpec{}, nil)
	if err != nil {
		t.Fatalf("build non-business context err=%v", err)
	}
	nonBusinessDryRun := assistantDryRunResult{ValidationErrors: nil, Explain: "before"}
	assistantApplyPlanContextV1(&plan, &nonBusinessDryRun, nonBusinessIntent, nonBusinessCtx)
	if !assistantContainsString(nonBusinessDryRun.ValidationErrors, "non_business_route") {
		t.Fatalf("expected non_business_route validation error, got=%v", nonBusinessDryRun.ValidationErrors)
	}
	if !strings.Contains(nonBusinessDryRun.Explain, "不会触发") {
		t.Fatalf("expected non-business explain, got=%q", nonBusinessDryRun.Explain)
	}
}

func TestAssistantKnowledgeRuntime_CompileValidation(t *testing.T) {
	catalog := assistantIntentRouteCatalog{
		AssetType:           "intent_route_catalog",
		RouteCatalogVersion: "2026-03-11.v1",
		SourceRefs:          []string{"docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md"},
		Entries: []assistantIntentRouteEntry{
			{IntentID: "org.orgunit_create", RouteKind: assistantRouteKindBusinessAction, ActionID: assistantIntentCreateOrgUnit},
		},
	}
	interpretation := []assistantInterpretationPack{
		{
			AssetType:        "interpretation_pack",
			PackID:           "knowledge.general_qa",
			KnowledgeVersion: "2026-03-11.v1",
			Locale:           "zh",
			IntentClasses:    []string{assistantRouteKindKnowledgeQA},
			ClarificationPrompts: []assistantKnowledgePrompt{
				{TemplateID: "clarify.knowledge.general_qa.v1", Text: "这是知识问答场景"},
			},
			SourceRefs: []string{"docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md"},
		},
	}
	replyGuidance := []assistantReplyGuidancePack{
		{
			AssetType:        "reply_guidance_pack",
			ReplyKind:        "missing_fields",
			KnowledgeVersion: "2026-03-11.v1",
			Locale:           "zh",
			GuidanceTemplates: []assistantKnowledgePrompt{
				{TemplateID: "reply.missing_fields.v1", Text: "请补充：{missing_fields}"},
			},
			SourceRefs: []string{"docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md"},
		},
	}

	t.Run("rejects invalid template field", func(t *testing.T) {
		actionViews := []assistantActionViewPack{
			{
				AssetType:        "action_view_pack",
				ActionID:         assistantIntentCreateOrgUnit,
				KnowledgeVersion: "2026-03-11.v1",
				Locale:           "zh",
				Summary:          "summary",
				TemplateFields:   []string{"invalid_template_field"},
				SourceRefs:       []string{"internal/server/assistant_action_registry.go"},
			},
		}
		_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, map[string][]byte{"x.json": []byte(`{"ok":true}`)})
		if err == nil || !strings.Contains(err.Error(), "template field not allowed") {
			t.Fatalf("expected invalid template field error, got=%v", err)
		}
	})

	t.Run("rejects forbidden key", func(t *testing.T) {
		actionViews := []assistantActionViewPack{
			{
				AssetType:        "action_view_pack",
				ActionID:         assistantIntentCreateOrgUnit,
				KnowledgeVersion: "2026-03-11.v1",
				Locale:           "zh",
				Summary:          "summary",
				TemplateFields:   []string{"action_view_pack.summary"},
				SourceRefs:       []string{"internal/server/assistant_action_registry.go"},
			},
		}
		_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, map[string][]byte{"x.json": []byte(`{"required_fields":["x"]}`)})
		if err == nil || !strings.Contains(err.Error(), "forbidden key") {
			t.Fatalf("expected forbidden key error, got=%v", err)
		}
	})
}

func assistantContainsString(items []string, expected string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == strings.TrimSpace(expected) {
			return true
		}
	}
	return false
}
