package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

func loadAuthorizer() (*authz.Authorizer, error) {
	modelPath := os.Getenv("AUTHZ_MODEL_PATH")
	if modelPath == "" {
		p, err := defaultAuthzModelPath()
		if err != nil {
			return nil, err
		}
		modelPath = p
	}

	policyPath, err := authzPolicyPath()
	if err != nil {
		return nil, err
	}

	mode, err := authz.ModeFromEnv()
	if err != nil {
		return nil, err
	}

	return authz.NewAuthorizer(modelPath, policyPath, mode)
}

func defaultAuthzModelPath() (string, error) {
	path := "config/access/model.conf"
	for range 8 {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		path = filepath.Join("..", path)
	}
	return "", errors.New("server: authz model not found")
}

func defaultAuthzPolicyPath() (string, error) {
	path := "config/access/policy.csv"
	for range 8 {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		path = filepath.Join("..", path)
	}
	return "", errors.New("server: authz policy not found")
}

func authzPolicyPath() (string, error) {
	if path := os.Getenv("AUTHZ_POLICY_PATH"); strings.TrimSpace(path) != "" {
		return path, nil
	}
	return defaultAuthzPolicyPath()
}

type authorizer interface {
	Authorize(subject string, domain string, object string, action string) (allowed bool, enforced bool, err error)
	Mode() authz.Mode
}

type routeRequirement struct {
	Method  string
	Path    string
	Object  string
	Action  string
	Surface string
}

var exactRouteRequirements = []routeRequirement{
	{Method: http.MethodPost, Path: "/iam/api/sessions", Object: authz.ObjectIAMSession, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/iam/api/authz/capabilities", Object: authz.ObjectIAMAuthz, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/iam/api/authz/api-catalog", Object: authz.ObjectIAMAuthz, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/iam/api/authz/roles", Object: authz.ObjectIAMAuthz, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/iam/api/authz/roles", Object: authz.ObjectIAMAuthz, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/iam/api/authz/user-assignments", Object: authz.ObjectIAMAuthz, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/iam/api/dicts", Object: authz.ObjectIAMDicts, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/iam/api/dicts", Object: authz.ObjectIAMDicts, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/iam/api/dicts:disable", Object: authz.ObjectIAMDicts, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/iam/api/dicts/values", Object: authz.ObjectIAMDicts, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/iam/api/dicts/values", Object: authz.ObjectIAMDicts, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/iam/api/dicts/values:disable", Object: authz.ObjectIAMDicts, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/iam/api/dicts/values:correct", Object: authz.ObjectIAMDicts, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/iam/api/dicts/values/audit", Object: authz.ObjectIAMDicts, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/iam/api/dicts:release", Object: authz.ObjectIAMDictRelease, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/iam/api/dicts:release:preview", Object: authz.ObjectIAMDictRelease, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/logout", Object: authz.ObjectIAMSession, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/org/api/org-units", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/org/api/org-units/field-definitions", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/org/api/org-units/field-configs", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/field-configs", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/org/api/org-units/field-configs:enable-candidates", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/field-configs:disable", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/org/api/org-units/fields:options", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/org/api/org-units/details", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/org/api/org-units/versions", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/org/api/org-units/audit", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/org/api/org-units/search", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/rename", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/move", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/disable", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/enable", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/write", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/corrections", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/status-corrections", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/rescinds", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/rescinds/org", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/org/api/org-units/set-business-unit", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/internal/cubebox/conversations", Object: authz.ObjectCubeBoxConversations, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/internal/cubebox/conversations", Object: authz.ObjectCubeBoxConversations, Action: authz.ActionUse, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/internal/cubebox/turns:stream", Object: authz.ObjectCubeBoxConversations, Action: authz.ActionUse, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/internal/cubebox/settings", Object: authz.ObjectCubeBoxModelCredential, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/internal/cubebox/settings/providers", Object: authz.ObjectCubeBoxModelProvider, Action: authz.ActionUpdate, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/internal/cubebox/settings/credentials", Object: authz.ObjectCubeBoxModelCredential, Action: authz.ActionRotate, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/internal/cubebox/settings/selection", Object: authz.ObjectCubeBoxModelSelection, Action: authz.ActionSelect, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/internal/cubebox/settings/verify", Object: authz.ObjectCubeBoxModelSelection, Action: authz.ActionVerify, Surface: authz.CapabilitySurfaceTenantAPI},
}

var patternRouteRequirements = []routeRequirement{
	{Method: http.MethodGet, Path: "/iam/api/authz/roles/{role_slug}", Object: authz.ObjectIAMAuthz, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPut, Path: "/iam/api/authz/roles/{role_slug}", Object: authz.ObjectIAMAuthz, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPut, Path: "/iam/api/authz/user-assignments/{principal_id}", Object: authz.ObjectIAMAuthz, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodGet, Path: "/internal/cubebox/conversations/{conversation_id}", Object: authz.ObjectCubeBoxConversations, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPatch, Path: "/internal/cubebox/conversations/{conversation_id}", Object: authz.ObjectCubeBoxConversations, Action: authz.ActionUse, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/internal/cubebox/turns/{turn_id}:interrupt", Object: authz.ObjectCubeBoxConversations, Action: authz.ActionUse, Surface: authz.CapabilitySurfaceTenantAPI},
	{Method: http.MethodPost, Path: "/internal/cubebox/settings/credentials/{credential_id}:deactivate", Object: authz.ObjectCubeBoxModelCredential, Action: authz.ActionDeactivate, Surface: authz.CapabilitySurfaceTenantAPI},
}

func withAuthz(classifier *routing.Classifier, a authorizer, runtime authzRuntimeStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		rc := routing.RouteClassUI
		if classifier != nil {
			rc = classifier.Classify(path)
		}

		switch path {
		case "/health", "/healthz":
			next.ServeHTTP(w, r)
			return
		default:
			if pathHasPrefixSegment(path, "/assets") || path == "/" || pathHasPrefixSegment(path, "/app") {
				next.ServeHTTP(w, r)
				return
			}
		}

		tenant, ok := currentTenant(r.Context())
		if !ok {
			routing.WriteError(w, r, rc, http.StatusInternalServerError, "tenant_missing", "tenant missing")
			return
		}

		object, action, shouldCheck := authzRequirementForRoute(r.Method, path)
		if !shouldCheck {
			next.ServeHTTP(w, r)
			return
		}

		if isBootstrapPolicyRoute(r.Method, path) {
			allowed, enforced, err := a.Authorize(authz.SubjectFromRoleSlug(authz.RoleAnonymous), authz.DomainFromTenantID(tenant.ID), object, action)
			if err != nil {
				routing.WriteError(w, r, rc, http.StatusInternalServerError, "authz_error", "authz error")
				return
			}
			if enforced && !allowed {
				routing.WriteError(w, r, rc, http.StatusForbidden, "forbidden", "forbidden")
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		principal, ok := currentPrincipal(r.Context())
		if !ok || strings.TrimSpace(principal.ID) == "" {
			routing.WriteError(w, r, rc, http.StatusUnauthorized, "principal_missing", "principal missing")
			return
		}
		if a.Mode() == authz.ModeDisabled {
			next.ServeHTTP(w, r)
			return
		}
		if runtime == nil {
			routing.WriteError(w, r, rc, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
			return
		}
		allowed, err := runtime.AuthorizePrincipal(r.Context(), tenant.ID, principal.ID, object, action)
		if err != nil {
			routing.WriteError(w, r, rc, http.StatusInternalServerError, "authz_error", "authz error")
			return
		}
		if a.Mode() == authz.ModeEnforce && !allowed {
			routing.WriteError(w, r, rc, http.StatusForbidden, "forbidden", "forbidden")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func listRouteRequirements() []routeRequirement {
	out := make([]routeRequirement, 0, len(exactRouteRequirements)+len(patternRouteRequirements))
	out = append(out, exactRouteRequirements...)
	out = append(out, patternRouteRequirements...)
	return out
}

func authzRequirementForRoute(method string, path string) (object string, action string, ok bool) {
	if req, ok := findRouteRequirement(method, path); ok {
		return req.Object, req.Action, true
	}
	return "", "", false
}

func findRouteRequirement(method string, path string) (routeRequirement, bool) {
	for _, req := range exactRouteRequirements {
		if req.Method == method && req.Path == path {
			return req, true
		}
	}
	for _, req := range patternRouteRequirements {
		if req.Method == method && pathMatchRouteTemplate(path, req.Path) {
			return req, true
		}
	}
	return routeRequirement{}, false
}

func isBootstrapPolicyRoute(method string, path string) bool {
	return (method == http.MethodPost && path == "/iam/api/sessions") ||
		(method == http.MethodPost && path == "/logout")
}

func pathMatchRouteTemplate(path string, template string) bool {
	in := splitRouteSegments(path)
	want := splitRouteSegments(template)
	if len(in) != len(want) {
		return false
	}
	for i := range want {
		w := want[i]
		g := in[i]
		if g == "" {
			return false
		}
		if isRouteParamSegment(w) {
			if strings.ContainsRune(g, ':') {
				return false
			}
			continue
		}
		if prefix, suffix, ok := splitRouteTemplateSegment(w); ok {
			if len(g) <= len(prefix)+len(suffix) {
				return false
			}
			if !strings.HasPrefix(g, prefix) || !strings.HasSuffix(g, suffix) {
				return false
			}
			continue
		}
		if g != w {
			return false
		}
	}
	return true
}

func splitRouteSegments(path string) []string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func isRouteParamSegment(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && len(s) > 2
}

func routeTemplateIsParamSegment(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && len(s) > 2
}

func splitRouteTemplateSegment(s string) (prefix string, suffix string, ok bool) {
	open := strings.IndexByte(s, '{')
	close := strings.IndexByte(s, '}')
	if open < 0 || close < 0 || close <= open {
		return "", "", false
	}
	if strings.Contains(s[close+1:], "{") || strings.Contains(s[:open], "}") {
		return "", "", false
	}
	name := strings.TrimSpace(s[open+1 : close])
	if name == "" {
		return "", "", false
	}
	if strings.ContainsRune(name, '{') || strings.ContainsRune(name, '}') {
		return "", "", false
	}
	if strings.Contains(s[close+1:], "}") {
		return "", "", false
	}
	return s[:open], s[close+1:], true
}
