package server

import (
	"encoding/base64"
	"net/http"
	"sort"
	"strings"
)

const (
	libreChatCompatRoleUser         = "USER"
	libreChatCompatProvider         = "bugs-and-blossoms-sid"
	libreChatCompatDefaultTimestamp = "1970-01-01T00:00:00Z"
	libreChatCompatDefaultAvatar    = ""
)

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
