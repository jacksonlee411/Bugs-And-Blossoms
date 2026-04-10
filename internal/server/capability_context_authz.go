package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

const (
	capabilityReasonContextRequired = "CAPABILITY_CONTEXT_REQUIRED"
	capabilityReasonContextMismatch = "CAPABILITY_CONTEXT_MISMATCH"
	actorScopeTenant                = "tenant"
	actorScopeSaaS                  = "saas"
)

type capabilityContextInput struct {
	CapabilityKey       string
	BusinessUnitOrgCode string
	AsOf                string
	RequireBusinessUnit bool
}

type capabilityContext struct {
	CapabilityKey       string
	BusinessUnitOrgCode string
	AsOf                string
	ActorScope          string
}

type capabilityContextError struct {
	Code    string
	Message string
}

type capabilityDynamicRelations struct {
	allowAll        bool
	managedOrgCodes map[string]struct{}
}

func resolveCapabilityContext(ctx context.Context, r *http.Request, input capabilityContextInput) (capabilityContext, *capabilityContextError) {
	capabilityKey := strings.ToLower(strings.TrimSpace(input.CapabilityKey))
	businessUnitOrgCode := strings.TrimSpace(input.BusinessUnitOrgCode)
	asOf := strings.TrimSpace(input.AsOf)

	if capabilityKey == "" || asOf == "" {
		return capabilityContext{}, &capabilityContextError{
			Code:    capabilityReasonContextRequired,
			Message: "capability context required",
		}
	}
	if input.RequireBusinessUnit && businessUnitOrgCode == "" {
		return capabilityContext{}, &capabilityContextError{
			Code:    capabilityReasonContextRequired,
			Message: "capability context required",
		}
	}

	requestScope := requestActorScope(r)
	authoritativeScope := resolveAuthoritativeActorScope(ctx)
	if requestScope != "" && requestScope != authoritativeScope {
		return capabilityContext{}, &capabilityContextError{
			Code:    capabilityReasonContextMismatch,
			Message: "capability context mismatch",
		}
	}

	return capabilityContext{
		CapabilityKey:       capabilityKey,
		BusinessUnitOrgCode: businessUnitOrgCode,
		AsOf:                asOf,
		ActorScope:          authoritativeScope,
	}, nil
}

func requestActorScope(r *http.Request) string {
	scope := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Actor-Scope")))
	if scope == "" {
		scope = strings.ToLower(strings.TrimSpace(r.Header.Get("x-actor-scope")))
	}
	return scope
}

func resolveAuthoritativeActorScope(ctx context.Context) string {
	p, ok := currentPrincipal(ctx)
	if !ok {
		return actorScopeTenant
	}
	switch strings.ToLower(strings.TrimSpace(p.RoleSlug)) {
	case authz.RoleSuperadmin:
		return actorScopeSaaS
	default:
		return actorScopeTenant
	}
}

func statusCodeForCapabilityContextError(code string) int {
	if strings.TrimSpace(code) == capabilityReasonContextRequired {
		return http.StatusBadRequest
	}
	return http.StatusForbidden
}

func preloadCapabilityDynamicRelations(ctx context.Context, businessUnitOrgCode string) capabilityDynamicRelations {
	businessUnitOrgCode = strings.TrimSpace(businessUnitOrgCode)
	relations := capabilityDynamicRelations{
		managedOrgCodes: make(map[string]struct{}, 1),
	}
	if businessUnitOrgCode != "" {
		relations.managedOrgCodes[businessUnitOrgCode] = struct{}{}
	}
	p, ok := currentPrincipal(ctx)
	if !ok {
		return relations
	}
	switch strings.ToLower(strings.TrimSpace(p.RoleSlug)) {
	case authz.RoleSuperadmin, authz.RoleTenantAdmin:
		relations.allowAll = true
	}
	return relations
}

func (r capabilityDynamicRelations) actorManages(targetOrgCode string, asOf string) bool {
	targetOrgCode = strings.TrimSpace(targetOrgCode)
	asOf = strings.TrimSpace(asOf)
	if targetOrgCode == "" || asOf == "" {
		return false
	}
	if r.allowAll {
		return true
	}
	_, ok := r.managedOrgCodes[targetOrgCode]
	return ok
}
