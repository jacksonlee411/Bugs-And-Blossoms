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

	policyPath := os.Getenv("AUTHZ_POLICY_PATH")
	if policyPath == "" {
		p, err := defaultAuthzPolicyPath()
		if err != nil {
			return nil, err
		}
		policyPath = p
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

type authorizer interface {
	Authorize(subject string, domain string, object string, action string) (allowed bool, enforced bool, err error)
}

func withAuthz(classifier *routing.Classifier, a authorizer, next http.Handler) http.Handler {
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

		roleSlug := authz.RoleAnonymous
		if p, ok := currentPrincipal(r.Context()); ok {
			roleSlug = p.RoleSlug
		}

		subject := authz.SubjectFromRoleSlug(roleSlug)
		domain := authz.DomainFromTenantID(tenant.ID)

		object, action, shouldCheck := authzRequirementForRoute(r.Method, path)
		if !shouldCheck {
			next.ServeHTTP(w, r)
			return
		}

		allowed, enforced, err := a.Authorize(subject, domain, object, action)
		if err != nil {
			routing.WriteError(w, r, rc, http.StatusInternalServerError, "authz_error", "authz error")
			return
		}
		if enforced && !allowed {
			code := "forbidden"
			if object == authz.ObjectOrgShareRead {
				code = "share_read_forbidden"
			}
			routing.WriteError(w, r, rc, http.StatusForbidden, code, code)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func authzRequirementForRoute(method string, path string) (object string, action string, ok bool) {
	switch path {
	case "/iam/api/sessions":
		if method == http.MethodPost {
			return authz.ObjectIAMSession, authz.ActionAdmin, true
		}
		return "", "", false
	case "/iam/api/me/capabilities":
		return "", "", false
	case "/iam/api/dicts":
		if method == http.MethodGet {
			return authz.ObjectIAMDicts, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectIAMDicts, authz.ActionAdmin, true
		}
		return "", "", false
	case "/iam/api/dicts:disable":
		if method == http.MethodPost {
			return authz.ObjectIAMDicts, authz.ActionAdmin, true
		}
		return "", "", false
	case "/iam/api/dicts/values":
		if method == http.MethodGet {
			return authz.ObjectIAMDicts, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectIAMDicts, authz.ActionAdmin, true
		}
		return "", "", false
	case "/iam/api/dicts/values:disable", "/iam/api/dicts/values:correct":
		if method == http.MethodPost {
			return authz.ObjectIAMDicts, authz.ActionAdmin, true
		}
		return "", "", false
	case "/iam/api/dicts/values/audit":
		if method == http.MethodGet {
			return authz.ObjectIAMDicts, authz.ActionRead, true
		}
		return "", "", false
	case "/iam/api/dicts:release", "/iam/api/dicts:release:preview":
		if method == http.MethodPost {
			return authz.ObjectIAMDictRelease, authz.ActionAdmin, true
		}
		return "", "", false
	case "/internal/cubebox/conversations":
		if method == http.MethodPost {
			return authz.ObjectCubeBoxConversations, authz.ActionUse, true
		}
		if method == http.MethodGet {
			return authz.ObjectCubeBoxConversations, authz.ActionRead, true
		}
		return "", "", false
	case "/internal/cubebox/turns:stream":
		if method == http.MethodPost {
			return authz.ObjectCubeBoxConversations, authz.ActionUse, true
		}
		return "", "", false
	case "/internal/cubebox/capabilities":
		if method == http.MethodGet {
			return "", "", false
		}
		return "", "", false
	case "/internal/cubebox/settings":
		if method == http.MethodGet {
			return authz.ObjectCubeBoxModelCredential, authz.ActionRead, true
		}
		return "", "", false
	case "/internal/cubebox/settings/providers":
		if method == http.MethodPost {
			return authz.ObjectCubeBoxModelProvider, authz.ActionUpdate, true
		}
		return "", "", false
	case "/internal/cubebox/settings/credentials":
		if method == http.MethodPost {
			return authz.ObjectCubeBoxModelCredential, authz.ActionRotate, true
		}
		return "", "", false
	case "/internal/cubebox/settings/selection":
		if method == http.MethodPost {
			return authz.ObjectCubeBoxModelSelection, authz.ActionSelect, true
		}
		return "", "", false
	case "/internal/cubebox/settings/verify":
		if method == http.MethodPost {
			return authz.ObjectCubeBoxModelSelection, authz.ActionVerify, true
		}
		return "", "", false
	case "/logout":
		if method == http.MethodPost {
			return authz.ObjectIAMSession, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/org-units":
		if method == http.MethodGet {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/org-units/field-definitions":
		if method == http.MethodGet {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/org-units/field-configs":
		if method == http.MethodGet || method == http.MethodPost {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/org-units/field-configs:enable-candidates":
		if method == http.MethodGet {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/org-units/field-configs:disable":
		if method == http.MethodPost {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/org-units/fields:options":
		if method == http.MethodGet {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionRead, true
		}
		return "", "", false
	case "/org/api/org-units/details", "/org/api/org-units/versions", "/org/api/org-units/audit", "/org/api/org-units/search":
		if method == http.MethodGet {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionRead, true
		}
		return "", "", false
	case "/org/api/org-units/rename", "/org/api/org-units/move", "/org/api/org-units/disable", "/org/api/org-units/enable", "/org/api/org-units/write", "/org/api/org-units/corrections", "/org/api/org-units/status-corrections", "/org/api/org-units/rescinds", "/org/api/org-units/rescinds/org":
		if method == http.MethodPost {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/org-units/set-business-unit":
		if method == http.MethodPost {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionAdmin, true
		}
		return "", "", false
	default:
		if pathMatchRouteTemplate(path, "/internal/cubebox/conversations/{conversation_id}") && method == http.MethodGet {
			return authz.ObjectCubeBoxConversations, authz.ActionRead, true
		}
		if pathMatchRouteTemplate(path, "/internal/cubebox/conversations/{conversation_id}") && method == http.MethodPatch {
			return authz.ObjectCubeBoxConversations, authz.ActionUse, true
		}
		if pathMatchRouteTemplate(path, "/internal/cubebox/turns/{turn_id}:interrupt") && method == http.MethodPost {
			return authz.ObjectCubeBoxConversations, authz.ActionUse, true
		}
		if pathMatchRouteTemplate(path, "/internal/cubebox/settings/credentials/{credential_id}:deactivate") && method == http.MethodPost {
			return authz.ObjectCubeBoxModelCredential, authz.ActionDeactivate, true
		}
		return "", "", false
	}
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
