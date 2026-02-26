package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

const (
	orgUnitEffectivePolicyVersionAlgorithm = "epv1"
)

var nowUTCForOrgUnitPolicyVersion = func() time.Time {
	return time.Now().UTC()
}

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

func isOrgUnitPolicyVersionAccepted(requestVersion string, expectedEffectiveVersion string, parts orgUnitEffectivePolicyVersionParts, nowUTC time.Time) bool {
	requestVersion = strings.TrimSpace(requestVersion)
	if requestVersion == "" {
		return false
	}
	if requestVersion == strings.TrimSpace(expectedEffectiveVersion) {
		return true
	}
	if !orgUnitPolicyVersionCompatibilityWindow(nowUTC) {
		return false
	}
	if requestVersion != strings.TrimSpace(parts.IntentPolicyVersion) {
		return false
	}
	return strings.TrimSpace(parts.BaselinePolicyVersion) == ""
}

func orgUnitPolicyVersionCompatibilityWindow(nowUTC time.Time) bool {
	if nowUTC.IsZero() {
		nowUTC = nowUTCForOrgUnitPolicyVersion()
	}
	day := nowUTC.Format("2006-01-02")
	return day >= "2026-03-01" && day <= "2026-04-30"
}
