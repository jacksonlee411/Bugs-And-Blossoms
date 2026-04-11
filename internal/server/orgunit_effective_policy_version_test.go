package server

import (
	"strings"
	"testing"
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
	if !isOrgUnitPolicyVersionAccepted("2026-02-23", effective, effective, parts) {
		t.Fatal("expected effective version accepted")
	}
	if isOrgUnitPolicyVersionAccepted("2026-01-01", effective, effective, parts) {
		t.Fatal("expected stale intent version rejected")
	}
	if isOrgUnitPolicyVersionAccepted("2026-02-23", "", effective, parts) {
		t.Fatal("expected missing effective version rejected")
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
	if isOrgUnitPolicyVersionAccepted("", effective, effective, parts) {
		t.Fatal("expected empty request_version rejected")
	}
	if isOrgUnitPolicyVersionAccepted("2026-02-23", "epv1:other", effective, parts) {
		t.Fatal("expected mismatched effective version rejected")
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
