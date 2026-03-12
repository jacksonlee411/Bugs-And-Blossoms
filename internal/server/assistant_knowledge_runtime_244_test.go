package server

import (
	"strings"
	"testing"
)

func TestAssistantCompileKnowledgeRuntime_InterpretationTemplateRefsValid(t *testing.T) {
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	catalog.Entries[1].ClarificationTemplateID = "qa.zh"

	runtime, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err != nil {
		t.Fatalf("compile knowledge runtime err=%v", err)
	}
	if runtime == nil {
		t.Fatal("runtime should not be nil")
	}
	if packID := strings.TrimSpace(runtime.routePackID["knowledge.general_qa"]); packID != assistantInterpretationDefaultPackID {
		t.Fatalf("unexpected pack id mapping=%q", packID)
	}
}

func TestAssistantCompileKnowledgeRuntime_RouteCatalogCrossRefsValid(t *testing.T) {
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	interpretation = append(interpretation, assistantInterpretationPack{
		AssetType:            "interpretation_pack",
		PackID:               "org.orgunit_create",
		KnowledgeVersion:     "2026-03-11.v1",
		Locale:               "zh",
		IntentClasses:        []string{assistantRouteKindBusinessAction},
		ClarificationPrompts: []assistantKnowledgePrompt{{TemplateID: "clarify.org.orgunit_create.v1", Text: "请补充父组织、名称和生效日期。"}},
		NegativeExamples:     []string{"hello"},
		SourceRefs:           []string{"internal/server/assistant_knowledge_runtime.go"},
	})
	catalog.Entries[0].ClarificationTemplateID = "clarify.org.orgunit_create.v1"
	catalog.Entries[0].RequiredSlots = []string{"parent_ref_text", "entity_name", "effective_date"}

	runtime, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err != nil {
		t.Fatalf("compile knowledge runtime err=%v", err)
	}
	if runtime == nil {
		t.Fatal("runtime should not be nil")
	}
	if got := strings.TrimSpace(runtime.routeByIntent["org.orgunit_create"].ClarificationTemplateID); got != "clarify.org.orgunit_create.v1" {
		t.Fatalf("unexpected clarification_template_id=%q", got)
	}
}

func TestAssistantCompileKnowledgeRuntime_SnapshotDigestStable(t *testing.T) {
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	catalog.Entries[1].ClarificationTemplateID = "qa.zh"

	runtimeA, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err != nil {
		t.Fatalf("compile runtimeA err=%v", err)
	}
	runtimeB, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err != nil {
		t.Fatalf("compile runtimeB err=%v", err)
	}
	if strings.TrimSpace(runtimeA.SnapshotDigest) == "" || strings.TrimSpace(runtimeB.SnapshotDigest) == "" {
		t.Fatalf("snapshot digest should not be empty: a=%q b=%q", runtimeA.SnapshotDigest, runtimeB.SnapshotDigest)
	}
	if runtimeA.SnapshotDigest != runtimeB.SnapshotDigest {
		t.Fatalf("snapshot digest should be stable: a=%q b=%q", runtimeA.SnapshotDigest, runtimeB.SnapshotDigest)
	}
}

func TestAssistantKnowledgeRuntime_FindInterpretationLocaleFallback(t *testing.T) {
	runtime := &assistantKnowledgeRuntime{
		interpretation: map[string]map[string]assistantInterpretationPack{
			assistantInterpretationDefaultPackID: {
				"zh": {PackID: assistantInterpretationDefaultPackID, Locale: "zh"},
				"en": {PackID: assistantInterpretationDefaultPackID, Locale: "en"},
			},
		},
	}
	pack, ok := runtime.findInterpretation(assistantInterpretationDefaultPackID, "fr")
	if !ok {
		t.Fatal("expected locale fallback to zh")
	}
	if strings.TrimSpace(pack.Locale) != "zh" {
		t.Fatalf("expected zh fallback, got=%q", pack.Locale)
	}
}

func TestAssistantKnowledgeRuntime_RouteCatalogVersionPropagates(t *testing.T) {
	runtime := &assistantKnowledgeRuntime{
		RouteCatalogVersion: "2026-03-11.v1",
		routeCatalog: assistantIntentRouteCatalog{
			Entries: []assistantIntentRouteEntry{
				{IntentID: "knowledge.general_qa", RouteKind: assistantRouteKindKnowledgeQA, Keywords: []string{"功能"}},
			},
		},
	}
	intent := runtime.routeIntent("系统支持哪些功能", assistantIntentSpec{Action: assistantIntentPlanOnly})
	if strings.TrimSpace(intent.RouteCatalogVersion) != "2026-03-11.v1" {
		t.Fatalf("unexpected route_catalog_version=%q", intent.RouteCatalogVersion)
	}
}

func TestAssistantCompileKnowledgeRuntime_RejectsDuplicateInterpretationTemplateID(t *testing.T) {
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	interpretation[0].ClarificationPrompts = append(interpretation[0].ClarificationPrompts, assistantKnowledgePrompt{TemplateID: "qa.zh", Text: "duplicate"})

	_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err == nil || !strings.Contains(err.Error(), "duplicated interpretation template_id") {
		t.Fatalf("expected duplicate interpretation template_id error, got=%v", err)
	}
}

func TestAssistantCompileKnowledgeRuntime_RejectsInvalidIntentClass(t *testing.T) {
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	interpretation[0].IntentClasses = []string{"bad_class"}

	_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err == nil || !strings.Contains(err.Error(), "invalid intent_class") {
		t.Fatalf("expected invalid intent_class error, got=%v", err)
	}
}

func TestAssistantCompileKnowledgeRuntime_RejectsUnknownClarificationTemplateID(t *testing.T) {
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	catalog.Entries[1].ClarificationTemplateID = "clarify.unknown.v1"

	_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err == nil || !strings.Contains(err.Error(), "unknown clarification_template_id") {
		t.Fatalf("expected unknown clarification_template_id error, got=%v", err)
	}
}

func TestAssistantCompileKnowledgeRuntime_RejectsInvalidRequiredSlot(t *testing.T) {
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	catalog.Entries[0].RequiredSlots = []string{"unknown_slot"}

	_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err == nil || !strings.Contains(err.Error(), "invalid required_slot") {
		t.Fatalf("expected invalid required_slot error, got=%v", err)
	}
}

func TestAssistantCompileKnowledgeRuntime_RejectsMinConfidenceOutOfRange(t *testing.T) {
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	catalog.Entries[0].MinConfidence = 1.1

	_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err == nil || !strings.Contains(err.Error(), "min_confidence out of range") {
		t.Fatalf("expected min_confidence out of range error, got=%v", err)
	}
}

func TestAssistantCompileKnowledgeRuntime_RejectsMissingInterpretationForNonBusinessIntent(t *testing.T) {
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	catalog.Entries[1].IntentID = "knowledge.other"
	interpretation[0].PackID = "other.pack"

	_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err == nil || !strings.Contains(err.Error(), "missing interpretation pack for non-business intent") {
		t.Fatalf("expected missing interpretation pack for non-business intent error, got=%v", err)
	}
}

func TestAssistantCompileKnowledgeRuntime_RejectsForbiddenExecutionTruthKeys(t *testing.T) {
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	rawByPath["forbidden.json"] = []byte(`{"required_fields":["x"]}`)

	_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err == nil || !strings.Contains(err.Error(), "forbidden key") {
		t.Fatalf("expected forbidden key error, got=%v", err)
	}
}

func TestAssistantCompileKnowledgeRuntime_RejectsInvalidSourceRefs(t *testing.T) {
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	catalog.SourceRefs = []string{"not/exist/path.md"}

	_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err == nil || !strings.Contains(err.Error(), "route catalog source_refs invalid") {
		t.Fatalf("expected route catalog source_refs invalid error, got=%v", err)
	}
}

func TestAssistantKnowledgeRuntime_HelperBranches_244(t *testing.T) {
	t.Run("normalize interpretation intent classes", func(t *testing.T) {
		if _, err := assistantNormalizeInterpretationIntentClasses([]string{"knowledge_qa", "knowledge_qa"}); err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
		if _, err := assistantNormalizeInterpretationIntentClasses([]string{" "}); err == nil {
			t.Fatal("expected empty intent class error")
		}
	})

	t.Run("normalize interpretation prompts", func(t *testing.T) {
		prompts, lookup, err := assistantNormalizeInterpretationPrompts("p", "zh", nil)
		if err != nil || len(prompts) != 0 || len(lookup) != 0 {
			t.Fatalf("unexpected result prompts=%v lookup=%v err=%v", prompts, lookup, err)
		}
		if _, _, err := assistantNormalizeInterpretationPrompts("p", "zh", []assistantKnowledgePrompt{{TemplateID: "t1", Text: " "}}); err == nil {
			t.Fatal("expected empty prompt text error")
		}
	})

	t.Run("normalize negative examples", func(t *testing.T) {
		got, err := assistantNormalizeNegativeExamples([]string{"x", "x"})
		if err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
		if len(got) != 1 || got[0] != "x" {
			t.Fatalf("unexpected negative examples=%v", got)
		}
	})

	t.Run("normalize required slots", func(t *testing.T) {
		got, err := assistantNormalizeRequiredSlots([]string{"parent_ref_text", "parent_ref_text"})
		if err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
		if len(got) != 1 || got[0] != "parent_ref_text" {
			t.Fatalf("unexpected required slots=%v", got)
		}
		if _, err := assistantNormalizeRequiredSlots([]string{" "}); err == nil {
			t.Fatal("expected required slot empty error")
		}
	})

	t.Run("allowed required slots and support checks", func(t *testing.T) {
		if got := assistantAllowedRequiredSlotsByAction("unknown.action"); len(got) != 0 {
			t.Fatalf("expected empty allowed slots, got=%v", got)
		}
		if assistantInterpretationSupportsRouteKind(nil, assistantRouteKindKnowledgeQA) {
			t.Fatal("nil locales should not support route kind")
		}
		if assistantInterpretationSupportsRouteKind(map[string]assistantInterpretationPack{
			"zh": {IntentClasses: []string{assistantRouteKindBusinessAction}},
		}, assistantRouteKindKnowledgeQA) {
			t.Fatal("unexpected route kind support")
		}
	})

	t.Run("template exists and sort helpers", func(t *testing.T) {
		if assistantInterpretationTemplateExists(nil, "x", "t") {
			t.Fatal("nil template map should not find template")
		}
		if got := assistantSortedInterpretationPacks(nil); got != nil {
			t.Fatalf("expected nil sorted packs, got=%v", got)
		}
	})

	t.Run("resolve pack id and uncertain fallback", func(t *testing.T) {
		if got := (*assistantKnowledgeRuntime)(nil).resolveInterpretationPackID("knowledge.general_qa", assistantRouteKindKnowledgeQA); got != "" {
			t.Fatalf("expected empty pack id for nil runtime, got=%q", got)
		}
		runtime := &assistantKnowledgeRuntime{
			routePackID: map[string]string{
				"knowledge.general_qa": "knowledge.general_qa",
			},
		}
		if got := runtime.resolveInterpretationPackID("knowledge.general_qa", assistantRouteKindKnowledgeQA); got != "knowledge.general_qa" {
			t.Fatalf("unexpected pack id=%q", got)
		}
		nilFallback := (*assistantKnowledgeRuntime)(nil).fallbackUncertainRoute()
		if nilFallback.IntentID != assistantRouteFallbackUncertainID || nilFallback.RouteKind != assistantRouteKindUncertain {
			t.Fatalf("unexpected nil fallback=%+v", nilFallback)
		}
		runtime = &assistantKnowledgeRuntime{
			routeCatalog: assistantIntentRouteCatalog{
				Entries: []assistantIntentRouteEntry{
					{IntentID: "route.pending", RouteKind: assistantRouteKindUncertain},
				},
			},
		}
		if got := runtime.fallbackUncertainRoute(); got.IntentID != "route.pending" {
			t.Fatalf("unexpected catalog fallback=%+v", got)
		}
	})

	t.Run("route intent fallback empty fields", func(t *testing.T) {
		runtime := &assistantKnowledgeRuntime{
			RouteCatalogVersion: "v1",
			routeByIntent: map[string]assistantIntentRouteEntry{
				assistantRouteFallbackUncertainID: {},
			},
		}
		intent := runtime.routeIntent("unmatched", assistantIntentSpec{Action: assistantIntentPlanOnly})
		if intent.IntentID != assistantRouteFallbackUncertainID || intent.RouteKind != assistantRouteKindUncertain {
			t.Fatalf("unexpected intent fallback=%+v", intent)
		}
	})
}
