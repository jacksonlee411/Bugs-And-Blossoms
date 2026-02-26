package server

import "strings"

const (
	orgUnitCreateFieldPolicyCapabilityKey        = "org.orgunit_create.field_policy"
	orgUnitAddVersionFieldPolicyCapabilityKey    = "org.orgunit_add_version.field_policy"
	orgUnitInsertVersionFieldPolicyCapabilityKey = "org.orgunit_insert_version.field_policy"
	orgUnitCorrectFieldPolicyCapabilityKey       = "org.orgunit_correct.field_policy"
)

func orgUnitFieldPolicyCapabilityKeyForWriteIntent(intent string) (string, bool) {
	switch strings.TrimSpace(intent) {
	case "create_org":
		return orgUnitCreateFieldPolicyCapabilityKey, true
	case "add_version":
		return orgUnitAddVersionFieldPolicyCapabilityKey, true
	case "insert_version":
		return orgUnitInsertVersionFieldPolicyCapabilityKey, true
	case "correct":
		return orgUnitCorrectFieldPolicyCapabilityKey, true
	default:
		return "", false
	}
}

func orgUnitFieldPolicyCapabilityKeyForScope(scopeType string, scopeKey string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(scopeType)) {
	case "FORM":
		switch strings.TrimSpace(scopeKey) {
		case "orgunit.create_dialog":
			return orgUnitCreateFieldPolicyCapabilityKey, true
		case "orgunit.details.add_version_dialog":
			return orgUnitAddVersionFieldPolicyCapabilityKey, true
		case "orgunit.details.insert_version_dialog":
			return orgUnitInsertVersionFieldPolicyCapabilityKey, true
		case "orgunit.details.correct_dialog":
			return orgUnitCorrectFieldPolicyCapabilityKey, true
		default:
			return "", false
		}
	default:
		return "", false
	}
}
