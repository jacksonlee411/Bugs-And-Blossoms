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

type knowledgePackQueriesDoc struct {
	Intents []struct {
		Key string `yaml:"key"`
	} `yaml:"intents"`
}

type knowledgePackAPIsDoc struct {
	APIs []struct {
		APIKey string `yaml:"api_key"`
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
