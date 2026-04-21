package services

import (
	"slices"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
)

type orgUnitFieldDecision struct {
	FieldKey          string
	Required          bool
	Visible           bool
	Maintainable      bool
	DefaultRuleRef    string
	DefaultValue      string
	AllowedValueCodes []string
}

func resolveCreateOrgUnitStaticFieldDecision(enabledExtFieldKeys []string, fieldKey string) (orgUnitFieldDecision, bool, string) {
	switch fieldKey {
	case orgUnitCreateFieldOrgCode:
		return orgUnitFieldDecision{
			FieldKey:       orgUnitCreateFieldOrgCode,
			Required:       true,
			Visible:        true,
			Maintainable:   true,
			DefaultRuleRef: `next_org_code("O", 6)`,
		}, true, ""
	case orgUnitCreateFieldOrgType:
		if !slices.Contains(enabledExtFieldKeys, orgUnitCreateFieldOrgType) {
			return orgUnitFieldDecision{}, false, ""
		}
		return orgUnitFieldDecision{
			FieldKey:      orgUnitCreateFieldOrgType,
			Visible:       true,
			Maintainable:  true,
			Required:      false,
			DefaultValue:  "",
			DefaultRuleRef: "",
		}, true, ""
	default:
		return orgUnitFieldDecision{}, false, ""
	}
}

func resolveOrgUnitWriteFieldDecision(_ string) (orgUnitFieldDecision, bool, string) {
	return orgUnitFieldDecision{}, false, ""
}

func fieldConfigKeys(cfgs []types.TenantFieldConfig) []string {
	if len(cfgs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		key := strings.TrimSpace(cfg.FieldKey)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}
