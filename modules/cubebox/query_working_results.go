package cubebox

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const (
	DefaultQueryLoopMaxPlanningRounds     = 80
	DefaultQueryLoopMaxExecutedSteps      = 160
	DefaultQueryLoopMaxWorkingResultItems = 1000
	DefaultQueryLoopMaxRepeatedPlan       = 2
)

type QueryLoopBudget struct {
	MaxPlanningRounds     int `json:"max_planning_rounds"`
	MaxExecutedSteps      int `json:"max_executed_steps"`
	MaxWorkingResultItems int `json:"max_working_result_items"`
	MaxRepeatedPlan       int `json:"max_repeated_plan_fingerprint"`
}

type QueryWorkingResults struct {
	RoundIndex           int                       `json:"round_index"`
	OriginalUserGoal     string                    `json:"original_user_goal,omitempty"`
	Budget               QueryWorkingResultsBudget `json:"budget"`
	CompletedPlans       []QueryCompletedPlan      `json:"completed_plans"`
	LatestObservation    *QueryWorkingObservation  `json:"latest_observation,omitempty"`
	ExecutedFingerprints []string                  `json:"executed_fingerprints"`
	RepeatObservations   []QueryRepeatObservation  `json:"repeat_observations"`
}

type QueryWorkingResultsBudget struct {
	MaxPlanningRounds       int `json:"max_planning_rounds"`
	RemainingPlanningRounds int `json:"remaining_planning_rounds"`
	MaxExecutedSteps        int `json:"max_executed_steps"`
	RemainingExecutedSteps  int `json:"remaining_executed_steps"`
	MaxWorkingResultItems   int `json:"max_working_result_items"`
}

type QueryWorkingResultsPromptBudget struct {
	MaxCompletedPlans         int
	MaxLatestObservationItems int
	MaxRepeatObservations     int
}

type QueryCompletedPlan struct {
	Round  int                      `json:"round"`
	Intent string                   `json:"intent,omitempty"`
	Steps  []QueryCompletedPlanStep `json:"steps"`
}

type QueryCompletedPlanStep struct {
	StepID            string         `json:"step_id"`
	Method            string         `json:"method"`
	Path              string         `json:"path"`
	OperationID       string         `json:"operation_id,omitempty"`
	ParamsFingerprint string         `json:"params_fingerprint"`
	ItemCount         int            `json:"item_count"`
	Truncated         bool           `json:"truncated"`
	Summary           map[string]any `json:"summary,omitempty"`
}

type QueryWorkingObservation struct {
	Round             int            `json:"round"`
	StepID            string         `json:"step_id"`
	Method            string         `json:"method"`
	Path              string         `json:"path"`
	OperationID       string         `json:"operation_id,omitempty"`
	ParamsFingerprint string         `json:"params_fingerprint"`
	Items             []any          `json:"items,omitempty"`
	ItemCount         int            `json:"item_count"`
	Truncated         bool           `json:"truncated"`
	Summary           map[string]any `json:"summary,omitempty"`
}

type QueryRepeatObservation struct {
	Round             int    `json:"round"`
	ParamsFingerprint string `json:"params_fingerprint"`
	Message           string `json:"message"`
}

type QueryWorkingResultsState struct {
	budget         QueryLoopBudget
	originalGoal   string
	planningRounds int
	executedSteps  int
	completed      []QueryCompletedPlan
	latest         *QueryWorkingObservation
	executed       map[string]struct{}
	executedOrder  []string
	repeats        map[string]int
	repeatItems    []QueryRepeatObservation
}

func DefaultQueryLoopBudget() QueryLoopBudget {
	return QueryLoopBudget{
		MaxPlanningRounds:     DefaultQueryLoopMaxPlanningRounds,
		MaxExecutedSteps:      DefaultQueryLoopMaxExecutedSteps,
		MaxWorkingResultItems: DefaultQueryLoopMaxWorkingResultItems,
		MaxRepeatedPlan:       DefaultQueryLoopMaxRepeatedPlan,
	}
}

func NormalizeQueryLoopBudget(budget QueryLoopBudget) QueryLoopBudget {
	if budget.MaxPlanningRounds <= 0 {
		budget.MaxPlanningRounds = DefaultQueryLoopMaxPlanningRounds
	}
	if budget.MaxExecutedSteps <= 0 {
		budget.MaxExecutedSteps = DefaultQueryLoopMaxExecutedSteps
	}
	if budget.MaxWorkingResultItems <= 0 {
		budget.MaxWorkingResultItems = DefaultQueryLoopMaxWorkingResultItems
	}
	if budget.MaxRepeatedPlan < 0 {
		budget.MaxRepeatedPlan = DefaultQueryLoopMaxRepeatedPlan
	}
	return budget
}

func NewQueryWorkingResultsState(originalGoal string, budget QueryLoopBudget) *QueryWorkingResultsState {
	budget = NormalizeQueryLoopBudget(budget)
	return &QueryWorkingResultsState{
		budget:       budget,
		originalGoal: strings.TrimSpace(originalGoal),
		executed:     make(map[string]struct{}),
		repeats:      make(map[string]int),
	}
}

func (s *QueryWorkingResultsState) CanPlan() bool {
	return s == nil || s.planningRounds < s.budget.MaxPlanningRounds
}

func (s *QueryWorkingResultsState) NotePlanningRound() {
	if s == nil {
		return
	}
	s.planningRounds++
}

func (s *QueryWorkingResultsState) CanExecute(plan APICallPlan) bool {
	if s == nil {
		return true
	}
	return s.executedSteps+len(plan.Calls) <= s.budget.MaxExecutedSteps
}

func (s *QueryWorkingResultsState) HasExecuted(fingerprint string) bool {
	if s == nil {
		return false
	}
	_, ok := s.executed[strings.TrimSpace(fingerprint)]
	return ok
}

func (s *QueryWorkingResultsState) NoteRepeat(fingerprint string) bool {
	if s == nil {
		return false
	}
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return false
	}
	s.repeats[fingerprint]++
	s.repeatItems = append(s.repeatItems, QueryRepeatObservation{
		Round:             s.planningRounds,
		ParamsFingerprint: fingerprint,
		Message:           "该查询步骤已执行过，请选择下一步或返回 DONE。",
	})
	return s.repeats[fingerprint] > s.budget.MaxRepeatedPlan
}

func (s *QueryWorkingResultsState) AppendPlan(round int, plan APICallPlan, results []ExecuteResult) {
	if s == nil {
		return
	}
	completed := QueryCompletedPlan{
		Round: round,
		Steps: make([]QueryCompletedPlanStep, 0, len(plan.Calls)),
	}
	for index, call := range plan.Calls {
		var result ExecuteResult
		if index < len(results) {
			result = results[index]
		}
		fingerprint := StepFingerprint(call)
		s.executed[fingerprint] = struct{}{}
		s.executedOrder = appendIfMissing(s.executedOrder, fingerprint)
		observation := buildWorkingObservation(round, call, result, fingerprint, s.budget.MaxWorkingResultItems)
		completed.Steps = append(completed.Steps, QueryCompletedPlanStep{
			StepID:            strings.TrimSpace(call.ID),
			Method:            strings.ToUpper(strings.TrimSpace(call.Method)),
			Path:              normalizeAPICallPath(call.Path),
			OperationID:       strings.TrimSpace(result.OperationID),
			ParamsFingerprint: fingerprint,
			ItemCount:         observation.ItemCount,
			Truncated:         observation.Truncated,
			Summary:           observation.Summary,
		})
		copyObservation := observation
		s.latest = &copyObservation
	}
	s.executedSteps += len(plan.Calls)
	s.completed = append(s.completed, completed)
}

func (s *QueryWorkingResultsState) Snapshot() QueryWorkingResults {
	if s == nil {
		return QueryWorkingResults{
			CompletedPlans:       []QueryCompletedPlan{},
			ExecutedFingerprints: []string{},
			RepeatObservations:   []QueryRepeatObservation{},
		}
	}
	return QueryWorkingResults{
		RoundIndex:       s.planningRounds,
		OriginalUserGoal: s.originalGoal,
		Budget: QueryWorkingResultsBudget{
			MaxPlanningRounds:       s.budget.MaxPlanningRounds,
			RemainingPlanningRounds: maxInt(0, s.budget.MaxPlanningRounds-s.planningRounds),
			MaxExecutedSteps:        s.budget.MaxExecutedSteps,
			RemainingExecutedSteps:  maxInt(0, s.budget.MaxExecutedSteps-s.executedSteps),
			MaxWorkingResultItems:   s.budget.MaxWorkingResultItems,
		},
		CompletedPlans:       cloneCompletedPlans(s.completed),
		LatestObservation:    cloneWorkingObservation(s.latest),
		ExecutedFingerprints: cloneStrings(s.executedOrder),
		RepeatObservations:   cloneRepeatObservations(s.repeatItems),
	}
}

func (s *QueryWorkingResultsState) HasExecution() bool {
	return s != nil && s.executedSteps > 0
}

func WorkingResultsPromptBlock(snapshot QueryWorkingResults) string {
	body, err := json.Marshal(map[string]any{"working_results": ProjectQueryWorkingResultsForPrompt(snapshot)})
	if err != nil {
		return ""
	}
	return string(body)
}

func DefaultQueryWorkingResultsPromptBudget() QueryWorkingResultsPromptBudget {
	return QueryWorkingResultsPromptBudget{
		MaxCompletedPlans:         20,
		MaxLatestObservationItems: 200,
		MaxRepeatObservations:     8,
	}
}

func NormalizeQueryWorkingResultsPromptBudget(budget QueryWorkingResultsPromptBudget) QueryWorkingResultsPromptBudget {
	defaults := DefaultQueryWorkingResultsPromptBudget()
	if budget.MaxCompletedPlans <= 0 {
		budget.MaxCompletedPlans = defaults.MaxCompletedPlans
	}
	if budget.MaxLatestObservationItems <= 0 {
		budget.MaxLatestObservationItems = defaults.MaxLatestObservationItems
	}
	if budget.MaxRepeatObservations <= 0 {
		budget.MaxRepeatObservations = defaults.MaxRepeatObservations
	}
	return budget
}

func ProjectQueryWorkingResultsForPrompt(snapshot QueryWorkingResults) map[string]any {
	return projectQueryWorkingResultsForPrompt(snapshot, DefaultQueryWorkingResultsPromptBudget())
}

func projectQueryWorkingResultsForPrompt(snapshot QueryWorkingResults, budget QueryWorkingResultsPromptBudget) map[string]any {
	budget = NormalizeQueryWorkingResultsPromptBudget(budget)
	projected := map[string]any{
		"round_index":                snapshot.RoundIndex,
		"original_user_goal":         snapshot.OriginalUserGoal,
		"budget":                     snapshot.Budget,
		"completed_plans":            projectCompletedPlansForPrompt(snapshot.CompletedPlans, budget.MaxCompletedPlans),
		"executed_fingerprints":      cloneStrings(snapshot.ExecutedFingerprints),
		"executed_fingerprint_count": len(snapshot.ExecutedFingerprints),
		"repeat_observations":        projectRepeatObservationsForPrompt(snapshot.RepeatObservations, budget.MaxRepeatObservations),
		"repeat_observation_count":   len(snapshot.RepeatObservations),
	}
	if snapshot.LatestObservation != nil {
		projected["latest_observation"] = projectWorkingObservationForPrompt(snapshot.LatestObservation, budget.MaxLatestObservationItems)
	}
	return projected
}

func projectCompletedPlansForPrompt(items []QueryCompletedPlan, maxItems int) []QueryCompletedPlan {
	items = cloneCompletedPlans(items)
	if maxItems <= 0 || len(items) <= maxItems {
		return items
	}
	return items[len(items)-maxItems:]
}

func projectRepeatObservationsForPrompt(items []QueryRepeatObservation, maxItems int) []QueryRepeatObservation {
	items = cloneRepeatObservations(items)
	if maxItems <= 0 || len(items) <= maxItems {
		return items
	}
	return items[len(items)-maxItems:]
}

func projectWorkingObservationForPrompt(item *QueryWorkingObservation, maxItems int) *QueryWorkingObservation {
	item = cloneWorkingObservation(item)
	if item == nil {
		return nil
	}
	if maxItems <= 0 || len(item.Items) <= maxItems {
		return item
	}
	item.Items = append([]any(nil), item.Items[:maxItems]...)
	item.Truncated = true
	return item
}

func PlanFingerprint(plan APICallPlan) string {
	parts := make([]string, 0, len(plan.Calls))
	for _, call := range plan.Calls {
		parts = append(parts, StepFingerprint(call))
	}
	return strings.Join(parts, "||")
}

func StepFingerprint(call APICallStep) string {
	return strings.ToUpper(strings.TrimSpace(call.Method)) + "|" + normalizeAPICallPath(call.Path) + "|" + canonicalParamFingerprint(call.Params)
}

func buildWorkingObservation(round int, call APICallStep, result ExecuteResult, fingerprint string, maxItems int) QueryWorkingObservation {
	items, itemCount, truncated := observationItems(result.Payload, maxItems)
	return QueryWorkingObservation{
		Round:             round,
		StepID:            strings.TrimSpace(call.ID),
		Method:            strings.ToUpper(strings.TrimSpace(call.Method)),
		Path:              normalizeAPICallPath(call.Path),
		OperationID:       strings.TrimSpace(result.OperationID),
		ParamsFingerprint: fingerprint,
		Items:             items,
		ItemCount:         itemCount,
		Truncated:         truncated,
		Summary:           observationSummary(result.Payload),
	}
}

func observationItems(payload map[string]any, maxItems int) ([]any, int, bool) {
	if maxItems <= 0 {
		maxItems = DefaultQueryLoopMaxWorkingResultItems
	}
	if len(payload) == 0 {
		return nil, 0, false
	}
	keys := make([]string, 0, len(payload))
	for key, value := range payload {
		if sliceLen(value) >= 0 {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		items := valueSlice(payload[key])
		if items == nil {
			continue
		}
		itemCount := len(items)
		truncated := itemCount > maxItems
		if truncated {
			items = items[:maxItems]
		}
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out, itemCount, truncated
	}
	if len(payload) > 0 {
		return []any{copyObservationValue(payload)}, 1, false
	}
	return nil, 0, false
}

func observationSummary(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	summary := make(map[string]any)
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		value := payload[key]
		if sliceLen(value) >= 0 {
			summary[key+"_count"] = sliceLen(value)
			continue
		}
		switch value.(type) {
		case string, bool, int, int64, float64, nil:
			summary[key] = value
		}
	}
	if len(summary) == 0 {
		return nil
	}
	return summary
}

func canonicalParamFingerprint(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, strings.TrimSpace(key))
	}
	slices.Sort(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+canonicalParamValue(params[key]))
	}
	return strings.Join(parts, "|")
}

func canonicalParamValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return strconv.Quote(strings.TrimSpace(v))
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, canonicalParamValue(item))
		}
		return "[" + strings.Join(items, ",") + "]"
	case map[string]any:
		return "{" + canonicalParamFingerprint(v) + "}"
	default:
		body, err := json.Marshal(v)
		if err != nil {
			return strconv.Quote(fmt.Sprint(v))
		}
		return string(body)
	}
}

func valueSlice(value any) []any {
	switch v := value.(type) {
	case []any:
		return append([]any(nil), v...)
	default:
		rv := reflectSlice(value)
		if rv == nil {
			return nil
		}
		return rv
	}
}

func sliceLen(value any) int {
	items := valueSlice(value)
	if items == nil {
		return -1
	}
	return len(items)
}

func reflectSlice(value any) []any {
	body, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var items []any
	if err := json.Unmarshal(body, &items); err != nil {
		return nil
	}
	return items
}

func copyObservationValue(value any) any {
	body, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return value
	}
	return out
}

func cloneWorkingObservation(in *QueryWorkingObservation) *QueryWorkingObservation {
	if in == nil {
		return nil
	}
	out := *in
	out.Items = append([]any(nil), in.Items...)
	if in.Summary != nil {
		out.Summary = make(map[string]any, len(in.Summary))
		for key, value := range in.Summary {
			out.Summary[key] = value
		}
	}
	return &out
}

func cloneCompletedPlans(items []QueryCompletedPlan) []QueryCompletedPlan {
	out := make([]QueryCompletedPlan, 0, len(items))
	for _, item := range items {
		copyItem := item
		copyItem.Steps = append([]QueryCompletedPlanStep(nil), item.Steps...)
		out = append(out, copyItem)
	}
	return out
}

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	return append([]string(nil), items...)
}

func cloneRepeatObservations(items []QueryRepeatObservation) []QueryRepeatObservation {
	if len(items) == 0 {
		return []QueryRepeatObservation{}
	}
	return append([]QueryRepeatObservation(nil), items...)
}

func appendIfMissing(items []string, item string) []string {
	item = strings.TrimSpace(item)
	if item == "" {
		return items
	}
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
