package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const (
	orgUnitEffectivePolicyVersionAlgorithm = "epv1"
)

type orgUnitEffectivePolicyVersionParts struct {
	IntentCapabilityKey   string `json:"intent_capability_key"`
	IntentPolicyVersion   string `json:"intent_policy_version"`
	BaselineCapabilityKey string `json:"baseline_capability_key"`
	BaselinePolicyVersion string `json:"baseline_policy_version"`
}

func resolveOrgUnitEffectivePolicyVersion(tenantID string, intentCapabilityKey string) (string, orgUnitEffectivePolicyVersionParts) {
	intentCapabilityKey = strings.ToLower(strings.TrimSpace(intentCapabilityKey))
	parts := orgUnitEffectivePolicyVersionParts{
		IntentCapabilityKey: intentCapabilityKey,
		IntentPolicyVersion: strings.TrimSpace(defaultPolicyActivationRuntime.activePolicyVersion(tenantID, intentCapabilityKey)),
	}
	if baselineCapabilityKey, ok := orgUnitBaselineCapabilityKeyForIntentCapability(intentCapabilityKey); ok {
		parts.BaselineCapabilityKey = baselineCapabilityKey
		parts.BaselinePolicyVersion = strings.TrimSpace(defaultPolicyActivationRuntime.activePolicyVersion(tenantID, baselineCapabilityKey))
	}
	return buildOrgUnitEffectivePolicyVersion(parts), parts
}

func buildOrgUnitEffectivePolicyVersion(parts orgUnitEffectivePolicyVersionParts) string {
	canonical := orgUnitEffectivePolicyVersionParts{
		IntentCapabilityKey:   strings.TrimSpace(parts.IntentCapabilityKey),
		IntentPolicyVersion:   strings.TrimSpace(parts.IntentPolicyVersion),
		BaselineCapabilityKey: strings.TrimSpace(parts.BaselineCapabilityKey),
		BaselinePolicyVersion: strings.TrimSpace(parts.BaselinePolicyVersion),
	}
	raw, _ := json.Marshal(canonical)
	digest := sha256.Sum256(raw)
	return orgUnitEffectivePolicyVersionAlgorithm + ":" + hex.EncodeToString(digest[:])
}

func isOrgUnitPolicyVersionAccepted(
	requestVersion string,
	requestEffectiveVersion string,
	expectedEffectiveVersion string,
	parts orgUnitEffectivePolicyVersionParts,
) bool {
	requestVersion = strings.TrimSpace(requestVersion)
	requestEffectiveVersion = strings.TrimSpace(requestEffectiveVersion)
	if requestVersion == "" || requestEffectiveVersion == "" {
		return false
	}
	if requestVersion != strings.TrimSpace(parts.IntentPolicyVersion) {
		return false
	}
	return requestEffectiveVersion == strings.TrimSpace(expectedEffectiveVersion)
}
