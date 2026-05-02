package server

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type authzRuntimeStore interface {
	AuthorizePrincipal(ctx context.Context, tenantID string, principalID string, object string, action string) (bool, error)
	CapabilitiesForPrincipal(ctx context.Context, tenantID string, principalID string) ([]string, error)
	OrgScopesForPrincipal(ctx context.Context, tenantID string, principalID string, capabilityKey string) ([]principalOrgScope, error)
	ListRoleDefinitions(ctx context.Context, tenantID string) ([]authzRoleDefinition, error)
	GetRoleDefinition(ctx context.Context, tenantID string, roleSlug string) (authzRoleDefinition, bool, error)
	CreateRoleDefinition(ctx context.Context, tenantID string, input saveAuthzRoleDefinitionInput) (authzRoleDefinition, error)
	UpdateRoleDefinition(ctx context.Context, tenantID string, roleSlug string, input saveAuthzRoleDefinitionInput) (authzRoleDefinition, error)
	GetPrincipalAssignment(ctx context.Context, tenantID string, principalID string) (principalAuthzAssignment, bool, error)
	ReplacePrincipalAssignment(ctx context.Context, tenantID string, principalID string, input replacePrincipalAssignmentInput) (principalAuthzAssignment, error)
}

type authzRuntimeRoleSeeder interface {
	EnsurePrincipalRoleAssignment(ctx context.Context, tenantID string, principalID string, roleSlug string) error
}

type authzRoleDefinition struct {
	RoleSlug            string
	Name                string
	Description         string
	SystemManaged       bool
	Revision            int64
	AuthzCapabilityKeys []string
	RequiresOrgScope    bool
}

type saveAuthzRoleDefinitionInput struct {
	RoleSlug            string
	Name                string
	Description         string
	Revision            int64
	AuthzCapabilityKeys []string
}

type principalRoleAssignment struct {
	RoleSlug         string
	DisplayName      string
	Description      string
	RequiresOrgScope bool
}

type principalOrgScope struct {
	OrgNodeKey         string
	IncludeDescendants bool
}

type principalAuthzAssignment struct {
	PrincipalID string
	Roles       []principalRoleAssignment
	OrgScopes   []principalOrgScope
	Revision    int64
}

type replacePrincipalAssignmentInput struct {
	Roles     []string
	OrgScopes []principalOrgScope
	Revision  int64
}

var (
	errAuthzRuntimeUnavailable = errors.New("authz_runtime_unavailable")
	errAuthzPrincipalMissing   = errors.New("authz_principal_missing")
	errRoleNotFound            = errors.New("role_not_found")
	errRoleSlugConflict        = errors.New("role_slug_conflict")
	errStaleRevision           = errors.New("stale_revision")
	errSystemRoleReadonly      = errors.New("system_role_readonly")
	errInvalidRoleDefinition   = errors.New("invalid_role_definition")
	errInvalidRolePayload      = errors.New("invalid_role_payload")
	errInvalidAssignment       = errors.New("invalid_user_assignment")
	errAuthzOrgScopeRequired   = errors.New("authz_org_scope_required")
	errAuthzScopeForbidden     = errors.New("authz_scope_forbidden")
)

var roleSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

var builtinIdentityRoleSlugs = map[string]bool{
	authz.RoleTenantAdmin:  true,
	authz.RoleTenantViewer: true,
}

type pgAuthzRuntimeStore struct {
	pool *pgxpool.Pool
}

func newAuthzRuntimeStore(pool *pgxpool.Pool) authzRuntimeStore {
	if pool == nil {
		return newMemoryAuthzRuntimeStore()
	}
	return &pgAuthzRuntimeStore{pool: pool}
}

func normalizeRoleSlug(roleSlug string) (string, error) {
	roleSlug = strings.ToLower(strings.TrimSpace(roleSlug))
	if strings.HasPrefix(roleSlug, "role:") {
		return "", errInvalidRoleDefinition
	}
	if roleSlug == authz.RoleAnonymous || roleSlug == authz.RoleSuperadmin {
		return "", errInvalidRoleDefinition
	}
	if !roleSlugPattern.MatchString(roleSlug) {
		return "", errInvalidRoleDefinition
	}
	return roleSlug, nil
}

func normalizeRoleName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errInvalidRoleDefinition
	}
	return name, nil
}

func normalizeRoleDescription(description string) string {
	return strings.TrimSpace(description)
}

func normalizeCapabilityKeys(keys []string) ([]string, error) {
	if len(keys) == 0 {
		return nil, errInvalidRoleDefinition
	}
	out := make([]string, 0, len(keys))
	for _, raw := range keys {
		key := strings.ToLower(strings.TrimSpace(raw))
		out = append(out, key)
	}
	if err := authz.ValidateAssignableTenantCapabilityKeys(out, TenantAPICoveredCapabilityKeys()); err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidRoleDefinition, err)
	}
	sort.Strings(out)
	return out, nil
}

func roleRequiresOrgScope(keys []string) bool {
	for _, key := range keys {
		entry, ok := authz.LookupAuthzCapability(key)
		if ok && entry.ScopeDimension == authz.ScopeDimensionOrganization {
			return true
		}
	}
	return false
}

func builtinTenantAdminCapabilityKeys() []string {
	return []string{
		"iam.authz:read",
		"iam.authz:admin",
		"iam.dicts:read",
		"iam.dicts:admin",
		"iam.dict_release:admin",
		"cubebox.conversations:read",
		"cubebox.conversations:use",
		"cubebox.model_provider:update",
		"cubebox.model_credential:read",
		"cubebox.model_credential:rotate",
		"cubebox.model_credential:deactivate",
		"cubebox.model_selection:select",
		"cubebox.model_selection:verify",
		"orgunit.orgunits:read",
		"orgunit.orgunits:admin",
	}
}

func builtinTenantViewerCapabilityKeys() []string {
	return []string{
		"iam.dicts:read",
		"cubebox.conversations:read",
		"cubebox.conversations:use",
		"orgunit.orgunits:read",
	}
}

func (s *pgAuthzRuntimeStore) beginTenantTx(ctx context.Context, tenantID string) (pgx.Tx, error) {
	if s == nil || s.pool == nil {
		return nil, errAuthzRuntimeUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

func (s *pgAuthzRuntimeStore) AuthorizePrincipal(ctx context.Context, tenantID string, principalID string, object string, action string) (bool, error) {
	key := authz.AuthzCapabilityKey(object, action)
	keys, err := s.CapabilitiesForPrincipal(ctx, tenantID, principalID)
	if err != nil {
		return false, err
	}
	for _, got := range keys {
		if got == key {
			return true, nil
		}
	}
	return false, nil
}

func (s *pgAuthzRuntimeStore) CapabilitiesForPrincipal(ctx context.Context, tenantID string, principalID string) ([]string, error) {
	tx, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	keys, err := capabilityKeysForPrincipalTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *pgAuthzRuntimeStore) EnsurePrincipalRoleAssignment(ctx context.Context, tenantID string, principalID string, roleSlug string) error {
	roleSlug, err := normalizeRoleSlug(roleSlug)
	if err != nil {
		return errInvalidAssignment
	}
	if !builtinIdentityRoleSlugs[roleSlug] {
		return errInvalidAssignment
	}
	tx, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT iam.seed_builtin_authz_roles($1::uuid)`, tenantID); err != nil {
		return err
	}
	exists, err := principalExistsTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return err
	}
	if !exists {
		return errAuthzPrincipalMissing
	}
	assignedRoleCount, err := principalRoleAssignmentCountTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return err
	}
	if assignedRoleCount > 0 {
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO iam.principal_role_assignments (tenant_uuid, principal_id, role_slug)
		VALUES ($1::uuid, $2::uuid, $3)
		ON CONFLICT DO NOTHING
		`, tenantID, principalID, roleSlug); err != nil {
		return err
	}
	if _, err := ensurePrincipalAssignmentRevisionTx(ctx, tx, tenantID, principalID); err != nil {
		return err
	}
	if err := ensureRootOrgScopeForUnscopedPrincipalTx(ctx, tx, tenantID, principalID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func principalRoleAssignmentCountTx(ctx context.Context, tx pgx.Tx, tenantID string, principalID string) (int, error) {
	var count int
	err := tx.QueryRow(ctx, `
SELECT count(*)
FROM iam.principal_role_assignments
WHERE tenant_uuid = $1::uuid
  AND principal_id = $2::uuid
`, tenantID, principalID).Scan(&count)
	return count, err
}

func capabilityKeysForPrincipalTx(ctx context.Context, tx pgx.Tx, tenantID string, principalID string) ([]string, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT rac.authz_capability_key
FROM iam.principal_role_assignments pra
JOIN iam.role_definitions rd
  ON rd.tenant_uuid = pra.tenant_uuid
 AND rd.role_slug = pra.role_slug
JOIN iam.role_authz_capabilities rac
  ON rac.role_id = rd.id
WHERE pra.tenant_uuid = $1::uuid
  AND pra.principal_id = $2::uuid
ORDER BY rac.authz_capability_key ASC
`, tenantID, principalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errAuthzPrincipalMissing
	}
	return out, nil
}

func (s *pgAuthzRuntimeStore) OrgScopesForPrincipal(ctx context.Context, tenantID string, principalID string, capabilityKey string) ([]principalOrgScope, error) {
	entry, ok := authz.LookupAuthzCapability(strings.TrimSpace(capabilityKey))
	if !ok {
		return nil, errInvalidRoleDefinition
	}
	if entry.ScopeDimension != authz.ScopeDimensionOrganization {
		return nil, nil
	}

	tx, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	scopes, err := orgScopesForPrincipalTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return nil, err
	}
	if len(scopes) == 0 {
		revision, err := ensurePrincipalAssignmentRevisionTx(ctx, tx, tenantID, principalID)
		if err != nil {
			return nil, err
		}
		if revision == 1 {
			if err := ensureRootOrgScopeForUnscopedPrincipalTx(ctx, tx, tenantID, principalID); err != nil {
				return nil, err
			}
			scopes, err = orgScopesForPrincipalTx(ctx, tx, tenantID, principalID)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(scopes) == 0 {
		return nil, errAuthzOrgScopeRequired
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return scopes, nil
}

func orgScopesForPrincipalTx(ctx context.Context, tx pgx.Tx, tenantID string, principalID string) ([]principalOrgScope, error) {
	rows, err := tx.Query(ctx, `
SELECT org_node_key, include_descendants
FROM iam.principal_org_scope_bindings
WHERE tenant_uuid = $1::uuid
  AND principal_id = $2::uuid
ORDER BY org_node_key ASC
`, tenantID, principalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []principalOrgScope
	for rows.Next() {
		var item principalOrgScope
		if err := rows.Scan(&item.OrgNodeKey, &item.IncludeDescendants); err != nil {
			return nil, err
		}
		item.OrgNodeKey = strings.TrimSpace(item.OrgNodeKey)
		if item.OrgNodeKey != "" {
			out = append(out, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *pgAuthzRuntimeStore) ListRoleDefinitions(ctx context.Context, tenantID string) ([]authzRoleDefinition, error) {
	tx, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	rows, err := tx.Query(ctx, `
SELECT
  rd.role_slug,
  rd.name,
  rd.description,
  rd.system_managed,
  rd.revision,
  COALESCE(array_agg(rac.authz_capability_key ORDER BY rac.authz_capability_key) FILTER (WHERE rac.authz_capability_key IS NOT NULL), ARRAY[]::text[])
FROM iam.role_definitions rd
LEFT JOIN iam.role_authz_capabilities rac
  ON rac.role_id = rd.id
WHERE rd.tenant_uuid = $1::uuid
GROUP BY rd.id
ORDER BY rd.system_managed DESC, rd.role_slug ASC
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []authzRoleDefinition
	for rows.Next() {
		item, err := scanRoleDefinition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *pgAuthzRuntimeStore) GetRoleDefinition(ctx context.Context, tenantID string, roleSlug string) (authzRoleDefinition, bool, error) {
	roleSlug = strings.ToLower(strings.TrimSpace(roleSlug))
	tx, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return authzRoleDefinition{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	item, ok, err := getRoleDefinitionTx(ctx, tx, tenantID, roleSlug)
	if err != nil {
		return authzRoleDefinition{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return authzRoleDefinition{}, false, err
	}
	return item, ok, nil
}

func getRoleDefinitionTx(ctx context.Context, tx pgx.Tx, tenantID string, roleSlug string) (authzRoleDefinition, bool, error) {
	row := tx.QueryRow(ctx, `
SELECT
  rd.role_slug,
  rd.name,
  rd.description,
  rd.system_managed,
  rd.revision,
  COALESCE(array_agg(rac.authz_capability_key ORDER BY rac.authz_capability_key) FILTER (WHERE rac.authz_capability_key IS NOT NULL), ARRAY[]::text[])
FROM iam.role_definitions rd
LEFT JOIN iam.role_authz_capabilities rac
  ON rac.role_id = rd.id
WHERE rd.tenant_uuid = $1::uuid
  AND rd.role_slug = $2
GROUP BY rd.id
`, tenantID, roleSlug)
	item, err := scanRoleDefinition(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authzRoleDefinition{}, false, nil
		}
		return authzRoleDefinition{}, false, err
	}
	return item, true, nil
}

type roleDefinitionScanner interface {
	Scan(dest ...any) error
}

func scanRoleDefinition(scanner roleDefinitionScanner) (authzRoleDefinition, error) {
	var item authzRoleDefinition
	if err := scanner.Scan(
		&item.RoleSlug,
		&item.Name,
		&item.Description,
		&item.SystemManaged,
		&item.Revision,
		&item.AuthzCapabilityKeys,
	); err != nil {
		return authzRoleDefinition{}, err
	}
	item.RequiresOrgScope = roleRequiresOrgScope(item.AuthzCapabilityKeys)
	return item, nil
}

func (s *pgAuthzRuntimeStore) CreateRoleDefinition(ctx context.Context, tenantID string, input saveAuthzRoleDefinitionInput) (authzRoleDefinition, error) {
	roleSlug, err := normalizeRoleSlug(input.RoleSlug)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	name, err := normalizeRoleName(input.Name)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	keys, err := normalizeCapabilityKeys(input.AuthzCapabilityKeys)
	if err != nil {
		return authzRoleDefinition{}, err
	}

	tx, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var roleID string
	err = tx.QueryRow(ctx, `
INSERT INTO iam.role_definitions (tenant_uuid, role_slug, name, description, system_managed)
VALUES ($1::uuid, $2, $3, $4, false)
RETURNING id::text
`, tenantID, roleSlug, name, normalizeRoleDescription(input.Description)).Scan(&roleID)
	if err != nil {
		if isUniqueViolation(err) {
			return authzRoleDefinition{}, errRoleSlugConflict
		}
		return authzRoleDefinition{}, err
	}
	if err := replaceRoleCapabilitiesTx(ctx, tx, roleID, keys); err != nil {
		return authzRoleDefinition{}, err
	}
	out, ok, err := getRoleDefinitionTx(ctx, tx, tenantID, roleSlug)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	if !ok {
		return authzRoleDefinition{}, errRoleNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return authzRoleDefinition{}, err
	}
	return out, nil
}

func (s *pgAuthzRuntimeStore) UpdateRoleDefinition(ctx context.Context, tenantID string, roleSlug string, input saveAuthzRoleDefinitionInput) (authzRoleDefinition, error) {
	roleSlug, err := normalizeRoleSlug(roleSlug)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	name, err := normalizeRoleName(input.Name)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	if input.Revision < 1 {
		return authzRoleDefinition{}, errInvalidRoleDefinition
	}
	keys, err := normalizeCapabilityKeys(input.AuthzCapabilityKeys)
	if err != nil {
		return authzRoleDefinition{}, err
	}

	tx, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var roleID string
	var systemManaged bool
	var currentRevision int64
	err = tx.QueryRow(ctx, `
SELECT id::text, system_managed, revision
FROM iam.role_definitions
WHERE tenant_uuid = $1::uuid
  AND role_slug = $2
FOR UPDATE
`, tenantID, roleSlug).Scan(&roleID, &systemManaged, &currentRevision)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authzRoleDefinition{}, errRoleNotFound
		}
		return authzRoleDefinition{}, err
	}
	if systemManaged {
		return authzRoleDefinition{}, errSystemRoleReadonly
	}
	if currentRevision != input.Revision {
		return authzRoleDefinition{}, errStaleRevision
	}

	if _, err := tx.Exec(ctx, `
UPDATE iam.role_definitions
SET name = $3,
    description = $4,
    revision = revision + 1,
    updated_at = now()
WHERE tenant_uuid = $1::uuid
  AND role_slug = $2
`, tenantID, roleSlug, name, normalizeRoleDescription(input.Description)); err != nil {
		return authzRoleDefinition{}, err
	}
	if err := replaceRoleCapabilitiesTx(ctx, tx, roleID, keys); err != nil {
		return authzRoleDefinition{}, err
	}
	out, ok, err := getRoleDefinitionTx(ctx, tx, tenantID, roleSlug)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	if !ok {
		return authzRoleDefinition{}, errRoleNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return authzRoleDefinition{}, err
	}
	return out, nil
}

func replaceRoleCapabilitiesTx(ctx context.Context, tx pgx.Tx, roleID string, keys []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM iam.role_authz_capabilities WHERE role_id = $1::uuid`, roleID); err != nil {
		return err
	}
	for _, key := range keys {
		if _, err := tx.Exec(ctx, `
INSERT INTO iam.role_authz_capabilities (role_id, authz_capability_key)
VALUES ($1::uuid, $2)
`, roleID, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *pgAuthzRuntimeStore) GetPrincipalAssignment(ctx context.Context, tenantID string, principalID string) (principalAuthzAssignment, bool, error) {
	tx, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return principalAuthzAssignment{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	out, ok, err := getPrincipalAssignmentTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return principalAuthzAssignment{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return principalAuthzAssignment{}, false, err
	}
	return out, ok, nil
}

func (s *pgAuthzRuntimeStore) ReplacePrincipalAssignment(ctx context.Context, tenantID string, principalID string, input replacePrincipalAssignmentInput) (principalAuthzAssignment, error) {
	roleSlugs, err := normalizeAssignmentRoleSlugs(input.Roles)
	if err != nil {
		return principalAuthzAssignment{}, err
	}
	orgScopes, err := normalizePrincipalOrgScopes(input.OrgScopes)
	if err != nil {
		return principalAuthzAssignment{}, err
	}

	tx, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return principalAuthzAssignment{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	principalExists, err := principalExistsTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return principalAuthzAssignment{}, err
	}
	if !principalExists {
		return principalAuthzAssignment{}, errAuthzPrincipalMissing
	}
	if input.Revision < 1 {
		return principalAuthzAssignment{}, errInvalidAssignment
	}
	currentRevision, err := ensurePrincipalAssignmentRevisionTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return principalAuthzAssignment{}, err
	}
	if currentRevision != input.Revision {
		return principalAuthzAssignment{}, errStaleRevision
	}

	requiresOrgScope := false
	for _, roleSlug := range roleSlugs {
		role, ok, err := getRoleDefinitionTx(ctx, tx, tenantID, roleSlug)
		if err != nil {
			return principalAuthzAssignment{}, err
		}
		if !ok {
			return principalAuthzAssignment{}, errRoleNotFound
		}
		if role.RequiresOrgScope {
			requiresOrgScope = true
		}
	}
	if requiresOrgScope && len(orgScopes) == 0 {
		return principalAuthzAssignment{}, errAuthzOrgScopeRequired
	}
	for _, scope := range orgScopes {
		if err := orgScopeExistsTx(ctx, tx, tenantID, scope.OrgNodeKey); err != nil {
			return principalAuthzAssignment{}, err
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM iam.principal_role_assignments WHERE tenant_uuid = $1::uuid AND principal_id = $2::uuid`, tenantID, principalID); err != nil {
		return principalAuthzAssignment{}, err
	}
	for _, roleSlug := range roleSlugs {
		if _, err := tx.Exec(ctx, `
INSERT INTO iam.principal_role_assignments (tenant_uuid, principal_id, role_slug)
VALUES ($1::uuid, $2::uuid, $3)
`, tenantID, principalID, roleSlug); err != nil {
			return principalAuthzAssignment{}, err
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM iam.principal_org_scope_bindings WHERE tenant_uuid = $1::uuid AND principal_id = $2::uuid`, tenantID, principalID); err != nil {
		return principalAuthzAssignment{}, err
	}
	for _, scope := range orgScopes {
		if _, err := tx.Exec(ctx, `
	INSERT INTO iam.principal_org_scope_bindings (tenant_uuid, principal_id, org_node_key, include_descendants)
	VALUES ($1::uuid, $2::uuid, $3, $4)
	`, tenantID, principalID, scope.OrgNodeKey, scope.IncludeDescendants); err != nil {
			return principalAuthzAssignment{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
	UPDATE iam.principal_authz_assignment_revisions
	SET revision = revision + 1,
	    updated_at = now()
	WHERE tenant_uuid = $1::uuid
	  AND principal_id = $2::uuid
	`, tenantID, principalID); err != nil {
		return principalAuthzAssignment{}, err
	}

	out, ok, err := getPrincipalAssignmentTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return principalAuthzAssignment{}, err
	}
	if !ok {
		return principalAuthzAssignment{}, errAuthzPrincipalMissing
	}
	if err := tx.Commit(ctx); err != nil {
		return principalAuthzAssignment{}, err
	}
	return out, nil
}

func normalizeAssignmentRoleSlugs(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, errInvalidAssignment
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		roleSlug, err := normalizeRoleSlug(raw)
		if err != nil {
			return nil, errInvalidAssignment
		}
		if seen[roleSlug] {
			return nil, errInvalidAssignment
		}
		seen[roleSlug] = true
		out = append(out, roleSlug)
	}
	sort.Strings(out)
	return out, nil
}

func normalizePrincipalOrgScopes(values []principalOrgScope) ([]principalOrgScope, error) {
	seen := map[string]bool{}
	out := make([]principalOrgScope, 0, len(values))
	for _, raw := range values {
		orgNodeKey, err := orgunitpkg.NormalizeOrgNodeKey(strings.TrimSpace(raw.OrgNodeKey))
		if err != nil {
			return nil, errInvalidAssignment
		}
		if seen[orgNodeKey] {
			return nil, errInvalidAssignment
		}
		seen[orgNodeKey] = true
		out = append(out, principalOrgScope{
			OrgNodeKey:         orgNodeKey,
			IncludeDescendants: raw.IncludeDescendants,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].OrgNodeKey < out[j].OrgNodeKey
	})
	return out, nil
}

func principalExistsTx(ctx context.Context, tx pgx.Tx, tenantID string, principalID string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM iam.principals
  WHERE tenant_uuid = $1::uuid
    AND id = $2::uuid
    AND status = 'active'
)
`, tenantID, principalID).Scan(&exists)
	return exists, err
}

func orgScopeExistsTx(ctx context.Context, tx pgx.Tx, tenantID string, orgNodeKey string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM orgunit.org_unit_codes c
  WHERE c.tenant_uuid = $1::uuid
    AND `+orgNodeKeyCompatExpr("c")+` = $2
)
`, tenantID, orgNodeKey).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errInvalidAssignment
	}
	return nil
}

func ensureRootOrgScopeForUnscopedPrincipalTx(ctx context.Context, tx pgx.Tx, tenantID string, principalID string) error {
	scopes, err := orgScopesForPrincipalTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return err
	}
	if len(scopes) > 0 {
		return nil
	}
	var orgTreesExists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('orgunit.org_trees') IS NOT NULL`).Scan(&orgTreesExists); err != nil {
		return err
	}
	if !orgTreesExists {
		return nil
	}
	var rootOrgNodeKey *string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s AS root_org_node_key
		FROM orgunit.org_trees t
		WHERE t.tenant_uuid = $1::uuid
	`, rootOrgNodeKeyCompatExpr("t")), tenantID).Scan(&rootOrgNodeKey); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if rootOrgNodeKey == nil || strings.TrimSpace(*rootOrgNodeKey) == "" {
		return nil
	}
	_, err = tx.Exec(ctx, `
	INSERT INTO iam.principal_org_scope_bindings (tenant_uuid, principal_id, org_node_key, include_descendants)
	VALUES ($1::uuid, $2::uuid, $3, true)
	ON CONFLICT DO NOTHING
	`, tenantID, principalID, strings.TrimSpace(*rootOrgNodeKey))
	return err
}

func ensurePrincipalAssignmentRevisionTx(ctx context.Context, tx pgx.Tx, tenantID string, principalID string) (int64, error) {
	if _, err := tx.Exec(ctx, `
	INSERT INTO iam.principal_authz_assignment_revisions (tenant_uuid, principal_id)
	VALUES ($1::uuid, $2::uuid)
	ON CONFLICT DO NOTHING
	`, tenantID, principalID); err != nil {
		return 0, err
	}
	var revision int64
	if err := tx.QueryRow(ctx, `
	SELECT revision
	FROM iam.principal_authz_assignment_revisions
	WHERE tenant_uuid = $1::uuid
	  AND principal_id = $2::uuid
	FOR UPDATE
	`, tenantID, principalID).Scan(&revision); err != nil {
		return 0, err
	}
	if revision < 1 {
		return 0, errInvalidAssignment
	}
	return revision, nil
}

func getPrincipalAssignmentTx(ctx context.Context, tx pgx.Tx, tenantID string, principalID string) (principalAuthzAssignment, bool, error) {
	exists, err := principalExistsTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return principalAuthzAssignment{}, false, err
	}
	if !exists {
		return principalAuthzAssignment{}, false, nil
	}
	revision, err := ensurePrincipalAssignmentRevisionTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return principalAuthzAssignment{}, false, err
	}

	roleRows, err := tx.Query(ctx, `
SELECT
  rd.role_slug,
  rd.name,
  rd.description,
  COALESCE(array_agg(rac.authz_capability_key ORDER BY rac.authz_capability_key) FILTER (WHERE rac.authz_capability_key IS NOT NULL), ARRAY[]::text[])
FROM iam.principal_role_assignments pra
JOIN iam.role_definitions rd
  ON rd.tenant_uuid = pra.tenant_uuid
 AND rd.role_slug = pra.role_slug
LEFT JOIN iam.role_authz_capabilities rac
  ON rac.role_id = rd.id
WHERE pra.tenant_uuid = $1::uuid
  AND pra.principal_id = $2::uuid
GROUP BY rd.id
ORDER BY rd.role_slug ASC
`, tenantID, principalID)
	if err != nil {
		return principalAuthzAssignment{}, false, err
	}
	defer roleRows.Close()

	out := principalAuthzAssignment{
		PrincipalID: principalID,
		Revision:    revision,
	}
	for roleRows.Next() {
		var role principalRoleAssignment
		var keys []string
		if err := roleRows.Scan(&role.RoleSlug, &role.DisplayName, &role.Description, &keys); err != nil {
			return principalAuthzAssignment{}, false, err
		}
		role.RequiresOrgScope = roleRequiresOrgScope(keys)
		out.Roles = append(out.Roles, role)
	}
	if err := roleRows.Err(); err != nil {
		return principalAuthzAssignment{}, false, err
	}

	scopes, err := orgScopesForPrincipalTx(ctx, tx, tenantID, principalID)
	if err != nil {
		return principalAuthzAssignment{}, false, err
	}
	out.OrgScopes = scopes
	return out, true, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type memoryAuthzRuntimeStore struct {
	mu                  sync.Mutex
	roles               map[string]map[string]authzRoleDefinition
	principalRoles      map[string]map[string]bool
	orgScopes           map[string][]principalOrgScope
	assignmentRevisions map[string]int64
}

func newMemoryAuthzRuntimeStore() *memoryAuthzRuntimeStore {
	return &memoryAuthzRuntimeStore{
		roles:               map[string]map[string]authzRoleDefinition{},
		principalRoles:      map[string]map[string]bool{},
		orgScopes:           map[string][]principalOrgScope{},
		assignmentRevisions: map[string]int64{},
	}
}

func memoryRuntimeKey(tenantID string, principalID string) string {
	return strings.TrimSpace(tenantID) + "|" + strings.TrimSpace(principalID)
}

func (s *memoryAuthzRuntimeStore) ensureTenantRolesLocked(tenantID string) {
	tenantID = strings.TrimSpace(tenantID)
	if s.roles[tenantID] != nil {
		return
	}
	adminKeys := builtinTenantAdminCapabilityKeys()
	viewerKeys := builtinTenantViewerCapabilityKeys()
	s.roles[tenantID] = map[string]authzRoleDefinition{
		authz.RoleTenantAdmin: {
			RoleSlug:            authz.RoleTenantAdmin,
			Name:                "Tenant Admin",
			Description:         "Built-in tenant administrator role",
			SystemManaged:       true,
			Revision:            1,
			AuthzCapabilityKeys: adminKeys,
			RequiresOrgScope:    roleRequiresOrgScope(adminKeys),
		},
		authz.RoleTenantViewer: {
			RoleSlug:            authz.RoleTenantViewer,
			Name:                "Tenant Viewer",
			Description:         "Built-in tenant viewer role",
			SystemManaged:       true,
			Revision:            1,
			AuthzCapabilityKeys: viewerKeys,
			RequiresOrgScope:    roleRequiresOrgScope(viewerKeys),
		},
	}
}

func (s *memoryAuthzRuntimeStore) AuthorizePrincipal(ctx context.Context, tenantID string, principalID string, object string, action string) (bool, error) {
	key := authz.AuthzCapabilityKey(object, action)
	keys, err := s.CapabilitiesForPrincipal(ctx, tenantID, principalID)
	if err != nil {
		return false, err
	}
	for _, got := range keys {
		if got == key {
			return true, nil
		}
	}
	return false, nil
}

func (s *memoryAuthzRuntimeStore) CapabilitiesForPrincipal(_ context.Context, tenantID string, principalID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureTenantRolesLocked(tenantID)
	assignments := s.principalRoles[memoryRuntimeKey(tenantID, principalID)]
	if len(assignments) == 0 {
		return nil, errAuthzPrincipalMissing
	}
	seen := map[string]bool{}
	for roleSlug := range assignments {
		role, ok := s.roles[strings.TrimSpace(tenantID)][roleSlug]
		if !ok {
			continue
		}
		for _, key := range role.AuthzCapabilityKeys {
			seen[key] = true
		}
	}
	if len(seen) == 0 {
		return nil, errAuthzPrincipalMissing
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out, nil
}

func (s *memoryAuthzRuntimeStore) OrgScopesForPrincipal(_ context.Context, tenantID string, principalID string, capabilityKey string) ([]principalOrgScope, error) {
	entry, ok := authz.LookupAuthzCapability(strings.TrimSpace(capabilityKey))
	if !ok {
		return nil, errInvalidRoleDefinition
	}
	if entry.ScopeDimension != authz.ScopeDimensionOrganization {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	scopes := append([]principalOrgScope(nil), s.orgScopes[memoryRuntimeKey(tenantID, principalID)]...)
	if len(scopes) == 0 {
		key := memoryRuntimeKey(tenantID, principalID)
		if s.assignmentRevisions[key] == 1 && len(s.principalRoles[key]) > 0 {
			rootOrgNodeKey, err := encodeOrgNodeKeyFromID(10000000)
			if err != nil {
				return nil, errInvalidAssignment
			}
			scopes = []principalOrgScope{{
				OrgNodeKey:         rootOrgNodeKey,
				IncludeDescendants: true,
			}}
			s.orgScopes[key] = append([]principalOrgScope(nil), scopes...)
		}
	}
	if len(scopes) == 0 {
		return nil, errAuthzOrgScopeRequired
	}
	return scopes, nil
}

func (s *memoryAuthzRuntimeStore) ListRoleDefinitions(_ context.Context, tenantID string) ([]authzRoleDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureTenantRolesLocked(tenantID)
	out := make([]authzRoleDefinition, 0, len(s.roles[strings.TrimSpace(tenantID)]))
	for _, role := range s.roles[strings.TrimSpace(tenantID)] {
		out = append(out, role)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SystemManaged != out[j].SystemManaged {
			return out[i].SystemManaged
		}
		return out[i].RoleSlug < out[j].RoleSlug
	})
	return out, nil
}

func (s *memoryAuthzRuntimeStore) GetRoleDefinition(_ context.Context, tenantID string, roleSlug string) (authzRoleDefinition, bool, error) {
	roleSlug = strings.ToLower(strings.TrimSpace(roleSlug))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureTenantRolesLocked(tenantID)
	role, ok := s.roles[strings.TrimSpace(tenantID)][roleSlug]
	return role, ok, nil
}

func (s *memoryAuthzRuntimeStore) CreateRoleDefinition(_ context.Context, tenantID string, input saveAuthzRoleDefinitionInput) (authzRoleDefinition, error) {
	roleSlug, err := normalizeRoleSlug(input.RoleSlug)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	name, err := normalizeRoleName(input.Name)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	keys, err := normalizeCapabilityKeys(input.AuthzCapabilityKeys)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureTenantRolesLocked(tenantID)
	roles := s.roles[strings.TrimSpace(tenantID)]
	if _, ok := roles[roleSlug]; ok {
		return authzRoleDefinition{}, errRoleSlugConflict
	}
	role := authzRoleDefinition{
		RoleSlug:            roleSlug,
		Name:                name,
		Description:         normalizeRoleDescription(input.Description),
		Revision:            1,
		AuthzCapabilityKeys: keys,
		RequiresOrgScope:    roleRequiresOrgScope(keys),
	}
	roles[roleSlug] = role
	return role, nil
}

func (s *memoryAuthzRuntimeStore) UpdateRoleDefinition(_ context.Context, tenantID string, roleSlug string, input saveAuthzRoleDefinitionInput) (authzRoleDefinition, error) {
	roleSlug, err := normalizeRoleSlug(roleSlug)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	name, err := normalizeRoleName(input.Name)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	if input.Revision < 1 {
		return authzRoleDefinition{}, errInvalidRoleDefinition
	}
	keys, err := normalizeCapabilityKeys(input.AuthzCapabilityKeys)
	if err != nil {
		return authzRoleDefinition{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureTenantRolesLocked(tenantID)
	roles := s.roles[strings.TrimSpace(tenantID)]
	role, ok := roles[roleSlug]
	if !ok {
		return authzRoleDefinition{}, errRoleNotFound
	}
	if role.SystemManaged {
		return authzRoleDefinition{}, errSystemRoleReadonly
	}
	if role.Revision != input.Revision {
		return authzRoleDefinition{}, errStaleRevision
	}
	role.Name = name
	role.Description = normalizeRoleDescription(input.Description)
	role.AuthzCapabilityKeys = keys
	role.RequiresOrgScope = roleRequiresOrgScope(keys)
	role.Revision++
	roles[roleSlug] = role
	return role, nil
}

func (s *memoryAuthzRuntimeStore) GetPrincipalAssignment(_ context.Context, tenantID string, principalID string) (principalAuthzAssignment, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureTenantRolesLocked(tenantID)
	key := memoryRuntimeKey(tenantID, principalID)
	revision := s.ensureAssignmentRevisionLocked(key)
	out := principalAuthzAssignment{PrincipalID: strings.TrimSpace(principalID), Revision: revision}
	for roleSlug := range s.principalRoles[key] {
		role, ok := s.roles[strings.TrimSpace(tenantID)][roleSlug]
		if !ok {
			continue
		}
		out.Roles = append(out.Roles, principalRoleAssignment{
			RoleSlug:         role.RoleSlug,
			DisplayName:      role.Name,
			Description:      role.Description,
			RequiresOrgScope: role.RequiresOrgScope,
		})
	}
	sort.SliceStable(out.Roles, func(i, j int) bool { return out.Roles[i].RoleSlug < out.Roles[j].RoleSlug })
	out.OrgScopes = append([]principalOrgScope(nil), s.orgScopes[key]...)
	return out, true, nil
}

func (s *memoryAuthzRuntimeStore) ReplacePrincipalAssignment(_ context.Context, tenantID string, principalID string, input replacePrincipalAssignmentInput) (principalAuthzAssignment, error) {
	roleSlugs, err := normalizeAssignmentRoleSlugs(input.Roles)
	if err != nil {
		return principalAuthzAssignment{}, err
	}
	scopes, err := normalizePrincipalOrgScopes(input.OrgScopes)
	if err != nil {
		return principalAuthzAssignment{}, err
	}
	if input.Revision < 1 {
		return principalAuthzAssignment{}, errInvalidAssignment
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureTenantRolesLocked(tenantID)
	key := memoryRuntimeKey(tenantID, principalID)
	currentRevision := s.ensureAssignmentRevisionLocked(key)
	if currentRevision != input.Revision {
		return principalAuthzAssignment{}, errStaleRevision
	}
	requiresOrgScope := false
	for _, roleSlug := range roleSlugs {
		role, ok := s.roles[strings.TrimSpace(tenantID)][roleSlug]
		if !ok {
			return principalAuthzAssignment{}, errRoleNotFound
		}
		if role.RequiresOrgScope {
			requiresOrgScope = true
		}
	}
	if requiresOrgScope && len(scopes) == 0 {
		return principalAuthzAssignment{}, errAuthzOrgScopeRequired
	}
	assignments := map[string]bool{}
	for _, roleSlug := range roleSlugs {
		assignments[roleSlug] = true
	}
	s.principalRoles[key] = assignments
	s.orgScopes[key] = append([]principalOrgScope(nil), scopes...)
	s.assignmentRevisions[key] = currentRevision + 1
	return s.getPrincipalAssignmentLocked(tenantID, principalID), nil
}

func (s *memoryAuthzRuntimeStore) EnsurePrincipalRoleAssignment(_ context.Context, tenantID string, principalID string, roleSlug string) error {
	roleSlug, err := normalizeRoleSlug(roleSlug)
	if err != nil {
		return errInvalidAssignment
	}
	if !builtinIdentityRoleSlugs[roleSlug] {
		return errInvalidAssignment
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureTenantRolesLocked(tenantID)
	if _, ok := s.roles[strings.TrimSpace(tenantID)][roleSlug]; !ok {
		return errRoleNotFound
	}
	key := memoryRuntimeKey(tenantID, principalID)
	if s.principalRoles[key] == nil {
		s.principalRoles[key] = map[string]bool{}
	}
	s.ensureAssignmentRevisionLocked(key)
	if len(s.principalRoles[key]) > 0 {
		return nil
	}
	s.principalRoles[key][roleSlug] = true
	if len(s.orgScopes[key]) == 0 {
		rootOrgNodeKey, err := encodeOrgNodeKeyFromID(10000000)
		if err != nil {
			return errInvalidAssignment
		}
		s.orgScopes[key] = []principalOrgScope{{
			OrgNodeKey:         rootOrgNodeKey,
			IncludeDescendants: true,
		}}
	}
	return nil
}

func (s *memoryAuthzRuntimeStore) getPrincipalAssignmentLocked(tenantID string, principalID string) principalAuthzAssignment {
	key := memoryRuntimeKey(tenantID, principalID)
	revision := s.ensureAssignmentRevisionLocked(key)
	out := principalAuthzAssignment{PrincipalID: strings.TrimSpace(principalID), Revision: revision}
	for roleSlug := range s.principalRoles[key] {
		role, ok := s.roles[strings.TrimSpace(tenantID)][roleSlug]
		if !ok {
			continue
		}
		out.Roles = append(out.Roles, principalRoleAssignment{
			RoleSlug:         role.RoleSlug,
			DisplayName:      role.Name,
			Description:      role.Description,
			RequiresOrgScope: role.RequiresOrgScope,
		})
	}
	sort.SliceStable(out.Roles, func(i, j int) bool { return out.Roles[i].RoleSlug < out.Roles[j].RoleSlug })
	out.OrgScopes = append([]principalOrgScope(nil), s.orgScopes[key]...)
	return out
}

func (s *memoryAuthzRuntimeStore) ensureAssignmentRevisionLocked(key string) int64 {
	revision := s.assignmentRevisions[key]
	if revision < 1 {
		revision = 1
		s.assignmentRevisions[key] = revision
	}
	return revision
}
