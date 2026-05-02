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
	rootOrgNodeKey, err := encodeOrgNodeKeyFromID(10000000)
	if err != nil {
		t.Fatalf("encode err=%v", err)
	}

	customRole, err := store.CreateRoleDefinition(ctx, "tenant-a", saveAuthzRoleDefinitionInput{
		RoleSlug:            "flower-hr",
		Name:                "Flower HR",
		AuthzCapabilityKeys: []string{"iam.dicts:read"},
	})
	if err != nil {
		t.Fatalf("create role err=%v", err)
	}
	assignment, err := store.ReplacePrincipalAssignment(ctx, "tenant-a", "principal-a", replacePrincipalAssignmentInput{
		Roles:    []string{authz.RoleTenantAdmin, customRole.RoleSlug},
		Revision: 1,
		OrgScopes: []principalOrgScope{{
			OrgNodeKey:         rootOrgNodeKey,
			IncludeDescendants: true,
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
	want := []string{customRole.RoleSlug, authz.RoleTenantViewer}
	if !reflect.DeepEqual(slugs, want) {
		t.Fatalf("roles=%v want=%v", slugs, want)
	}
}
