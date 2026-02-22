package services

import (
	"errors"
	"sort"
	"strings"
)

type OrgUnitWriteCapabilitiesFacts struct {
	CanAdmin             bool
	TreeInitialized      bool
	OrgAlreadyExists     bool
	TargetExistsAsOf     bool
	TargetEventNotFound  bool
	TargetEventRescinded bool

	OrgCode             string
	EffectiveDate       string
	TargetEffectiveDate string
}

type OrgUnitWriteCapabilitiesDecision struct {
	Enabled          bool
	AllowedFields    []string
	FieldPayloadKeys map[string]string
	DenyReasons      []string
}

func ResolveWriteCapabilities(intent OrgUnitWriteIntent, enabledExtFieldKeys []string, facts OrgUnitWriteCapabilitiesFacts) (OrgUnitWriteCapabilitiesDecision, error) {
	switch intent {
	case OrgUnitWriteIntentCreateOrg, OrgUnitWriteIntentAddVersion, OrgUnitWriteIntentInsertVersion, OrgUnitWriteIntentCorrect:
	default:
		return OrgUnitWriteCapabilitiesDecision{}, errors.New("orgunit write capabilities: invalid intent")
	}

	core := []string{"is_business_unit", "manager_pernr", "name", "parent_org_code", "status"}
	ext := normalizeWriteExtFieldKeys(enabledExtFieldKeys)
	allowed := mergeAndSortWriteKeys(core, ext)

	deny := []string{}
	if !facts.CanAdmin {
		deny = append(deny, "FORBIDDEN")
	}

	orgCodeUpper := strings.ToUpper(strings.TrimSpace(facts.OrgCode))

	switch intent {
	case OrgUnitWriteIntentCreateOrg:
		if facts.OrgAlreadyExists {
			deny = append(deny, "ORG_ALREADY_EXISTS")
		}
		if orgCodeUpper == "ROOT" {
			if facts.TreeInitialized {
				deny = append(deny, "ORG_ROOT_ALREADY_EXISTS")
			}
		}

	case OrgUnitWriteIntentAddVersion, OrgUnitWriteIntentInsertVersion:
		if !facts.TreeInitialized {
			deny = append(deny, "ORG_TREE_NOT_INITIALIZED")
		}
		if !facts.TargetExistsAsOf {
			deny = append(deny, "ORG_NOT_FOUND_AS_OF")
		}

	case OrgUnitWriteIntentCorrect:
		if !facts.TreeInitialized {
			deny = append(deny, "ORG_TREE_NOT_INITIALIZED")
		}
		if !facts.TargetExistsAsOf {
			deny = append(deny, "ORG_NOT_FOUND_AS_OF")
		}
		if facts.TargetEventNotFound {
			deny = append(deny, "ORG_EVENT_NOT_FOUND")
		}
		if facts.TargetEventRescinded {
			deny = append(deny, "ORG_EVENT_RESCINDED")
		}
	}

	deny = dedupAndSortWriteDenyReasons(deny)
	enabled := len(deny) == 0
	if !enabled {
		return OrgUnitWriteCapabilitiesDecision{
			Enabled:          false,
			AllowedFields:    []string{},
			FieldPayloadKeys: map[string]string{},
			DenyReasons:      deny,
		}, nil
	}

	return OrgUnitWriteCapabilitiesDecision{
		Enabled:          true,
		AllowedFields:    allowed,
		FieldPayloadKeys: buildWriteFieldPayloadKeys(allowed, ext),
		DenyReasons:      []string{},
	}, nil
}

func buildWriteFieldPayloadKeys(allowedFields []string, extFieldKeys []string) map[string]string {
	out := make(map[string]string, len(allowedFields))
	extSet := make(map[string]struct{}, len(extFieldKeys))
	for _, key := range extFieldKeys {
		extSet[key] = struct{}{}
	}

	for _, field := range allowedFields {
		if _, ok := extSet[field]; ok {
			out[field] = "ext." + field
			continue
		}
		switch field {
		case "name":
			out[field] = "name"
		case "parent_org_code":
			// DEV-PLAN-108: capabilities maps UI field -> kernel payload key.
			out[field] = "parent_id"
		case "status":
			out[field] = "status"
		case "is_business_unit":
			out[field] = "is_business_unit"
		case "manager_pernr":
			out[field] = "manager_pernr"
		}
	}
	return out
}

func normalizeWriteExtFieldKeys(keys []string) []string {
	if len(keys) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, raw := range keys {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		if isReservedWriteExtFieldKey(key) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func isReservedWriteExtFieldKey(fieldKey string) bool {
	switch strings.TrimSpace(fieldKey) {
	case "name", "parent_org_code", "status", "is_business_unit", "manager_pernr":
		return true
	case "ext", "ext_labels_snapshot":
		return true
	default:
		return false
	}
}

func mergeAndSortWriteKeys(a []string, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	seen := make(map[string]struct{}, len(a)+len(b))
	for _, item := range a {
		key := strings.TrimSpace(item)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	for _, item := range b {
		key := strings.TrimSpace(item)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func dedupAndSortWriteDenyReasons(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		code := strings.TrimSpace(item)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return writeDenyReasonPriority(out[i]) < writeDenyReasonPriority(out[j])
	})
	return out
}

func writeDenyReasonPriority(code string) int {
	switch code {
	case "FORBIDDEN":
		return 10
	case "ORG_TREE_NOT_INITIALIZED":
		return 20
	case "ORG_NOT_FOUND_AS_OF":
		return 30
	case "ORG_ALREADY_EXISTS":
		return 40
	case "ORG_ROOT_ALREADY_EXISTS":
		return 50
	case "ORG_EVENT_NOT_FOUND":
		return 60
	case "ORG_EVENT_RESCINDED":
		return 70
	default:
		return 100
	}
}
