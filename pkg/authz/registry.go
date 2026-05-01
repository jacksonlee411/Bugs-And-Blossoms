package authz

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	RoleTenantAdmin  = "tenant-admin"
	RoleTenantViewer = "tenant-viewer"
	RoleAnonymous    = "anonymous"
	RoleSuperadmin   = "superadmin"
)

const (
	ActionRead       = "read"
	ActionAdmin      = "admin"
	ActionUpdate     = "update"
	ActionRotate     = "rotate"
	ActionSelect     = "select"
	ActionVerify     = "verify"
	ActionDeactivate = "deactivate"
	ActionUse        = "use"
)

const DomainGlobal = "global"

const (
	ObjectIAMSession             = "iam.session"
	ObjectIAMAuthz               = "iam.authz"
	ObjectIAMDicts               = "iam.dicts"
	ObjectIAMDictRelease         = "iam.dict_release"
	ObjectCubeBoxConversations   = "cubebox.conversations"
	ObjectCubeBoxModelProvider   = "cubebox.model_provider"
	ObjectCubeBoxModelCredential = "cubebox.model_credential"
	ObjectCubeBoxModelSelection  = "cubebox.model_selection"
	ObjectOrgUnitOrgUnits        = "orgunit.orgunits"

	ObjectSuperadminTenants = "superadmin.tenants"
	ObjectSuperadminSession = "superadmin.session"
)

const (
	CapabilityStatusEnabled    = "enabled"
	CapabilityStatusDisabled   = "disabled"
	CapabilityStatusDeprecated = "deprecated"
)

const (
	CapabilitySurfaceTenantAPI       = "tenant_api"
	CapabilitySurfaceSuperadminRoute = "superadmin_route"
	CapabilitySurfaceInternalSystem  = "internal_system"
)

const (
	ScopeDimensionNone         = "none"
	ScopeDimensionOrganization = "organization"
)

const RegistryRevision = "20260501-static"

type AuthzCapability struct {
	Key            string `json:"authz_capability_key"`
	Object         string `json:"object"`
	Action         string `json:"action"`
	OwnerModule    string `json:"owner_module"`
	ResourceLabel  string `json:"resource_label"`
	ActionLabel    string `json:"action_label"`
	ScopeDimension string `json:"scope_dimension"`
	Assignable     bool   `json:"assignable"`
	Status         string `json:"status"`
	Surface        string `json:"surface"`
	SortOrder      int    `json:"sort_order"`
}

type AuthzCapabilityOption struct {
	AuthzCapability
	Label   string `json:"label"`
	Covered bool   `json:"covered"`
}

type CapabilityListFilter struct {
	Query             string
	OwnerModule       string
	ScopeDimension    string
	IncludeDisabled   bool
	IncludeUncovered  bool
	RequireAssignable bool
	RequireTenantAPI  bool
	CoveredKeys       map[string]bool
}

var authzKeyPartPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)*$`)

func AuthzCapabilityKey(object string, action string) string {
	return strings.TrimSpace(object) + ":" + strings.TrimSpace(action)
}

func ParseAuthzCapabilityKey(key string) (object string, action string, err error) {
	key = strings.TrimSpace(key)
	object, action, ok := strings.Cut(key, ":")
	if !ok || strings.Contains(action, ":") {
		return "", "", fmt.Errorf("invalid authz capability key %q", key)
	}
	object = strings.TrimSpace(object)
	action = strings.TrimSpace(action)
	if !authzKeyPartPattern.MatchString(object) || !authzKeyPartPattern.MatchString(action) {
		return "", "", fmt.Errorf("invalid authz capability key %q", key)
	}
	return object, action, nil
}

var registryEntries = []AuthzCapability{
	capability(ObjectIAMSession, ActionAdmin, "iam", "会话", "登录/退出", ScopeDimensionNone, false, CapabilitySurfaceTenantAPI, 10),
	capability(ObjectIAMAuthz, ActionRead, "iam", "功能授权项", "查看", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 20),
	capability(ObjectIAMDicts, ActionRead, "iam", "字典配置", "查看", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 100),
	capability(ObjectIAMDicts, ActionAdmin, "iam", "字典配置", "管理", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 110),
	capability(ObjectIAMDictRelease, ActionAdmin, "iam", "字典发布", "管理", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 120),
	capability(ObjectOrgUnitOrgUnits, ActionRead, "orgunit", "组织管理", "查看", ScopeDimensionOrganization, true, CapabilitySurfaceTenantAPI, 200),
	capability(ObjectOrgUnitOrgUnits, ActionAdmin, "orgunit", "组织管理", "管理", ScopeDimensionOrganization, true, CapabilitySurfaceTenantAPI, 210),
	capability(ObjectCubeBoxConversations, ActionRead, "cubebox", "CubeBox 对话", "查看", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 300),
	capability(ObjectCubeBoxConversations, ActionUse, "cubebox", "CubeBox 对话", "使用", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 310),
	capability(ObjectCubeBoxModelCredential, ActionRead, "cubebox", "CubeBox 模型凭据", "查看", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 320),
	capability(ObjectCubeBoxModelCredential, ActionRotate, "cubebox", "CubeBox 模型凭据", "轮换", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 330),
	capability(ObjectCubeBoxModelCredential, ActionDeactivate, "cubebox", "CubeBox 模型凭据", "停用", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 340),
	capability(ObjectCubeBoxModelProvider, ActionUpdate, "cubebox", "CubeBox 模型 Provider", "更新", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 350),
	capability(ObjectCubeBoxModelSelection, ActionSelect, "cubebox", "CubeBox 当前模型", "选择", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 360),
	capability(ObjectCubeBoxModelSelection, ActionVerify, "cubebox", "CubeBox 当前模型", "验证", ScopeDimensionNone, true, CapabilitySurfaceTenantAPI, 370),
	capability(ObjectSuperadminSession, ActionRead, "superadmin", "超级管理员会话", "查看", ScopeDimensionNone, false, CapabilitySurfaceSuperadminRoute, 900),
	capability(ObjectSuperadminSession, ActionAdmin, "superadmin", "超级管理员会话", "管理", ScopeDimensionNone, false, CapabilitySurfaceSuperadminRoute, 910),
	capability(ObjectSuperadminTenants, ActionRead, "superadmin", "租户控制台", "查看", ScopeDimensionNone, false, CapabilitySurfaceSuperadminRoute, 920),
	capability(ObjectSuperadminTenants, ActionAdmin, "superadmin", "租户控制台", "管理", ScopeDimensionNone, false, CapabilitySurfaceSuperadminRoute, 930),
}

func capability(object, action, ownerModule, resourceLabel, actionLabel, scopeDimension string, assignable bool, surface string, sortOrder int) AuthzCapability {
	return AuthzCapability{
		Key:            AuthzCapabilityKey(object, action),
		Object:         object,
		Action:         action,
		OwnerModule:    ownerModule,
		ResourceLabel:  resourceLabel,
		ActionLabel:    actionLabel,
		ScopeDimension: scopeDimension,
		Assignable:     assignable,
		Status:         CapabilityStatusEnabled,
		Surface:        surface,
		SortOrder:      sortOrder,
	}
}

func ListAuthzCapabilities() []AuthzCapability {
	out := append([]AuthzCapability(nil), registryEntries...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].Key < out[j].Key
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out
}

func LookupAuthzCapability(key string) (AuthzCapability, bool) {
	key = strings.TrimSpace(key)
	for _, entry := range registryEntries {
		if entry.Key == key {
			return entry, true
		}
	}
	return AuthzCapability{}, false
}

func LookupAuthzCapabilityByObjectAction(object string, action string) (AuthzCapability, bool) {
	return LookupAuthzCapability(AuthzCapabilityKey(object, action))
}

func ListAuthzCapabilityOptions(filter CapabilityListFilter) []AuthzCapabilityOption {
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	ownerModule := strings.TrimSpace(filter.OwnerModule)
	scopeDimension := strings.TrimSpace(filter.ScopeDimension)
	out := make([]AuthzCapabilityOption, 0)
	for _, entry := range ListAuthzCapabilities() {
		covered := filter.CoveredKeys[entry.Key]
		if filter.RequireAssignable && !entry.Assignable {
			continue
		}
		if filter.RequireTenantAPI && entry.Surface != CapabilitySurfaceTenantAPI {
			continue
		}
		if !filter.IncludeDisabled && entry.Status != CapabilityStatusEnabled {
			continue
		}
		if !filter.IncludeUncovered && !covered {
			continue
		}
		if ownerModule != "" && entry.OwnerModule != ownerModule {
			continue
		}
		if scopeDimension != "" && entry.ScopeDimension != scopeDimension {
			continue
		}
		if query != "" && !capabilityMatchesQuery(entry, query) {
			continue
		}
		out = append(out, AuthzCapabilityOption{
			AuthzCapability: entry,
			Label:           entry.ResourceLabel + " / " + entry.ActionLabel,
			Covered:         covered,
		})
	}
	return out
}

func ValidateAssignableTenantCapabilityKeys(keys []string, coveredKeys map[string]bool) error {
	seen := map[string]bool{}
	for _, raw := range keys {
		key := strings.TrimSpace(raw)
		if _, _, err := ParseAuthzCapabilityKey(key); err != nil {
			return err
		}
		if seen[key] {
			return fmt.Errorf("duplicate authz capability key %q", key)
		}
		seen[key] = true
		entry, ok := LookupAuthzCapability(key)
		if !ok {
			return fmt.Errorf("unknown authz capability key %q", key)
		}
		if entry.Status != CapabilityStatusEnabled {
			return fmt.Errorf("authz capability key %q is not enabled", key)
		}
		if !entry.Assignable {
			return fmt.Errorf("authz capability key %q is not assignable", key)
		}
		if entry.Surface != CapabilitySurfaceTenantAPI {
			return fmt.Errorf("authz capability key %q is not tenant api", key)
		}
		if !coveredKeys[key] {
			return fmt.Errorf("authz capability key %q has no tenant api coverage", key)
		}
	}
	return nil
}

func ValidateRegistry() error {
	seen := map[string]bool{}
	for _, entry := range registryEntries {
		if entry.Key != AuthzCapabilityKey(entry.Object, entry.Action) {
			return fmt.Errorf("authz registry entry %q does not match object/action", entry.Key)
		}
		if _, _, err := ParseAuthzCapabilityKey(entry.Key); err != nil {
			return err
		}
		if seen[entry.Key] {
			return fmt.Errorf("duplicate authz capability key %q", entry.Key)
		}
		seen[entry.Key] = true
		switch entry.Status {
		case CapabilityStatusEnabled, CapabilityStatusDisabled, CapabilityStatusDeprecated:
		default:
			return fmt.Errorf("authz capability key %q has invalid status %q", entry.Key, entry.Status)
		}
		switch entry.Surface {
		case CapabilitySurfaceTenantAPI, CapabilitySurfaceSuperadminRoute, CapabilitySurfaceInternalSystem:
		default:
			return fmt.Errorf("authz capability key %q has invalid surface %q", entry.Key, entry.Surface)
		}
		switch entry.ScopeDimension {
		case ScopeDimensionNone, ScopeDimensionOrganization:
		default:
			return fmt.Errorf("authz capability key %q has invalid scope dimension %q", entry.Key, entry.ScopeDimension)
		}
	}
	return nil
}

func capabilityMatchesQuery(entry AuthzCapability, query string) bool {
	fields := []string{
		entry.Key,
		entry.Object,
		entry.Action,
		entry.OwnerModule,
		entry.ResourceLabel,
		entry.ActionLabel,
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

var ErrEmptyPolicyGrant = errors.New("empty policy grant")
