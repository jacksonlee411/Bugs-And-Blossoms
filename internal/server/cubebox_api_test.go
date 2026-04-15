package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cubeboxservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services"
)

func TestCubeBoxConversationAndTaskAPIWrappers(t *testing.T) {
	t.Run("conversation detail delete not implemented", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/internal/cubebox/conversations/conv-1", nil)

		handleCubeBoxConversationDetailAPI(rec, req, nil)
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "cubebox_conversation_delete_not_implemented") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("conversation detail missing service", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations/conv-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxConversationDetailAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("conversation turns gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns", strings.NewReader(`{"user_input":"hello"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxConversationTurnsAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), errAssistantGateUnavailable.Error()) {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("turn action gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:confirm", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxTurnActionAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tasks gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks", strings.NewReader(`{"conversation_id":"conv-1","turn_id":"turn-1","task_type":"confirm","request_id":"req-1","contract_snapshot":{"plan_hash":"p"}}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxTasksAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("task detail gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks/task-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxTaskDetailAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("task action gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks/task-1:cancel", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxTaskActionAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestCubeBoxRuntimeStatusAPI(t *testing.T) {
	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/runtime-status", nil)

		handleCubeBoxRuntimeStatusAPI(rec, req, nil, nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("assistant and file store missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/runtime-status", nil)

		handleCubeBoxRuntimeStatusAPI(rec, req, nil, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload cubeboxRuntimeStatusResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Status != assistantRuntimeHealthUnavailable {
			t.Fatalf("status=%+v", payload)
		}
		if payload.Backend.Reason != "assistant_service_missing" || payload.FileStore.Reason != "file_store_missing" {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.KnowledgeRuntime.Reason != "knowledge_runtime_missing" || payload.ModelGateway.Reason != "model_gateway_missing" {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.Capabilities.ConversationEnabled != true || payload.Capabilities.FilesEnabled != true {
			t.Fatalf("capabilities=%+v", payload.Capabilities)
		}
		if len(payload.RetiredCapabilities) == 0 {
			t.Fatalf("retired capabilities missing: %+v", payload)
		}
	})

	t.Run("knowledge degraded but model and file healthy", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/runtime-status", nil)
		svc := &assistantConversationService{
			modelGateway: &assistantModelGateway{},
			knowledgeErr: errors.New("knowledge unavailable"),
		}
		fileSvc := cubeboxservices.NewFileService(&runtimeHealthyFileStore{})

		handleCubeBoxRuntimeStatusAPI(rec, req, svc, fileSvc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload cubeboxRuntimeStatusResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Status != assistantRuntimeHealthDegraded {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.KnowledgeRuntime.Reason != "knowledge_runtime_unavailable" {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.ModelGateway.Healthy != assistantRuntimeHealthHealthy || payload.FileStore.Healthy != assistantRuntimeHealthHealthy {
			t.Fatalf("payload=%+v", payload)
		}
	})

	t.Run("model gateway unavailable and file store unhealthy", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/runtime-status", nil)
		svc := &assistantConversationService{
			modelGateway: &assistantModelGateway{},
			gatewayErr:   errors.New("gateway unavailable"),
		}
		fileSvc := cubeboxservices.NewFileService(&runtimeHealthyFileStore{healthyErr: errors.New("disk unavailable")})

		handleCubeBoxRuntimeStatusAPI(rec, req, svc, fileSvc)
		var payload cubeboxRuntimeStatusResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Status != assistantRuntimeHealthUnavailable {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.ModelGateway.Reason != "model_gateway_unavailable" || payload.FileStore.Reason != "file_store_unavailable" {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.KnowledgeRuntime.Healthy != assistantRuntimeHealthHealthy {
			t.Fatalf("payload=%+v", payload)
		}
	})

	t.Run("assistant present but model gateway missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/runtime-status", nil)
		svc := &assistantConversationService{}
		fileSvc := cubeboxservices.NewFileService(&runtimeHealthyFileStore{})

		handleCubeBoxRuntimeStatusAPI(rec, req, svc, fileSvc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload cubeboxRuntimeStatusResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Status != assistantRuntimeHealthUnavailable {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.ModelGateway.Reason != "model_gateway_missing" {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.KnowledgeRuntime.Healthy != assistantRuntimeHealthHealthy {
			t.Fatalf("payload=%+v", payload)
		}
	})
}

func TestCubeBoxTaskPollURIHelpers(t *testing.T) {
	t.Parallel()

	if shouldRewriteCubeBoxTaskPollURI("text/plain", []byte(`{"poll_uri":"/internal/assistant/tasks/t1"}`)) {
		t.Fatal("plain text should not rewrite")
	}
	if shouldRewriteCubeBoxTaskPollURI("application/json", nil) {
		t.Fatal("empty body should not rewrite")
	}
	if !shouldRewriteCubeBoxTaskPollURI(" application/json; charset=utf-8 ", []byte(`{"poll_uri":"/internal/assistant/tasks/t1"}`)) {
		t.Fatal("json assistant task poll uri should rewrite")
	}

	body := rewriteCubeBoxTaskPollURI([]byte(`{"poll_uri":"/internal/assistant/tasks/task_1","status":"queued"}`))
	if !strings.Contains(string(body), "/internal/cubebox/tasks/task_1") {
		t.Fatalf("unexpected rewritten body: %s", string(body))
	}
	if got := rewriteCubeBoxTaskPollURI([]byte(`not-json`)); string(got) != "not-json" {
		t.Fatalf("expected original body, got %s", string(got))
	}
	if got := rewriteCubeBoxTaskPollURI([]byte(`{"status":"queued"}`)); !strings.Contains(string(got), `"status":"queued"`) {
		t.Fatalf("expected unchanged payload, got %s", string(got))
	}
	if got := rewriteCubeBoxTaskPollURI([]byte(`{"poll_uri":123}`)); string(got) != `{"poll_uri":123}` {
		t.Fatalf("expected unchanged non-string poll_uri, got %s", string(got))
	}

	if got := cubeboxTaskPollURI(" /internal/assistant/tasks/task_2 "); got != "/internal/cubebox/tasks/task_2" {
		t.Fatalf("unexpected task poll uri: %q", got)
	}
	if got := cubeboxTaskPollURI("/internal/cubebox/tasks/task_2"); got != "/internal/cubebox/tasks/task_2" {
		t.Fatalf("unexpected unchanged uri: %q", got)
	}
}

func TestProxyCubeBoxTaskPollURIResponse(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks", nil)
	rec := httptest.NewRecorder()

	proxyCubeBoxTaskPollURIResponse(rec, req, func(inner *httptest.ResponseRecorder) {
		inner.Header().Set("Content-Type", "application/json")
		inner.Header().Add("X-Test", "one")
		inner.WriteHeader(http.StatusAccepted)
		_, _ = inner.Write([]byte(`{"poll_uri":"/internal/assistant/tasks/task_3"}`))
	})

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Test") != "one" {
		t.Fatalf("headers=%v", rec.Header())
	}
	if !strings.Contains(rec.Body.String(), "/internal/cubebox/tasks/task_3") {
		t.Fatalf("body=%s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	proxyCubeBoxTaskPollURIResponse(rec, req, func(inner *httptest.ResponseRecorder) {
		inner.Header().Set("Content-Type", "text/plain")
		inner.WriteHeader(http.StatusAccepted)
		_, _ = inner.Write([]byte("plain body"))
	})
	if rec.Body.String() != "plain body" {
		t.Fatalf("expected passthrough body, got %q", rec.Body.String())
	}
}

type runtimeHealthyFileStore struct {
	healthyErr error
}

func (s *runtimeHealthyFileStore) List(_ context.Context, _ string, _ string) ([]cubeboxservices.FileRecord, error) {
	return nil, nil
}

func (s *runtimeHealthyFileStore) Save(_ context.Context, _ string, _ string, _ string, _ string, _ string, _ io.Reader) (cubeboxservices.FileRecord, error) {
	return cubeboxservices.FileRecord{}, nil
}

func (s *runtimeHealthyFileStore) Delete(_ context.Context, _ string, _ string) (bool, error) {
	return false, nil
}

func (s *runtimeHealthyFileStore) Healthy(context.Context) error {
	return s.healthyErr
}

func TestAssistantNamespaceSegment(t *testing.T) {
	t.Parallel()

	if !assistantNamespaceSegment("assistant") {
		t.Fatal("assistant should be accepted")
	}
	if !assistantNamespaceSegment("cubebox") {
		t.Fatal("cubebox should be accepted")
	}
	if assistantNamespaceSegment("other") {
		t.Fatal("other should be rejected")
	}
}
