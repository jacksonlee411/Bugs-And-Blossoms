package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	cubeboxservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services"
)

type cubeboxRuntimeComponentStatus struct {
	Healthy string `json:"healthy"`
	Reason  string `json:"reason,omitempty"`
}

type cubeboxRuntimeCapabilities struct {
	ConversationEnabled bool `json:"conversation_enabled"`
	FilesEnabled        bool `json:"files_enabled"`
	AgentsUIEnabled     bool `json:"agents_ui_enabled"`
	AgentsWriteEnabled  bool `json:"agents_write_enabled"`
	MemoryEnabled       bool `json:"memory_enabled"`
	WebSearchEnabled    bool `json:"web_search_enabled"`
	FileSearchEnabled   bool `json:"file_search_enabled"`
	MCPEnabled          bool `json:"mcp_enabled"`
}

type cubeboxRuntimeStatusResponse struct {
	Status              string                        `json:"status"`
	CheckedAt           string                        `json:"checked_at"`
	Frontend            cubeboxRuntimeComponentStatus `json:"frontend"`
	Backend             cubeboxRuntimeComponentStatus `json:"backend"`
	KnowledgeRuntime    cubeboxRuntimeComponentStatus `json:"knowledge_runtime"`
	ModelGateway        cubeboxRuntimeComponentStatus `json:"model_gateway"`
	FileStore           cubeboxRuntimeComponentStatus `json:"file_store"`
	RetiredCapabilities []string                      `json:"retired_capabilities"`
	Capabilities        cubeboxRuntimeCapabilities    `json:"capabilities"`
}

func handleCubeBoxConversationsAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	handleAssistantConversationsAPI(w, r, svc)
}

func handleCubeBoxConversationDetailAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method == http.MethodDelete {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotImplemented, "cubebox_conversation_delete_not_implemented", "cubebox conversation delete not implemented")
		return
	}
	handleAssistantConversationDetailAPI(w, r, svc)
}

func handleCubeBoxConversationTurnsAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	handleAssistantConversationTurnsAPI(w, r, svc)
}

func handleCubeBoxTurnActionAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	proxyCubeBoxTaskPollURIResponse(w, r, func(rec *httptest.ResponseRecorder) {
		handleAssistantTurnActionAPI(rec, r, svc)
	})
}

func handleCubeBoxTasksAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	proxyCubeBoxTaskPollURIResponse(w, r, func(rec *httptest.ResponseRecorder) {
		handleAssistantTasksAPI(rec, r, svc)
	})
}

func handleCubeBoxTaskDetailAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	handleAssistantTaskDetailAPI(w, r, svc)
}

func handleCubeBoxTaskActionAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	handleAssistantTaskActionAPI(w, r, svc)
}

func handleCubeBoxModelsAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	handleAssistantModelsAPI(w, r, svc)
}

func handleCubeBoxRuntimeStatusAPI(w http.ResponseWriter, r *http.Request, assistantSvc *assistantConversationService, fileSvc *cubeboxservices.FileService) {
	if r.Method != http.MethodGet {
		routingWriteMethodNotAllowed(w, r)
		return
	}

	resp := cubeboxRuntimeStatusResponse{
		Status:    assistantRuntimeHealthHealthy,
		CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Frontend:  cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthHealthy},
		Backend:   cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthHealthy},
		Capabilities: cubeboxRuntimeCapabilities{
			ConversationEnabled: true,
			FilesEnabled:        true,
			AgentsUIEnabled:     false,
			AgentsWriteEnabled:  false,
			MemoryEnabled:       false,
			WebSearchEnabled:    false,
			FileSearchEnabled:   false,
			MCPEnabled:          false,
		},
		RetiredCapabilities: []string{
			"librechat_web_ui",
			"agents",
			"memory",
			"web_search",
			"file_search",
			"mcp",
		},
	}

	if assistantSvc == nil {
		resp.Backend = cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthUnavailable, Reason: "assistant_service_missing"}
		resp.Status = assistantRuntimeHealthUnavailable
	} else {
		resp.KnowledgeRuntime = cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthHealthy}
		if assistantSvc.knowledgeErr != nil {
			resp.KnowledgeRuntime = cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthUnavailable, Reason: "knowledge_runtime_unavailable"}
			resp.Status = assistantRuntimeHealthDegraded
		}

		resp.ModelGateway = cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthHealthy}
		switch {
		case assistantSvc.modelGateway == nil:
			resp.ModelGateway = cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthUnavailable, Reason: "model_gateway_missing"}
			resp.Status = assistantRuntimeHealthUnavailable
		case assistantSvc.gatewayErr != nil:
			resp.ModelGateway = cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthUnavailable, Reason: "model_gateway_unavailable"}
			resp.Status = assistantRuntimeHealthUnavailable
		}
	}

	resp.FileStore = cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthHealthy}
	if fileSvc == nil {
		resp.FileStore = cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthUnavailable, Reason: "file_store_missing"}
		resp.Status = assistantRuntimeHealthUnavailable
	} else if err := fileSvc.Healthy(context.Background()); err != nil {
		resp.FileStore = cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthUnavailable, Reason: "file_store_unavailable"}
		resp.Status = assistantRuntimeHealthUnavailable
	}

	if resp.KnowledgeRuntime.Healthy == "" {
		resp.KnowledgeRuntime = cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthUnavailable, Reason: "knowledge_runtime_missing"}
		resp.Status = assistantRuntimeHealthUnavailable
	}
	if resp.ModelGateway.Healthy == "" {
		resp.ModelGateway = cubeboxRuntimeComponentStatus{Healthy: assistantRuntimeHealthUnavailable, Reason: "model_gateway_missing"}
		resp.Status = assistantRuntimeHealthUnavailable
	}

	writeJSON(w, http.StatusOK, resp)
}

func proxyCubeBoxTaskPollURIResponse(w http.ResponseWriter, r *http.Request, next func(rec *httptest.ResponseRecorder)) {
	rec := httptest.NewRecorder()
	next(rec)

	result := rec.Result()
	defer result.Body.Close()

	for key, values := range result.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	body := rec.Body.Bytes()
	if shouldRewriteCubeBoxTaskPollURI(result.Header.Get("Content-Type"), body) {
		body = rewriteCubeBoxTaskPollURI(body)
	}

	w.WriteHeader(rec.Code)
	if len(body) > 0 {
		_, _ = w.Write(body)
	}
}

func shouldRewriteCubeBoxTaskPollURI(contentType string, body []byte) bool {
	if len(body) == 0 {
		return false
	}
	if !strings.Contains(strings.ToLower(strings.TrimSpace(contentType)), "application/json") {
		return false
	}
	return strings.Contains(string(body), "/internal/assistant/tasks/")
}

func rewriteCubeBoxTaskPollURI(body []byte) []byte {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	raw, ok := payload["poll_uri"].(string)
	if !ok {
		return body
	}
	payload["poll_uri"] = cubeboxTaskPollURI(raw)
	rewritten, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return rewritten
}

func cubeboxTaskPollURI(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "/internal/assistant/tasks/") {
		return strings.Replace(trimmed, "/internal/assistant/tasks/", "/internal/cubebox/tasks/", 1)
	}
	return trimmed
}
