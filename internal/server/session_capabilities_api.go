package server

import (
	"net/http"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type sessionCapabilitiesResponse struct {
	AuthzCapabilityKeys []string `json:"authz_capability_keys"`
}

type sessionCapabilityRequirement struct {
	Object string
	Action string
}

type policyAllowAuthorizer interface {
	PolicyAllows(subject string, domain string, object string, action string) (bool, error)
}

var sessionCapabilityRequirements = []sessionCapabilityRequirement{
	{Object: authz.ObjectCubeBoxConversations, Action: authz.ActionRead},
	{Object: authz.ObjectCubeBoxConversations, Action: authz.ActionUse},
	{Object: authz.ObjectIAMDictRelease, Action: authz.ActionAdmin},
	{Object: authz.ObjectIAMDicts, Action: authz.ActionAdmin},
	{Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin},
	{Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionRead},
}

func sessionCapabilityAllowed(a authorizer, subject string, domain string, object string, action string) (bool, error) {
	if policyAuthorizer, ok := a.(policyAllowAuthorizer); ok {
		return policyAuthorizer.PolicyAllows(subject, domain, object, action)
	}

	allowed, enforced, err := a.Authorize(subject, domain, object, action)
	if err != nil {
		return false, err
	}
	if !enforced {
		return false, nil
	}
	return allowed, nil
}

func handleSessionCapabilitiesAPI(w http.ResponseWriter, r *http.Request, authorizer authorizer) {
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
	if authorizer == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
		return
	}

	subject := authz.SubjectFromRoleSlug(principal.RoleSlug)
	domain := authz.DomainFromTenantID(tenant.ID)
	keys := make([]string, 0, len(sessionCapabilityRequirements))
	for _, req := range sessionCapabilityRequirements {
		allowed, err := sessionCapabilityAllowed(authorizer, subject, domain, req.Object, req.Action)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
			return
		}
		if allowed {
			keys = append(keys, authz.AuthzCapabilityKey(req.Object, req.Action))
		}
	}

	writeJSON(w, http.StatusOK, sessionCapabilitiesResponse{AuthzCapabilityKeys: keys})
}
