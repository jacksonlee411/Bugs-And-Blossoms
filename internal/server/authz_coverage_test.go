package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

func TestLintAuthzCoverage_CurrentFactsPass(t *testing.T) {
	facts := AuthzCoverageFacts{
		AllowlistRoutes: ListAuthzAllowlistRoutes(testAuthzAllowlist()),
		Routes:          ListAuthzRouteCoverage(),
		Registry:        authz.ListAuthzCapabilities(),
		PolicyGrants:    []authz.PolicyGrant{{Subject: "role:tenant-admin", Domain: "*", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionRead}},
	}
	if errs := LintAuthzCoverage(facts); len(errs) != 0 {
		t.Fatalf("errs=%v", errs)
	}
}

func TestLintAuthzCoverage_Failures(t *testing.T) {
	t.Run("policy outside registry", func(t *testing.T) {
		facts := AuthzCoverageFacts{
			Routes:   ListAuthzRouteCoverage(),
			Registry: authz.ListAuthzCapabilities(),
			PolicyGrants: []authz.PolicyGrant{
				{Subject: "role:tenant-admin", Domain: "*", Object: "org." + "share_read", Action: authz.ActionRead},
			},
		}
		assertLintContains(t, facts, "unregistered authz capability key org."+"share_read:read")
	})

	t.Run("policy only tenant capability", func(t *testing.T) {
		facts := AuthzCoverageFacts{
			AllowlistRoutes: nil,
			Routes:          nil,
			Registry:        authz.ListAuthzCapabilities(),
			PolicyGrants: []authz.PolicyGrant{
				{Subject: "role:tenant-admin", Domain: "*", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionRead},
			},
		}
		assertLintContains(t, facts, "without API coverage")
	})

	t.Run("assignable without tenant api coverage", func(t *testing.T) {
		facts := AuthzCoverageFacts{
			AllowlistRoutes: nil,
			Routes:          nil,
			Registry:        authz.ListAuthzCapabilities(),
			PolicyGrants:    nil,
		}
		assertLintContains(t, facts, "assignable tenant authz capability key has no tenant API coverage")
	})

	t.Run("allowlist api without requirement", func(t *testing.T) {
		facts := AuthzCoverageFacts{
			AllowlistRoutes: []AuthzAllowlistRoute{
				{Entrypoint: "server", Method: "GET", Path: "/iam/api/uncovered", RouteClass: string(routing.RouteClassInternalAPI)},
			},
			Routes:       ListAuthzRouteCoverage(),
			Registry:     authz.ListAuthzCapabilities(),
			PolicyGrants: nil,
		}
		assertLintContains(t, facts, "has no authz requirement")
	})

	t.Run("route requirement without allowlist coverage", func(t *testing.T) {
		facts := AuthzCoverageFacts{
			AllowlistRoutes: []AuthzAllowlistRoute{
				{Entrypoint: "server", Method: "GET", Path: "/iam/api/dicts", RouteClass: string(routing.RouteClassInternalAPI)},
			},
			Routes: []AuthzRouteCoverage{
				{Method: "GET", Path: "/iam/api/dicts", Object: authz.ObjectIAMDicts, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI, Key: "iam.dicts:read"},
				{Method: "GET", Path: "/iam/api/fake", Object: authz.ObjectIAMAuthz, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI, Key: "iam.authz:read"},
			},
			Registry:     authz.ListAuthzCapabilities(),
			PolicyGrants: nil,
		}
		assertLintContains(t, facts, "route requirement has no allowlist route: GET /iam/api/fake")
	})

	t.Run("authn allowlist coverage accepted", func(t *testing.T) {
		facts := AuthzCoverageFacts{
			AllowlistRoutes: []AuthzAllowlistRoute{
				{Entrypoint: "server", Method: "POST", Path: "/logout", RouteClass: string(routing.RouteClassAuthn)},
			},
			Routes: []AuthzRouteCoverage{
				{Method: "POST", Path: "/logout", Object: authz.ObjectIAMSession, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI, Key: "iam.session:admin"},
			},
			Registry:     testRegistryEntries(t, "iam.session:admin"),
			PolicyGrants: nil,
		}
		if errs := LintAuthzCoverage(facts); len(errs) != 0 {
			t.Fatalf("errs=%v", errs)
		}
	})

	t.Run("superadmin policy without route coverage", func(t *testing.T) {
		facts := AuthzCoverageFacts{
			AllowlistRoutes: nil,
			Routes:          nil,
			Registry:        authz.ListAuthzCapabilities(),
			PolicyGrants: []authz.PolicyGrant{
				{Subject: authz.SubjectFromRoleSlug(authz.RoleSuperadmin), Domain: authz.DomainGlobal, Object: authz.ObjectSuperadminTenants, Action: authz.ActionRead},
			},
		}
		assertLintContains(t, facts, "without route coverage")
	})
}

func TestLintAuthzCoverage_AllowlistExemptions(t *testing.T) {
	facts := AuthzCoverageFacts{
		AllowlistRoutes: []AuthzAllowlistRoute{
			{Entrypoint: "server", Method: "GET", Path: "/iam/api/me/capabilities", RouteClass: string(routing.RouteClassInternalAPI)},
			{Entrypoint: "server", Method: "GET", Path: "/internal/cubebox/capabilities", RouteClass: string(routing.RouteClassInternalAPI)},
			{Entrypoint: "server", Method: "POST", Path: "/internal/cubebox/conversations/{conversation_id}:compact", RouteClass: string(routing.RouteClassInternalAPI)},
		},
		Routes:       nil,
		Registry:     nil,
		PolicyGrants: nil,
	}
	if errs := LintAuthzCoverage(facts); len(errs) != 0 {
		t.Fatalf("errs=%v", errs)
	}
}

func TestListAuthzAllowlistRoutes(t *testing.T) {
	routes := ListAuthzAllowlistRoutes(routing.Allowlist{
		Entrypoints: map[string]routing.Entrypoint{
			"server": {
				Routes: []routing.Route{
					{Path: "/iam/api/dicts", Methods: []string{"POST", "GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
				},
			},
		},
	})
	if len(routes) != 2 {
		t.Fatalf("routes=%v", routes)
	}
	if routes[0].Method != "GET" || routes[1].Method != "POST" {
		t.Fatalf("routes=%v", routes)
	}
}

func TestTenantAPICoveredCapabilityKeys(t *testing.T) {
	covered := TenantAPICoveredCapabilityKeysForAllowlist(testAuthzAllowlist())
	for _, key := range []string{
		"iam.authz:read",
		"orgunit.orgunits:read",
		"orgunit.orgunits:admin",
	} {
		if !covered[key] {
			t.Fatalf("missing covered key %s", key)
		}
	}
	if covered["superadmin.tenants:read"] {
		t.Fatal("superadmin capability must not be tenant API covered")
	}
}

func TestTenantAPICoveredCapabilityKeysForAllowlist_OnlyCountsAllowlistedRequirements(t *testing.T) {
	covered := tenantAPICoveredCapabilityKeys(
		[]AuthzAllowlistRoute{
			{Entrypoint: "server", Method: "GET", Path: "/iam/api/dicts", RouteClass: string(routing.RouteClassInternalAPI)},
		},
		[]AuthzRouteCoverage{
			{Method: "GET", Path: "/iam/api/dicts", Object: authz.ObjectIAMDicts, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI, Key: "iam.dicts:read"},
			{Method: "GET", Path: "/iam/api/fake", Object: authz.ObjectIAMAuthz, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI, Key: "iam.authz:read"},
		},
	)
	if !covered["iam.dicts:read"] {
		t.Fatal("expected allowlisted route to count as covered")
	}
	if covered["iam.authz:read"] {
		t.Fatal("route requirement without allowlist route must not count as covered")
	}
}

func TestAuthzCoverageAllowlistPath_UsesEnvironmentOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowlist.yaml")
	t.Setenv("ALLOWLIST_PATH", path)

	got, err := authzCoverageAllowlistPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("path=%q want %q", got, path)
	}
}

func TestAuthzCoverageAllowlistPath_DefaultExists(t *testing.T) {
	if os.Getenv("ALLOWLIST_PATH") != "" {
		t.Skip("environment override is process-global")
	}
	got, err := authzCoverageAllowlistPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, filepath.Join("config", "routing", "allowlist.yaml")) {
		t.Fatalf("path=%q", got)
	}
}

func testAuthzAllowlist() routing.Allowlist {
	return routing.Allowlist{
		Version: 1,
		Entrypoints: map[string]routing.Entrypoint{
			"server": {
				Routes: []routing.Route{
					{Path: "/iam/api/sessions", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/iam/api/authz/capabilities", Methods: []string{"GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/iam/api/dicts", Methods: []string{"GET", "POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/iam/api/dicts:disable", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/iam/api/dicts/values", Methods: []string{"GET", "POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/iam/api/dicts/values:disable", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/iam/api/dicts/values:correct", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/iam/api/dicts/values/audit", Methods: []string{"GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/iam/api/dicts:release", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/iam/api/dicts:release:preview", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/logout", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassAuthn)},
					{Path: "/org/api/org-units", Methods: []string{"GET", "POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/field-definitions", Methods: []string{"GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/field-configs", Methods: []string{"GET", "POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/field-configs:enable-candidates", Methods: []string{"GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/field-configs:disable", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/fields:options", Methods: []string{"GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/details", Methods: []string{"GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/versions", Methods: []string{"GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/audit", Methods: []string{"GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/search", Methods: []string{"GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/rename", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/move", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/disable", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/enable", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/write", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/corrections", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/status-corrections", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/rescinds", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/rescinds/org", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/org/api/org-units/set-business-unit", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/conversations", Methods: []string{"GET", "POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/conversations/{conversation_id}", Methods: []string{"GET", "PATCH"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/conversations/{conversation_id}:compact", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/turns:stream", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/turns/{turn_id}:interrupt", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/capabilities", Methods: []string{"GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/settings", Methods: []string{"GET"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/settings/providers", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/settings/credentials", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/settings/credentials/{credential_id}:deactivate", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/settings/selection", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
					{Path: "/internal/cubebox/settings/verify", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassInternalAPI)},
				},
			},
			"superadmin": {
				Routes: []routing.Route{
					{Path: "/superadmin/login", Methods: []string{"GET", "POST"}, RouteClass: string(routing.RouteClassUI)},
					{Path: "/superadmin/logout", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassUI)},
					{Path: "/superadmin/tenants", Methods: []string{"GET", "POST"}, RouteClass: string(routing.RouteClassUI)},
					{Path: "/superadmin/tenants/{tenant_id}/enable", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassUI)},
					{Path: "/superadmin/tenants/{tenant_id}/disable", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassUI)},
					{Path: "/superadmin/tenants/{tenant_id}/domains", Methods: []string{"POST"}, RouteClass: string(routing.RouteClassUI)},
				},
			},
		},
	}
}

func testRegistryEntries(t *testing.T, keys ...string) []authz.AuthzCapability {
	t.Helper()
	out := make([]authz.AuthzCapability, 0, len(keys))
	for _, key := range keys {
		entry, ok := authz.LookupAuthzCapability(key)
		if !ok {
			t.Fatalf("missing test registry key %s", key)
		}
		out = append(out, entry)
	}
	return out
}

func assertLintContains(t *testing.T, facts AuthzCoverageFacts, want string) {
	t.Helper()
	errs := LintAuthzCoverage(facts)
	for _, err := range errs {
		if strings.Contains(err.Error(), want) {
			return
		}
	}
	t.Fatalf("expected lint error containing %q, got %v", want, errs)
}
