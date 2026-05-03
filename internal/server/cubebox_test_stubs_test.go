package server

import (
	"context"
	"io"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

type cubeboxProviderChatStreamTextStub struct {
	chunks []cubebox.ProviderChatChunk
	errs   []error
	index  int
}

func (s *cubeboxProviderChatStreamTextStub) Recv() (cubebox.ProviderChatChunk, error) {
	if s.index < len(s.errs) && s.errs[s.index] != nil {
		err := s.errs[s.index]
		s.index++
		return cubebox.ProviderChatChunk{}, err
	}
	if s.index >= len(s.chunks) {
		return cubebox.ProviderChatChunk{}, io.EOF
	}
	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
}

func (s *cubeboxProviderChatStreamTextStub) Close() error {
	return nil
}

type cubeboxRuntimeConfigReaderStub struct {
	config cubebox.ActiveModelRuntimeConfig
	err    error
}

func (s cubeboxRuntimeConfigReaderStub) GetActiveModelRuntimeConfig(context.Context, string) (cubebox.ActiveModelRuntimeConfig, error) {
	if s.err != nil {
		return cubebox.ActiveModelRuntimeConfig{}, s.err
	}
	return s.config, nil
}

type cubeboxProviderAdapterStub struct {
	stream       cubebox.ProviderChatStream
	err          error
	lastRequest  cubebox.ProviderChatRequest
	requestCount int
}

func (s *cubeboxProviderAdapterStub) StreamChatCompletion(_ context.Context, request cubebox.ProviderChatRequest) (cubebox.ProviderChatStream, error) {
	s.lastRequest = request
	s.requestCount++
	if s.err != nil {
		return nil, s.err
	}
	return s.stream, nil
}

type cubeboxSecretResolverStub struct {
	secret string
	err    error
}

func (s cubeboxSecretResolverStub) ResolveSecretRef(context.Context, string, string, string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.secret, nil
}

type cubeboxAPIToolRunnerStub struct {
	tools   []cubebox.APITool
	results []cubebox.ExecuteResult
	err     error
	fn      func(context.Context, cubebox.ExecuteRequest, cubebox.APICallPlan) ([]cubebox.ExecuteResult, error)
}

func (s cubeboxAPIToolRunnerStub) Tools() []cubebox.APITool {
	if s.tools != nil {
		return append([]cubebox.APITool(nil), s.tools...)
	}
	return testCubeBoxAPITools()
}

func (s cubeboxAPIToolRunnerStub) ExecutePlan(ctx context.Context, request cubebox.ExecuteRequest, plan cubebox.APICallPlan) ([]cubebox.ExecuteResult, error) {
	if s.fn != nil {
		return s.fn(ctx, request, plan)
	}
	if s.err != nil {
		return nil, s.err
	}
	if s.results != nil {
		return append([]cubebox.ExecuteResult(nil), s.results...), nil
	}
	return []cubebox.ExecuteResult{{
		Method:      plan.Calls[0].Method,
		Path:        plan.Calls[0].Path,
		OperationID: "orgunit.list",
		Payload: map[string]any{
			"as_of":     "2026-04-25",
			"org_units": []map[string]any{{"org_code": "100000", "name": "飞虫与鲜花"}},
		},
	}}, nil
}

func testCubeBoxAPITools() []cubebox.APITool {
	return []cubebox.APITool{
		testCubeBoxAPITool("GET", "/org/api/org-units", "orgunit.list", []string{"as_of"}, []string{"include_disabled", "parent_org_code", "all_org_units", "keyword", "page", "page_size"}),
		testCubeBoxAPITool("GET", "/org/api/org-units/details", "orgunit.details", []string{"org_code", "as_of"}, []string{"include_disabled"}),
		testCubeBoxAPITool("GET", "/org/api/org-units/search", "orgunit.search", []string{"keyword", "as_of"}, []string{"include_disabled", "page", "page_size"}),
		testCubeBoxAPITool("GET", "/org/api/org-units/audit", "orgunit.audit", []string{"org_code"}, []string{"as_of"}),
	}
}

func testCubeBoxAPITool(method string, path string, operationID string, required []string, optional []string) cubebox.APITool {
	return cubebox.APITool{
		OperationID:        operationID,
		Method:             method,
		Path:               path,
		ResourceObject:     "orgunit",
		Action:             actionForTestCubeBoxAPITool(operationID),
		AuthzCapabilityKey: "orgunit." + actionForTestCubeBoxAPITool(operationID),
		RequestSchema: cubebox.APIToolRequestSchema{
			Required: required,
			Optional: optional,
		},
		UseSummary: "orgunit api tool",
		ObservationProjection: cubebox.APIToolObservationProjection{
			RootField:       "org_unit",
			EntityKeyField:  "org_code",
			EntityNameField: "name",
		},
	}
}

func actionForTestCubeBoxAPITool(operationID string) string {
	switch operationID {
	case "orgunit.list":
		return "list"
	case "orgunit.details":
		return "read"
	case "orgunit.search":
		return "search"
	case "orgunit.audit":
		return "audit_read"
	default:
		return "read"
	}
}

func turnIDPtrForTest(value string) *string {
	return &value
}
