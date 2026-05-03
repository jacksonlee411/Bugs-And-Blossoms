package server

import (
	"net/http"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type sessionCapabilitiesResponse struct {
	AuthzCapabilityKeys []string `json:"authz_capability_keys"`
}

func handleSessionCapabilitiesAPI(w http.ResponseWriter, r *http.Request, runtime authzRuntimeStore) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "principal_missing", "principal missing")
		return
	}
	if runtime == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
		return
	}

	keys, err := runtime.CapabilitiesForPrincipal(r.Context(), tenant.ID, principal.ID)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
		return
	}

	writeJSON(w, http.StatusOK, sessionCapabilitiesResponse{AuthzCapabilityKeys: keys})
}
