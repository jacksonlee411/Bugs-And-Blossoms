package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

const accessControlProtected = "protected"

type authzAPICatalogResponse struct {
	APIEntries []authzAPICatalogEntry `json:"api_entries"`
}

type authzAPICatalogEntry struct {
	Method             string `json:"method"`
	Path               string `json:"path"`
	AccessControl      string `json:"access_control"`
	OwnerModule        string `json:"owner_module"`
	ResourceLabel      string `json:"resource_label,omitempty"`
	ResourceObject     string `json:"resource_object,omitempty"`
	Action             string `json:"action,omitempty"`
	AuthzCapabilityKey string `json:"authz_capability_key,omitempty"`
	CapabilityStatus   string `json:"capability_status,omitempty"`
	Assignable         bool   `json:"assignable"`
	CubeBoxCallable    bool   `json:"cubebox_callable"`
}

type authzAPICatalogFilter struct {
	Query              string
	Method             string
	AccessControl      string
	OwnerModule        string
	ResourceObject     string
	AuthzCapabilityKey string
}

func handleAuthzAPICatalogAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	values := r.URL.Query()
	filter := authzAPICatalogFilter{
		Query:              values.Get("q"),
		Method:             values.Get("method"),
		AccessControl:      values.Get("access_control"),
		OwnerModule:        values.Get("owner_module"),
		ResourceObject:     values.Get("resource_object"),
		AuthzCapabilityKey: values.Get("authz_capability_key"),
	}
	if filter.AuthzCapabilityKey != "" {
		entry, ok, err := validateTenantAuthzCapabilityKeyFilter(filter.AuthzCapabilityKey)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_authz_capability_key", "invalid authz capability key")
			return
		}
		if !ok {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "unknown_authz_capability_key", "unknown authz capability key")
			return
		}
		if entry.Surface != authz.CapabilitySurfaceTenantAPI {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "authz_capability_key_not_supported", "authz capability key not supported")
			return
		}
		filter.AuthzCapabilityKey = entry.Key
	}

	policyPath, err := authzPolicyPath()
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_catalog_error", "authz catalog error")
		return
	}
	facts, err := CollectAuthzCoverageFacts(policyPath)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_catalog_error", "authz catalog error")
		return
	}

	entries, err := ListAuthzAPICatalogEntries(facts, filter)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_catalog_error", "authz catalog error")
		return
	}
	writeJSON(w, http.StatusOK, authzAPICatalogResponse{APIEntries: entries})
}

func validateTenantAuthzCapabilityKeyFilter(key string) (authz.AuthzCapability, bool, error) {
	object, action, err := authz.ParseAuthzCapabilityKey(key)
	if err != nil {
		return authz.AuthzCapability{}, false, err
	}
	entry, ok := authz.LookupAuthzCapabilityByObjectAction(object, action)
	return entry, ok, nil
}

func ListAuthzAPICatalogEntries(facts AuthzCoverageFacts, filter authzAPICatalogFilter) ([]authzAPICatalogEntry, error) {
	registryByKey := map[string]authz.AuthzCapability{}
	for _, entry := range facts.Registry {
		registryByKey[entry.Key] = entry
	}
	requirementByRoute := map[string]AuthzRouteCoverage{}
	for _, route := range facts.Routes {
		if route.Surface != authz.CapabilitySurfaceTenantAPI {
			continue
		}
		requirementByRoute[routeCoverageID(route.Method, route.Path)] = route
	}
	cubeboxCallableByRoute := map[string]bool{}
	for _, overlay := range facts.ToolOverlays {
		if overlay.Surface != authz.CapabilitySurfaceTenantAPI {
			continue
		}
		cubeboxCallableByRoute[routeCoverageID(overlay.Method, overlay.Path)] = overlay.CubeBoxCallable
	}

	entries := make([]authzAPICatalogEntry, 0, len(facts.AllowlistRoutes))
	for _, route := range facts.AllowlistRoutes {
		if route.Entrypoint != "server" || !authzAllowlistRouteRequiresRequirement(route) {
			continue
		}
		routeID := routeCoverageID(route.Method, route.Path)
		requirement, hasRequirement := requirementByRoute[routeID]
		if !hasRequirement {
			return nil, fmt.Errorf("authz catalog route has no authz requirement: %s", routeID)
		}
		registryEntry, ok := registryByKey[requirement.Key]
		if !ok {
			return nil, fmt.Errorf("authz catalog route %s references unregistered authz capability key %s", routeID, requirement.Key)
		}
		if registryEntry.Surface != authz.CapabilitySurfaceTenantAPI || registryEntry.Status != authz.CapabilityStatusEnabled || !registryEntry.Assignable {
			continue
		}
		entry := authzAPICatalogEntry{
			Method:             route.Method,
			Path:               route.Path,
			AccessControl:      accessControlProtected,
			OwnerModule:        registryEntry.OwnerModule,
			ResourceLabel:      registryEntry.ResourceLabel,
			ResourceObject:     registryEntry.Object,
			Action:             registryEntry.Action,
			AuthzCapabilityKey: registryEntry.Key,
			CapabilityStatus:   registryEntry.Status,
			Assignable:         registryEntry.Assignable,
			CubeBoxCallable:    cubeboxCallableByRoute[routeID],
		}
		if matchesAuthzAPICatalogFilter(entry, filter) {
			entries = append(entries, entry)
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Path == entries[j].Path {
			return entries[i].Method < entries[j].Method
		}
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

func matchesAuthzAPICatalogFilter(entry authzAPICatalogEntry, filter authzAPICatalogFilter) bool {
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	method := strings.ToUpper(strings.TrimSpace(filter.Method))
	accessControl := strings.TrimSpace(filter.AccessControl)
	ownerModule := strings.TrimSpace(filter.OwnerModule)
	resourceObject := strings.TrimSpace(filter.ResourceObject)
	authzCapabilityKey := strings.TrimSpace(filter.AuthzCapabilityKey)

	if method != "" && entry.Method != method {
		return false
	}
	if accessControl != "" && entry.AccessControl != accessControl {
		return false
	}
	if ownerModule != "" && entry.OwnerModule != ownerModule {
		return false
	}
	if resourceObject != "" && entry.ResourceObject != resourceObject {
		return false
	}
	if authzCapabilityKey != "" && entry.AuthzCapabilityKey != authzCapabilityKey {
		return false
	}
	if query == "" {
		return true
	}
	fields := []string{
		entry.Method,
		entry.Path,
		entry.ResourceObject,
		entry.AuthzCapabilityKey,
		entry.ResourceLabel,
		entry.Action,
		entry.OwnerModule,
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}
