package services

import (
	"errors"
	"sort"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
)

type OrgUnitActionKind string

const (
	OrgUnitActionCorrectEvent  OrgUnitActionKind = "correct_event"
	OrgUnitActionCorrectStatus OrgUnitActionKind = "correct_status"
	OrgUnitActionRescindEvent  OrgUnitActionKind = "rescind_event"
	OrgUnitActionRescindOrg    OrgUnitActionKind = "rescind_org"
)

type OrgUnitEmittedEventType string

const (
	OrgUnitEmittedCorrectEvent  OrgUnitEmittedEventType = "CORRECT_EVENT"
	OrgUnitEmittedCorrectStatus OrgUnitEmittedEventType = "CORRECT_STATUS"
	OrgUnitEmittedRescindEvent  OrgUnitEmittedEventType = "RESCIND_EVENT"
	OrgUnitEmittedRescindOrg    OrgUnitEmittedEventType = "RESCIND_ORG"
)

type OrgUnitMutationPolicyKey struct {
	ActionKind               OrgUnitActionKind
	EmittedEventType         OrgUnitEmittedEventType
	TargetEffectiveEventType *types.OrgUnitEventType
}

type OrgUnitMutationPolicyFacts struct {
	CanAdmin              bool
	EnabledExtFieldKeys   []string
	RescindOrgDenyReasons []string
}

type OrgUnitMutationPolicyDecision struct {
	Enabled               bool
	AllowedFields         []string
	FieldPayloadKeys      map[string]string
	AllowedTargetStatuses []string
	DenyReasons           []string
}

func ResolvePolicy(key OrgUnitMutationPolicyKey, facts OrgUnitMutationPolicyFacts) (OrgUnitMutationPolicyDecision, error) {
	switch {
	case key.ActionKind == OrgUnitActionCorrectEvent && key.EmittedEventType == OrgUnitEmittedCorrectEvent && key.TargetEffectiveEventType != nil:
		core := allowedCoreFieldsForTargetEvent(*key.TargetEffectiveEventType)
		ext := normalizeFieldKeys(facts.EnabledExtFieldKeys)
		allowed := mergeAndSortKeys(core, ext)
		deny := []string{}
		if !facts.CanAdmin {
			deny = append(deny, "FORBIDDEN")
		}
		return OrgUnitMutationPolicyDecision{
			Enabled:          facts.CanAdmin,
			AllowedFields:    allowed,
			FieldPayloadKeys: buildFieldPayloadKeys(allowed, ext),
			DenyReasons:      dedupAndSortDenyReasons(deny),
		}, nil

	case key.ActionKind == OrgUnitActionCorrectStatus && key.EmittedEventType == OrgUnitEmittedCorrectStatus && key.TargetEffectiveEventType != nil:
		deny := []string{}
		statusSupported := *key.TargetEffectiveEventType == types.OrgUnitEventEnable || *key.TargetEffectiveEventType == types.OrgUnitEventDisable
		allowedTargetStatuses := []string{}
		if statusSupported {
			allowedTargetStatuses = []string{"active", "disabled"}
		} else {
			deny = append(deny, "ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET")
		}
		if !facts.CanAdmin {
			deny = append(deny, "FORBIDDEN")
		}
		return OrgUnitMutationPolicyDecision{
			Enabled:               facts.CanAdmin && statusSupported,
			AllowedTargetStatuses: allowedTargetStatuses,
			DenyReasons:           dedupAndSortDenyReasons(deny),
		}, nil

	case key.ActionKind == OrgUnitActionRescindEvent && key.EmittedEventType == OrgUnitEmittedRescindEvent && key.TargetEffectiveEventType != nil:
		deny := []string{}
		if !facts.CanAdmin {
			deny = append(deny, "FORBIDDEN")
		}
		return OrgUnitMutationPolicyDecision{
			Enabled:     facts.CanAdmin,
			DenyReasons: dedupAndSortDenyReasons(deny),
		}, nil

	case key.ActionKind == OrgUnitActionRescindOrg && key.EmittedEventType == OrgUnitEmittedRescindOrg && key.TargetEffectiveEventType == nil:
		deny := append([]string(nil), facts.RescindOrgDenyReasons...)
		if !facts.CanAdmin {
			deny = append(deny, "FORBIDDEN")
		}
		deny = dedupAndSortDenyReasons(deny)
		return OrgUnitMutationPolicyDecision{
			Enabled:     facts.CanAdmin && len(deny) == 0,
			DenyReasons: deny,
		}, nil
	default:
		return OrgUnitMutationPolicyDecision{}, errors.New("orgunit mutation policy: invalid key")
	}
}

func AllowedFields(decision OrgUnitMutationPolicyDecision) []string {
	return append([]string(nil), decision.AllowedFields...)
}

func ValidatePatch(targetEffectiveDate string, decision OrgUnitMutationPolicyDecision, patch OrgUnitCorrectionPatch) error {
	// "Effective-date correction mode": if corrected effective_date changes, only allow effective_date.
	if patch.EffectiveDate != nil {
		corrected, err := validateDate(*patch.EffectiveDate)
		if err != nil {
			return err
		}
		if corrected != "" && strings.TrimSpace(targetEffectiveDate) != "" && corrected != strings.TrimSpace(targetEffectiveDate) {
			if patch.Name != nil || patch.ParentOrgCode != nil || patch.IsBusinessUnit != nil || patch.ManagerPernr != nil {
				return httperr.NewBadRequest(errPatchFieldNotAllowed)
			}
			if len(patch.Ext) > 0 {
				return httperr.NewBadRequest(errPatchFieldNotAllowed)
			}
		}
	}

	allowed := make(map[string]struct{}, len(decision.AllowedFields))
	for _, key := range decision.AllowedFields {
		allowed[key] = struct{}{}
	}

	checkAllowed := func(fieldKey string) error {
		if _, ok := allowed[fieldKey]; ok {
			return nil
		}
		return httperr.NewBadRequest(errPatchFieldNotAllowed)
	}

	if patch.EffectiveDate != nil {
		if err := checkAllowed("effective_date"); err != nil {
			return err
		}
	}
	if patch.Name != nil {
		if err := checkAllowed("name"); err != nil {
			return err
		}
	}
	if patch.ParentOrgCode != nil {
		if err := checkAllowed("parent_org_code"); err != nil {
			return err
		}
	}
	if patch.IsBusinessUnit != nil {
		if err := checkAllowed("is_business_unit"); err != nil {
			return err
		}
	}
	if patch.ManagerPernr != nil {
		if err := checkAllowed("manager_pernr"); err != nil {
			return err
		}
	}
	for key := range patch.Ext {
		fieldKey := strings.TrimSpace(key)
		if fieldKey == "" {
			return httperr.NewBadRequest(errPatchFieldNotAllowed)
		}
		if err := checkAllowed(fieldKey); err != nil {
			return err
		}
	}
	return nil
}

func allowedCoreFieldsForTargetEvent(eventType types.OrgUnitEventType) []string {
	switch strings.TrimSpace(string(eventType)) {
	case "CREATE":
		return []string{"effective_date", "is_business_unit", "manager_pernr", "name", "parent_org_code"}
	case "RENAME":
		return []string{"effective_date", "name"}
	case "MOVE":
		return []string{"effective_date", "parent_org_code"}
	case "SET_BUSINESS_UNIT":
		return []string{"effective_date", "is_business_unit"}
	case "DISABLE", "ENABLE":
		return []string{"effective_date"}
	default:
		return []string{"effective_date"}
	}
}

func buildFieldPayloadKeys(allowedFields []string, extFieldKeys []string) map[string]string {
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
		case "effective_date":
			out[field] = "effective_date"
		case "name":
			out[field] = "name"
		case "parent_org_code":
			out[field] = "parent_org_code"
		case "is_business_unit":
			out[field] = "is_business_unit"
		case "manager_pernr":
			out[field] = "manager_pernr"
		}
	}
	return out
}

func normalizeFieldKeys(keys []string) []string {
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
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func mergeAndSortKeys(a []string, b []string) []string {
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

func dedupAndSortDenyReasons(in []string) []string {
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
		return denyReasonPriority(out[i]) < denyReasonPriority(out[j])
	})
	return out
}

func denyReasonPriority(code string) int {
	switch code {
	case "FORBIDDEN":
		return 10
	case "ORG_EVENT_NOT_FOUND":
		return 20
	case "ORG_EVENT_RESCINDED":
		return 30
	case "ORG_ROOT_DELETE_FORBIDDEN":
		return 40
	case "ORG_HAS_CHILDREN_CANNOT_DELETE":
		return 50
	case "ORG_HAS_DEPENDENCIES_CANNOT_DELETE":
		return 60
	case "ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET":
		return 70
	default:
		return 100
	}
}
