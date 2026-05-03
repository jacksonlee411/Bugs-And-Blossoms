package cubebox

import (
	"context"
	"errors"
	"strings"
)

var ErrAPICatalogDriftOrExecutorMissing = errors.New("CUBEBOX_API_CATALOG_DRIFT_OR_EXECUTOR_MISSING")
var ErrKnowledgePackInvalid = errors.New("CUBEBOX_KNOWLEDGE_PACK_INVALID")

type ExecuteRequest struct {
	TenantID       string
	PrincipalID    string
	ConversationID string
}

type ExecutionTerminalError struct {
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

type QueryClarificationFacts struct {
	ErrorCode          string `json:"error_code,omitempty"`
	CandidateSource    string `json:"candidate_source,omitempty"`
	CandidateCount     int    `json:"candidate_count,omitempty"`
	CannotSilentSelect bool   `json:"cannot_silent_select,omitempty"`
}

type ScopeParamSemantics struct {
	ExpandAll []string `json:"expand_all,omitempty"`
	Narrowing []string `json:"narrowing,omitempty"`
}

type QueryRuntimeHints struct {
	UnsupportedPromptTerms []string            `json:"unsupported_prompt_terms,omitempty"`
	ScopeParams            ScopeParamSemantics `json:"scope_params,omitempty"`
}

type QueryCandidatesProvider interface {
	QueryCandidates() []QueryCandidate
}

type QueryClarificationFactsProvider interface {
	QueryClarificationFacts() QueryClarificationFacts
}

type QueryTerminalErrorProvider interface {
	QueryTerminalError() *ExecutionTerminalError
}

type ExecuteResult struct {
	Method              string
	Path                string
	OperationID         string
	StepID              string
	Payload             map[string]any
	ResultFocus         []string
	ConfirmedEntity     *QueryEntity
	PresentedCandidates []QueryCandidate
}

type QueryNarrationResult struct {
	Domain string         `json:"domain,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

type APIPlanExecutor interface {
	ExecutePlan(ctx context.Context, request ExecuteRequest, plan APICallPlan) ([]ExecuteResult, error)
}

func ProjectNarrationResults(results []ExecuteResult) []QueryNarrationResult {
	out := make([]QueryNarrationResult, 0, len(results))
	for _, result := range results {
		out = append(out, defaultQueryNarrationResult(result))
	}
	return out
}

func defaultQueryNarrationResult(result ExecuteResult) QueryNarrationResult {
	view := QueryNarrationResult{
		Domain: queryNarrationDomainForResult(result),
	}
	if len(result.Payload) > 0 {
		view.Data = copyQueryNarrationPayload(result.Payload)
	}
	return view
}

func copyQueryNarrationPayload(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func queryNarrationDomainForResult(result ExecuteResult) string {
	if result.ConfirmedEntity != nil {
		if normalized := NormalizeQueryEntity(*result.ConfirmedEntity); normalized != nil {
			return normalized.Domain
		}
	}
	return strings.TrimSpace(stringValue(result.Payload["domain"]))
}

func normalizeParamNames(items []string) []string {
	if items == nil {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
