package server

import (
	"errors"
	"strings"
	"testing"
)

type assistantGateAuthorizerStub struct {
	allowed  bool
	enforced bool
	err      error
}

func (s assistantGateAuthorizerStub) Authorize(string, string, string, string) (bool, bool, error) {
	return s.allowed, s.enforced, s.err
}

func TestAssistantActionInterceptor_Gates(t *testing.T) {
	spec, ok := assistantLookupDefaultActionSpec(assistantIntentCreateOrgUnit)
	if !ok {
		t.Fatal("expected default create_orgunit spec")
	}

	t.Run("plan authz denied", func(t *testing.T) {
		original := assistantLoadAuthorizerFn
		assistantLoadAuthorizerFn = func() (authorizer, error) {
			return assistantGateAuthorizerStub{allowed: false, enforced: true}, nil
		}
		defer func() { assistantLoadAuthorizerFn = original }()

		decision := assistantEvaluateActionGate(assistantActionGateInput{
			Stage:     assistantActionStagePlan,
			TenantID:  "tenant_1",
			Principal: Principal{ID: "actor_1", RoleSlug: "tenant-admin"},
			Action:    spec,
			Intent: assistantIntentSpec{
				Action:        assistantIntentCreateOrgUnit,
				ParentRefText: "鲜花组织",
				EntityName:    "运营部",
				EffectiveDate: "2026-01-01",
			},
			UserInput: "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01",
		})
		if decision.Allowed || decision.ErrorCode != errAssistantActionAuthzDenied.Error() {
			t.Fatalf("unexpected decision=%+v", decision)
		}
	})

	t.Run("commit ignores resolved candidate confirmation residue", func(t *testing.T) {
		original := assistantLoadAuthorizerFn
		assistantLoadAuthorizerFn = func() (authorizer, error) {
			return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
		}
		defer func() { assistantLoadAuthorizerFn = original }()

		decision := assistantEvaluateActionGate(assistantActionGateInput{
			Stage:         assistantActionStageCommit,
			TenantID:      "tenant_1",
			Principal:     Principal{ID: "actor_1", RoleSlug: "tenant-admin"},
			Action:        spec,
			Intent:        assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"},
			RouteDecision: assistantTestBusinessRouteDecision(assistantIntentCreateOrgUnit),
			ResolvedID:    "c1",
			DryRun:        &assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required"}},
			UserInput:     "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01",
		})
		if !decision.Allowed {
			t.Fatalf("unexpected decision=%+v", decision)
		}
	})

	t.Run("commit missing candidate returns candidate not found", func(t *testing.T) {
		original := assistantLoadAuthorizerFn
		assistantLoadAuthorizerFn = func() (authorizer, error) {
			return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
		}
		defer func() { assistantLoadAuthorizerFn = original }()

		decision := assistantEvaluateActionGate(assistantActionGateInput{
			Stage:         assistantActionStageCommit,
			TenantID:      "tenant_1",
			Principal:     Principal{ID: "actor_1", RoleSlug: "tenant-admin"},
			Action:        spec,
			Intent:        assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"},
			RouteDecision: assistantTestBusinessRouteDecision(assistantIntentCreateOrgUnit),
			DryRun:        &assistantDryRunResult{},
			UserInput:     "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01",
		})
		if decision.Allowed || decision.ErrorCode != errAssistantCandidateNotFound.Error() {
			t.Fatalf("unexpected decision=%+v", decision)
		}
	})

	t.Run("commit with empty dry run and resolved candidate passes", func(t *testing.T) {
		original := assistantLoadAuthorizerFn
		assistantLoadAuthorizerFn = func() (authorizer, error) {
			return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
		}
		defer func() { assistantLoadAuthorizerFn = original }()

		decision := assistantEvaluateActionGate(assistantActionGateInput{
			Stage:         assistantActionStageCommit,
			TenantID:      "tenant_1",
			Principal:     Principal{ID: "actor_1", RoleSlug: "tenant-admin"},
			Action:        spec,
			Intent:        assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
			RouteDecision: assistantTestBusinessRouteDecision(assistantIntentCreateOrgUnit),
			Turn:          &assistantTurn{State: assistantStateConfirmed},
			ResolvedID:    "c1",
			DryRun:        &assistantDryRunResult{},
		})
		if !decision.Allowed {
			t.Fatalf("unexpected decision=%+v", decision)
		}
	})

	t.Run("missing action spec denied", func(t *testing.T) {
		decision := assistantEvaluateActionGate(assistantActionGateInput{})
		if decision.Allowed || !errors.Is(decision.Error, errAssistantActionSpecMissing) {
			t.Fatalf("unexpected decision=%+v", decision)
		}
	})

	t.Run("route gate branches via evaluate", func(t *testing.T) {
		planDecision := assistantEvaluateActionGate(assistantActionGateInput{
			Stage:         assistantActionStagePlan,
			Action:        spec,
			RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create", CandidateActionIDs: []string{assistantIntentRenameOrgUnit}, ConfidenceBand: assistantRouteConfidenceHigh, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"},
		})
		if planDecision.Allowed || !errors.Is(planDecision.Error, errAssistantRouteActionConflict) {
			t.Fatalf("unexpected plan route decision=%+v", planDecision)
		}
		confirmDecision := assistantEvaluateActionGate(assistantActionGateInput{
			Stage:         assistantActionStageConfirm,
			Action:        spec,
			RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa", ConfidenceBand: assistantRouteConfidenceLow, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"},
		})
		if confirmDecision.Allowed || !errors.Is(confirmDecision.Error, errAssistantRouteNonBusinessBlocked) {
			t.Fatalf("unexpected confirm route decision=%+v", confirmDecision)
		}
		commitDecision := assistantEvaluateActionGate(assistantActionGateInput{
			Stage:         assistantActionStageCommit,
			Action:        spec,
			RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create", CandidateActionIDs: []string{assistantIntentCreateOrgUnit}, ConfidenceBand: assistantRouteConfidenceMedium, ClarificationRequired: true, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"},
		})
		if commitDecision.Allowed || !errors.Is(commitDecision.Error, errAssistantRouteClarificationRequired) {
			t.Fatalf("unexpected commit route decision=%+v", commitDecision)
		}
	})

	t.Run("capability registration branches", func(t *testing.T) {
		original := capabilityDefinitionByKey
		defer func() { capabilityDefinitionByKey = original }()
		capabilityDefinitionByKey = map[string]capabilityDefinition{}
		if decision := assistantCheckCapabilityRegistered(spec); decision.Allowed || !errors.Is(decision.Error, errAssistantActionCapabilityUnregistered) {
			t.Fatalf("missing capability decision=%+v", decision)
		}
		capabilityDefinitionByKey = map[string]capabilityDefinition{
			spec.CapabilityKey: {CapabilityKey: spec.CapabilityKey, Status: "inactive", ActivationState: "inactive"},
		}
		if decision := assistantCheckCapabilityRegistered(spec); decision.Allowed || decision.ReasonCode != "capability_inactive" {
			t.Fatalf("inactive capability decision=%+v", decision)
		}
	})

	t.Run("authz error branches", func(t *testing.T) {
		original := assistantLoadAuthorizerFn
		defer func() { assistantLoadAuthorizerFn = original }()
		assistantLoadAuthorizerFn = func() (authorizer, error) { return nil, errors.New("load failed") }
		if decision := assistantCheckActionAuthz(assistantActionGateInput{TenantID: "tenant_1", Principal: Principal{RoleSlug: "tenant-admin"}, Action: spec}); decision.Allowed || decision.ReasonCode != "action_authz_unavailable" {
			t.Fatalf("load error decision=%+v", decision)
		}
		assistantLoadAuthorizerFn = func() (authorizer, error) {
			return assistantGateAuthorizerStub{allowed: true, enforced: true, err: errors.New("authorize failed")}, nil
		}
		if decision := assistantCheckActionAuthz(assistantActionGateInput{TenantID: "tenant_1", Principal: Principal{RoleSlug: "tenant-admin"}, Action: spec}); decision.Allowed || decision.ReasonCode != "action_authz_error" {
			t.Fatalf("authorize error decision=%+v", decision)
		}
		assistantLoadAuthorizerFn = func() (authorizer, error) {
			return assistantGateAuthorizerStub{allowed: false, enforced: false}, nil
		}
		if decision := assistantCheckActionAuthz(assistantActionGateInput{TenantID: "tenant_1", Principal: Principal{RoleSlug: "tenant-admin"}, Action: spec}); !decision.Allowed {
			t.Fatalf("shadow authz should allow, got %+v", decision)
		}
	})

	t.Run("risk gate branches", func(t *testing.T) {
		badSpec := spec
		badSpec.Security.RiskTier = "extreme"
		if decision := assistantCheckRiskGate(assistantActionGateInput{Stage: assistantActionStageCommit, Action: badSpec}); decision.Allowed || decision.ReasonCode != "risk_tier_invalid" {
			t.Fatalf("invalid risk decision=%+v", decision)
		}
		if decision := assistantCheckRiskGate(assistantActionGateInput{Stage: assistantActionStageCommit, Action: spec, Turn: &assistantTurn{State: assistantStateValidated}}); decision.Allowed || decision.ReasonCode != "high_risk_commit_requires_confirmation" {
			t.Fatalf("high risk decision=%+v", decision)
		}
		decision := assistantEvaluateActionGate(assistantActionGateInput{Stage: assistantActionStageCommit, TenantID: "tenant_1", Principal: Principal{RoleSlug: "tenant-admin"}, Action: badSpec, RouteDecision: assistantTestBusinessRouteDecision(badSpec.ID)})
		if decision.Allowed || decision.ReasonCode != "risk_tier_invalid" {
			t.Fatalf("evaluate risk decision=%+v", decision)
		}
	})

	t.Run("required check branches", func(t *testing.T) {
		planInput := assistantActionGateInput{Stage: assistantActionStagePlan, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EntityName: "运营部", EffectiveDate: "2026-01-01"}}
		if decision := assistantCheckRequiredChecks(assistantActionGateInput{Stage: planInput.Stage, Intent: planInput.Intent, Action: assistantActionSpec{Security: assistantActionSecuritySpec{RequiredChecks: []string{"strict_decode"}}}}); decision.Allowed || decision.ReasonCode != "strict_decode_failed" {
			t.Fatalf("strict decode decision=%+v", decision)
		}
		if decision := assistantCheckRequiredChecks(assistantActionGateInput{Stage: assistantActionStagePlan, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, UserInput: "drop table org_unit_nodes", Action: assistantActionSpec{Security: assistantActionSecuritySpec{RequiredChecks: []string{"boundary_lint"}}}}); decision.Allowed || decision.ReasonCode != "boundary_lint_failed" {
			t.Fatalf("boundary lint decision=%+v", decision)
		}
		if decision := assistantCheckRequiredChecks(assistantActionGateInput{Stage: assistantActionStageCommit, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, Action: assistantActionSpec{Security: assistantActionSecuritySpec{RequiredChecks: []string{"candidate_confirmation"}}}}); decision.Allowed || !errors.Is(decision.Error, errAssistantCandidateNotFound) {
			t.Fatalf("candidate missing decision=%+v", decision)
		}
		if decision := assistantCheckRequiredChecks(assistantActionGateInput{Stage: assistantActionStageConfirm, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, Action: assistantActionSpec{Security: assistantActionSecuritySpec{RequiredChecks: []string{"candidate_confirmation"}}}}); decision.Allowed || !errors.Is(decision.Error, errAssistantConfirmationRequired) {
			t.Fatalf("candidate confirm decision=%+v", decision)
		}
		if decision := assistantCheckRequiredChecks(assistantActionGateInput{Stage: assistantActionStageCommit, Action: assistantActionSpec{Security: assistantActionSecuritySpec{RequiredChecks: []string{"dry_run"}}}}); decision.Allowed || decision.ReasonCode != "dry_run_validation_failed" {
			t.Fatalf("dry run missing decision=%+v", decision)
		}
		if decision := assistantCheckRequiredChecks(assistantActionGateInput{Stage: assistantActionStageCommit, Action: assistantActionSpec{Security: assistantActionSecuritySpec{RequiredChecks: []string{"unknown_check"}}}}); decision.Allowed || decision.ReasonCode != "required_check_unknown" {
			t.Fatalf("unknown required check decision=%+v", decision)
		}
		if decision := assistantCheckRequiredChecks(assistantActionGateInput{Stage: assistantActionStageCommit, Action: assistantActionSpec{Security: assistantActionSecuritySpec{RequiredChecks: []string{"", "strict_decode", "boundary_lint"}}}}); !decision.Allowed {
			t.Fatalf("non-plan strict/boundary checks should skip, got %+v", decision)
		}
	})

	t.Run("confirm requirement branch", func(t *testing.T) {
		decision := assistantCheckConfirmRequirements(assistantActionGateInput{Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}})
		if decision.Allowed || !errors.Is(decision.Error, errAssistantConfirmationRequired) {
			t.Fatalf("unexpected decision=%+v", decision)
		}
	})

	t.Run("dry run helpers", func(t *testing.T) {
		if decision := assistantRequiredCheckDenied("custom_reason"); decision.Allowed || decision.ReasonCode != "custom_reason" {
			t.Fatalf("required check denied decision=%+v", decision)
		}
		if got := assistantDryRunValidationErrorsForGate(assistantActionGateInput{DryRun: nil}); len(got) != 1 || got[0] != "dry_run_missing" {
			t.Fatalf("nil dry run errors=%v", got)
		}
		dryRun := assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required", "other_error"}}
		if got := assistantDryRunValidationErrorsForGate(assistantActionGateInput{ResolvedID: "c1", DryRun: &dryRun}); len(got) != 1 || got[0] != "other_error" {
			t.Fatalf("filtered dry run errors=%v", got)
		}
		dryRun = assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required"}}
		if got := assistantDryRunValidationErrorsForGate(assistantActionGateInput{DryRun: &dryRun}); len(got) != 1 || got[0] != "candidate_confirmation_required" {
			t.Fatalf("unresolved dry run errors=%v", got)
		}
		hydrated := assistantDryRunValidationErrorsForGate(assistantActionGateInput{Stage: assistantActionStageCommit, Turn: &assistantTurn{State: assistantStateConfirmed}, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, ResolvedID: "c1", DryRun: &assistantDryRunResult{}})
		if len(hydrated) != 0 {
			t.Fatalf("hydrated errors=%v", hydrated)
		}
		if !assistantDryRunMissing(assistantDryRunResult{}) {
			t.Fatal("expected empty dry run to be missing")
		}
		if assistantDryRunMissing(assistantDryRunResult{Explain: "ok"}) {
			t.Fatal("expected explained dry run to be present")
		}
		if got := assistantHydratedDryRunForGate(assistantStateConfirmed, assistantIntentSpec{}, nil, "c1"); got.Explain == "" {
			t.Fatalf("expected confirmed hydration explain, got %+v", got)
		}
		if got := assistantHydratedDryRunForGate(assistantStateValidated, assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, []assistantCandidate{{CandidateID: "c1"}}, "c1"); len(got.ValidationErrors) != 0 {
			t.Fatalf("expected rebuilt dry run without errors, got %+v", got)
		}
		assistantHydrateTurnForActionGate(nil)
		turn := &assistantTurn{State: assistantStateConfirmed, ResolvedCandidateID: "c1"}
		assistantHydrateTurnForActionGate(turn)
		if turn.DryRun.Explain == "" {
			t.Fatalf("expected turn dry run explain, got %+v", turn.DryRun)
		}
		turn = &assistantTurn{State: assistantStateConfirmed, ResolvedCandidateID: "c1", DryRun: assistantDryRunResult{Explain: "existing"}}
		assistantHydrateTurnForActionGate(turn)
		if turn.DryRun.Explain != "existing" {
			t.Fatalf("expected existing dry run preserved, got %+v", turn.DryRun)
		}
	})

	t.Run("apply gate decision branches", func(t *testing.T) {
		if result, err := assistantApplyGateDecision(nil, nil, Principal{}, "commit", assistantActionGateDecision{Allowed: true}); err != nil || result.PersistTurn {
			t.Fatalf("allowed decision result=%+v err=%v", result, err)
		}
		if _, err := assistantApplyGateDecision(nil, nil, Principal{}, "commit", assistantActionGateDecision{Allowed: false, Error: errAssistantActionRiskGateDenied}); !errors.Is(err, errAssistantActionRiskGateDenied) {
			t.Fatalf("expected original error, got %v", err)
		}
		if _, err := assistantApplyGateDecision(nil, nil, Principal{}, "commit", assistantActionGateDecision{Allowed: false, ErrorCode: "plain_code"}); err == nil || err.Error() != "plain_code" {
			t.Fatalf("expected plain code error, got %v", err)
		}
		conversation := &assistantConversation{}
		turn := &assistantTurn{TurnID: "turn_1", RequestID: "req_1", TraceID: "trace_1", State: assistantStateValidated}
		result, err := assistantApplyGateDecision(conversation, turn, Principal{ID: "actor_1"}, "confirm", assistantActionGateDecision{Allowed: false, Error: errAssistantConfirmationRequired, ErrorCode: errAssistantConfirmationRequired.Error(), ReasonCode: "candidate_confirmation_required"})
		if !errors.Is(err, errAssistantConfirmationRequired) || !result.PersistTurn || turn.ErrorCode != "" || len(conversation.Transitions) != 1 {
			t.Fatalf("confirm decision result=%+v turn=%+v err=%v", result, turn, err)
		}
		conversation = &assistantConversation{}
		turn = &assistantTurn{TurnID: "turn_2", RequestID: "req_2", TraceID: "trace_2", State: assistantStateConfirmed}
		result, err = assistantApplyGateDecision(conversation, turn, Principal{ID: "actor_2"}, "commit", assistantActionGateDecision{Allowed: false, ErrorCode: errAssistantActionRiskGateDenied.Error(), ReasonCode: "blocked"})
		if err == nil || turn.ErrorCode != errAssistantActionRiskGateDenied.Error() || result.Transition == nil {
			t.Fatalf("commit decision result=%+v turn=%+v err=%v", result, turn, err)
		}
	})

	t.Run("expanded actions plan confirm commit matrix", func(t *testing.T) {
		type gateMatrixCase struct {
			name              string
			action            string
			intent            assistantIntentSpec
			requiresCandidate bool
		}
		hasRequiredCheck := func(spec assistantActionSpec, check string) bool {
			target := strings.TrimSpace(check)
			for _, item := range spec.Security.RequiredChecks {
				if strings.TrimSpace(item) == target {
					return true
				}
			}
			return false
		}
		cases := []gateMatrixCase{
			{
				name:              "add_version",
				action:            assistantIntentAddOrgUnitVersion,
				intent:            assistantIntentSpec{Action: assistantIntentAddOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01", NewName: "运营一部", NewParentRefText: "共享服务中心"},
				requiresCandidate: true,
			},
			{
				name:              "insert_version",
				action:            assistantIntentInsertOrgUnitVersion,
				intent:            assistantIntentSpec{Action: assistantIntentInsertOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01", NewName: "运营二部", NewParentRefText: "共享服务中心"},
				requiresCandidate: true,
			},
			{
				name:              "correct",
				action:            assistantIntentCorrectOrgUnit,
				intent:            assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, OrgCode: "FLOWER-C", TargetEffectiveDate: "2026-01-01", NewName: "运营中心", NewParentRefText: "共享服务中心"},
				requiresCandidate: true,
			},
			{
				name:              "move",
				action:            assistantIntentMoveOrgUnit,
				intent:            assistantIntentSpec{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01", NewParentRefText: "共享服务中心"},
				requiresCandidate: true,
			},
			{
				name:              "rename",
				action:            assistantIntentRenameOrgUnit,
				intent:            assistantIntentSpec{Action: assistantIntentRenameOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-03-01", NewName: "运营平台部"},
				requiresCandidate: false,
			},
			{
				name:              "disable",
				action:            assistantIntentDisableOrgUnit,
				intent:            assistantIntentSpec{Action: assistantIntentDisableOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-05-01"},
				requiresCandidate: false,
			},
			{
				name:              "enable",
				action:            assistantIntentEnableOrgUnit,
				intent:            assistantIntentSpec{Action: assistantIntentEnableOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-06-01"},
				requiresCandidate: false,
			},
		}

		originalDefinitions := capabilityDefinitionByKey
		originalAuthorizer := assistantLoadAuthorizerFn
		defer func() {
			capabilityDefinitionByKey = originalDefinitions
			assistantLoadAuthorizerFn = originalAuthorizer
		}()
		assistantLoadAuthorizerFn = func() (authorizer, error) {
			return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
		}
		capabilities := map[string]capabilityDefinition{}
		for _, tc := range cases {
			spec, ok := assistantLookupDefaultActionSpec(tc.action)
			if !ok {
				t.Fatalf("missing default spec for action=%s", tc.action)
			}
			capabilities[spec.CapabilityKey] = capabilityDefinition{
				CapabilityKey:   spec.CapabilityKey,
				Status:          routeCapabilityStatusActive,
				ActivationState: routeCapabilityStatusActive,
			}
		}
		capabilityDefinitionByKey = capabilities

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				spec, ok := assistantLookupDefaultActionSpec(tc.action)
				if !ok {
					t.Fatalf("missing default spec for action=%s", tc.action)
				}
				if got := hasRequiredCheck(spec, "candidate_confirmation"); got != tc.requiresCandidate {
					t.Fatalf("candidate_confirmation required mismatch action=%s got=%v want=%v required_checks=%v", tc.action, got, tc.requiresCandidate, spec.Security.RequiredChecks)
				}

				decision := assistantEvaluateActionGate(assistantActionGateInput{
					Stage:     assistantActionStagePlan,
					TenantID:  "tenant_1",
					Principal: Principal{ID: "actor_1", RoleSlug: "tenant-admin"},
					Action:    spec,
					Intent:    tc.intent,
					UserInput: "请生成计划",
				})
				if !decision.Allowed {
					t.Fatalf("plan allow decision=%+v", decision)
				}

				assistantLoadAuthorizerFn = func() (authorizer, error) {
					return assistantGateAuthorizerStub{allowed: false, enforced: true}, nil
				}
				decision = assistantEvaluateActionGate(assistantActionGateInput{
					Stage:     assistantActionStagePlan,
					TenantID:  "tenant_1",
					Principal: Principal{ID: "actor_1", RoleSlug: "tenant-admin"},
					Action:    spec,
					Intent:    tc.intent,
					UserInput: "请生成计划",
				})
				if decision.Allowed || !errors.Is(decision.Error, errAssistantActionAuthzDenied) || decision.ReasonCode != "action_authz_denied" {
					t.Fatalf("plan authz denied decision=%+v", decision)
				}
				assistantLoadAuthorizerFn = func() (authorizer, error) {
					return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
				}

				decision = assistantEvaluateActionGate(assistantActionGateInput{
					Stage:         assistantActionStageConfirm,
					TenantID:      "tenant_1",
					Principal:     Principal{ID: "actor_1", RoleSlug: "tenant-admin"},
					Action:        spec,
					Intent:        tc.intent,
					RouteDecision: assistantTestBusinessRouteDecision(tc.action),
					Turn:          &assistantTurn{State: assistantStateValidated},
					Candidates:    []assistantCandidate{{CandidateID: "candidate_1"}},
					ResolvedID:    "",
					DryRun:        &assistantDryRunResult{Explain: "ok"},
					UserInput:     "请确认",
				})
				if tc.requiresCandidate {
					if decision.Allowed || !errors.Is(decision.Error, errAssistantConfirmationRequired) || decision.ReasonCode != "candidate_confirmation_required" {
						t.Fatalf("confirm required decision=%+v", decision)
					}
				} else if !decision.Allowed {
					t.Fatalf("confirm should allow decision=%+v", decision)
				}

				resolvedForDryRun := ""
				if tc.requiresCandidate {
					resolvedForDryRun = "candidate_1"
				}
				decision = assistantEvaluateActionGate(assistantActionGateInput{
					Stage:         assistantActionStageCommit,
					TenantID:      "tenant_1",
					Principal:     Principal{ID: "actor_1", RoleSlug: "tenant-admin"},
					Action:        spec,
					Intent:        tc.intent,
					RouteDecision: assistantTestBusinessRouteDecision(tc.action),
					Turn:          &assistantTurn{State: assistantStateConfirmed},
					Candidates:    []assistantCandidate{{CandidateID: "candidate_1"}},
					ResolvedID:    resolvedForDryRun,
					DryRun:        nil,
					UserInput:     "请提交",
				})
				if decision.Allowed || !errors.Is(decision.Error, errAssistantActionRequiredCheckFailed) || decision.ReasonCode != "dry_run_validation_failed" {
					t.Fatalf("commit dry run missing decision=%+v", decision)
				}

				if tc.requiresCandidate {
					decision = assistantEvaluateActionGate(assistantActionGateInput{
						Stage:         assistantActionStageCommit,
						TenantID:      "tenant_1",
						Principal:     Principal{ID: "actor_1", RoleSlug: "tenant-admin"},
						Action:        spec,
						Intent:        tc.intent,
						RouteDecision: assistantTestBusinessRouteDecision(tc.action),
						Turn:          &assistantTurn{State: assistantStateConfirmed},
						Candidates:    []assistantCandidate{{CandidateID: "candidate_1"}},
						ResolvedID:    "",
						DryRun:        &assistantDryRunResult{Explain: "ok"},
						UserInput:     "请提交",
					})
					if decision.Allowed || !errors.Is(decision.Error, errAssistantCandidateNotFound) || decision.ReasonCode != "candidate_missing_at_commit" {
						t.Fatalf("commit candidate missing decision=%+v", decision)
					}
				}

				allowResolvedID := ""
				if tc.requiresCandidate {
					allowResolvedID = "candidate_1"
				}
				decision = assistantEvaluateActionGate(assistantActionGateInput{
					Stage:         assistantActionStageCommit,
					TenantID:      "tenant_1",
					Principal:     Principal{ID: "actor_1", RoleSlug: "tenant-admin"},
					Action:        spec,
					Intent:        tc.intent,
					RouteDecision: assistantTestBusinessRouteDecision(tc.action),
					Turn:          &assistantTurn{State: assistantStateConfirmed},
					Candidates:    []assistantCandidate{{CandidateID: "candidate_1"}},
					ResolvedID:    allowResolvedID,
					DryRun:        &assistantDryRunResult{Explain: "ok"},
					UserInput:     "请提交",
				})
				if !decision.Allowed {
					t.Fatalf("commit allow decision=%+v", decision)
				}
			})
		}
	})
}

func TestAssistantActionInterceptor_ClarificationAndCapabilityGaps(t *testing.T) {
	spec, ok := assistantLookupDefaultActionSpec(assistantIntentCreateOrgUnit)
	if !ok {
		t.Fatal("missing create spec")
	}
	originalDefs := capabilityDefinitionByKey
	defer func() { capabilityDefinitionByKey = originalDefs }()
	capabilityDefinitionByKey = map[string]capabilityDefinition{}

	decision := assistantEvaluateActionGate(assistantActionGateInput{
		Stage: assistantActionStagePlan,
		Action: assistantActionSpec{
			ID:            spec.ID,
			CapabilityKey: spec.CapabilityKey,
		},
		RouteDecision: assistantIntentRouteDecision{
			RouteKind:               assistantRouteKindBusinessAction,
			IntentID:                "org.orgunit_create",
			CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
			ConfidenceBand:          assistantRouteConfidenceHigh,
			RouteCatalogVersion:     "v1",
			KnowledgeSnapshotDigest: "d",
			ResolverContractVersion: "r",
			DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
		},
	})
	if decision.Allowed || !errors.Is(decision.Error, errAssistantActionCapabilityUnregistered) {
		t.Fatalf("capability unregistered decision=%+v", decision)
	}

	validTurn := &assistantTurn{
		State: assistantStateValidated,
		RouteDecision: assistantIntentRouteDecision{
			RouteKind:               assistantRouteKindBusinessAction,
			IntentID:                "org.orgunit_create",
			CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
			ConfidenceBand:          assistantRouteConfidenceHigh,
			RouteCatalogVersion:     "v1",
			KnowledgeSnapshotDigest: "d",
			ResolverContractVersion: "r",
			DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
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
		Phase: assistantPhaseAwaitMissingFields,
	}
	if got := assistantCheckClarificationGate(assistantActionGateInput{Stage: assistantActionStagePlan, Turn: validTurn}); !got.Allowed {
		t.Fatalf("plan stage should bypass open clarification gate, got=%+v", got)
	}

	invalidRuntimeTurn := &assistantTurn{
		State:         assistantStateValidated,
		RouteDecision: validTurn.RouteDecision,
		Clarification: &assistantClarificationDecision{
			Status:            assistantClarificationStatusOpen,
			ClarificationKind: assistantClarificationKindMissingSlots,
			AwaitPhase:        assistantPhaseAwaitMissingFields,
			MaxRounds:         2,
			CurrentRound:      1,
		},
	}
	if got := assistantCheckClarificationGate(assistantActionGateInput{Stage: assistantActionStageConfirm, Turn: invalidRuntimeTurn}); got.Allowed || !errors.Is(got.Error, errAssistantClarificationRuntimeInvalid) {
		t.Fatalf("runtime invalid clarification gate=%+v", got)
	}

	resolved := *validTurn
	resolved.Clarification = &assistantClarificationDecision{
		Status:            assistantClarificationStatusResolved,
		ClarificationKind: assistantClarificationKindMissingSlots,
		AwaitPhase:        assistantPhaseAwaitMissingFields,
		MaxRounds:         2,
		CurrentRound:      1,
	}
	if got := assistantCheckClarificationGate(assistantActionGateInput{Stage: assistantActionStageConfirm, Turn: &resolved}); !got.Allowed {
		t.Fatalf("resolved clarification should pass gate, got=%+v", got)
	}

	staleOpen := *validTurn
	staleOpen.Clarification = &assistantClarificationDecision{
		Status:                  assistantClarificationStatusOpen,
		ClarificationKind:       assistantClarificationKindMissingSlots,
		AwaitPhase:              assistantPhaseAwaitMissingFields,
		MaxRounds:               2,
		CurrentRound:            1,
		ExitTo:                  assistantClarificationExitBusinessResume,
		KnowledgeSnapshotDigest: "d",
		RouteCatalogVersion:     "v1",
	}
	staleOpen.Phase = assistantPhaseIdle
	if got := assistantCheckClarificationGate(assistantActionGateInput{Stage: assistantActionStageConfirm, Turn: &staleOpen}); got.Allowed {
		t.Fatalf("phase mismatch should be blocked, got=%+v", got)
	}

	nonOpen := *validTurn
	nonOpen.Clarification = &assistantClarificationDecision{
		Status:                  assistantClarificationStatusResolved,
		ClarificationKind:       assistantClarificationKindMissingSlots,
		AwaitPhase:              assistantPhaseAwaitMissingFields,
		MaxRounds:               2,
		CurrentRound:            1,
		ExitTo:                  assistantClarificationExitBusinessResume,
		KnowledgeSnapshotDigest: "d",
		RouteCatalogVersion:     "v1",
	}
	if got := assistantCheckClarificationGate(assistantActionGateInput{Stage: assistantActionStageCommit, Turn: &nonOpen}); !got.Allowed {
		t.Fatalf("non-open clarification status should pass gate, got=%+v", got)
	}
}
