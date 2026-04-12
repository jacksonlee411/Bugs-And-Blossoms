package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

const (
	assistantFormalContractVersion         = "v1"
	assistantSessionInvalidCode            = "assistant_session_invalid"
	assistantPrincipalInvalidCode          = "assistant_principal_invalid"
	assistantUIBootstrapUnavailableCode    = "assistant_ui_bootstrap_unavailable"
	assistantSessionInvalidMessage         = "登录会话已失效，请重新登录。"
	assistantPrincipalInvalidMessage       = "登录主体已失效，请重新登录。"
	assistantUIBootstrapUnavailableMessage = "正式入口启动信息暂不可用，请稍后重试。"
)

type assistantFormalViewer struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

type assistantFormalBootstrapUI struct {
	ModelSelect            bool `json:"model_select"`
	ArtifactsEnabled       bool `json:"artifacts_enabled"`
	AgentsUIEnabled        bool `json:"agents_ui_enabled"`
	MemoryEnabled          bool `json:"memory_enabled"`
	WebSearchEnabled       bool `json:"web_search_enabled"`
	FileSearchEnabled      bool `json:"file_search_enabled"`
	CodeInterpreterEnabled bool `json:"code_interpreter_enabled"`
}

type assistantFormalBootstrapModel struct {
	EndpointKey  string `json:"endpoint_key"`
	EndpointType string `json:"endpoint_type"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Label        string `json:"label"`
}

type assistantFormalBootstrapRuntime struct {
	Status              string `json:"status"`
	RuntimeCutoverMode  string `json:"runtime_cutover_mode"`
	DomainPolicyVersion string `json:"domain_policy_version"`
}

type assistantFormalUIBootstrapResponse struct {
	ContractVersion string                          `json:"contract_version"`
	Viewer          assistantFormalViewer           `json:"viewer"`
	UI              assistantFormalBootstrapUI      `json:"ui"`
	Models          []assistantFormalBootstrapModel `json:"models"`
	Runtime         assistantFormalBootstrapRuntime `json:"runtime"`
}

type assistantFormalSessionResponse struct {
	ContractVersion string                `json:"contract_version"`
	Authenticated   bool                  `json:"authenticated"`
	Viewer          assistantFormalViewer `json:"viewer"`
}

type assistantFormalSessionRefreshResponse struct {
	ContractVersion string                `json:"contract_version"`
	Authenticated   bool                  `json:"authenticated"`
	Viewer          assistantFormalViewer `json:"viewer"`
	RefreshedAt     string                `json:"refreshed_at"`
}

type assistantFormalEntryAPIHandler struct {
	assistantSvc *assistantConversationService
	sessions     sessionStore
}

func newAssistantFormalEntryAPIHandler(assistantSvc *assistantConversationService, sessions sessionStore) *assistantFormalEntryAPIHandler {
	return &assistantFormalEntryAPIHandler{
		assistantSvc: assistantSvc,
		sessions:     sessions,
	}
}

func (h *assistantFormalEntryAPIHandler) handleUIBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		routingWriteMethodNotAllowed(w, r)
		return
	}

	viewer, ok := assistantFormalViewerFromRequest(r)
	if !ok {
		assistantWritePrincipalInvalid(w, r)
		return
	}

	models, ok := assistantFormalBootstrapModels(h.assistantSvc)
	if !ok {
		assistantWriteUIBootstrapUnavailable(w, r, assistantUIBootstrapUnavailableMessage)
		return
	}

	runtimeStatus := assistantRuntimeStatus()
	runtime, ok := assistantFormalBootstrapRuntimeFromStatus(runtimeStatus)
	if !ok {
		assistantWriteUIBootstrapUnavailable(w, r, assistantUIBootstrapUnavailableMessage)
		return
	}

	writeJSON(w, http.StatusOK, assistantFormalUIBootstrapResponse{
		ContractVersion: assistantFormalContractVersion,
		Viewer:          viewer,
		UI:              assistantFormalBootstrapUIFromCapabilities(runtimeStatus.Capabilities),
		Models:          models,
		Runtime:         runtime,
	})
}

func (h *assistantFormalEntryAPIHandler) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	payload, ok := assistantFormalSessionPayload(r)
	if !ok {
		assistantWritePrincipalInvalid(w, r)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (h *assistantFormalEntryAPIHandler) handleSessionRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	payload, ok := assistantFormalSessionPayload(r)
	if !ok {
		assistantWritePrincipalInvalid(w, r)
		return
	}
	writeJSON(w, http.StatusOK, assistantFormalSessionRefreshResponse{
		ContractVersion: payload.ContractVersion,
		Authenticated:   payload.Authenticated,
		Viewer:          payload.Viewer,
		RefreshedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (h *assistantFormalEntryAPIHandler) handleSessionLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	sid, ok := readSID(r)
	if !ok {
		assistantWriteSessionInvalid(w, r)
		return
	}
	if _, ok := assistantFormalViewerFromRequest(r); !ok {
		assistantWritePrincipalInvalid(w, r)
		return
	}
	if h.sessions != nil {
		_ = h.sessions.Revoke(r.Context(), sid)
	}
	clearSIDCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func assistantFormalSessionPayload(r *http.Request) (assistantFormalSessionResponse, bool) {
	viewer, ok := assistantFormalViewerFromRequest(r)
	if !ok {
		return assistantFormalSessionResponse{}, false
	}
	return assistantFormalSessionResponse{
		ContractVersion: assistantFormalContractVersion,
		Authenticated:   true,
		Viewer:          viewer,
	}, true
}

func assistantFormalViewerFromRequest(r *http.Request) (assistantFormalViewer, bool) {
	if _, ok := currentTenant(r.Context()); !ok {
		return assistantFormalViewer{}, false
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		return assistantFormalViewer{}, false
	}
	return assistantFormalViewer{
		ID:       strings.TrimSpace(principal.ID),
		Username: libreChatCompatUsername(principal),
		Email:    strings.TrimSpace(principal.Email),
		Name:     libreChatCompatDisplayName(principal),
		Role:     libreChatCompatRoleForPrincipal(principal),
	}, true
}

func assistantFormalBootstrapUIFromCapabilities(capabilities assistantRuntimeCapabilities) assistantFormalBootstrapUI {
	return assistantFormalBootstrapUI{
		ModelSelect:            true,
		ArtifactsEnabled:       capabilities.ArtifactsEnabled,
		AgentsUIEnabled:        capabilities.AgentsUIEnabled,
		MemoryEnabled:          capabilities.MemoryEnabled,
		WebSearchEnabled:       capabilities.WebSearchEnabled,
		FileSearchEnabled:      capabilities.FileSearchEnabled,
		CodeInterpreterEnabled: capabilities.CodeInterpreterEnabled,
	}
}

func assistantFormalBootstrapRuntimeFromStatus(status assistantRuntimeStatusResponse) (assistantFormalBootstrapRuntime, bool) {
	if strings.TrimSpace(status.Status) == "" {
		return assistantFormalBootstrapRuntime{}, false
	}
	if strings.TrimSpace(status.Capabilities.RuntimeCutoverMode) == "" {
		return assistantFormalBootstrapRuntime{}, false
	}
	if strings.TrimSpace(status.Capabilities.DomainPolicyVersion) == "" {
		return assistantFormalBootstrapRuntime{}, false
	}
	return assistantFormalBootstrapRuntime{
		Status:              status.Status,
		RuntimeCutoverMode:  status.Capabilities.RuntimeCutoverMode,
		DomainPolicyVersion: status.Capabilities.DomainPolicyVersion,
	}, true
}

func assistantFormalBootstrapModels(assistantSvc *assistantConversationService) ([]assistantFormalBootstrapModel, bool) {
	providers, _, _ := assistantStartupProviders(assistantSvc)
	if len(providers) == 0 {
		return nil, false
	}
	out := make([]assistantFormalBootstrapModel, 0, len(providers))
	seen := make(map[string]struct{}, len(providers))
	for _, provider := range providers {
		model := strings.TrimSpace(provider.Model)
		if model == "" {
			continue
		}
		endpointKey, endpointType := libreChatCompatEndpoint(provider)
		entry := assistantFormalBootstrapModel{
			EndpointKey:  endpointKey,
			EndpointType: endpointType,
			Provider:     strings.ToLower(strings.TrimSpace(provider.Name)),
			Model:        model,
			Label:        libreChatCompatEndpointLabel(provider, endpointKey) + " / " + model,
		}
		identity := entry.EndpointKey + "|" + entry.Provider + "|" + entry.Model
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		out = append(out, entry)
	}
	return out, len(out) > 0
}

func assistantWriteSessionInvalid(w http.ResponseWriter, r *http.Request) {
	routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, assistantSessionInvalidCode, assistantSessionInvalidMessage)
}

func assistantWritePrincipalInvalid(w http.ResponseWriter, r *http.Request) {
	routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, assistantPrincipalInvalidCode, assistantPrincipalInvalidMessage)
}

func assistantWriteUIBootstrapUnavailable(w http.ResponseWriter, r *http.Request, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		message = assistantUIBootstrapUnavailableMessage
	}
	routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusServiceUnavailable, assistantUIBootstrapUnavailableCode, message)
}

func isAssistantFormalSuccessorAPIPath(path string) bool {
	switch strings.TrimSpace(path) {
	case "/internal/assistant/ui-bootstrap",
		"/internal/assistant/session",
		"/internal/assistant/session/refresh",
		"/internal/assistant/session/logout":
		return true
	default:
		return false
	}
}
