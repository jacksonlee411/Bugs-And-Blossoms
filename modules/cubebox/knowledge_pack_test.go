package cubebox

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadKnowledgePack(t *testing.T) {
	dir := t.TempDir()
	writeKnowledgePackFile(t, dir, "CUBEBOX-SKILL.md", "# Skill\n\nqueries.md\napis.md\nexamples.md\n")
	writeKnowledgePackFile(t, dir, "queries.md", "```yaml\nintents:\n  - key: orgunit.details\n```\n")
	writeKnowledgePackFile(t, dir, "apis.md", "```yaml\napis:\n  - api_key: orgunit.details\n```\n")
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
	writeKnowledgePackFile(t, dir, "queries.md", "```yaml\nintents:\n  - key: orgunit.details\n```\n")
	writeKnowledgePackFile(t, dir, "apis.md", "```yaml\napis:\n  - api_key: orgunit.details\n```\n")

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
			"apis.md":          "api_key: orgunit.details\n",
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
			"apis.md":          "这里提到了 api_key: orgunit.details 但没有 yaml fenced block",
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
			"apis.md":          "```yaml\napis:\n  - api_key: orgunit.details\n```\n",
			"examples.md":      "```json\n{\"steps\": [}\n```\n",
		},
	}

	err := ValidateKnowledgePack(pack)
	if !errors.Is(err, ErrKnowledgePackInvalid) {
		t.Fatalf("expected ErrKnowledgePackInvalid, got %v", err)
	}
}

func writeKnowledgePackFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
