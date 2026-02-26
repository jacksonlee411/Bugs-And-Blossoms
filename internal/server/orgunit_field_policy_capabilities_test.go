package server

import "testing"

func TestOrgUnitFieldPolicyCapabilityKeyForWriteIntent(t *testing.T) {
	tests := []struct {
		name       string
		intent     string
		wantKey    string
		wantMapped bool
	}{
		{name: "create", intent: "create_org", wantKey: orgUnitCreateFieldPolicyCapabilityKey, wantMapped: true},
		{name: "add", intent: "add_version", wantKey: orgUnitAddVersionFieldPolicyCapabilityKey, wantMapped: true},
		{name: "insert", intent: "insert_version", wantKey: orgUnitInsertVersionFieldPolicyCapabilityKey, wantMapped: true},
		{name: "correct", intent: "correct", wantKey: orgUnitCorrectFieldPolicyCapabilityKey, wantMapped: true},
		{name: "unknown", intent: "noop", wantMapped: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, mapped := orgUnitFieldPolicyCapabilityKeyForWriteIntent(tc.intent)
			if mapped != tc.wantMapped {
				t.Fatalf("mapped=%v want=%v", mapped, tc.wantMapped)
			}
			if gotKey != tc.wantKey {
				t.Fatalf("key=%q want=%q", gotKey, tc.wantKey)
			}
		})
	}
}

func TestOrgUnitFieldPolicyCapabilityKeyForScope(t *testing.T) {
	tests := []struct {
		name       string
		scopeType  string
		scopeKey   string
		wantKey    string
		wantMapped bool
	}{
		{name: "create dialog", scopeType: "FORM", scopeKey: "orgunit.create_dialog", wantKey: orgUnitCreateFieldPolicyCapabilityKey, wantMapped: true},
		{name: "add dialog", scopeType: "FORM", scopeKey: "orgunit.details.add_version_dialog", wantKey: orgUnitAddVersionFieldPolicyCapabilityKey, wantMapped: true},
		{name: "insert dialog", scopeType: "FORM", scopeKey: "orgunit.details.insert_version_dialog", wantKey: orgUnitInsertVersionFieldPolicyCapabilityKey, wantMapped: true},
		{name: "correct dialog", scopeType: "FORM", scopeKey: "orgunit.details.correct_dialog", wantKey: orgUnitCorrectFieldPolicyCapabilityKey, wantMapped: true},
		{name: "global not mapped", scopeType: "GLOBAL", scopeKey: "global", wantMapped: false},
		{name: "bad scope", scopeType: "FORM", scopeKey: "bad.scope", wantMapped: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, mapped := orgUnitFieldPolicyCapabilityKeyForScope(tc.scopeType, tc.scopeKey)
			if mapped != tc.wantMapped {
				t.Fatalf("mapped=%v want=%v", mapped, tc.wantMapped)
			}
			if gotKey != tc.wantKey {
				t.Fatalf("key=%q want=%q", gotKey, tc.wantKey)
			}
		})
	}
}
