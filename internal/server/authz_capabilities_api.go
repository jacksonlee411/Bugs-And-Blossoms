package server

import (
	"net/http"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type authzCapabilitiesResponse struct {
	Capabilities []authz.AuthzCapabilityOption `json:"capabilities"`
	RegistryRev  string                        `json:"registry_rev"`
}

func handleAuthzCapabilitiesAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	values := r.URL.Query()
	if values.Has("include_disabled") || values.Has("include_uncovered") {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "diagnostic_parameter_not_supported", "diagnostic parameter not supported")
		return
	}
	options := authz.ListAuthzCapabilityOptions(authz.CapabilityListFilter{
		Query:             values.Get("q"),
		OwnerModule:       values.Get("owner_module"),
		ScopeDimension:    values.Get("scope_dimension"),
		RequireAssignable: true,
		RequireTenantAPI:  true,
		CoveredKeys:       TenantAPICoveredCapabilityKeys(),
	})

	filtered := options[:0]
	for _, option := range options {
		if option.Status == authz.CapabilityStatusEnabled && option.Assignable && option.Surface == authz.CapabilitySurfaceTenantAPI && option.Covered {
			filtered = append(filtered, option)
		}
	}
	options = filtered

	for i := range options {
		options[i].Label = strings.TrimSpace(options[i].Label)
	}
	writeJSON(w, http.StatusOK, authzCapabilitiesResponse{
		Capabilities: options,
		RegistryRev:  authz.RegistryRevision,
	})
}
