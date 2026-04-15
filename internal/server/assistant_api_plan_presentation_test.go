package server

import (
	"errors"
	"testing"
)

func TestAssistantKnowledgePlanPresentation(t *testing.T) {
	hooks := captureAssistantKnowledgeHooks()
	defer hooks.restore()

	intent := assistantIntentSpec{Action: assistantIntentCreateOrgUnit}

	t.Run("runtime load error returns empty", func(t *testing.T) {
		assistantLoadKnowledgeRuntimeFn = func() (*assistantKnowledgeRuntime, error) {
			return nil, errors.New("runtime unavailable")
		}
		title, summary := assistantKnowledgePlanPresentation(intent)
		if title != "" || summary != "" {
			t.Fatalf("expected empty presentation, got title=%q summary=%q", title, summary)
		}
	})

	t.Run("nil runtime returns empty", func(t *testing.T) {
		assistantLoadKnowledgeRuntimeFn = func() (*assistantKnowledgeRuntime, error) {
			return nil, nil
		}
		title, summary := assistantKnowledgePlanPresentation(intent)
		if title != "" || summary != "" {
			t.Fatalf("expected empty presentation, got title=%q summary=%q", title, summary)
		}
	})

	t.Run("runtime presentation success trims whitespace", func(t *testing.T) {
		assistantLoadKnowledgeRuntimeFn = func() (*assistantKnowledgeRuntime, error) {
			return &assistantKnowledgeRuntime{
				actionView: map[string]map[string]assistantActionViewPack{
					assistantIntentCreateOrgUnit: {
						"zh": {
							AssetType:        "action_view_pack",
							ActionID:         assistantIntentCreateOrgUnit,
							KnowledgeVersion: "v1",
							Locale:           "zh",
							Summary:          " 生成组织草案 ",
							SourceRefs:       []string{"docs/dev-plans/380a-cubebox-postgresql-data-plane-and-migration-contract.md"},
						},
					},
				},
				actionDocsByAction: map[string]map[string]assistantKnowledgeMarkdownDocument{
					assistantIntentCreateOrgUnit: {
						"zh": {Title: " 创建组织 "},
					},
				},
			}, nil
		}
		title, summary := assistantKnowledgePlanPresentation(intent)
		if title != "创建组织" || summary != "生成组织草案" {
			t.Fatalf("unexpected presentation title=%q summary=%q", title, summary)
		}
	})

	t.Run("runtime presentation resolve error returns empty", func(t *testing.T) {
		assistantLoadKnowledgeRuntimeFn = func() (*assistantKnowledgeRuntime, error) {
			return &assistantKnowledgeRuntime{}, nil
		}
		title, summary := assistantKnowledgePlanPresentation(intent)
		if title != "" || summary != "" {
			t.Fatalf("expected empty presentation, got title=%q summary=%q", title, summary)
		}
	})
}
