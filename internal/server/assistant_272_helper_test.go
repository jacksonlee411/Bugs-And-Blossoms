package server

import (
	"context"
	"errors"
	"testing"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

func TestAssistant272Coverage_ActionHelpersAndAdapters(t *testing.T) {
	if !assistantIntentRequiresCandidateConfirmation(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}) {
		t.Fatal("create should require candidate confirmation")
	}
	if assistantIntentRequiresCandidateConfirmation(assistantIntentSpec{Action: assistantIntentMoveOrgUnit}) {
		t.Fatal("move without new parent should not require candidate confirmation")
	}
	if !assistantIntentRequiresCandidateConfirmation(assistantIntentSpec{Action: assistantIntentMoveOrgUnit, NewParentRefText: "共享服务中心"}) {
		t.Fatal("move with new parent should require candidate confirmation")
	}
	if assistantIntentRequiresCandidateConfirmation(assistantIntentSpec{Action: assistantIntentRenameOrgUnit}) {
		t.Fatal("rename should not require candidate confirmation")
	}
	if got := assistantIntentCandidateAsOf(assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, TargetEffectiveDate: "2026-01-01"}); got != "2026-01-01" {
		t.Fatalf("candidate as-of=%s", got)
	}
	if got := assistantIntentCandidateAsOf(assistantIntentSpec{Action: assistantIntentMoveOrgUnit, EffectiveDate: "2026-04-01"}); got != "2026-04-01" {
		t.Fatalf("candidate as-of=%s", got)
	}

	if patch := assistantWritePatchFromIntent(assistantIntentSpec{}, ""); patch.Name != nil || patch.ParentOrgCode != nil {
		t.Fatalf("expected empty patch, got %+v", patch)
	}
	patch := assistantWritePatchFromIntent(assistantIntentSpec{NewName: "运营一部"}, "FLOWER-B")
	if patch.Name == nil || *patch.Name != "运营一部" || patch.ParentOrgCode == nil || *patch.ParentOrgCode != "FLOWER-B" {
		t.Fatalf("unexpected patch=%+v", patch)
	}

	if err := assistantCommitAdapterReady(nil, assistantCommitRequest{}); !errors.Is(err, errAssistantServiceMissing) {
		t.Fatalf("expected service missing, got %v", err)
	}
	if err := assistantCommitAdapterReady(&assistantWriteServiceRecorder{}, assistantCommitRequest{}); !errors.Is(err, errAssistantConversationCorrupted) {
		t.Fatalf("expected corrupted, got %v", err)
	}
	if err := assistantCommitAdapterReady(&assistantWriteServiceRecorder{}, assistantCommitRequest{Turn: &assistantTurn{}}); err != nil {
		t.Fatalf("unexpected ready err=%v", err)
	}

	recorder := &assistantWriteServiceRecorder{}
	addAdapter := assistantAddOrgUnitVersionCommitAdapter{writeSvc: recorder}
	addResult, addErr := addAdapter.Commit(context.Background(), assistantCommitRequest{
		TenantID:          "tenant_1",
		Principal:         Principal{ID: "actor_1"},
		Turn:              &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentAddOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01", NewName: "运营一部"}, PolicyVersion: capabilityPolicyVersionBaseline, RequestID: "req_add"},
		ResolvedCandidate: assistantCandidate{CandidateCode: "FLOWER-B"},
	})
	if addErr != nil || addResult == nil || recorder.writeReq.Intent != string(orgunitservices.OrgUnitWriteIntentAddVersion) || recorder.writeReq.Patch.ParentOrgCode == nil || *recorder.writeReq.Patch.ParentOrgCode != "FLOWER-B" {
		t.Fatalf("unexpected add result=%+v err=%v req=%+v", addResult, addErr, recorder.writeReq)
	}
	recorder.writeErr = errors.New("add write failed")
	if _, err := addAdapter.Commit(context.Background(), assistantCommitRequest{TenantID: "tenant_1", Principal: Principal{ID: "actor_1"}, Turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentAddOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01"}, RequestID: "req_add_err"}}); err == nil {
		t.Fatal("expected add write error")
	}
	recorder.writeErr = nil
	insertAdapter := assistantInsertOrgUnitVersionCommitAdapter{writeSvc: recorder}
	result, err := insertAdapter.Commit(context.Background(), assistantCommitRequest{
		TenantID:  "tenant_1",
		Principal: Principal{ID: "actor_1"},
		Turn: &assistantTurn{
			Intent:        assistantIntentSpec{Action: assistantIntentInsertOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01", NewName: "运营二部"},
			PolicyVersion: capabilityPolicyVersionBaseline,
			RequestID:     "req_insert",
		},
	})
	if err != nil || result == nil || recorder.writeReq.Intent != string(orgunitservices.OrgUnitWriteIntentInsertVersion) {
		t.Fatalf("unexpected insert result=%+v err=%v req=%+v", result, err, recorder.writeReq)
	}
	recorder.writeErr = errors.New("write failed")
	if _, err := insertAdapter.Commit(context.Background(), assistantCommitRequest{TenantID: "tenant_1", Principal: Principal{ID: "actor_1"}, Turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentInsertOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01"}, PolicyVersion: capabilityPolicyVersionBaseline, RequestID: "req_insert_err"}}); err == nil {
		t.Fatal("expected insert write error")
	}
	recorder.writeErr = nil
	recorder.correctErr = errors.New("correct failed")
	if _, err := (assistantCorrectOrgUnitCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{TenantID: "tenant_1", Principal: Principal{ID: "actor_1"}, Turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, OrgCode: "FLOWER-C", TargetEffectiveDate: "2026-01-01"}, RequestID: "req_correct_err"}}); err == nil {
		t.Fatal("expected correct error")
	}
	recorder.correctErr = nil
	correctResult, correctErr := (assistantCorrectOrgUnitCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{TenantID: "tenant_1", Principal: Principal{ID: "actor_1"}, Turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, OrgCode: "FLOWER-C", TargetEffectiveDate: "2026-01-01", NewName: "运营中心"}, RequestID: "req_correct_ok"}, ResolvedCandidate: assistantCandidate{CandidateCode: "FLOWER-B"}})
	if correctErr != nil || correctResult == nil || recorder.correctReq.Patch.ParentOrgCode == nil || *recorder.correctReq.Patch.ParentOrgCode != "FLOWER-B" {
		t.Fatalf("unexpected correct success result=%+v err=%v req=%+v", correctResult, correctErr, recorder.correctReq)
	}
	recorder.disableErr = errors.New("disable failed")
	if _, err := (assistantDisableOrgUnitCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{TenantID: "tenant_1", Principal: Principal{ID: "actor_1"}, Turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentDisableOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-05-01"}}}); err == nil {
		t.Fatal("expected disable error")
	}
	recorder.disableErr = nil
	recorder.enableErr = errors.New("enable failed")
	if _, err := (assistantEnableOrgUnitCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{TenantID: "tenant_1", Principal: Principal{ID: "actor_1"}, Turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentEnableOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-06-01"}}}); err == nil {
		t.Fatal("expected enable error")
	}
	recorder.enableErr = nil
	recorder.moveErr = errors.New("move failed")
	if _, err := (assistantMoveOrgUnitCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{TenantID: "tenant_1", Principal: Principal{ID: "actor_1"}, Turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01"}}, ResolvedCandidate: assistantCandidate{CandidateCode: "FLOWER-B"}}); err == nil {
		t.Fatal("expected move error")
	}
	recorder.moveErr = nil
	recorder.renameErr = errors.New("rename failed")
	if _, err := (assistantRenameOrgUnitCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{TenantID: "tenant_1", Principal: Principal{ID: "actor_1"}, Turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentRenameOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-03-01", NewName: "运营平台部"}}}); err == nil {
		t.Fatal("expected rename error")
	}
	recorder.renameErr = nil
	if _, err := (assistantAddOrgUnitVersionCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{}); !errors.Is(err, errAssistantConversationCorrupted) {
		t.Fatalf("expected add corrupted, got %v", err)
	}
	if _, err := (assistantInsertOrgUnitVersionCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{}); !errors.Is(err, errAssistantConversationCorrupted) {
		t.Fatalf("expected insert corrupted, got %v", err)
	}
	if _, err := (assistantCorrectOrgUnitCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{}); !errors.Is(err, errAssistantConversationCorrupted) {
		t.Fatalf("expected correct corrupted, got %v", err)
	}
	if _, err := (assistantDisableOrgUnitCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{}); !errors.Is(err, errAssistantConversationCorrupted) {
		t.Fatalf("expected disable corrupted, got %v", err)
	}
	if _, err := (assistantEnableOrgUnitCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{}); !errors.Is(err, errAssistantConversationCorrupted) {
		t.Fatalf("expected enable corrupted, got %v", err)
	}
	if _, err := (assistantMoveOrgUnitCommitAdapter{writeSvc: recorder}).Commit(context.Background(), assistantCommitRequest{}); !errors.Is(err, errAssistantConversationCorrupted) {
		t.Fatalf("expected move corrupted, got %v", err)
	}

}

func TestAssistant272Coverage_DryRunAndValidation(t *testing.T) {
	cases := []struct {
		name        string
		intent      assistantIntentSpec
		candidates  []assistantCandidate
		resolvedID  string
		wantErrCode string
	}{
		{name: "add_missing_fields", intent: assistantIntentSpec{Action: assistantIntentAddOrgUnitVersion}, wantErrCode: "missing_org_code"},
		{name: "correct_missing_target", intent: assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, OrgCode: "FLOWER-C"}, wantErrCode: "missing_target_effective_date"},
		{name: "rename_missing_name", intent: assistantIntentSpec{Action: assistantIntentRenameOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-03-01"}, wantErrCode: "missing_new_name"},
		{name: "move_missing_parent", intent: assistantIntentSpec{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01"}, wantErrCode: "missing_new_parent_ref_text"},
		{name: "disable_missing_date", intent: assistantIntentSpec{Action: assistantIntentDisableOrgUnit, OrgCode: "FLOWER-C"}, wantErrCode: "missing_effective_date"},
		{name: "candidate_not_found", intent: assistantIntentSpec{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01", NewParentRefText: "共享服务中心"}, wantErrCode: "parent_candidate_not_found"},
		{name: "candidate_confirmation", intent: assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, OrgCode: "FLOWER-C", TargetEffectiveDate: "2026-01-01", NewParentRefText: "共享服务中心"}, candidates: []assistantCandidate{{CandidateID: "A"}, {CandidateID: "B"}}, wantErrCode: "candidate_confirmation_required"},
		{name: "create_parent_missing", intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, wantErrCode: "parent_candidate_not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dryRun := assistantBuildDryRun(tc.intent, tc.candidates, tc.resolvedID)
			if len(dryRun.ValidationErrors) == 0 || dryRun.ValidationErrors[0] != tc.wantErrCode {
				t.Fatalf("unexpected dryRun=%+v", dryRun)
			}
		})
	}

	goodExplain := assistantDryRunValidationExplain([]string{"missing_new_name", "missing_org_code", "missing_target_effective_date", "missing_change_fields"})
	if goodExplain == "" {
		t.Fatal("expected non-empty explain")
	}
	if got := assistantDryRunValidationExplain(nil); got == "" {
		t.Fatal("expected default explain")
	}
	if got := assistantDryRunValidationExplain([]string{"candidate_confirmation_required"}); got == "" {
		t.Fatal("expected candidate confirmation explain")
	}
	if got := assistantDryRunValidationExplain([]string{"parent_candidate_not_found"}); got == "" {
		t.Fatal("expected parent not found explain")
	}
	if got := assistantDryRunValidationExplain([]string{"invalid_target_effective_date_format"}); got == "" {
		t.Fatal("expected invalid target date explain")
	}
	if got := assistantDryRunValidationExplain([]string{"unknown_code"}); got == "" {
		t.Fatal("expected unknown-code explain fallback")
	}
	createDryRun := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, []assistantCandidate{{CandidateID: "A"}, {CandidateID: "B"}}, "")
	if len(createDryRun.Diff) == 0 {
		t.Fatalf("unexpected create dryrun=%+v", createDryRun)
	}
	createResolved := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, nil, "A")
	if len(createResolved.Diff) < 3 {
		t.Fatalf("unexpected create resolved dryrun=%+v", createResolved)
	}
	insertResolved := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentInsertOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01", NewName: "运营二部", NewParentRefText: "共享服务中心"}, nil, "FLOWER-B")
	if len(insertResolved.Diff) < 4 {
		t.Fatalf("unexpected insert resolved dryrun=%+v", insertResolved)
	}
	insertPending := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentInsertOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01", NewParentRefText: "共享服务中心"}, []assistantCandidate{{CandidateID: "A"}, {CandidateID: "B"}}, "")
	if len(insertPending.Diff) < 3 {
		t.Fatalf("unexpected insert pending dryrun=%+v", insertPending)
	}
	correctPending := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, OrgCode: "FLOWER-C", TargetEffectiveDate: "2026-01-01", NewName: "运营中心", NewParentRefText: "共享服务中心"}, []assistantCandidate{{CandidateID: "A"}, {CandidateID: "B"}}, "")
	if len(correctPending.Diff) < 3 {
		t.Fatalf("unexpected correct pending dryrun=%+v", correctPending)
	}
	correctResolved := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, OrgCode: "FLOWER-C", TargetEffectiveDate: "2026-01-01", NewParentRefText: "共享服务中心"}, nil, "FLOWER-B")
	if len(correctResolved.Diff) < 3 {
		t.Fatalf("unexpected correct resolved dryrun=%+v", correctResolved)
	}
	moveResolved := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01", NewParentRefText: "共享服务中心"}, nil, "FLOWER-B")
	if len(moveResolved.Diff) < 3 {
		t.Fatalf("unexpected move resolved dryrun=%+v", moveResolved)
	}
	movePending := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01", NewParentRefText: "共享服务中心"}, []assistantCandidate{{CandidateID: "A"}, {CandidateID: "B"}}, "")
	if len(movePending.Diff) < 3 {
		t.Fatalf("unexpected move pending dryrun=%+v", movePending)
	}
	moveSingleMatch := assistantBuildDryRunWithRetrieval(
		assistantIntentSpec{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01", NewParentRefText: "共享服务中心"},
		nil,
		"",
		assistantSemanticRetrievalResult{State: assistantSemanticRetrievalStateSingleMatch},
	)
	if len(moveSingleMatch.ValidationErrors) != 0 {
		t.Fatalf("unexpected single-match dryrun=%+v", moveSingleMatch)
	}
	enableDryRun := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentEnableOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-06-01"}, nil, "")
	if len(enableDryRun.Diff) == 0 || len(enableDryRun.ValidationErrors) != 0 {
		t.Fatalf("unexpected enable dryrun=%+v", enableDryRun)
	}
	disableDryRun := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentDisableOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-05-01"}, nil, "")
	if len(disableDryRun.Diff) == 0 || len(disableDryRun.ValidationErrors) != 0 {
		t.Fatalf("unexpected disable dryrun=%+v", disableDryRun)
	}
	if errs := assistantIntentValidationErrors(assistantIntentSpec{Action: assistantIntentEnableOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-06-01"}); len(errs) != 0 {
		t.Fatalf("unexpected enable errs=%v", errs)
	}
	if errs := assistantIntentValidationErrors(assistantIntentSpec{Action: assistantIntentAddOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "bad", NewName: "运营一部"}); len(errs) == 0 || errs[0] != "invalid_effective_date_format" {
		t.Fatalf("unexpected add invalid errs=%v", errs)
	}
	if errs := assistantIntentValidationErrors(assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, OrgCode: "FLOWER-C", TargetEffectiveDate: "bad", NewName: "运营中心"}); len(errs) == 0 || errs[0] != "invalid_target_effective_date_format" {
		t.Fatalf("unexpected correct invalid errs=%v", errs)
	}
	compileCases := []assistantIntentSpec{
		{Action: assistantIntentPlanOnly},
		{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"},
		{Action: assistantIntentAddOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01", NewName: "运营一部"},
		{Action: assistantIntentInsertOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01", NewName: "运营二部"},
		{Action: assistantIntentCorrectOrgUnit, OrgCode: "FLOWER-C", TargetEffectiveDate: "2026-01-01", NewName: "运营中心", NewParentRefText: "共享服务中心"},
		{Action: assistantIntentRenameOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-03-01", NewName: "运营平台部"},
		{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01", NewParentRefText: "共享服务中心"},
		{Action: assistantIntentDisableOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-05-01"},
		{Action: assistantIntentEnableOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-06-01"},
	}
	for _, intent := range compileCases {
		spec, ok := assistantLookupDefaultActionSpec(intent.Action)
		if !ok {
			t.Fatalf("missing spec for %s", intent.Action)
		}
		skill, delta := assistantCompileIntentToPlansWithSpec(intent, "FLOWER-B", spec)
		if intent.Action == assistantIntentPlanOnly {
			if len(skill.SelectedSkills) != 0 || delta.CapabilityKey == "" {
				t.Fatalf("plan_only should no longer compile as executable action skill=%+v delta=%+v", skill, delta)
			}
			continue
		}
		if len(skill.SelectedSkills) == 0 || delta.CapabilityKey == "" {
			t.Fatalf("unexpected compile output action=%s skill=%+v delta=%+v", intent.Action, skill, delta)
		}
	}
	fallbackSkill, fallbackDelta := assistantCompileIntentToPlansWithSpec(assistantIntentSpec{Action: "unknown_action"}, "", assistantActionSpec{})
	if fallbackSkill.RiskTier == "" || len(fallbackSkill.RequiredChecks) == 0 || fallbackDelta.CapabilityKey == "" {
		t.Fatalf("unexpected fallback compile skill=%+v delta=%+v", fallbackSkill, fallbackDelta)
	}
	moveSkill, moveDelta := assistantCompileIntentToPlansWithSpec(assistantIntentSpec{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01", NewParentRefText: "共享服务中心"}, "", assistantActionSpec{CapabilityKey: "org.orgunit_write.field_policy", Security: assistantActionSecuritySpec{RiskTier: "high", RequiredChecks: []string{"strict_decode"}}})
	if len(moveSkill.SelectedSkills) == 0 || len(moveDelta.Changes) < 3 {
		t.Fatalf("unexpected move unresolved compile skill=%+v delta=%+v", moveSkill, moveDelta)
	}
}

func TestAssistant272Coverage_ModelNormalizeAndMissingFields(t *testing.T) {
	actionCases := map[string]string{
		"add_version":    assistantIntentAddOrgUnitVersion,
		"insert_version": assistantIntentInsertOrgUnitVersion,
		"correct":        assistantIntentCorrectOrgUnit,
		"rename":         assistantIntentRenameOrgUnit,
		"move":           assistantIntentMoveOrgUnit,
		"disable":        assistantIntentDisableOrgUnit,
		"enable":         assistantIntentEnableOrgUnit,
		"custom_action":  "custom_action",
	}
	for input, want := range actionCases {
		if got := assistantNormalizeOpenAIIntentAction(input); got != want {
			t.Fatalf("input=%s got=%s want=%s", input, got, want)
		}
	}

	payload := assistantNormalizeOpenAIIntentPayload(`{"action":"correct","orgCode":"FLOWER-C","targetEffectiveDate":"2026-01-01","newName":"运营中心","newParentName":"共享服务中心"}`)
	intent, err := assistantStrictDecodeIntent(payload)
	if err != nil {
		t.Fatalf("decode err=%v payload=%s", err, string(payload))
	}
	if intent.Action != assistantIntentCorrectOrgUnit || intent.OrgCode != "FLOWER-C" || intent.TargetEffectiveDate != "2026-01-01" || intent.NewName != "运营中心" || intent.NewParentRefText != "共享服务中心" {
		t.Fatalf("unexpected normalized intent=%+v", intent)
	}

	turn := &assistantTurn{DryRun: assistantDryRunResult{ValidationErrors: []string{"missing_new_parent_ref_text", "missing_new_name", "missing_org_code", "missing_target_effective_date", "missing_change_fields"}}}
	fields := assistantTurnMissingFields(turn)
	if len(fields) != 5 {
		t.Fatalf("unexpected fields=%v", fields)
	}
}
