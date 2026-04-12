package server

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

func assistantTestVersionTurnWithReadiness(now time.Time, action string, readiness string) *assistantTurn {
	turn := assistantTaskSampleAppendTurn(now, action)
	snapshot := assistantCloneOrgUnitVersionProjectionSnapshot(turn.DryRun.OrgUnitVersionProjection)
	snapshot.Projection.Readiness = readiness
	switch readiness {
	case "candidate_confirmation_required":
		snapshot.Projection.CandidateConfirmationRequirements = []string{"new_parent_org_code"}
		snapshot.Projection.MissingFields = nil
		snapshot.Projection.RejectionReasons = nil
		snapshot.Projection.PendingDraftSummary = ""
	case "missing_fields":
		snapshot.Projection.CandidateConfirmationRequirements = nil
		snapshot.Projection.MissingFields = []string{"new_name"}
		snapshot.Projection.RejectionReasons = nil
		snapshot.Projection.PendingDraftSummary = ""
	case "rejected":
		snapshot.Projection.CandidateConfirmationRequirements = nil
		snapshot.Projection.MissingFields = nil
		snapshot.Projection.RejectionReasons = []string{"FORBIDDEN"}
		snapshot.Projection.PendingDraftSummary = ""
	case "ready":
		snapshot.Projection.CandidateConfirmationRequirements = nil
		snapshot.Projection.MissingFields = nil
		snapshot.Projection.RejectionReasons = nil
		if snapshot.Projection.PendingDraftSummary == "" {
			snapshot.Projection.PendingDraftSummary = "目标组织：FLOWER-C；新名称：运营一部；生效日期：2026-01-01"
		}
	}
	turn.DryRun.OrgUnitVersionProjection = snapshot
	assistantRefreshTurnDerivedFields(turn)
	return turn
}

func TestAssistantOrgUnitVersionProjectionHelpers(t *testing.T) {
	t.Run("projection for turn branches", func(t *testing.T) {
		if projection, ok := assistantOrgUnitVersionProjectionForTurn(nil); ok || projection != nil {
			t.Fatalf("nil turn projection=%v ok=%v", projection, ok)
		}
		if projection, ok := assistantOrgUnitVersionProjectionForTurn(&assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}}); ok || projection != nil {
			t.Fatalf("wrong action projection=%v ok=%v", projection, ok)
		}
		if projection, ok := assistantOrgUnitVersionProjectionForTurn(&assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentAddOrgUnitVersion}}); ok || projection != nil {
			t.Fatalf("missing snapshot projection=%v ok=%v", projection, ok)
		}
		turn := assistantTaskSampleAppendTurn(time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC), assistantIntentAddOrgUnitVersion)
		if projection, ok := assistantOrgUnitVersionProjectionForTurn(turn); !ok || projection == nil {
			t.Fatalf("expected snapshot, got=%v ok=%v", projection, ok)
		}
	})

	t.Run("snapshot builders and field decision helpers deep copy", func(t *testing.T) {
		if got := assistantCloneOrgUnitVersionFieldDecisions(nil); got != nil {
			t.Fatalf("nil clone=%v", got)
		}
		appendInput := []orgunitservices.OrgUnitAppendVersionFieldDecisionV1{{
			FieldKey:             " name ",
			Visible:              true,
			Required:             true,
			Maintainable:         true,
			FieldPayloadKey:      " new_name ",
			ResolvedDefaultValue: " 运营一部 ",
			DefaultRuleRef:       " default_name ",
			AllowedValueCodes:    []string{"A", "B"},
		}}
		appendDecisions := assistantOrgUnitVersionFieldDecisionsFromAppend(appendInput)
		if appendDecisions[0].FieldKey != "name" ||
			appendDecisions[0].FieldPayloadKey != "new_name" ||
			appendDecisions[0].ResolvedDefaultValue != "运营一部" ||
			appendDecisions[0].DefaultRuleRef != "default_name" ||
			!slices.Equal(appendDecisions[0].AllowedValueCodes, []string{"A", "B"}) {
			t.Fatalf("append decisions=%+v", appendDecisions)
		}
		appendInput[0].AllowedValueCodes[0] = "X"
		if appendDecisions[0].AllowedValueCodes[0] != "A" {
			t.Fatalf("append decisions should deep copy: %+v", appendDecisions)
		}
		if got := assistantOrgUnitVersionFieldDecisionsFromAppend(nil); got != nil {
			t.Fatalf("nil append decisions=%v", got)
		}

		maintainInput := []orgunitservices.OrgUnitMaintainFieldDecisionV1{{
			FieldKey:             " parent_org_code ",
			Visible:              true,
			Maintainable:         true,
			FieldPayloadKey:      " new_parent_org_code ",
			ResolvedDefaultValue: " FLOWER-A ",
			DefaultRuleRef:       " default_parent ",
			AllowedValueCodes:    []string{"FLOWER-A", "FLOWER-B"},
		}}
		maintainDecisions := assistantOrgUnitVersionFieldDecisionsFromMaintain(maintainInput)
		if maintainDecisions[0].FieldKey != "parent_org_code" ||
			maintainDecisions[0].FieldPayloadKey != "new_parent_org_code" ||
			maintainDecisions[0].ResolvedDefaultValue != "FLOWER-A" ||
			maintainDecisions[0].DefaultRuleRef != "default_parent" ||
			!slices.Equal(maintainDecisions[0].AllowedValueCodes, []string{"FLOWER-A", "FLOWER-B"}) {
			t.Fatalf("maintain decisions=%+v", maintainDecisions)
		}
		maintainInput[0].AllowedValueCodes[0] = "FLOWER-X"
		if maintainDecisions[0].AllowedValueCodes[0] != "FLOWER-A" {
			t.Fatalf("maintain decisions should deep copy: %+v", maintainDecisions)
		}
		if got := assistantOrgUnitVersionFieldDecisionsFromMaintain(nil); got != nil {
			t.Fatalf("nil maintain decisions=%v", got)
		}

		clonedDecisions := assistantCloneOrgUnitVersionFieldDecisions(appendDecisions)
		appendDecisions[0].AllowedValueCodes[1] = "Y"
		if clonedDecisions[0].AllowedValueCodes[1] != "B" {
			t.Fatalf("cloned decisions=%+v", clonedDecisions)
		}

		appendSnapshot := assistantOrgUnitVersionProjectionSnapshotFromAppendResult(orgunitservices.OrgUnitAppendVersionPrecheckResultV1{
			PolicyContext: orgunitservices.OrgUnitAppendVersionPolicyContextV1{
				TenantID:            " tenant-1 ",
				CapabilityKey:       " cap.append ",
				Intent:              " add_version ",
				EffectiveDate:       " 2026-01-01 ",
				OrgCode:             " FLOWER-C ",
				OrgNodeKey:          " 10000003 ",
				ResolvedSetID:       " S2601 ",
				SetIDSource:         " custom ",
				PolicyContextDigest: " digest-ctx ",
			},
			Projection: orgunitservices.OrgUnitAppendVersionPrecheckProjectionV1{
				Readiness:                         " ready ",
				MissingFields:                     []string{"effective_date"},
				FieldDecisions:                    appendInput,
				CandidateConfirmationRequirements: []string{"new_parent_org_code"},
				PendingDraftSummary:               " 草案摘要 ",
				EffectivePolicyVersion:            " epv1 ",
				MutationPolicyVersion:             " mpv1 ",
				ResolvedSetID:                     " S2601 ",
				SetIDSource:                       " custom ",
				PolicyExplain:                     " 已就绪 ",
				RejectionReasons:                  []string{"FORBIDDEN"},
				ProjectionDigest:                  " digest-proj ",
			},
		})
		if appendSnapshot.PolicyContextContractVersion != orgunitservices.OrgUnitAppendVersionPolicyContextContractVersionV1 ||
			appendSnapshot.PrecheckProjectionContractVersion != orgunitservices.OrgUnitAppendVersionPrecheckProjectionContractV1 ||
			appendSnapshot.PolicyContext.TenantID != "tenant-1" ||
			appendSnapshot.PolicyContext.OrgCode != "FLOWER-C" ||
			appendSnapshot.Projection.Readiness != "ready" ||
			appendSnapshot.Projection.PendingDraftSummary != "草案摘要" ||
			appendSnapshot.Projection.PolicyExplain != "已就绪" {
			t.Fatalf("append snapshot=%+v", appendSnapshot)
		}

		maintainSnapshot := assistantOrgUnitVersionProjectionSnapshotFromMaintainResult(orgunitservices.OrgUnitMaintainPrecheckResultV1{
			PolicyContext: orgunitservices.OrgUnitMaintainPolicyContextV1{
				TenantID:            " tenant-1 ",
				CapabilityKey:       " cap.maintain ",
				Intent:              " move ",
				EffectiveDate:       " 2026-04-01 ",
				TargetEffectiveDate: " 2026-01-01 ",
				OrgCode:             " FLOWER-C ",
				OrgNodeKey:          " 10000003 ",
				ResolvedSetID:       " S2601 ",
				SetIDSource:         " custom ",
				PolicyContextDigest: " digest-maintain ",
			},
			Projection: orgunitservices.OrgUnitMaintainPrecheckProjectionV1{
				Readiness:                         " candidate_confirmation_required ",
				MissingFields:                     []string{"new_parent_ref_text"},
				FieldDecisions:                    maintainInput,
				CandidateConfirmationRequirements: []string{"new_parent_org_code"},
				PendingDraftSummary:               " 待确认草案 ",
				EffectivePolicyVersion:            " epv2 ",
				MutationPolicyVersion:             " mpv2 ",
				ResolvedSetID:                     " S2601 ",
				SetIDSource:                       " custom ",
				PolicyExplain:                     " 仍需确认 ",
				RejectionReasons:                  []string{"PARENT_CANDIDATE_REQUIRED"},
				ProjectionDigest:                  " digest-maintain-proj ",
			},
		})
		if maintainSnapshot.PolicyContextContractVersion != orgunitservices.OrgUnitMaintainPolicyContextContractVersionV1 ||
			maintainSnapshot.PrecheckProjectionContractVersion != orgunitservices.OrgUnitMaintainPrecheckProjectionContractV1 ||
			maintainSnapshot.PolicyContext.TargetEffectiveDate != "2026-01-01" ||
			maintainSnapshot.Projection.Readiness != "candidate_confirmation_required" ||
			maintainSnapshot.Projection.PendingDraftSummary != "待确认草案" ||
			maintainSnapshot.Projection.PolicyExplain != "仍需确认" {
			t.Fatalf("maintain snapshot=%+v", maintainSnapshot)
		}

		clonedSnapshot := assistantCloneOrgUnitVersionProjectionSnapshot(maintainSnapshot)
		maintainSnapshot.Projection.MissingFields[0] = "changed"
		maintainSnapshot.Projection.FieldDecisions[0].AllowedValueCodes[1] = "FLOWER-Y"
		maintainSnapshot.Projection.CandidateConfirmationRequirements[0] = "changed"
		maintainSnapshot.Projection.RejectionReasons[0] = "changed"
		if clonedSnapshot.Projection.MissingFields[0] != "new_parent_ref_text" ||
			clonedSnapshot.Projection.FieldDecisions[0].AllowedValueCodes[1] != "FLOWER-B" ||
			clonedSnapshot.Projection.CandidateConfirmationRequirements[0] != "new_parent_org_code" ||
			clonedSnapshot.Projection.RejectionReasons[0] != "PARENT_CANDIDATE_REQUIRED" {
			t.Fatalf("cloned snapshot=%+v", clonedSnapshot)
		}
	})

	t.Run("validation errors map missing fields and confirmation", func(t *testing.T) {
		if got := assistantOrgUnitVersionProjectionValidationErrors(nil); got != nil {
			t.Fatalf("nil snapshot errors=%v", got)
		}
		snapshot := assistantTestOrgUnitVersionProjectionSnapshot(assistantIntentAddOrgUnitVersion)
		snapshot.Projection.RejectionReasons = []string{"FORBIDDEN"}
		snapshot.Projection.MissingFields = []string{"effective_date", "target_effective_date", "org_code", "change_fields", "new_name", "new_parent_ref_text", "custom"}
		snapshot.Projection.Readiness = "candidate_confirmation_required"
		got := assistantOrgUnitVersionProjectionValidationErrors(snapshot)
		want := []string{
			"FORBIDDEN",
			"missing_effective_date",
			"missing_target_effective_date",
			"missing_org_code",
			"missing_change_fields",
			"missing_new_name",
			"missing_new_parent_ref_text",
			"FIELD_REQUIRED_VALUE_MISSING",
			"candidate_confirmation_required",
		}
		for _, code := range want {
			if !slices.Contains(got, code) {
				t.Fatalf("expected %q in %v", code, got)
			}
		}
	})

	t.Run("contract missing branches", func(t *testing.T) {
		if assistantOrgUnitVersionProjectionContractMissing(nil) {
			t.Fatal("nil turn should not require contract")
		}
		if assistantOrgUnitVersionProjectionContractMissing(&assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, IntentSchemaVersion: assistantIntentSchemaVersionV1}}) {
			t.Fatal("create action should not use version contract")
		}
		noSchemaTurn := assistantTaskSampleAppendTurn(time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC), assistantIntentAddOrgUnitVersion)
		noSchemaTurn.Intent.IntentSchemaVersion = ""
		if assistantOrgUnitVersionProjectionContractMissing(noSchemaTurn) {
			t.Fatal("empty schema version should not fail contract")
		}

		cases := []struct {
			name   string
			mutate func(*assistantTurn)
		}{
			{
				name: "missing snapshot",
				mutate: func(turn *assistantTurn) {
					turn.DryRun.OrgUnitVersionProjection = nil
				},
			},
			{
				name: "bad policy context contract version",
				mutate: func(turn *assistantTurn) {
					turn.DryRun.OrgUnitVersionProjection.PolicyContextContractVersion = "bad"
				},
			},
			{
				name: "bad projection contract version",
				mutate: func(turn *assistantTurn) {
					turn.DryRun.OrgUnitVersionProjection.PrecheckProjectionContractVersion = "bad"
				},
			},
			{
				name: "missing policy context digest",
				mutate: func(turn *assistantTurn) {
					turn.DryRun.OrgUnitVersionProjection.PolicyContext.PolicyContextDigest = ""
				},
			},
			{
				name: "missing effective policy version",
				mutate: func(turn *assistantTurn) {
					turn.DryRun.OrgUnitVersionProjection.Projection.EffectivePolicyVersion = ""
				},
			},
			{
				name: "missing projection digest",
				mutate: func(turn *assistantTurn) {
					turn.DryRun.OrgUnitVersionProjection.Projection.ProjectionDigest = ""
				},
			},
			{
				name: "missing mutation policy version",
				mutate: func(turn *assistantTurn) {
					turn.DryRun.OrgUnitVersionProjection.Projection.MutationPolicyVersion = ""
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				turn := assistantTaskSampleAppendTurn(time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC), assistantIntentInsertOrgUnitVersion)
				tc.mutate(turn)
				if !assistantOrgUnitVersionProjectionContractMissing(turn) {
					t.Fatalf("expected contract missing for %s", tc.name)
				}
			})
		}

		validTurn := assistantTaskSampleAppendTurn(time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC), assistantIntentAddOrgUnitVersion)
		if assistantOrgUnitVersionProjectionContractMissing(validTurn) {
			t.Fatal("valid projection contract should pass")
		}
	})
}

func TestAssistantOrgUnitVersionPolicyHelpers(t *testing.T) {
	t.Run("binding and required action branches", func(t *testing.T) {
		if binding, ok := assistantOrgUnitVersionPolicyBinding(assistantIntentAddOrgUnitVersion); !ok || binding.CapabilityKey != orgUnitAddVersionFieldPolicyCapabilityKey || binding.AppendIntent != string(orgunitservices.OrgUnitWriteIntentAddVersion) || binding.MaintainIntent != "" {
			t.Fatalf("add binding=%+v ok=%v", binding, ok)
		}
		if binding, ok := assistantOrgUnitVersionPolicyBinding(assistantIntentInsertOrgUnitVersion); !ok || binding.CapabilityKey != orgUnitInsertVersionFieldPolicyCapabilityKey || binding.AppendIntent != string(orgunitservices.OrgUnitWriteIntentInsertVersion) || binding.MaintainIntent != "" {
			t.Fatalf("insert binding=%+v ok=%v", binding, ok)
		}
		if binding, ok := assistantOrgUnitVersionPolicyBinding(assistantIntentCorrectOrgUnit); !ok || binding.CapabilityKey != orgUnitCorrectFieldPolicyCapabilityKey || binding.AppendIntent != "" || binding.MaintainIntent != orgunitservices.OrgUnitMaintainIntentCorrect {
			t.Fatalf("correct binding=%+v ok=%v", binding, ok)
		}
		if binding, ok := assistantOrgUnitVersionPolicyBinding(assistantIntentRenameOrgUnit); !ok || binding.CapabilityKey != orgUnitWriteFieldPolicyCapabilityKey || binding.AppendIntent != "" || binding.MaintainIntent != orgunitservices.OrgUnitMaintainIntentRename {
			t.Fatalf("rename binding=%+v ok=%v", binding, ok)
		}
		if binding, ok := assistantOrgUnitVersionPolicyBinding(assistantIntentMoveOrgUnit); !ok || binding.CapabilityKey != orgUnitWriteFieldPolicyCapabilityKey || binding.AppendIntent != "" || binding.MaintainIntent != orgunitservices.OrgUnitMaintainIntentMove {
			t.Fatalf("move binding=%+v ok=%v", binding, ok)
		}
		if _, ok := assistantOrgUnitVersionPolicyBinding("other"); ok {
			t.Fatal("unexpected binding for other action")
		}

		if !assistantActionRequiresPolicyProjection(assistantIntentCreateOrgUnit) ||
			!assistantActionRequiresPolicyProjection(assistantIntentAddOrgUnitVersion) ||
			!assistantActionRequiresPolicyProjection(assistantIntentInsertOrgUnitVersion) ||
			!assistantActionRequiresPolicyProjection(assistantIntentCorrectOrgUnit) ||
			!assistantActionRequiresPolicyProjection(assistantIntentRenameOrgUnit) ||
			!assistantActionRequiresPolicyProjection(assistantIntentMoveOrgUnit) ||
			assistantActionRequiresPolicyProjection("knowledge_qa") {
			t.Fatal("assistantActionRequiresPolicyProjection branches failed")
		}
	})

	t.Run("parent org code resolution branches", func(t *testing.T) {
		if code, requested, ok := assistantOrgUnitVersionPolicyParentOrgCode(assistantIntentSpec{}, nil, "", nil); code != "" || requested || !ok {
			t.Fatalf("empty ref code=%q requested=%v ok=%v", code, requested, ok)
		}
		candidates := []assistantCandidate{
			{CandidateID: "c1", CandidateCode: "FLOWER-A"},
			{CandidateID: "c2", CandidateCode: "FLOWER-B"},
		}
		intent := assistantIntentSpec{NewParentRefText: "鲜花组织"}
		if code, requested, ok := assistantOrgUnitVersionPolicyParentOrgCode(intent, candidates, "c2", nil); code != "FLOWER-B" || !requested || !ok {
			t.Fatalf("resolved candidate code=%q requested=%v ok=%v", code, requested, ok)
		}
		if code, requested, ok := assistantOrgUnitVersionPolicyParentOrgCode(intent, candidates[:1], "", nil); code != "FLOWER-A" || !requested || !ok {
			t.Fatalf("single candidate code=%q requested=%v ok=%v", code, requested, ok)
		}
		if code, requested, ok := assistantOrgUnitVersionPolicyParentOrgCode(intent, candidates, "", []string{"candidate_confirmation_required"}); code != "" || !requested || !ok {
			t.Fatalf("confirmation code=%q requested=%v ok=%v", code, requested, ok)
		}
		if code, requested, ok := assistantOrgUnitVersionPolicyParentOrgCode(intent, candidates, "", nil); code != "" || !requested || ok {
			t.Fatalf("ambiguous code=%q requested=%v ok=%v", code, requested, ok)
		}
	})

	t.Run("enrichment early returns and success", func(t *testing.T) {
		unchanged := assistantDryRunResult{Explain: "keep"}
		if got := (*assistantConversationService)(nil).enrichAuthoritativeOrgUnitDryRunWithPolicy(context.Background(), "tenant-1", assistantIntentSpec{Action: assistantIntentAddOrgUnitVersion}, nil, "", unchanged); got.Explain != "keep" {
			t.Fatalf("nil service should keep dry run: %+v", got)
		}
		if got := (&assistantConversationService{}).enrichOrgUnitVersionDryRunWithPolicy(context.Background(), "tenant-1", assistantIntentSpec{Action: assistantIntentAddOrgUnitVersion}, nil, "", unchanged); got.Explain != "keep" {
			t.Fatalf("missing store should keep dry run: %+v", got)
		}

		storeWithErr := assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), fieldConfigsErr: errors.New("boom")}
		svcWithErr := &assistantConversationService{orgStore: storeWithErr}
		if got := svcWithErr.enrichOrgUnitVersionDryRunWithPolicy(context.Background(), "tenant-1", assistantIntentSpec{Action: assistantIntentPlanOnly}, nil, "", unchanged); got.Explain != "keep" {
			t.Fatalf("non business action should keep dry run: %+v", got)
		}
		if got := svcWithErr.enrichOrgUnitVersionDryRunWithPolicy(context.Background(), "tenant-1", assistantIntentSpec{Action: assistantIntentAddOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-01-01"}, nil, "", assistantDryRunResult{ValidationErrors: []string{"parent_candidate_not_found"}, Explain: "keep"}); got.Explain != "keep" {
			t.Fatalf("parent candidate not found should short-circuit: %+v", got)
		}
		if got := svcWithErr.enrichOrgUnitVersionDryRunWithPolicy(context.Background(), "tenant-1", assistantIntentSpec{
			Action:           assistantIntentAddOrgUnitVersion,
			OrgCode:          "FLOWER-C",
			EffectiveDate:    "2026-01-01",
			NewName:          "运营一部",
			NewParentRefText: "鲜花组织",
		}, []assistantCandidate{{CandidateID: "c1"}, {CandidateID: "c2"}}, "", unchanged); got.Explain != "keep" {
			t.Fatalf("ambiguous parent should short-circuit: %+v", got)
		}
		if got := svcWithErr.enrichOrgUnitVersionDryRunWithPolicy(context.Background(), "tenant-1", assistantIntentSpec{
			Action:        assistantIntentAddOrgUnitVersion,
			OrgCode:       "FLOWER-C",
			EffectiveDate: "2026-01-01",
			NewName:       "运营一部",
		}, nil, "", unchanged); got.Explain != "keep" {
			t.Fatalf("precheck error should keep dry run: %+v", got)
		}

		previous := defaultSetIDStrategyRegistryStore
		defer func() { defaultSetIDStrategyRegistryStore = previous }()
		defaultSetIDStrategyRegistryStore = setIDStrategyRegistryStoreStub{
			resolveFieldDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string, _ string) (setIDFieldDecision, error) {
				switch fieldKey {
				case "name", "parent_org_code":
					return setIDFieldDecision{FieldKey: fieldKey, Visible: true, Maintainable: true}, nil
				default:
					return setIDFieldDecision{}, nil
				}
			},
		}

		store := assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}
		parent, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true)
		if err != nil {
			t.Fatalf("seed parent err=%v", err)
		}
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-C", "运营部", strconv.Itoa(parent.OrgID), false); err != nil {
			t.Fatalf("seed child err=%v", err)
		}
		svc := &assistantConversationService{orgStore: store}
		adminCtx := withPrincipal(context.Background(), Principal{ID: "actor-1", RoleSlug: "tenant-admin"})
		result := svc.enrichOrgUnitVersionDryRunWithPolicy(adminCtx, "tenant-1", assistantIntentSpec{
			Action:        assistantIntentAddOrgUnitVersion,
			OrgCode:       "FLOWER-C",
			EffectiveDate: "2026-01-02",
			NewName:       "运营一部",
		}, nil, "", assistantDryRunResult{})
		if result.OrgUnitVersionProjection == nil {
			t.Fatal("expected orgunit version projection")
		}
		if got := result.OrgUnitVersionProjection.Projection.Readiness; got != "ready" {
			t.Fatalf("readiness=%q", got)
		}
		if len(result.ValidationErrors) != 0 || result.Explain == "" {
			t.Fatalf("dry run=%+v", result)
		}
	})
}

func TestAssistantOrgUnitVersionPhaseTaskAndHelperCoverage(t *testing.T) {
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)

	t.Run("phase snapshot uses version projection readiness", func(t *testing.T) {
		pickTurn := assistantTestVersionTurnWithReadiness(now, assistantIntentAddOrgUnitVersion, "candidate_confirmation_required")
		pickTurn.ResolvedCandidateID = ""
		pickTurn.SelectedCandidateID = ""
		assistantRefreshTurnDerivedFields(pickTurn)
		if pickTurn.Phase != assistantPhaseAwaitCandidatePick {
			t.Fatalf("candidate pick phase=%q", pickTurn.Phase)
		}

		confirmTurn := assistantTestVersionTurnWithReadiness(now, assistantIntentInsertOrgUnitVersion, "candidate_confirmation_required")
		confirmTurn.State = assistantStateValidated
		confirmTurn.ResolvedCandidateID = "c1"
		assistantRefreshTurnDerivedFields(confirmTurn)
		if confirmTurn.Phase != assistantPhaseAwaitCandidateConfirm {
			t.Fatalf("candidate confirm phase=%q", confirmTurn.Phase)
		}

		missingTurn := assistantTestVersionTurnWithReadiness(now, assistantIntentAddOrgUnitVersion, "missing_fields")
		if missingTurn.Phase != assistantPhaseAwaitMissingFields {
			t.Fatalf("missing fields phase=%q", missingTurn.Phase)
		}

		rejectedTurn := assistantTestVersionTurnWithReadiness(now, assistantIntentAddOrgUnitVersion, "rejected")
		if rejectedTurn.Phase != assistantPhaseFailed {
			t.Fatalf("rejected phase=%q", rejectedTurn.Phase)
		}

		readyTurn := assistantTestVersionTurnWithReadiness(now, assistantIntentInsertOrgUnitVersion, "ready")
		if readyTurn.Phase != assistantPhaseAwaitCommitConfirm {
			t.Fatalf("ready phase=%q", readyTurn.Phase)
		}
	})

	t.Run("task snapshot validation respects version policy contract", func(t *testing.T) {
		turn := assistantTaskSampleAppendTurn(now, assistantIntentAddOrgUnitVersion)
		req := assistantTaskSampleRequest(turn)
		req.ContractSnapshot.MutationPolicyVersion = ""
		if err := assistantTaskValidateSubmitRequest(req); err == nil {
			t.Fatal("expected contract snapshot incomplete")
		}

		if built, err := assistantBuildTaskSubmitRequestFromTurn("conv_1", turn); err != nil {
			t.Fatalf("unexpected build submit err=%v", err)
		} else if built.ContractSnapshot.PolicyContextDigest == "" || built.ContractSnapshot.PrecheckProjectionDigest == "" {
			t.Fatalf("submit request snapshot=%+v", built.ContractSnapshot)
		}

		badTurn := assistantTaskSampleAppendTurn(now, assistantIntentInsertOrgUnitVersion)
		badTurn.DryRun.OrgUnitVersionProjection.Projection.MutationPolicyVersion = ""
		if _, err := assistantBuildTaskSubmitRequestFromTurn("conv_1", badTurn); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
			t.Fatalf("expected contract mismatch, got=%v", err)
		}
	})

	t.Run("confirm rebuild fail-closed on missing version projection contract", func(t *testing.T) {
		svc := &assistantConversationService{}
		turn := assistantTaskSampleAppendTurn(now, assistantIntentAddOrgUnitVersion)
		turn.State = assistantStateValidated
		turn.Plan.ExpiresAt = time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
		turn.AmbiguityCount = 2
		turn.Candidates = []assistantCandidate{
			{CandidateID: "c1", CandidateCode: "FLOWER-A", Name: "鲜花组织"},
			{CandidateID: "c2", CandidateCode: "FLOWER-B", Name: "花店组织"},
		}
		turn.ResolvedCandidateID = ""
		conversation := &assistantConversation{TenantID: "tenant-1"}
		if _, err := svc.applyConfirmTurn(conversation, turn, Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "c1"); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
			t.Fatalf("expected plan contract mismatch, got=%v", err)
		}
	})

	t.Run("api and registry helpers", func(t *testing.T) {
		if got := assistantOpaqueCandidateID("", ""); got != "" {
			t.Fatalf("opaque candidate id=%q", got)
		}
		if got := assistantDryRunValidationExplain([]string{"FIELD_REQUIRED_VALUE_MISSING"}); got != "当前组织创建策略缺少可用默认值，请联系管理员补齐 org_code / 组织类型策略后重试。" {
			t.Fatalf("field required explain=%q", got)
		}
		if got := assistantDryRunValidationExplain([]string{"PATCH_FIELD_NOT_ALLOWED"}); got != "当前租户未启用创建所需组织字段配置，请联系管理员启用 org_type 字段后重试。" {
			t.Fatalf("patch field explain=%q", got)
		}

		svc := &assistantConversationService{orgStore: assistantOrgStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			search: []OrgUnitSearchCandidate{{
				OrgID:   10000001,
				OrgCode: "FLOWER-A",
				Name:    "鲜花组织",
				Status:  "active",
			}},
			detailsByNodeKey: OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "FLOWER-A", FullNamePath: ""},
		}}
		candidates, err := svc.resolveCandidates(context.Background(), "tenant-1", "鲜花组织", "2026-01-01")
		if err != nil {
			t.Fatalf("resolve candidates err=%v", err)
		}
		if len(candidates) != 1 || candidates[0].Path != "鲜花组织" {
			t.Fatalf("candidates=%+v", candidates)
		}

		orgNodeKey := mustOrgNodeKeyForTest(t, 10000001)
		key, err := svc.resolveAssistantCandidateOrgNodeKey(context.Background(), "tenant-1", assistantCandidate{CandidateID: orgNodeKey})
		if err != nil || key != orgNodeKey {
			t.Fatalf("resolved org node key=%q err=%v", key, err)
		}
	})
}
