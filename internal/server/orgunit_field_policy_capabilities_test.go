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

func TestOrgUnitFieldPolicyCapabilityBindingForWriteIntent(t *testing.T) {
	tests := []struct {
		name       string
		intent     string
		wantIntent string
	}{
		{name: "create", intent: "create_org", wantIntent: orgUnitCreateFieldPolicyCapabilityKey},
		{name: "add", intent: "add_version", wantIntent: orgUnitAddVersionFieldPolicyCapabilityKey},
		{name: "insert", intent: "insert_version", wantIntent: orgUnitInsertVersionFieldPolicyCapabilityKey},
		{name: "correct", intent: "correct", wantIntent: orgUnitCorrectFieldPolicyCapabilityKey},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binding, ok := orgUnitFieldPolicyCapabilityBindingForWriteIntent(tc.intent)
			if !ok {
				t.Fatal("expected mapped binding")
			}
			if binding.IntentCapabilityKey != tc.wantIntent {
				t.Fatalf("intent=%q want=%q", binding.IntentCapabilityKey, tc.wantIntent)
			}
			if binding.BaselineCapabilityKey != orgUnitWriteFieldPolicyCapabilityKey {
				t.Fatalf("baseline=%q", binding.BaselineCapabilityKey)
			}
		})
	}
	if _, ok := orgUnitFieldPolicyCapabilityBindingForWriteIntent("unknown"); ok {
		t.Fatal("expected unknown intent unmapped")
	}
}

func TestOrgUnitBaselineCapabilityKeyForIntentCapability(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		mapped     bool
	}{
		{name: "baseline", capability: orgUnitWriteFieldPolicyCapabilityKey, mapped: true},
		{name: "create", capability: orgUnitCreateFieldPolicyCapabilityKey, mapped: true},
		{name: "add", capability: orgUnitAddVersionFieldPolicyCapabilityKey, mapped: true},
		{name: "insert", capability: orgUnitInsertVersionFieldPolicyCapabilityKey, mapped: true},
		{name: "correct", capability: orgUnitCorrectFieldPolicyCapabilityKey, mapped: true},
		{name: "unknown", capability: "staffing.assignment_create.field_policy", mapped: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := orgUnitBaselineCapabilityKeyForIntentCapability(tc.capability)
			if ok != tc.mapped {
				t.Fatalf("mapped=%v want=%v", ok, tc.mapped)
			}
			if tc.mapped && got != orgUnitWriteFieldPolicyCapabilityKey {
				t.Fatalf("baseline=%q", got)
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
