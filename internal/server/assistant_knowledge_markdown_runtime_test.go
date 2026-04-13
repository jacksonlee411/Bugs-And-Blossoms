package server

import (
	"errors"
	"io/fs"
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

	makeMutated := func() []assistantKnowledgeMarkdownDocument {
		return append([]assistantKnowledgeMarkdownDocument(nil), docs...)
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

	t.Run("duplicate path rejected", func(t *testing.T) {
		mutated := append([]assistantKnowledgeMarkdownDocument{}, docs...)
		dup := mutated[0]
		mutated = append(mutated, dup)
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "duplicated markdown knowledge path") {
			t.Fatalf("expected duplicate path error, got=%v", err)
		}
	})

	t.Run("business intent action key required", func(t *testing.T) {
		mutated := append([]assistantKnowledgeMarkdownDocument{}, docs...)
		for idx := range mutated {
			if mutated[idx].Kind == "intent" && mutated[idx].RouteKind == assistantRouteKindBusinessAction {
				mutated[idx].ActionKey = ""
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "intent action_key required") {
			t.Fatalf("expected intent action_key required error, got=%v", err)
		}
	})

	t.Run("business intent action key must be registered", func(t *testing.T) {
		mutated := append([]assistantKnowledgeMarkdownDocument{}, docs...)
		for idx := range mutated {
			if mutated[idx].Kind == "intent" && mutated[idx].RouteKind == assistantRouteKindBusinessAction {
				mutated[idx].ActionKey = "unknown_action"
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "intent action_key not registered") {
			t.Fatalf("expected intent action_key registered error, got=%v", err)
		}
	})

	t.Run("action doc action key required", func(t *testing.T) {
		mutated := append([]assistantKnowledgeMarkdownDocument{}, docs...)
		for idx := range mutated {
			if mutated[idx].Kind == "action" {
				mutated[idx].ActionKey = ""
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "action_key required for action doc") {
			t.Fatalf("expected action doc action_key required error, got=%v", err)
		}
	})

	t.Run("action template fields must stay whitelisted", func(t *testing.T) {
		mutated := append([]assistantKnowledgeMarkdownDocument{}, docs...)
		for idx := range mutated {
			if mutated[idx].Kind == "action" && mutated[idx].Status == "active" {
				mutated[idx].TemplateFields = append(mutated[idx].TemplateFields, "not_allowed_field")
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "template field not allowed") {
			t.Fatalf("expected template field error, got=%v", err)
		}
	})

	t.Run("action missing field guidance error codes must be known", func(t *testing.T) {
		mutated := append([]assistantKnowledgeMarkdownDocument{}, docs...)
		for idx := range mutated {
			if mutated[idx].Kind == "action" && mutated[idx].Status == "active" {
				mutated[idx].MissingFieldGuidance = append(mutated[idx].MissingFieldGuidance, assistantActionViewGuidance{ErrorCode: "unknown_error_code", Text: "x"})
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "unknown error_code") {
			t.Fatalf("expected unknown error_code error, got=%v", err)
		}
	})

	t.Run("tool schema refs must exist in repo", func(t *testing.T) {
		mutated := append([]assistantKnowledgeMarkdownDocument{}, docs...)
		for idx := range mutated {
			if mutated[idx].Kind == "tool" {
				mutated[idx].InputSchemaRef = "missing/input.json"
				mutated[idx].OutputSchemaRef = "missing/output.json"
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "tool schema ref not found") {
			t.Fatalf("expected tool schema not found error, got=%v", err)
		}
	})

	t.Run("missing intent markdown route for action is rejected", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "intent" && mutated[idx].ActionKey == assistantIntentCreateOrgUnit {
				mutated[idx].Status = "draft"
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "missing intent markdown route") {
			t.Fatalf("expected missing intent markdown route error, got=%v", err)
		}
	})

	t.Run("intent required slots cannot contain empty values", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "intent" && mutated[idx].ID == "org.orgunit_create" {
				mutated[idx].RequiredSlots = []string{" "}
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "invalid required_slots") {
			t.Fatalf("expected invalid required_slots error, got=%v", err)
		}
	})

	t.Run("business intent required slots must match action contract", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "intent" && mutated[idx].ID == "org.orgunit_create" {
				mutated[idx].RequiredSlots = []string{"not_allowed_slot"}
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "invalid required_slot") {
			t.Fatalf("expected invalid required_slot error, got=%v", err)
		}
	})

	t.Run("intent clarification prompts must remain valid", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "intent" && mutated[idx].ID == "knowledge.general_qa" {
				mutated[idx].ClarificationPrompts = []assistantKnowledgePrompt{{TemplateID: "", Text: "test"}}
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "interpretation template_id required") {
			t.Fatalf("expected invalid clarification prompt error, got=%v", err)
		}
	})

	t.Run("intent classes must stay valid", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "intent" && mutated[idx].ID == "knowledge.general_qa" {
				mutated[idx].IntentClasses = []string{"bad_route_kind"}
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "invalid intent_class") {
			t.Fatalf("expected invalid intent_class error, got=%v", err)
		}
	})

	t.Run("negative examples cannot contain empty values", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "intent" && mutated[idx].ID == "knowledge.general_qa" {
				mutated[idx].NegativeExamples = []string{" "}
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "negative_examples contains empty value") {
			t.Fatalf("expected invalid negative_examples error, got=%v", err)
		}
	})

	t.Run("action key must stay registered for action docs", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "action" && mutated[idx].ID == "action.org.orgunit_create" {
				mutated[idx].ActionKey = "unknown_action"
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "action_key not registered") {
			t.Fatalf("expected action_key not registered error, got=%v", err)
		}
	})

	t.Run("draft action docs are indexed but skipped from active action views", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "action" && mutated[idx].ID == "action.org.orgunit_enable" {
				mutated[idx].Status = "draft"
				mutated[idx].Summary = ""
				mutated[idx].TemplateFields = []string{" ", "field_display_map"}
				mutated[idx].MissingFieldGuidance = []assistantActionViewGuidance{{ErrorCode: "", Text: "ignored"}}
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err != nil {
			t.Fatalf("draft action doc should compile successfully, got=%v", err)
		}
	})

	t.Run("blank action template fields and blank guidance codes are ignored", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "action" && mutated[idx].ID == "action.org.orgunit_enable" {
				mutated[idx].TemplateFields = append(mutated[idx].TemplateFields, " ")
				mutated[idx].MissingFieldGuidance = append(mutated[idx].MissingFieldGuidance, assistantActionViewGuidance{
					ErrorCode: " ",
					Text:      "ignored",
				})
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err != nil {
			t.Fatalf("blank action template field/guidance code should be ignored, got=%v", err)
		}
	})

	t.Run("draft reply docs are indexed but skipped from active reply guidance", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "reply" && mutated[idx].ID == "reply.manual_takeover" {
				mutated[idx].Status = "draft"
				mutated[idx].ReplyKind = "manual_takeover"
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err != nil {
			t.Fatalf("draft reply doc should compile successfully, got=%v", err)
		}
	})

	t.Run("tool name is required for tool docs", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "tool" && mutated[idx].ID == "tool.orgunit_action_precheck" {
				mutated[idx].ToolName = ""
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "tool_name required") {
			t.Fatalf("expected tool_name required error, got=%v", err)
		}
	})

	t.Run("tool locale duplicates are rejected by tool index", func(t *testing.T) {
		mutated := makeMutated()
		var dup assistantKnowledgeMarkdownDocument
		found := false
		for _, doc := range mutated {
			if doc.Kind == "tool" && doc.ID == "tool.orgunit_action_precheck" {
				dup = doc
				dup.ID = "tool.orgunit_action_precheck.alt"
				dup.Path = "assistant_knowledge_md/tools/tool.orgunit_action_precheck.alt.zh.md"
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected base tool doc to exist")
		}
		mutated = append(mutated, dup)
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "duplicated markdown doc orgunit_action_precheck locale zh") {
			t.Fatalf("expected duplicated tool locale error, got=%v", err)
		}
	})

	t.Run("wiki locale duplicates are rejected by wiki index", func(t *testing.T) {
		mutated := makeMutated()
		var dup assistantKnowledgeMarkdownDocument
		found := false
		for _, doc := range mutated {
			if doc.Kind == "wiki" && doc.ID == "wiki.assistant_runtime" {
				dup = doc
				dup.ID = "wiki.assistant_runtime.alt"
				dup.Path = "assistant_knowledge_md/wiki/wiki.assistant_runtime.alt.zh.md"
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected base wiki doc to exist")
		}
		mutated = append(mutated, dup)
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "duplicated markdown doc assistant.runtime locale zh") {
			t.Fatalf("expected duplicated wiki locale error, got=%v", err)
		}
	})

	t.Run("all intent docs cannot become inactive", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "intent" {
				mutated[idx].Status = "draft"
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "intent markdown docs missing") {
			t.Fatalf("expected missing intent docs error, got=%v", err)
		}
	})

	t.Run("all action docs cannot become inactive", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "action" {
				mutated[idx].Status = "draft"
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "action markdown docs missing") {
			t.Fatalf("expected missing action docs error, got=%v", err)
		}
	})

	t.Run("all reply docs cannot become inactive", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "reply" {
				mutated[idx].Status = "draft"
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "reply markdown docs missing") {
			t.Fatalf("expected missing reply docs error, got=%v", err)
		}
	})

	t.Run("missing action markdown doc is rejected", func(t *testing.T) {
		mutated := make([]assistantKnowledgeMarkdownDocument, 0, len(docs))
		for _, doc := range docs {
			if doc.Kind == "action" && doc.ActionKey == assistantIntentDisableOrgUnit {
				continue
			}
			mutated = append(mutated, doc)
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "missing action markdown doc for disable_orgunit") {
			t.Fatalf("expected missing action markdown doc error, got=%v", err)
		}
	})

	t.Run("missing non business intent docs are rejected", func(t *testing.T) {
		mutated := make([]assistantKnowledgeMarkdownDocument, 0, len(docs))
		for _, doc := range docs {
			if doc.Kind == "intent" && doc.ID == "chat.greeting" {
				continue
			}
			mutated = append(mutated, doc)
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "missing non-business intent markdown doc chat.greeting") {
			t.Fatalf("expected missing non-business intent error, got=%v", err)
		}
	})

	t.Run("reference validation still runs at compile tail", func(t *testing.T) {
		mutated := makeMutated()
		for idx := range mutated {
			if mutated[idx].Kind == "action" && mutated[idx].ID == "action.org.orgunit_create" {
				mutated[idx].ToolRefs = []string{"tool.missing"}
				break
			}
		}
		if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "bad tool_ref") {
			t.Fatalf("expected bad tool_ref error, got=%v", err)
		}
	})

}

func TestAssistantKnowledgeMarkdownHelperCoverage(t *testing.T) {
	t.Run("front matter split helpers", func(t *testing.T) {
		if _, _, err := assistantSplitMarkdownFrontMatter([]byte("title: missing")); err == nil || !strings.Contains(err.Error(), "front matter required") {
			t.Fatalf("expected front matter required err, got=%v", err)
		}
		if _, _, err := assistantSplitMarkdownFrontMatter([]byte("---\nid: x\nbody")); err == nil || !strings.Contains(err.Error(), "closing delimiter missing") {
			t.Fatalf("expected closing delimiter err, got=%v", err)
		}
		frontMatter, body, err := assistantSplitMarkdownFrontMatter([]byte("---\r\nid: x\r\n---\r\nbody\r\n"))
		if err != nil || string(frontMatter) != "id: x" || strings.TrimSpace(string(body)) != "body" {
			t.Fatalf("front matter=%q body=%q err=%v", string(frontMatter), string(body), err)
		}
	})

	t.Run("file identity and kind helpers", func(t *testing.T) {
		if _, _, err := assistantKnowledgeFileIdentity("assistant_knowledge_md/intent/foo.zh"); err == nil || !strings.Contains(err.Error(), "extension required") {
			t.Fatalf("expected extension err, got=%v", err)
		}
		if _, _, err := assistantKnowledgeFileIdentity("assistant_knowledge_md/intent/foo.md"); err == nil || !strings.Contains(err.Error(), "file name must be <id>.<locale>.md") {
			t.Fatalf("expected filename err, got=%v", err)
		}
		if id, locale, err := assistantKnowledgeFileIdentity("assistant_knowledge_md/intent/knowledge.general_qa.zh.md"); err != nil || id != "knowledge.general_qa" || locale != "zh" {
			t.Fatalf("identity id=%q locale=%q err=%v", id, locale, err)
		}
		cases := map[string]string{
			"assistant_knowledge_md/intent/a.zh.md":  "intent",
			"assistant_knowledge_md/actions/a.zh.md": "action",
			"assistant_knowledge_md/replies/a.zh.md": "reply",
			"assistant_knowledge_md/tools/a.zh.md":   "tool",
			"assistant_knowledge_md/wiki/a.zh.md":    "wiki",
		}
		for path, want := range cases {
			if got, err := assistantKnowledgeKindForPath(path); err != nil || got != want {
				t.Fatalf("path=%s kind=%q err=%v", path, got, err)
			}
		}
		if _, err := assistantKnowledgeKindForPath("assistant_knowledge_md/other/a.zh.md"); err == nil || !strings.Contains(err.Error(), "unsupported markdown knowledge directory") {
			t.Fatalf("expected unsupported dir err, got=%v", err)
		}
	})

	t.Run("parse document path errors", func(t *testing.T) {
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
		if _, err := assistantParseMarkdownKnowledgeDocument("assistant_knowledge_md/intent/knowledge.general_qa", raw); err == nil || !strings.Contains(err.Error(), "markdown extension required") {
			t.Fatalf("expected extension required error, got=%v", err)
		}
		if _, err := assistantParseMarkdownKnowledgeDocument("assistant_knowledge_md/other/knowledge.general_qa.zh.md", raw); err == nil || !strings.Contains(err.Error(), "unsupported markdown knowledge directory") {
			t.Fatalf("expected unsupported directory error, got=%v", err)
		}
	})

	t.Run("document validation and reference helpers", func(t *testing.T) {
		valid := assistantKnowledgeMarkdownDocument{
			Path:       "assistant_knowledge_md/tools/orgunit_action_precheck.zh.md",
			ID:         "tool.orgunit_action_precheck",
			Title:      "工具",
			Locale:     "zh",
			Kind:       "tool",
			Version:    "2026-04-13.v1",
			Status:     "active",
			SourceRefs: []string{"internal/server/assistant_action_registry.go"},
			AppliesTo:  []string{"orgunit_disable"},
		}
		if err := assistantValidateMarkdownKnowledgeDocument(valid); err != nil {
			t.Fatalf("valid doc err=%v", err)
		}
		invalidLocale := valid
		invalidLocale.Locale = "fr"
		if err := assistantValidateMarkdownKnowledgeDocument(invalidLocale); err == nil || !strings.Contains(err.Error(), "locale invalid") {
			t.Fatalf("expected locale err, got=%v", err)
		}
		invalidKind := valid
		invalidKind.Kind = "other"
		if err := assistantValidateMarkdownKnowledgeDocument(invalidKind); err == nil || !strings.Contains(err.Error(), "kind invalid") {
			t.Fatalf("expected kind err, got=%v", err)
		}
		invalidStatus := valid
		invalidStatus.Status = "offline"
		if err := assistantValidateMarkdownKnowledgeDocument(invalidStatus); err == nil || !strings.Contains(err.Error(), "status invalid") {
			t.Fatalf("expected status err, got=%v", err)
		}
		invalidID := valid
		invalidID.ID = ""
		if err := assistantValidateMarkdownKnowledgeDocument(invalidID); err == nil || !strings.Contains(err.Error(), "id required") {
			t.Fatalf("expected id err, got=%v", err)
		}
		invalidVersion := valid
		invalidVersion.Version = ""
		if err := assistantValidateMarkdownKnowledgeDocument(invalidVersion); err == nil || !strings.Contains(err.Error(), "version required") {
			t.Fatalf("expected version err, got=%v", err)
		}
		invalidAppliesTo := valid
		invalidAppliesTo.AppliesTo = nil
		if err := assistantValidateMarkdownKnowledgeDocument(invalidAppliesTo); err == nil || !strings.Contains(err.Error(), "applies_to required") {
			t.Fatalf("expected applies_to err, got=%v", err)
		}

		toolDocs := map[string]map[string]assistantKnowledgeMarkdownDocument{
			"orgunit_action_precheck": {
				"zh": {ID: "tool.orgunit_action_precheck", Locale: "zh"},
			},
		}
		wikiDocs := map[string]map[string]assistantKnowledgeMarkdownDocument{
			"assistant.task.submit": {
				"zh": {ID: "wiki.assistant.task.submit", Locale: "zh"},
			},
		}
		replyIDs := map[string]struct{}{"reply.ok": {}}
		successDocs := []assistantKnowledgeMarkdownDocument{{
			Path:      "assistant_knowledge_md/actions/action.orgunit_disable.zh.md",
			ToolRefs:  []string{"tool.orgunit_action_precheck"},
			WikiRefs:  []string{"wiki.assistant.task.submit"},
			ReplyRefs: []string{"reply.ok"},
		}}
		if err := assistantValidateMarkdownKnowledgeReferences(successDocs, replyIDs, toolDocs, wikiDocs); err != nil {
			t.Fatalf("valid refs err=%v", err)
		}
		badTool := append([]assistantKnowledgeMarkdownDocument(nil), successDocs...)
		badTool[0].ToolRefs = []string{"tool.missing"}
		if err := assistantValidateMarkdownKnowledgeReferences(badTool, replyIDs, toolDocs, wikiDocs); err == nil || !strings.Contains(err.Error(), "bad tool_ref") {
			t.Fatalf("expected tool ref err, got=%v", err)
		}
		badWiki := append([]assistantKnowledgeMarkdownDocument(nil), successDocs...)
		badWiki[0].WikiRefs = []string{"wiki.missing"}
		badWiki[0].ToolRefs = nil
		if err := assistantValidateMarkdownKnowledgeReferences(badWiki, replyIDs, toolDocs, wikiDocs); err == nil || !strings.Contains(err.Error(), "bad wiki_ref") {
			t.Fatalf("expected wiki ref err, got=%v", err)
		}
		badReply := append([]assistantKnowledgeMarkdownDocument(nil), successDocs...)
		badReply[0].ReplyRefs = []string{"reply.missing"}
		badReply[0].ToolRefs = nil
		badReply[0].WikiRefs = nil
		if err := assistantValidateMarkdownKnowledgeReferences(badReply, replyIDs, toolDocs, wikiDocs); err == nil || !strings.Contains(err.Error(), "bad reply_ref") {
			t.Fatalf("expected reply ref err, got=%v", err)
		}
	})

	t.Run("load and compile markdown knowledge", func(t *testing.T) {
		docs, rawByPath, err := assistantLoadMarkdownKnowledgeDocuments()
		if err != nil {
			t.Fatalf("load docs err=%v", err)
		}
		if len(docs) == 0 || len(rawByPath) == 0 {
			t.Fatalf("expected loaded docs and raw files, docs=%d raw=%d", len(docs), len(rawByPath))
		}
		compilation, err := assistantCompileMarkdownKnowledgeDocuments(docs)
		if err != nil {
			t.Fatalf("compile docs err=%v", err)
		}
		if len(compilation.IntentDocs) == 0 || len(compilation.ActionDocsByAction) == 0 || len(compilation.ActionDocsByIntent) == 0 {
			t.Fatalf("compilation indexes missing: %+v", compilation)
		}
		if len(compilation.ToolDocs) == 0 || len(compilation.WikiDocs) == 0 || len(compilation.ReplyGuidance) == 0 {
			t.Fatalf("compilation assets missing: %+v", compilation)
		}
		joined := assistantKnowledgeJoinedVersion([]string{"2026-04-13.v2", "2026-04-13.v1", "2026-04-13.v2"})
		if joined != "2026-04-13.v1+2026-04-13.v2" {
			t.Fatalf("joined version=%q", joined)
		}
		if len(assistantRegisteredReadonlyTools()) == 0 {
			t.Fatal("expected readonly tools to be registered")
		}
		originalRegistry := assistantDefaultActionRegistry
		defer func() { assistantDefaultActionRegistry = originalRegistry }()
		mutatedRegistry := originalRegistry
		spec := mutatedRegistry.specs[assistantIntentCreateOrgUnit]
		spec.ReadonlyTools = append(spec.ReadonlyTools, " ")
		mutatedRegistry.specs[assistantIntentCreateOrgUnit] = spec
		assistantDefaultActionRegistry = mutatedRegistry
		tools := assistantRegisteredReadonlyTools()
		if _, ok := tools[""]; ok {
			t.Fatal("blank readonly tool names should be ignored")
		}
		loaded, err := assistantLoadMarkdownKnowledgeCompilation()
		if err != nil {
			t.Fatalf("load compilation err=%v", err)
		}
		if len(loaded.RawByPath) == 0 || len(loaded.IntentDocs) == 0 {
			t.Fatalf("loaded compilation missing assets: %+v", loaded)
		}
	})

	t.Run("load markdown knowledge error helpers", func(t *testing.T) {
		t.Run("glob failure is surfaced", func(t *testing.T) {
			hooks := captureAssistantKnowledgeHooks()
			defer hooks.restore()
			assistantKnowledgeGlobFn = func(fs.FS, string) ([]string, error) {
				return nil, errors.New("boom")
			}
			if _, _, err := assistantLoadMarkdownKnowledgeDocuments(); err == nil || !strings.Contains(err.Error(), "list markdown knowledge failed") {
				t.Fatalf("expected glob failure, got=%v", err)
			}
		})

		t.Run("missing markdown files are rejected", func(t *testing.T) {
			hooks := captureAssistantKnowledgeHooks()
			defer hooks.restore()
			assistantKnowledgeGlobFn = func(fs.FS, string) ([]string, error) {
				return nil, nil
			}
			if _, _, err := assistantLoadMarkdownKnowledgeDocuments(); err == nil || !strings.Contains(err.Error(), "markdown knowledge missing") {
				t.Fatalf("expected markdown missing error, got=%v", err)
			}
		})

		t.Run("read failures are surfaced", func(t *testing.T) {
			hooks := captureAssistantKnowledgeHooks()
			defer hooks.restore()
			assistantKnowledgeGlobFn = func(_ fs.FS, pattern string) ([]string, error) {
				if pattern == "assistant_knowledge_md/intent/*.md" {
					return []string{"assistant_knowledge_md/intent/knowledge.general_qa.zh.md"}, nil
				}
				return nil, nil
			}
			assistantKnowledgeReadFileFn = func(fs.FS, string) ([]byte, error) {
				return nil, errors.New("read failed")
			}
			if _, _, err := assistantLoadMarkdownKnowledgeDocuments(); err == nil || !strings.Contains(err.Error(), "read markdown knowledge failed") {
				t.Fatalf("expected read failure, got=%v", err)
			}
		})

		t.Run("parse failures are surfaced", func(t *testing.T) {
			hooks := captureAssistantKnowledgeHooks()
			defer hooks.restore()
			assistantKnowledgeGlobFn = func(_ fs.FS, pattern string) ([]string, error) {
				if pattern == "assistant_knowledge_md/intent/*.md" {
					return []string{"assistant_knowledge_md/intent/knowledge.general_qa.zh.md"}, nil
				}
				return nil, nil
			}
			assistantKnowledgeReadFileFn = func(fs.FS, string) ([]byte, error) {
				return []byte("bad"), nil
			}
			if _, _, err := assistantLoadMarkdownKnowledgeDocuments(); err == nil || !strings.Contains(err.Error(), "markdown front matter required") {
				t.Fatalf("expected parse failure, got=%v", err)
			}
		})

		t.Run("compile failures are surfaced", func(t *testing.T) {
			hooks := captureAssistantKnowledgeHooks()
			defer hooks.restore()
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
			assistantKnowledgeGlobFn = func(_ fs.FS, pattern string) ([]string, error) {
				if pattern == "assistant_knowledge_md/intent/*.md" {
					return []string{"assistant_knowledge_md/intent/knowledge.general_qa.zh.md"}, nil
				}
				return nil, nil
			}
			assistantKnowledgeReadFileFn = func(fs.FS, string) ([]byte, error) {
				return raw, nil
			}
			if _, err := assistantLoadMarkdownKnowledgeCompilation(); err == nil || !strings.Contains(err.Error(), "action markdown docs missing") {
				t.Fatalf("expected compile failure, got=%v", err)
			}
		})
	})

	t.Run("parse helpers and required checks", func(t *testing.T) {
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
		if doc, err := assistantParseMarkdownKnowledgeDocument("assistant_knowledge_md/intent/knowledge.general_qa.zh.md", raw); err != nil || doc.ID != "knowledge.general_qa" || doc.Kind != "intent" {
			t.Fatalf("parse success doc=%+v err=%v", doc, err)
		}
		if _, err := assistantParseMarkdownKnowledgeDocument("assistant_knowledge_md/intent/knowledge.general_qa.zh.md", []byte("---\n{bad\n---\nbody")); err == nil || !strings.Contains(err.Error(), "decode front matter failed") {
			t.Fatalf("expected decode err, got=%v", err)
		}
		localeMismatch := strings.ReplaceAll(string(raw), "locale: zh", "locale: en")
		if _, err := assistantParseMarkdownKnowledgeDocument("assistant_knowledge_md/intent/knowledge.general_qa.zh.md", []byte(localeMismatch)); err == nil || !strings.Contains(err.Error(), "front matter locale mismatch") {
			t.Fatalf("expected locale mismatch err, got=%v", err)
		}
		kindMismatch := strings.ReplaceAll(string(raw), "kind: intent", "kind: action")
		if _, err := assistantParseMarkdownKnowledgeDocument("assistant_knowledge_md/intent/knowledge.general_qa.zh.md", []byte(kindMismatch)); err == nil || !strings.Contains(err.Error(), "front matter kind mismatch") {
			t.Fatalf("expected kind mismatch err, got=%v", err)
		}
		if err := assistantValidateRequiredChecksAgainstSpec(assistantActionSpec{ID: "x"}, nil); err != nil {
			t.Fatalf("empty required checks err=%v", err)
		}
		if err := assistantValidateRequiredChecksAgainstSpec(assistantActionSpec{ID: "x", Security: assistantActionSecuritySpec{RequiredChecks: []string{"tenant_admin"}}}, []string{" ", "tenant_admin"}); err != nil {
			t.Fatalf("blank required checks should be ignored err=%v", err)
		}
	})

	t.Run("compilation branch validations", func(t *testing.T) {
		docs, _, err := assistantLoadMarkdownKnowledgeDocuments()
		if err != nil {
			t.Fatalf("load docs err=%v", err)
		}

		makeMutated := func() []assistantKnowledgeMarkdownDocument {
			return append([]assistantKnowledgeMarkdownDocument(nil), docs...)
		}

		t.Run("action summary required", func(t *testing.T) {
			mutated := makeMutated()
			for idx := range mutated {
				if mutated[idx].Kind == "action" && mutated[idx].Status == "active" {
					mutated[idx].Summary = ""
					break
				}
			}
			if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "action summary required") {
				t.Fatalf("expected action summary err, got=%v", err)
			}
		})

		t.Run("action id prefix required", func(t *testing.T) {
			mutated := makeMutated()
			for idx := range mutated {
				if mutated[idx].Kind == "action" {
					mutated[idx].ID = "orgunit_disable"
					break
				}
			}
			if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "action doc id must start with action.") {
				t.Fatalf("expected action id err, got=%v", err)
			}
		})

		t.Run("reply kind required", func(t *testing.T) {
			mutated := makeMutated()
			for idx := range mutated {
				if mutated[idx].Kind == "reply" {
					mutated[idx].ReplyKind = ""
					break
				}
			}
			if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "reply_kind required") {
				t.Fatalf("expected reply kind err, got=%v", err)
			}
		})

		t.Run("tool schema refs required", func(t *testing.T) {
			mutated := makeMutated()
			for idx := range mutated {
				if mutated[idx].Kind == "tool" {
					mutated[idx].InputSchemaRef = ""
					break
				}
			}
			if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "tool schema refs required") {
				t.Fatalf("expected tool schema ref err, got=%v", err)
			}
		})

		t.Run("tool allowed route kind validated", func(t *testing.T) {
			mutated := makeMutated()
			for idx := range mutated {
				if mutated[idx].Kind == "tool" {
					mutated[idx].AllowedRouteKinds = []string{"bad_route_kind"}
					break
				}
			}
			if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "invalid allowed_route_kind") {
				t.Fatalf("expected invalid allowed route kind err, got=%v", err)
			}
		})

		t.Run("wiki topic key required", func(t *testing.T) {
			mutated := makeMutated()
			for idx := range mutated {
				if mutated[idx].Kind == "wiki" {
					mutated[idx].TopicKey = ""
					break
				}
			}
			if _, err := assistantCompileMarkdownKnowledgeDocuments(mutated); err == nil || !strings.Contains(err.Error(), "topic_key required") {
				t.Fatalf("expected wiki topic err, got=%v", err)
			}
		})
	})
}
