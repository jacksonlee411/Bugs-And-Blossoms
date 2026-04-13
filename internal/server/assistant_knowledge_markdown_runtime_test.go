package server

import (
	"strings"
	"testing"
)

func TestAssistantKnowledgeMarkdownDocumentValidation(t *testing.T) {
	t.Run("front matter missing required field", func(t *testing.T) {
		raw := []byte(`---
id: knowledge.general_qa
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
applies_to:
  - knowledge_qa
route_kind: knowledge_qa
clarification_prompts:
  - template_id: clarify.knowledge.general_qa.v1
    text: test
---
body`)
		if _, err := assistantParseMarkdownKnowledgeDocument("assistant_knowledge_md/intent/knowledge.general_qa.zh.md", raw); err == nil || !strings.Contains(err.Error(), "title required") {
			t.Fatalf("expected missing title error, got=%v", err)
		}
	})

	t.Run("file name and front matter mismatch", func(t *testing.T) {
		raw := []byte(`---
id: knowledge.general_qa
title: 知识问答
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
applies_to:
  - knowledge_qa
route_kind: knowledge_qa
clarification_prompts:
  - template_id: clarify.knowledge.general_qa.v1
    text: test
---
body`)
		if _, err := assistantParseMarkdownKnowledgeDocument("assistant_knowledge_md/intent/chat.greeting.zh.md", raw); err == nil || !strings.Contains(err.Error(), "front matter id mismatch") {
			t.Fatalf("expected id mismatch error, got=%v", err)
		}
	})

	t.Run("archive ref rejected", func(t *testing.T) {
		doc := assistantKnowledgeMarkdownDocument{
			Path:       "assistant_knowledge_md/intent/knowledge.general_qa.zh.md",
			ID:         "knowledge.general_qa",
			Title:      "知识问答",
			Locale:     "zh",
			Kind:       "intent",
			Version:    "2026-04-13.v1",
			Status:     "active",
			SourceRefs: []string{"docs/archive/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md"},
			AppliesTo:  []string{"knowledge_qa"},
		}
		if err := assistantValidateMarkdownKnowledgeDocument(doc); err == nil || !strings.Contains(err.Error(), "source_refs invalid") {
			t.Fatalf("expected archive ref error, got=%v", err)
		}
	})
}

func TestAssistantKnowledgeMarkdownCompilationValidation(t *testing.T) {
	docs, _, err := assistantLoadMarkdownKnowledgeDocuments()
	if err != nil {
		t.Fatalf("load markdown docs err=%v", err)
	}

	t.Run("duplicate id locale rejected", func(t *testing.T) {
		duplicated := append([]assistantKnowledgeMarkdownDocument{}, docs...)
		dup := duplicated[0]
		dup.Path = dup.Path + ".copy"
		duplicated = append(duplicated, dup)
		if _, err := assistantCompileMarkdownKnowledgeDocuments(duplicated); err == nil || !strings.Contains(err.Error(), "duplicated knowledge doc") {
			t.Fatalf("expected duplicate doc error, got=%v", err)
		}
	})

	t.Run("invalid route kind rejected", func(t *testing.T) {
		mutated := append([]assistantKnowledgeMarkdownDocument{}, docs...)
		for idx := range mutated {
			if mutated[idx].Kind == "intent" && mutated[idx].ID == "knowledge.general_qa" {
				mutated[idx].RouteKind = "bad_route_kind"
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "invalid route_kind") {
			t.Fatalf("expected invalid route_kind error, got=%v", err)
		}
	})

	t.Run("tool name must exist in registry", func(t *testing.T) {
		mutated := append([]assistantKnowledgeMarkdownDocument{}, docs...)
		for idx := range mutated {
			if mutated[idx].Kind == "tool" {
				mutated[idx].ToolName = "tool_not_registered"
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "tool_name not registered") {
			t.Fatalf("expected tool_name error, got=%v", err)
		}
	})

	t.Run("required checks must stay within formal contract", func(t *testing.T) {
		mutated := append([]assistantKnowledgeMarkdownDocument{}, docs...)
		for idx := range mutated {
			if mutated[idx].Kind == "action" && mutated[idx].ActionKey == assistantIntentCreateOrgUnit {
				mutated[idx].RequiredChecks = append(mutated[idx].RequiredChecks, "made_up_check")
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "required_check not registered") {
			t.Fatalf("expected required_check error, got=%v", err)
		}
	})
}
