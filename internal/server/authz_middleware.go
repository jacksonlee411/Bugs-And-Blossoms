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
	if pathMatchRouteTemplate(path, "/org/api/scope-packages/{package_id}/disable") {
		if method == http.MethodPost {
			return authz.ObjectOrgScopePackage, authz.ActionAdmin, true
		}
		return "", "", false
	}

	switch path {
	case "/iam/api/sessions":
		if method == http.MethodPost {
			return authz.ObjectIAMSession, authz.ActionAdmin, true
		}
		return "", "", false
	case "/logout":
		if method == http.MethodPost {
			return authz.ObjectIAMSession, authz.ActionAdmin, true
		}
		return "", "", false
	case "/jobcatalog/api/catalog":
		if method == http.MethodGet {
			return authz.ObjectJobCatalogCatalog, authz.ActionRead, true
		}
		return "", "", false
	case "/jobcatalog/api/catalog/actions":
		if method == http.MethodPost {
			return authz.ObjectJobCatalogCatalog, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/positions":
		if method == http.MethodGet {
			return authz.ObjectStaffingPositions, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectStaffingPositions, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/positions:options":
		if method == http.MethodGet {
			return authz.ObjectStaffingPositions, authz.ActionRead, true
		}
		return "", "", false
	case "/org/api/assignments":
		if method == http.MethodGet {
			return authz.ObjectStaffingAssignments, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectStaffingAssignments, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/assignment-events:correct":
		if method == http.MethodPost {
			return authz.ObjectStaffingAssignments, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/assignment-events:rescind":
		if method == http.MethodPost {
			return authz.ObjectStaffingAssignments, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/setids":
		if method == http.MethodGet {
			return authz.ObjectOrgUnitSetID, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectOrgUnitSetID, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/setid-bindings":
		if method == http.MethodGet {
			return authz.ObjectOrgUnitSetID, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectOrgUnitSetID, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/scope-packages":
		if method == http.MethodGet {
			return authz.ObjectOrgScopePackage, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectOrgScopePackage, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/owned-scope-packages":
		if method == http.MethodGet {
			return authz.ObjectJobCatalogCatalog, authz.ActionRead, true
		}
		return "", "", false
	case "/org/api/scope-subscriptions":
		if method == http.MethodGet {
			return authz.ObjectOrgScopeSubscription, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectOrgScopeSubscription, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/global-setids":
		if method == http.MethodGet {
			return authz.ObjectOrgShareRead, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectOrgUnitSetID, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/global-scope-packages":
		if method == http.MethodGet {
			return authz.ObjectOrgScopePackage, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectOrgScopePackage, authz.ActionAdmin, true
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
	case "/org/api/org-units/field-configs:disable":
		if method == http.MethodPost {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/org-units/fields:options", "/org/api/org-units/mutation-capabilities":
		if method == http.MethodGet {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionRead, true
		}
		return "", "", false
	case "/org/api/org-units/details", "/org/api/org-units/versions", "/org/api/org-units/audit", "/org/api/org-units/search":
		if method == http.MethodGet {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionRead, true
		}
		return "", "", false
	case "/org/api/org-units/rename", "/org/api/org-units/move", "/org/api/org-units/disable", "/org/api/org-units/enable", "/org/api/org-units/corrections", "/org/api/org-units/status-corrections", "/org/api/org-units/rescinds", "/org/api/org-units/rescinds/org":
		if method == http.MethodPost {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/api/org-units/set-business-unit":
		if method == http.MethodPost {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionAdmin, true
		}
		return "", "", false
	case "/person/api/persons":
		if method == http.MethodGet {
			return authz.ObjectPersonPersons, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectPersonPersons, authz.ActionAdmin, true
		}
		return "", "", false
	case "/person/api/persons:options", "/person/api/persons:by-pernr":
		if method == http.MethodGet {
			return authz.ObjectPersonPersons, authz.ActionRead, true
		}
		return "", "", false
	default:
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
		if routeTemplateIsParamSegment(w) {
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

func routeTemplateIsParamSegment(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && len(s) > 2
}
