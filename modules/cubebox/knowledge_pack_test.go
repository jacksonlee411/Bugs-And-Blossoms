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
	writeKnowledgePackFile(t, dir, "queries.md", "```yaml\nintents:\n  - key: orgunit.details\n    required_params: [org_code, as_of]\n    optional_params: [include_disabled]\n```\n")
	writeKnowledgePackFile(t, dir, "apis.md", "```yaml\napi_tools:\n  - operation_id: orgunit.details\n    query_intent: orgunit.details\n```\n")
	writeKnowledgePackFile(t, dir, "examples.md", "```json\n{\"outcome\":\"API_CALLS\",\"calls\":[{\"id\":\"step-1\",\"method\":\"GET\",\"path\":\"/org/api/org-units/details\",\"params\":{\"org_code\":\"1001\",\"as_of\":\"2026-04-23\"},\"depends_on\":[]}]}\n```\n")

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
	writeKnowledgePackFile(t, dir, "apis.md", "```yaml\napi_tools:\n  - operation_id: orgunit.details\n    query_intent: orgunit.details\n```\n")

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
			"apis.md":          "operation_id: orgunit.details\n",
			"examples.md":      `{"outcome":"API_CALLS"}`,
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
			"apis.md":          "这里提到了 operation_id: orgunit.details 但没有 yaml fenced block",
			"examples.md":      "这里提到了 API_CALLS 但没有 json fenced block",
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
			"apis.md":          "```yaml\napi_tools:\n  - operation_id: orgunit.details\n    query_intent: orgunit.details\n```\n",
			"examples.md":      "```json\n{\"outcome\":\"API_CALLS\",\"calls\": [}\n```\n",
		},
	}

	err := ValidateKnowledgePack(pack)
	if !errors.Is(err, ErrKnowledgePackInvalid) {
		t.Fatalf("expected ErrKnowledgePackInvalid, got %v", err)
	}
}

func TestValidateKnowledgePackRejectsOperationIDWithoutIntent(t *testing.T) {
	pack := KnowledgePack{
		Dir: "modules/orgunit/presentation/cubebox",
		Files: map[string]string{
			"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
			"queries.md":       "```yaml\nintents:\n  - key: orgunit.details\n    required_params: [org_code, as_of]\n    optional_params: [include_disabled]\n```\n",
			"apis.md":          "```yaml\napi_tools:\n  - operation_id: orgunit.search\n    query_intent: orgunit.search\n```\n",
			"examples.md":      "```json\n{\"outcome\":\"API_CALLS\",\"calls\":[{\"id\":\"step-1\",\"method\":\"GET\",\"path\":\"/org/api/org-units/search\",\"params\":{\"query\":\"销售\",\"as_of\":\"2026-04-23\"},\"depends_on\":[]}]}\n```\n",
		},
	}

	err := ValidateKnowledgePack(pack)
	if !errors.Is(err, ErrKnowledgePackInvalid) {
		t.Fatalf("expected ErrKnowledgePackInvalid, got %v", err)
	}
}

func TestValidateKnowledgePackRejectsLegacyExampleShape(t *testing.T) {
	pack := KnowledgePack{
		Dir: "modules/orgunit/presentation/cubebox",
		Files: map[string]string{
			"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
			"queries.md":       "```yaml\nintents:\n  - key: orgunit.details\n    required_params: [org_code, as_of]\n    optional_params: []\n```\n",
			"apis.md":          "```yaml\napi_tools:\n  - operation_id: orgunit.details\n    query_intent: orgunit.details\n```\n",
			"examples.md":      "```json\n{\"steps\":[{\"id\":\"step-1\",\"executor_key\":\"orgunit.details\",\"params\":{\"org_code\":\"1001\",\"as_of\":\"2026-04-23\"},\"depends_on\":[]}]}\n```\n",
		},
	}

	err := ValidateKnowledgePack(pack)
	if !errors.Is(err, ErrKnowledgePackInvalid) {
		t.Fatalf("expected ErrKnowledgePackInvalid, got %v", err)
	}
}

func TestNoQueryGuidanceFromKnowledgePacks(t *testing.T) {
	packs := []KnowledgePack{
		fakeKnowledgePack("modules/orgunit/presentation/cubebox", "orgunit.details", []string{"org_code", "as_of"}, "当前主要支持组织相关只读查询。", []string{
			"查“华东销售中心”的详情",
			"查“华东销售中心”的详情",
			"搜索名称包含“销售”的组织",
		}),
		fakeKnowledgePack("modules/sample/presentation/cubebox", "sample.details", []string{"sample_id"}, "也支持样例对象只读查询。", []string{
			"查样例对象 S-100 的详情",
			"搜索名称包含“固定资产”的样例对象",
			"搜索名称包含“销售”的组织",
		}),
	}

	guidance := NoQueryGuidanceFromKnowledgePacks(packs)
	if guidance.ScopeSummary != "当前主要支持组织相关只读查询。 也支持样例对象只读查询。" {
		t.Fatalf("unexpected scope summary=%q", guidance.ScopeSummary)
	}
	if !slices.Equal(guidance.SuggestedPrompts, []string{
		"查“华东销售中心”的详情",
		"搜索名称包含“销售”的组织",
		"查样例对象 S-100 的详情",
		"搜索名称包含“固定资产”的样例对象",
	}) {
		t.Fatalf("unexpected suggested prompts=%#v", guidance.SuggestedPrompts)
	}
}

func TestNoQueryGuidanceFromKnowledgePacksKeepsLaterScopeSummaryWhenPromptsAreCapped(t *testing.T) {
	packs := []KnowledgePack{
		fakeKnowledgePack("modules/first/presentation/cubebox", "first.search", []string{"query"}, "支持第一类对象查询。", []string{
			"第一类问题 1",
			"第一类问题 2",
			"第一类问题 3",
			"第一类问题 4",
			"第一类问题 5",
			"第一类问题 6",
		}),
		fakeKnowledgePack("modules/second/presentation/cubebox", "second.search", []string{"query"}, "支持第二类对象查询。", []string{"第二类问题 1"}),
	}

	guidance := NoQueryGuidanceFromKnowledgePacks(packs)
	if guidance.ScopeSummary != "支持第一类对象查询。 支持第二类对象查询。" {
		t.Fatalf("unexpected scope summary=%q", guidance.ScopeSummary)
	}
	if !slices.Equal(guidance.SuggestedPrompts, []string{
		"第一类问题 1",
		"第一类问题 2",
		"第一类问题 3",
		"第一类问题 4",
		"第一类问题 5",
		"第一类问题 6",
	}) {
		t.Fatalf("unexpected suggested prompts=%#v", guidance.SuggestedPrompts)
	}
}

func TestRuntimeHintsFromKnowledgePacks(t *testing.T) {
	packs := []KnowledgePack{
		fakeKnowledgePackWithRuntimeHints(
			"modules/orgunit/presentation/cubebox",
			"orgunit.list",
			[]string{"as_of"},
			[]string{"all_org_units", "keyword"},
			[]string{"成本组织", "org_type"},
			[]string{"all_org_units"},
			[]string{"keyword", "parent_org_code"},
		),
		fakeKnowledgePackWithRuntimeHints(
			"modules/sample/presentation/cubebox",
			"sample.details",
			[]string{"sample_id"},
			nil,
			[]string{"成本组织", "sample hierarchy"},
			[]string{"all_samples"},
			[]string{"sample_id"},
		),
	}

	hints := RuntimeHintsFromKnowledgePacks(packs)
	if !slices.Equal(hints.UnsupportedPromptTerms, []string{"成本组织", "org_type", "sample hierarchy"}) {
		t.Fatalf("unexpected unsupported terms=%#v", hints.UnsupportedPromptTerms)
	}
	if !slices.Equal(hints.ScopeParams.ExpandAll, []string{"all_org_units", "all_samples"}) {
		t.Fatalf("unexpected expand-all params=%#v", hints.ScopeParams.ExpandAll)
	}
	if !slices.Equal(hints.ScopeParams.Narrowing, []string{"keyword", "parent_org_code", "sample_id"}) {
		t.Fatalf("unexpected narrowing params=%#v", hints.ScopeParams.Narrowing)
	}
}

func TestValidateKnowledgePacksRejectsDuplicateOperationID(t *testing.T) {
	packs := []KnowledgePack{
		fakeKnowledgePack("modules/orgunit/presentation/cubebox", "orgunit.details", []string{"org_code"}, "", nil),
		fakeKnowledgePack("modules/sample/presentation/cubebox", "sample.details", []string{"sample_id"}, "", nil),
	}
	if err := ValidateKnowledgePacks(packs); err != nil {
		t.Fatalf("ValidateKnowledgePacks err=%v", err)
	}

	duplicate := append([]KnowledgePack(nil), packs...)
	duplicate[1] = fakeKnowledgePack("modules/sample/presentation/cubebox", "orgunit.details", []string{"org_code"}, "", nil)
	err := ValidateKnowledgePacks(duplicate)
	if !errors.Is(err, ErrKnowledgePackInvalid) {
		t.Fatalf("expected ErrKnowledgePackInvalid for duplicate operation_id, got %v", err)
	}
}

func fakeKnowledgePack(dir string, operationID string, requiredParams []string, scopeSummary string, prompts []string) KnowledgePack {
	return KnowledgePack{
		Dir: dir,
		Files: map[string]string{
			"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
			"queries.md":       "```yaml\nintents:\n  - key: " + operationID + "\n    required_params: [" + joinYAMLList(requiredParams) + "]\n    optional_params: []\nno_query_guidance:\n  scope_summary: " + scopeSummary + "\n  suggested_prompts:\n" + yamlPromptListForKnowledgePackTest(prompts) + "```\n",
			"apis.md":          "```yaml\napi_tools:\n  - operation_id: " + operationID + "\n    query_intent: " + operationID + "\n```\n",
			"examples.md":      "```json\n{\"outcome\":\"API_CALLS\",\"calls\":[{\"id\":\"step-1\",\"method\":\"GET\",\"path\":\"/sample/api/items\",\"params\":{\"" + firstParam(requiredParams) + "\":\"S-100\"},\"depends_on\":[]}]}\n```\n",
		},
	}
}

func fakeKnowledgePackWithRuntimeHints(dir string, operationID string, requiredParams []string, optionalParams []string, unsupportedTerms []string, expandAll []string, narrowing []string) KnowledgePack {
	return KnowledgePack{
		Dir: dir,
		Files: map[string]string{
			"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
			"queries.md":       "```yaml\nintents:\n  - key: " + operationID + "\n    required_params: [" + joinYAMLList(requiredParams) + "]\n    optional_params: [" + joinYAMLList(optionalParams) + "]\nruntime_hints:\n  unsupported_prompt_terms: [" + joinYAMLList(unsupportedTerms) + "]\n  scope_params:\n    expand_all: [" + joinYAMLList(expandAll) + "]\n    narrowing: [" + joinYAMLList(narrowing) + "]\n```\n",
			"apis.md":          "```yaml\napi_tools:\n  - operation_id: " + operationID + "\n    query_intent: " + operationID + "\n```\n",
			"examples.md":      "```json\n{\"outcome\":\"API_CALLS\",\"calls\":[{\"id\":\"step-1\",\"method\":\"GET\",\"path\":\"/sample/api/items\",\"params\":{\"" + firstParam(requiredParams) + "\":\"S-100\"},\"depends_on\":[]}]}\n```\n",
		},
	}
}

func firstParam(items []string) string {
	if len(items) == 0 {
		return "id"
	}
	return items[0]
}

func joinYAMLList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ", "
		}
		out += item
	}
	return out
}

func yamlPromptListForKnowledgePackTest(items []string) string {
	if len(items) == 0 {
		return ""
	}
	out := ""
	for _, item := range items {
		out += "    - " + item + "\n"
	}
	return out
}

func writeKnowledgePackFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
