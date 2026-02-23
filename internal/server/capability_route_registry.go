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
		Method:        "GET",
		Path:          "/org/api/setid-explain",
		RouteClass:    "internal_api",
		Action:        authz.ActionRead,
		CapabilityKey: "staffing.assignment_create.field_policy",
		OwnerModule:   "staffing",
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
}

var capabilityRouteBindingByKey = buildCapabilityRouteBindingIndex(capabilityRouteBindings)

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
	return binding, ok
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
