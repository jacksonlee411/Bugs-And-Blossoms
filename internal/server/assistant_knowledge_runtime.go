package server

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	assistantRouteKindBusinessAction = "business_action"
	assistantRouteKindKnowledgeQA    = "knowledge_qa"
	assistantRouteKindChitchat       = "chitchat"
	assistantRouteKindUncertain      = "uncertain"

	assistantResolverContractVersionV1 = "resolver_contract_v1"
	assistantContextTemplateVersionV1  = "plan_context_v1"
)

var assistantTemplateFieldWhitelist = map[string]struct{}{
	"action_view_pack.summary":                 {},
	"field_display_map":                        {},
	"missing_field_guidance":                   {},
	"contract_projection.required_fields_view": {},
	"contract_projection.action_spec_summary":  {},
	"conversation_snapshot.current_phase":      {},
}

var assistantKnowledgeForbiddenKeys = map[string]struct{}{
	"required_fields":    {},
	"phase":              {},
	"confirm_conditions": {},
	"confirm_condition":  {},
	"commit_conditions":  {},
	"commit_condition":   {},
}

var assistantKnowledgeKnownErrorCodes = map[string]struct{}{
	"missing_parent_ref_text":                       {},
	"missing_new_parent_ref_text":                   {},
	"parent_candidate_not_found":                    {},
	"missing_entity_name":                           {},
	"missing_new_name":                              {},
	"missing_org_code":                              {},
	"missing_effective_date":                        {},
	"invalid_effective_date_format":                 {},
	"missing_target_effective_date":                 {},
	"invalid_target_effective_date_format":          {},
	"missing_change_fields":                         {},
	"candidate_confirmation_required":               {},
	"FIELD_REQUIRED_VALUE_MISSING":                  {},
	"PATCH_FIELD_NOT_ALLOWED":                       {},
	"non_business_route":                            {},
	errAssistantUnsupportedIntent.Error():           {},
	errAssistantConfirmationRequired.Error():        {},
	errAssistantPlanContractVersionMismatch.Error(): {},
	errAssistantPlanDeterminismViolation.Error():    {},
}

//go:embed assistant_knowledge/*.json assistant_knowledge/*/*.json
var assistantKnowledgeFS embed.FS

var assistantKnowledgeReadFileFn = fs.ReadFile
var assistantKnowledgeGlobFn = fs.Glob
var assistantKnowledgeJSONUnmarshalFn = json.Unmarshal
var assistantKnowledgeRepoStatFn = os.Stat
var assistantKnowledgeRuntimeCallerFn = runtime.Caller
var assistantKnowledgeCanonicalHashFn = assistantCanonicalHash
var assistantLoadKnowledgeRuntimeFn = assistantLoadKnowledgeRuntime

type assistantKnowledgePrompt struct {
	TemplateID string `json:"template_id"`
	Text       string `json:"text"`
}

type assistantInterpretationPack struct {
	AssetType            string                     `json:"asset_type"`
	PackID               string                     `json:"pack_id"`
	KnowledgeVersion     string                     `json:"knowledge_version"`
	Locale               string                     `json:"locale"`
	IntentClasses        []string                   `json:"intent_classes"`
	ClarificationPrompts []assistantKnowledgePrompt `json:"clarification_prompts"`
	NegativeExamples     []string                   `json:"negative_examples"`
	SourceRefs           []string                   `json:"source_refs"`
}

type assistantActionViewField struct {
	Field string `json:"field"`
	Label string `json:"label"`
}

type assistantActionViewGuidance struct {
	ErrorCode string `json:"error_code"`
	Text      string `json:"text"`
}

type assistantActionViewExample struct {
	Field   string `json:"field"`
	Example string `json:"example"`
}

type assistantActionViewPack struct {
	AssetType                     string                        `json:"asset_type"`
	ActionID                      string                        `json:"action_id"`
	KnowledgeVersion              string                        `json:"knowledge_version"`
	Locale                        string                        `json:"locale"`
	Summary                       string                        `json:"summary"`
	FieldDisplayMap               []assistantActionViewField    `json:"field_display_map"`
	MissingFieldGuidance          []assistantActionViewGuidance `json:"missing_field_guidance"`
	FieldExamples                 []assistantActionViewExample  `json:"field_examples"`
	CandidateExplanationTemplates []string                      `json:"candidate_explanation_templates"`
	ConfirmationSummaryTemplates  []string                      `json:"confirmation_summary_templates"`
	TemplateFields                []string                      `json:"template_fields"`
	SourceRefs                    []string                      `json:"source_refs"`
}

type assistantReplyGuidancePack struct {
	AssetType         string                     `json:"asset_type"`
	ReplyKind         string                     `json:"reply_kind"`
	KnowledgeVersion  string                     `json:"knowledge_version"`
	Locale            string                     `json:"locale"`
	GuidanceTemplates []assistantKnowledgePrompt `json:"guidance_templates"`
	ToneConstraints   []string                   `json:"tone_constraints"`
	NegativeExamples  []string                   `json:"negative_examples"`
	ErrorCodes        []string                   `json:"error_codes"`
	SourceRefs        []string                   `json:"source_refs"`
}

type assistantIntentRouteEntry struct {
	IntentID                string   `json:"intent_id"`
	RouteKind               string   `json:"route_kind"`
	ActionID                string   `json:"action_id,omitempty"`
	RequiredSlots           []string `json:"required_slots"`
	MinConfidence           float64  `json:"min_confidence"`
	ClarificationTemplateID string   `json:"clarification_template_id"`
	Keywords                []string `json:"keywords,omitempty"`
}

type assistantIntentRouteCatalog struct {
	AssetType           string                      `json:"asset_type"`
	RouteCatalogVersion string                      `json:"route_catalog_version"`
	SourceRefs          []string                    `json:"source_refs"`
	Entries             []assistantIntentRouteEntry `json:"entries"`
}

type assistantKnowledgeRuntime struct {
	SnapshotDigest          string
	RouteCatalogVersion     string
	ReplyGuidanceVersion    string
	ResolverContractVersion string
	ContextTemplateVersion  string

	routeCatalog   assistantIntentRouteCatalog
	routeByAction  map[string]assistantIntentRouteEntry
	interpretation map[string]map[string]assistantInterpretationPack
	actionView     map[string]map[string]assistantActionViewPack
	replyGuidance  map[string]map[string]assistantReplyGuidancePack
}

type assistantConversationSnapshotResolverResult struct {
	CurrentPhase        string
	MissingFields       []string
	Candidates          []assistantCandidate
	SelectedCandidateID string
	ErrorCode           string
	TenantID            string
	RequestID           string
	TraceID             string
}

type assistantContractProjectionResolverResult struct {
	ActionID           string
	ActionSpecSummary  string
	RequiredFieldsView []string
}

type assistantPlanContextV1 struct {
	ActionViewSummary    string
	FieldDisplayMap      []assistantActionViewField
	MissingFieldGuidance []assistantActionViewGuidance
	ContractProjection   assistantContractProjectionResolverResult
	ConversationSnapshot assistantConversationSnapshotResolverResult
}

func assistantLoadKnowledgeRuntime() (*assistantKnowledgeRuntime, error) {
	catalogRaw, err := assistantKnowledgeReadFileFn(assistantKnowledgeFS, "assistant_knowledge/intent_route_catalog.json")
	if err != nil {
		return nil, fmt.Errorf("%w: load intent_route_catalog failed", errAssistantRuntimeConfigInvalid)
	}
	var catalog assistantIntentRouteCatalog
	if err := assistantKnowledgeJSONUnmarshalFn(catalogRaw, &catalog); err != nil {
		return nil, fmt.Errorf("%w: decode intent_route_catalog failed", errAssistantRuntimeConfigInvalid)
	}

	interpretation, interpretationRaw, err := assistantLoadInterpretationPacks()
	if err != nil {
		return nil, err
	}
	actionViews, actionViewRaw, err := assistantLoadActionViewPacks()
	if err != nil {
		return nil, err
	}
	replyGuidance, replyGuidanceRaw, err := assistantLoadReplyGuidancePacks()
	if err != nil {
		return nil, err
	}

	rawByPath := map[string][]byte{
		"assistant_knowledge/intent_route_catalog.json": catalogRaw,
	}
	for k, v := range interpretationRaw {
		rawByPath[k] = v
	}
	for k, v := range actionViewRaw {
		rawByPath[k] = v
	}
	for k, v := range replyGuidanceRaw {
		rawByPath[k] = v
	}

	runtime, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err != nil {
		return nil, err
	}
	return runtime, nil
}

func assistantLoadInterpretationPacks() ([]assistantInterpretationPack, map[string][]byte, error) {
	files, err := assistantKnowledgeGlobFn(assistantKnowledgeFS, "assistant_knowledge/interpretation/*.json")
	if err != nil {
		return nil, nil, fmt.Errorf("%w: list interpretation packs failed", errAssistantRuntimeConfigInvalid)
	}
	packs := make([]assistantInterpretationPack, 0, len(files))
	rawByPath := make(map[string][]byte, len(files))
	for _, file := range files {
		raw, err := assistantKnowledgeReadFileFn(assistantKnowledgeFS, file)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: read interpretation pack failed", errAssistantRuntimeConfigInvalid)
		}
		var pack assistantInterpretationPack
		if err := assistantKnowledgeJSONUnmarshalFn(raw, &pack); err != nil {
			return nil, nil, fmt.Errorf("%w: decode interpretation pack failed", errAssistantRuntimeConfigInvalid)
		}
		packs = append(packs, pack)
		rawByPath[file] = raw
	}
	return packs, rawByPath, nil
}

func assistantLoadActionViewPacks() ([]assistantActionViewPack, map[string][]byte, error) {
	files, err := assistantKnowledgeGlobFn(assistantKnowledgeFS, "assistant_knowledge/action_view/*.json")
	if err != nil {
		return nil, nil, fmt.Errorf("%w: list action view packs failed", errAssistantRuntimeConfigInvalid)
	}
	packs := make([]assistantActionViewPack, 0, len(files))
	rawByPath := make(map[string][]byte, len(files))
	for _, file := range files {
		raw, err := assistantKnowledgeReadFileFn(assistantKnowledgeFS, file)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: read action view pack failed", errAssistantRuntimeConfigInvalid)
		}
		var pack assistantActionViewPack
		if err := assistantKnowledgeJSONUnmarshalFn(raw, &pack); err != nil {
			return nil, nil, fmt.Errorf("%w: decode action view pack failed", errAssistantRuntimeConfigInvalid)
		}
		packs = append(packs, pack)
		rawByPath[file] = raw
	}
	return packs, rawByPath, nil
}

func assistantLoadReplyGuidancePacks() ([]assistantReplyGuidancePack, map[string][]byte, error) {
	files, err := assistantKnowledgeGlobFn(assistantKnowledgeFS, "assistant_knowledge/reply_guidance/*.json")
	if err != nil {
		return nil, nil, fmt.Errorf("%w: list reply guidance packs failed", errAssistantRuntimeConfigInvalid)
	}
	packs := make([]assistantReplyGuidancePack, 0, len(files))
	rawByPath := make(map[string][]byte, len(files))
	for _, file := range files {
		raw, err := assistantKnowledgeReadFileFn(assistantKnowledgeFS, file)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: read reply guidance pack failed", errAssistantRuntimeConfigInvalid)
		}
		var pack assistantReplyGuidancePack
		if err := assistantKnowledgeJSONUnmarshalFn(raw, &pack); err != nil {
			return nil, nil, fmt.Errorf("%w: decode reply guidance pack failed", errAssistantRuntimeConfigInvalid)
		}
		packs = append(packs, pack)
		rawByPath[file] = raw
	}
	return packs, rawByPath, nil
}

func assistantCompileKnowledgeRuntime(
	catalog assistantIntentRouteCatalog,
	interpretation []assistantInterpretationPack,
	actionViews []assistantActionViewPack,
	replyGuidance []assistantReplyGuidancePack,
	rawByPath map[string][]byte,
) (*assistantKnowledgeRuntime, error) {
	if strings.TrimSpace(catalog.AssetType) != "intent_route_catalog" {
		return nil, fmt.Errorf("%w: intent route catalog asset_type invalid", errAssistantRuntimeConfigInvalid)
	}
	if strings.TrimSpace(catalog.RouteCatalogVersion) == "" {
		return nil, fmt.Errorf("%w: route_catalog_version required", errAssistantRuntimeConfigInvalid)
	}
	if err := assistantValidateSourceRefs(catalog.SourceRefs); err != nil {
		return nil, fmt.Errorf("%w: route catalog source_refs invalid: %v", errAssistantRuntimeConfigInvalid, err)
	}
	if err := assistantValidateForbiddenKeys(rawByPath); err != nil {
		return nil, fmt.Errorf("%w: %v", errAssistantRuntimeConfigInvalid, err)
	}

	routeByIntentID := make(map[string]assistantIntentRouteEntry, len(catalog.Entries))
	routeByAction := make(map[string]assistantIntentRouteEntry)
	for _, entry := range catalog.Entries {
		if strings.TrimSpace(entry.IntentID) == "" {
			return nil, fmt.Errorf("%w: intent_id required", errAssistantRuntimeConfigInvalid)
		}
		intentID := strings.TrimSpace(entry.IntentID)
		if _, exists := routeByIntentID[intentID]; exists {
			return nil, fmt.Errorf("%w: duplicated intent_id %s", errAssistantRuntimeConfigInvalid, intentID)
		}
		if !assistantValidRouteKind(entry.RouteKind) {
			return nil, fmt.Errorf("%w: invalid route_kind %s", errAssistantRuntimeConfigInvalid, entry.RouteKind)
		}
		if strings.TrimSpace(entry.RouteKind) == assistantRouteKindBusinessAction {
			actionID := strings.TrimSpace(entry.ActionID)
			if actionID == "" {
				return nil, fmt.Errorf("%w: action_id required for business_action", errAssistantRuntimeConfigInvalid)
			}
			if _, ok := assistantLookupDefaultActionSpec(actionID); !ok {
				return nil, fmt.Errorf("%w: action_id not registered %s", errAssistantRuntimeConfigInvalid, actionID)
			}
			routeByAction[actionID] = entry
		}
		routeByIntentID[intentID] = entry
	}

	interpretationIndex := make(map[string]map[string]assistantInterpretationPack)
	for _, pack := range interpretation {
		if strings.TrimSpace(pack.AssetType) != "interpretation_pack" {
			return nil, fmt.Errorf("%w: interpretation asset_type invalid", errAssistantRuntimeConfigInvalid)
		}
		if strings.TrimSpace(pack.PackID) == "" {
			return nil, fmt.Errorf("%w: interpretation pack_id required", errAssistantRuntimeConfigInvalid)
		}
		if !assistantValidLocale(pack.Locale) {
			return nil, fmt.Errorf("%w: interpretation locale invalid %s", errAssistantRuntimeConfigInvalid, pack.Locale)
		}
		if len(pack.SourceRefs) == 0 {
			return nil, fmt.Errorf("%w: interpretation source_refs required", errAssistantRuntimeConfigInvalid)
		}
		if err := assistantValidateSourceRefs(pack.SourceRefs); err != nil {
			return nil, fmt.Errorf("%w: interpretation source_refs invalid: %v", errAssistantRuntimeConfigInvalid, err)
		}
		packID := strings.TrimSpace(pack.PackID)
		locale := strings.TrimSpace(pack.Locale)
		if _, ok := interpretationIndex[packID]; !ok {
			interpretationIndex[packID] = make(map[string]assistantInterpretationPack)
		}
		if _, duplicated := interpretationIndex[packID][locale]; duplicated {
			return nil, fmt.Errorf("%w: duplicated interpretation pack %s locale %s", errAssistantRuntimeConfigInvalid, packID, locale)
		}
		interpretationIndex[packID][locale] = pack
	}

	actionViewIndex := make(map[string]map[string]assistantActionViewPack)
	for _, pack := range actionViews {
		if strings.TrimSpace(pack.AssetType) != "action_view_pack" {
			return nil, fmt.Errorf("%w: action view asset_type invalid", errAssistantRuntimeConfigInvalid)
		}
		actionID := strings.TrimSpace(pack.ActionID)
		if actionID == "" {
			return nil, fmt.Errorf("%w: action view action_id required", errAssistantRuntimeConfigInvalid)
		}
		if _, ok := assistantLookupDefaultActionSpec(actionID); !ok {
			return nil, fmt.Errorf("%w: action view action_id not registered %s", errAssistantRuntimeConfigInvalid, actionID)
		}
		if !assistantValidLocale(pack.Locale) {
			return nil, fmt.Errorf("%w: action view locale invalid %s", errAssistantRuntimeConfigInvalid, pack.Locale)
		}
		if strings.TrimSpace(pack.Summary) == "" {
			return nil, fmt.Errorf("%w: action view summary required", errAssistantRuntimeConfigInvalid)
		}
		if len(pack.SourceRefs) == 0 {
			return nil, fmt.Errorf("%w: action view source_refs required", errAssistantRuntimeConfigInvalid)
		}
		if err := assistantValidateSourceRefs(pack.SourceRefs); err != nil {
			return nil, fmt.Errorf("%w: action view source_refs invalid: %v", errAssistantRuntimeConfigInvalid, err)
		}
		for _, field := range pack.TemplateFields {
			name := strings.TrimSpace(field)
			if name == "" {
				continue
			}
			if _, ok := assistantTemplateFieldWhitelist[name]; !ok {
				return nil, fmt.Errorf("%w: template field not allowed %s", errAssistantRuntimeConfigInvalid, name)
			}
		}
		for _, guidance := range pack.MissingFieldGuidance {
			code := strings.TrimSpace(guidance.ErrorCode)
			if code == "" {
				continue
			}
			if _, ok := assistantKnowledgeKnownErrorCodes[code]; !ok {
				return nil, fmt.Errorf("%w: unknown error_code %s", errAssistantRuntimeConfigInvalid, code)
			}
		}
		locale := strings.TrimSpace(pack.Locale)
		if _, ok := actionViewIndex[actionID]; !ok {
			actionViewIndex[actionID] = make(map[string]assistantActionViewPack)
		}
		if _, duplicated := actionViewIndex[actionID][locale]; duplicated {
			return nil, fmt.Errorf("%w: duplicated action view %s locale %s", errAssistantRuntimeConfigInvalid, actionID, locale)
		}
		actionViewIndex[actionID][locale] = pack
	}

	replyGuidanceIndex := make(map[string]map[string]assistantReplyGuidancePack)
	replyVersions := make([]string, 0, len(replyGuidance))
	for _, pack := range replyGuidance {
		if strings.TrimSpace(pack.AssetType) != "reply_guidance_pack" {
			return nil, fmt.Errorf("%w: reply guidance asset_type invalid", errAssistantRuntimeConfigInvalid)
		}
		replyKind := strings.TrimSpace(pack.ReplyKind)
		if replyKind == "" {
			return nil, fmt.Errorf("%w: reply guidance reply_kind required", errAssistantRuntimeConfigInvalid)
		}
		if !assistantValidLocale(pack.Locale) {
			return nil, fmt.Errorf("%w: reply guidance locale invalid %s", errAssistantRuntimeConfigInvalid, pack.Locale)
		}
		if len(pack.SourceRefs) == 0 {
			return nil, fmt.Errorf("%w: reply guidance source_refs required", errAssistantRuntimeConfigInvalid)
		}
		if err := assistantValidateSourceRefs(pack.SourceRefs); err != nil {
			return nil, fmt.Errorf("%w: reply guidance source_refs invalid: %v", errAssistantRuntimeConfigInvalid, err)
		}
		for _, code := range pack.ErrorCodes {
			errorCode := strings.TrimSpace(code)
			if errorCode == "" {
				continue
			}
			if _, ok := assistantKnowledgeKnownErrorCodes[errorCode]; !ok {
				return nil, fmt.Errorf("%w: unknown reply guidance error_code %s", errAssistantRuntimeConfigInvalid, errorCode)
			}
		}
		locale := strings.TrimSpace(pack.Locale)
		if _, ok := replyGuidanceIndex[replyKind]; !ok {
			replyGuidanceIndex[replyKind] = make(map[string]assistantReplyGuidancePack)
		}
		if _, duplicated := replyGuidanceIndex[replyKind][locale]; duplicated {
			return nil, fmt.Errorf("%w: duplicated reply guidance %s locale %s", errAssistantRuntimeConfigInvalid, replyKind, locale)
		}
		replyGuidanceIndex[replyKind][locale] = pack
		replyVersions = append(replyVersions, strings.TrimSpace(pack.KnowledgeVersion))
	}
	if _, ok := actionViewIndex[assistantIntentCreateOrgUnit]; !ok {
		return nil, fmt.Errorf("%w: missing create_orgunit action view pack", errAssistantRuntimeConfigInvalid)
	}
	if _, ok := interpretationIndex["knowledge.general_qa"]; !ok {
		return nil, fmt.Errorf("%w: missing knowledge.general_qa interpretation pack", errAssistantRuntimeConfigInvalid)
	}
	if len(replyGuidanceIndex) == 0 {
		return nil, fmt.Errorf("%w: reply guidance packs missing", errAssistantRuntimeConfigInvalid)
	}

	snapshotDigest := assistantKnowledgeCanonicalHashFn(map[string]any{
		"catalog":        catalog,
		"interpretation": interpretation,
		"action_view":    actionViews,
		"reply_guidance": replyGuidance,
	})
	if strings.TrimSpace(snapshotDigest) == "" {
		return nil, fmt.Errorf("%w: knowledge snapshot digest empty", errAssistantRuntimeConfigInvalid)
	}

	sort.Strings(replyVersions)
	replyGuidanceVersion := ""
	if len(replyVersions) > 0 {
		replyGuidanceVersion = strings.Join(replyVersions, "+")
	}

	return &assistantKnowledgeRuntime{
		SnapshotDigest:          snapshotDigest,
		RouteCatalogVersion:     strings.TrimSpace(catalog.RouteCatalogVersion),
		ReplyGuidanceVersion:    replyGuidanceVersion,
		ResolverContractVersion: assistantResolverContractVersionV1,
		ContextTemplateVersion:  assistantContextTemplateVersionV1,
		routeCatalog:            catalog,
		routeByAction:           routeByAction,
		interpretation:          interpretationIndex,
		actionView:              actionViewIndex,
		replyGuidance:           replyGuidanceIndex,
	}, nil
}

func assistantValidateForbiddenKeys(rawByPath map[string][]byte) error {
	for path, raw := range rawByPath {
		if len(raw) == 0 {
			continue
		}
		var obj any
		if err := assistantKnowledgeJSONUnmarshalFn(raw, &obj); err != nil {
			return fmt.Errorf("decode %s failed: %w", path, err)
		}
		if key, ok := assistantFindForbiddenKey(obj); ok {
			return fmt.Errorf("forbidden key %s in %s", key, path)
		}
	}
	return nil
}

func assistantFindForbiddenKey(value any) (string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, sub := range typed {
			normalized := strings.TrimSpace(key)
			if _, forbidden := assistantKnowledgeForbiddenKeys[normalized]; forbidden {
				return normalized, true
			}
			if child, found := assistantFindForbiddenKey(sub); found {
				return child, true
			}
		}
	case []any:
		for _, item := range typed {
			if child, found := assistantFindForbiddenKey(item); found {
				return child, true
			}
		}
	}
	return "", false
}

func assistantValidRouteKind(routeKind string) bool {
	switch strings.TrimSpace(routeKind) {
	case assistantRouteKindBusinessAction, assistantRouteKindKnowledgeQA, assistantRouteKindChitchat, assistantRouteKindUncertain:
		return true
	default:
		return false
	}
}

func assistantValidLocale(locale string) bool {
	switch strings.TrimSpace(locale) {
	case "zh", "en":
		return true
	default:
		return false
	}
}

func assistantValidateSourceRefs(refs []string) error {
	if len(refs) == 0 {
		return errors.New("source_refs empty")
	}
	for _, ref := range refs {
		path := strings.TrimSpace(ref)
		if path == "" {
			return errors.New("source_ref empty")
		}
		if !assistantRepoPathExists(path) {
			return fmt.Errorf("source_ref not found: %s", path)
		}
	}
	return nil
}

func assistantRepoPathExists(path string) bool {
	if path == "" {
		return false
	}
	if _, err := assistantKnowledgeRepoStatFn(path); err == nil {
		return true
	}
	_, currentFile, _, ok := assistantKnowledgeRuntimeCallerFn(0)
	if !ok {
		return false
	}
	candidate := filepath.Join(filepath.Dir(currentFile), "..", "..", path)
	_, err := assistantKnowledgeRepoStatFn(filepath.Clean(candidate))
	return err == nil
}

func (runtime *assistantKnowledgeRuntime) localeCandidates(locale string) []string {
	requested := strings.TrimSpace(locale)
	if requested == "" {
		requested = "zh"
	}
	candidates := []string{requested, "zh", "en"}
	out := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func (runtime *assistantKnowledgeRuntime) findActionView(actionID string, locale string) (assistantActionViewPack, bool) {
	if runtime == nil {
		return assistantActionViewPack{}, false
	}
	locales, ok := runtime.actionView[strings.TrimSpace(actionID)]
	if !ok {
		return assistantActionViewPack{}, false
	}
	for _, candidate := range runtime.localeCandidates(locale) {
		if pack, exists := locales[candidate]; exists {
			return pack, true
		}
	}
	return assistantActionViewPack{}, false
}

func (runtime *assistantKnowledgeRuntime) findInterpretation(packID string, locale string) (assistantInterpretationPack, bool) {
	if runtime == nil {
		return assistantInterpretationPack{}, false
	}
	locales, ok := runtime.interpretation[strings.TrimSpace(packID)]
	if !ok {
		return assistantInterpretationPack{}, false
	}
	for _, candidate := range runtime.localeCandidates(locale) {
		if pack, exists := locales[candidate]; exists {
			return pack, true
		}
	}
	return assistantInterpretationPack{}, false
}

func (runtime *assistantKnowledgeRuntime) routeIntent(userInput string, intent assistantIntentSpec) assistantIntentSpec {
	out := intent
	actionID := strings.TrimSpace(out.Action)
	if actionID != "" && actionID != assistantIntentPlanOnly {
		if entry, ok := runtime.routeByAction[actionID]; ok {
			out.IntentID = strings.TrimSpace(entry.IntentID)
			out.RouteKind = strings.TrimSpace(entry.RouteKind)
			out.RouteCatalogVersion = strings.TrimSpace(runtime.RouteCatalogVersion)
			return out
		}
		out.RouteKind = assistantRouteKindBusinessAction
		out.IntentID = "action." + actionID
		out.RouteCatalogVersion = strings.TrimSpace(runtime.RouteCatalogVersion)
		return out
	}
	text := strings.ToLower(strings.TrimSpace(userInput))
	matched := assistantIntentRouteEntry{}
	matchedLen := -1
	for _, entry := range runtime.routeCatalog.Entries {
		routeKind := strings.TrimSpace(entry.RouteKind)
		if routeKind == assistantRouteKindBusinessAction {
			continue
		}
		for _, keyword := range entry.Keywords {
			needle := strings.ToLower(strings.TrimSpace(keyword))
			if needle == "" {
				continue
			}
			if strings.Contains(text, needle) && len(needle) > matchedLen {
				matched = entry
				matchedLen = len(needle)
			}
		}
	}
	if matchedLen >= 0 {
		out.Action = assistantIntentPlanOnly
		out.IntentID = strings.TrimSpace(matched.IntentID)
		out.RouteKind = strings.TrimSpace(matched.RouteKind)
		out.RouteCatalogVersion = strings.TrimSpace(runtime.RouteCatalogVersion)
		return out
	}
	out.Action = assistantIntentPlanOnly
	out.IntentID = "route.uncertain"
	out.RouteKind = assistantRouteKindUncertain
	out.RouteCatalogVersion = strings.TrimSpace(runtime.RouteCatalogVersion)
	return out
}

func assistantConversationSnapshotResolver(tenantID string, turn *assistantTurn) assistantConversationSnapshotResolverResult {
	result := assistantConversationSnapshotResolverResult{TenantID: strings.TrimSpace(tenantID)}
	if turn == nil {
		return result
	}
	result.CurrentPhase = strings.TrimSpace(turn.Phase)
	result.MissingFields = append([]string(nil), assistantTurnMissingFields(turn)...)
	result.Candidates = append([]assistantCandidate(nil), turn.Candidates...)
	result.SelectedCandidateID = strings.TrimSpace(turn.SelectedCandidateID)
	if result.SelectedCandidateID == "" {
		result.SelectedCandidateID = strings.TrimSpace(turn.ResolvedCandidateID)
	}
	result.ErrorCode = strings.TrimSpace(turn.ErrorCode)
	result.RequestID = strings.TrimSpace(turn.RequestID)
	result.TraceID = strings.TrimSpace(turn.TraceID)
	return result
}

func assistantContractProjectionResolver(intent assistantIntentSpec, spec assistantActionSpec) assistantContractProjectionResolverResult {
	actionID := strings.TrimSpace(spec.ID)
	if actionID == "" {
		actionID = strings.TrimSpace(intent.Action)
	}
	summary := strings.TrimSpace(spec.PlanSummary)
	if summary == "" {
		summary = strings.TrimSpace(spec.PlanTitle)
	}
	return assistantContractProjectionResolverResult{
		ActionID:           actionID,
		ActionSpecSummary:  summary,
		RequiredFieldsView: assistantRequiredFieldsViewByAction(strings.TrimSpace(intent.Action)),
	}
}

func assistantRequiredFieldsViewByAction(action string) []string {
	switch strings.TrimSpace(action) {
	case assistantIntentCreateOrgUnit:
		return []string{"parent_ref_text", "entity_name", "effective_date"}
	case assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion:
		return []string{"org_code", "effective_date", "change_fields"}
	case assistantIntentCorrectOrgUnit:
		return []string{"org_code", "target_effective_date", "change_fields"}
	case assistantIntentRenameOrgUnit:
		return []string{"org_code", "effective_date", "new_name"}
	case assistantIntentMoveOrgUnit:
		return []string{"org_code", "effective_date", "new_parent_ref_text"}
	case assistantIntentDisableOrgUnit, assistantIntentEnableOrgUnit:
		return []string{"org_code", "effective_date"}
	default:
		return nil
	}
}

func (runtime *assistantKnowledgeRuntime) buildPlanContextV1(tenantID string, locale string, intent assistantIntentSpec, spec assistantActionSpec, turn *assistantTurn) (assistantPlanContextV1, error) {
	if runtime == nil {
		return assistantPlanContextV1{}, fmt.Errorf("%w: knowledge runtime missing", errAssistantRuntimeConfigInvalid)
	}
	context := assistantPlanContextV1{
		ContractProjection:   assistantContractProjectionResolver(intent, spec),
		ConversationSnapshot: assistantConversationSnapshotResolver(tenantID, turn),
	}
	if strings.TrimSpace(intent.RouteKind) == "" {
		intent.RouteKind = assistantRouteKindBusinessAction
	}
	if strings.TrimSpace(intent.RouteKind) != assistantRouteKindBusinessAction {
		packID := strings.TrimSpace(intent.IntentID)
		if packID == "" {
			packID = "knowledge.general_qa"
		}
		pack, ok := runtime.findInterpretation(packID, locale)
		if !ok {
			pack, ok = runtime.findInterpretation("knowledge.general_qa", locale)
		}
		if !ok {
			return assistantPlanContextV1{}, fmt.Errorf("%w: interpretation pack missing for %s", errAssistantRuntimeConfigInvalid, packID)
		}
		if len(pack.ClarificationPrompts) > 0 {
			context.ActionViewSummary = strings.TrimSpace(pack.ClarificationPrompts[0].Text)
		}
		if context.ActionViewSummary == "" {
			context.ActionViewSummary = "这是非动作请求，不会触发业务提交。"
		}
		return context, nil
	}
	pack, ok := runtime.findActionView(strings.TrimSpace(intent.Action), locale)
	if !ok {
		if strings.TrimSpace(intent.Action) == assistantIntentCreateOrgUnit {
			return assistantPlanContextV1{}, fmt.Errorf("%w: action view pack missing for %s", errAssistantRuntimeConfigInvalid, strings.TrimSpace(intent.Action))
		}
		context.ActionViewSummary = strings.TrimSpace(spec.PlanSummary)
		return context, nil
	}
	context.ActionViewSummary = strings.TrimSpace(pack.Summary)
	context.FieldDisplayMap = append([]assistantActionViewField(nil), pack.FieldDisplayMap...)
	context.MissingFieldGuidance = append([]assistantActionViewGuidance(nil), pack.MissingFieldGuidance...)
	return context, nil
}

func assistantKnowledgeGuidanceText(context assistantPlanContextV1, validationErrors []string) string {
	if len(validationErrors) == 0 {
		return ""
	}
	guidanceMap := make(map[string]string, len(context.MissingFieldGuidance))
	for _, item := range context.MissingFieldGuidance {
		code := strings.TrimSpace(item.ErrorCode)
		text := strings.TrimSpace(item.Text)
		if code == "" || text == "" {
			continue
		}
		guidanceMap[code] = text
	}
	for _, code := range validationErrors {
		if text, ok := guidanceMap[strings.TrimSpace(code)]; ok {
			return text
		}
	}
	return ""
}

func assistantApplyPlanContextV1(plan *assistantPlanSummary, dryRun *assistantDryRunResult, intent assistantIntentSpec, context assistantPlanContextV1) {
	if plan != nil {
		if summary := strings.TrimSpace(context.ActionViewSummary); summary != "" {
			plan.Summary = summary
		}
	}
	if dryRun == nil {
		return
	}
	validationErrors := assistantNormalizeValidationErrors(dryRun.ValidationErrors)
	if strings.TrimSpace(intent.RouteKind) != "" && strings.TrimSpace(intent.RouteKind) != assistantRouteKindBusinessAction {
		validationErrors = append(validationErrors, "non_business_route")
		validationErrors = assistantNormalizeValidationErrors(validationErrors)
		dryRun.ValidationErrors = validationErrors
		if summary := strings.TrimSpace(context.ActionViewSummary); summary != "" {
			dryRun.Explain = summary
		} else {
			dryRun.Explain = "这是非动作请求，不会触发业务提交。"
		}
		return
	}
	if text := strings.TrimSpace(assistantKnowledgeGuidanceText(context, validationErrors)); text != "" {
		dryRun.Explain = text
	}
}

func (runtime *assistantKnowledgeRuntime) planContextLocale() string {
	return "zh"
}

func (s *assistantConversationService) ensureKnowledgeRuntime() (*assistantKnowledgeRuntime, error) {
	if s == nil {
		return nil, errAssistantServiceMissing
	}
	runtime := s.knowledgeRuntime
	loadErr := s.knowledgeErr
	if runtime != nil {
		return runtime, nil
	}
	if loadErr != nil {
		return nil, loadErr
	}
	loaded, err := assistantLoadKnowledgeRuntimeFn()
	if err != nil {
		s.knowledgeErr = err
		return nil, err
	}
	s.knowledgeRuntime = loaded
	return loaded, nil
}
