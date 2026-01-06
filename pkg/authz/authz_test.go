package authz

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModeFromEnv_Default(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "")
	m, err := ModeFromEnv()
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if m != ModeEnforce {
		t.Fatalf("mode=%q", m)
	}
}

func TestModeFromEnv_Shadow(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "shadow")
	m, err := ModeFromEnv()
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if m != ModeShadow {
		t.Fatalf("mode=%q", m)
	}
}

func TestModeFromEnv_DisabledRequiresUnsafe(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "")
	if _, err := ModeFromEnv(); err == nil {
		t.Fatal("expected error")
	}
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")
	m, err := ModeFromEnv()
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if m != ModeDisabled {
		t.Fatalf("mode=%q", m)
	}
}

func TestModeFromEnv_Invalid(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "nope")
	if _, err := ModeFromEnv(); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewAuthorizer_AndAuthorize(t *testing.T) {
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
	if err := os.WriteFile(policy, []byte("p, role:tenant-admin, t1, jobcatalog.catalog, read\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := NewAuthorizer(model, policy, ModeEnforce)
	if err != nil {
		t.Fatalf("err=%v", err)
	}

	allowed, enforced, err := a.Authorize("role:tenant-admin", "t1", "jobcatalog.catalog", "read")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !enforced || !allowed {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}

	allowed, enforced, err = a.Authorize("role:tenant-admin", "t1", "jobcatalog.catalog", "admin")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !enforced || allowed {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}

	aShadow, err := NewAuthorizer(model, policy, ModeShadow)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	allowed, enforced, err = aShadow.Authorize("role:tenant-admin", "t1", "jobcatalog.catalog", "admin")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if enforced || allowed {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}

	aDisabled, err := NewAuthorizer(model, policy, ModeDisabled)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	allowed, enforced, err = aDisabled.Authorize("role:tenant-admin", "t1", "jobcatalog.catalog", "admin")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if enforced || !allowed {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}
}

func TestNewAuthorizer_Error(t *testing.T) {
	dir := t.TempDir()
	invalidModel := filepath.Join(dir, "invalid.conf")
	if err := os.WriteFile(invalidModel, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewAuthorizer(invalidModel, "nope-policy.csv", ModeEnforce); err == nil {
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
	if _, err := NewAuthorizer(model, policyDir, ModeEnforce); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewAuthorizer_LoadPolicyError(t *testing.T) {
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

	if _, err := NewAuthorizer(model, policy, ModeEnforce); err == nil {
		t.Fatal("expected error")
	}
}

func TestSubjectFromRoleSlug(t *testing.T) {
	if got := SubjectFromRoleSlug(""); got != "role:anonymous" {
		t.Fatalf("got=%q", got)
	}
	if got := SubjectFromRoleSlug("Tenant-Admin"); got != "role:tenant-admin" {
		t.Fatalf("got=%q", got)
	}
}

func TestDomainFromTenantID(t *testing.T) {
	if got := DomainFromTenantID(" ABC "); got != "abc" {
		t.Fatalf("got=%q", got)
	}
}

func TestAuthorize_UnknownMode(t *testing.T) {
	a := &Authorizer{mode: Mode("nope")}
	if _, _, err := a.Authorize("role:x", "d", "o", "a"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthorize_EnforceError(t *testing.T) {
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
	if err := os.WriteFile(policy, []byte("p, role:tenant-admin, t1, jobcatalog.catalog, read\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	aShadow, err := NewAuthorizer(model, policy, ModeShadow)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	allowed, enforced, err := aShadow.Authorize("role:tenant-admin", "t1", "jobcatalog.catalog", "read")
	if err == nil {
		t.Fatal("expected error")
	}
	if allowed || enforced {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}

	aEnforce, err := NewAuthorizer(model, policy, ModeEnforce)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	allowed, enforced, err = aEnforce.Authorize("role:tenant-admin", "t1", "jobcatalog.catalog", "read")
	if err == nil {
		t.Fatal("expected error")
	}
	if allowed || !enforced {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}
}
