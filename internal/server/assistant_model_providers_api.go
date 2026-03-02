package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type assistantModelProvidersResponse struct {
	ProviderRouting assistantProviderRouting           `json:"provider_routing"`
	Providers       []assistantModelProviderConfigView `json:"providers"`
}

type assistantModelProviderConfigView struct {
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	Model        string `json:"model"`
	Endpoint     string `json:"endpoint"`
	TimeoutMS    int    `json:"timeout_ms"`
	Retries      int    `json:"retries"`
	Priority     int    `json:"priority"`
	KeyRef       string `json:"key_ref"`
	Healthy      string `json:"healthy"`
	HealthReason string `json:"health_reason,omitempty"`
}

type assistantModelConfigPayload struct {
	ProviderRouting assistantProviderRouting       `json:"provider_routing"`
	Providers       []assistantModelProviderConfig `json:"providers"`
}

type assistantModelProvidersValidateResponse struct {
	Valid      bool                        `json:"valid"`
	Errors     []string                    `json:"errors,omitempty"`
	Normalized assistantModelConfigPayload `json:"normalized"`
}

type assistantModelProvidersApplyResponse struct {
	AppliedAt  string                      `json:"applied_at"`
	AppliedBy  string                      `json:"applied_by"`
	Normalized assistantModelConfigPayload `json:"normalized"`
}

type assistantModelsResponse struct {
	Models []assistantModelEntry `json:"models"`
}

type assistantModelEntry struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func handleAssistantModelProvidersAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil || svc.modelGateway == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_service_missing", "assistant service missing")
		return
	}
	providers, statuses := svc.modelGateway.listProviderStatus()
	statusByName := make(map[string]assistantProviderStatus, len(statuses))
	for _, status := range statuses {
		statusByName[strings.ToLower(strings.TrimSpace(status.Name))] = status
	}
	rows := make([]assistantModelProviderConfigView, 0, len(providers))
	for _, provider := range providers {
		status := statusByName[strings.ToLower(strings.TrimSpace(provider.Name))]
		rows = append(rows, assistantModelProviderConfigView{
			Name:         provider.Name,
			Enabled:      provider.Enabled,
			Model:        provider.Model,
			Endpoint:     provider.Endpoint,
			TimeoutMS:    provider.TimeoutMS,
			Retries:      provider.Retries,
			Priority:     provider.Priority,
			KeyRef:       provider.KeyRef,
			Healthy:      status.Healthy,
			HealthReason: status.HealthReason,
		})
	}
	cfg := svc.modelGateway.snapshot()
	writeJSON(w, http.StatusOK, assistantModelProvidersResponse{
		ProviderRouting: cfg.ProviderRouting,
		Providers:       rows,
	})
}

func handleAssistantModelProvidersValidateAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil || svc.modelGateway == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_service_missing", "assistant service missing")
		return
	}
	var payload assistantModelConfigPayload
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	normalized, errs := svc.modelGateway.validateConfig(assistantModelConfig{
		ProviderRouting: payload.ProviderRouting,
		Providers:       payload.Providers,
	})
	resp := assistantModelProvidersValidateResponse{
		Valid:  len(errs) == 0,
		Errors: errs,
		Normalized: assistantModelConfigPayload{
			ProviderRouting: normalized.ProviderRouting,
			Providers:       normalized.Providers,
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleAssistantModelProvidersApplyAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil || svc.modelGateway == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_service_missing", "assistant service missing")
		return
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var payload assistantModelConfigPayload
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	normalized, errs := svc.modelGateway.applyConfig(assistantModelConfig{
		ProviderRouting: payload.ProviderRouting,
		Providers:       payload.Providers,
	})
	if len(errs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, assistantModelProvidersValidateResponse{
			Valid:  false,
			Errors: errs,
			Normalized: assistantModelConfigPayload{
				ProviderRouting: normalized.ProviderRouting,
				Providers:       normalized.Providers,
			},
		})
		return
	}
	writeJSON(w, http.StatusOK, assistantModelProvidersApplyResponse{
		AppliedAt: time.Now().UTC().Format(time.RFC3339Nano),
		AppliedBy: principal.ID,
		Normalized: assistantModelConfigPayload{
			ProviderRouting: normalized.ProviderRouting,
			Providers:       normalized.Providers,
		},
	})
}

func handleAssistantModelsAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil || svc.modelGateway == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_service_missing", "assistant service missing")
		return
	}
	providers := svc.modelGateway.listModels()
	models := make([]assistantModelEntry, 0, len(providers))
	for _, provider := range providers {
		models = append(models, assistantModelEntry{
			Provider: provider.Name,
			Model:    provider.Model,
		})
	}
	writeJSON(w, http.StatusOK, assistantModelsResponse{Models: models})
}
