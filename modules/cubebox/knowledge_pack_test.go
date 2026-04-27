package cubebox

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoadKnowledgePack(t *testing.T) {
	dir := t.TempDir()
	writeKnowledgePackFile(t, dir, "CUBEBOX-SKILL.md", "# Skill\n\nqueries.md\napis.md\nexamples.md\n")
	writeKnowledgePackFile(t, dir, "queries.md", "```yaml\nintents:\n  - key: orgunit.details\n    required_params: []\n    optional_params: []\n```\n")
	writeKnowledgePackFile(t, dir, "apis.md", "```yaml\napis:\n  - executor_key: orgunit.details\n    required_params: []\n    optional_params: []\n```\n")
	writeKnowledgePackFile(t, dir, "examples.md", "```json\n{\"steps\": []}\n```\n")

	pack, err := LoadKnowledgePack(dir)
	if err != nil {
		t.Fatalf("LoadKnowledgePack err=%v", err)
	}
	if pack.Dir != dir {
		t.Fatalf("dir=%q", pack.Dir)
	}
	if len(pack.Files) != 4 {
		t.Fatalf("files=%d", len(pack.Files))
	}
}

func TestLoadKnowledgePackRejectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeKnowledgePackFile(t, dir, "CUBEBOX-SKILL.md", "# Skill\n\nqueries.md\napis.md\nexamples.md\n")
	writeKnowledgePackFile(t, dir, "queries.md", "```yaml\nintents:\n  - key: orgunit.details\n    required_params: []\n    optional_params: []\n```\n")
	writeKnowledgePackFile(t, dir, "apis.md", "```yaml\napis:\n  - executor_key: orgunit.details\n    required_params: []\n    optional_params: []\n```\n")

	_, err := LoadKnowledgePack(dir)
	if !errors.Is(err, ErrKnowledgePackInvalid) {
		t.Fatalf("expected ErrKnowledgePackInvalid, got %v", err)
	}
}

func TestValidateKnowledgePackRejectsMissingStructuredAnchor(t *testing.T) {
	pack := KnowledgePack{
		Dir: "modules/orgunit/presentation/cubebox",
		Files: map[string]string{
			"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
			"queries.md":       "# Queries\n",
			"apis.md":          "executor_key: orgunit.details\n",
			"examples.md":      "{\"steps\": []}",
		},
	}

	err := ValidateKnowledgePack(pack)
	if !errors.Is(err, ErrKnowledgePackInvalid) {
		t.Fatalf("expected ErrKnowledgePackInvalid, got %v", err)
	}
}

func TestValidateKnowledgePackRejectsBodyKeywordWithoutFencedBlock(t *testing.T) {
	pack := KnowledgePack{
		Dir: "modules/orgunit/presentation/cubebox",
		Files: map[string]string{
			"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
			"queries.md":       "这里提到了 intents: 但没有 yaml fenced block",
			"apis.md":          "这里提到了 executor_key: orgunit.details 但没有 yaml fenced block",
			"examples.md":      "这里提到了 \"steps\" 但没有 json fenced block",
		},
	}

	err := ValidateKnowledgePack(pack)
	if !errors.Is(err, ErrKnowledgePackInvalid) {
		t.Fatalf("expected ErrKnowledgePackInvalid, got %v", err)
	}
}

func TestValidateKnowledgePackRejectsInvalidYAMLOrJSONBlock(t *testing.T) {
	pack := KnowledgePack{
		Dir: "modules/orgunit/presentation/cubebox",
		Files: map[string]string{
			"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
			"queries.md":       "```yaml\nintents:\n  - key: [\n```\n",
			"apis.md":          "```yaml\napis:\n  - executor_key: orgunit.details\n    required_params: []\n    optional_params: []\n```\n",
			"examples.md":      "```json\n{\"steps\": [}\n```\n",
		},
	}

	err := ValidateKnowledgePack(pack)
	if !errors.Is(err, ErrKnowledgePackInvalid) {
		t.Fatalf("expected ErrKnowledgePackInvalid, got %v", err)
	}
}

func TestValidateKnowledgePackAgainstRegistryRejectsParamDrift(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{
			ExecutorKey:    "orgunit.details",
			RequiredParams: []string{"org_code", "as_of"},
			OptionalParams: []string{"include_disabled"},
			Executor:       readExecutorStub{},
		},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	pack := KnowledgePack{
		Dir: "modules/orgunit/presentation/cubebox",
		Files: map[string]string{
			"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
			"queries.md":       "```yaml\nintents:\n  - key: orgunit.details\n    required_params: [org_code, as_of]\n    optional_params: [include_disabled]\n```\n",
			"apis.md":          "```yaml\napis:\n  - executor_key: orgunit.details\n    required_params: [org_code]\n    optional_params: [include_disabled]\n```\n",
			"examples.md":      "```json\n{\"steps\": [{\"id\":\"step-1\",\"executor_key\":\"orgunit.details\",\"params\":{\"org_code\":\"1001\",\"as_of\":\"2026-04-23\"},\"depends_on\":[]}],\"intent\":\"orgunit.details\",\"confidence\":0.9}\n```\n",
		},
	}

	err = ValidateKnowledgePackAgainstRegistry(pack, registry)
	if !errors.Is(err, ErrKnowledgePackInvalid) {
		t.Fatalf("expected ErrKnowledgePackInvalid, got %v", err)
	}
}

func TestValidateKnowledgePackAgainstRegistryRejectsRegistryExecutorKeyMissingFromAPIsDoc(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{
			ExecutorKey:    "orgunit.details",
			RequiredParams: []string{"org_code"},
			OptionalParams: []string{"as_of"},
			Executor:       readExecutorStub{},
		},
		RegisteredExecutor{
			ExecutorKey:    "orgunit.list_children",
			RequiredParams: []string{"parent_org_code"},
			OptionalParams: []string{"as_of"},
			Executor:       readExecutorStub{},
		},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	pack := KnowledgePack{
		Dir: "modules/orgunit/presentation/cubebox",
		Files: map[string]string{
			"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
			"queries.md":       "```yaml\nintents:\n  - key: orgunit.details\n    required_params: [org_code]\n    optional_params: [as_of]\n```\n",
			"apis.md":          "```yaml\napis:\n  - executor_key: orgunit.details\n    required_params: [org_code]\n    optional_params: [as_of]\n```\n",
			"examples.md":      "```json\n{\"steps\": [{\"id\":\"step-1\",\"executor_key\":\"orgunit.details\",\"params\":{\"org_code\":\"1001\"},\"depends_on\":[]}],\"intent\":\"orgunit.details\",\"confidence\":0.9}\n```\n",
		},
	}

	err = ValidateKnowledgePackAgainstRegistry(pack, registry)
	if !errors.Is(err, ErrKnowledgePackInvalid) {
		t.Fatalf("expected ErrKnowledgePackInvalid, got %v", err)
	}
}

func TestNoQueryGuidanceFromKnowledgePacks(t *testing.T) {
	packs := []KnowledgePack{
		{
			Dir: "modules/orgunit/presentation/cubebox",
			Files: map[string]string{
				"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
				"queries.md":       "```yaml\nintents:\n  - key: orgunit.details\n    required_params: [org_code, as_of]\n    optional_params: []\nno_query_guidance:\n  scope_summary: 当前主要支持组织相关只读查询。\n  suggested_prompts:\n    - 查“华东销售中心”的详情\n    - 查“华东销售中心”的详情\n    - 搜索名称包含“销售”的组织\n```\n",
				"apis.md":          "```yaml\napis:\n  - executor_key: orgunit.details\n    required_params: [org_code, as_of]\n    optional_params: []\n```\n",
				"examples.md":      "```json\n{\"steps\": []}\n```\n",
			},
		},
	}

	guidance := NoQueryGuidanceFromKnowledgePacks(packs)
	if guidance.ScopeSummary != "当前主要支持组织相关只读查询。" {
		t.Fatalf("unexpected scope summary=%q", guidance.ScopeSummary)
	}
	if !slices.Equal(guidance.SuggestedPrompts, []string{"查“华东销售中心”的详情", "搜索名称包含“销售”的组织"}) {
		t.Fatalf("unexpected suggested prompts=%#v", guidance.SuggestedPrompts)
	}
}

func writeKnowledgePackFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
