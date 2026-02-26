package server

import (
	"strings"
	"testing"
	"time"
)

func TestBuildOrgUnitEffectivePolicyVersionDeterministic(t *testing.T) {
	parts := orgUnitEffectivePolicyVersionParts{
		IntentCapabilityKey:   orgUnitCreateFieldPolicyCapabilityKey,
		IntentPolicyVersion:   "2026-02-23",
		BaselineCapabilityKey: orgUnitWriteFieldPolicyCapabilityKey,
		BaselinePolicyVersion: "2026-02-23",
	}
	v1 := buildOrgUnitEffectivePolicyVersion(parts)
	v2 := buildOrgUnitEffectivePolicyVersion(parts)
	if v1 != v2 {
		t.Fatalf("version mismatch v1=%q v2=%q", v1, v2)
	}
	if !strings.HasPrefix(v1, orgUnitEffectivePolicyVersionAlgorithm+":") {
		t.Fatalf("version=%q", v1)
	}
}

func TestIsOrgUnitPolicyVersionAccepted(t *testing.T) {
	parts := orgUnitEffectivePolicyVersionParts{
		IntentCapabilityKey:   orgUnitCreateFieldPolicyCapabilityKey,
		IntentPolicyVersion:   "2026-02-23",
		BaselineCapabilityKey: orgUnitWriteFieldPolicyCapabilityKey,
		BaselinePolicyVersion: "2026-03-01",
	}
	effective := buildOrgUnitEffectivePolicyVersion(parts)
	if !isOrgUnitPolicyVersionAccepted(effective, effective, parts, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("expected effective version accepted")
	}
	if isOrgUnitPolicyVersionAccepted("2026-02-23", effective, parts, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("expected old intent version rejected outside window")
	}

	parts.BaselinePolicyVersion = ""
	effective = buildOrgUnitEffectivePolicyVersion(parts)
	if !isOrgUnitPolicyVersionAccepted("2026-02-23", effective, parts, time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("expected old intent version accepted inside compatibility window when baseline missing")
	}
	if isOrgUnitPolicyVersionAccepted("2026-02-23", effective, parts, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("expected old intent version rejected after hard switch")
	}
}

func TestIsOrgUnitPolicyVersionAccepted_AdditionalBranches(t *testing.T) {
	parts := orgUnitEffectivePolicyVersionParts{
		IntentCapabilityKey:   orgUnitCreateFieldPolicyCapabilityKey,
		IntentPolicyVersion:   "2026-02-23",
		BaselineCapabilityKey: orgUnitWriteFieldPolicyCapabilityKey,
		BaselinePolicyVersion: "",
	}
	effective := buildOrgUnitEffectivePolicyVersion(parts)
	if isOrgUnitPolicyVersionAccepted("", effective, parts, time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("expected empty request_version rejected")
	}
	if isOrgUnitPolicyVersionAccepted("2026-01-01", effective, parts, time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("expected mismatched intent version rejected in compatibility window")
	}
}

func TestOrgUnitPolicyVersionCompatibilityWindow_UsesNowWhenZero(t *testing.T) {
	origNow := nowUTCForOrgUnitPolicyVersion
	t.Cleanup(func() {
		nowUTCForOrgUnitPolicyVersion = origNow
	})

	nowUTCForOrgUnitPolicyVersion = func() time.Time {
		return time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	}
	if !orgUnitPolicyVersionCompatibilityWindow(time.Time{}) {
		t.Fatal("expected zero-time fallback within compatibility window")
	}
}

func TestResolveOrgUnitEffectivePolicyVersion(t *testing.T) {
	resetPolicyActivationRuntimeForTest()
	version, parts := resolveOrgUnitEffectivePolicyVersion("tenant-a", orgUnitCreateFieldPolicyCapabilityKey)
	if version == "" {
		t.Fatal("expected effective version")
	}
	if parts.IntentCapabilityKey != orgUnitCreateFieldPolicyCapabilityKey {
		t.Fatalf("intent=%q", parts.IntentCapabilityKey)
	}
	if parts.BaselineCapabilityKey != orgUnitWriteFieldPolicyCapabilityKey {
		t.Fatalf("baseline=%q", parts.BaselineCapabilityKey)
	}
	if strings.TrimSpace(parts.IntentPolicyVersion) == "" || strings.TrimSpace(parts.BaselinePolicyVersion) == "" {
		t.Fatalf("parts=%+v", parts)
	}
}
