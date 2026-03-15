package server

import (
	"errors"
	"testing"
)

func TestAssistantIntentRouter_DecisionCoverage(t *testing.T) {
	t.Run("normalize and confidence helpers", func(t *testing.T) {
		if !assistantValidRouteConfidenceBand(assistantRouteConfidenceHigh) || assistantValidRouteConfidenceBand("bad") {
			t.Fatal("confidence band helper mismatch")
		}
		normalized := assistantNormalizeRouteStringSlice([]string{" b ", "a", "", "a"})
		if len(normalized) != 2 || normalized[0] != "a" || normalized[1] != "b" {
			t.Fatalf("normalized=%v", normalized)
		}
		if assistantNormalizeRouteStringSlice(nil) != nil {
			t.Fatal("expected nil normalize result")
		}
		if assistantNormalizeRouteStringSlice([]string{"", " "}) != nil {
			t.Fatal("expected empty normalize result")
		}
	})

	t.Run("validate decision branches", func(t *testing.T) {
		valid := assistantIntentRouteDecision{
			RouteKind:               assistantRouteKindBusinessAction,
			IntentID:                "org.orgunit_create",
			CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
			ConfidenceBand:          assistantRouteConfidenceHigh,
			RouteCatalogVersion:     "2026-03-11.v1",
			KnowledgeSnapshotDigest: "sha256:test",
			ResolverContractVersion: "resolver_contract_v1",
			DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
		}
		cases := []struct {
			name     string
			decision assistantIntentRouteDecision
			wantErr  error
		}{
			{name: "missing", decision: assistantIntentRouteDecision{}, wantErr: errAssistantRouteDecisionMissing},
			{name: "bad kind", decision: assistantIntentRouteDecision{RouteKind: "bad"}, wantErr: errAssistantRouteRuntimeInvalid},
			{name: "missing intent", decision: assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, ConfidenceBand: assistantRouteConfidenceHigh, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s", CandidateActionIDs: []string{assistantIntentCreateOrgUnit}}, wantErr: errAssistantRouteRuntimeInvalid},
			{name: "bad band", decision: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa", ConfidenceBand: "bad", RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}, wantErr: errAssistantRouteRuntimeInvalid},
			{name: "missing catalog", decision: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa", ConfidenceBand: assistantRouteConfidenceLow, KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}, wantErr: errAssistantRouteRuntimeInvalid},
			{name: "missing snapshot", decision: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa", ConfidenceBand: assistantRouteConfidenceLow, RouteCatalogVersion: "v1", ResolverContractVersion: "r", DecisionSource: "s"}, wantErr: errAssistantRouteRuntimeInvalid},
			{name: "missing resolver", decision: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa", ConfidenceBand: assistantRouteConfidenceLow, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", DecisionSource: "s"}, wantErr: errAssistantRouteRuntimeInvalid},
			{name: "missing source", decision: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa", ConfidenceBand: assistantRouteConfidenceLow, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r"}, wantErr: errAssistantRouteRuntimeInvalid},
			{name: "business missing candidate", decision: assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create", ConfidenceBand: assistantRouteConfidenceHigh, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}, wantErr: errAssistantRouteRuntimeInvalid},
			{name: "non-business conflict", decision: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa", CandidateActionIDs: []string{"x"}, ConfidenceBand: assistantRouteConfidenceLow, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}, wantErr: errAssistantRouteActionConflict},
			{name: "valid", decision: valid},
		}
		for _, tc := range cases {
			err := assistantValidateIntentRouteDecision(tc.decision)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("%s: unexpected err=%v", tc.name, err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("%s: want %v got %v", tc.name, tc.wantErr, err)
			}
		}
	})

	t.Run("build decision branches", func(t *testing.T) {
		if _, err := assistantBuildIntentRouteDecision("x", assistantResolveIntentResult{}, assistantIntentSpec{}, nil); !errors.Is(err, errAssistantRouteCatalogMissing) {
			t.Fatalf("expected catalog missing, got=%v", err)
		}
		runtime := &assistantKnowledgeRuntime{
			RouteCatalogVersion:     "",
			SnapshotDigest:          "",
			ResolverContractVersion: "",
			routeCatalog: assistantIntentRouteCatalog{Entries: []assistantIntentRouteEntry{
				{IntentID: "knowledge.general_qa", RouteKind: assistantRouteKindKnowledgeQA, Keywords: []string{"功能"}},
				{IntentID: "route.chitchat", RouteKind: assistantRouteKindChitchat, Keywords: []string{"你好"}},
				{IntentID: "route.bad", RouteKind: "bad_kind", Keywords: []string{"坏"}},
			}},
			routeByAction: map[string]assistantIntentRouteEntry{
				assistantIntentCreateOrgUnit: {IntentID: "org.orgunit_create", RouteKind: assistantRouteKindBusinessAction, ActionID: assistantIntentCreateOrgUnit},
			},
		}
		if _, err := assistantBuildIntentRouteDecision(
			"在鲜花组织之下新建部门",
			assistantResolveIntentResult{Intent: assistantIntentSpec{Action: assistantIntentPlanOnly}},
			assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部"},
			runtime,
		); !errors.Is(err, errAssistantRouteDecisionMissing) {
			t.Fatalf("expected missing semantic route err, got=%v", err)
		}

		semanticBusiness, err := assistantBuildIntentRouteDecision(
			"随便写什么都不重要",
			assistantResolveIntentResult{
				Intent: assistantIntentSpec{
					Action:              assistantIntentCreateOrgUnit,
					IntentID:            "org.orgunit_create",
					RouteKind:           assistantRouteKindBusinessAction,
					RouteCatalogVersion: "semantic.v1",
				},
				Readiness: assistantSemanticReadinessNeedMoreInfo,
			},
			assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			runtime,
		)
		if err != nil {
			t.Fatalf("semantic business err=%v", err)
		}
		if semanticBusiness.DecisionSource != assistantRouteDecisionSourceSemanticModelV1 || semanticBusiness.RouteCatalogVersion != "semantic.v1" || semanticBusiness.ConfidenceBand != assistantRouteConfidenceMedium {
			t.Fatalf("unexpected semantic business decision=%+v", semanticBusiness)
		}

		semanticQA, err := assistantBuildIntentRouteDecision(
			"系统有哪些功能",
			assistantResolveIntentResult{
				Intent: assistantIntentSpec{
					Action:              assistantIntentPlanOnly,
					IntentID:            "knowledge.general_qa",
					RouteKind:           assistantRouteKindKnowledgeQA,
					RouteCatalogVersion: "semantic.v1",
				},
			},
			assistantIntentSpec{Action: assistantIntentPlanOnly},
			runtime,
		)
		if err != nil || semanticQA.DecisionSource != assistantRouteDecisionSourceSemanticModelV1 || semanticQA.RouteKind != assistantRouteKindKnowledgeQA {
			t.Fatalf("unexpected semantic qa=%+v err=%v", semanticQA, err)
		}

		if _, err := assistantBuildIntentRouteDecision(
			"非法语义路由",
			assistantResolveIntentResult{Intent: assistantIntentSpec{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create"}},
			assistantIntentSpec{Action: assistantIntentPlanOnly},
			runtime,
		); !errors.Is(err, errAssistantRouteRuntimeInvalid) {
			t.Fatalf("expected invalid semantic business route err, got=%v", err)
		}

		if decision, ok, err := assistantBuildSemanticIntentRouteDecision(assistantResolveIntentResult{}, assistantIntentSpec{}, runtime); err != nil || ok || assistantIntentRouteDecisionPresent(decision) {
			t.Fatalf("expected semantic helper to skip empty metadata, decision=%+v ok=%v err=%v", decision, ok, err)
		}

		if _, ok, err := assistantBuildSemanticIntentRouteDecision(
			assistantResolveIntentResult{Intent: assistantIntentSpec{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa"}},
			assistantIntentSpec{Action: assistantIntentPlanOnly},
			nil,
		); !ok || !errors.Is(err, errAssistantRouteCatalogMissing) {
			t.Fatalf("expected semantic helper catalog missing, ok=%v err=%v", ok, err)
		}

		if _, ok, err := assistantBuildSemanticIntentRouteDecision(
			assistantResolveIntentResult{Intent: assistantIntentSpec{RouteKind: assistantRouteKindKnowledgeQA}},
			assistantIntentSpec{Action: assistantIntentPlanOnly},
			runtime,
		); !ok || !errors.Is(err, errAssistantRouteRuntimeInvalid) {
			t.Fatalf("expected semantic helper missing intent id err, ok=%v err=%v", ok, err)
		}

		if _, ok, err := assistantBuildSemanticIntentRouteDecision(
			assistantResolveIntentResult{Intent: assistantIntentSpec{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create"}},
			assistantIntentSpec{Action: "unsupported"},
			runtime,
		); !ok || !errors.Is(err, errAssistantRouteRuntimeInvalid) {
			t.Fatalf("expected semantic helper unsupported action err, ok=%v err=%v", ok, err)
		}

		semanticUncertain, ok, err := assistantBuildSemanticIntentRouteDecision(
			assistantResolveIntentResult{Intent: assistantIntentSpec{RouteKind: assistantRouteKindUncertain, IntentID: "route.uncertain"}},
			assistantIntentSpec{Action: assistantIntentPlanOnly},
			runtime,
		)
		if err != nil || !ok || semanticUncertain.RouteKind != assistantRouteKindUncertain || !semanticUncertain.ClarificationRequired {
			t.Fatalf("unexpected semantic uncertain decision=%+v ok=%v err=%v", semanticUncertain, ok, err)
		}
	})

	t.Run("projection and turn helpers", func(t *testing.T) {
		decision := assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create", CandidateActionIDs: []string{assistantIntentCreateOrgUnit}, RouteCatalogVersion: "v1"}
		projected := assistantProjectIntentRouteDecision(assistantIntentSpec{Action: assistantIntentPlanOnly}, decision)
		if projected.Action != assistantIntentCreateOrgUnit || projected.RouteKind != assistantRouteKindBusinessAction {
			t.Fatalf("unexpected projected intent=%+v", projected)
		}
		nonBusiness := assistantProjectIntentRouteDecision(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa", RouteCatalogVersion: "v1"})
		if nonBusiness.Action != assistantIntentPlanOnly {
			t.Fatalf("unexpected non business projection=%+v", nonBusiness)
		}
		if got := assistantTurnRouteKind(nil); got != "" {
			t.Fatalf("nil turn route kind=%q", got)
		}
		turn := &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}}
		if got := assistantTurnRouteKind(turn); got != assistantRouteKindBusinessAction {
			t.Fatalf("route kind=%q", got)
		}
		turn.RouteDecision = assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, ClarificationRequired: true}
		if got := assistantTurnRouteKind(turn); got != assistantRouteKindKnowledgeQA {
			t.Fatalf("route kind=%q", got)
		}
		if !assistantTurnHasRouteClarificationSignal(turn) {
			t.Fatal("expected clarification required")
		}
		if assistantTurnActionChainAllowed(turn) {
			t.Fatal("non-business route should not allow action chain")
		}
		if assistantTurnActionChainAllowed(nil) {
			t.Fatal("nil turn should not allow action chain")
		}
		turn = &assistantTurn{RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, CandidateActionIDs: []string{assistantIntentCreateOrgUnit}, ConfidenceBand: assistantRouteConfidenceHigh, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}}
		if !assistantTurnActionChainAllowed(turn) {
			t.Fatal("business route should allow action chain")
		}
		turn.RouteDecision.ClarificationRequired = true
		if assistantTurnActionChainAllowed(turn) {
			t.Fatal("clarification route should block action chain")
		}
		if assistantTurnHasRouteClarificationSignal(nil) {
			t.Fatal("nil turn should not require clarification")
		}
	})

	t.Run("action gate route helpers", func(t *testing.T) {
		spec, _ := assistantLookupDefaultActionSpec(assistantIntentCreateOrgUnit)
		if _, ok := assistantActionGateRouteDecision(assistantActionGateInput{}); ok {
			t.Fatal("unexpected route decision")
		}
		if explicit, ok := assistantActionGateRouteDecision(assistantActionGateInput{RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA}}); !ok || explicit.RouteKind != assistantRouteKindKnowledgeQA {
			t.Fatalf("unexpected explicit=%+v ok=%v", explicit, ok)
		}
		if fromTurn, ok := assistantActionGateRouteDecision(assistantActionGateInput{Turn: &assistantTurn{RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA}}}); !ok || fromTurn.RouteKind != assistantRouteKindKnowledgeQA {
			t.Fatalf("unexpected turn decision=%+v ok=%v", fromTurn, ok)
		}
		if got := httpStatusForAssistantRouteError(errAssistantRouteRuntimeInvalid); got != 422 {
			t.Fatalf("unexpected runtime status=%d", got)
		}
		if got := httpStatusForAssistantRouteError(errAssistantRouteCatalogMissing); got != 503 {
			t.Fatalf("unexpected catalog status=%d", got)
		}
		if got := httpStatusForAssistantRouteError(errAssistantRouteActionConflict); got != 422 {
			t.Fatalf("unexpected conflict status=%d", got)
		}
		if got := httpStatusForAssistantRouteError(errAssistantRouteDecisionMissing); got != 409 {
			t.Fatalf("unexpected decision status=%d", got)
		}
		if got := httpStatusForAssistantRouteError(errAssistantRouteNonBusinessBlocked); got != 409 {
			t.Fatalf("unexpected blocked status=%d", got)
		}
		if got := httpStatusForAssistantRouteError(errAssistantRouteClarificationRequired); got != 409 {
			t.Fatalf("unexpected clarification status=%d", got)
		}
		if got := httpStatusForAssistantRouteError(errors.New("x")); got != 0 {
			t.Fatalf("unexpected unknown status=%d", got)
		}
		denied := assistantRouteGateDenied(errAssistantRouteNonBusinessBlocked, assistantRouteReasonNonBusinessBlocked)
		if denied.HTTPStatus != 409 || denied.ReasonCode != assistantRouteReasonNonBusinessBlocked {
			t.Fatalf("unexpected denied=%+v", denied)
		}
		if denied := assistantRouteGateDenied(errors.New("unknown"), "unknown_reason"); denied.HTTPStatus != 0 || denied.ReasonCode != "unknown_reason" {
			t.Fatalf("unexpected unknown denied=%+v", denied)
		}

		valid := assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create", CandidateActionIDs: []string{assistantIntentCreateOrgUnit}, ConfidenceBand: assistantRouteConfidenceHigh, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}
		cases := []struct {
			name  string
			input assistantActionGateInput
			want  error
		}{
			{name: "plan missing allowed", input: assistantActionGateInput{Stage: assistantActionStagePlan}},
			{name: "plan valid allowed", input: assistantActionGateInput{Stage: assistantActionStagePlan, Action: spec, RouteDecision: valid}},
			{name: "confirm missing", input: assistantActionGateInput{Stage: assistantActionStageConfirm}, want: errAssistantRouteDecisionMissing},
			{name: "invalid", input: assistantActionGateInput{Stage: assistantActionStageConfirm, RouteDecision: assistantIntentRouteDecision{RouteKind: "bad"}}, want: errAssistantRouteRuntimeInvalid},
			{name: "validate conflict", input: assistantActionGateInput{Stage: assistantActionStageConfirm, RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa", CandidateActionIDs: []string{"x"}, ConfidenceBand: assistantRouteConfidenceLow, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}}, want: errAssistantRouteActionConflict},
			{name: "plan conflict", input: assistantActionGateInput{Stage: assistantActionStagePlan, Action: assistantActionSpec{ID: assistantIntentRenameOrgUnit}, RouteDecision: valid}, want: errAssistantRouteActionConflict},
			{name: "commit empty action conflict", input: assistantActionGateInput{Stage: assistantActionStageCommit, RouteDecision: valid}, want: errAssistantRouteActionConflict},
			{name: "commit candidate missing conflict", input: assistantActionGateInput{Stage: assistantActionStageCommit, RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create", ConfidenceBand: assistantRouteConfidenceHigh, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}}, want: errAssistantRouteRuntimeInvalid},
			{name: "non-business blocked", input: assistantActionGateInput{Stage: assistantActionStageConfirm, Action: spec, RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa", ConfidenceBand: assistantRouteConfidenceLow, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}}, want: errAssistantRouteNonBusinessBlocked},
			{name: "clarification blocked", input: assistantActionGateInput{Stage: assistantActionStageCommit, Action: spec, RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create", CandidateActionIDs: []string{assistantIntentCreateOrgUnit}, ConfidenceBand: assistantRouteConfidenceMedium, ClarificationRequired: true, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}}, want: errAssistantRouteClarificationRequired},
			{name: "allowed", input: assistantActionGateInput{Stage: assistantActionStageCommit, Action: spec, RouteDecision: valid}},
		}
		for _, tc := range cases {
			decision := assistantCheckRouteDecision(tc.input)
			if tc.want == nil {
				if !decision.Allowed {
					t.Fatalf("%s: unexpected decision=%+v", tc.name, decision)
				}
				continue
			}
			if decision.Allowed || !errors.Is(decision.Error, tc.want) {
				t.Fatalf("%s: want %v got %+v", tc.name, tc.want, decision)
			}
		}
	})
}

func TestAssistantIntentRouter_RouteAuditVersionConsistency(t *testing.T) {
	turn := &assistantTurn{
		RouteDecision: assistantIntentRouteDecision{
			RouteKind:               assistantRouteKindBusinessAction,
			IntentID:                "org.orgunit_create",
			CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
			ConfidenceBand:          assistantRouteConfidenceHigh,
			KnowledgeSnapshotDigest: "sha256:route",
			RouteCatalogVersion:     "2026-03-11.v1",
			ResolverContractVersion: "resolver_contract_v1",
			DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
		},
		Plan: assistantPlanSummary{
			KnowledgeSnapshotDigest: "sha256:route",
			RouteCatalogVersion:     "2026-03-11.v1",
			ResolverContractVersion: "resolver_contract_v1",
			ContextTemplateVersion:  assistantContextTemplateVersionV1,
			ReplyGuidanceVersion:    "2026-03-11.v1",
		},
	}
	if !assistantTurnRouteAuditVersionsConsistent(turn) {
		t.Fatal("expected route audit versions consistent")
	}
	turn.Plan.RouteCatalogVersion = "2026-03-11.v2"
	if assistantTurnRouteAuditVersionsConsistent(turn) {
		t.Fatal("route catalog drift should fail consistency check")
	}
	turn.Plan.RouteCatalogVersion = "2026-03-11.v1"
	turn.Plan.ContextTemplateVersion = ""
	if assistantTurnRouteAuditVersionsConsistent(turn) {
		t.Fatal("missing context template version should fail consistency check")
	}
}

func TestAssistantIntentRouter_MissingGapBranches(t *testing.T) {
	t.Run("validate decision candidate blank action id", func(t *testing.T) {
		err := assistantValidateIntentRouteDecision(assistantIntentRouteDecision{
			RouteKind:               assistantRouteKindBusinessAction,
			IntentID:                "org.orgunit_create",
			CandidateActionIDs:      []string{""},
			ConfidenceBand:          assistantRouteConfidenceHigh,
			RouteCatalogVersion:     "v1",
			KnowledgeSnapshotDigest: "d",
			ResolverContractVersion: "r",
			DecisionSource:          "s",
		})
		if !errors.Is(err, errAssistantRouteRuntimeInvalid) {
			t.Fatalf("blank candidate action should be invalid err=%v", err)
		}
	})

	t.Run("build decision runtime invalid and validate error path", func(t *testing.T) {
		runtime := &assistantKnowledgeRuntime{
			RouteCatalogVersion:     "v1",
			SnapshotDigest:          "d",
			ResolverContractVersion: "r",
			routeByAction: map[string]assistantIntentRouteEntry{
				assistantIntentCreateOrgUnit: {IntentID: "org.orgunit_create", RouteKind: "bad_kind"},
			},
		}
		if _, err := assistantBuildIntentRouteDecision(
			"创建组织",
			assistantResolveIntentResult{Intent: assistantIntentSpec{Action: assistantIntentPlanOnly}},
			assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			runtime,
		); !errors.Is(err, errAssistantRouteDecisionMissing) {
			t.Fatalf("expected missing semantic route err, got=%v", err)
		}

		if _, err := assistantBuildIntentRouteDecision(
			"创建",
			assistantResolveIntentResult{Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create"}},
			assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "bad-date", ParentRefText: "p", EntityName: "n"},
			runtime,
		); err != nil {
			t.Fatalf("validation-warning route should still build, err=%v", err)
		}
	})

	t.Run("route audit and action chain status branches", func(t *testing.T) {
		if !assistantTurnRouteAuditVersionsConsistent(nil) {
			t.Fatal("nil turn should be consistent")
		}
		if !assistantTurnRouteAuditVersionsConsistent(&assistantTurn{}) {
			t.Fatal("missing route decision should be consistent")
		}
		turn := &assistantTurn{
			RouteDecision: assistantIntentRouteDecision{
				RouteKind:               assistantRouteKindBusinessAction,
				IntentID:                "org.orgunit_create",
				CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
				ConfidenceBand:          assistantRouteConfidenceHigh,
				RouteCatalogVersion:     "v1",
				KnowledgeSnapshotDigest: "d",
				ResolverContractVersion: "r",
				DecisionSource:          "s",
			},
			Plan: assistantPlanSummary{
				KnowledgeSnapshotDigest: "d",
				RouteCatalogVersion:     "v1",
				ResolverContractVersion: "r",
				ContextTemplateVersion:  assistantContextTemplateVersionV1,
				ReplyGuidanceVersion:    "reply.v1",
			},
		}
		turn.Plan.KnowledgeSnapshotDigest = ""
		if assistantTurnRouteAuditVersionsConsistent(turn) {
			t.Fatal("missing plan audit fields should fail")
		}
		turn.Plan.KnowledgeSnapshotDigest = "d"
		turn.Plan.ResolverContractVersion = "r2"
		if assistantTurnRouteAuditVersionsConsistent(turn) {
			t.Fatal("resolver drift should fail")
		}

		exhausted := &assistantTurn{
			Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			RouteDecision: assistantIntentRouteDecision{
				RouteKind:               assistantRouteKindBusinessAction,
				IntentID:                "org.orgunit_create",
				CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
				ConfidenceBand:          assistantRouteConfidenceHigh,
				RouteCatalogVersion:     "v1",
				KnowledgeSnapshotDigest: "d",
				ResolverContractVersion: "r",
				DecisionSource:          "s",
			},
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusExhausted},
		}
		if assistantTurnActionChainAllowed(exhausted) {
			t.Fatal("exhausted clarification should block action chain")
		}
		aborted := *exhausted
		aborted.Clarification = &assistantClarificationDecision{Status: assistantClarificationStatusAborted}
		if assistantTurnActionChainAllowed(&aborted) {
			t.Fatal("aborted clarification should block action chain")
		}
	})

	t.Run("check route decision remaining branches", func(t *testing.T) {
		spec, ok := assistantLookupDefaultActionSpec(assistantIntentCreateOrgUnit)
		if !ok {
			t.Fatal("missing create spec")
		}
		planAllow := assistantCheckRouteDecision(assistantActionGateInput{
			Stage:  assistantActionStagePlan,
			Action: assistantActionSpec{},
			RouteDecision: assistantIntentRouteDecision{
				RouteKind:               assistantRouteKindBusinessAction,
				IntentID:                "org.orgunit_create",
				CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
				ConfidenceBand:          assistantRouteConfidenceHigh,
				RouteCatalogVersion:     "v1",
				KnowledgeSnapshotDigest: "d",
				ResolverContractVersion: "r",
				DecisionSource:          "s",
			},
		})
		if !planAllow.Allowed {
			t.Fatalf("plan stage with missing action id should allow, got=%+v", planAllow)
		}

		openClarificationTurn := &assistantTurn{
			RouteDecision: assistantIntentRouteDecision{
				RouteKind:               assistantRouteKindBusinessAction,
				IntentID:                "org.orgunit_create",
				CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
				ConfidenceBand:          assistantRouteConfidenceMedium,
				RouteCatalogVersion:     "v1",
				KnowledgeSnapshotDigest: "d",
				ResolverContractVersion: "r",
				DecisionSource:          "s",
			},
			Clarification: &assistantClarificationDecision{
				Status:                  assistantClarificationStatusOpen,
				ClarificationKind:       assistantClarificationKindMissingSlots,
				AwaitPhase:              assistantPhaseAwaitMissingFields,
				MaxRounds:               2,
				CurrentRound:            1,
				ExitTo:                  assistantClarificationExitBusinessResume,
				KnowledgeSnapshotDigest: "d",
				RouteCatalogVersion:     "v1",
			},
		}
		denied := assistantCheckRouteDecision(assistantActionGateInput{
			Stage:  assistantActionStageConfirm,
			Action: spec,
			Turn:   openClarificationTurn,
			RouteDecision: assistantIntentRouteDecision{
				RouteKind:               assistantRouteKindBusinessAction,
				IntentID:                "org.orgunit_create",
				CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
				ConfidenceBand:          assistantRouteConfidenceMedium,
				ClarificationRequired:   true,
				RouteCatalogVersion:     "v1",
				KnowledgeSnapshotDigest: "d",
				ResolverContractVersion: "r",
				DecisionSource:          "s",
			},
		})
		if denied.Allowed || !errors.Is(denied.Error, errAssistantClarificationRequired) {
			t.Fatalf("open clarification route gate should return clarification required, got=%+v", denied)
		}
	})

	t.Run("http status mapping clarification errors", func(t *testing.T) {
		if got := httpStatusForAssistantRouteError(errAssistantClarificationRequired); got != 409 {
			t.Fatalf("clarification required status=%d", got)
		}
		if got := httpStatusForAssistantRouteError(errAssistantClarificationRuntimeInvalid); got != 409 {
			t.Fatalf("clarification runtime invalid status=%d", got)
		}
	})
}
