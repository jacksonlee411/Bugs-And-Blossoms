package server

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type apiToolOverlayDefinition struct {
	Method                string
	Path                  string
	CubeBoxCallable       bool
	OperationID           string
	UseSummary            string
	RequestSchema         cubebox.APIToolRequestSchema
	ResponseSchemaRef     string
	ObservationProjection cubebox.APIToolObservationProjection
}

var apiToolOverlayDefinitions = []apiToolOverlayDefinition{
	{
		Method:          http.MethodGet,
		Path:            "/org/api/org-units",
		CubeBoxCallable: true,
		OperationID:     "orgunit.list",
		UseSummary:      "按日期列出当前用户可访问的组织；可用 keyword、parent_org_code、status、is_business_unit 和分页参数收窄。",
		RequestSchema: cubebox.APIToolRequestSchema{
			Required: []string{"as_of"},
			Optional: []string{"include_disabled", "parent_org_code", "all_org_units", "keyword", "status", "is_business_unit", "page", "page_size"},
			Params: map[string]cubebox.APIParamSpec{
				"as_of":            {Type: "date", Description: "业务生效日期，YYYY-MM-DD。"},
				"include_disabled": {Type: "boolean"},
				"parent_org_code":  {Type: "string"},
				"all_org_units":    {Type: "boolean"},
				"keyword":          {Type: "string"},
				"status":           {Type: "string"},
				"is_business_unit": {Type: "boolean"},
				"page":             {Type: "integer", Description: "planner-facing 1 基页码。"},
				"page_size":        {Type: "integer"},
			},
		},
		ResponseSchemaRef: "orgUnitListAPIResponse",
		ObservationProjection: cubebox.APIToolObservationProjection{
			RootField:       "org_units",
			SummaryFields:   []string{"as_of", "total", "page", "size"},
			EntityKeyField:  "org_code",
			EntityNameField: "name",
		},
	},
	{
		Method:          http.MethodGet,
		Path:            "/org/api/org-units/details",
		CubeBoxCallable: true,
		OperationID:     "orgunit.details",
		UseSummary:      "按组织编码和日期读取当前用户有权访问的组织详情。",
		RequestSchema: cubebox.APIToolRequestSchema{
			Required: []string{"org_code", "as_of"},
			Optional: []string{"include_disabled"},
			Params: map[string]cubebox.APIParamSpec{
				"org_code":         {Type: "string"},
				"as_of":            {Type: "date", Description: "业务生效日期，YYYY-MM-DD。"},
				"include_disabled": {Type: "boolean"},
			},
		},
		ResponseSchemaRef: "orgUnitDetailsAPIResponse",
		ObservationProjection: cubebox.APIToolObservationProjection{
			RootField:       "org_unit",
			SummaryFields:   []string{"as_of"},
			EntityKeyField:  "org_code",
			EntityNameField: "name",
		},
	},
	{
		Method:          http.MethodGet,
		Path:            "/org/api/org-units/search",
		CubeBoxCallable: true,
		OperationID:     "orgunit.search",
		UseSummary:      "按关键词在指定日期搜索一个组织，并返回其路径编码。",
		RequestSchema: cubebox.APIToolRequestSchema{
			Required: []string{"query", "as_of"},
			Optional: []string{"include_disabled"},
			Params: map[string]cubebox.APIParamSpec{
				"query":            {Type: "string"},
				"as_of":            {Type: "date", Description: "业务生效日期，YYYY-MM-DD。"},
				"include_disabled": {Type: "boolean"},
			},
		},
		ResponseSchemaRef: "OrgUnitSearchResult",
		ObservationProjection: cubebox.APIToolObservationProjection{
			RootField:       "",
			SummaryFields:   []string{"target_org_code", "target_name", "tree_as_of"},
			EntityKeyField:  "target_org_code",
			EntityNameField: "target_name",
		},
	},
	{
		Method:          http.MethodGet,
		Path:            "/org/api/org-units/audit",
		CubeBoxCallable: true,
		OperationID:     "orgunit.audit",
		UseSummary:      "按组织编码读取当前用户有权访问的组织审计事件。",
		RequestSchema: cubebox.APIToolRequestSchema{
			Required: []string{"org_code"},
			Optional: []string{"limit"},
			Params: map[string]cubebox.APIParamSpec{
				"org_code": {Type: "string"},
				"limit":    {Type: "integer"},
			},
		},
		ResponseSchemaRef: "orgUnitAuditAPIResponse",
		ObservationProjection: cubebox.APIToolObservationProjection{
			RootField:      "events",
			SummaryFields:  []string{"org_code", "limit", "has_more"},
			EntityKeyField: "org_code",
		},
	},
}

func listAPIToolOverlayDefinitions() []apiToolOverlayDefinition {
	out := append([]apiToolOverlayDefinition(nil), apiToolOverlayDefinitions...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Method < out[j].Method
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func ListAuthzToolOverlayCoverage() []AuthzToolOverlayCoverage {
	definitions := listAPIToolOverlayDefinitions()
	out := make([]AuthzToolOverlayCoverage, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, AuthzToolOverlayCoverage{
			Method:          strings.ToUpper(strings.TrimSpace(definition.Method)),
			Path:            normalizeServerAPIToolPath(definition.Path),
			CubeBoxCallable: definition.CubeBoxCallable,
			Surface:         authz.CapabilitySurfaceTenantAPI,
		})
	}
	return out
}

func BuildCubeBoxRuntimeAPITools(facts AuthzCoverageFacts) ([]cubebox.APITool, error) {
	entries, err := ListAuthzAPICatalogEntries(facts, authzAPICatalogFilter{})
	if err != nil {
		return nil, err
	}
	entryByRoute := make(map[string]authzAPICatalogEntry, len(entries))
	for _, entry := range entries {
		entryByRoute[cubebox.APIToolRouteID(entry.Method, entry.Path)] = entry
	}

	seen := map[string]struct{}{}
	tools := make([]cubebox.APITool, 0, len(apiToolOverlayDefinitions))
	for _, definition := range listAPIToolOverlayDefinitions() {
		routeID := cubebox.APIToolRouteID(definition.Method, definition.Path)
		if _, ok := seen[routeID]; ok {
			return nil, fmt.Errorf("duplicate cubebox api tool overlay: %s", routeID)
		}
		seen[routeID] = struct{}{}
		if !definition.CubeBoxCallable {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(definition.Method)) != http.MethodGet {
			return nil, fmt.Errorf("cubebox api tool overlay must be GET: %s", routeID)
		}
		entry, ok := entryByRoute[routeID]
		if !ok {
			return nil, fmt.Errorf("cubebox api tool overlay missing authz catalog entry: %s", routeID)
		}
		if !entry.CubeBoxCallable {
			return nil, fmt.Errorf("cubebox api tool overlay not reflected in authz catalog: %s", routeID)
		}
		if entry.AuthzCapabilityKey == "" || entry.ResourceObject == "" || entry.Action == "" {
			return nil, fmt.Errorf("cubebox api tool overlay missing authz requirement fields: %s", routeID)
		}
		tool := cubebox.APITool{
			Method:                definition.Method,
			Path:                  definition.Path,
			OperationID:           definition.OperationID,
			UseSummary:            definition.UseSummary,
			RequestSchema:         definition.RequestSchema,
			ResponseSchemaRef:     definition.ResponseSchemaRef,
			ObservationProjection: definition.ObservationProjection,
			ResourceObject:        entry.ResourceObject,
			Action:                entry.Action,
			AuthzCapabilityKey:    entry.AuthzCapabilityKey,
		}.Normalized()
		tools = append(tools, tool)
	}
	if len(tools) == 0 {
		return nil, errors.New("cubebox api tools missing")
	}
	sort.SliceStable(tools, func(i, j int) bool {
		if tools[i].Path == tools[j].Path {
			return tools[i].Method < tools[j].Method
		}
		return tools[i].Path < tools[j].Path
	})
	return tools, nil
}

func normalizeServerAPIToolPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}
