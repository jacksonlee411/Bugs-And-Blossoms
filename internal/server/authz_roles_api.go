package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type authzRolesResponse struct {
	Roles []authzRoleDefinitionDTO `json:"roles"`
}

type authzRoleResponse struct {
	Role authzRoleDefinitionDTO `json:"role"`
}

type authzRoleDefinitionDTO struct {
	RoleSlug            string   `json:"role_slug"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	SystemManaged       bool     `json:"system_managed"`
	Revision            int64    `json:"revision"`
	AuthzCapabilityKeys []string `json:"authz_capability_keys"`
	RequiresOrgScope    bool     `json:"requires_org_scope"`
}

type saveAuthzRoleDefinitionRequest struct {
	RoleSlug            string   `json:"role_slug"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Revision            int64    `json:"revision"`
	AuthzCapabilityKeys []string `json:"authz_capability_keys"`
}

type principalAuthzAssignmentResponse struct {
	PrincipalID string                       `json:"principal_id"`
	Roles       []principalRoleAssignmentDTO `json:"roles"`
	OrgScopes   []principalOrgScopeDTO       `json:"org_scopes"`
	Revision    int64                        `json:"revision"`
}

type principalAssignmentCandidatesResponse struct {
	Principals []principalAssignmentCandidateDTO `json:"principals"`
}

type principalAssignmentCandidateDTO struct {
	PrincipalID string `json:"principal_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
}

type principalRoleAssignmentDTO struct {
	RoleSlug         string `json:"role_slug"`
	DisplayName      string `json:"display_name"`
	Description      string `json:"description"`
	RequiresOrgScope bool   `json:"requires_org_scope"`
}

type principalOrgScopeDTO struct {
	OrgNodeKey         string `json:"org_node_key"`
	OrgCode            string `json:"org_code,omitempty"`
	OrgName            string `json:"org_name,omitempty"`
	IncludeDescendants bool   `json:"include_descendants"`
}

type replacePrincipalAssignmentRequest struct {
	Roles     []principalAssignmentRoleRequest `json:"roles"`
	OrgScopes []principalOrgScopeDTO           `json:"org_scopes"`
	Revision  int64                            `json:"revision"`
}

type principalAssignmentRoleRequest struct {
	RoleSlug string `json:"role_slug"`
}

func handleAuthzRolesAPI(w http.ResponseWriter, r *http.Request, store authzRuntimeStore) {
	switch r.Method {
	case http.MethodGet:
		handleAuthzRolesListAPI(w, r, store)
	case http.MethodPost:
		handleAuthzRolesCreateAPI(w, r, store)
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func handleAuthzRoleAPI(w http.ResponseWriter, r *http.Request, store authzRuntimeStore) {
	switch r.Method {
	case http.MethodGet:
		handleAuthzRoleGetAPI(w, r, store)
	case http.MethodPut:
		handleAuthzRoleUpdateAPI(w, r, store)
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func handleAuthzRolesListAPI(w http.ResponseWriter, r *http.Request, store authzRuntimeStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if store == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
		return
	}
	roles, err := store.ListRoleDefinitions(r.Context(), tenant.ID)
	if err != nil {
		writeAuthzRoleStoreError(w, r, err)
		return
	}
	out := make([]authzRoleDefinitionDTO, 0, len(roles))
	for _, role := range roles {
		out = append(out, authzRoleDefinitionDTOFromModel(role))
	}
	writeJSON(w, http.StatusOK, authzRolesResponse{Roles: out})
}

func handleAuthzRoleGetAPI(w http.ResponseWriter, r *http.Request, store authzRuntimeStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if store == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
		return
	}
	roleSlug := authzRoleSlugFromPath(r.URL.Path)
	role, found, err := store.GetRoleDefinition(r.Context(), tenant.ID, roleSlug)
	if err != nil {
		writeAuthzRoleStoreError(w, r, err)
		return
	}
	if !found {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "role_not_found", "role not found")
		return
	}
	writeJSON(w, http.StatusOK, authzRoleResponse{Role: authzRoleDefinitionDTOFromModel(role)})
}

func handleAuthzRolesCreateAPI(w http.ResponseWriter, r *http.Request, store authzRuntimeStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if store == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
		return
	}
	var req saveAuthzRoleDefinitionRequest
	if !decodeStrictJSON(w, r, &req, "invalid_role_payload") {
		return
	}
	role, err := store.CreateRoleDefinition(r.Context(), tenant.ID, saveAuthzRoleDefinitionInput{
		RoleSlug:            req.RoleSlug,
		Name:                req.Name,
		Description:         req.Description,
		AuthzCapabilityKeys: req.AuthzCapabilityKeys,
	})
	if err != nil {
		writeAuthzRoleStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, authzRoleResponse{Role: authzRoleDefinitionDTOFromModel(role)})
}

func handleAuthzRoleUpdateAPI(w http.ResponseWriter, r *http.Request, store authzRuntimeStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if store == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
		return
	}
	var req saveAuthzRoleDefinitionRequest
	if !decodeStrictJSON(w, r, &req, "invalid_role_payload") {
		return
	}
	role, err := store.UpdateRoleDefinition(r.Context(), tenant.ID, authzRoleSlugFromPath(r.URL.Path), saveAuthzRoleDefinitionInput{
		Name:                req.Name,
		Description:         req.Description,
		Revision:            req.Revision,
		AuthzCapabilityKeys: req.AuthzCapabilityKeys,
	})
	if err != nil {
		writeAuthzRoleStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authzRoleResponse{Role: authzRoleDefinitionDTOFromModel(role)})
}

func handlePrincipalAuthzAssignmentGetAPI(w http.ResponseWriter, r *http.Request, store authzRuntimeStore, principals principalStore, orgResolver OrgUnitCodeResolver) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if store == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
		return
	}
	principalID := strings.TrimSpace(r.URL.Query().Get("principal_id"))
	if principalID == "" {
		handlePrincipalAuthzAssignmentCandidatesAPI(w, r, principals, tenant.ID)
		return
	}
	assignment, found, err := store.GetPrincipalAssignment(r.Context(), tenant.ID, principalID)
	if err != nil {
		writePrincipalAssignmentStoreError(w, r, err)
		return
	}
	if !found {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "principal_missing", "principal missing")
		return
	}
	response, err := principalAuthzAssignmentDTOFromModelWithOrgCodes(r.Context(), tenant.ID, assignment, orgResolver)
	if err != nil {
		writePrincipalAssignmentOrgResolveError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func handlePrincipalAuthzAssignmentCandidatesAPI(w http.ResponseWriter, r *http.Request, principals principalStore, tenantID string) {
	if principals == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "principal_error", "principal error")
		return
	}
	items, err := principals.ListActive(r.Context(), tenantID)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "principal_lookup_error", "principal lookup error")
		return
	}
	out := make([]principalAssignmentCandidateDTO, 0, len(items))
	for _, item := range items {
		out = append(out, principalAssignmentCandidateDTO{
			PrincipalID: item.ID,
			Email:       item.Email,
			DisplayName: strings.TrimSpace(item.DisplayName),
		})
	}
	writeJSON(w, http.StatusOK, principalAssignmentCandidatesResponse{Principals: out})
}

func handlePrincipalAuthzAssignmentPutAPI(w http.ResponseWriter, r *http.Request, store authzRuntimeStore, orgResolver OrgUnitCodeResolver, scopeRuntime authzRuntimeStore) {
	if r.Method != http.MethodPut {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if store == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
		return
	}
	var req replacePrincipalAssignmentRequest
	if !decodeStrictJSON(w, r, &req, "invalid_user_assignment") {
		return
	}
	roleSlugs := make([]string, 0, len(req.Roles))
	for _, role := range req.Roles {
		roleSlugs = append(roleSlugs, role.RoleSlug)
	}
	orgScopes, err := resolvePrincipalAssignmentOrgScopes(r.Context(), tenant.ID, req.OrgScopes, orgResolver)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) || errors.Is(err, orgunitpkg.ErrOrgNodeKeyNotFound) {
			writeOrgUnitScopeError(w, r, errAuthzScopeForbidden)
			return
		}
		writePrincipalAssignmentOrgResolveError(w, r, err)
		return
	}
	if len(orgScopes) > 0 {
		if err := ensurePrincipalAssignmentOrgScopesAllowed(r.Context(), tenant.ID, orgScopes, orgResolver, scopeRuntime); err != nil {
			writeOrgUnitScopeError(w, r, err)
			return
		}
	}
	assignment, err := store.ReplacePrincipalAssignment(r.Context(), tenant.ID, principalIDFromAssignmentPath(r.URL.Path), replacePrincipalAssignmentInput{
		Roles:     roleSlugs,
		OrgScopes: orgScopes,
		Revision:  req.Revision,
	})
	if err != nil {
		writePrincipalAssignmentStoreError(w, r, err)
		return
	}
	response, err := principalAuthzAssignmentDTOFromModelWithOrgCodes(r.Context(), tenant.ID, assignment, orgResolver)
	if err != nil {
		writePrincipalAssignmentOrgResolveError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, dest any, code string) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, code, code)
		return false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, code, code)
		return false
	}
	return true
}

func authzRoleDefinitionDTOFromModel(role authzRoleDefinition) authzRoleDefinitionDTO {
	return authzRoleDefinitionDTO{
		RoleSlug:            role.RoleSlug,
		Name:                role.Name,
		Description:         role.Description,
		SystemManaged:       role.SystemManaged,
		Revision:            role.Revision,
		AuthzCapabilityKeys: append([]string(nil), role.AuthzCapabilityKeys...),
		RequiresOrgScope:    role.RequiresOrgScope,
	}
}

func principalAuthzAssignmentDTOFromModel(assignment principalAuthzAssignment) principalAuthzAssignmentResponse {
	roles := make([]principalRoleAssignmentDTO, 0, len(assignment.Roles))
	for _, role := range assignment.Roles {
		roles = append(roles, principalRoleAssignmentDTO{
			RoleSlug:         role.RoleSlug,
			DisplayName:      role.DisplayName,
			Description:      role.Description,
			RequiresOrgScope: role.RequiresOrgScope,
		})
	}
	scopes := make([]principalOrgScopeDTO, 0, len(assignment.OrgScopes))
	for _, scope := range assignment.OrgScopes {
		scopes = append(scopes, principalOrgScopeDTO{
			OrgNodeKey:         scope.OrgNodeKey,
			IncludeDescendants: scope.IncludeDescendants,
		})
	}
	return principalAuthzAssignmentResponse{
		PrincipalID: assignment.PrincipalID,
		Roles:       roles,
		OrgScopes:   scopes,
		Revision:    assignment.Revision,
	}
}

func principalAuthzAssignmentDTOFromModelWithOrgCodes(ctx context.Context, tenantID string, assignment principalAuthzAssignment, resolver OrgUnitCodeResolver) (principalAuthzAssignmentResponse, error) {
	response := principalAuthzAssignmentDTOFromModel(assignment)
	if resolver == nil || len(response.OrgScopes) == 0 {
		return response, nil
	}
	orgNodeKeys := make([]string, 0, len(response.OrgScopes))
	for _, scope := range response.OrgScopes {
		orgNodeKeys = append(orgNodeKeys, scope.OrgNodeKey)
	}
	codes, err := resolver.ResolveOrgCodesByNodeKeys(ctx, tenantID, orgNodeKeys)
	if err != nil {
		return principalAuthzAssignmentResponse{}, err
	}
	for i := range response.OrgScopes {
		orgCode := strings.TrimSpace(codes[response.OrgScopes[i].OrgNodeKey])
		if orgCode == "" {
			return principalAuthzAssignmentResponse{}, orgunitpkg.ErrOrgNodeKeyNotFound
		}
		response.OrgScopes[i].OrgCode = orgCode
	}
	return response, nil
}

func resolvePrincipalAssignmentOrgScopes(ctx context.Context, tenantID string, values []principalOrgScopeDTO, resolver OrgUnitCodeResolver) ([]principalOrgScope, error) {
	scopes := make([]principalOrgScope, 0, len(values))
	for _, value := range values {
		orgNodeKey := strings.TrimSpace(value.OrgNodeKey)
		orgCode := strings.TrimSpace(value.OrgCode)
		if orgCode != "" {
			if resolver == nil {
				return nil, errAuthzRuntimeUnavailable
			}
			resolvedNodeKey, err := resolver.ResolveOrgNodeKeyByCode(ctx, tenantID, orgCode)
			if err != nil {
				return nil, err
			}
			if orgNodeKey != "" {
				normalizedNodeKey, err := orgunitpkg.NormalizeOrgNodeKey(orgNodeKey)
				if err != nil || normalizedNodeKey != resolvedNodeKey {
					return nil, errInvalidAssignment
				}
			}
			orgNodeKey = resolvedNodeKey
		}
		scopes = append(scopes, principalOrgScope{
			OrgNodeKey:         orgNodeKey,
			IncludeDescendants: value.IncludeDescendants,
		})
	}
	return normalizePrincipalOrgScopes(scopes)
}

func ensurePrincipalAssignmentOrgScopesAllowed(ctx context.Context, tenantID string, scopes []principalOrgScope, orgResolver OrgUnitCodeResolver, runtime authzRuntimeStore) error {
	if runtime == nil {
		return nil
	}
	if len(scopes) == 0 {
		return nil
	}
	principal, ok := currentPrincipal(ctx)
	if !ok || strings.TrimSpace(principal.ID) == "" {
		return errAuthzPrincipalMissing
	}
	actorScopes, err := runtime.OrgScopesForPrincipal(ctx, tenantID, principal.ID, authz.AuthzCapabilityKey(authz.ObjectOrgUnitOrgUnits, authz.ActionRead))
	if err != nil {
		return err
	}
	orgStore, ok := orgResolver.(OrgUnitStore)
	if !ok || orgStore == nil {
		return errAuthzRuntimeUnavailable
	}
	asOf, err := orgUnitScopeCheckAsOf(ctx, orgStore, tenantID, "")
	if err != nil {
		return err
	}
	readSvc := orgunitservices.NewOrgUnitReadService(orgUnitReadStoreAdapter{store: orgStore})
	scopeFilter := orgUnitReadScopeFilterFromPrincipalScopes(actorScopes)
	for _, scope := range scopes {
		targetOrgNodeKey := strings.TrimSpace(scope.OrgNodeKey)
		if targetOrgNodeKey == "" {
			return errAuthzScopeForbidden
		}
		resolved, err := readSvc.Resolve(ctx, orgunitservices.OrgUnitResolveRequest{
			TenantID:        tenantID,
			AsOf:            asOf,
			ScopeFilter:     scopeFilter,
			OrgNodeKeys:     []string{targetOrgNodeKey},
			IncludeDisabled: true,
			Caller:          "authz.assignment.org_scope",
		})
		if err != nil {
			if errors.Is(err, orgunitservices.ErrOrgUnitReadNotFound) {
				return errAuthzScopeForbidden
			}
			return err
		}
		if len(resolved) == 0 {
			return errAuthzScopeForbidden
		}
	}
	return nil
}

func writeAuthzRoleStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errInvalidRolePayload):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_role_payload", "invalid role payload")
	case errors.Is(err, errInvalidRoleDefinition):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_role_definition", "invalid role definition")
	case errors.Is(err, errRoleNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "role_not_found", "role not found")
	case errors.Is(err, errRoleSlugConflict):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "role_slug_conflict", "role slug conflict")
	case errors.Is(err, errStaleRevision):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "stale_revision", "stale revision")
	case errors.Is(err, errSystemRoleReadonly):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "system_role_readonly", "system role readonly")
	case errors.Is(err, errAuthzRuntimeUnavailable):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
	}
}

func writePrincipalAssignmentOrgResolveError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
	case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound), errors.Is(err, orgunitpkg.ErrOrgNodeKeyNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_code_not_found", "org_code not found")
	case errors.Is(err, errInvalidAssignment):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_user_assignment", "invalid user assignment")
	case errors.Is(err, errAuthzRuntimeUnavailable):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
	}
}

func writePrincipalAssignmentStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errInvalidAssignment):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_user_assignment", "invalid user assignment")
	case errors.Is(err, errRoleNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "role_not_found", "role not found")
	case errors.Is(err, errAuthzPrincipalMissing):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "principal_missing", "principal missing")
	case errors.Is(err, errAuthzOrgScopeRequired):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "authz_org_scope_required", "authz org scope required")
	case errors.Is(err, errStaleRevision):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "stale_revision", "stale revision")
	case errors.Is(err, errAuthzRuntimeUnavailable):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
	}
}

func authzRoleSlugFromPath(path string) string {
	const prefix = "/iam/api/authz/roles/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(path, prefix))
}

func principalIDFromAssignmentPath(path string) string {
	const prefix = "/iam/api/authz/user-assignments/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(path, prefix))
}
