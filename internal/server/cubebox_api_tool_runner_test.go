package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

func TestBuildAPIToolHTTPRequestPreservesAllOrgUnits(t *testing.T) {
	tool := cubebox.APITool{Method: http.MethodGet, Path: "/org/api/org-units"}
	call := cubebox.APICallStep{Params: map[string]any{
		"as_of":         "2026-01-01",
		"all_org_units": true,
		"page":          1,
		"page_size":     100,
	}}

	req, err := buildAPIToolHTTPRequest(context.Background(), cubebox.ExecuteRequest{
		TenantID:    "t1",
		PrincipalID: "p1",
	}, tool, call)
	if err != nil {
		t.Fatalf("build request err=%v", err)
	}

	query := req.URL.Query()
	if query.Get("all_org_units") != "true" {
		t.Fatalf("all_org_units=%q raw=%s", query.Get("all_org_units"), req.URL.RawQuery)
	}
	if query.Get("page") != "0" || query.Get("size") != "100" {
		t.Fatalf("unexpected pagination raw=%s", req.URL.RawQuery)
	}
}

func TestProjectAPIToolResultProjectsListCandidates(t *testing.T) {
	result := projectAPIToolResult(cubebox.APITool{OperationID: "orgunit.list"}, map[string]any{
		"as_of": "2026-01-01",
		"org_units": []any{
			map[string]any{"org_code": "A001", "name": "Root", "status": "active"},
			map[string]any{"org_code": "A002", "name": "Child", "status": "disabled"},
		},
	})

	if len(result.PresentedCandidates) != 2 {
		t.Fatalf("candidates=%#v", result.PresentedCandidates)
	}
	if result.PresentedCandidates[0].EntityKey != "A001" || result.PresentedCandidates[0].AsOf != "2026-01-01" {
		t.Fatalf("unexpected first candidate=%#v", result.PresentedCandidates[0])
	}
}

func TestAPIToolHTTPErrorMapsSearchAmbiguous(t *testing.T) {
	err := apiToolHTTPError(http.StatusConflict, `{
		"error_code":"org_unit_search_ambiguous",
		"tree_as_of":"2026-01-01",
		"candidates":[
			{"org_code":"A001","name":"Root","status":"active"},
			{"org_code":"A002","name":"Child","status":"active"}
		]
	}`)

	var ambiguous *orgUnitSearchAmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected ambiguous error, got %T %v", err, err)
	}
	candidates := ambiguous.QueryCandidates()
	if len(candidates) != 2 || candidates[0].EntityKey != "A001" || candidates[0].AsOf != "2026-01-01" {
		t.Fatalf("unexpected candidates=%#v", candidates)
	}
	facts := ambiguous.QueryClarificationFacts()
	if facts.ErrorCode != "org_unit_search_ambiguous" || !facts.CannotSilentSelect {
		t.Fatalf("unexpected facts=%#v", facts)
	}
}

func TestAPIToolHTTPErrorLeavesUnknownConflictTerminal(t *testing.T) {
	err := apiToolHTTPError(http.StatusConflict, `{"error_code":"other"}`)
	if !strings.Contains(err.Error(), "api tool http status 409") {
		t.Fatalf("unexpected err=%T %v", err, err)
	}
}
