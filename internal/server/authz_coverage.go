package server

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type AuthzCoverageFacts struct {
	AllowlistRoutes []AuthzAllowlistRoute
	Routes          []AuthzRouteCoverage
	ToolOverlays    []AuthzToolOverlayCoverage
	RoleSeeds       []AuthzRoleSeedCoverage
	Registry        []authz.AuthzCapability
	PolicyGrants    []authz.PolicyGrant
}

type AuthzAllowlistRoute struct {
	Entrypoint  string
	Method      string
	Path        string
	RouteClass  string
	OwnerModule string
}

type AuthzRouteCoverage struct {
	Method  string
	Path    string
	Object  string
	Action  string
	Surface string
	Key     string
}

type AuthzToolOverlayCoverage struct {
	Method          string
	Path            string
	CubeBoxCallable bool
	Surface         string
}

type AuthzRoleSeedCoverage struct {
	TenantID           string
	RoleSlug           string
	AuthzCapabilityKey string
	SystemManaged      bool
}

func CollectAuthzCoverageFacts(policyPath string) (AuthzCoverageFacts, error) {
	allowlistPath, err := authzCoverageAllowlistPath()
	if err != nil {
		return AuthzCoverageFacts{}, err
	}
	return CollectAuthzCoverageFactsWithAllowlist(policyPath, allowlistPath)
}

func CollectAuthzAPICatalogRuntimeFacts() (AuthzCoverageFacts, error) {
	if err := authz.ValidateRegistry(); err != nil {
		return AuthzCoverageFacts{}, err
	}
	allowlistPath, err := authzCoverageAllowlistPath()
	if err != nil {
		return AuthzCoverageFacts{}, err
	}
	allowlist, err := routing.LoadAllowlist(allowlistPath)
	if err != nil {
		return AuthzCoverageFacts{}, err
	}
	return AuthzCoverageFacts{
		AllowlistRoutes: ListAuthzAllowlistRoutes(allowlist),
		Routes:          ListAuthzRouteCoverage(),
		ToolOverlays:    ListAuthzToolOverlayCoverage(),
		Registry:        authz.ListAuthzCapabilities(),
	}, nil
}

func CollectAuthzCoverageFactsWithAllowlist(policyPath string, allowlistPath string) (AuthzCoverageFacts, error) {
	if err := authz.ValidateRegistry(); err != nil {
		return AuthzCoverageFacts{}, err
	}
	grants, err := authz.ReadPolicyGrants(policyPath)
	if err != nil {
		return AuthzCoverageFacts{}, err
	}
	var allowlistRoutes []AuthzAllowlistRoute
	if allowlistPath != "" {
		allowlist, err := routing.LoadAllowlist(allowlistPath)
		if err != nil {
			return AuthzCoverageFacts{}, err
		}
		allowlistRoutes = ListAuthzAllowlistRoutes(allowlist)
	}
	return AuthzCoverageFacts{
		AllowlistRoutes: allowlistRoutes,
		Routes:          ListAuthzRouteCoverage(),
		ToolOverlays:    ListAuthzToolOverlayCoverage(),
		RoleSeeds:       ListAuthzRoleSeedCoverage(),
		Registry:        authz.ListAuthzCapabilities(),
		PolicyGrants:    grants,
	}, nil
}

func ListAuthzAllowlistRoutes(allowlist routing.Allowlist) []AuthzAllowlistRoute {
	var out []AuthzAllowlistRoute
	for entrypoint, ep := range allowlist.Entrypoints {
		for _, route := range ep.Routes {
			for _, method := range route.Methods {
				out = append(out, AuthzAllowlistRoute{
					Entrypoint:  entrypoint,
					Method:      method,
					Path:        route.Path,
					RouteClass:  route.RouteClass,
					OwnerModule: ownerModuleForAllowlistRoute(entrypoint, route),
				})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Entrypoint != out[j].Entrypoint {
			return out[i].Entrypoint < out[j].Entrypoint
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

func ownerModuleForAllowlistRoute(entrypoint string, route routing.Route) string {
	path := strings.TrimSpace(route.Path)
	switch {
	case entrypoint == "superadmin":
		return "superadmin"
	case path == "/health" || path == "/healthz" || strings.HasPrefix(path, "/assets/"):
		return "platform"
	case path == "/logout" || strings.HasPrefix(path, "/iam/"):
		return "iam"
	case strings.HasPrefix(path, "/org/"):
		return "orgunit"
	case strings.HasPrefix(path, "/internal/cubebox/"):
		return "cubebox"
	default:
		return "platform"
	}
}

func ListAuthzRouteCoverage() []AuthzRouteCoverage {
	requirements := listRouteRequirements()
	superadminRequirements := listSuperadminRouteRequirements()
	out := make([]AuthzRouteCoverage, 0, len(requirements)+len(superadminRequirements))
	for _, req := range requirements {
		out = append(out, AuthzRouteCoverage{
			Method:  req.Method,
			Path:    req.Path,
			Object:  req.Object,
			Action:  req.Action,
			Surface: req.Surface,
			Key:     authz.AuthzCapabilityKey(req.Object, req.Action),
		})
	}
	for _, req := range superadminRequirements {
		out = append(out, req)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Method < out[j].Method
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func routeCoverageID(method string, path string) string {
	return method + " " + path
}

func listSuperadminRouteRequirements() []AuthzRouteCoverage {
	return []AuthzRouteCoverage{
		superadminRouteCoverage(http.MethodGet, "/superadmin/login", authz.ObjectSuperadminSession, authz.ActionRead),
		superadminRouteCoverage(http.MethodPost, "/superadmin/login", authz.ObjectSuperadminSession, authz.ActionAdmin),
		superadminRouteCoverage(http.MethodPost, "/superadmin/logout", authz.ObjectSuperadminSession, authz.ActionAdmin),
		superadminRouteCoverage(http.MethodGet, "/superadmin/tenants", authz.ObjectSuperadminTenants, authz.ActionRead),
		superadminRouteCoverage(http.MethodPost, "/superadmin/tenants", authz.ObjectSuperadminTenants, authz.ActionAdmin),
		superadminRouteCoverage(http.MethodPost, "/superadmin/tenants/{tenant_id}/enable", authz.ObjectSuperadminTenants, authz.ActionAdmin),
		superadminRouteCoverage(http.MethodPost, "/superadmin/tenants/{tenant_id}/disable", authz.ObjectSuperadminTenants, authz.ActionAdmin),
		superadminRouteCoverage(http.MethodPost, "/superadmin/tenants/{tenant_id}/domains", authz.ObjectSuperadminTenants, authz.ActionAdmin),
	}
}

func superadminRouteCoverage(method string, path string, object string, action string) AuthzRouteCoverage {
	return AuthzRouteCoverage{
		Method:  method,
		Path:    path,
		Object:  object,
		Action:  action,
		Surface: authz.CapabilitySurfaceSuperadminRoute,
		Key:     authz.AuthzCapabilityKey(object, action),
	}
}

func TenantAPICoveredCapabilityKeys() map[string]bool {
	allowlistPath, err := authzCoverageAllowlistPath()
	if err != nil {
		return map[string]bool{}
	}
	allowlist, err := routing.LoadAllowlist(allowlistPath)
	if err != nil {
		return map[string]bool{}
	}
	return TenantAPICoveredCapabilityKeysForAllowlist(allowlist)
}

func TenantAPICoveredCapabilityKeysForAllowlist(allowlist routing.Allowlist) map[string]bool {
	return tenantAPICoveredCapabilityKeys(ListAuthzAllowlistRoutes(allowlist), ListAuthzRouteCoverage())
}

func authzCoverageAllowlistPath() (string, error) {
	if path := os.Getenv("ALLOWLIST_PATH"); path != "" {
		return path, nil
	}
	return defaultAllowlistPath()
}

func tenantAPICoveredCapabilityKeys(allowlistRoutes []AuthzAllowlistRoute, routeRequirements []AuthzRouteCoverage) map[string]bool {
	requirementByID := map[string]AuthzRouteCoverage{}
	for _, route := range routeRequirements {
		requirementByID[routeCoverageID(route.Method, route.Path)] = route
	}

	covered := map[string]bool{}
	for _, route := range allowlistRoutes {
		if !authzAllowlistRouteRequiresRequirement(route) {
			continue
		}
		requirement, ok := requirementByID[routeCoverageID(route.Method, route.Path)]
		if !ok || requirement.Surface != authz.CapabilitySurfaceTenantAPI {
			continue
		}
		covered[requirement.Key] = true
	}
	return covered
}

func ListAuthzRoleSeedCoverage() []AuthzRoleSeedCoverage {
	out := make([]AuthzRoleSeedCoverage, 0)
	for _, key := range builtinTenantAdminCapabilityKeys() {
		out = append(out, AuthzRoleSeedCoverage{
			RoleSlug:           authz.RoleTenantAdmin,
			AuthzCapabilityKey: key,
			SystemManaged:      true,
		})
	}
	for _, key := range builtinTenantViewerCapabilityKeys() {
		out = append(out, AuthzRoleSeedCoverage{
			RoleSlug:           authz.RoleTenantViewer,
			AuthzCapabilityKey: key,
			SystemManaged:      true,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RoleSlug != out[j].RoleSlug {
			return out[i].RoleSlug < out[j].RoleSlug
		}
		return out[i].AuthzCapabilityKey < out[j].AuthzCapabilityKey
	})
	return out
}

func LintAuthzCoverage(facts AuthzCoverageFacts) []error {
	var errs []error
	registryByKey := map[string]authz.AuthzCapability{}
	tenantAPICovered := map[string]bool{}
	superadminRouteCovered := map[string]bool{}
	routeRequirementByID := map[string]AuthzRouteCoverage{}
	allowlistRouteByID := map[string]AuthzAllowlistRoute{}
	requiredAllowlistRouteByID := map[string]AuthzAllowlistRoute{}

	for _, entry := range facts.Registry {
		registryByKey[entry.Key] = entry
	}
	seenRoutes := map[string]bool{}
	for _, route := range facts.Routes {
		routeID := routeCoverageID(route.Method, route.Path)
		routeRequirementByID[routeID] = route
		if seenRoutes[routeID] {
			errs = append(errs, fmt.Errorf("duplicate route requirement: %s", routeID))
		}
		seenRoutes[routeID] = true
		entry, ok := registryByKey[route.Key]
		if !ok {
			errs = append(errs, fmt.Errorf("route requirement %s references unregistered authz capability key %s", routeID, route.Key))
			continue
		}
		if route.Surface != entry.Surface {
			errs = append(errs, fmt.Errorf("route requirement %s surface %s does not match registry surface %s for %s", routeID, route.Surface, entry.Surface, route.Key))
		}
	}

	for _, route := range facts.AllowlistRoutes {
		routeID := routeCoverageID(route.Method, route.Path)
		allowlistRouteByID[routeID] = route
		if !authzAllowlistRouteRequiresRequirement(route) {
			continue
		}
		requiredAllowlistRouteByID[routeID] = route
		requirement, ok := routeRequirementByID[routeID]
		if !ok {
			errs = append(errs, fmt.Errorf("allowlist route %s/%s has no authz requirement: %s", route.Entrypoint, route.RouteClass, routeID))
			continue
		}
		if route.RouteClass == string(routing.RouteClassInternalAPI) && requirement.Surface != authz.CapabilitySurfaceTenantAPI {
			errs = append(errs, fmt.Errorf("allowlist route %s/%s requirement surface %s is not tenant_api: %s", route.Entrypoint, route.RouteClass, requirement.Surface, routeID))
		}
		if route.Entrypoint == "superadmin" && requirement.Surface != authz.CapabilitySurfaceSuperadminRoute {
			errs = append(errs, fmt.Errorf("allowlist route %s/%s requirement surface %s is not superadmin_route: %s", route.Entrypoint, route.RouteClass, requirement.Surface, routeID))
		}
		if requirement.Surface == authz.CapabilitySurfaceTenantAPI {
			tenantAPICovered[requirement.Key] = true
		}
		if requirement.Surface == authz.CapabilitySurfaceSuperadminRoute {
			superadminRouteCovered[requirement.Key] = true
		}
	}

	for _, route := range facts.Routes {
		routeID := routeCoverageID(route.Method, route.Path)
		if _, ok := allowlistRouteByID[routeID]; !ok {
			errs = append(errs, fmt.Errorf("route requirement has no allowlist route: %s", routeID))
			continue
		}
		if _, ok := requiredAllowlistRouteByID[routeID]; !ok {
			errs = append(errs, fmt.Errorf("route requirement is not covered by an authz-protected allowlist route: %s", routeID))
		}
	}

	for _, overlay := range facts.ToolOverlays {
		routeID := routeCoverageID(overlay.Method, overlay.Path)
		if _, ok := routeRequirementByID[routeID]; !ok {
			errs = append(errs, fmt.Errorf("tool overlay references route without authz requirement: %s", routeID))
		}
		if overlay.Surface != authz.CapabilitySurfaceTenantAPI {
			errs = append(errs, fmt.Errorf("tool overlay %s has unsupported surface %s", routeID, overlay.Surface))
		}
	}

	for _, seed := range facts.RoleSeeds {
		key := seed.AuthzCapabilityKey
		entry, ok := registryByKey[key]
		if !ok {
			errs = append(errs, fmt.Errorf("role seed %s/%s references unregistered authz capability key %s", seed.TenantID, seed.RoleSlug, key))
			continue
		}
		if entry.Surface == authz.CapabilitySurfaceTenantAPI && !tenantAPICovered[key] {
			errs = append(errs, fmt.Errorf("role seed %s/%s references tenant authz capability key without API coverage: %s", seed.TenantID, seed.RoleSlug, key))
		}
	}

	for _, grant := range facts.PolicyGrants {
		key := authz.AuthzCapabilityKey(grant.Object, grant.Action)
		entry, ok := registryByKey[key]
		if !ok {
			errs = append(errs, fmt.Errorf("policy grant %s/%s references unregistered authz capability key %s", grant.Subject, grant.Domain, key))
			continue
		}
		if isBuiltinTenantRoleSubject(grant.Subject) && entry.Status == authz.CapabilityStatusEnabled && entry.Assignable && entry.Surface == authz.CapabilitySurfaceTenantAPI {
			errs = append(errs, fmt.Errorf("policy grant %s/%s must not grant assignable tenant API capability %s; use DB role seed/runtime instead", grant.Subject, grant.Domain, key))
		}
		if entry.Surface == authz.CapabilitySurfaceTenantAPI && !tenantAPICovered[key] {
			errs = append(errs, fmt.Errorf("policy grant %s/%s references tenant authz capability key without API coverage: %s", grant.Subject, grant.Domain, key))
		}
		if entry.Surface == authz.CapabilitySurfaceSuperadminRoute {
			if !superadminRouteCovered[key] {
				errs = append(errs, fmt.Errorf("policy grant %s/%s references superadmin authz capability key without route coverage: %s", grant.Subject, grant.Domain, key))
			}
			if grant.Subject != authz.SubjectFromRoleSlug(authz.RoleSuperadmin) {
				errs = append(errs, fmt.Errorf("policy grant %s/%s references superadmin-only authz capability key %s", grant.Subject, grant.Domain, key))
			}
		}
	}

	for _, entry := range facts.Registry {
		if entry.Status == authz.CapabilityStatusEnabled && entry.Assignable && entry.Surface == authz.CapabilitySurfaceTenantAPI && !tenantAPICovered[entry.Key] {
			errs = append(errs, fmt.Errorf("assignable tenant authz capability key has no tenant API coverage: %s", entry.Key))
		}
	}

	return errs
}

func isBuiltinTenantRoleSubject(subject string) bool {
	switch strings.TrimSpace(subject) {
	case authz.SubjectFromRoleSlug(authz.RoleTenantAdmin), authz.SubjectFromRoleSlug(authz.RoleTenantViewer):
		return true
	default:
		return false
	}
}

func authzAllowlistRouteRequiresRequirement(route AuthzAllowlistRoute) bool {
	if route.Entrypoint == "superadmin" {
		switch route.Method + " " + route.Path {
		case "GET /superadmin/login",
			"POST /superadmin/login",
			"POST /superadmin/logout",
			"GET /superadmin/tenants",
			"POST /superadmin/tenants",
			"POST /superadmin/tenants/{tenant_id}/enable",
			"POST /superadmin/tenants/{tenant_id}/disable",
			"POST /superadmin/tenants/{tenant_id}/domains":
			return true
		default:
			return false
		}
	}
	if route.Entrypoint != "server" {
		return false
	}
	if route.RouteClass != string(routing.RouteClassInternalAPI) && route.RouteClass != string(routing.RouteClassPublicAPI) {
		return route.Method == http.MethodPost && route.Path == "/logout"
	}
	switch route.Method + " " + route.Path {
	case "GET /iam/api/me/capabilities",
		"GET /internal/cubebox/capabilities",
		"POST /internal/cubebox/conversations/{conversation_id}:compact":
		return false
	default:
		return true
	}
}
