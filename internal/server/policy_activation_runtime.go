package server

import (
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	policyActivationStateActive = "active"
	policyActivationStateDraft  = "draft"

	policyActivationCodeVersionRequired = "POLICY_VERSION_REQUIRED"
	policyActivationCodeDraftMissing    = "POLICY_DRAFT_MISSING"
	policyActivationCodeRollbackMissing = "POLICY_ROLLBACK_UNAVAILABLE"
)

type capabilityPolicyState struct {
	CapabilityKey       string `json:"capability_key"`
	ActivationState     string `json:"activation_state"`
	ActivePolicyVersion string `json:"active_policy_version"`
	DraftPolicyVersion  string `json:"draft_policy_version,omitempty"`
	RollbackFromVersion string `json:"rollback_from_version,omitempty"`
	ActivatedAt         string `json:"activated_at,omitempty"`
	ActivatedBy         string `json:"activated_by,omitempty"`
}

type policyActivationRuntime struct {
	mu       sync.RWMutex
	byTenant map[string]map[string]capabilityPolicyState
}

func newPolicyActivationRuntime() *policyActivationRuntime {
	return &policyActivationRuntime{
		byTenant: make(map[string]map[string]capabilityPolicyState),
	}
}

var defaultPolicyActivationRuntime = newPolicyActivationRuntime()

func resetPolicyActivationRuntimeForTest() {
	defaultPolicyActivationRuntime = newPolicyActivationRuntime()
}

func (r *policyActivationRuntime) state(tenantID string, capabilityKey string) (capabilityPolicyState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.ensureLocked(tenantID, capabilityKey)
	if err != nil {
		return capabilityPolicyState{}, err
	}
	return state, nil
}

func (r *policyActivationRuntime) setDraft(tenantID string, capabilityKey string, draftVersion string, operator string) (capabilityPolicyState, error) {
	draftVersion = strings.TrimSpace(draftVersion)
	if draftVersion == "" {
		return capabilityPolicyState{}, errors.New(policyActivationCodeVersionRequired)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.ensureLocked(tenantID, capabilityKey)
	if err != nil {
		return capabilityPolicyState{}, err
	}
	state.DraftPolicyVersion = draftVersion
	state.ActivationState = policyActivationStateDraft
	state.ActivatedBy = strings.TrimSpace(operator)
	r.storeLocked(strings.TrimSpace(tenantID), state)
	return state, nil
}

func (r *policyActivationRuntime) activate(tenantID string, capabilityKey string, targetVersion string, operator string) (capabilityPolicyState, error) {
	targetVersion = strings.TrimSpace(targetVersion)
	if targetVersion == "" {
		return capabilityPolicyState{}, errors.New(policyActivationCodeVersionRequired)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.ensureLocked(tenantID, capabilityKey)
	if err != nil {
		return capabilityPolicyState{}, err
	}
	if strings.TrimSpace(state.DraftPolicyVersion) != targetVersion {
		return capabilityPolicyState{}, errors.New(policyActivationCodeDraftMissing)
	}
	previousActive := strings.TrimSpace(state.ActivePolicyVersion)
	state.ActivePolicyVersion = targetVersion
	state.DraftPolicyVersion = ""
	state.RollbackFromVersion = previousActive
	state.ActivationState = policyActivationStateActive
	state.ActivatedBy = strings.TrimSpace(operator)
	state.ActivatedAt = time.Now().UTC().Format(time.RFC3339)
	r.storeLocked(strings.TrimSpace(tenantID), state)
	return state, nil
}

func (r *policyActivationRuntime) rollback(tenantID string, capabilityKey string, targetVersion string, operator string) (capabilityPolicyState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.ensureLocked(tenantID, capabilityKey)
	if err != nil {
		return capabilityPolicyState{}, err
	}
	targetVersion = strings.TrimSpace(targetVersion)
	if targetVersion == "" {
		targetVersion = strings.TrimSpace(state.RollbackFromVersion)
	}
	if targetVersion == "" {
		return capabilityPolicyState{}, errors.New(policyActivationCodeRollbackMissing)
	}
	previousActive := strings.TrimSpace(state.ActivePolicyVersion)
	state.ActivePolicyVersion = targetVersion
	state.DraftPolicyVersion = ""
	state.RollbackFromVersion = previousActive
	state.ActivationState = policyActivationStateActive
	state.ActivatedBy = strings.TrimSpace(operator)
	state.ActivatedAt = time.Now().UTC().Format(time.RFC3339)
	r.storeLocked(strings.TrimSpace(tenantID), state)
	return state, nil
}

func (r *policyActivationRuntime) activePolicyVersion(tenantID string, capabilityKey string) string {
	state, err := r.state(tenantID, capabilityKey)
	if err != nil {
		return capabilityPolicyVersionBaseline
	}
	version := strings.TrimSpace(state.ActivePolicyVersion)
	if version == "" {
		return capabilityPolicyVersionBaseline
	}
	return version
}

func (r *policyActivationRuntime) ensureLocked(tenantID string, capabilityKey string) (capabilityPolicyState, error) {
	tenantID = strings.TrimSpace(tenantID)
	definition, ok := capabilityDefinitionForKey(capabilityKey)
	if !ok {
		return capabilityPolicyState{}, errors.New(functionalAreaMissingCode)
	}
	capabilityKey = strings.ToLower(strings.TrimSpace(definition.CapabilityKey))
	if tenantID == "" {
		return capabilityPolicyState{}, errors.New("tenant missing")
	}
	byCapability, ok := r.byTenant[tenantID]
	if !ok {
		byCapability = make(map[string]capabilityPolicyState)
		r.byTenant[tenantID] = byCapability
	}
	state, ok := byCapability[capabilityKey]
	if ok {
		return state, nil
	}
	state = capabilityPolicyState{
		CapabilityKey:       capabilityKey,
		ActivationState:     strings.TrimSpace(definition.ActivationState),
		ActivePolicyVersion: strings.TrimSpace(definition.CurrentPolicy),
	}
	if state.ActivePolicyVersion == "" {
		state.ActivePolicyVersion = capabilityPolicyVersionBaseline
	}
	if state.ActivationState == "" {
		state.ActivationState = policyActivationStateActive
	}
	byCapability[capabilityKey] = state
	return state, nil
}

func (r *policyActivationRuntime) storeLocked(tenantID string, state capabilityPolicyState) {
	byCapability, ok := r.byTenant[tenantID]
	if !ok {
		byCapability = make(map[string]capabilityPolicyState)
		r.byTenant[tenantID] = byCapability
	}
	state.CapabilityKey = strings.ToLower(strings.TrimSpace(state.CapabilityKey))
	byCapability[state.CapabilityKey] = state
}
