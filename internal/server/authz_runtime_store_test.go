package server

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

func TestMemoryAuthzRuntimeStore_PrincipalAssignmentRevision(t *testing.T) {
	store := newMemoryAuthzRuntimeStore()
	ctx := context.Background()
	rootOrgNodeKey, err := encodeOrgNodeKeyFromID(10000000)
	if err != nil {
		t.Fatalf("encode err=%v", err)
	}

	first, err := store.ReplacePrincipalAssignment(ctx, "tenant-a", "principal-a", replacePrincipalAssignmentInput{
		Roles:    []string{authz.RoleTenantViewer},
		Revision: 1,
		OrgScopes: []principalOrgScope{{
			OrgNodeKey:         rootOrgNodeKey,
			IncludeDescendants: true,
		}},
	})
	if err != nil {
		t.Fatalf("first replace err=%v", err)
	}
	if first.Revision != 2 {
		t.Fatalf("first revision=%d", first.Revision)
	}

	if _, err := store.ReplacePrincipalAssignment(ctx, "tenant-a", "principal-a", replacePrincipalAssignmentInput{
		Roles:    []string{authz.RoleTenantAdmin},
		Revision: 1,
	}); !errors.Is(err, errStaleRevision) {
		t.Fatalf("expected stale revision, got %v", err)
	}

	second, err := store.ReplacePrincipalAssignment(ctx, "tenant-a", "principal-a", replacePrincipalAssignmentInput{
		Roles:    []string{authz.RoleTenantAdmin},
		Revision: first.Revision,
		OrgScopes: []principalOrgScope{{
			OrgNodeKey:         rootOrgNodeKey,
			IncludeDescendants: true,
		}},
	})
	if err != nil {
		t.Fatalf("second replace err=%v", err)
	}
	if second.Revision != 3 {
		t.Fatalf("second revision=%d", second.Revision)
	}
}

func TestMemoryAuthzRuntimeStore_EnsurePrincipalRoleAssignmentSyncsBuiltinOnly(t *testing.T) {
	store := newMemoryAuthzRuntimeStore()
	ctx := context.Background()

	customRole, err := store.CreateRoleDefinition(ctx, "tenant-a", saveAuthzRoleDefinitionInput{
		RoleSlug:            "flower-hr",
		Name:                "Flower HR",
		AuthzCapabilityKeys: []string{"iam.dicts:read"},
	})
	if err != nil {
		t.Fatalf("create role err=%v", err)
	}
	narrowOrgNodeKey, err := encodeOrgNodeKeyFromID(10000001)
	if err != nil {
		t.Fatalf("encode narrow err=%v", err)
	}
	assignment, err := store.ReplacePrincipalAssignment(ctx, "tenant-a", "principal-a", replacePrincipalAssignmentInput{
		Roles:    []string{authz.RoleTenantAdmin, customRole.RoleSlug},
		Revision: 1,
		OrgScopes: []principalOrgScope{{
			OrgNodeKey:         narrowOrgNodeKey,
			IncludeDescendants: false,
		}},
	})
	if err != nil {
		t.Fatalf("replace err=%v", err)
	}
	if assignment.Revision != 2 {
		t.Fatalf("revision after replace=%d", assignment.Revision)
	}

	if err := store.EnsurePrincipalRoleAssignment(ctx, "tenant-a", "principal-a", authz.RoleTenantViewer); err != nil {
		t.Fatalf("ensure err=%v", err)
	}
	got, ok, err := store.GetPrincipalAssignment(ctx, "tenant-a", "principal-a")
	if err != nil {
		t.Fatalf("get err=%v", err)
	}
	if !ok {
		t.Fatal("expected assignment")
	}
	if got.Revision != assignment.Revision {
		t.Fatalf("login seed must not bump revision: got=%d want=%d", got.Revision, assignment.Revision)
	}
	var slugs []string
	for _, role := range got.Roles {
		slugs = append(slugs, role.RoleSlug)
	}
	want := []string{customRole.RoleSlug, authz.RoleTenantAdmin}
	if !reflect.DeepEqual(slugs, want) {
		t.Fatalf("roles=%v want=%v", slugs, want)
	}
	wantScopes := []principalOrgScope{{
		OrgNodeKey:         narrowOrgNodeKey,
		IncludeDescendants: false,
	}}
	if !reflect.DeepEqual(got.OrgScopes, wantScopes) {
		t.Fatalf("scopes=%v want=%v", got.OrgScopes, wantScopes)
	}
}

func TestMemoryAuthzRuntimeStore_EnsurePrincipalRoleAssignmentSeedsEmptyAssignment(t *testing.T) {
	store := newMemoryAuthzRuntimeStore()
	ctx := context.Background()
	rootOrgNodeKey, err := encodeOrgNodeKeyFromID(10000000)
	if err != nil {
		t.Fatalf("encode err=%v", err)
	}

	if err := store.EnsurePrincipalRoleAssignment(ctx, "tenant-a", "principal-a", authz.RoleTenantViewer); err != nil {
		t.Fatalf("ensure err=%v", err)
	}
	got, ok, err := store.GetPrincipalAssignment(ctx, "tenant-a", "principal-a")
	if err != nil {
		t.Fatalf("get err=%v", err)
	}
	if !ok {
		t.Fatal("expected assignment")
	}
	var slugs []string
	for _, role := range got.Roles {
		slugs = append(slugs, role.RoleSlug)
	}
	if want := []string{authz.RoleTenantViewer}; !reflect.DeepEqual(slugs, want) {
		t.Fatalf("roles=%v want=%v", slugs, want)
	}
	wantScopes := []principalOrgScope{{
		OrgNodeKey:         rootOrgNodeKey,
		IncludeDescendants: true,
	}}
	if !reflect.DeepEqual(got.OrgScopes, wantScopes) {
		t.Fatalf("scopes=%v want=%v", got.OrgScopes, wantScopes)
	}
}

func TestMemoryAuthzRuntimeStore_OrgScopesSeedsRootForInitialAssignmentOnly(t *testing.T) {
	store := newMemoryAuthzRuntimeStore()
	ctx := context.Background()
	rootOrgNodeKey, err := encodeOrgNodeKeyFromID(10000000)
	if err != nil {
		t.Fatalf("encode root err=%v", err)
	}

	if err := store.EnsurePrincipalRoleAssignment(ctx, "tenant-a", "principal-a", authz.RoleTenantViewer); err != nil {
		t.Fatalf("ensure err=%v", err)
	}
	store.mu.Lock()
	store.orgScopes[memoryRuntimeKey("tenant-a", "principal-a")] = nil
	store.mu.Unlock()

	scopes, err := store.OrgScopesForPrincipal(ctx, "tenant-a", "principal-a", "orgunit.orgunits:read")
	if err != nil {
		t.Fatalf("initial scopes err=%v", err)
	}
	wantScopes := []principalOrgScope{{
		OrgNodeKey:         rootOrgNodeKey,
		IncludeDescendants: true,
	}}
	if !reflect.DeepEqual(scopes, wantScopes) {
		t.Fatalf("initial scopes=%v want=%v", scopes, wantScopes)
	}

	assignment, ok, err := store.GetPrincipalAssignment(ctx, "tenant-a", "principal-a")
	if err != nil {
		t.Fatalf("get err=%v", err)
	}
	if !ok {
		t.Fatal("expected assignment")
	}
	customRole, err := store.CreateRoleDefinition(ctx, "tenant-a", saveAuthzRoleDefinitionInput{
		RoleSlug:            "dict-reader",
		Name:                "Dict Reader",
		AuthzCapabilityKeys: []string{"iam.dicts:read"},
	})
	if err != nil {
		t.Fatalf("create custom role err=%v", err)
	}
	_, err = store.ReplacePrincipalAssignment(ctx, "tenant-a", "principal-a", replacePrincipalAssignmentInput{
		Roles:    []string{customRole.RoleSlug},
		Revision: assignment.Revision,
	})
	if err != nil {
		t.Fatalf("replace err=%v", err)
	}
	if err := store.EnsurePrincipalRoleAssignment(ctx, "tenant-a", "principal-a", authz.RoleTenantAdmin); err != nil {
		t.Fatalf("ensure existing err=%v", err)
	}
	if _, err := store.OrgScopesForPrincipal(ctx, "tenant-a", "principal-a", "orgunit.orgunits:read"); !errors.Is(err, errAuthzOrgScopeRequired) {
		t.Fatalf("expected explicit empty scopes to stay empty, got %v", err)
	}
}
