package server

import "strings"

const (
	orgUnitWriteFieldPolicyCapabilityKey         = "org.orgunit_write.field_policy"
	orgUnitCreateFieldPolicyCapabilityKey        = "org.orgunit_create.field_policy"
	orgUnitAddVersionFieldPolicyCapabilityKey    = "org.orgunit_add_version.field_policy"
	orgUnitInsertVersionFieldPolicyCapabilityKey = "org.orgunit_insert_version.field_policy"
	orgUnitCorrectFieldPolicyCapabilityKey       = "org.orgunit_correct.field_policy"
)

type orgUnitWriteCapabilityBinding struct {
	IntentCapabilityKey   string
	BaselineCapabilityKey string
}

func orgUnitFieldPolicyCapabilityKeyForWriteIntent(intent string) (string, bool) {
	binding, ok := orgUnitFieldPolicyCapabilityBindingForWriteIntent(intent)
	if !ok {
		return "", false
	}
	return binding.IntentCapabilityKey, true
}

func orgUnitFieldPolicyCapabilityBindingForWriteIntent(intent string) (orgUnitWriteCapabilityBinding, bool) {
	switch strings.TrimSpace(intent) {
	case "create_org":
		return orgUnitWriteCapabilityBinding{
			IntentCapabilityKey:   orgUnitCreateFieldPolicyCapabilityKey,
			BaselineCapabilityKey: orgUnitWriteFieldPolicyCapabilityKey,
		}, true
	case "add_version":
		return orgUnitWriteCapabilityBinding{
			IntentCapabilityKey:   orgUnitAddVersionFieldPolicyCapabilityKey,
			BaselineCapabilityKey: orgUnitWriteFieldPolicyCapabilityKey,
		}, true
	case "insert_version":
		return orgUnitWriteCapabilityBinding{
			IntentCapabilityKey:   orgUnitInsertVersionFieldPolicyCapabilityKey,
			BaselineCapabilityKey: orgUnitWriteFieldPolicyCapabilityKey,
		}, true
	case "correct":
		return orgUnitWriteCapabilityBinding{
			IntentCapabilityKey:   orgUnitCorrectFieldPolicyCapabilityKey,
			BaselineCapabilityKey: orgUnitWriteFieldPolicyCapabilityKey,
		}, true
	default:
		return orgUnitWriteCapabilityBinding{}, false
	}
}

func orgUnitBaselineCapabilityKeyForIntentCapability(capabilityKey string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(capabilityKey)) {
	case orgUnitCreateFieldPolicyCapabilityKey,
		orgUnitAddVersionFieldPolicyCapabilityKey,
		orgUnitInsertVersionFieldPolicyCapabilityKey,
		orgUnitCorrectFieldPolicyCapabilityKey:
		return orgUnitWriteFieldPolicyCapabilityKey, true
	case orgUnitWriteFieldPolicyCapabilityKey:
		return orgUnitWriteFieldPolicyCapabilityKey, true
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
