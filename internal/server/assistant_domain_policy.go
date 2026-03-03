package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultAssistantDomainAllowlistPath = "config/assistant/domain-allowlist.yaml"

var (
	errAssistantDomainPolicyMissing = errors.New("assistant_domain_policy_missing")
	errAssistantDomainPolicyInvalid = errors.New("assistant_domain_policy_invalid")
)

var assistantDomainPatternRegex = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*$`)

type assistantDomainPolicy struct {
	Version        int                                    `yaml:"version"`
	Default        string                                 `yaml:"default"`
	Sources        map[string]assistantDomainPolicySource `yaml:"sources"`
	BlockedDomains []string                               `yaml:"blocked_domains"`
}

type assistantDomainPolicySource struct {
	AllowedDomains []string `yaml:"allowed_domains"`
}

type assistantRuntimeCapabilities struct {
	MCPEnabled          bool   `json:"mcp_enabled"`
	ActionsEnabled      bool   `json:"actions_enabled"`
	AgentsWriteEnabled  bool   `json:"agents_write_enabled"`
	DomainPolicyVersion string `json:"domain_policy_version,omitempty"`
}

func readAssistantDomainPolicy() (assistantDomainPolicy, error) {
	var policy assistantDomainPolicy
	path := assistantRuntimeResolvePath(strings.TrimSpace(os.Getenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH")), defaultAssistantDomainAllowlistPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return policy, errAssistantDomainPolicyMissing
		}
		return policy, fmt.Errorf("%w: %v", errAssistantDomainPolicyInvalid, err)
	}
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return policy, fmt.Errorf("%w: %v", errAssistantDomainPolicyInvalid, err)
	}
	if err := validateAssistantDomainPolicy(policy); err != nil {
		return policy, fmt.Errorf("%w: %v", errAssistantDomainPolicyInvalid, err)
	}
	return policy, nil
}

func validateAssistantDomainPolicy(policy assistantDomainPolicy) error {
	if policy.Version != 1 {
		return fmt.Errorf("unsupported version: %d", policy.Version)
	}
	if !strings.EqualFold(strings.TrimSpace(policy.Default), "deny") {
		return errors.New("default must be deny")
	}
	requiredSources := []string{"mcp", "actions"}
	for _, source := range requiredSources {
		config, ok := policy.Sources[source]
		if !ok {
			return fmt.Errorf("missing source: %s", source)
		}
		if len(config.AllowedDomains) == 0 {
			return fmt.Errorf("source %s has empty allowed_domains", source)
		}
		for _, pattern := range config.AllowedDomains {
			normalized, err := normalizeAssistantDomainPattern(pattern)
			if err != nil {
				return fmt.Errorf("source %s has invalid allowed domain %q: %w", source, pattern, err)
			}
			if assistantDomainPatternDangerous(normalized) {
				return fmt.Errorf("source %s has disallowed domain %q", source, pattern)
			}
		}
	}

	requiredBlocked := map[string]bool{
		"localhost":       false,
		"127.0.0.1":       false,
		"169.254.169.254": false,
	}
	for _, item := range policy.BlockedDomains {
		normalized, err := normalizeAssistantDomainPattern(item)
		if err != nil {
			return fmt.Errorf("invalid blocked domain %q: %w", item, err)
		}
		if _, ok := requiredBlocked[normalized]; ok {
			requiredBlocked[normalized] = true
		}
	}
	for domain, seen := range requiredBlocked {
		if !seen {
			return fmt.Errorf("blocked_domains must include %s", domain)
		}
	}
	return nil
}

func normalizeAssistantDomainPattern(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", errors.New("empty domain")
	}
	if strings.Contains(value, "://") || strings.Contains(value, "/") || strings.Contains(value, " ") || strings.Contains(value, "@") {
		return "", errors.New("domain must not include scheme, path, auth or spaces")
	}
	if strings.HasPrefix(value, "*.") {
		suffix := strings.TrimPrefix(value, "*.")
		if suffix == "" {
			return "", errors.New("wildcard suffix is empty")
		}
		if !assistantDomainPatternRegex.MatchString(suffix) {
			return "", errors.New("invalid wildcard suffix")
		}
		return value, nil
	}
	if strings.Contains(value, "*") {
		return "", errors.New("wildcard must be prefix '*.'")
	}
	if net.ParseIP(value) != nil {
		return value, nil
	}
	if !assistantDomainPatternRegex.MatchString(value) {
		return "", errors.New("invalid domain")
	}
	return value, nil
}

func assistantDomainPatternDangerous(pattern string) bool {
	p := strings.TrimSpace(strings.ToLower(pattern))
	base := strings.TrimPrefix(p, "*.")
	if base == "localhost" || strings.HasSuffix(base, ".localhost") {
		return true
	}
	if ip := net.ParseIP(base); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
			return true
		}
		if ip4 := ip.To4(); ip4 != nil {
			if ip4[0] == 10 {
				return true
			}
			if ip4[0] == 127 {
				return true
			}
			if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
				return true
			}
			if ip4[0] == 192 && ip4[1] == 168 {
				return true
			}
		}
	}
	return false
}

func assistantRuntimeCapabilitiesStatus() (assistantRuntimeCapabilities, error) {
	capabilities := assistantRuntimeCapabilities{
		AgentsWriteEnabled: assistantRuntimeAgentsWriteEnabled(),
	}
	policy, err := readAssistantDomainPolicy()
	if err != nil {
		return capabilities, err
	}
	capabilities.MCPEnabled = len(policy.Sources["mcp"].AllowedDomains) > 0
	capabilities.ActionsEnabled = len(policy.Sources["actions"].AllowedDomains) > 0
	capabilities.DomainPolicyVersion = fmt.Sprintf("v%d", policy.Version)
	return capabilities, nil
}

func assistantRuntimeAgentsWriteEnabled() bool {
	candidates := []string{
		strings.TrimSpace(os.Getenv("ASSISTANT_AGENTS_WRITE_ENABLED")),
		strings.TrimSpace(os.Getenv("LIBRECHAT_AGENTS_WRITE_ENABLED")),
	}
	for _, raw := range candidates {
		if raw == "" {
			continue
		}
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return false
		}
		return enabled
	}
	return false
}
