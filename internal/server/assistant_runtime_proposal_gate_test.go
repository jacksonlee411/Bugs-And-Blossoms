package server

import (
	"context"
	"errors"
	"testing"
)

func TestAssistantRuntimeProposalValidationAndPayloadBridge(t *testing.T) {
	payload := assistantSemanticIntentPayload{
		Action:           assistantIntentCreateOrgUnit,
		IntentID:         "org.orgunit_create",
		RouteKind:        assistantRouteKindBusinessAction,
		ParentRefText:    "鲜花组织",
		EntityName:       "运营部",
		EffectiveDate:    "2026-01-01",
		GoalSummary:      "创建运营部",
		UserVisibleReply: "已生成草案",
	}
	if state := payload.semanticState(); state.Action != assistantIntentCreateOrgUnit || state.Slots.EntityName != "运营部" {
		t.Fatalf("unexpected semantic state=%+v", state)
	}
	if err := assistantValidateProposalWriteDates(assistantRuntimeProposal{EffectiveDate: "2026/01/01"}, assistantSemanticRetrievalResult{}); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected invalid effective date err, got=%v", err)
	}
	if err := assistantValidateProposalWriteDates(assistantRuntimeProposal{TargetEffectiveDate: "2026/01/01"}, assistantSemanticRetrievalResult{}); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected invalid target date err, got=%v", err)
	}
	if err := assistantValidateProposalWriteDates(assistantRuntimeProposal{}, assistantSemanticRetrievalResult{AsOf: "2026-01-01"}); err != nil {
		t.Fatalf("expected retrieval as_of without write date to pass, got=%v", err)
	}
}

func TestAssistantAcceptProposalCandidateFailClosedCoverage(t *testing.T) {
	var svc *assistantConversationService
	proposal := assistantRuntimeProposal{
		ActionHint:    assistantIntentCreateOrgUnit,
		IntentIDHint:  "org.orgunit_create",
		RouteKindHint: assistantRouteKindBusinessAction,
		ParentRefText: "鲜花组织",
		EntityName:    "运营部",
		EffectiveDate: "2026-01-01",
	}
	routeDecision := assistantTestBusinessRouteDecision(assistantIntentCreateOrgUnit)
	candidates := []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}}

	if _, err := svc.assistantAcceptProposal(context.Background(), "tenant-1", Principal{}, "创建运营部", proposal, routeDecision, candidates, "", "missing", nil, assistantSemanticRetrievalResult{}, nil); !errors.Is(err, errAssistantCandidateNotFound) {
		t.Fatalf("expected selected candidate not found, got=%v", err)
	}
	if _, err := svc.assistantAcceptProposal(context.Background(), "tenant-1", Principal{}, "创建运营部", proposal, routeDecision, candidates, "missing", "", nil, assistantSemanticRetrievalResult{}, nil); !errors.Is(err, errAssistantCandidateNotFound) {
		t.Fatalf("expected resolved candidate not found, got=%v", err)
	}
}

func TestAssistantTurnAuthoritativeStateReadyForCommitCoverage(t *testing.T) {
	if err := assistantTurnAuthoritativeStateReadyForCommit(nil); !errors.Is(err, errAssistantConversationStateInvalid) {
		t.Fatalf("expected state invalid for nil turn, got=%v", err)
	}
	if err := assistantTurnAuthoritativeStateReadyForCommit(&assistantTurn{State: assistantStateValidated}); !errors.Is(err, errAssistantConfirmationRequired) {
		t.Fatalf("expected confirmation required for non-confirmed turn, got=%v", err)
	}

	missingFieldsTurn := assistantTestAttachBusinessRoute(&assistantTurn{
		State: assistantStateConfirmed,
		Intent: assistantIntentSpec{
			Action:        assistantIntentCreateOrgUnit,
			EffectiveDate: "2026-01-01",
		},
		DryRun: assistantDryRunResult{ValidationErrors: []string{"missing_entity_name"}},
	})
	if err := assistantTurnAuthoritativeStateReadyForCommit(missingFieldsTurn); !errors.Is(err, errAssistantConfirmationRequired) {
		t.Fatalf("expected confirmation required for missing fields, got=%v", err)
	}

	nonBusinessTurn := &assistantTurn{
		State: assistantStateConfirmed,
		Intent: assistantIntentSpec{
			RouteKind: assistantRouteKindKnowledgeQA,
		},
		RouteDecision: assistantIntentRouteDecision{
			RouteKind:               assistantRouteKindKnowledgeQA,
			IntentID:                "knowledge.general_qa",
			ConfidenceBand:          assistantRouteConfidenceLow,
			RouteCatalogVersion:     "v1",
			KnowledgeSnapshotDigest: "digest",
			ResolverContractVersion: "resolver-v1",
			DecisionSource:          assistantRouteDecisionSourceSemanticModelV1,
		},
	}
	if err := assistantTurnAuthoritativeStateReadyForCommit(nonBusinessTurn); !errors.Is(err, errAssistantRouteNonBusinessBlocked) {
		t.Fatalf("expected non-business blocked, got=%v", err)
	}

	successTurn := assistantTestAttachBusinessRoute(&assistantTurn{
		State: assistantStateConfirmed,
		Intent: assistantIntentSpec{
			Action:        assistantIntentCreateOrgUnit,
			EffectiveDate: "2026-01-01",
		},
	})
	if err := assistantTurnAuthoritativeStateReadyForCommit(successTurn); err != nil {
		t.Fatalf("expected successful authoritative commit readiness, got=%v", err)
	}
}
