package authz_test

import (
	"strings"
	"testing"

	authz "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

func TestParseAuthzCapabilityKey_BlackBox(t *testing.T) {
	object, action, err := authz.ParseAuthzCapabilityKey(" orgunit.orgunits:read ")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if object != authz.ObjectOrgUnitOrgUnits || action != authz.ActionRead {
		t.Fatalf("object=%q action=%q", object, action)
	}

	invalid := []string{
		"orgunit" + ".read",
		"dict" + ".admin",
		"orgunit.orgunits.read",
		"orgunit.orgunits:",
		":read",
		"orgunit.orgunits:read:extra",
	}
	for _, key := range invalid {
		key := key
		t.Run(key, func(t *testing.T) {
			if _, _, err := authz.ParseAuthzCapabilityKey(key); err == nil {
				t.Fatal("expected invalid key error")
			}
		})
	}
}

func TestRegistryLookupAndOptions_BlackBox(t *testing.T) {
	if err := authz.ValidateRegistry(); err != nil {
		t.Fatalf("ValidateRegistry() error = %v", err)
	}
	entry, ok := authz.LookupAuthzCapability("orgunit.orgunits:read")
	if !ok {
		t.Fatal("missing orgunit read capability")
	}
	if !entry.Assignable || entry.ScopeDimension != authz.ScopeDimensionOrganization {
		t.Fatalf("entry=%+v", entry)
	}

	covered := map[string]bool{"orgunit.orgunits:read": true}
	options := authz.ListAuthzCapabilityOptions(authz.CapabilityListFilter{
		RequireAssignable: true,
		RequireTenantAPI:  true,
		CoveredKeys:       covered,
	})
	if len(options) != 1 || options[0].Key != "orgunit.orgunits:read" || !options[0].Covered {
		t.Fatalf("options=%+v", options)
	}

	diagnostic := authz.ListAuthzCapabilityOptions(authz.CapabilityListFilter{
		RequireAssignable: true,
		RequireTenantAPI:  true,
		IncludeUncovered:  true,
		Query:             "cubebox",
		CoveredKeys:       covered,
	})
	if len(diagnostic) == 0 {
		t.Fatal("expected diagnostic query to include uncovered cubebox capabilities")
	}
	for _, option := range diagnostic {
		if !strings.Contains(option.Key, "cubebox.") {
			t.Fatalf("unexpected option=%+v", option)
		}
	}
}

func TestValidateAssignableTenantCapabilityKeys_BlackBox(t *testing.T) {
	covered := map[string]bool{"orgunit.orgunits:read": true}
	if err := authz.ValidateAssignableTenantCapabilityKeys([]string{"orgunit.orgunits:read"}, covered); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cases := map[string][]string{
		"old key":        {"orgunit" + ".read"},
		"unknown":        {"unknown.object:read"},
		"duplicate":      {"orgunit.orgunits:read", "orgunit.orgunits:read"},
		"superadmin":     {"superadmin.tenants:read"},
		"not assignable": {"iam.session:admin"},
		"uncovered":      {"orgunit.orgunits:admin"},
	}
	for name, keys := range cases {
		keys := keys
		t.Run(name, func(t *testing.T) {
			if err := authz.ValidateAssignableTenantCapabilityKeys(keys, covered); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
