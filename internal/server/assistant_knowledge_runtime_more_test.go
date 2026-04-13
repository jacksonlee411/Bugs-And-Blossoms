package server

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"
)

type assistantKnowledgeHooksSnapshot struct {
	readFileFn      func(fs.FS, string) ([]byte, error)
	globFn          func(fs.FS, string) ([]string, error)
	unmarshalFn     func([]byte, any) error
	statFn          func(string) (os.FileInfo, error)
	callerFn        func(int) (uintptr, string, int, bool)
	canonicalHashFn func(any) string
	loadRuntimeFn   func() (*assistantKnowledgeRuntime, error)
}

func captureAssistantKnowledgeHooks() assistantKnowledgeHooksSnapshot {
	return assistantKnowledgeHooksSnapshot{
		readFileFn:      assistantKnowledgeReadFileFn,
		globFn:          assistantKnowledgeGlobFn,
		unmarshalFn:     assistantKnowledgeJSONUnmarshalFn,
		statFn:          assistantKnowledgeRepoStatFn,
		callerFn:        assistantKnowledgeRuntimeCallerFn,
		canonicalHashFn: assistantKnowledgeCanonicalHashFn,
		loadRuntimeFn:   assistantLoadKnowledgeRuntimeFn,
	}
}

func (snapshot assistantKnowledgeHooksSnapshot) restore() {
	assistantKnowledgeReadFileFn = snapshot.readFileFn
	assistantKnowledgeGlobFn = snapshot.globFn
	assistantKnowledgeJSONUnmarshalFn = snapshot.unmarshalFn
	assistantKnowledgeRepoStatFn = snapshot.statFn
	assistantKnowledgeRuntimeCallerFn = snapshot.callerFn
	assistantKnowledgeCanonicalHashFn = snapshot.canonicalHashFn
	assistantLoadKnowledgeRuntimeFn = snapshot.loadRuntimeFn
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "fake" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0o644 }
func (fakeFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }

func assistantKnowledgeBaseCompileInput() (
	assistantIntentRouteCatalog,
	[]assistantInterpretationPack,
	[]assistantActionViewPack,
	[]assistantReplyGuidancePack,
	map[string][]byte,
) {
	sourceRef := "internal/server/assistant_knowledge_runtime.go"
	catalog := assistantIntentRouteCatalog{
		AssetType:           "intent_route_catalog",
		RouteCatalogVersion: "2026-03-11.v1",
		SourceRefs:          []string{sourceRef},
		Entries: []assistantIntentRouteEntry{
			{
				IntentID:  "org.orgunit_create",
				RouteKind: assistantRouteKindBusinessAction,
				ActionID:  assistantIntentCreateOrgUnit,
			},
			{
				IntentID:  "knowledge.general_qa",
				RouteKind: assistantRouteKindKnowledgeQA,
				Keywords:  []string{"功能", "help"},
			},
		},
	}
	interpretation := []assistantInterpretationPack{
		{
			AssetType:            "interpretation_pack",
			PackID:               "knowledge.general_qa",
			KnowledgeVersion:     "2026-03-11.v1",
			Locale:               "zh",
			IntentClasses:        []string{assistantRouteKindKnowledgeQA},
			ClarificationPrompts: []assistantKnowledgePrompt{{TemplateID: "qa.zh", Text: "你好"}},
			SourceRefs:           []string{sourceRef},
		},
	}
	actionViews := []assistantActionViewPack{
		{
			AssetType:        "action_view_pack",
			ActionID:         assistantIntentCreateOrgUnit,
			KnowledgeVersion: "2026-03-11.v1",
			Locale:           "zh",
			Summary:          "创建组织",
			TemplateFields:   []string{"action_view_pack.summary"},
			MissingFieldGuidance: []assistantActionViewGuidance{
				{ErrorCode: "missing_parent_ref_text", Text: "请补充上级组织"},
			},
			SourceRefs: []string{sourceRef},
		},
	}
	replyGuidance := []assistantReplyGuidancePack{
		{
			AssetType:        "reply_guidance_pack",
			ReplyKind:        "missing_fields",
			KnowledgeVersion: "2026-03-11.v1",
			Locale:           "zh",
			GuidanceTemplates: []assistantKnowledgePrompt{
				{TemplateID: "reply.missing_fields.v1", Text: "请补充：{missing_fields}"},
			},
			ErrorCodes: []string{"missing_parent_ref_text"},
			SourceRefs: []string{sourceRef},
		},
	}
	rawByPath := map[string][]byte{
		"ok.json": []byte(`{"ok":true}`),
	}
	return catalog, interpretation, actionViews, replyGuidance, rawByPath
}

func TestAssistantKnowledgeRuntime_LoadersErrorPaths(t *testing.T) {
	t.Run("markdown runtime read failed", func(t *testing.T) {
		hooks := captureAssistantKnowledgeHooks()
		defer hooks.restore()
		assistantKnowledgeReadFileFn = func(_ fs.FS, path string) ([]byte, error) {
			if path == "assistant_knowledge_md/intent/org.orgunit_create.zh.md" {
				return nil, errors.New("read failed")
			}
			return hooks.readFileFn(assistantKnowledgeFS, path)
		}
		if _, err := assistantLoadKnowledgeRuntime(); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
			t.Fatalf("expected runtime config invalid, got=%v", err)
		}
	})

	t.Run("markdown runtime front matter decode failed", func(t *testing.T) {
		hooks := captureAssistantKnowledgeHooks()
		defer hooks.restore()
		assistantKnowledgeReadFileFn = func(_ fs.FS, path string) ([]byte, error) {
			if path == "assistant_knowledge_md/intent/org.orgunit_create.zh.md" {
				return []byte("---\nid: org.orgunit_create\nlocale: zh\nkind: intent\nversion:\n  - bad\n---\nbody"), nil
			}
			return hooks.readFileFn(assistantKnowledgeFS, path)
		}
		if _, err := assistantLoadKnowledgeRuntime(); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
			t.Fatalf("expected runtime config invalid, got=%v", err)
		}
	})

	t.Run("markdown runtime glob failed", func(t *testing.T) {
		hooks := captureAssistantKnowledgeHooks()
		defer hooks.restore()
		assistantKnowledgeGlobFn = func(fsys fs.FS, pattern string) ([]string, error) {
			if pattern == "assistant_knowledge_md/intent/*.md" {
				return nil, errors.New("glob failed")
			}
			return hooks.globFn(fsys, pattern)
		}
		if _, err := assistantLoadKnowledgeRuntime(); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
			t.Fatalf("expected runtime config invalid, got=%v", err)
		}
	})

	t.Run("markdown runtime compile failed", func(t *testing.T) {
		hooks := captureAssistantKnowledgeHooks()
		defer hooks.restore()
		assistantKnowledgeCanonicalHashFn = func(any) string { return "" }
		if _, err := assistantLoadKnowledgeRuntime(); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
			t.Fatalf("expected runtime config invalid, got=%v", err)
		}
	})
}

func TestAssistantKnowledgeRuntime_CompileValidationCoverage(t *testing.T) {
	type compileMutator func(
		*assistantIntentRouteCatalog,
		*[]assistantInterpretationPack,
		*[]assistantActionViewPack,
		*[]assistantReplyGuidancePack,
		map[string][]byte,
	)

	runCompileError := func(name string, want string, mutate compileMutator) {
		t.Run(name, func(t *testing.T) {
			catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
			if mutate != nil {
				mutate(&catalog, &interpretation, &actionViews, &replyGuidance, rawByPath)
			}
			_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("expected %q, got=%v", want, err)
			}
		})
	}

	runCompileError("catalog asset type invalid", "intent route catalog asset_type invalid", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.AssetType = "x"
	})
	runCompileError("route catalog version required", "route_catalog_version required", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.RouteCatalogVersion = " "
	})
	runCompileError("route catalog source refs invalid", "route catalog source_refs invalid", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.SourceRefs = []string{""}
	})
	runCompileError("forbidden key", "forbidden key", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, raw map[string][]byte) {
		raw["bad.json"] = []byte(`{"required_fields":["x"]}`)
	})
	runCompileError("forbidden key decode failed", "decode", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, raw map[string][]byte) {
		raw["bad.json"] = []byte(`{`)
	})
	runCompileError("intent id required", "intent_id required", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries[0].IntentID = ""
	})
	runCompileError("duplicate intent id", "duplicated intent_id", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries = append(c.Entries, c.Entries[0])
	})
	runCompileError("invalid route kind", "invalid route_kind", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries[0].RouteKind = "bad_kind"
	})
	runCompileError("business action missing action id", "action_id required for business_action", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries[0].ActionID = ""
	})
	runCompileError("business action not registered", "action_id not registered", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries[0].ActionID = "org.unknown"
	})
	runCompileError("non-business action id must be empty", "action_id must be empty for non-business route", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries[1].ActionID = assistantIntentCreateOrgUnit
	})
	runCompileError("duplicated route action id", "duplicated route action_id", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries = append(c.Entries, assistantIntentRouteEntry{
			IntentID:  "org.orgunit_create_dup",
			RouteKind: assistantRouteKindBusinessAction,
			ActionID:  assistantIntentCreateOrgUnit,
		})
	})
	runCompileError("min confidence out of range", "min_confidence out of range", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries[0].MinConfidence = 1.2
	})
	runCompileError("required slots empty item", "invalid required_slots", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries[0].RequiredSlots = []string{" "}
	})
	runCompileError("invalid required slots", "invalid required_slot", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries[0].RequiredSlots = []string{"unknown_slot"}
	})
	runCompileError("unknown clarification template id", "unknown clarification_template_id", func(c *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries[1].ClarificationTemplateID = "clarify.unknown.v1"
	})

	runCompileError("interpretation asset type invalid", "interpretation asset_type invalid", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].AssetType = "bad"
	})
	runCompileError("interpretation pack id required", "interpretation pack_id required", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].PackID = " "
	})
	runCompileError("interpretation knowledge version required", "interpretation knowledge_version required", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].KnowledgeVersion = " "
	})
	runCompileError("interpretation locale invalid", "interpretation locale invalid", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].Locale = "jp"
	})
	runCompileError("interpretation source refs required", "interpretation source_refs required", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].SourceRefs = nil
	})
	runCompileError("interpretation source refs invalid", "interpretation source_refs invalid", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].SourceRefs = []string{"not/exist/path.md"}
	})
	runCompileError("interpretation intent classes required", "interpretation intent_classes required", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].IntentClasses = nil
	})
	runCompileError("interpretation intent class invalid", "invalid intent_class", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].IntentClasses = []string{"invalid"}
	})
	runCompileError("interpretation template id required", "interpretation template_id required", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].ClarificationPrompts[0].TemplateID = " "
	})
	runCompileError("interpretation duplicate template id", "duplicated interpretation template_id", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].ClarificationPrompts = append((*i)[0].ClarificationPrompts, assistantKnowledgePrompt{TemplateID: "qa.zh", Text: "duplicate"})
	})
	runCompileError("interpretation negative examples empty", "negative_examples contains empty value", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].NegativeExamples = []string{" "}
	})
	runCompileError("interpretation duplicate locale", "duplicated interpretation pack", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		*i = append(*i, (*i)[0])
	})
	runCompileError("interpretation intent class mismatch route", "intent_classes mismatch", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].IntentClasses = []string{assistantRouteKindBusinessAction}
	})

	runCompileError("action view asset type invalid", "action view asset_type invalid", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, a *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*a)[0].AssetType = "bad"
	})
	runCompileError("action view action id required", "action view action_id required", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, a *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*a)[0].ActionID = " "
	})
	runCompileError("action view action not registered", "action view action_id not registered", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, a *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*a)[0].ActionID = "org.unknown"
	})
	runCompileError("action view locale invalid", "action view locale invalid", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, a *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*a)[0].Locale = "jp"
	})
	runCompileError("action view summary required", "action view summary required", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, a *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*a)[0].Summary = " "
	})
	runCompileError("action view source refs required", "action view source_refs required", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, a *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*a)[0].SourceRefs = nil
	})
	runCompileError("action view source refs invalid", "action view source_refs invalid", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, a *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*a)[0].SourceRefs = []string{"not/exist/path.md"}
	})
	runCompileError("action view template field not allowed", "template field not allowed", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, a *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*a)[0].TemplateFields = []string{"x.invalid"}
	})
	runCompileError("action view unknown guidance error", "unknown error_code", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, a *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*a)[0].MissingFieldGuidance = []assistantActionViewGuidance{{ErrorCode: "UNKNOWN", Text: "x"}}
	})
	runCompileError("action view duplicate locale", "duplicated action view", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, a *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		*a = append(*a, (*a)[0])
	})

	runCompileError("reply guidance asset type invalid", "reply guidance asset_type invalid", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].AssetType = "bad"
	})
	runCompileError("reply guidance kind required", "reply guidance reply_kind required", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].ReplyKind = " "
	})
	runCompileError("reply guidance knowledge version required", "reply guidance knowledge_version required", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].KnowledgeVersion = " "
	})
	runCompileError("reply guidance locale invalid", "reply guidance locale invalid", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].Locale = "jp"
	})
	runCompileError("reply guidance template required", "reply guidance templates required", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].GuidanceTemplates = nil
	})
	runCompileError("reply guidance template id required", "reply guidance template_id required", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].GuidanceTemplates = []assistantKnowledgePrompt{{TemplateID: " ", Text: "x"}}
	})
	runCompileError("reply guidance template text required", "reply guidance template text required", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].GuidanceTemplates = []assistantKnowledgePrompt{{TemplateID: "reply.missing_fields.v1", Text: " "}}
	})
	runCompileError("reply guidance multiple templates not allowed", "reply guidance multiple templates not allowed", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].GuidanceTemplates = []assistantKnowledgePrompt{
			{TemplateID: "reply.missing_fields.v1", Text: "x"},
			{TemplateID: "reply.missing_fields.v2", Text: "y"},
		}
	})
	runCompileError("reply guidance source refs required", "reply guidance source_refs required", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].SourceRefs = nil
	})
	runCompileError("reply guidance source refs invalid", "reply guidance source_refs invalid", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].SourceRefs = []string{"not/exist/path.md"}
	})
	runCompileError("reply guidance unknown error code", "unknown reply guidance error_code", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].ErrorCodes = []string{"UNKNOWN"}
	})
	runCompileError("reply guidance duplicate generic in locale", "ambiguous reply guidance selection", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*r)[0].ErrorCodes = nil
		*r = append(*r, (*r)[0])
	})
	runCompileError("reply guidance duplicated error code in locale", "ambiguous reply guidance selection", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		*r = append(*r, assistantReplyGuidancePack{
			AssetType:        "reply_guidance_pack",
			ReplyKind:        "missing_fields",
			KnowledgeVersion: "2026-03-11.v2",
			Locale:           "zh",
			GuidanceTemplates: []assistantKnowledgePrompt{
				{TemplateID: "reply.missing_fields.v2", Text: "x"},
			},
			ErrorCodes: []string{"missing_parent_ref_text"},
			SourceRefs: []string{"internal/server/assistant_knowledge_runtime.go"},
		})
	})
	runCompileError("missing create action view", "missing create_orgunit action view pack", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, a *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*a)[0].ActionID = assistantIntentRenameOrgUnit
	})
	runCompileError("missing default interpretation after compile", "missing knowledge.general_qa interpretation pack", func(c *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		c.Entries = []assistantIntentRouteEntry{
			{
				IntentID:  "org.orgunit_create",
				RouteKind: assistantRouteKindBusinessAction,
				ActionID:  assistantIntentCreateOrgUnit,
			},
		}
		(*i)[0].PackID = "org.orgunit_create"
		(*i)[0].IntentClasses = []string{assistantRouteKindBusinessAction}
		(*i)[0].ClarificationPrompts = []assistantKnowledgePrompt{{TemplateID: "clarify.org.orgunit_create.v1", Text: "x"}}
	})
	runCompileError("missing knowledge interpretation", "missing interpretation pack for non-business intent", func(_ *assistantIntentRouteCatalog, i *[]assistantInterpretationPack, _ *[]assistantActionViewPack, _ *[]assistantReplyGuidancePack, _ map[string][]byte) {
		(*i)[0].PackID = "knowledge.other"
	})
	runCompileError("missing reply guidance", "reply guidance packs missing", func(_ *assistantIntentRouteCatalog, _ *[]assistantInterpretationPack, _ *[]assistantActionViewPack, r *[]assistantReplyGuidancePack, _ map[string][]byte) {
		*r = nil
	})

	t.Run("snapshot digest empty", func(t *testing.T) {
		hooks := captureAssistantKnowledgeHooks()
		defer hooks.restore()
		assistantKnowledgeCanonicalHashFn = func(any) string { return "" }
		catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
		_, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
		if err == nil || !strings.Contains(err.Error(), "knowledge snapshot digest empty") {
			t.Fatalf("expected digest error, got=%v", err)
		}
	})

	t.Run("compile success", func(t *testing.T) {
		catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
		actionViews[0].TemplateFields = []string{"action_view_pack.summary", " "}
		actionViews[0].MissingFieldGuidance = append(actionViews[0].MissingFieldGuidance, assistantActionViewGuidance{ErrorCode: " ", Text: "ignored"})
		replyGuidance[0].ErrorCodes = append(replyGuidance[0].ErrorCodes, " ")
		replyGuidance = append(replyGuidance, assistantReplyGuidancePack{
			AssetType:        "reply_guidance_pack",
			ReplyKind:        "missing_fields",
			KnowledgeVersion: "2026-03-11.v2",
			Locale:           "en",
			GuidanceTemplates: []assistantKnowledgePrompt{
				{TemplateID: "reply.missing_fields.v1", Text: "Missing required fields: {missing_fields}"},
			},
			ErrorCodes: []string{"missing_parent_ref_text"},
			SourceRefs: []string{"internal/server/assistant_knowledge_runtime.go"},
		})
		runtime, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
		if err != nil {
			t.Fatalf("compile runtime err=%v", err)
		}
		if runtime == nil || strings.TrimSpace(runtime.SnapshotDigest) == "" {
			t.Fatalf("runtime invalid: %+v", runtime)
		}
		if runtime.ReplyGuidanceVersion != "2026-03-11.v1+2026-03-11.v2" {
			t.Fatalf("unexpected reply guidance version=%q", runtime.ReplyGuidanceVersion)
		}
	})
}

func TestAssistantKnowledgeRuntime_HelperCoverage(t *testing.T) {
	t.Run("validate forbidden keys", func(t *testing.T) {
		if err := assistantValidateForbiddenKeys(map[string][]byte{"empty.json": nil}); err != nil {
			t.Fatalf("expected nil err for empty raw, got=%v", err)
		}
		if err := assistantValidateForbiddenKeys(map[string][]byte{"bad.json": []byte(`{`)}); err == nil {
			t.Fatal("expected decode error")
		}
		if err := assistantValidateForbiddenKeys(map[string][]byte{"bad.md": []byte("not-front-matter")}); err == nil {
			t.Fatal("expected markdown decode error")
		}
		if err := assistantValidateForbiddenKeys(map[string][]byte{"bad.json": []byte(`{"confirm_conditions":["x"]}`)}); err == nil {
			t.Fatal("expected forbidden key error")
		}
		if err := assistantValidateForbiddenKeys(map[string][]byte{"ok.json": []byte(`{"a":[{"b":"c"}]}`)}); err != nil {
			t.Fatalf("expected nil err, got=%v", err)
		}
	})

	t.Run("find forbidden key", func(t *testing.T) {
		if key, ok := assistantFindForbiddenKey(map[string]any{"x": []any{map[string]any{"phase": "init"}}}); !ok || key != "phase" {
			t.Fatalf("key=%q ok=%v", key, ok)
		}
		if key, ok := assistantFindForbiddenKey([]any{map[string]any{"ok": true}}); ok || key != "" {
			t.Fatalf("key=%q ok=%v", key, ok)
		}
	})

	t.Run("route kind and locale validation", func(t *testing.T) {
		if !assistantValidRouteKind(assistantRouteKindBusinessAction) {
			t.Fatal("business action route kind should be valid")
		}
		if assistantValidRouteKind("bad") {
			t.Fatal("bad route kind should be invalid")
		}
		if !assistantValidLocale("zh") || !assistantValidLocale(" en ") {
			t.Fatal("zh/en should be valid locales")
		}
		if assistantValidLocale("jp") {
			t.Fatal("jp should be invalid locale")
		}
		promptActions := assistantOrderedPromptActionIDs()
		if len(promptActions) == 0 {
			t.Fatal("expected ordered prompt action ids")
		}
	})

	t.Run("source refs validation", func(t *testing.T) {
		if err := assistantValidateSourceRefs(nil); err == nil {
			t.Fatal("expected empty source refs error")
		}
		if err := assistantValidateSourceRefs([]string{""}); err == nil {
			t.Fatal("expected empty source ref item error")
		}
		if err := assistantValidateSourceRefs([]string{"not/exist/path.md"}); err == nil {
			t.Fatal("expected source ref not found error")
		}
		if err := assistantValidateSourceRefs([]string{"internal/server/assistant_knowledge_runtime.go"}); err != nil {
			t.Fatalf("expected valid source ref, got=%v", err)
		}
	})

	t.Run("repo path exists", func(t *testing.T) {
		if assistantRepoPathExists("") {
			t.Fatal("empty path should be false")
		}
		t.Run("direct stat true", func(t *testing.T) {
			hooks := captureAssistantKnowledgeHooks()
			defer hooks.restore()
			assistantKnowledgeRepoStatFn = func(path string) (os.FileInfo, error) {
				if path == "direct-exists" {
					return fakeFileInfo{}, nil
				}
				return nil, os.ErrNotExist
			}
			if !assistantRepoPathExists("direct-exists") {
				t.Fatal("direct stat success should return true")
			}
		})

		t.Run("reply guidance normalization helpers", func(t *testing.T) {
			if _, err := assistantNormalizeReplyGuidanceTemplates("missing_fields", "zh", nil); err == nil {
				t.Fatal("expected template required error")
			}
			if _, err := assistantNormalizeReplyGuidanceTemplates("missing_fields", "zh", []assistantKnowledgePrompt{{TemplateID: "x", Text: "a"}, {TemplateID: "y", Text: "b"}}); err == nil {
				t.Fatal("expected multiple template error")
			}
			if _, err := assistantNormalizeReplyGuidanceTemplates("missing_fields", "zh", []assistantKnowledgePrompt{{TemplateID: " ", Text: "a"}}); err == nil {
				t.Fatal("expected template id required error")
			}
			if _, err := assistantNormalizeReplyGuidanceTemplates("missing_fields", "zh", []assistantKnowledgePrompt{{TemplateID: "x", Text: " "}}); err == nil {
				t.Fatal("expected template text required error")
			}
			templates, err := assistantNormalizeReplyGuidanceTemplates("missing_fields", "zh", []assistantKnowledgePrompt{{TemplateID: " reply.missing_fields.v1 ", Text: "  请补充  "}})
			if err != nil || len(templates) != 1 || templates[0].TemplateID != "reply.missing_fields.v1" || templates[0].Text != "请补充" {
				t.Fatalf("unexpected templates=%+v err=%v", templates, err)
			}

			if _, err := assistantNormalizeReplyGuidanceErrorCodes([]string{"UNKNOWN"}); err == nil {
				t.Fatal("expected unknown error code")
			}
			if _, err := assistantNormalizeReplyGuidanceErrorCodes([]string{"missing_parent_ref_text", "missing_parent_ref_text"}); err == nil {
				t.Fatal("expected duplicate error code")
			}
			errorCodes, err := assistantNormalizeReplyGuidanceErrorCodes([]string{" missing_parent_ref_text ", " "})
			if err != nil || len(errorCodes) != 1 || errorCodes[0] != "missing_parent_ref_text" {
				t.Fatalf("unexpected error codes=%v err=%v", errorCodes, err)
			}

			normalized := assistantNormalizeOptionalTextList([]string{"  ", "明确", "明确", "简洁"})
			if len(normalized) != 2 || normalized[0] != "明确" || normalized[1] != "简洁" {
				t.Fatalf("unexpected optional text list=%v", normalized)
			}
			if out := assistantNormalizeOptionalTextList([]string{" ", "  "}); out != nil {
				t.Fatalf("expected nil for empty optional list, got=%v", out)
			}

			if assistantReplyGuidanceMatchErrorCode(assistantReplyGuidancePack{ErrorCodes: []string{"missing_parent_ref_text"}}, "missing_effective_date") {
				t.Fatal("unexpected matched error code")
			}
			if !assistantReplyGuidanceMatchErrorCode(assistantReplyGuidancePack{ErrorCodes: []string{"missing_parent_ref_text"}}, "missing_parent_ref_text") {
				t.Fatal("expected matched error code")
			}
			if !assistantReplyGuidanceIsGeneric(assistantReplyGuidancePack{}) {
				t.Fatal("expected empty error code pack to be generic")
			}
		})

		t.Run("caller unavailable", func(t *testing.T) {
			hooks := captureAssistantKnowledgeHooks()
			defer hooks.restore()
			assistantKnowledgeRepoStatFn = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
			assistantKnowledgeRuntimeCallerFn = func(int) (uintptr, string, int, bool) { return 0, "", 0, false }
			if assistantRepoPathExists("some/path") {
				t.Fatal("expected false when caller unavailable")
			}
		})

		t.Run("fallback path exists", func(t *testing.T) {
			hooks := captureAssistantKnowledgeHooks()
			defer hooks.restore()
			call := 0
			assistantKnowledgeRepoStatFn = func(string) (os.FileInfo, error) {
				call++
				if call == 1 {
					return nil, os.ErrNotExist
				}
				return fakeFileInfo{}, nil
			}
			assistantKnowledgeRuntimeCallerFn = func(int) (uintptr, string, int, bool) {
				return 0, "/tmp/runtime/file.go", 1, true
			}
			if !assistantRepoPathExists("docs/dev-plans/009-implementation-roadmap.md") {
				t.Fatal("expected fallback path true")
			}
		})
	})

	t.Run("locale candidates and finders", func(t *testing.T) {
		runtime := &assistantKnowledgeRuntime{
			intentDocs: map[string]map[string]assistantKnowledgeMarkdownDocument{
				"org.orgunit_create": {
					"zh": {ID: "intent.zh", Locale: "zh"},
					"en": {ID: "intent.en", Locale: "en"},
				},
			},
			actionDocsByAction: map[string]map[string]assistantKnowledgeMarkdownDocument{
				assistantIntentCreateOrgUnit: {
					"zh": {ID: "action.zh", Locale: "zh"},
				},
			},
			actionDocsByIntent: map[string]map[string]assistantKnowledgeMarkdownDocument{
				"org.orgunit_create": {
					"en": {ID: "action-intent.en", Locale: "en"},
				},
			},
			actionView: map[string]map[string]assistantActionViewPack{
				assistantIntentCreateOrgUnit: {
					"zh": {Summary: "中文摘要"},
					"en": {Summary: "English summary"},
				},
			},
			interpretation: map[string]map[string]assistantInterpretationPack{
				"knowledge.general_qa": {
					"zh": {PackID: "knowledge.general_qa", Locale: "zh"},
					"en": {PackID: "knowledge.general_qa", Locale: "en"},
				},
			},
			replyGuidance: map[string]map[string][]assistantReplyGuidancePack{
				"missing_fields": {
					"zh": {
						{
							ReplyKind:         "missing_fields",
							Locale:            "zh",
							GuidanceTemplates: []assistantKnowledgePrompt{{TemplateID: "reply.missing_fields.zh.v1", Text: "请补充：{missing_fields}"}},
						},
						{
							ReplyKind:         "missing_fields",
							Locale:            "zh",
							GuidanceTemplates: []assistantKnowledgePrompt{{TemplateID: "reply.missing_fields.zh.error.v1", Text: "缺少上级组织"}},
							ErrorCodes:        []string{"missing_parent_ref_text"},
						},
					},
					"en": {
						{
							ReplyKind:         "missing_fields",
							Locale:            "en",
							GuidanceTemplates: []assistantKnowledgePrompt{{TemplateID: "reply.missing_fields.en.v1", Text: "Missing: {missing_fields}"}},
						},
					},
				},
			},
		}
		candidates := runtime.localeCandidates("en")
		if len(candidates) != 2 || candidates[0] != "en" || candidates[1] != "zh" {
			t.Fatalf("unexpected locale candidates=%v", candidates)
		}
		defaultCandidates := runtime.localeCandidates("")
		if len(defaultCandidates) != 2 || defaultCandidates[0] != "zh" || defaultCandidates[1] != "en" {
			t.Fatalf("unexpected default locale candidates=%v", defaultCandidates)
		}
		if got, ok := (*assistantKnowledgeRuntime)(nil).findIntentDoc("org.orgunit_create", "zh"); ok || got.ID != "" {
			t.Fatalf("nil runtime should not find intent doc, got=%+v ok=%v", got, ok)
		}
		if _, ok := runtime.findIntentDoc("unknown.intent", "zh"); ok {
			t.Fatal("unknown intent doc should not be found")
		}
		if got, ok := runtime.findIntentDoc("org.orgunit_create", "fr"); !ok || got.ID != "intent.zh" {
			t.Fatalf("expected zh fallback intent doc, got=%+v ok=%v", got, ok)
		}
		runtime.intentDocs["only.ja"] = map[string]assistantKnowledgeMarkdownDocument{"ja": {ID: "intent.ja", Locale: "ja"}}
		if _, ok := runtime.findIntentDoc("only.ja", "zh"); ok {
			t.Fatal("locale not matched intent doc should return false")
		}
		if got, ok := (*assistantKnowledgeRuntime)(nil).findActionDocByAction(assistantIntentCreateOrgUnit, "zh"); ok || got.ID != "" {
			t.Fatalf("nil runtime should not find action doc by action, got=%+v ok=%v", got, ok)
		}
		if _, ok := runtime.findActionDocByAction("unknown.action", "zh"); ok {
			t.Fatal("unknown action doc should not be found")
		}
		if got, ok := runtime.findActionDocByAction(assistantIntentCreateOrgUnit, "fr"); !ok || got.ID != "action.zh" {
			t.Fatalf("expected zh fallback action doc, got=%+v ok=%v", got, ok)
		}
		runtime.actionDocsByAction["only.ja"] = map[string]assistantKnowledgeMarkdownDocument{"ja": {ID: "action.ja", Locale: "ja"}}
		if _, ok := runtime.findActionDocByAction("only.ja", "zh"); ok {
			t.Fatal("locale not matched action doc should return false")
		}
		if got, ok := (*assistantKnowledgeRuntime)(nil).findActionDocByIntent("org.orgunit_create", "zh"); ok || got.ID != "" {
			t.Fatalf("nil runtime should not find action doc by intent, got=%+v ok=%v", got, ok)
		}
		if _, ok := runtime.findActionDocByIntent("unknown.intent", "zh"); ok {
			t.Fatal("unknown action doc by intent should not be found")
		}
		if got, ok := runtime.findActionDocByIntent("org.orgunit_create", "fr"); !ok || got.ID != "action-intent.en" {
			t.Fatalf("expected locale fallback action doc by intent, got=%+v ok=%v", got, ok)
		}
		runtime.actionDocsByIntent["only.ja"] = map[string]assistantKnowledgeMarkdownDocument{"ja": {ID: "action-intent.ja", Locale: "ja"}}
		if _, ok := runtime.findActionDocByIntent("only.ja", "zh"); ok {
			t.Fatal("locale not matched action doc by intent should return false")
		}
		if got, ok := (*assistantKnowledgeRuntime)(nil).findActionView(assistantIntentCreateOrgUnit, "zh"); ok || got.Summary != "" {
			t.Fatalf("nil runtime should not find action view, got=%+v ok=%v", got, ok)
		}
		if _, ok := runtime.findActionView("unknown.action", "zh"); ok {
			t.Fatal("unknown action should not be found")
		}
		if got, ok := runtime.findActionView(assistantIntentCreateOrgUnit, "fr"); !ok || got.Summary != "中文摘要" {
			t.Fatalf("expected zh fallback action view, got=%+v ok=%v", got, ok)
		}
		runtime.actionView["only.ja"] = map[string]assistantActionViewPack{"ja": {Summary: "ja"}}
		if _, ok := runtime.findActionView("only.ja", "zh"); ok {
			t.Fatal("locale not matched should return false")
		}
		if got, ok := (*assistantKnowledgeRuntime)(nil).findInterpretation("knowledge.general_qa", "zh"); ok || got.PackID != "" {
			t.Fatalf("nil runtime should not find interpretation, got=%+v ok=%v", got, ok)
		}
		if _, ok := runtime.findInterpretation("unknown.pack", "zh"); ok {
			t.Fatal("unknown interpretation should not be found")
		}
		if got, ok := runtime.findInterpretation("knowledge.general_qa", "fr"); !ok || got.Locale != "zh" {
			t.Fatalf("expected zh fallback interpretation, got=%+v ok=%v", got, ok)
		}
		runtime.interpretation["only.ja"] = map[string]assistantInterpretationPack{"ja": {PackID: "only.ja", Locale: "ja"}}
		if _, ok := runtime.findInterpretation("only.ja", "zh"); ok {
			t.Fatal("locale not matched should return false")
		}
		if got, ok := (*assistantKnowledgeRuntime)(nil).findReplyGuidance("missing_fields", "zh", "missing_parent_ref_text"); ok || got.ReplyKind != "" {
			t.Fatalf("nil runtime should not find reply guidance, got=%+v ok=%v", got, ok)
		}
		if _, ok := runtime.findReplyGuidance("unknown_kind", "zh", ""); ok {
			t.Fatal("unknown reply kind should not be found")
		}
		if got, ok := runtime.findReplyGuidance("missing_fields", "zh", "missing_parent_ref_text"); !ok || got.GuidanceTemplates[0].TemplateID != "reply.missing_fields.zh.error.v1" {
			t.Fatalf("expected exact error code guidance, got=%+v ok=%v", got, ok)
		}
		if got, ok := runtime.findReplyGuidance("missing_fields", "fr", "missing_unknown"); !ok || got.Locale != "zh" || got.GuidanceTemplates[0].TemplateID != "reply.missing_fields.zh.v1" {
			t.Fatalf("expected locale fallback to zh generic pack, got=%+v ok=%v", got, ok)
		}
		if got, ok := runtime.findReplyGuidance("missing_fields", "en", ""); !ok || got.Locale != "en" {
			t.Fatalf("expected en generic pack, got=%+v ok=%v", got, ok)
		}
		runtime.replyGuidance["only_ja"] = map[string][]assistantReplyGuidancePack{
			"ja": {{ReplyKind: "only_ja", Locale: "ja", GuidanceTemplates: []assistantKnowledgePrompt{{TemplateID: "reply.only_ja.ja.v1", Text: "ja"}}}},
		}
		if _, ok := runtime.findReplyGuidance("only_ja", "zh", ""); ok {
			t.Fatal("locale not matched reply guidance should return false")
		}
	})

	t.Run("route intent", func(t *testing.T) {
		runtime := &assistantKnowledgeRuntime{
			RouteCatalogVersion: "v1",
			routeByAction: map[string]assistantIntentRouteEntry{
				assistantIntentCreateOrgUnit: {
					IntentID:  "org.orgunit_create",
					RouteKind: assistantRouteKindBusinessAction,
				},
			},
			routeCatalog: assistantIntentRouteCatalog{
				Entries: []assistantIntentRouteEntry{
					{IntentID: "org.orgunit_create", RouteKind: assistantRouteKindBusinessAction, ActionID: assistantIntentCreateOrgUnit},
					{IntentID: "knowledge.general_qa", RouteKind: assistantRouteKindKnowledgeQA, Keywords: []string{"功能", "help", " "}},
					{IntentID: "route.chitchat", RouteKind: assistantRouteKindChitchat, Keywords: []string{"你好"}},
				},
			},
		}
		mapped := runtime.routeIntent("创建部门", assistantIntentSpec{Action: assistantIntentCreateOrgUnit})
		if mapped.IntentID != "org.orgunit_create" || mapped.RouteKind != assistantRouteKindBusinessAction {
			t.Fatalf("mapped route invalid: %+v", mapped)
		}
		unmapped := runtime.routeIntent("执行动作", assistantIntentSpec{Action: assistantIntentRenameOrgUnit})
		if unmapped.IntentID != "action."+assistantIntentRenameOrgUnit || unmapped.RouteKind != assistantRouteKindBusinessAction {
			t.Fatalf("unmapped action route invalid: %+v", unmapped)
		}
		qa := runtime.routeIntent("系统有哪些功能", assistantIntentSpec{Action: assistantIntentPlanOnly})
		if qa.RouteKind != assistantRouteKindKnowledgeQA {
			t.Fatalf("qa route invalid: %+v", qa)
		}
		uncertain := runtime.routeIntent("随机输入", assistantIntentSpec{Action: assistantIntentPlanOnly})
		if uncertain.RouteKind != assistantRouteKindUncertain || uncertain.IntentID != "route.uncertain" {
			t.Fatalf("uncertain route invalid: %+v", uncertain)
		}
		if entry, ok := (*assistantKnowledgeRuntime)(nil).findRouteByRouteKind(assistantRouteKindKnowledgeQA); ok || entry.IntentID != "" {
			t.Fatalf("nil runtime route=%+v ok=%v", entry, ok)
		}
		if entry, ok := runtime.findRouteByRouteKind(""); ok || entry.IntentID != "" {
			t.Fatalf("empty route kind route=%+v ok=%v", entry, ok)
		}
		if entry, ok := runtime.findRouteByRouteKind(assistantRouteKindKnowledgeQA); !ok || entry.IntentID != "knowledge.general_qa" {
			t.Fatalf("knowledge route=%+v ok=%v", entry, ok)
		}
		if entry, ok := runtime.findRouteByRouteKind(assistantRouteKindUncertain); !ok || entry.RouteKind != assistantRouteKindUncertain {
			t.Fatalf("uncertain route lookup=%+v ok=%v", entry, ok)
		}
		if entry, ok := runtime.findRouteByRouteKind("missing"); ok || entry.IntentID != "" {
			t.Fatalf("missing route=%+v ok=%v", entry, ok)
		}
		if entry, ok := runtime.findRouteByRouteKind(assistantRouteKindBusinessAction); ok || entry.IntentID != "" {
			t.Fatalf("business action route without standalone entry should fail, got=%+v ok=%v", entry, ok)
		}
	})

	t.Run("interpretation pack helpers", func(t *testing.T) {
		if packID := assistantResolveInterpretationPackIDForIntent("unknown.intent", assistantRouteKindKnowledgeQA, map[string]map[string]assistantInterpretationPack{
			assistantInterpretationDefaultPackID: {
				"zh": {PackID: assistantInterpretationDefaultPackID, IntentClasses: []string{assistantRouteKindKnowledgeQA}},
			},
		}); packID != assistantInterpretationDefaultPackID {
			t.Fatalf("expected default interpretation pack, got=%q", packID)
		}

		runtime := &assistantKnowledgeRuntime{
			routePackID: map[string]string{},
			interpretation: map[string]map[string]assistantInterpretationPack{
				assistantInterpretationDefaultPackID: {
					"zh": {PackID: assistantInterpretationDefaultPackID, IntentClasses: []string{assistantRouteKindKnowledgeQA}},
				},
			},
		}
		if packID := runtime.resolveInterpretationPackID("unknown.intent", assistantRouteKindKnowledgeQA); packID != assistantInterpretationDefaultPackID {
			t.Fatalf("expected runtime fallback interpretation pack, got=%q", packID)
		}
	})

	t.Run("build plan context", func(t *testing.T) {
		spec := assistantActionSpec{ID: assistantIntentCreateOrgUnit, PlanSummary: "默认摘要", PlanTitle: "默认标题"}
		if _, err := (*assistantKnowledgeRuntime)(nil).buildPlanContextV1("tenant_1", "zh", assistantIntentSpec{}, spec, nil); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
			t.Fatalf("expected runtime missing error, got=%v", err)
		}

		nonBusinessRuntime := &assistantKnowledgeRuntime{
			intentDocs: map[string]map[string]assistantKnowledgeMarkdownDocument{
				"knowledge.general_qa": {
					"zh": {ID: "knowledge.general_qa", Locale: "zh", Summary: "这是知识问答摘要"},
				},
			},
		}
		ctx, err := nonBusinessRuntime.buildPlanContextV1("tenant_1", "zh", assistantIntentSpec{
			Action:    assistantIntentPlanOnly,
			IntentID:  "knowledge.general_qa",
			RouteKind: assistantRouteKindKnowledgeQA,
		}, assistantActionSpec{}, nil)
		if err != nil {
			t.Fatalf("build non-business context err=%v", err)
		}
		if !strings.Contains(ctx.ActionViewSummary, "知识问答") {
			t.Fatalf("unexpected non-business summary=%q", ctx.ActionViewSummary)
		}

		emptyRuntime := &assistantKnowledgeRuntime{}
		emptyCtx, err := emptyRuntime.buildPlanContextV1("tenant_1", "zh", assistantIntentSpec{
			Action:    assistantIntentPlanOnly,
			IntentID:  "knowledge.unknown",
			RouteKind: assistantRouteKindKnowledgeQA,
		}, assistantActionSpec{}, nil)
		if err != nil {
			t.Fatalf("unexpected non-business fallback err=%v", err)
		}
		if !strings.Contains(emptyCtx.ActionViewSummary, "非动作请求") {
			t.Fatalf("unexpected fallback non-business summary=%q", emptyCtx.ActionViewSummary)
		}

		businessRuntime := &assistantKnowledgeRuntime{
			actionView: map[string]map[string]assistantActionViewPack{
				assistantIntentCreateOrgUnit: {
					"zh": {
						Summary:              "创建组织摘要",
						FieldDisplayMap:      []assistantActionViewField{{Field: "parent_ref_text", Label: "上级组织"}},
						MissingFieldGuidance: []assistantActionViewGuidance{{ErrorCode: "missing_parent_ref_text", Text: "请补充上级组织"}},
					},
				},
			},
		}
		actionCtx, err := businessRuntime.buildPlanContextV1("tenant_1", "zh", assistantIntentSpec{
			Action: assistantIntentCreateOrgUnit,
		}, spec, &assistantTurn{Phase: assistantPhaseAwaitMissingFields})
		if err != nil {
			t.Fatalf("build business context err=%v", err)
		}
		if actionCtx.ActionViewSummary != "创建组织摘要" || len(actionCtx.FieldDisplayMap) == 0 {
			t.Fatalf("unexpected action context=%+v", actionCtx)
		}

		fallbackCtx, err := (&assistantKnowledgeRuntime{actionView: map[string]map[string]assistantActionViewPack{}}).buildPlanContextV1("tenant_1", "zh", assistantIntentSpec{
			Action: assistantIntentRenameOrgUnit,
		}, assistantActionSpec{PlanSummary: "重命名摘要"}, nil)
		if err != nil {
			t.Fatalf("expected non-create fallback summary err=nil, got=%v", err)
		}
		if fallbackCtx.ActionViewSummary != "重命名摘要" {
			t.Fatalf("unexpected fallback summary=%q", fallbackCtx.ActionViewSummary)
		}

		if _, err := (&assistantKnowledgeRuntime{actionView: map[string]map[string]assistantActionViewPack{}}).buildPlanContextV1("tenant_1", "zh", assistantIntentSpec{
			Action: assistantIntentCreateOrgUnit,
		}, spec, nil); err == nil {
			t.Fatal("expected create action view missing error")
		}
	})

	t.Run("guidance and apply plan context", func(t *testing.T) {
		if got := assistantKnowledgeGuidanceText(assistantPlanContextV1{}, nil); got != "" {
			t.Fatalf("expected empty guidance, got=%q", got)
		}
		context := assistantPlanContextV1{
			ActionViewSummary: "动作摘要",
			MissingFieldGuidance: []assistantActionViewGuidance{
				{ErrorCode: "missing_parent_ref_text", Text: "请补充上级组织"},
				{ErrorCode: "", Text: "ignored"},
			},
		}
		if got := assistantKnowledgeGuidanceText(context, []string{"missing_parent_ref_text"}); got != "请补充上级组织" {
			t.Fatalf("unexpected guidance=%q", got)
		}
		if got := assistantKnowledgeGuidanceText(context, []string{"other"}); got != "" {
			t.Fatalf("unexpected guidance=%q", got)
		}

		assistantApplyPlanContextV1(nil, nil, assistantIntentSpec{}, context)

		plan := assistantPlanSummary{Summary: "old"}
		dryRun := assistantDryRunResult{ValidationErrors: []string{"missing_parent_ref_text"}, Explain: "old"}
		assistantApplyPlanContextV1(&plan, &dryRun, assistantIntentSpec{RouteKind: assistantRouteKindBusinessAction}, context)
		if plan.Summary != "动作摘要" {
			t.Fatalf("unexpected plan summary=%q", plan.Summary)
		}
		if dryRun.Explain != "请补充上级组织" {
			t.Fatalf("unexpected dryrun explain=%q", dryRun.Explain)
		}

		nonBusinessDryRun := assistantDryRunResult{Explain: "old"}
		assistantApplyPlanContextV1(&plan, &nonBusinessDryRun, assistantIntentSpec{RouteKind: assistantRouteKindKnowledgeQA}, assistantPlanContextV1{})
		if !assistantContainsString(nonBusinessDryRun.ValidationErrors, "non_business_route") {
			t.Fatalf("expected non_business_route validation, got=%v", nonBusinessDryRun.ValidationErrors)
		}
		if !strings.Contains(nonBusinessDryRun.Explain, "不会触发业务提交") {
			t.Fatalf("unexpected non-business explain=%q", nonBusinessDryRun.Explain)
		}
	})

	t.Run("plan context locale and ensure runtime", func(t *testing.T) {
		runtime := &assistantKnowledgeRuntime{}
		if runtime.planContextLocale() != "zh" {
			t.Fatalf("unexpected locale=%q", runtime.planContextLocale())
		}

		var nilSvc *assistantConversationService
		if _, err := nilSvc.ensureKnowledgeRuntime(); !errors.Is(err, errAssistantServiceMissing) {
			t.Fatalf("expected service missing, got=%v", err)
		}

		loadedRuntime := &assistantKnowledgeRuntime{SnapshotDigest: "digest"}
		svc := &assistantConversationService{knowledgeRuntime: loadedRuntime}
		got, err := svc.ensureKnowledgeRuntime()
		if err != nil || got != loadedRuntime {
			t.Fatalf("expected cached runtime, got=%+v err=%v", got, err)
		}

		loadErrSvc := &assistantConversationService{knowledgeErr: errAssistantRuntimeConfigInvalid}
		if _, err := loadErrSvc.ensureKnowledgeRuntime(); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
			t.Fatalf("expected cached load err, got=%v", err)
		}

		t.Run("load runtime failed and cached", func(t *testing.T) {
			hooks := captureAssistantKnowledgeHooks()
			defer hooks.restore()
			assistantLoadKnowledgeRuntimeFn = func() (*assistantKnowledgeRuntime, error) {
				return nil, errAssistantRuntimeConfigInvalid
			}
			emptySvc := &assistantConversationService{}
			if _, err := emptySvc.ensureKnowledgeRuntime(); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
				t.Fatalf("expected runtime config invalid, got=%v", err)
			}
			if !errors.Is(emptySvc.knowledgeErr, errAssistantRuntimeConfigInvalid) {
				t.Fatalf("knowledgeErr should be cached, got=%v", emptySvc.knowledgeErr)
			}
		})

		t.Run("load runtime success", func(t *testing.T) {
			hooks := captureAssistantKnowledgeHooks()
			defer hooks.restore()
			expected := &assistantKnowledgeRuntime{SnapshotDigest: "from_loader"}
			assistantLoadKnowledgeRuntimeFn = func() (*assistantKnowledgeRuntime, error) {
				return expected, nil
			}
			emptySvc := &assistantConversationService{}
			gotRuntime, err := emptySvc.ensureKnowledgeRuntime()
			if err != nil || gotRuntime != expected {
				t.Fatalf("expected loaded runtime, got=%+v err=%v", gotRuntime, err)
			}
			if emptySvc.knowledgeRuntime != expected {
				t.Fatalf("knowledgeRuntime should be cached")
			}
		})
	})
}

func TestAssistantKnowledgeRuntime_SnapshotDigestCarriesVersionSet(t *testing.T) {
	hooks := captureAssistantKnowledgeHooks()
	defer hooks.restore()
	catalog, interpretation, actionViews, replyGuidance, rawByPath := assistantKnowledgeBaseCompileInput()
	captured := map[string]any{}
	assistantKnowledgeCanonicalHashFn = func(value any) string {
		payload, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("unexpected digest payload type=%T", value)
		}
		captured = payload
		return "sha256:test"
	}
	runtime, err := assistantCompileKnowledgeRuntime(catalog, interpretation, actionViews, replyGuidance, rawByPath)
	if err != nil {
		t.Fatalf("compile knowledge runtime err=%v", err)
	}
	if runtime == nil || strings.TrimSpace(runtime.SnapshotDigest) == "" {
		t.Fatal("expected runtime snapshot digest")
	}
	if got := strings.TrimSpace(captured["route_catalog_version"].(string)); got != strings.TrimSpace(catalog.RouteCatalogVersion) {
		t.Fatalf("unexpected route catalog version=%q", got)
	}
	if got := strings.TrimSpace(captured["resolver_contract_version"].(string)); got != assistantResolverContractVersionV1 {
		t.Fatalf("unexpected resolver contract version=%q", got)
	}
	if got := strings.TrimSpace(captured["context_template_version"].(string)); got != assistantContextTemplateVersionV1 {
		t.Fatalf("unexpected context template version=%q", got)
	}
	if got := strings.TrimSpace(captured["reply_guidance_version"].(string)); got != "2026-03-11.v1" {
		t.Fatalf("unexpected reply guidance version=%q", got)
	}
}
