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
	OrgUnitActionCreate        OrgUnitActionKind = "create"
	OrgUnitActionEventUpdate   OrgUnitActionKind = "event_update"
	OrgUnitActionCorrectEvent  OrgUnitActionKind = "correct_event"
	OrgUnitActionCorrectStatus OrgUnitActionKind = "correct_status"
	OrgUnitActionRescindEvent  OrgUnitActionKind = "rescind_event"
	OrgUnitActionRescindOrg    OrgUnitActionKind = "rescind_org"
)

type OrgUnitEmittedEventType string

const (
	OrgUnitEmittedCreate          OrgUnitEmittedEventType = "CREATE"
	OrgUnitEmittedMove            OrgUnitEmittedEventType = "MOVE"
	OrgUnitEmittedRename          OrgUnitEmittedEventType = "RENAME"
	OrgUnitEmittedDisable         OrgUnitEmittedEventType = "DISABLE"
	OrgUnitEmittedEnable          OrgUnitEmittedEventType = "ENABLE"
	OrgUnitEmittedSetBusinessUnit OrgUnitEmittedEventType = "SET_BUSINESS_UNIT"
	OrgUnitEmittedCorrectEvent    OrgUnitEmittedEventType = "CORRECT_EVENT"
	OrgUnitEmittedCorrectStatus   OrgUnitEmittedEventType = "CORRECT_STATUS"
	OrgUnitEmittedRescindEvent    OrgUnitEmittedEventType = "RESCIND_EVENT"
	OrgUnitEmittedRescindOrg      OrgUnitEmittedEventType = "RESCIND_ORG"
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

	// Append-only facts.
	TreeInitialized  bool
	TargetExistsAsOf bool
	TargetStatusAsOf string
	IsRoot           bool
	OrgAlreadyExists bool
	CreateAsRoot     bool
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
	case key.ActionKind == OrgUnitActionCreate && key.EmittedEventType == OrgUnitEmittedCreate && key.TargetEffectiveEventType == nil:
		core := []string{"effective_date", "is_business_unit", "manager_pernr", "name", "org_code", "parent_org_code"}
		ext := normalizeFieldKeys(facts.EnabledExtFieldKeys)
		allowed := mergeAndSortKeys(core, ext)
		deny := []string{}
		if !facts.CanAdmin {
			deny = append(deny, "FORBIDDEN")
		}
		if facts.OrgAlreadyExists {
			deny = append(deny, "ORG_ALREADY_EXISTS")
		}
		if facts.CreateAsRoot && facts.TreeInitialized {
			deny = append(deny, "ORG_ROOT_ALREADY_EXISTS")
		}
		if !facts.CreateAsRoot && !facts.TreeInitialized {
			deny = append(deny, "ORG_TREE_NOT_INITIALIZED")
		}

		deny = dedupAndSortDenyReasons(deny)
		enabled := len(deny) == 0
		if !enabled {
			allowed = []string{}
		}
		payloadKeys := map[string]string{}
		if enabled {
			payloadKeys = buildAppendFieldPayloadKeys(OrgUnitEmittedCreate, allowed, ext)
		}
		return OrgUnitMutationPolicyDecision{
			Enabled:          enabled,
			AllowedFields:    allowed,
			FieldPayloadKeys: payloadKeys,
			DenyReasons:      deny,
		}, nil

	case key.ActionKind == OrgUnitActionEventUpdate && key.TargetEffectiveEventType == nil:
		deny := []string{}
		if !facts.CanAdmin {
			deny = append(deny, "FORBIDDEN")
		}
		if !facts.TreeInitialized {
			deny = append(deny, "ORG_TREE_NOT_INITIALIZED")
		}
		if !facts.TargetExistsAsOf {
			deny = append(deny, "ORG_NOT_FOUND_AS_OF")
		}
		if key.EmittedEventType == OrgUnitEmittedMove && facts.IsRoot {
			deny = append(deny, "ORG_ROOT_CANNOT_BE_MOVED")
		}
		deny = dedupAndSortDenyReasons(deny)

		core, ok := allowedAppendCoreFieldsForEmittedEvent(key.EmittedEventType)
		if !ok {
			return OrgUnitMutationPolicyDecision{}, errors.New("orgunit mutation policy: invalid key")
		}
		ext := normalizeFieldKeys(facts.EnabledExtFieldKeys)
		allowed := mergeAndSortKeys(core, ext)
		enabled := len(deny) == 0
		if !enabled {
			allowed = []string{}
		}
		payloadKeys := map[string]string{}
		if enabled {
			payloadKeys = buildAppendFieldPayloadKeys(key.EmittedEventType, allowed, ext)
		}
		return OrgUnitMutationPolicyDecision{
			Enabled:          enabled,
			AllowedFields:    allowed,
			FieldPayloadKeys: payloadKeys,
			DenyReasons:      deny,
		}, nil

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
	if patch.EffectiveDate != nil {
		if _, err := validateDate(*patch.EffectiveDate); err != nil {
			return err
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

func allowedAppendCoreFieldsForEmittedEvent(emitted OrgUnitEmittedEventType) ([]string, bool) {
	switch strings.TrimSpace(string(emitted)) {
	case "CREATE":
		return []string{"effective_date", "is_business_unit", "manager_pernr", "name", "org_code", "parent_org_code"}, true
	case "RENAME":
		return []string{"effective_date", "name"}, true
	case "MOVE":
		return []string{"effective_date", "parent_org_code"}, true
	case "SET_BUSINESS_UNIT":
		return []string{"effective_date", "is_business_unit"}, true
	case "DISABLE", "ENABLE":
		return []string{"effective_date"}, true
	default:
		return nil, false
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

func buildAppendFieldPayloadKeys(emitted OrgUnitEmittedEventType, allowedFields []string, extFieldKeys []string) map[string]string {
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
		case "org_code":
			out[field] = "org_code"
		case "effective_date":
			out[field] = "effective_date"
		case "name":
			if emitted == OrgUnitEmittedRename {
				out[field] = "new_name"
			} else {
				out[field] = "name"
			}
		case "parent_org_code":
			if emitted == OrgUnitEmittedMove {
				out[field] = "new_parent_org_code"
			} else {
				out[field] = "parent_org_code"
			}
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
		if isReservedExtFieldKey(key) {
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

func isReservedExtFieldKey(fieldKey string) bool {
	switch strings.TrimSpace(fieldKey) {
	case "org_code", "effective_date", "name", "parent_org_code", "is_business_unit", "manager_pernr":
		return true
	case "ext", "ext_labels_snapshot":
		return true
	default:
		return false
	}
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
	case "ORG_TREE_NOT_INITIALIZED":
		return 20
	case "ORG_NOT_FOUND_AS_OF":
		return 30
	case "ORG_ROOT_CANNOT_BE_MOVED":
		return 40
	case "ORG_ALREADY_EXISTS":
		return 50
	case "ORG_ROOT_ALREADY_EXISTS":
		return 60
	case "ORG_EVENT_NOT_FOUND":
		return 70
	case "ORG_EVENT_RESCINDED":
		return 80
	case "ORG_ROOT_DELETE_FORBIDDEN":
		return 90
	case "ORG_HAS_CHILDREN_CANNOT_DELETE":
		return 91
	case "ORG_HAS_DEPENDENCIES_CANNOT_DELETE":
		return 92
	case "ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET":
		return 93
	default:
		return 100
	}
}
