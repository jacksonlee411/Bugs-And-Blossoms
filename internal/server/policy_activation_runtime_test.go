package server

import "testing"

func TestPolicyActivationRuntime(t *testing.T) {
	runtime := newPolicyActivationRuntime()

	if _, err := runtime.state("", "staffing.assignment_create.field_policy"); err == nil {
		t.Fatal("expected tenant missing error")
	}
	if _, err := runtime.state("t1", "unknown.key"); err == nil {
		t.Fatal("expected unknown capability error")
	}

	state, err := runtime.state("t1", "staffing.assignment_create.field_policy")
	if err != nil {
		t.Fatalf("state err=%v", err)
	}
	if state.ActivePolicyVersion != capabilityPolicyVersionBaseline || state.ActivationState != policyActivationStateActive {
		t.Fatalf("state=%+v", state)
	}

	if _, err := runtime.setDraft("t1", "staffing.assignment_create.field_policy", "", "op"); err == nil || err.Error() != policyActivationCodeVersionRequired {
		t.Fatalf("expected version required error, got=%v", err)
	}
	draftState, err := runtime.setDraft("t1", "staffing.assignment_create.field_policy", "2026-04-01", "op")
	if err != nil {
		t.Fatalf("setDraft err=%v", err)
	}
	if draftState.DraftPolicyVersion != "2026-04-01" || draftState.ActivationState != policyActivationStateDraft {
		t.Fatalf("draft state=%+v", draftState)
	}

	if _, err := runtime.activate("t1", "staffing.assignment_create.field_policy", "", "op"); err == nil || err.Error() != policyActivationCodeVersionRequired {
		t.Fatalf("expected activate version required error, got=%v", err)
	}
	if _, err := runtime.activate("t1", "unknown.key", "2026-05-01", "op"); err == nil || err.Error() != functionalAreaMissingCode {
		t.Fatalf("expected activate unknown capability error, got=%v", err)
	}
	if _, err := runtime.activate("t1", "staffing.assignment_create.field_policy", "2026-05-01", "op"); err == nil || err.Error() != policyActivationCodeDraftMissing {
		t.Fatalf("expected activate draft missing error, got=%v", err)
	}
	activatedState, err := runtime.activate("t1", "staffing.assignment_create.field_policy", "2026-04-01", "op")
	if err != nil {
		t.Fatalf("activate err=%v", err)
	}
	if activatedState.ActivePolicyVersion != "2026-04-01" || activatedState.RollbackFromVersion != capabilityPolicyVersionBaseline || activatedState.DraftPolicyVersion != "" {
		t.Fatalf("activated state=%+v", activatedState)
	}

	if _, err := runtime.rollback("t2", "staffing.assignment_create.field_policy", "", "op"); err == nil || err.Error() != policyActivationCodeRollbackMissing {
		t.Fatalf("expected rollback missing error, got=%v", err)
	}
	if _, err := runtime.rollback("t1", "unknown.key", "2026-04-01", "op"); err == nil || err.Error() != functionalAreaMissingCode {
		t.Fatalf("expected rollback unknown capability error, got=%v", err)
	}
	rolledBackState, err := runtime.rollback("t1", "staffing.assignment_create.field_policy", "", "op")
	if err != nil {
		t.Fatalf("rollback err=%v", err)
	}
	if rolledBackState.ActivePolicyVersion != capabilityPolicyVersionBaseline || rolledBackState.RollbackFromVersion != "2026-04-01" {
		t.Fatalf("rolled back state=%+v", rolledBackState)
	}

	if got := runtime.activePolicyVersion("t1", "staffing.assignment_create.field_policy"); got != capabilityPolicyVersionBaseline {
		t.Fatalf("activePolicyVersion=%q", got)
	}
	if got := runtime.activePolicyVersion("t1", "unknown.key"); got != capabilityPolicyVersionBaseline {
		t.Fatalf("activePolicyVersion=%q", got)
	}

	runtime.byTenant["t1"]["staffing.assignment_create.field_policy"] = capabilityPolicyState{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		ActivationState:     policyActivationStateActive,
		ActivePolicyVersion: "",
	}
	if got := runtime.activePolicyVersion("t1", "staffing.assignment_create.field_policy"); got != capabilityPolicyVersionBaseline {
		t.Fatalf("activePolicyVersion=%q", got)
	}
}

func TestPolicyActivationRuntime_EnsureLockedDefaults(t *testing.T) {
	runtime := newPolicyActivationRuntime()

	const capabilityKey = "test.policy.default"
	oldDef, hasOldDef := capabilityDefinitionByKey[capabilityKey]
	capabilityDefinitionByKey[capabilityKey] = capabilityDefinition{
		CapabilityKey: capabilityKey,
	}
	t.Cleanup(func() {
		if hasOldDef {
			capabilityDefinitionByKey[capabilityKey] = oldDef
			return
		}
		delete(capabilityDefinitionByKey, capabilityKey)
	})

	state, err := runtime.state("tenant-default", capabilityKey)
	if err != nil {
		t.Fatalf("state err=%v", err)
	}
	if state.ActivePolicyVersion != capabilityPolicyVersionBaseline {
		t.Fatalf("active_policy_version=%q", state.ActivePolicyVersion)
	}
	if state.ActivationState != policyActivationStateActive {
		t.Fatalf("activation_state=%q", state.ActivationState)
	}
}

func TestPolicyActivationRuntime_StoreLockedCreatesTenantMap(t *testing.T) {
	runtime := newPolicyActivationRuntime()

	runtime.storeLocked("tenant-x", capabilityPolicyState{
		CapabilityKey:       "ORG.POLICY_ACTIVATION.MANAGE",
		ActivationState:     policyActivationStateActive,
		ActivePolicyVersion: "2026-03-01",
	})

	state, err := runtime.state("tenant-x", "org.policy_activation.manage")
	if err != nil {
		t.Fatalf("state err=%v", err)
	}
	if state.CapabilityKey != "org.policy_activation.manage" {
		t.Fatalf("capability_key=%q", state.CapabilityKey)
	}
	if state.ActivePolicyVersion != "2026-03-01" {
		t.Fatalf("active_policy_version=%q", state.ActivePolicyVersion)
	}
}
