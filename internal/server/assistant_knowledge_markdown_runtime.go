package server

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type assistantKnowledgeMarkdownDocument struct {
	Path                           string                        `yaml:"-"`
	BodyMarkdown                   string                        `yaml:"-"`
	ID                             string                        `yaml:"id"`
	Title                          string                        `yaml:"title"`
	Locale                         string                        `yaml:"locale"`
	Kind                           string                        `yaml:"kind"`
	Version                        string                        `yaml:"version"`
	Status                         string                        `yaml:"status"`
	SourceRefs                     []string                      `yaml:"source_refs"`
	AppliesTo                      []string                      `yaml:"applies_to"`
	Summary                        string                        `yaml:"summary"`
	RouteKind                      string                        `yaml:"route_kind"`
	ActionKey                      string                        `yaml:"action_key"`
	IntentClasses                  []string                      `yaml:"intent_classes"`
	RequiredSlots                  []string                      `yaml:"required_slots"`
	MinConfidence                  float64                       `yaml:"min_confidence"`
	ClarificationPrompts           []assistantKnowledgePrompt     `yaml:"clarification_prompts"`
	Keywords                       []string                      `yaml:"keywords"`
	ToolRefs                       []string                      `yaml:"tool_refs"`
	WikiRefs                       []string                      `yaml:"wiki_refs"`
	RequiredChecks                 []string                      `yaml:"required_checks"`
	ProposalTemplate               string                        `yaml:"proposal_template"`
	ReplyRefs                      []string                      `yaml:"reply_refs"`
	FieldDisplayMap                []assistantActionViewField    `yaml:"field_display_map"`
	MissingFieldGuidance           []assistantActionViewGuidance `yaml:"missing_field_guidance"`
	FieldExamples                  []assistantActionViewExample  `yaml:"field_examples"`
	CandidateExplanationTemplates  []string                      `yaml:"candidate_explanation_templates"`
	ConfirmationSummaryTemplates   []string                      `yaml:"confirmation_summary_templates"`
	TemplateFields                 []string                      `yaml:"template_fields"`
	ReplyKind                      string                        `yaml:"reply_kind"`
	GuidanceTemplates              []assistantKnowledgePrompt     `yaml:"guidance_templates"`
	ToneConstraints                []string                      `yaml:"tone_constraints"`
	NegativeExamples               []string                      `yaml:"negative_examples"`
	ErrorCodes                     []string                      `yaml:"error_codes"`
	ToolName                       string                        `yaml:"tool_name"`
	AllowedRouteKinds              []string                      `yaml:"allowed_route_kinds"`
	InputSchemaRef                 string                        `yaml:"input_schema_ref"`
	OutputSchemaRef                string                        `yaml:"output_schema_ref"`
	UsageConstraints               []string                      `yaml:"usage_constraints"`
	TopicKey                       string                        `yaml:"topic_key"`
	RetrievalTerms                 []string                      `yaml:"retrieval_terms"`
	RelatedTopics                  []string                      `yaml:"related_topics"`
}

type assistantMarkdownKnowledgeCompilation struct {
	Catalog           assistantIntentRouteCatalog
	Interpretation    []assistantInterpretationPack
	ActionViews       []assistantActionViewPack
	ReplyGuidance     []assistantReplyGuidancePack
	RawByPath         map[string][]byte
	IntentDocs        map[string]map[string]assistantKnowledgeMarkdownDocument
	ActionDocsByAction map[string]map[string]assistantKnowledgeMarkdownDocument
	ActionDocsByIntent map[string]map[string]assistantKnowledgeMarkdownDocument
	ToolDocs          map[string]map[string]assistantKnowledgeMarkdownDocument
	WikiDocs          map[string]map[string]assistantKnowledgeMarkdownDocument
}

func assistantLoadMarkdownKnowledgeCompilation() (assistantMarkdownKnowledgeCompilation, error) {
	docs, rawByPath, err := assistantLoadMarkdownKnowledgeDocuments()
	if err != nil {
		return assistantMarkdownKnowledgeCompilation{}, err
	}
	compilation, err := assistantCompileMarkdownKnowledgeDocuments(docs)
	if err != nil {
		return assistantMarkdownKnowledgeCompilation{}, err
	}
	compilation.RawByPath = rawByPath
	return compilation, nil
}

func assistantLoadMarkdownKnowledgeDocuments() ([]assistantKnowledgeMarkdownDocument, map[string][]byte, error) {
	patterns := []string{
		"assistant_knowledge_md/intent/*.md",
		"assistant_knowledge_md/actions/*.md",
		"assistant_knowledge_md/replies/*.md",
		"assistant_knowledge_md/tools/*.md",
		"assistant_knowledge_md/wiki/*.md",
	}
	var files []string
	for _, pattern := range patterns {
		matches, err := assistantKnowledgeGlobFn(assistantKnowledgeFS, pattern)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: list markdown knowledge failed", errAssistantRuntimeConfigInvalid)
		}
		files = append(files, matches...)
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("%w: markdown knowledge missing", errAssistantRuntimeConfigInvalid)
	}
	docs := make([]assistantKnowledgeMarkdownDocument, 0, len(files))
	rawByPath := make(map[string][]byte, len(files))
	for _, file := range files {
		raw, err := assistantKnowledgeReadFileFn(assistantKnowledgeFS, file)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: read markdown knowledge failed", errAssistantRuntimeConfigInvalid)
		}
		doc, err := assistantParseMarkdownKnowledgeDocument(file, raw)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %v", errAssistantRuntimeConfigInvalid, err)
		}
		docs = append(docs, doc)
		rawByPath[file] = raw
	}
	return docs, rawByPath, nil
}

func assistantParseMarkdownKnowledgeDocument(path string, raw []byte) (assistantKnowledgeMarkdownDocument, error) {
	frontMatter, body, err := assistantSplitMarkdownFrontMatter(raw)
	if err != nil {
		return assistantKnowledgeMarkdownDocument{}, fmt.Errorf("%s: %w", path, err)
	}
	var doc assistantKnowledgeMarkdownDocument
	if err := assistantKnowledgeJSONUnmarshalFn(frontMatter, &doc); err != nil {
		return assistantKnowledgeMarkdownDocument{}, fmt.Errorf("%s: decode front matter failed", path)
	}
	doc.Path = path
	doc.BodyMarkdown = strings.TrimSpace(string(body))
	fileID, fileLocale, err := assistantKnowledgeFileIdentity(path)
	if err != nil {
		return assistantKnowledgeMarkdownDocument{}, err
	}
	if strings.TrimSpace(doc.ID) != fileID {
		return assistantKnowledgeMarkdownDocument{}, fmt.Errorf("%s: front matter id mismatch", path)
	}
	if strings.TrimSpace(doc.Locale) != fileLocale {
		return assistantKnowledgeMarkdownDocument{}, fmt.Errorf("%s: front matter locale mismatch", path)
	}
	expectedKind, err := assistantKnowledgeKindForPath(path)
	if err != nil {
		return assistantKnowledgeMarkdownDocument{}, err
	}
	if strings.TrimSpace(doc.Kind) != expectedKind {
		return assistantKnowledgeMarkdownDocument{}, fmt.Errorf("%s: front matter kind mismatch", path)
	}
	if err := assistantValidateMarkdownKnowledgeDocument(doc); err != nil {
		return assistantKnowledgeMarkdownDocument{}, err
	}
	return doc, nil
}

func assistantSplitMarkdownFrontMatter(raw []byte) ([]byte, []byte, error) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, nil, errors.New("markdown front matter required")
	}
	rest := text[len("---\n"):]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return nil, nil, errors.New("markdown front matter closing delimiter missing")
	}
	frontMatter := rest[:idx]
	body := rest[idx+len("\n---\n"):]
	return []byte(frontMatter), []byte(body), nil
}

func assistantKnowledgeFileIdentity(path string) (string, string, error) {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".md") {
		return "", "", fmt.Errorf("%s: markdown extension required", path)
	}
	stem := strings.TrimSuffix(base, ".md")
	lastDot := strings.LastIndex(stem, ".")
	if lastDot <= 0 || lastDot == len(stem)-1 {
		return "", "", fmt.Errorf("%s: file name must be <id>.<locale>.md", path)
	}
	return stem[:lastDot], stem[lastDot+1:], nil
}

func assistantKnowledgeKindForPath(path string) (string, error) {
	switch filepath.Base(filepath.Dir(path)) {
	case "intent":
		return "intent", nil
	case "actions":
		return "action", nil
	case "replies":
		return "reply", nil
	case "tools":
		return "tool", nil
	case "wiki":
		return "wiki", nil
	default:
		return "", fmt.Errorf("%s: unsupported markdown knowledge directory", path)
	}
}

func assistantValidateMarkdownKnowledgeDocument(doc assistantKnowledgeMarkdownDocument) error {
	if strings.TrimSpace(doc.ID) == "" {
		return fmt.Errorf("%s: id required", doc.Path)
	}
	if strings.TrimSpace(doc.Title) == "" {
		return fmt.Errorf("%s: title required", doc.Path)
	}
	if !assistantValidLocale(doc.Locale) {
		return fmt.Errorf("%s: locale invalid", doc.Path)
	}
	switch strings.TrimSpace(doc.Kind) {
	case "intent", "action", "reply", "tool", "wiki":
	default:
		return fmt.Errorf("%s: kind invalid", doc.Path)
	}
	if strings.TrimSpace(doc.Version) == "" {
		return fmt.Errorf("%s: version required", doc.Path)
	}
	switch strings.TrimSpace(doc.Status) {
	case "active", "draft", "deprecated":
	default:
		return fmt.Errorf("%s: status invalid", doc.Path)
	}
	if err := assistantValidateSourceRefs(doc.SourceRefs); err != nil {
		return fmt.Errorf("%s: source_refs invalid: %v", doc.Path, err)
	}
	if len(doc.AppliesTo) == 0 {
		return fmt.Errorf("%s: applies_to required", doc.Path)
	}
	return nil
}

func assistantCompileMarkdownKnowledgeDocuments(docs []assistantKnowledgeMarkdownDocument) (assistantMarkdownKnowledgeCompilation, error) {
	intentDocs := map[string]map[string]assistantKnowledgeMarkdownDocument{}
	actionDocsByAction := map[string]map[string]assistantKnowledgeMarkdownDocument{}
	actionDocsByIntent := map[string]map[string]assistantKnowledgeMarkdownDocument{}
	toolDocs := map[string]map[string]assistantKnowledgeMarkdownDocument{}
	wikiDocs := map[string]map[string]assistantKnowledgeMarkdownDocument{}
	replyIDs := map[string]struct{}{}

	intentVersions := make([]string, 0, len(docs))
	intentSourceRefs := make([]string, 0, len(docs))
	interpretation := make([]assistantInterpretationPack, 0, len(docs))
	actionViews := make([]assistantActionViewPack, 0, len(docs))
	replyGuidance := make([]assistantReplyGuidancePack, 0, len(docs))
	catalogEntries := make([]assistantIntentRouteEntry, 0, len(docs))
	registeredTools := assistantRegisteredReadonlyTools()

	seenByPath := map[string]struct{}{}
	seenDocIDs := map[string]struct{}{}
	for _, doc := range docs {
		if _, duplicated := seenByPath[doc.Path]; duplicated {
			return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: duplicated markdown knowledge path %s", errAssistantRuntimeConfigInvalid, doc.Path)
		}
		seenByPath[doc.Path] = struct{}{}
		docKey := strings.TrimSpace(doc.ID) + "|" + strings.TrimSpace(doc.Locale)
		if _, duplicated := seenDocIDs[docKey]; duplicated {
			return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: duplicated knowledge doc %s locale %s", errAssistantRuntimeConfigInvalid, doc.ID, doc.Locale)
		}
		seenDocIDs[docKey] = struct{}{}
		switch strings.TrimSpace(doc.Kind) {
		case "intent":
			if err := assistantIndexMarkdownDoc(intentDocs, strings.TrimSpace(doc.ID), doc); err != nil {
				return assistantMarkdownKnowledgeCompilation{}, err
			}
			if strings.TrimSpace(doc.Status) != "active" {
				continue
			}
			if !assistantValidRouteKind(doc.RouteKind) {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: invalid route_kind %s", errAssistantRuntimeConfigInvalid, doc.RouteKind)
			}
			if doc.RouteKind == assistantRouteKindBusinessAction {
				if strings.TrimSpace(doc.ActionKey) == "" {
					return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: intent action_key required for business_action", errAssistantRuntimeConfigInvalid)
				}
				if _, ok := assistantLookupDefaultActionSpec(strings.TrimSpace(doc.ActionKey)); !ok {
					return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: intent action_key not registered %s", errAssistantRuntimeConfigInvalid, doc.ActionKey)
				}
			}
			requiredSlots, err := assistantNormalizeRequiredSlots(doc.RequiredSlots)
			if err != nil {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: invalid required_slots for intent %s: %v", errAssistantRuntimeConfigInvalid, doc.ID, err)
			}
			if doc.RouteKind == assistantRouteKindBusinessAction {
				allowed := assistantAllowedRequiredSlotsByAction(strings.TrimSpace(doc.ActionKey))
				for _, slot := range requiredSlots {
					if _, ok := allowed[slot]; !ok {
						return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: invalid required_slot %s for action %s", errAssistantRuntimeConfigInvalid, slot, doc.ActionKey)
					}
				}
			}
			prompts, _, err := assistantNormalizeInterpretationPrompts(doc.ID, doc.Locale, doc.ClarificationPrompts)
			if err != nil {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: %v", errAssistantRuntimeConfigInvalid, err)
			}
			intentClasses := doc.IntentClasses
			if len(intentClasses) == 0 {
				intentClasses = []string{doc.RouteKind}
			}
			normalizedIntentClasses, err := assistantNormalizeInterpretationIntentClasses(intentClasses)
			if err != nil {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: %v", errAssistantRuntimeConfigInvalid, err)
			}
			negativeExamples, err := assistantNormalizeNegativeExamples(doc.NegativeExamples)
			if err != nil {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: %v", errAssistantRuntimeConfigInvalid, err)
			}
			interpretation = append(interpretation, assistantInterpretationPack{
				AssetType:            "interpretation_pack",
				PackID:               strings.TrimSpace(doc.ID),
				KnowledgeVersion:     strings.TrimSpace(doc.Version),
				Locale:               strings.TrimSpace(doc.Locale),
				IntentClasses:        normalizedIntentClasses,
				ClarificationPrompts: prompts,
				NegativeExamples:     negativeExamples,
				SourceRefs:           append([]string(nil), doc.SourceRefs...),
			})
			entry := assistantIntentRouteEntry{
				IntentID:      strings.TrimSpace(doc.ID),
				RouteKind:     strings.TrimSpace(doc.RouteKind),
				ActionID:      strings.TrimSpace(doc.ActionKey),
				RequiredSlots: requiredSlots,
				MinConfidence: doc.MinConfidence,
				Keywords:      assistantNormalizeOptionalTextList(doc.Keywords),
			}
			if len(prompts) > 0 {
				entry.ClarificationTemplateID = strings.TrimSpace(prompts[0].TemplateID)
			}
			catalogEntries = append(catalogEntries, entry)
			intentVersions = append(intentVersions, strings.TrimSpace(doc.Version))
			intentSourceRefs = append(intentSourceRefs, doc.SourceRefs...)
		case "action":
			actionKey := strings.TrimSpace(doc.ActionKey)
			if actionKey == "" {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: action_key required for action doc %s", errAssistantRuntimeConfigInvalid, doc.ID)
			}
			spec, ok := assistantLookupDefaultActionSpec(actionKey)
			if !ok {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: action_key not registered %s", errAssistantRuntimeConfigInvalid, actionKey)
			}
			if err := assistantValidateRequiredChecksAgainstSpec(spec, doc.RequiredChecks); err != nil {
				return assistantMarkdownKnowledgeCompilation{}, err
			}
			intentID := strings.TrimPrefix(strings.TrimSpace(doc.ID), "action.")
			if intentID == strings.TrimSpace(doc.ID) || strings.TrimSpace(intentID) == "" {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: action doc id must start with action.", errAssistantRuntimeConfigInvalid)
			}
			if err := assistantIndexMarkdownDoc(actionDocsByAction, actionKey, doc); err != nil {
				return assistantMarkdownKnowledgeCompilation{}, err
			}
			if err := assistantIndexMarkdownDoc(actionDocsByIntent, intentID, doc); err != nil {
				return assistantMarkdownKnowledgeCompilation{}, err
			}
			if strings.TrimSpace(doc.Status) != "active" {
				continue
			}
			if strings.TrimSpace(doc.Summary) == "" {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: action summary required", errAssistantRuntimeConfigInvalid)
			}
			for _, field := range doc.TemplateFields {
				name := strings.TrimSpace(field)
				if name == "" {
					continue
				}
				if _, ok := assistantTemplateFieldWhitelist[name]; !ok {
					return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: template field not allowed %s", errAssistantRuntimeConfigInvalid, name)
				}
			}
			for _, guidance := range doc.MissingFieldGuidance {
				code := strings.TrimSpace(guidance.ErrorCode)
				if code == "" {
					continue
				}
				if _, ok := assistantKnowledgeKnownErrorCodes[code]; !ok {
					return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: unknown error_code %s", errAssistantRuntimeConfigInvalid, code)
				}
			}
			actionViews = append(actionViews, assistantActionViewPack{
				AssetType:                     "action_view_pack",
				ActionID:                      actionKey,
				KnowledgeVersion:              strings.TrimSpace(doc.Version),
				Locale:                        strings.TrimSpace(doc.Locale),
				Summary:                       strings.TrimSpace(doc.Summary),
				FieldDisplayMap:               append([]assistantActionViewField(nil), doc.FieldDisplayMap...),
				MissingFieldGuidance:          append([]assistantActionViewGuidance(nil), doc.MissingFieldGuidance...),
				FieldExamples:                 append([]assistantActionViewExample(nil), doc.FieldExamples...),
				CandidateExplanationTemplates: assistantNormalizeOptionalTextList(doc.CandidateExplanationTemplates),
				ConfirmationSummaryTemplates:  assistantNormalizeOptionalTextList(doc.ConfirmationSummaryTemplates),
				TemplateFields:                assistantNormalizeOptionalTextList(doc.TemplateFields),
				SourceRefs:                    append([]string(nil), doc.SourceRefs...),
			})
		case "reply":
			replyID := strings.TrimSpace(doc.ID)
			replyIDs[replyID] = struct{}{}
			if strings.TrimSpace(doc.ReplyKind) == "" {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: reply_kind required for reply doc %s", errAssistantRuntimeConfigInvalid, doc.ID)
			}
			if strings.TrimSpace(doc.Status) != "active" {
				continue
			}
			replyGuidance = append(replyGuidance, assistantReplyGuidancePack{
				AssetType:         "reply_guidance_pack",
				ReplyKind:         strings.TrimSpace(doc.ReplyKind),
				KnowledgeVersion:  strings.TrimSpace(doc.Version),
				Locale:            strings.TrimSpace(doc.Locale),
				GuidanceTemplates: append([]assistantKnowledgePrompt(nil), doc.GuidanceTemplates...),
				ToneConstraints:   append([]string(nil), doc.ToneConstraints...),
				NegativeExamples:  append([]string(nil), doc.NegativeExamples...),
				ErrorCodes:        append([]string(nil), doc.ErrorCodes...),
				SourceRefs:        append([]string(nil), doc.SourceRefs...),
			})
		case "tool":
			toolName := strings.TrimSpace(doc.ToolName)
			if toolName == "" {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: tool_name required for tool doc %s", errAssistantRuntimeConfigInvalid, doc.ID)
			}
			if _, ok := registeredTools[toolName]; !ok {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: tool_name not registered %s", errAssistantRuntimeConfigInvalid, toolName)
			}
			if strings.TrimSpace(doc.InputSchemaRef) == "" || strings.TrimSpace(doc.OutputSchemaRef) == "" {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: tool schema refs required for %s", errAssistantRuntimeConfigInvalid, toolName)
			}
			if !assistantRepoPathExists(strings.TrimSpace(doc.InputSchemaRef)) || !assistantRepoPathExists(strings.TrimSpace(doc.OutputSchemaRef)) {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: tool schema ref not found for %s", errAssistantRuntimeConfigInvalid, toolName)
			}
			for _, routeKind := range doc.AllowedRouteKinds {
				if !assistantValidRouteKind(routeKind) {
					return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: invalid allowed_route_kind %s", errAssistantRuntimeConfigInvalid, routeKind)
				}
			}
			if err := assistantIndexMarkdownDoc(toolDocs, toolName, doc); err != nil {
				return assistantMarkdownKnowledgeCompilation{}, err
			}
		case "wiki":
			topicKey := strings.TrimSpace(doc.TopicKey)
			if topicKey == "" {
				return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: topic_key required for wiki doc %s", errAssistantRuntimeConfigInvalid, doc.ID)
			}
			if err := assistantIndexMarkdownDoc(wikiDocs, topicKey, doc); err != nil {
				return assistantMarkdownKnowledgeCompilation{}, err
			}
		}
	}

	if len(catalogEntries) == 0 {
		return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: intent markdown docs missing", errAssistantRuntimeConfigInvalid)
	}
	if len(actionViews) == 0 {
		return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: action markdown docs missing", errAssistantRuntimeConfigInvalid)
	}
	if len(replyGuidance) == 0 {
		return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: reply markdown docs missing", errAssistantRuntimeConfigInvalid)
	}
	for _, actionID := range assistantOrderedBusinessActionIDs() {
		if _, ok := actionDocsByAction[actionID]; !ok {
			return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: missing action markdown doc for %s", errAssistantRuntimeConfigInvalid, actionID)
		}
		matched := false
		for _, entry := range catalogEntries {
			if strings.TrimSpace(entry.ActionID) == actionID {
				matched = true
				break
			}
		}
		if !matched {
			return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: missing intent markdown route for %s", errAssistantRuntimeConfigInvalid, actionID)
		}
	}
	for _, intentID := range []string{assistantInterpretationDefaultPackID, "chat.greeting", assistantRouteFallbackUncertainID} {
		if _, ok := intentDocs[intentID]; !ok {
			return assistantMarkdownKnowledgeCompilation{}, fmt.Errorf("%w: missing non-business intent markdown doc %s", errAssistantRuntimeConfigInvalid, intentID)
		}
	}

	if err := assistantValidateMarkdownKnowledgeReferences(docs, replyIDs, toolDocs, wikiDocs); err != nil {
		return assistantMarkdownKnowledgeCompilation{}, err
	}

	return assistantMarkdownKnowledgeCompilation{
		Catalog: assistantIntentRouteCatalog{
			AssetType:           "intent_route_catalog",
			RouteCatalogVersion: assistantKnowledgeJoinedVersion(intentVersions),
			SourceRefs:          assistantNormalizeOptionalTextList(intentSourceRefs),
			Entries:             catalogEntries,
		},
		Interpretation:     interpretation,
		ActionViews:        actionViews,
		ReplyGuidance:      replyGuidance,
		IntentDocs:         intentDocs,
		ActionDocsByAction: actionDocsByAction,
		ActionDocsByIntent: actionDocsByIntent,
		ToolDocs:           toolDocs,
		WikiDocs:           wikiDocs,
	}, nil
}

func assistantIndexMarkdownDoc(index map[string]map[string]assistantKnowledgeMarkdownDocument, key string, doc assistantKnowledgeMarkdownDocument) error {
	key = strings.TrimSpace(key)
	locale := strings.TrimSpace(doc.Locale)
	if _, ok := index[key]; !ok {
		index[key] = map[string]assistantKnowledgeMarkdownDocument{}
	}
	if _, duplicated := index[key][locale]; duplicated {
		return fmt.Errorf("%w: duplicated markdown doc %s locale %s", errAssistantRuntimeConfigInvalid, key, locale)
	}
	index[key][locale] = doc
	return nil
}

func assistantKnowledgeJoinedVersion(versions []string) string {
	normalized := assistantNormalizeOptionalTextList(versions)
	sort.Strings(normalized)
	return strings.Join(normalized, "+")
}

func assistantValidateRequiredChecksAgainstSpec(spec assistantActionSpec, requiredChecks []string) error {
	if len(requiredChecks) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(spec.Security.RequiredChecks))
	for _, item := range spec.Security.RequiredChecks {
		allowed[strings.TrimSpace(item)] = struct{}{}
	}
	for _, item := range requiredChecks {
		check := strings.TrimSpace(item)
		if check == "" {
			continue
		}
		if _, ok := allowed[check]; !ok {
			return fmt.Errorf("%w: required_check not registered for %s: %s", errAssistantRuntimeConfigInvalid, spec.ID, check)
		}
	}
	return nil
}

func assistantRegisteredReadonlyTools() map[string]struct{} {
	out := map[string]struct{}{}
	for _, spec := range assistantDefaultActionRegistry.specs {
		for _, toolName := range spec.ReadonlyTools {
			trimmed := strings.TrimSpace(toolName)
			if trimmed == "" {
				continue
			}
			out[trimmed] = struct{}{}
		}
	}
	return out
}

func assistantValidateMarkdownKnowledgeReferences(
	docs []assistantKnowledgeMarkdownDocument,
	replyIDs map[string]struct{},
	toolDocs map[string]map[string]assistantKnowledgeMarkdownDocument,
	wikiDocs map[string]map[string]assistantKnowledgeMarkdownDocument,
) error {
	toolIDs := map[string]struct{}{}
	for _, locales := range toolDocs {
		for _, doc := range locales {
			toolIDs[strings.TrimSpace(doc.ID)] = struct{}{}
		}
	}
	wikiIDs := map[string]struct{}{}
	for _, locales := range wikiDocs {
		for _, doc := range locales {
			wikiIDs[strings.TrimSpace(doc.ID)] = struct{}{}
		}
	}
	for _, doc := range docs {
		for _, ref := range doc.ToolRefs {
			if _, ok := toolIDs[strings.TrimSpace(ref)]; !ok {
				return fmt.Errorf("%w: bad tool_ref %s in %s", errAssistantRuntimeConfigInvalid, ref, doc.Path)
			}
		}
		for _, ref := range doc.WikiRefs {
			if _, ok := wikiIDs[strings.TrimSpace(ref)]; !ok {
				return fmt.Errorf("%w: bad wiki_ref %s in %s", errAssistantRuntimeConfigInvalid, ref, doc.Path)
			}
		}
		for _, ref := range doc.ReplyRefs {
			if _, ok := replyIDs[strings.TrimSpace(ref)]; !ok {
				return fmt.Errorf("%w: bad reply_ref %s in %s", errAssistantRuntimeConfigInvalid, ref, doc.Path)
			}
		}
	}
	return nil
}
