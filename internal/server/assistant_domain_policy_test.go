package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDomainPolicyRepoConfigIsValid(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH", filepath.Join(wd, "..", "..", "config", "assistant", "domain-allowlist.yaml"))

	policy, err := readAssistantDomainPolicy()
	if err != nil {
		t.Fatalf("read policy: %v", err)
	}
	if policy.Version != 1 {
		t.Fatalf("version=%d", policy.Version)
	}
	if !strings.EqualFold(policy.Default, "deny") {
		t.Fatalf("default=%q", policy.Default)
	}
}

func TestValidateAssistantDomainPolicy(t *testing.T) {
	valid := assistantDomainPolicy{
		Version: 1,
		Default: "deny",
		Sources: map[string]assistantDomainPolicySource{
			"mcp": {
				AllowedDomains: []string{"api.openai.com", "*.openai.com"},
			},
			"actions": {
				AllowedDomains: []string{"api.openai.com"},
			},
		},
		BlockedDomains: []string{"localhost", "127.0.0.1", "169.254.169.254"},
	}
	if err := validateAssistantDomainPolicy(valid); err != nil {
		t.Fatalf("valid policy rejected: %v", err)
	}

	invalidDefault := valid
	invalidDefault.Default = "allow"
	if err := validateAssistantDomainPolicy(invalidDefault); err == nil {
		t.Fatal("expected default=allow to fail")
	}

	invalidDomain := valid
	invalidDomain.Sources = map[string]assistantDomainPolicySource{
		"mcp": {
			AllowedDomains: []string{"https://api.openai.com"},
		},
		"actions": {
			AllowedDomains: []string{"api.openai.com"},
		},
	}
	if err := validateAssistantDomainPolicy(invalidDomain); err == nil {
		t.Fatal("expected domain with scheme to fail")
	}

	dangerousDomain := valid
	dangerousDomain.Sources = map[string]assistantDomainPolicySource{
		"mcp": {
			AllowedDomains: []string{"localhost"},
		},
		"actions": {
			AllowedDomains: []string{"api.openai.com"},
		},
	}
	if err := validateAssistantDomainPolicy(dangerousDomain); err == nil {
		t.Fatal("expected localhost to fail")
	}

	missingBlocked := valid
	missingBlocked.BlockedDomains = []string{"localhost", "127.0.0.1"}
	if err := validateAssistantDomainPolicy(missingBlocked); err == nil {
		t.Fatal("expected missing blocked domain to fail")
	}

	missingSource := valid
	missingSource.Sources = map[string]assistantDomainPolicySource{
		"mcp": {AllowedDomains: []string{"api.openai.com"}},
	}
	if err := validateAssistantDomainPolicy(missingSource); err == nil {
		t.Fatal("expected missing actions source to fail")
	}

	emptyAllowed := valid
	emptyAllowed.Sources = map[string]assistantDomainPolicySource{
		"mcp":     {AllowedDomains: []string{"api.openai.com"}},
		"actions": {AllowedDomains: nil},
	}
	if err := validateAssistantDomainPolicy(emptyAllowed); err == nil {
		t.Fatal("expected empty allowed_domains to fail")
	}

	invalidBlocked := valid
	invalidBlocked.BlockedDomains = []string{"localhost", "127.0.0.1", "https://169.254.169.254"}
	if err := validateAssistantDomainPolicy(invalidBlocked); err == nil {
		t.Fatal("expected blocked domain with scheme to fail")
	}
}

func TestAssistantRuntimeAgentsWriteEnabled(t *testing.T) {
	t.Setenv("ASSISTANT_AGENTS_WRITE_ENABLED", "")
	t.Setenv("LIBRECHAT_AGENTS_WRITE_ENABLED", "")
	if assistantRuntimeAgentsWriteEnabled() {
		t.Fatal("expected false by default")
	}

	t.Setenv("ASSISTANT_AGENTS_WRITE_ENABLED", "true")
	if !assistantRuntimeAgentsWriteEnabled() {
		t.Fatal("expected true when ASSISTANT_AGENTS_WRITE_ENABLED=true")
	}

	t.Setenv("ASSISTANT_AGENTS_WRITE_ENABLED", "")
	t.Setenv("LIBRECHAT_AGENTS_WRITE_ENABLED", "true")
	if !assistantRuntimeAgentsWriteEnabled() {
		t.Fatal("expected true when LIBRECHAT_AGENTS_WRITE_ENABLED=true")
	}

	t.Setenv("LIBRECHAT_AGENTS_WRITE_ENABLED", "not-a-bool")
	if assistantRuntimeAgentsWriteEnabled() {
		t.Fatal("expected invalid bool to fail-closed")
	}
}

func TestAssistantRuntimeCutoverMode(t *testing.T) {
	t.Setenv("ASSISTANT_RUNTIME_CUTOVER_MODE", assistantRuntimeCutoverModeCutoverPrep)
	if got := assistantRuntimeCutoverMode(); got != assistantRuntimeCutoverModeCutoverPrep {
		t.Fatalf("cutover prep mode=%q", got)
	}
	t.Setenv("ASSISTANT_RUNTIME_CUTOVER_MODE", assistantRuntimeCutoverModeUIShellOnly)
	if got := assistantRuntimeCutoverMode(); got != assistantRuntimeCutoverModeUIShellOnly {
		t.Fatalf("ui shell only mode=%q", got)
	}
}

func TestReadAssistantDomainPolicy_ErrorAndSuccess(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH", filepath.Join(dir, "missing.yaml"))
	if _, err := readAssistantDomainPolicy(); !errors.Is(err, errAssistantDomainPolicyMissing) {
		t.Fatalf("expected missing error, got=%v", err)
	}

	t.Setenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH", dir)
	if _, err := readAssistantDomainPolicy(); !errors.Is(err, errAssistantDomainPolicyInvalid) {
		t.Fatalf("expected invalid policy error for unreadable path, got=%v", err)
	}

	invalidYAMLPath := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(invalidYAMLPath, []byte("version: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH", invalidYAMLPath)
	if _, err := readAssistantDomainPolicy(); !errors.Is(err, errAssistantDomainPolicyInvalid) {
		t.Fatalf("expected invalid policy error for bad yaml, got=%v", err)
	}

	badPolicyPath := filepath.Join(dir, "bad-policy.yaml")
	if err := os.WriteFile(badPolicyPath, []byte("version: 2\ndefault: deny\nsources: {}\nblocked_domains: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH", badPolicyPath)
	if _, err := readAssistantDomainPolicy(); !errors.Is(err, errAssistantDomainPolicyInvalid) {
		t.Fatalf("expected invalid policy error for bad policy, got=%v", err)
	}

	goodPolicyPath := filepath.Join(dir, "good-policy.yaml")
	goodPolicy := "version: 1\ndefault: deny\nsources:\n  mcp:\n    allowed_domains: [\"api.openai.com\"]\n  actions:\n    allowed_domains: [\"api.openai.com\"]\nblocked_domains: [\"localhost\", \"127.0.0.1\", \"169.254.169.254\"]\n"
	if err := os.WriteFile(goodPolicyPath, []byte(goodPolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH", goodPolicyPath)
	if _, err := readAssistantDomainPolicy(); err != nil {
		t.Fatalf("expected success, got=%v", err)
	}
}

func TestNormalizeAssistantDomainPattern(t *testing.T) {
	if _, err := normalizeAssistantDomainPattern(""); err == nil {
		t.Fatal("expected empty domain to fail")
	}
	if _, err := normalizeAssistantDomainPattern("https://api.openai.com"); err == nil {
		t.Fatal("expected scheme to fail")
	}
	if _, err := normalizeAssistantDomainPattern("@api.openai.com"); err == nil {
		t.Fatal("expected auth marker to fail")
	}
	if _, err := normalizeAssistantDomainPattern("*."); err == nil {
		t.Fatal("expected empty wildcard suffix to fail")
	}
	if _, err := normalizeAssistantDomainPattern("*._bad.com"); err == nil {
		t.Fatal("expected invalid wildcard suffix to fail")
	}
	if got, err := normalizeAssistantDomainPattern("*.openai.com"); err != nil || got != "*.openai.com" {
		t.Fatalf("wildcard normalize failed, got=%q err=%v", got, err)
	}
	if _, err := normalizeAssistantDomainPattern("a*b.com"); err == nil {
		t.Fatal("expected wildcard-in-middle to fail")
	}
	if got, err := normalizeAssistantDomainPattern("127.0.0.1"); err != nil || got != "127.0.0.1" {
		t.Fatalf("ip normalize failed, got=%q err=%v", got, err)
	}
	if _, err := normalizeAssistantDomainPattern("bad_domain"); err == nil {
		t.Fatal("expected invalid hostname to fail")
	}
	if got, err := normalizeAssistantDomainPattern("Api.OpenAI.com"); err != nil || got != "api.openai.com" {
		t.Fatalf("hostname normalize failed, got=%q err=%v", got, err)
	}
}

func TestAssistantDomainPatternDangerous(t *testing.T) {
	dangerous := []string{
		"localhost",
		"foo.localhost",
		"::1",
		"0.0.0.0",
		"169.254.1.2",
		"10.0.0.1",
		"127.0.0.2",
		"172.16.10.9",
		"192.168.1.1",
	}
	for _, pattern := range dangerous {
		if !assistantDomainPatternDangerous(pattern) {
			t.Fatalf("expected dangerous pattern: %s", pattern)
		}
	}
	if assistantDomainPatternDangerous("api.openai.com") {
		t.Fatal("expected api.openai.com to be safe")
	}
	if !assistantDomainPatternDangerous("*.localhost") {
		t.Fatal("expected *.localhost to be dangerous")
	}
	if assistantDomainPatternDangerous("172.15.0.1") {
		t.Fatal("expected 172.15.0.1 to be safe")
	}
	if !assistantDomainPatternDangerous("::") {
		t.Fatal("expected :: to be dangerous")
	}
}

func TestAssistantRuntimeCapabilitiesStatus(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	policy := "version: 1\ndefault: deny\nsources:\n  mcp:\n    allowed_domains: [\"api.openai.com\"]\n  actions:\n    allowed_domains: [\"api.openai.com\"]\nblocked_domains: [\"localhost\", \"127.0.0.1\", \"169.254.169.254\"]\n"
	if err := os.WriteFile(policyPath, []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH", policyPath)
	t.Setenv("ASSISTANT_AGENTS_WRITE_ENABLED", "true")

	capabilities, err := assistantRuntimeCapabilitiesStatus()
	if err != nil {
		t.Fatalf("capabilities err=%v", err)
	}
	if !capabilities.MCPEnabled || !capabilities.ActionsEnabled || !capabilities.AgentsWriteEnabled {
		t.Fatalf("capabilities=%+v", capabilities)
	}
	if capabilities.DomainPolicyVersion != "v1" {
		t.Fatalf("domain policy version=%q", capabilities.DomainPolicyVersion)
	}
}
