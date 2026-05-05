package server

import (
	"errors"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

type orgUnitSearchAmbiguousError struct {
	Query      string
	Candidates []OrgUnitSearchCandidate
	AsOf       string
}

func (e *orgUnitSearchAmbiguousError) Error() string {
	return "org_unit_search_ambiguous"
}

func (e *orgUnitSearchAmbiguousError) QueryCandidates() []cubebox.QueryCandidate {
	if e == nil {
		return nil
	}
	items := make([]cubebox.QueryCandidate, 0, len(e.Candidates))
	for _, candidate := range e.Candidates {
		normalized := cubebox.NormalizeQueryCandidate(cubebox.QueryCandidate{
			Domain:    "orgunit",
			EntityKey: candidate.OrgCode,
			Name:      candidate.Name,
			AsOf:      e.AsOf,
			Status:    candidate.Status,
		})
		if normalized == nil {
			continue
		}
		items = append(items, *normalized)
	}
	return items
}

func (e *orgUnitSearchAmbiguousError) QueryClarificationFacts() cubebox.QueryClarificationFacts {
	return cubebox.QueryClarificationFacts{
		ErrorCode:          "org_unit_search_ambiguous",
		CandidateSource:    "execution_error",
		CandidateCount:     len(e.QueryCandidates()),
		CannotSilentSelect: true,
	}
}

type orgUnitNotFoundError struct{}

func (e *orgUnitNotFoundError) Error() string {
	return "org_unit_not_found"
}

func (e *orgUnitNotFoundError) Unwrap() error {
	return errOrgUnitNotFound
}

func (e *orgUnitNotFoundError) QueryTerminalError() *cubebox.ExecutionTerminalError {
	return &cubebox.ExecutionTerminalError{
		Code:      "orgunit_not_found",
		Message:   "未找到符合条件的组织，请调整关键词或提供组织编码。",
		Retryable: false,
	}
}

type orgUnitAuthzScopeError struct {
	err error
}

func (e *orgUnitAuthzScopeError) Error() string {
	return "authz_scope_forbidden"
}

func (e *orgUnitAuthzScopeError) Unwrap() error {
	return e.err
}

func (e *orgUnitAuthzScopeError) QueryTerminalError() *cubebox.ExecutionTerminalError {
	return &cubebox.ExecutionTerminalError{
		Code:      "authz_scope_forbidden",
		Message:   "当前账号没有访问该组织范围的权限。",
		Retryable: false,
	}
}

func wrapOrgUnitAPIToolError(err error) error {
	if err == nil {
		return nil
	}
	if isOrgUnitAuthzScopeError(err) {
		return &orgUnitAuthzScopeError{err: err}
	}
	if errors.Is(err, errOrgUnitNotFound) {
		return &orgUnitNotFoundError{}
	}
	return err
}
