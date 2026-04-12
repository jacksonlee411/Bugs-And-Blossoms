package server

import (
	"encoding/base64"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

const (
	libreChatCompatAPIPrefix         = libreChatStaticPrefix + "/api"
	libreChatFormalEntryAPIPrefix    = libreChatFormalEntryPrefix + "/api"
	libreChatCompatRoleUser          = "USER"
	libreChatCompatRoleAdmin         = "ADMIN"
	libreChatCompatProvider          = "bugs-and-blossoms-sid"
	libreChatCompatProjectID         = "bugs-and-blossoms"
	libreChatCompatDefaultTimestamp  = "1970-01-01T00:00:00Z"
	libreChatCompatDefaultAvatar     = ""
	libreChatCompatDefaultHelpAndFAQ = ""
)

type libreChatCompatAPIHandler struct {
	assistantSvc *assistantConversationService
	sessions     sessionStore
}

type libreChatCompatRefreshResponse struct {
	Token string                  `json:"token"`
	User  libreChatCompatUserView `json:"user"`
}

type libreChatCompatUserView struct {
	ID               string                                  `json:"id"`
	Username         string                                  `json:"username"`
	Email            string                                  `json:"email"`
	Name             string                                  `json:"name"`
	Avatar           string                                  `json:"avatar"`
	Role             string                                  `json:"role"`
	Provider         string                                  `json:"provider"`
	Plugins          []string                                `json:"plugins,omitempty"`
	TwoFactorEnabled bool                                    `json:"twoFactorEnabled,omitempty"`
	Personalization  *libreChatCompatUserPersonalizationView `json:"personalization,omitempty"`
	CreatedAt        string                                  `json:"createdAt"`
	UpdatedAt        string                                  `json:"updatedAt"`
}

type libreChatCompatUserPersonalizationView struct {
	Memories bool `json:"memories"`
}

type libreChatCompatRoleView struct {
	Name        string                     `json:"name"`
	Permissions map[string]map[string]bool `json:"permissions"`
}

func newLibreChatCompatAPIHandler(assistantSvc *assistantConversationService, sessions sessionStore) http.Handler {
	return &libreChatCompatAPIHandler{assistantSvc: assistantSvc, sessions: sessions}
}

func (h *libreChatCompatAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	suffix, ok := libreChatCompatAPISuffix(r.URL.Path)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "not_found", "未找到兼容接口。")
		return
	}
	switch suffix {
	case "/auth/refresh":
		h.handleRefresh(w, r)
	case "/auth/logout":
		h.handleLogout(w, r)
	case "/user":
		h.handleUser(w, r)
	case "/roles/user":
		h.handleRole(w, r, libreChatCompatRoleUser)
	case "/roles/admin":
		h.handleRole(w, r, libreChatCompatRoleAdmin)
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "not_found", "未找到兼容接口。")
	}
}

func (h *libreChatCompatAPIHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "当前请求方法不被允许。")
		return
	}
	user, token, ok := libreChatCompatUserFromRequest(r)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "assistant_vendored_principal_invalid", "正式入口认证主体缺失，请重新登录。")
		return
	}
	writeJSON(w, http.StatusOK, libreChatCompatRefreshResponse{Token: token, User: user})
}

func (h *libreChatCompatAPIHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "当前请求方法不被允许。")
		return
	}
	if sid, ok := readSID(r); ok && h.sessions != nil {
		_ = h.sessions.Revoke(r.Context(), sid)
	}
	clearSIDCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *libreChatCompatAPIHandler) handleUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "当前请求方法不被允许。")
		return
	}
	user, _, ok := libreChatCompatUserFromRequest(r)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "assistant_vendored_principal_invalid", "正式入口认证主体缺失，请重新登录。")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *libreChatCompatAPIHandler) handleRole(w http.ResponseWriter, r *http.Request, roleName string) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "当前请求方法不被允许。")
		return
	}
	if _, _, ok := libreChatCompatUserFromRequest(r); !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "assistant_vendored_principal_invalid", "正式入口认证主体缺失，请重新登录。")
		return
	}
	writeJSON(w, http.StatusOK, libreChatCompatRoleView{
		Name:        roleName,
		Permissions: libreChatCompatPermissions(roleName),
	})
}

func (h *libreChatCompatAPIHandler) compatProviders() ([]assistantModelProviderConfig, string, string) {
	return assistantStartupProviders(h.assistantSvc)
}

func libreChatCompatUserFromRequest(r *http.Request) (libreChatCompatUserView, string, bool) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		return libreChatCompatUserView{}, "", false
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		return libreChatCompatUserView{}, "", false
	}
	sid, ok := readSID(r)
	if !ok {
		return libreChatCompatUserView{}, "", false
	}
	username := libreChatCompatUsername(principal)
	return libreChatCompatUserView{
		ID:               principal.ID,
		Username:         username,
		Email:            principal.Email,
		Name:             libreChatCompatDisplayName(principal),
		Avatar:           libreChatCompatDefaultAvatar,
		Role:             libreChatCompatRoleForPrincipal(principal),
		Provider:         libreChatCompatProvider,
		Plugins:          nil,
		TwoFactorEnabled: false,
		Personalization:  &libreChatCompatUserPersonalizationView{Memories: false},
		CreatedAt:        libreChatCompatDefaultTimestamp,
		UpdatedAt:        libreChatCompatDefaultTimestamp,
	}, libreChatCompatToken(tenant.ID, principal.ID, sid), true
}

func libreChatCompatToken(tenantID string, principalID string, sid string) string {
	raw := strings.TrimSpace(tenantID) + ":" + strings.TrimSpace(principalID) + ":" + strings.TrimSpace(sid)
	return "compat-sid." + base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func libreChatCompatUsername(principal Principal) string {
	email := strings.TrimSpace(principal.Email)
	if email == "" {
		return strings.TrimSpace(principal.ID)
	}
	name, _, found := strings.Cut(email, "@")
	if !found || strings.TrimSpace(name) == "" {
		return email
	}
	return name
}

func libreChatCompatDisplayName(principal Principal) string {
	username := libreChatCompatUsername(principal)
	username = strings.ReplaceAll(username, ".", " ")
	username = strings.ReplaceAll(username, "-", " ")
	username = strings.ReplaceAll(username, "_", " ")
	username = strings.TrimSpace(username)
	if username == "" {
		return strings.TrimSpace(principal.ID)
	}
	parts := strings.Fields(username)
	for idx := range parts {
		parts[idx] = strings.ToUpper(parts[idx][:1]) + strings.ToLower(parts[idx][1:])
	}
	return strings.Join(parts, " ")
}

func libreChatCompatRoleForPrincipal(_ Principal) string {
	return libreChatCompatRoleUser
}

func libreChatCompatPermissions(roleName string) map[string]map[string]bool {
	user := map[string]map[string]bool{
		"PROMPTS": {
			"SHARED_GLOBAL": false,
			"USE":           true,
			"CREATE":        true,
		},
		"BOOKMARKS": {
			"USE": true,
		},
		"MEMORIES": {
			"USE":     false,
			"CREATE":  false,
			"UPDATE":  false,
			"READ":    false,
			"OPT_OUT": false,
		},
		"AGENTS": {
			"SHARED_GLOBAL": false,
			"USE":           false,
			"CREATE":        false,
		},
		"MULTI_CONVO": {
			"USE": true,
		},
		"TEMPORARY_CHAT": {
			"USE": false,
		},
		"RUN_CODE": {
			"USE": false,
		},
		"WEB_SEARCH": {
			"USE": false,
		},
		"PEOPLE_PICKER": {
			"VIEW_USERS":  false,
			"VIEW_GROUPS": false,
			"VIEW_ROLES":  false,
		},
		"MARKETPLACE": {
			"USE": false,
		},
		"FILE_SEARCH": {
			"USE": false,
		},
		"FILE_CITATIONS": {
			"USE": false,
		},
	}
	if roleName == libreChatCompatRoleAdmin {
		return user
	}
	return user
}

func libreChatCompatEndpoint(provider assistantModelProviderConfig) (string, string) {
	name := strings.ToLower(strings.TrimSpace(provider.Name))
	switch name {
	case "openai":
		return "openAI", "openAI"
	case "anthropic", "claude":
		return "anthropic", "anthropic"
	case "google", "gemini":
		return "google", "google"
	case "bedrock":
		return "bedrock", "bedrock"
	case "azureopenai", "azure-openai", "azure_openai":
		return "azureOpenAI", "azureOpenAI"
	case "assistants":
		return "assistants", "assistants"
	case "azureassistants", "azure-assistants", "azure_assistants":
		return "azureAssistants", "azureAssistants"
	default:
		if name == "" {
			return "custom", "custom"
		}
		return name, "custom"
	}
}

func libreChatCompatEndpointLabel(provider assistantModelProviderConfig, endpointKey string) string {
	switch endpointKey {
	case "openAI":
		return "OpenAI"
	case "anthropic":
		return "Anthropic"
	case "google":
		return "Google"
	case "bedrock":
		return "Bedrock"
	case "azureOpenAI":
		return "Azure OpenAI"
	case "assistants":
		return "Assistants"
	case "azureAssistants":
		return "Azure Assistants"
	default:
		return libreChatCompatTitle(strings.TrimSpace(provider.Name))
	}
}

func libreChatCompatTitle(v string) string {
	v = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(v, "-", " "), "_", " "))
	if v == "" {
		return "Custom"
	}
	parts := strings.Fields(v)
	for idx := range parts {
		parts[idx] = strings.ToUpper(parts[idx][:1]) + strings.ToLower(parts[idx][1:])
	}
	return strings.Join(parts, " ")
}

func libreChatCompatModelExists(models []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, model := range models {
		if strings.TrimSpace(model) == target {
			return true
		}
	}
	return false
}

func assistantStartupProviders(assistantSvc *assistantConversationService) ([]assistantModelProviderConfig, string, string) {
	if assistantSvc == nil || assistantSvc.modelGateway == nil {
		return nil, "ai_runtime_config_missing", "Assistant 运行时模型配置缺失，请先完成配置。"
	}
	providers := assistantSvc.modelGateway.listModels()
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].Priority == providers[j].Priority {
			return providers[i].Name < providers[j].Name
		}
		return providers[i].Priority < providers[j].Priority
	})
	if len(providers) == 0 {
		return nil, "assistant_startup_endpoints_unavailable", "正式入口缺少可用 endpoint 配置，请检查 Assistant 运行时模型配置。"
	}
	return providers, "", ""
}

func libreChatCompatAPISuffix(path string) (string, bool) {
	for _, prefix := range []string{libreChatCompatAPIPrefix, libreChatFormalEntryAPIPrefix} {
		if path == prefix {
			return "", true
		}
		if strings.HasPrefix(path, prefix+"/") {
			return strings.TrimPrefix(path, prefix), true
		}
	}
	return "", false
}

func isLibreChatCompatAPIPath(path string) bool {
	_, ok := libreChatCompatAPISuffix(path)
	return ok
}

var _ = time.RFC3339
