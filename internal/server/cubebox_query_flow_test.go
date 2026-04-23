package server

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

func TestNormalizeCubeboxReadPlanPromotesOrgUnitListClarificationToExecutableDefault(t *testing.T) {
	plan := normalizeCubeboxReadPlan(cubebox.ReadPlan{
		Intent:             "orgunit.list",
		Confidence:         0.41,
		MissingParams:      []string{"as_of", "parent_org_code"},
		ClarifyingQuestion: "请告诉我要按哪一天查询组织树，以及是查一级组织还是某个 parent_org_code 下面的子组织。",
	}, "查询组织树", time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC))

	if len(plan.MissingParams) != 0 {
		t.Fatalf("expected missing params cleared, got %+v", plan.MissingParams)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected one step, got %+v", plan.Steps)
	}
	step := plan.Steps[0]
	if step.APIKey != "orgunit.list" {
		t.Fatalf("unexpected api key=%q", step.APIKey)
	}
	if got := step.Params["as_of"]; got != "2026-04-23" {
		t.Fatalf("expected as_of defaulted, got %#v", got)
	}
	if got := step.Params["include_disabled"]; got != false {
		t.Fatalf("expected include_disabled defaulted false, got %#v", got)
	}
	if _, ok := step.Params["parent_org_code"]; ok {
		t.Fatalf("expected root list query without parent_org_code, got %+v", step.Params)
	}
	if plan.ClarifyingQuestion != "" {
		t.Fatalf("expected clarifying question cleared, got %q", plan.ClarifyingQuestion)
	}
	if !strings.Contains(strings.Join(plan.ExplainFocus, " "), "一级组织") {
		t.Fatalf("expected explain focus to mention 一级组织, got %+v", plan.ExplainFocus)
	}
}

func TestNormalizeCubeboxReadPlanKeepsNonListClarification(t *testing.T) {
	plan := normalizeCubeboxReadPlan(cubebox.ReadPlan{
		Intent:             "orgunit.details",
		Confidence:         0.41,
		MissingParams:      []string{"org_code", "as_of"},
		ClarifyingQuestion: "请提供组织编码和查询日期。",
	}, "查一下这个组织详情", time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC))

	if len(plan.MissingParams) != 2 {
		t.Fatalf("expected missing params kept, got %+v", plan.MissingParams)
	}
	if len(plan.Steps) != 0 {
		t.Fatalf("expected no executable steps, got %+v", plan.Steps)
	}
	if plan.ClarifyingQuestion != "请提供组织编码和查询日期。" {
		t.Fatalf("unexpected clarifying question=%q", plan.ClarifyingQuestion)
	}
}

func TestNormalizeCubeboxReadPlanKeepsChildrenQueryAsClarification(t *testing.T) {
	plan := normalizeCubeboxReadPlan(cubebox.ReadPlan{
		Intent:             "orgunit.list",
		Confidence:         0.41,
		MissingParams:      []string{"as_of", "parent_org_code"},
		ClarifyingQuestion: "请提供 parent_org_code。",
	}, "看华东事业部下面的子组织", time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC))

	if len(plan.Steps) != 0 {
		t.Fatalf("expected children query to remain clarifying, got %+v", plan.Steps)
	}
	if len(plan.MissingParams) != 2 {
		t.Fatalf("expected missing params kept, got %+v", plan.MissingParams)
	}
	if !strings.Contains(plan.ClarifyingQuestion, "上级组织的组织编码") {
		t.Fatalf("expected humanized clarification, got %q", plan.ClarifyingQuestion)
	}
}

func TestBuildCubeboxQueryAnswerRendersOrgUnitListPayloadViaSummaryRenderer(t *testing.T) {
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:   "orgunit.list",
		Executor: queryExecutorStub{},
		SummaryRenderer: func(result cubebox.ExecuteResult) []string {
			return summarizeOrgUnitListQueryResult(result.Payload)
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	answer := buildCubeboxQueryAnswer(
		cubebox.ReadPlan{
			ExplainFocus: []string{"按 2026-04-23 的组织列表（默认不含停用组织）"},
		},
		[]cubebox.ExecuteResult{
			{
				APIKey:      "orgunit.list",
				StepID:      "step-1",
				ResultFocus: []string{"as_of", "include_disabled"},
				Payload: map[string]any{
					"as_of":            "2026-04-23",
					"include_disabled": false,
					"org_units": []orgUnitListItem{
						{
							OrgCode:        "1001",
							Name:           "总部",
							Status:         "active",
							IsBusinessUnit: ptrBool(true),
							HasChildren:    ptrBool(false),
						},
					},
				},
			},
		},
		registry,
	)

	for _, expected := range []string{
		"已完成只读查询。",
		"step-1（orgunit.list）",
		"组织列表：1 条",
		"- 1001 | 总部 | 状态：active | 业务单元：是 | 有下级：否",
	} {
		if !strings.Contains(answer, expected) {
			t.Fatalf("expected %q in answer=%s", expected, answer)
		}
	}
}

func TestBuildCubeboxQueryAnswerLimitsListSummaryAndAddsMoreNotice(t *testing.T) {
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:   "orgunit.list",
		Executor: queryExecutorStub{},
		SummaryRenderer: func(result cubebox.ExecuteResult) []string {
			return summarizeOrgUnitListQueryResult(result.Payload)
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	items := make([]orgUnitListItem, 0, cubeboxQueryListSummaryMaxItems+2)
	for i := 0; i < cubeboxQueryListSummaryMaxItems+2; i++ {
		items = append(items, orgUnitListItem{
			OrgCode:        "10" + strings.Repeat("0", 2) + string(rune('A'+i)),
			Name:           "组织" + string(rune('A'+i)),
			Status:         "active",
			IsBusinessUnit: ptrBool(false),
			HasChildren:    ptrBool(false),
		})
	}

	answer := buildCubeboxQueryAnswer(
		cubebox.ReadPlan{},
		[]cubebox.ExecuteResult{
			{
				APIKey: "orgunit.list",
				StepID: "step-1",
				Payload: map[string]any{
					"as_of":            "2026-04-23",
					"include_disabled": false,
					"total":            len(items),
					"org_units":        items,
				},
			},
		},
		registry,
	)

	if count := strings.Count(answer, "- "); count != cubeboxQueryListSummaryMaxItems {
		t.Fatalf("expected %d visible items, got answer=%s", cubeboxQueryListSummaryMaxItems, answer)
	}
	if !strings.Contains(answer, cubeboxQuerySummaryFallbackListNotice) {
		t.Fatalf("expected more notice in answer=%s", answer)
	}
}

func TestBuildCubeboxQueryAnswerFallsBackToResultFocusWhenSummaryRendererMissing(t *testing.T) {
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:   "orgunit.details",
		Executor: queryExecutorStub{},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	answer := buildCubeboxQueryAnswer(
		cubebox.ReadPlan{},
		[]cubebox.ExecuteResult{
			{
				APIKey:      "orgunit.details",
				StepID:      "step-1",
				ResultFocus: []string{"org_unit.name", "org_unit.status"},
				Payload: map[string]any{
					"org_unit": map[string]any{
						"name":   "总部",
						"status": "active",
					},
				},
			},
		},
		registry,
	)

	for _, expected := range []string{
		"org_unit.name：总部",
		"org_unit.status：active",
	} {
		if !strings.Contains(answer, expected) {
			t.Fatalf("expected %q in answer=%s", expected, answer)
		}
	}
}

func TestBuildCubeboxQueryAnswerDoesNotDumpLargePayloadWhenFocusMissing(t *testing.T) {
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:   "orgunit.details",
		Executor: queryExecutorStub{},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	secret := "raw-payload-should-not-appear"
	answer := buildCubeboxQueryAnswer(
		cubebox.ReadPlan{},
		[]cubebox.ExecuteResult{
			{
				APIKey:      "orgunit.details",
				StepID:      "step-1",
				ResultFocus: []string{"org_unit.name"},
				Payload: map[string]any{
					"as_of":        "2026-04-23",
					"total":        99,
					"large_object": strings.Repeat(secret, 50),
				},
			},
		},
		registry,
	)

	for _, expected := range []string{"as_of：2026-04-23", "total：99"} {
		if !strings.Contains(answer, expected) {
			t.Fatalf("expected %q in answer=%s", expected, answer)
		}
	}
	if strings.Contains(answer, secret) || strings.Contains(answer, "large_object") {
		t.Fatalf("expected large payload to be omitted, answer=%s", answer)
	}
	if utf8.RuneCountInString(answer) > cubeboxQueryAnswerMaxChars {
		t.Fatalf("answer over budget: %d > %d", utf8.RuneCountInString(answer), cubeboxQueryAnswerMaxChars)
	}
}

func TestBuildCubeboxQueryAnswerLimitsOverlongRendererOutput(t *testing.T) {
	longLine := strings.Repeat("超长摘要", 160)
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:   "orgunit.list",
		Executor: queryExecutorStub{},
		SummaryRenderer: func(cubebox.ExecuteResult) []string {
			return []string{longLine}
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	answer := buildCubeboxQueryAnswer(
		cubebox.ReadPlan{},
		[]cubebox.ExecuteResult{{APIKey: "orgunit.list", StepID: "step-1", Payload: map[string]any{"ignored": true}}},
		registry,
	)

	if strings.Contains(answer, longLine) {
		t.Fatalf("expected overlong renderer line to be omitted, answer=%s", answer)
	}
	if !strings.Contains(answer, cubeboxQuerySummaryFallbackOmitted) && !strings.Contains(answer, cubeboxQuerySummaryFallbackListNotice) {
		t.Fatalf("expected budget fallback in answer=%s", answer)
	}
	if utf8.RuneCountInString(answer) > cubeboxQueryAnswerMaxChars {
		t.Fatalf("answer over budget: %d > %d", utf8.RuneCountInString(answer), cubeboxQueryAnswerMaxChars)
	}
}

func TestBuildCubeboxQueryAnswerLimitsMultiStepAnswerButKeepsStepContext(t *testing.T) {
	registry, err := cubebox.NewExecutionRegistry(
		cubebox.RegisteredExecutor{
			APIKey:   "step.one",
			Executor: queryExecutorStub{},
			SummaryRenderer: func(cubebox.ExecuteResult) []string {
				return []string{"摘要一：" + strings.Repeat("A", 260)}
			},
		},
		cubebox.RegisteredExecutor{
			APIKey:   "step.two",
			Executor: queryExecutorStub{},
			SummaryRenderer: func(cubebox.ExecuteResult) []string {
				return []string{"摘要二：" + strings.Repeat("B", 260)}
			},
		},
		cubebox.RegisteredExecutor{
			APIKey:   "step.three",
			Executor: queryExecutorStub{},
			SummaryRenderer: func(cubebox.ExecuteResult) []string {
				return []string{"摘要三：" + strings.Repeat("C", 260)}
			},
		},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	answer := buildCubeboxQueryAnswer(
		cubebox.ReadPlan{ExplainFocus: []string{"多步查询验证"}},
		[]cubebox.ExecuteResult{
			{APIKey: "step.one", StepID: "step-1"},
			{APIKey: "step.two", StepID: "step-2"},
			{APIKey: "step.three", StepID: "step-3"},
		},
		registry,
	)

	for _, expected := range []string{"已完成只读查询。", "本次关注：多步查询验证。", "step-1（step.one）"} {
		if !strings.Contains(answer, expected) {
			t.Fatalf("expected %q in answer=%s", expected, answer)
		}
	}
	if !strings.Contains(answer, cubeboxQuerySummaryFallbackListNotice) && !strings.Contains(answer, cubeboxQuerySummaryFallbackOmitted) {
		t.Fatalf("expected budget fallback in answer=%s", answer)
	}
	if utf8.RuneCountInString(answer) > cubeboxQueryAnswerMaxChars {
		t.Fatalf("answer over budget: %d > %d", utf8.RuneCountInString(answer), cubeboxQueryAnswerMaxChars)
	}
}

func TestBuildCubeboxQueryAnswerDoesNotLimitSmallAnswer(t *testing.T) {
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:   "orgunit.details",
		Executor: queryExecutorStub{},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	answer := buildCubeboxQueryAnswer(
		cubebox.ReadPlan{ExplainFocus: []string{"查询总部"}},
		[]cubebox.ExecuteResult{
			{
				APIKey:      "orgunit.details",
				StepID:      "step-1",
				ResultFocus: []string{"org_unit.name"},
				Payload:     map[string]any{"org_unit": map[string]any{"name": "总部"}},
			},
		},
		registry,
	)

	for _, expected := range []string{"已完成只读查询。", "本次关注：查询总部。", "step-1（orgunit.details）", "org_unit.name：总部"} {
		if !strings.Contains(answer, expected) {
			t.Fatalf("expected %q in answer=%s", expected, answer)
		}
	}
	if strings.Contains(answer, cubeboxQuerySummaryFallbackListNotice) || strings.Contains(answer, cubeboxQuerySummaryFallbackOmitted) {
		t.Fatalf("unexpected budget fallback in answer=%s", answer)
	}
}
