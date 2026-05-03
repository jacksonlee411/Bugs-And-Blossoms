package cubebox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var requiredKnowledgePackFiles = []string{
	"CUBEBOX-SKILL.md",
	"queries.md",
	"apis.md",
	"examples.md",
}

type KnowledgePack struct {
	Dir   string
	Files map[string]string
}

type KnowledgePackNoQueryGuidance struct {
	ScopeSummary     string
	SuggestedPrompts []string
}

type KnowledgePackRuntimeHints struct {
	UnsupportedPromptTerms []string
	ScopeParams            ScopeParamSemantics
}

type knowledgePackQueriesDoc struct {
	Intents []struct {
		Key            string   `yaml:"key"`
		RequiredParams []string `yaml:"required_params"`
		OptionalParams []string `yaml:"optional_params"`
	} `yaml:"intents"`
	NoQueryGuidance struct {
		ScopeSummary     string   `yaml:"scope_summary"`
		SuggestedPrompts []string `yaml:"suggested_prompts"`
	} `yaml:"no_query_guidance"`
	RuntimeHints struct {
		UnsupportedPromptTerms []string `yaml:"unsupported_prompt_terms"`
		ScopeParams            struct {
			ExpandAll []string `yaml:"expand_all"`
			Narrowing []string `yaml:"narrowing"`
		} `yaml:"scope_params"`
	} `yaml:"runtime_hints"`
}

type knowledgePackAPIsDoc struct {
	APITools []struct {
		OperationID string `yaml:"operation_id"`
		QueryIntent string `yaml:"query_intent"`
	} `yaml:"api_tools"`
}

func LoadKnowledgePack(dir string) (KnowledgePack, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return KnowledgePack{}, wrapKnowledgePackError("dir required")
	}

	pack := KnowledgePack{
		Dir:   dir,
		Files: make(map[string]string, len(requiredKnowledgePackFiles)),
	}

	for _, name := range requiredKnowledgePackFiles {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return KnowledgePack{}, wrapKnowledgePackError(fmt.Sprintf("%s missing: %v", name, err))
		}
		pack.Files[name] = string(raw)
	}
	if err := ValidateKnowledgePack(pack); err != nil {
		return KnowledgePack{}, err
	}
	return pack, nil
}

func ValidateKnowledgePack(pack KnowledgePack) error {
	for _, name := range requiredKnowledgePackFiles {
		content := strings.TrimSpace(pack.Files[name])
		if content == "" {
			return wrapKnowledgePackError(fmt.Sprintf("%s empty", name))
		}
	}

	skill := pack.Files["CUBEBOX-SKILL.md"]
	if !strings.Contains(skill, "queries.md") || !strings.Contains(skill, "apis.md") || !strings.Contains(skill, "examples.md") {
		return wrapKnowledgePackError("CUBEBOX-SKILL.md missing required references")
	}
	queriesBlock, err := extractFencedBlock(pack.Files["queries.md"], "yaml")
	if err != nil {
		return wrapKnowledgePackError(fmt.Sprintf("queries.md invalid: %v", err))
	}
	var queriesDoc knowledgePackQueriesDoc
	if err := yaml.Unmarshal([]byte(queriesBlock), &queriesDoc); err != nil {
		return wrapKnowledgePackError(fmt.Sprintf("queries.md yaml invalid: %v", err))
	}
	if len(queriesDoc.Intents) == 0 {
		return wrapKnowledgePackError("queries.md missing intents block")
	}
	queriesByKey := make(map[string]struct{}, len(queriesDoc.Intents))
	for _, item := range queriesDoc.Intents {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			return wrapKnowledgePackError("queries.md intent key required")
		}
		if _, exists := queriesByKey[key]; exists {
			return wrapKnowledgePackError(fmt.Sprintf("queries.md duplicate intent key: %s", key))
		}
		if item.RequiredParams == nil || item.OptionalParams == nil {
			return wrapKnowledgePackError(fmt.Sprintf("queries.md params missing for intent: %s", key))
		}
		queriesByKey[key] = struct{}{}
	}

	apisBlock, err := extractFencedBlock(pack.Files["apis.md"], "yaml")
	if err != nil {
		return wrapKnowledgePackError(fmt.Sprintf("apis.md invalid: %v", err))
	}
	var apisDoc knowledgePackAPIsDoc
	if err := yaml.Unmarshal([]byte(apisBlock), &apisDoc); err != nil {
		return wrapKnowledgePackError(fmt.Sprintf("apis.md yaml invalid: %v", err))
	}
	if len(apisDoc.APITools) == 0 {
		return wrapKnowledgePackError("apis.md missing api_tools declaration")
	}
	for _, item := range apisDoc.APITools {
		operationID := strings.TrimSpace(item.OperationID)
		if operationID == "" {
			return wrapKnowledgePackError("apis.md operation_id required")
		}
		if _, ok := queriesByKey[operationID]; !ok {
			return wrapKnowledgePackError(fmt.Sprintf("apis.md operation_id has no query intent: %s", operationID))
		}
		if strings.TrimSpace(item.QueryIntent) == "" {
			return wrapKnowledgePackError(fmt.Sprintf("apis.md query_intent required for operation_id: %s", operationID))
		}
	}

	exampleBlocks, err := extractAllFencedBlocks(pack.Files["examples.md"], "json")
	if err != nil {
		return wrapKnowledgePackError(fmt.Sprintf("examples.md invalid: %v", err))
	}
	if len(exampleBlocks) == 0 {
		return wrapKnowledgePackError("examples.md missing API_CALLS example")
	}
	validExample := false
	for _, block := range exampleBlocks {
		var payload map[string]any
		if err := json.Unmarshal([]byte(block), &payload); err != nil {
			continue
		}
		if payload["outcome"] == "API_CALLS" {
			if _, ok := payload["calls"]; ok {
				validExample = true
				break
			}
		}
	}
	if !validExample {
		return wrapKnowledgePackError("examples.md missing API_CALLS example")
	}

	return nil
}

func ValidateKnowledgePacks(packs []KnowledgePack) error {
	if len(packs) == 0 {
		return wrapKnowledgePackError("knowledge packs missing")
	}
	declaredOperations := map[string]string{}
	for _, pack := range packs {
		if err := ValidateKnowledgePack(pack); err != nil {
			return err
		}
		apisBlock, err := extractFencedBlock(pack.Files["apis.md"], "yaml")
		if err != nil {
			return wrapKnowledgePackError(fmt.Sprintf("apis.md invalid: %v", err))
		}
		var apisDoc knowledgePackAPIsDoc
		if err := yaml.Unmarshal([]byte(apisBlock), &apisDoc); err != nil {
			return wrapKnowledgePackError(fmt.Sprintf("apis.md yaml invalid: %v", err))
		}
		for _, item := range apisDoc.APITools {
			operationID := strings.TrimSpace(item.OperationID)
			if operationID == "" {
				continue
			}
			if ownerDir, exists := declaredOperations[operationID]; exists {
				return wrapKnowledgePackError(fmt.Sprintf(
					"operation_id declared by multiple knowledge packs: %s (%s, %s)",
					operationID,
					ownerDir,
					strings.TrimSpace(pack.Dir),
				))
			}
			declaredOperations[operationID] = strings.TrimSpace(pack.Dir)
		}
	}
	return nil
}

func NoQueryGuidanceFromKnowledgePacks(packs []KnowledgePack) KnowledgePackNoQueryGuidance {
	scopeParts := make([]string, 0, len(packs))
	promptIndex := make(map[string]struct{}, len(packs)*2)
	prompts := make([]string, 0, len(packs)*2)
	for _, pack := range packs {
		guidance, ok := noQueryGuidanceFromKnowledgePack(pack)
		if ok {
			if guidance.ScopeSummary != "" {
				scopeParts = append(scopeParts, guidance.ScopeSummary)
			}
			for _, prompt := range guidance.SuggestedPrompts {
				if len(prompts) >= 6 {
					break
				}
				if _, exists := promptIndex[prompt]; exists {
					continue
				}
				promptIndex[prompt] = struct{}{}
				prompts = append(prompts, prompt)
			}
		}
	}
	scopeParts = normalizeGuidancePrompts(scopeParts)
	return KnowledgePackNoQueryGuidance{
		ScopeSummary:     strings.Join(scopeParts, " "),
		SuggestedPrompts: prompts,
	}
}

func noQueryGuidanceFromKnowledgePack(pack KnowledgePack) (KnowledgePackNoQueryGuidance, bool) {
	block, err := extractFencedBlock(pack.Files["queries.md"], "yaml")
	if err != nil {
		return KnowledgePackNoQueryGuidance{}, false
	}
	var doc knowledgePackQueriesDoc
	if err := yaml.Unmarshal([]byte(block), &doc); err != nil {
		return KnowledgePackNoQueryGuidance{}, false
	}
	scope := strings.TrimSpace(doc.NoQueryGuidance.ScopeSummary)
	prompts := normalizeGuidancePrompts(doc.NoQueryGuidance.SuggestedPrompts)
	if scope == "" || len(prompts) == 0 {
		return KnowledgePackNoQueryGuidance{}, false
	}
	return KnowledgePackNoQueryGuidance{
		ScopeSummary:     scope,
		SuggestedPrompts: prompts,
	}, true
}

func normalizeGuidancePrompts(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func RuntimeHintsFromKnowledgePacks(packs []KnowledgePack) KnowledgePackRuntimeHints {
	out := KnowledgePackRuntimeHints{}
	unsupportedSeen := map[string]struct{}{}
	expandSeen := map[string]struct{}{}
	narrowSeen := map[string]struct{}{}
	for _, pack := range packs {
		hints, ok := runtimeHintsFromKnowledgePack(pack)
		if !ok {
			continue
		}
		for _, term := range hints.UnsupportedPromptTerms {
			if _, exists := unsupportedSeen[term]; exists {
				continue
			}
			unsupportedSeen[term] = struct{}{}
			out.UnsupportedPromptTerms = append(out.UnsupportedPromptTerms, term)
		}
		for _, param := range hints.ScopeParams.ExpandAll {
			if _, exists := expandSeen[param]; exists {
				continue
			}
			expandSeen[param] = struct{}{}
			out.ScopeParams.ExpandAll = append(out.ScopeParams.ExpandAll, param)
		}
		for _, param := range hints.ScopeParams.Narrowing {
			if _, exists := narrowSeen[param]; exists {
				continue
			}
			narrowSeen[param] = struct{}{}
			out.ScopeParams.Narrowing = append(out.ScopeParams.Narrowing, param)
		}
	}
	return out
}

func runtimeHintsFromKnowledgePack(pack KnowledgePack) (KnowledgePackRuntimeHints, bool) {
	block, err := extractFencedBlock(pack.Files["queries.md"], "yaml")
	if err != nil {
		return KnowledgePackRuntimeHints{}, false
	}
	var doc knowledgePackQueriesDoc
	if err := yaml.Unmarshal([]byte(block), &doc); err != nil {
		return KnowledgePackRuntimeHints{}, false
	}
	return KnowledgePackRuntimeHints{
		UnsupportedPromptTerms: normalizeGuidancePrompts(doc.RuntimeHints.UnsupportedPromptTerms),
		ScopeParams: ScopeParamSemantics{
			ExpandAll: normalizeParamNames(doc.RuntimeHints.ScopeParams.ExpandAll),
			Narrowing: normalizeParamNames(doc.RuntimeHints.ScopeParams.Narrowing),
		},
	}, true
}

func wrapKnowledgePackError(detail string) error {
	return fmt.Errorf("%w: %s", ErrKnowledgePackInvalid, strings.TrimSpace(detail))
}

func extractFencedBlock(content string, lang string) (string, error) {
	blocks, err := extractAllFencedBlocks(content, lang)
	if err != nil {
		return "", err
	}
	if len(blocks) == 0 {
		return "", fmt.Errorf("%s fenced block missing", lang)
	}
	return blocks[0], nil
}

func extractAllFencedBlocks(content string, lang string) ([]string, error) {
	lang = regexp.QuoteMeta(strings.TrimSpace(lang))
	pattern := regexp.MustCompile("(?s)```" + lang + "[ \t]*\n(.*?)\n```")
	matches := pattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("%s fenced block missing", strings.TrimSpace(lang))
	}
	blocks := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		blocks = append(blocks, match[1])
	}
	return blocks, nil
}
