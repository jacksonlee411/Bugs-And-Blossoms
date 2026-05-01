package authz_test

import (
	"os"
	"path/filepath"
	"testing"

	authz "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

func TestModeFromEnv_Default_BlackBox(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "")
	m, err := authz.ModeFromEnv()
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if m != authz.ModeEnforce {
		t.Fatalf("mode=%q", m)
	}
}

func TestModeFromEnv_Shadow_BlackBox(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "shadow")
	m, err := authz.ModeFromEnv()
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if m != authz.ModeShadow {
		t.Fatalf("mode=%q", m)
	}
}

func TestModeFromEnv_DisabledRequiresUnsafe_BlackBox(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "")
	if _, err := authz.ModeFromEnv(); err == nil {
		t.Fatal("expected error")
	}
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")
	m, err := authz.ModeFromEnv()
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if m != authz.ModeDisabled {
		t.Fatalf("mode=%q", m)
	}
}

func TestModeFromEnv_Invalid_BlackBox(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "nope")
	if _, err := authz.ModeFromEnv(); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewAuthorizer_AndAuthorize_BlackBox(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.conf")
	policy := filepath.Join(dir, "policy.csv")

	if err := os.WriteFile(model, []byte(`
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.dom == p.dom && r.obj == p.obj && r.act == p.act
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("p, role:tenant-admin, t1, orgunit.read, read\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := authz.NewAuthorizer(model, policy, authz.ModeEnforce)
	if err != nil {
		t.Fatalf("err=%v", err)
	}

	allowed, enforced, err := a.Authorize("role:tenant-admin", "t1", "orgunit.read", "read")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !enforced || !allowed {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}

	allowed, enforced, err = a.Authorize("role:tenant-admin", "t1", "orgunit.read", "admin")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !enforced || allowed {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}

	aShadow, err := authz.NewAuthorizer(model, policy, authz.ModeShadow)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	allowed, enforced, err = aShadow.Authorize("role:tenant-admin", "t1", "orgunit.read", "admin")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if enforced || allowed {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}

	aDisabled, err := authz.NewAuthorizer(model, policy, authz.ModeDisabled)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	allowed, enforced, err = aDisabled.Authorize("role:tenant-admin", "t1", "orgunit.read", "admin")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if enforced || !allowed {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}
}

func TestNewAuthorizer_Error_BlackBox(t *testing.T) {
	dir := t.TempDir()
	invalidModel := filepath.Join(dir, "invalid.conf")
	if err := os.WriteFile(invalidModel, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := authz.NewAuthorizer(invalidModel, "nope-policy.csv", authz.ModeEnforce); err == nil {
		t.Fatal("expected error")
	}

	model := filepath.Join(dir, "model.conf")
	policyDir := filepath.Join(dir, "policy-dir")
	if err := os.WriteFile(model, []byte(`
[request_definition]
r = sub, dom, obj, act
[policy_definition]
p = sub, dom, obj, act
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == p.sub && r.dom == p.dom && r.obj == p.obj && r.act == p.act
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(policyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := authz.NewAuthorizer(model, policyDir, authz.ModeEnforce); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewAuthorizer_LoadPolicyError_BlackBox(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.conf")
	policy := filepath.Join(dir, "missing-policy.csv")

	if err := os.WriteFile(model, []byte(`
[request_definition]
r = sub, dom, obj, act
[policy_definition]
p = sub, dom, obj, act
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == p.sub && r.dom == p.dom && r.obj == p.obj && r.act == p.act
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := authz.NewAuthorizer(model, policy, authz.ModeEnforce); err == nil {
		t.Fatal("expected error")
	}
}

func TestSubjectFromRoleSlug_BlackBox(t *testing.T) {
	if got := authz.SubjectFromRoleSlug(""); got != "role:anonymous" {
		t.Fatalf("got=%q", got)
	}
	if got := authz.SubjectFromRoleSlug("Tenant-Admin"); got != "role:tenant-admin" {
		t.Fatalf("got=%q", got)
	}
}

func TestDomainFromTenantID_BlackBox(t *testing.T) {
	if got := authz.DomainFromTenantID(" ABC "); got != "abc" {
		t.Fatalf("got=%q", got)
	}
}

func TestAuthzCapabilityKey_BlackBox(t *testing.T) {
	if got := authz.AuthzCapabilityKey(authz.ObjectOrgUnitOrgUnits, authz.ActionRead); got != "orgunit.orgunits:read" {
		t.Fatalf("got=%q", got)
	}
}

func TestAuthorize_UnknownMode_BlackBox(t *testing.T) {
	a := &authz.Authorizer{}
	if _, _, err := a.Authorize("role:x", "d", "o", "a"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthorize_EnforceError_BlackBox(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.conf")
	policy := filepath.Join(dir, "policy.csv")
	if err := os.WriteFile(model, []byte(`
[request_definition]
r = sub, dom, obj, act
[policy_definition]
p = sub, dom, obj, act
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == 
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("p, role:tenant-admin, t1, orgunit.read, read\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	aShadow, err := authz.NewAuthorizer(model, policy, authz.ModeShadow)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	allowed, enforced, err := aShadow.Authorize("role:tenant-admin", "t1", "orgunit.read", "read")
	if err == nil {
		t.Fatal("expected error")
	}
	if allowed || enforced {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}

	aEnforce, err := authz.NewAuthorizer(model, policy, authz.ModeEnforce)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	allowed, enforced, err = aEnforce.Authorize("role:tenant-admin", "t1", "orgunit.read", "read")
	if err == nil {
		t.Fatal("expected error")
	}
	if allowed || !enforced {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}
}

func BenchmarkSubjectFromRoleSlug_BlackBox(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = authz.SubjectFromRoleSlug("Tenant-Admin")
	}
}
