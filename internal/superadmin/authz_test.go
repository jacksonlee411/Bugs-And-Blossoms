package superadmin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultAuthzPaths(t *testing.T) {
	modelPath, err := defaultAuthzModelPath()
	if err != nil {
		t.Fatal(err)
	}
	if modelPath == "" {
		t.Fatal("expected model path")
	}

	policyPath, err := defaultAuthzPolicyPath()
	if err != nil {
		t.Fatal(err)
	}
	if policyPath == "" {
		t.Fatal("expected policy path")
	}
}

func TestDefaultAuthzPaths_NotFound(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	if _, err := defaultAuthzModelPath(); err == nil {
		t.Fatal("expected error")
	}
	if _, err := defaultAuthzPolicyPath(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadAuthorizer_Success(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	model := filepath.Clean(filepath.Join(wd, "..", "..", "config", "access", "model.conf"))
	policy := filepath.Clean(filepath.Join(wd, "..", "..", "config", "access", "policy.csv"))

	t.Setenv("AUTHZ_MODEL_PATH", model)
	t.Setenv("AUTHZ_POLICY_PATH", policy)
	t.Setenv("AUTHZ_MODE", "enforce")

	a, err := loadAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("nil authorizer")
	}
}

func TestLoadAuthorizer_ModeError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	model := filepath.Clean(filepath.Join(wd, "..", "..", "config", "access", "model.conf"))
	policy := filepath.Clean(filepath.Join(wd, "..", "..", "config", "access", "policy.csv"))

	t.Setenv("AUTHZ_MODEL_PATH", model)
	t.Setenv("AUTHZ_POLICY_PATH", policy)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "")

	_, err = loadAuthorizer()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadAuthorizer_DefaultPaths_Success(t *testing.T) {
	t.Setenv("AUTHZ_MODEL_PATH", "")
	t.Setenv("AUTHZ_POLICY_PATH", "")
	t.Setenv("AUTHZ_MODE", "enforce")

	a, err := loadAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("nil authorizer")
	}
}

func TestLoadAuthorizer_DefaultPaths_NotFound(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	t.Setenv("AUTHZ_MODEL_PATH", "")
	t.Setenv("AUTHZ_POLICY_PATH", "")
	t.Setenv("AUTHZ_MODE", "enforce")

	if _, err := loadAuthorizer(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadAuthorizer_ModelFromEnv_PolicyDefault_Success(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	model := filepath.Clean(filepath.Join(wd, "..", "..", "config", "access", "model.conf"))

	t.Setenv("AUTHZ_MODEL_PATH", model)
	t.Setenv("AUTHZ_POLICY_PATH", "")
	t.Setenv("AUTHZ_MODE", "enforce")

	a, err := loadAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("nil authorizer")
	}
}

func TestLoadAuthorizer_ModelDefault_PolicyFromEnv_Success(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	policy := filepath.Clean(filepath.Join(wd, "..", "..", "config", "access", "policy.csv"))

	t.Setenv("AUTHZ_MODEL_PATH", "")
	t.Setenv("AUTHZ_POLICY_PATH", policy)
	t.Setenv("AUTHZ_MODE", "enforce")

	a, err := loadAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("nil authorizer")
	}
}

func TestLoadAuthorizer_PolicyDefault_NotFound(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	model := filepath.Clean(filepath.Join(wd, "..", "..", "config", "access", "model.conf"))

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	t.Setenv("AUTHZ_MODEL_PATH", model)
	t.Setenv("AUTHZ_POLICY_PATH", "")
	t.Setenv("AUTHZ_MODE", "enforce")

	if _, err := loadAuthorizer(); err == nil {
		t.Fatal("expected error")
	}
}
