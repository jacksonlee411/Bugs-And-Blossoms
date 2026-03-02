package server

import (
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type capabilityRouteBinding struct {
	Method        string `json:"method"`
	Path          string `json:"path"`
	RouteClass    string `json:"route_class"`
	Action        string `json:"action"`
	CapabilityKey string `json:"capability_key"`
	OwnerModule   string `json:"owner_module"`
	Status        string `json:"status"`
}

const routeCapabilityStatusActive = "active"

type capabilityDefinition struct {
	CapabilityKey     string `json:"capability_key"`
	FunctionalAreaKey string `json:"functional_area_key"`
	CapabilityType    string `json:"capability_type"`
	OwnerModule       string `json:"owner_module"`
	Status            string `json:"status"`
	ActivationState   string `json:"activation_state"`
	CurrentPolicy     string `json:"current_policy_version"`
}

var capabilityDefinitions = []capabilityDefinition{
	{
		CapabilityKey:     "staffing.assignment_create.field_policy",
		FunctionalAreaKey: "staffing",
		CapabilityType:    "process_capability",
		OwnerModule:       "staffing",
		Status:            routeCapabilityStatusActive,
		ActivationState:   "active",
		CurrentPolicy:     capabilityPolicyVersionBaseline,
	},
	{
		CapabilityKey:     "org.policy_activation.manage",
		FunctionalAreaKey: "org_foundation",
		CapabilityType:    "process_capability",
		OwnerModule:       "orgunit",
		Status:            routeCapabilityStatusActive,
		ActivationState:   "active",
		CurrentPolicy:     capabilityPolicyVersionBaseline,
	},
	{
		CapabilityKey:     "org.orgunit_write.field_policy",
		FunctionalAreaKey: "org_foundation",
		CapabilityType:    "process_capability",
		OwnerModule:       "orgunit",
		Status:            routeCapabilityStatusActive,
		ActivationState:   "active",
		CurrentPolicy:     capabilityPolicyVersionBaseline,
	},
	{
		CapabilityKey:     "org.orgunit_create.field_policy",
		FunctionalAreaKey: "org_foundation",
		CapabilityType:    "process_capability",
		OwnerModule:       "orgunit",
		Status:            routeCapabilityStatusActive,
		ActivationState:   "active",
		CurrentPolicy:     capabilityPolicyVersionBaseline,
	},
	{
		CapabilityKey:     "org.orgunit_add_version.field_policy",
		FunctionalAreaKey: "org_foundation",
		CapabilityType:    "process_capability",
		OwnerModule:       "orgunit",
		Status:            routeCapabilityStatusActive,
		ActivationState:   "active",
		CurrentPolicy:     capabilityPolicyVersionBaseline,
	},
	{
		CapabilityKey:     "org.orgunit_insert_version.field_policy",
		FunctionalAreaKey: "org_foundation",
		CapabilityType:    "process_capability",
		OwnerModule:       "orgunit",
		Status:            routeCapabilityStatusActive,
		ActivationState:   "active",
		CurrentPolicy:     capabilityPolicyVersionBaseline,
	},
	{
		CapabilityKey:     "org.orgunit_correct.field_policy",
		FunctionalAreaKey: "org_foundation",
		CapabilityType:    "process_capability",
		OwnerModule:       "orgunit",
		Status:            routeCapabilityStatusActive,
		ActivationState:   "active",
		CurrentPolicy:     capabilityPolicyVersionBaseline,
	},
	{
		CapabilityKey:     "org.assistant_conversation.manage",
		FunctionalAreaKey: "org_foundation",
		CapabilityType:    "process_capability",
		OwnerModule:       "orgunit",
		Status:            routeCapabilityStatusActive,
		ActivationState:   "active",
		CurrentPolicy:     capabilityPolicyVersionBaseline,
	},
}

var capabilityDefinitionByKey = buildCapabilityDefinitionIndex(capabilityDefinitions)

var capabilityRouteBindings = []capabilityRouteBinding{
	{
		Method:        "GET",
		Path:          "/org/api/setid-strategy-registry",
		RouteClass:    "internal_api",
		Action:        authz.ActionRead,
		CapabilityKey: "staffing.assignment_create.field_policy",
		OwnerModule:   "staffing",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "POST",
		Path:          "/org/api/setid-strategy-registry",
		RouteClass:    "internal_api",
		Action:        authz.ActionAdmin,
		CapabilityKey: "staffing.assignment_create.field_policy",
		OwnerModule:   "staffing",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "POST",
		Path:          "/org/api/setid-strategy-registry:disable",
		RouteClass:    "internal_api",
		Action:        authz.ActionAdmin,
		CapabilityKey: "staffing.assignment_create.field_policy",
		OwnerModule:   "staffing",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "GET",
		Path:          "/org/api/setid-explain",
		RouteClass:    "internal_api",
		Action:        authz.ActionRead,
		CapabilityKey: "staffing.assignment_create.field_policy",
		OwnerModule:   "staffing",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "GET",
		Path:          "/org/api/org-units/create-field-decisions",
		RouteClass:    "internal_api",
		Action:        authz.ActionRead,
		CapabilityKey: "org.orgunit_create.field_policy",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "POST",
		Path:          "/internal/rules/evaluate",
		RouteClass:    "internal_api",
		Action:        authz.ActionAdmin,
		CapabilityKey: "staffing.assignment_create.field_policy",
		OwnerModule:   "staffing",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "GET",
		Path:          "/internal/capabilities/catalog",
		RouteClass:    "internal_api",
		Action:        authz.ActionRead,
		CapabilityKey: "org.policy_activation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "GET",
		Path:          "/internal/capabilities/catalog:by-intent",
		RouteClass:    "internal_api",
		Action:        authz.ActionRead,
		CapabilityKey: "org.policy_activation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "GET",
		Path:          "/internal/policies/state",
		RouteClass:    "internal_api",
		Action:        authz.ActionRead,
		CapabilityKey: "org.policy_activation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "POST",
		Path:          "/internal/policies/draft",
		RouteClass:    "internal_api",
		Action:        authz.ActionAdmin,
		CapabilityKey: "org.policy_activation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "POST",
		Path:          "/internal/policies/activate",
		RouteClass:    "internal_api",
		Action:        authz.ActionAdmin,
		CapabilityKey: "org.policy_activation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "POST",
		Path:          "/internal/policies/rollback",
		RouteClass:    "internal_api",
		Action:        authz.ActionAdmin,
		CapabilityKey: "org.policy_activation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "GET",
		Path:          "/internal/functional-areas/state",
		RouteClass:    "internal_api",
		Action:        authz.ActionRead,
		CapabilityKey: "org.policy_activation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "POST",
		Path:          "/internal/functional-areas/switch",
		RouteClass:    "internal_api",
		Action:        authz.ActionAdmin,
		CapabilityKey: "org.policy_activation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "POST",
		Path:          "/internal/assistant/conversations",
		RouteClass:    "internal_api",
		Action:        authz.ActionAdmin,
		CapabilityKey: "org.assistant_conversation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "GET",
		Path:          "/internal/assistant/conversations/{conversation_id}",
		RouteClass:    "internal_api",
		Action:        authz.ActionRead,
		CapabilityKey: "org.assistant_conversation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "POST",
		Path:          "/internal/assistant/conversations/{conversation_id}/turns",
		RouteClass:    "internal_api",
		Action:        authz.ActionAdmin,
		CapabilityKey: "org.assistant_conversation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
	{
		Method:        "POST",
		Path:          "/internal/assistant/conversations/{conversation_id}/turns/{turn_action}",
		RouteClass:    "internal_api",
		Action:        authz.ActionAdmin,
		CapabilityKey: "org.assistant_conversation.manage",
		OwnerModule:   "orgunit",
		Status:        routeCapabilityStatusActive,
	},
}

var capabilityRouteBindingByKey = buildCapabilityRouteBindingIndex(capabilityRouteBindings)

func buildCapabilityDefinitionIndex(definitions []capabilityDefinition) map[string]capabilityDefinition {
	index := make(map[string]capabilityDefinition, len(definitions))
	for _, definition := range definitions {
		key := strings.ToLower(strings.TrimSpace(definition.CapabilityKey))
		index[key] = definition
	}
	return index
}

func capabilityDefinitionForKey(capabilityKey string) (capabilityDefinition, bool) {
	definition, ok := capabilityDefinitionByKey[strings.ToLower(strings.TrimSpace(capabilityKey))]
	return definition, ok
}

func buildCapabilityRouteBindingIndex(bindings []capabilityRouteBinding) map[string]capabilityRouteBinding {
	index := make(map[string]capabilityRouteBinding, len(bindings))
	for _, binding := range bindings {
		index[capabilityRouteBindingKey(binding.Method, binding.Path)] = binding
	}
	return index
}

func capabilityRouteBindingKey(method string, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(path)
}

func capabilityRouteBindingForRoute(method string, path string) (capabilityRouteBinding, bool) {
	binding, ok := capabilityRouteBindingByKey[capabilityRouteBindingKey(method, path)]
	if ok {
		return binding, true
	}
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	for _, candidate := range capabilityRouteBindings {
		if strings.ToUpper(strings.TrimSpace(candidate.Method)) != normalizedMethod {
			continue
		}
		if pathMatchRouteTemplate(path, candidate.Path) {
			return candidate, true
		}
	}
	return capabilityRouteBinding{}, false
}

func capabilityAuthzRequirementForRoute(method string, path string) (object string, action string, ok bool) {
	binding, ok := capabilityRouteBindingForRoute(method, path)
	if !ok {
		return "", "", false
	}
	return capabilityAuthzRequirementForBinding(binding)
}

func capabilityAuthzRequirementForBinding(binding capabilityRouteBinding) (object string, action string, ok bool) {
	switch binding.Action {
	case authz.ActionRead, authz.ActionAdmin:
		return authz.ObjectOrgSetIDCapability, binding.Action, true
	default:
		return "", "", false
	}
}
