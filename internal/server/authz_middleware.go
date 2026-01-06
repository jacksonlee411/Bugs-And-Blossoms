package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

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

func withAuthz(a authorizer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch path {
		case "/health", "/healthz", "/login", "/logout":
			next.ServeHTTP(w, r)
			return
		default:
			if pathHasPrefixSegment(path, "/assets") || pathHasPrefixSegment(path, "/lang") || pathHasPrefixSegment(path, "/ui") || path == "/" || pathHasPrefixSegment(path, "/app") {
				next.ServeHTTP(w, r)
				return
			}
		}

		tenant, ok := currentTenant(r.Context())
		if !ok {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
			return
		}

		roleSlug := authz.RoleAnonymous
		if hasSession(r) {
			roleSlug = authz.RoleTenantAdmin
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
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "authz_error", "authz error")
			return
		}
		if enforced && !allowed {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusForbidden, "forbidden", "forbidden")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func authzRequirementForRoute(method string, path string) (object string, action string, ok bool) {
	switch path {
	case "/org/nodes":
		if method == http.MethodGet {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectOrgUnitOrgUnits, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/snapshot":
		return authz.ObjectOrgUnitOrgUnits, authz.ActionRead, true
	case "/org/setid":
		if method == http.MethodGet {
			return authz.ObjectOrgUnitSetID, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectOrgUnitSetID, authz.ActionAdmin, true
		}
		return "", "", false
	case "/org/job-catalog":
		if method == http.MethodGet {
			return authz.ObjectJobCatalogCatalog, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectJobCatalogCatalog, authz.ActionAdmin, true
		}
		return "", "", false
	default:
		return "", "", false
	}
}
