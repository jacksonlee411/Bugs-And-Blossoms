package server

import (
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/fieldpolicy"
)

const (
	orgUnitWriteFieldPolicyCapabilityKey         = fieldpolicy.OrgUnitWriteFieldPolicyCapabilityKey
	orgUnitCreateFieldPolicyCapabilityKey        = fieldpolicy.OrgUnitCreateFieldPolicyCapabilityKey
	orgUnitAddVersionFieldPolicyCapabilityKey    = fieldpolicy.OrgUnitAddVersionFieldPolicyCapabilityKey
	orgUnitInsertVersionFieldPolicyCapabilityKey = fieldpolicy.OrgUnitInsertVersionFieldPolicyCapabilityKey
	orgUnitCorrectFieldPolicyCapabilityKey       = fieldpolicy.OrgUnitCorrectFieldPolicyCapabilityKey
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
	return fieldpolicy.OrgUnitBaselineCapabilityKey(strings.ToLower(strings.TrimSpace(capabilityKey)))
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
