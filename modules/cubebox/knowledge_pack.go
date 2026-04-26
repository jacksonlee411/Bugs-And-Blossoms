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
	ScopeSummary           string
	SuggestedPrompts       []string
	ContextFollowupPrompts []string
}

type knowledgePackQueriesDoc struct {
	Intents []struct {
		Key            string   `yaml:"key"`
		RequiredParams []string `yaml:"required_params"`
		OptionalParams []string `yaml:"optional_params"`
	} `yaml:"intents"`
	NoQueryGuidance struct {
		ScopeSummary           string   `yaml:"scope_summary"`
		SuggestedPrompts       []string `yaml:"suggested_prompts"`
		ContextFollowupPrompts []string `yaml:"context_followup_prompts"`
	} `yaml:"no_query_guidance"`
}

type knowledgePackAPIsDoc struct {
	APIs []struct {
		APIKey         string   `yaml:"api_key"`
		RequiredParams []string `yaml:"required_params"`
		OptionalParams []string `yaml:"optional_params"`
	} `yaml:"apis"`
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
	queriesByKey := make(map[string]struct {
		RequiredParams []string
		OptionalParams []string
	}, len(queriesDoc.Intents))
	for _, item := range queriesDoc.Intents {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			return wrapKnowledgePackError("queries.md intent key required")
		}
		if _, exists := queriesByKey[key]; exists {
			return wrapKnowledgePackError(fmt.Sprintf("queries.md duplicate intent key: %s", key))
		}
		required := normalizeParamNames(item.RequiredParams)
		optional := normalizeParamNames(item.OptionalParams)
		if item.RequiredParams == nil || item.OptionalParams == nil {
			return wrapKnowledgePackError(fmt.Sprintf("queries.md params missing for intent: %s", key))
		}
		queriesByKey[key] = struct {
			RequiredParams []string
			OptionalParams []string
		}{
			RequiredParams: required,
			OptionalParams: optional,
		}
	}

	apisBlock, err := extractFencedBlock(pack.Files["apis.md"], "yaml")
	if err != nil {
		return wrapKnowledgePackError(fmt.Sprintf("apis.md invalid: %v", err))
	}
	var apisDoc knowledgePackAPIsDoc
	if err := yaml.Unmarshal([]byte(apisBlock), &apisDoc); err != nil {
		return wrapKnowledgePackError(fmt.Sprintf("apis.md yaml invalid: %v", err))
	}
	if len(apisDoc.APIs) == 0 {
		return wrapKnowledgePackError("apis.md missing api_key declaration")
	}
	for _, item := range apisDoc.APIs {
		apiKey := strings.TrimSpace(item.APIKey)
		if apiKey == "" {
			return wrapKnowledgePackError("apis.md api_key required")
		}
		if item.RequiredParams == nil || item.OptionalParams == nil {
			return wrapKnowledgePackError(fmt.Sprintf("apis.md params missing for api_key: %s", apiKey))
		}
		if queryDoc, ok := queriesByKey[apiKey]; ok {
			required := normalizeParamNames(item.RequiredParams)
			optional := normalizeParamNames(item.OptionalParams)
			if !sameNormalizedParamSet(required, queryDoc.RequiredParams) || !sameNormalizedParamSet(optional, queryDoc.OptionalParams) {
				return wrapKnowledgePackError(fmt.Sprintf("queries/apis params drift for key: %s", apiKey))
			}
		}
	}

	exampleBlocks, err := extractAllFencedBlocks(pack.Files["examples.md"], "json")
	if err != nil {
		return wrapKnowledgePackError(fmt.Sprintf("examples.md invalid: %v", err))
	}
	if len(exampleBlocks) == 0 {
		return wrapKnowledgePackError("examples.md missing ReadPlan example")
	}
	validExample := false
	for _, block := range exampleBlocks {
		var payload map[string]any
		if err := json.Unmarshal([]byte(block), &payload); err != nil {
			continue
		}
		if _, ok := payload["steps"]; ok {
			validExample = true
			break
		}
	}
	if !validExample {
		return wrapKnowledgePackError("examples.md missing ReadPlan example")
	}

	return nil
}

func NoQueryGuidanceFromKnowledgePacks(packs []KnowledgePack) KnowledgePackNoQueryGuidance {
	for _, pack := range packs {
		guidance, ok := noQueryGuidanceFromKnowledgePack(pack)
		if ok {
			return guidance
		}
	}
	return KnowledgePackNoQueryGuidance{}
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
	followups := normalizeGuidancePrompts(doc.NoQueryGuidance.ContextFollowupPrompts)
	if scope == "" || len(prompts) == 0 {
		return KnowledgePackNoQueryGuidance{}, false
	}
	return KnowledgePackNoQueryGuidance{
		ScopeSummary:           scope,
		SuggestedPrompts:       prompts,
		ContextFollowupPrompts: followups,
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

func ValidateKnowledgePackAgainstRegistry(pack KnowledgePack, registry *ExecutionRegistry) error {
	if registry == nil {
		return wrapKnowledgePackError("execution registry missing")
	}
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
	declaredAPIKeys := make(map[string]struct{}, len(apisDoc.APIs))
	for _, item := range apisDoc.APIs {
		apiKey := strings.TrimSpace(item.APIKey)
		declaredAPIKeys[apiKey] = struct{}{}
		registered, ok := registry.Resolve(apiKey)
		if !ok {
			return wrapKnowledgePackError(fmt.Sprintf("apis.md api_key not registered: %s", apiKey))
		}
		required := normalizeParamNames(item.RequiredParams)
		optional := normalizeParamNames(item.OptionalParams)
		if !sameNormalizedParamSet(required, registered.RequiredParams) || !sameNormalizedParamSet(optional, registered.OptionalParams) {
			return wrapKnowledgePackError(fmt.Sprintf(
				"apis.md params drift for %s: required=%v optional=%v registry_required=%v registry_optional=%v",
				apiKey,
				required,
				optional,
				registered.RequiredParams,
				registered.OptionalParams,
			))
		}
	}
	for _, registered := range registry.RegisteredExecutors() {
		if _, ok := declaredAPIKeys[registered.APIKey]; ok {
			continue
		}
		return wrapKnowledgePackError(fmt.Sprintf("execution registry api_key missing from apis.md: %s", registered.APIKey))
	}
	return nil
}

func sameNormalizedParamSet(left []string, right []string) bool {
	left = normalizeParamNames(left)
	right = normalizeParamNames(right)
	if len(left) != len(right) {
		return false
	}
	index := make(map[string]struct{}, len(left))
	for _, item := range left {
		index[item] = struct{}{}
	}
	for _, item := range right {
		if _, ok := index[item]; !ok {
			return false
		}
	}
	return true
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
