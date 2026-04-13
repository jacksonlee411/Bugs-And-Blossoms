package server

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	assistantRouteKindBusinessAction = "business_action"
	assistantRouteKindKnowledgeQA    = "knowledge_qa"
	assistantRouteKindChitchat       = "chitchat"
	assistantRouteKindUncertain      = "uncertain"

	assistantInterpretationDefaultPackID = "knowledge.general_qa"
	assistantRouteFallbackUncertainID    = "route.uncertain"

	assistantResolverContractVersionV1 = "resolver_contract_v1"
	assistantContextTemplateVersionV1  = "plan_context_v1"
)

var assistantTemplateFieldWhitelist = map[string]struct{}{
	"action_view_pack.summary":                 {},
	"field_display_map":                        {},
	"missing_field_guidance":                   {},
	"contract_projection.required_fields_view": {},
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

//go:embed assistant_knowledge_md/*/*.md
var assistantKnowledgeFS embed.FS

var assistantKnowledgeReadFileFn = fs.ReadFile
var assistantKnowledgeGlobFn = fs.Glob
var assistantKnowledgeJSONUnmarshalFn = yaml.Unmarshal
var assistantKnowledgeRepoStatFn = os.Stat
var assistantKnowledgeRuntimeCallerFn = runtime.Caller
var assistantKnowledgeCanonicalHashFn = assistantCanonicalHash
var assistantLoadKnowledgeRuntimeFn = assistantLoadKnowledgeRuntime

type assistantKnowledgePrompt struct {
	TemplateID string `json:"template_id" yaml:"template_id"`
	Text       string `json:"text" yaml:"text"`
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
	ErrorCode string `json:"error_code" yaml:"error_code"`
	Text      string `json:"text" yaml:"text"`
}

type assistantActionViewExample struct {
	Field   string `json:"field"`
	Example string `json:"example"`
}

func assistantOrderedBusinessActionIDs() []string {
	return []string{
		assistantIntentCreateOrgUnit,
		assistantIntentAddOrgUnitVersion,
		assistantIntentInsertOrgUnitVersion,
		assistantIntentCorrectOrgUnit,
		assistantIntentMoveOrgUnit,
		assistantIntentRenameOrgUnit,
		assistantIntentDisableOrgUnit,
		assistantIntentEnableOrgUnit,
	}
}

func assistantOrderedPromptActionIDs() []string {
	actions := assistantOrderedBusinessActionIDs()
	return append(actions, assistantIntentPlanOnly)
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

	routeCatalog             assistantIntentRouteCatalog
	routeByIntent            map[string]assistantIntentRouteEntry
	routeByAction            map[string]assistantIntentRouteEntry
	interpretation           map[string]map[string]assistantInterpretationPack
	interpretationTemplateID map[string]map[string]map[string]assistantKnowledgePrompt
	routePackID              map[string]string
	actionView               map[string]map[string]assistantActionViewPack
	replyGuidance            map[string]map[string][]assistantReplyGuidancePack
	intentDocs               map[string]map[string]assistantKnowledgeMarkdownDocument
	actionDocsByAction       map[string]map[string]assistantKnowledgeMarkdownDocument
	actionDocsByIntent       map[string]map[string]assistantKnowledgeMarkdownDocument
	toolDocs                 map[string]map[string]assistantKnowledgeMarkdownDocument
	wikiDocs                 map[string]map[string]assistantKnowledgeMarkdownDocument
}

type assistantCompiledInterpretationAssets struct {
	ByPack     map[string]map[string]assistantInterpretationPack
	ByTemplate map[string]map[string]map[string]assistantKnowledgePrompt
}

type assistantCompiledIntentRouteCatalog struct {
	Catalog         assistantIntentRouteCatalog
	ByIntent        map[string]assistantIntentRouteEntry
	ByAction        map[string]assistantIntentRouteEntry
	PackByIntentID  map[string]string
	TemplateByRoute map[string]string
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
	RequiredFieldsView []string
}

type assistantPlanContextV1 struct {
	ActionViewTitle      string
	ActionViewSummary    string
	FieldDisplayMap      []assistantActionViewField
	MissingFieldGuidance []assistantActionViewGuidance
	ContractProjection   assistantContractProjectionResolverResult
	ConversationSnapshot assistantConversationSnapshotResolverResult
}

func assistantLoadKnowledgeRuntime() (*assistantKnowledgeRuntime, error) {
	compilation, err := assistantLoadMarkdownKnowledgeCompilation()
	if err != nil {
		return nil, err
	}
	runtime, err := assistantCompileKnowledgeRuntime(
		compilation.Catalog,
		compilation.Interpretation,
		compilation.ActionViews,
		compilation.ReplyGuidance,
		compilation.RawByPath,
	)
	if err != nil {
		return nil, err
	}
	runtime.intentDocs = compilation.IntentDocs
	runtime.actionDocsByAction = compilation.ActionDocsByAction
	runtime.actionDocsByIntent = compilation.ActionDocsByIntent
	runtime.toolDocs = compilation.ToolDocs
	runtime.wikiDocs = compilation.WikiDocs
	return runtime, nil
}

func assistantCompileKnowledgeRuntime(
	catalog assistantIntentRouteCatalog,
	interpretation []assistantInterpretationPack,
	actionViews []assistantActionViewPack,
	replyGuidance []assistantReplyGuidancePack,
	rawByPath map[string][]byte,
) (*assistantKnowledgeRuntime, error) {
	if err := assistantValidateForbiddenKeys(rawByPath); err != nil {
		return nil, fmt.Errorf("%w: %v", errAssistantRuntimeConfigInvalid, err)
	}

	compiledInterpretation, err := assistantCompileInterpretationAssets(interpretation)
	if err != nil {
		return nil, err
	}
	compiledCatalog, err := assistantCompileIntentRouteCatalog(catalog, compiledInterpretation)
	if err != nil {
		return nil, err
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

	replyGuidanceIndex := make(map[string]map[string][]assistantReplyGuidancePack)
	replyVersions := make([]string, 0, len(replyGuidance))
	for _, pack := range replyGuidance {
		if strings.TrimSpace(pack.AssetType) != "reply_guidance_pack" {
			return nil, fmt.Errorf("%w: reply guidance asset_type invalid", errAssistantRuntimeConfigInvalid)
		}
		replyKind := strings.TrimSpace(pack.ReplyKind)
		if replyKind == "" {
			return nil, fmt.Errorf("%w: reply guidance reply_kind required", errAssistantRuntimeConfigInvalid)
		}
		knowledgeVersion := strings.TrimSpace(pack.KnowledgeVersion)
		if knowledgeVersion == "" {
			return nil, fmt.Errorf("%w: reply guidance knowledge_version required", errAssistantRuntimeConfigInvalid)
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
		guidanceTemplates, err := assistantNormalizeReplyGuidanceTemplates(replyKind, strings.TrimSpace(pack.Locale), pack.GuidanceTemplates)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errAssistantRuntimeConfigInvalid, err)
		}
		errorCodes, err := assistantNormalizeReplyGuidanceErrorCodes(pack.ErrorCodes)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errAssistantRuntimeConfigInvalid, err)
		}
		toneConstraints := assistantNormalizeOptionalTextList(pack.ToneConstraints)
		negativeExamples := assistantNormalizeOptionalTextList(pack.NegativeExamples)
		locale := strings.TrimSpace(pack.Locale)
		if _, ok := replyGuidanceIndex[replyKind]; !ok {
			replyGuidanceIndex[replyKind] = make(map[string][]assistantReplyGuidancePack)
		}
		normalized := pack
		normalized.ReplyKind = replyKind
		normalized.Locale = locale
		normalized.KnowledgeVersion = knowledgeVersion
		normalized.GuidanceTemplates = guidanceTemplates
		normalized.ErrorCodes = errorCodes
		normalized.ToneConstraints = toneConstraints
		normalized.NegativeExamples = negativeExamples
		replyGuidanceIndex[replyKind][locale] = append(replyGuidanceIndex[replyKind][locale], normalized)
		replyVersions = append(replyVersions, knowledgeVersion)
	}
	for replyKind, locales := range replyGuidanceIndex {
		for locale, packs := range locales {
			genericCount := 0
			seenErrorCodes := make(map[string]struct{})
			for _, pack := range packs {
				if len(pack.ErrorCodes) == 0 {
					genericCount++
					continue
				}
				for _, code := range pack.ErrorCodes {
					if _, duplicated := seenErrorCodes[code]; duplicated {
						return nil, fmt.Errorf("%w: ambiguous reply guidance selection %s locale %s error_code %s", errAssistantRuntimeConfigInvalid, replyKind, locale, code)
					}
					seenErrorCodes[code] = struct{}{}
				}
			}
			if genericCount > 1 {
				return nil, fmt.Errorf("%w: ambiguous reply guidance selection %s locale %s generic pack duplicated", errAssistantRuntimeConfigInvalid, replyKind, locale)
			}
		}
	}
	if _, ok := actionViewIndex[assistantIntentCreateOrgUnit]; !ok {
		return nil, fmt.Errorf("%w: missing create_orgunit action view pack", errAssistantRuntimeConfigInvalid)
	}
	if _, ok := compiledInterpretation.ByPack[assistantInterpretationDefaultPackID]; !ok {
		return nil, fmt.Errorf("%w: missing knowledge.general_qa interpretation pack", errAssistantRuntimeConfigInvalid)
	}
	if len(replyGuidanceIndex) == 0 {
		return nil, fmt.Errorf("%w: reply guidance packs missing", errAssistantRuntimeConfigInvalid)
	}

	sort.Strings(replyVersions)
	replyGuidanceVersion := ""
	if len(replyVersions) > 0 {
		replyGuidanceVersion = strings.Join(replyVersions, "+")
	}

	snapshotDigest := assistantKnowledgeCanonicalHashFn(map[string]any{
		"route_catalog_version":     strings.TrimSpace(compiledCatalog.Catalog.RouteCatalogVersion),
		"resolver_contract_version": assistantResolverContractVersionV1,
		"context_template_version":  assistantContextTemplateVersionV1,
		"reply_guidance_version":    replyGuidanceVersion,
		"catalog":                   compiledCatalog.Catalog,
		"interpretation":            assistantSortedInterpretationPacks(compiledInterpretation.ByPack),
		"action_view":               actionViews,
		"reply_guidance":            replyGuidance,
	})
	if strings.TrimSpace(snapshotDigest) == "" {
		return nil, fmt.Errorf("%w: knowledge snapshot digest empty", errAssistantRuntimeConfigInvalid)
	}

	return &assistantKnowledgeRuntime{
		SnapshotDigest:           snapshotDigest,
		RouteCatalogVersion:      strings.TrimSpace(compiledCatalog.Catalog.RouteCatalogVersion),
		ReplyGuidanceVersion:     replyGuidanceVersion,
		ResolverContractVersion:  assistantResolverContractVersionV1,
		ContextTemplateVersion:   assistantContextTemplateVersionV1,
		routeCatalog:             compiledCatalog.Catalog,
		routeByIntent:            compiledCatalog.ByIntent,
		routeByAction:            compiledCatalog.ByAction,
		interpretation:           compiledInterpretation.ByPack,
		interpretationTemplateID: compiledInterpretation.ByTemplate,
		routePackID:              compiledCatalog.PackByIntentID,
		actionView:               actionViewIndex,
		replyGuidance:            replyGuidanceIndex,
	}, nil
}

func assistantCompileInterpretationAssets(interpretation []assistantInterpretationPack) (assistantCompiledInterpretationAssets, error) {
	byPack := make(map[string]map[string]assistantInterpretationPack)
	byTemplate := make(map[string]map[string]map[string]assistantKnowledgePrompt)
	for _, rawPack := range interpretation {
		if strings.TrimSpace(rawPack.AssetType) != "interpretation_pack" {
			return assistantCompiledInterpretationAssets{}, fmt.Errorf("%w: interpretation asset_type invalid", errAssistantRuntimeConfigInvalid)
		}
		packID := strings.TrimSpace(rawPack.PackID)
		if packID == "" {
			return assistantCompiledInterpretationAssets{}, fmt.Errorf("%w: interpretation pack_id required", errAssistantRuntimeConfigInvalid)
		}
		knowledgeVersion := strings.TrimSpace(rawPack.KnowledgeVersion)
		if knowledgeVersion == "" {
			return assistantCompiledInterpretationAssets{}, fmt.Errorf("%w: interpretation knowledge_version required", errAssistantRuntimeConfigInvalid)
		}
		locale := strings.TrimSpace(rawPack.Locale)
		if !assistantValidLocale(locale) {
			return assistantCompiledInterpretationAssets{}, fmt.Errorf("%w: interpretation locale invalid %s", errAssistantRuntimeConfigInvalid, rawPack.Locale)
		}
		if len(rawPack.SourceRefs) == 0 {
			return assistantCompiledInterpretationAssets{}, fmt.Errorf("%w: interpretation source_refs required", errAssistantRuntimeConfigInvalid)
		}
		if err := assistantValidateSourceRefs(rawPack.SourceRefs); err != nil {
			return assistantCompiledInterpretationAssets{}, fmt.Errorf("%w: interpretation source_refs invalid: %v", errAssistantRuntimeConfigInvalid, err)
		}

		intentClasses, err := assistantNormalizeInterpretationIntentClasses(rawPack.IntentClasses)
		if err != nil {
			return assistantCompiledInterpretationAssets{}, fmt.Errorf("%w: %v", errAssistantRuntimeConfigInvalid, err)
		}
		prompts, templateLookup, err := assistantNormalizeInterpretationPrompts(packID, locale, rawPack.ClarificationPrompts)
		if err != nil {
			return assistantCompiledInterpretationAssets{}, fmt.Errorf("%w: %v", errAssistantRuntimeConfigInvalid, err)
		}
		negativeExamples, err := assistantNormalizeNegativeExamples(rawPack.NegativeExamples)
		if err != nil {
			return assistantCompiledInterpretationAssets{}, fmt.Errorf("%w: %v", errAssistantRuntimeConfigInvalid, err)
		}

		if _, ok := byPack[packID]; !ok {
			byPack[packID] = make(map[string]assistantInterpretationPack)
		}
		if _, duplicated := byPack[packID][locale]; duplicated {
			return assistantCompiledInterpretationAssets{}, fmt.Errorf("%w: duplicated interpretation pack %s locale %s", errAssistantRuntimeConfigInvalid, packID, locale)
		}
		if _, ok := byTemplate[packID]; !ok {
			byTemplate[packID] = make(map[string]map[string]assistantKnowledgePrompt)
		}
		byTemplate[packID][locale] = templateLookup

		normalized := rawPack
		normalized.PackID = packID
		normalized.Locale = locale
		normalized.KnowledgeVersion = knowledgeVersion
		normalized.IntentClasses = intentClasses
		normalized.ClarificationPrompts = prompts
		normalized.NegativeExamples = negativeExamples
		byPack[packID][locale] = normalized
	}
	return assistantCompiledInterpretationAssets{
		ByPack:     byPack,
		ByTemplate: byTemplate,
	}, nil
}

func assistantCompileIntentRouteCatalog(
	catalog assistantIntentRouteCatalog,
	interpretation assistantCompiledInterpretationAssets,
) (assistantCompiledIntentRouteCatalog, error) {
	if strings.TrimSpace(catalog.AssetType) != "intent_route_catalog" {
		return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: intent route catalog asset_type invalid", errAssistantRuntimeConfigInvalid)
	}
	routeCatalogVersion := strings.TrimSpace(catalog.RouteCatalogVersion)
	if routeCatalogVersion == "" {
		return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: route_catalog_version required", errAssistantRuntimeConfigInvalid)
	}
	if err := assistantValidateSourceRefs(catalog.SourceRefs); err != nil {
		return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: route catalog source_refs invalid: %v", errAssistantRuntimeConfigInvalid, err)
	}

	byIntent := make(map[string]assistantIntentRouteEntry, len(catalog.Entries))
	byAction := make(map[string]assistantIntentRouteEntry)
	packByIntentID := make(map[string]string, len(catalog.Entries))
	templateByRoute := make(map[string]string, len(catalog.Entries))
	normalizedEntries := make([]assistantIntentRouteEntry, 0, len(catalog.Entries))
	for _, rawEntry := range catalog.Entries {
		intentID := strings.TrimSpace(rawEntry.IntentID)
		if intentID == "" {
			return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: intent_id required", errAssistantRuntimeConfigInvalid)
		}
		if _, exists := byIntent[intentID]; exists {
			return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: duplicated intent_id %s", errAssistantRuntimeConfigInvalid, intentID)
		}

		routeKind := strings.TrimSpace(rawEntry.RouteKind)
		if !assistantValidRouteKind(routeKind) {
			return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: invalid route_kind %s", errAssistantRuntimeConfigInvalid, rawEntry.RouteKind)
		}
		if rawEntry.MinConfidence < 0 || rawEntry.MinConfidence > 1 {
			return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: min_confidence out of range for intent %s", errAssistantRuntimeConfigInvalid, intentID)
		}

		requiredSlots, err := assistantNormalizeRequiredSlots(rawEntry.RequiredSlots)
		if err != nil {
			return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: invalid required_slots for intent %s: %v", errAssistantRuntimeConfigInvalid, intentID, err)
		}

		actionID := strings.TrimSpace(rawEntry.ActionID)
		switch routeKind {
		case assistantRouteKindBusinessAction:
			if actionID == "" {
				return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: action_id required for business_action", errAssistantRuntimeConfigInvalid)
			}
			if _, ok := assistantLookupDefaultActionSpec(actionID); !ok {
				return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: action_id not registered %s", errAssistantRuntimeConfigInvalid, actionID)
			}
			allowedSlots := assistantAllowedRequiredSlotsByAction(actionID)
			for _, slot := range requiredSlots {
				if _, ok := allowedSlots[slot]; !ok {
					return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: invalid required_slot %s for action %s", errAssistantRuntimeConfigInvalid, slot, actionID)
				}
			}
			if _, duplicated := byAction[actionID]; duplicated {
				return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: duplicated route action_id %s", errAssistantRuntimeConfigInvalid, actionID)
			}
		default:
			if actionID != "" {
				return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: action_id must be empty for non-business route %s", errAssistantRuntimeConfigInvalid, intentID)
			}
		}

		packID := assistantResolveInterpretationPackIDForIntent(intentID, routeKind, interpretation.ByPack)
		if routeKind != assistantRouteKindBusinessAction && packID == "" {
			return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: missing interpretation pack for non-business intent %s", errAssistantRuntimeConfigInvalid, intentID)
		}
		if packID != "" {
			if !assistantInterpretationSupportsRouteKind(interpretation.ByPack[packID], routeKind) {
				return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: interpretation intent_classes mismatch for intent %s route_kind %s", errAssistantRuntimeConfigInvalid, intentID, routeKind)
			}
			packByIntentID[intentID] = packID
		}

		clarificationTemplateID := strings.TrimSpace(rawEntry.ClarificationTemplateID)
		if clarificationTemplateID != "" {
			if packID == "" || !assistantInterpretationTemplateExists(interpretation.ByTemplate, packID, clarificationTemplateID) {
				return assistantCompiledIntentRouteCatalog{}, fmt.Errorf("%w: unknown clarification_template_id %s for intent %s", errAssistantRuntimeConfigInvalid, clarificationTemplateID, intentID)
			}
			templateByRoute[intentID] = clarificationTemplateID
		}

		normalizedEntry := rawEntry
		normalizedEntry.IntentID = intentID
		normalizedEntry.RouteKind = routeKind
		normalizedEntry.ActionID = actionID
		normalizedEntry.RequiredSlots = requiredSlots
		normalizedEntry.ClarificationTemplateID = clarificationTemplateID
		byIntent[intentID] = normalizedEntry
		if routeKind == assistantRouteKindBusinessAction {
			byAction[actionID] = normalizedEntry
		}
		normalizedEntries = append(normalizedEntries, normalizedEntry)
	}

	catalogNormalized := catalog
	catalogNormalized.RouteCatalogVersion = routeCatalogVersion
	catalogNormalized.Entries = normalizedEntries
	return assistantCompiledIntentRouteCatalog{
		Catalog:         catalogNormalized,
		ByIntent:        byIntent,
		ByAction:        byAction,
		PackByIntentID:  packByIntentID,
		TemplateByRoute: templateByRoute,
	}, nil
}

func assistantNormalizeInterpretationIntentClasses(classes []string) ([]string, error) {
	if len(classes) == 0 {
		return nil, errors.New("interpretation intent_classes required")
	}
	seen := make(map[string]struct{}, len(classes))
	out := make([]string, 0, len(classes))
	for _, item := range classes {
		intentClass := strings.TrimSpace(item)
		if intentClass == "" {
			return nil, errors.New("interpretation intent_classes contains empty value")
		}
		if !assistantValidRouteKind(intentClass) {
			return nil, fmt.Errorf("invalid intent_class %s", intentClass)
		}
		if _, exists := seen[intentClass]; exists {
			continue
		}
		seen[intentClass] = struct{}{}
		out = append(out, intentClass)
	}
	return out, nil
}

func assistantNormalizeInterpretationPrompts(
	packID string,
	locale string,
	prompts []assistantKnowledgePrompt,
) ([]assistantKnowledgePrompt, map[string]assistantKnowledgePrompt, error) {
	lookup := make(map[string]assistantKnowledgePrompt, len(prompts))
	out := make([]assistantKnowledgePrompt, 0, len(prompts))
	for _, rawPrompt := range prompts {
		templateID := strings.TrimSpace(rawPrompt.TemplateID)
		if templateID == "" {
			return nil, nil, errors.New("interpretation template_id required")
		}
		text := strings.TrimSpace(rawPrompt.Text)
		if text == "" {
			return nil, nil, fmt.Errorf("interpretation template text required for %s", templateID)
		}
		if _, duplicated := lookup[templateID]; duplicated {
			return nil, nil, fmt.Errorf("duplicated interpretation template_id %s in pack %s locale %s", templateID, packID, locale)
		}
		prompt := assistantKnowledgePrompt{TemplateID: templateID, Text: text}
		lookup[templateID] = prompt
		out = append(out, prompt)
	}
	return out, lookup, nil
}

func assistantNormalizeNegativeExamples(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		example := strings.TrimSpace(value)
		if example == "" {
			return nil, errors.New("interpretation negative_examples contains empty value")
		}
		if _, exists := seen[example]; exists {
			continue
		}
		seen[example] = struct{}{}
		out = append(out, example)
	}
	return out, nil
}

func assistantNormalizeRequiredSlots(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		slot := strings.TrimSpace(value)
		if slot == "" {
			return nil, errors.New("required_slot empty")
		}
		if _, exists := seen[slot]; exists {
			continue
		}
		seen[slot] = struct{}{}
		out = append(out, slot)
	}
	return out, nil
}

func assistantAllowedRequiredSlotsByAction(actionID string) map[string]struct{} {
	fields := assistantRequiredFieldsViewByAction(actionID)
	out := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		out[strings.TrimSpace(field)] = struct{}{}
	}
	return out
}

func assistantResolveInterpretationPackIDForIntent(intentID string, routeKind string, byPack map[string]map[string]assistantInterpretationPack) string {
	if _, ok := byPack[intentID]; ok {
		return intentID
	}
	switch routeKind {
	case assistantRouteKindKnowledgeQA, assistantRouteKindChitchat, assistantRouteKindUncertain:
		if _, ok := byPack[assistantInterpretationDefaultPackID]; ok {
			return assistantInterpretationDefaultPackID
		}
	}
	return ""
}

func assistantInterpretationSupportsRouteKind(locales map[string]assistantInterpretationPack, routeKind string) bool {
	if len(locales) == 0 {
		return false
	}
	for _, pack := range locales {
		for _, intentClass := range pack.IntentClasses {
			if strings.TrimSpace(intentClass) == strings.TrimSpace(routeKind) {
				return true
			}
		}
	}
	return false
}

func assistantInterpretationTemplateExists(
	byTemplate map[string]map[string]map[string]assistantKnowledgePrompt,
	packID string,
	templateID string,
) bool {
	locales, ok := byTemplate[packID]
	if !ok {
		return false
	}
	for _, templates := range locales {
		if _, exists := templates[templateID]; exists {
			return true
		}
	}
	return false
}

func assistantSortedInterpretationPacks(byPack map[string]map[string]assistantInterpretationPack) []assistantInterpretationPack {
	if len(byPack) == 0 {
		return nil
	}
	packIDs := make([]string, 0, len(byPack))
	for packID := range byPack {
		packIDs = append(packIDs, packID)
	}
	sort.Strings(packIDs)
	out := make([]assistantInterpretationPack, 0, len(byPack)*2)
	for _, packID := range packIDs {
		locales := byPack[packID]
		localeKeys := make([]string, 0, len(locales))
		for locale := range locales {
			localeKeys = append(localeKeys, locale)
		}
		sort.Strings(localeKeys)
		for _, locale := range localeKeys {
			out = append(out, locales[locale])
		}
	}
	return out
}

func assistantValidateForbiddenKeys(rawByPath map[string][]byte) error {
	for path, raw := range rawByPath {
		if len(raw) == 0 {
			continue
		}
		if strings.HasSuffix(strings.TrimSpace(path), ".md") {
			frontMatter, _, err := assistantSplitMarkdownFrontMatter(raw)
			if err != nil {
				return fmt.Errorf("decode %s failed: %w", path, err)
			}
			raw = frontMatter
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
		if strings.HasPrefix(path, "docs/archive/") {
			return fmt.Errorf("source_ref must not target archive: %s", path)
		}
		if !assistantRepoPathExists(path) {
			return fmt.Errorf("source_ref not found: %s", path)
		}
	}
	return nil
}

func assistantNormalizeReplyGuidanceTemplates(replyKind string, locale string, templates []assistantKnowledgePrompt) ([]assistantKnowledgePrompt, error) {
	if len(templates) == 0 {
		return nil, fmt.Errorf("reply guidance templates required for %s locale %s", replyKind, locale)
	}
	if len(templates) > 1 {
		return nil, fmt.Errorf("reply guidance multiple templates not allowed for %s locale %s", replyKind, locale)
	}
	out := make([]assistantKnowledgePrompt, 0, len(templates))
	for _, rawTemplate := range templates {
		templateID := strings.TrimSpace(rawTemplate.TemplateID)
		if templateID == "" {
			return nil, fmt.Errorf("reply guidance template_id required for %s locale %s", replyKind, locale)
		}
		text := strings.TrimSpace(rawTemplate.Text)
		if text == "" {
			return nil, fmt.Errorf("reply guidance template text required for %s locale %s", replyKind, locale)
		}
		out = append(out, assistantKnowledgePrompt{
			TemplateID: templateID,
			Text:       text,
		})
	}
	return out, nil
}

func assistantNormalizeReplyGuidanceErrorCodes(codes []string) ([]string, error) {
	out := make([]string, 0, len(codes))
	seen := make(map[string]struct{}, len(codes))
	for _, rawCode := range codes {
		errorCode := strings.TrimSpace(rawCode)
		if errorCode == "" {
			continue
		}
		if _, ok := assistantKnowledgeKnownErrorCodes[errorCode]; !ok {
			return nil, fmt.Errorf("unknown reply guidance error_code %s", errorCode)
		}
		if _, duplicated := seen[errorCode]; duplicated {
			return nil, fmt.Errorf("duplicated reply guidance error_code %s", errorCode)
		}
		seen[errorCode] = struct{}{}
		out = append(out, errorCode)
	}
	return out, nil
}

func assistantNormalizeOptionalTextList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, raw := range items {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, duplicated := seen[trimmed]; duplicated {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

func (runtime *assistantKnowledgeRuntime) findIntentDoc(intentID string, locale string) (assistantKnowledgeMarkdownDocument, bool) {
	if runtime == nil {
		return assistantKnowledgeMarkdownDocument{}, false
	}
	locales, ok := runtime.intentDocs[strings.TrimSpace(intentID)]
	if !ok {
		return assistantKnowledgeMarkdownDocument{}, false
	}
	for _, candidate := range runtime.localeCandidates(locale) {
		if doc, exists := locales[candidate]; exists {
			return doc, true
		}
	}
	return assistantKnowledgeMarkdownDocument{}, false
}

func (runtime *assistantKnowledgeRuntime) findActionDocByAction(actionID string, locale string) (assistantKnowledgeMarkdownDocument, bool) {
	if runtime == nil {
		return assistantKnowledgeMarkdownDocument{}, false
	}
	locales, ok := runtime.actionDocsByAction[strings.TrimSpace(actionID)]
	if !ok {
		return assistantKnowledgeMarkdownDocument{}, false
	}
	for _, candidate := range runtime.localeCandidates(locale) {
		if doc, exists := locales[candidate]; exists {
			return doc, true
		}
	}
	return assistantKnowledgeMarkdownDocument{}, false
}

func (runtime *assistantKnowledgeRuntime) findActionDocByIntent(intentID string, locale string) (assistantKnowledgeMarkdownDocument, bool) {
	if runtime == nil {
		return assistantKnowledgeMarkdownDocument{}, false
	}
	locales, ok := runtime.actionDocsByIntent[strings.TrimSpace(intentID)]
	if !ok {
		return assistantKnowledgeMarkdownDocument{}, false
	}
	for _, candidate := range runtime.localeCandidates(locale) {
		if doc, exists := locales[candidate]; exists {
			return doc, true
		}
	}
	return assistantKnowledgeMarkdownDocument{}, false
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

func assistantReplyGuidanceMatchErrorCode(pack assistantReplyGuidancePack, errorCode string) bool {
	target := strings.TrimSpace(errorCode)
	if target == "" {
		return false
	}
	for _, code := range pack.ErrorCodes {
		if strings.TrimSpace(code) == target {
			return true
		}
	}
	return false
}

func assistantReplyGuidanceIsGeneric(pack assistantReplyGuidancePack) bool {
	return len(pack.ErrorCodes) == 0
}

func (runtime *assistantKnowledgeRuntime) findReplyGuidance(replyKind string, locale string, errorCode string) (assistantReplyGuidancePack, bool) {
	if runtime == nil {
		return assistantReplyGuidancePack{}, false
	}
	locales, ok := runtime.replyGuidance[strings.TrimSpace(replyKind)]
	if !ok {
		return assistantReplyGuidancePack{}, false
	}
	for _, candidateLocale := range runtime.localeCandidates(locale) {
		packs, exists := locales[candidateLocale]
		if !exists || len(packs) == 0 {
			continue
		}
		for _, pack := range packs {
			if assistantReplyGuidanceMatchErrorCode(pack, errorCode) {
				return pack, true
			}
		}
		for _, pack := range packs {
			if assistantReplyGuidanceIsGeneric(pack) {
				return pack, true
			}
		}
	}
	return assistantReplyGuidancePack{}, false
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

func (runtime *assistantKnowledgeRuntime) resolveInterpretationPackID(intentID string, routeKind string) string {
	intentID = strings.TrimSpace(intentID)
	if runtime == nil {
		return ""
	}
	if packID := strings.TrimSpace(runtime.routePackID[intentID]); packID != "" {
		return packID
	}
	return assistantResolveInterpretationPackIDForIntent(intentID, routeKind, runtime.interpretation)
}

func (runtime *assistantKnowledgeRuntime) findRouteByRouteKind(routeKind string) (assistantIntentRouteEntry, bool) {
	if runtime == nil {
		return assistantIntentRouteEntry{}, false
	}
	target := strings.TrimSpace(routeKind)
	if target == "" {
		return assistantIntentRouteEntry{}, false
	}
	for _, entry := range runtime.routeCatalog.Entries {
		if strings.TrimSpace(entry.RouteKind) != target {
			continue
		}
		if strings.TrimSpace(entry.ActionID) != "" {
			continue
		}
		return entry, true
	}
	if target == assistantRouteKindUncertain {
		return runtime.fallbackUncertainRoute(), true
	}
	return assistantIntentRouteEntry{}, false
}

func (runtime *assistantKnowledgeRuntime) fallbackUncertainRoute() assistantIntentRouteEntry {
	if runtime == nil {
		return assistantIntentRouteEntry{IntentID: assistantRouteFallbackUncertainID, RouteKind: assistantRouteKindUncertain}
	}
	if entry, ok := runtime.routeByIntent[assistantRouteFallbackUncertainID]; ok {
		return entry
	}
	for _, entry := range runtime.routeCatalog.Entries {
		if strings.TrimSpace(entry.RouteKind) == assistantRouteKindUncertain {
			return entry
		}
	}
	return assistantIntentRouteEntry{IntentID: assistantRouteFallbackUncertainID, RouteKind: assistantRouteKindUncertain}
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
	fallback := runtime.fallbackUncertainRoute()
	out.Action = assistantIntentPlanOnly
	out.IntentID = strings.TrimSpace(fallback.IntentID)
	if out.IntentID == "" {
		out.IntentID = assistantRouteFallbackUncertainID
	}
	out.RouteKind = strings.TrimSpace(fallback.RouteKind)
	if out.RouteKind == "" {
		out.RouteKind = assistantRouteKindUncertain
	}
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
	return assistantContractProjectionResolverResult{
		ActionID:           actionID,
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

type assistantPlanPresentation struct {
	Title   string
	Summary string
}

func (runtime *assistantKnowledgeRuntime) resolvePlanPresentation(intent assistantIntentSpec, locale string) (assistantPlanPresentation, error) {
	if runtime == nil {
		return assistantPlanPresentation{}, fmt.Errorf("%w: knowledge runtime missing", errAssistantRuntimeConfigInvalid)
	}
	routeKind := strings.TrimSpace(intent.RouteKind)
	if routeKind == "" {
		routeKind = assistantRouteKindBusinessAction
	}
	if routeKind != assistantRouteKindBusinessAction {
		intentID := strings.TrimSpace(intent.IntentID)
		if intentID == "" {
			entry, ok := runtime.findRouteByRouteKind(routeKind)
			if !ok {
				return assistantPlanPresentation{}, fmt.Errorf("%w: route intent doc missing for %s", errAssistantRuntimeConfigInvalid, routeKind)
			}
			intentID = strings.TrimSpace(entry.IntentID)
		}
		doc, ok := runtime.findIntentDoc(intentID, locale)
		if !ok {
			return assistantPlanPresentation{}, fmt.Errorf("%w: intent doc missing for %s", errAssistantRuntimeConfigInvalid, intentID)
		}
		title := strings.TrimSpace(doc.Title)
		summary := strings.TrimSpace(doc.Summary)
		if title == "" || summary == "" {
			return assistantPlanPresentation{}, fmt.Errorf("%w: intent doc title/summary required for %s", errAssistantRuntimeConfigInvalid, intentID)
		}
		return assistantPlanPresentation{Title: title, Summary: summary}, nil
	}

	actionID := strings.TrimSpace(intent.Action)
	if actionID == "" {
		return assistantPlanPresentation{}, fmt.Errorf("%w: action id required for business action", errAssistantRuntimeConfigInvalid)
	}
	pack, ok := runtime.findActionView(actionID, locale)
	if !ok {
		return assistantPlanPresentation{}, fmt.Errorf("%w: action view pack missing for %s", errAssistantRuntimeConfigInvalid, actionID)
	}
	doc, ok := runtime.findActionDocByAction(actionID, locale)
	if !ok {
		return assistantPlanPresentation{}, fmt.Errorf("%w: action doc missing for %s", errAssistantRuntimeConfigInvalid, actionID)
	}
	title := strings.TrimSpace(doc.Title)
	summary := strings.TrimSpace(pack.Summary)
	if title == "" || summary == "" {
		return assistantPlanPresentation{}, fmt.Errorf("%w: action doc title/summary required for %s", errAssistantRuntimeConfigInvalid, actionID)
	}
	return assistantPlanPresentation{Title: title, Summary: summary}, nil
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
	presentation, err := runtime.resolvePlanPresentation(intent, locale)
	if err != nil {
		return assistantPlanContextV1{}, err
	}
	context.ActionViewTitle = strings.TrimSpace(presentation.Title)
	context.ActionViewSummary = strings.TrimSpace(presentation.Summary)
	if strings.TrimSpace(intent.RouteKind) != assistantRouteKindBusinessAction {
		return context, nil
	}
	pack, ok := runtime.findActionView(strings.TrimSpace(intent.Action), locale)
	if !ok {
		return assistantPlanContextV1{}, fmt.Errorf("%w: action view pack missing for %s", errAssistantRuntimeConfigInvalid, strings.TrimSpace(intent.Action))
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
		if title := strings.TrimSpace(context.ActionViewTitle); title != "" {
			plan.Title = title
		}
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
