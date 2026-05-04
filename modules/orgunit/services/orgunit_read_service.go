package services

import (
	"context"
	"errors"
	"strings"
	"time"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

var (
	ErrOrgUnitReadInvalidArgument     = errors.New("orgunit_read_invalid_argument")
	ErrOrgUnitReadScopeRequired       = errors.New("orgunit_read_scope_required")
	ErrOrgUnitReadScopeForbidden      = errors.New("orgunit_read_scope_forbidden")
	ErrOrgUnitReadNotFound            = errors.New("orgunit_read_not_found")
	ErrOrgUnitReadSafePathUnavailable = errors.New("orgunit_read_safe_path_unavailable")
)

type OrgUnitReadService interface {
	VisibleRoots(ctx context.Context, req OrgUnitReadRequest) ([]OrgUnitReadNode, error)
	Children(ctx context.Context, req OrgUnitChildrenRequest) ([]OrgUnitReadNode, error)
	Search(ctx context.Context, req OrgUnitSearchRequest) ([]OrgUnitReadNode, error)
	Resolve(ctx context.Context, req OrgUnitResolveRequest) ([]OrgUnitReadNode, error)
}

type OrgUnitReadStore interface {
	ListRoots(ctx context.Context, tenantID string, asOf string, includeDisabled bool) ([]OrgUnitReadNode, error)
	ListChildren(ctx context.Context, tenantID string, parentOrgNodeKey string, asOf string, includeDisabled bool) ([]OrgUnitReadNode, error)
	ResolveByOrgNodeKey(ctx context.Context, tenantID string, orgNodeKey string, asOf string, includeDisabled bool) (OrgUnitReadNode, error)
	ResolveByOrgCode(ctx context.Context, tenantID string, orgCode string, asOf string, includeDisabled bool) (OrgUnitReadNode, error)
	Search(ctx context.Context, tenantID string, query string, asOf string, includeDisabled bool, limit int) ([]OrgUnitReadNode, error)
}

type OrgUnitScope struct {
	OrgNodeKey         string
	IncludeDescendants bool
}

type OrgUnitReadScopeFilter struct {
	PrincipalID string
	AllTenant   bool
	Scopes      []OrgUnitScope
}

type OrgUnitReadRequest struct {
	TenantID        string
	AsOf            string
	ScopeFilter     OrgUnitReadScopeFilter
	IncludeDisabled bool
	Caller          string
}

type OrgUnitChildrenRequest struct {
	TenantID         string
	AsOf             string
	ScopeFilter      OrgUnitReadScopeFilter
	ParentOrgCode    string
	ParentOrgNodeKey string
	IncludeDisabled  bool
	Caller           string
}

type OrgUnitSearchRequest struct {
	TenantID        string
	AsOf            string
	ScopeFilter     OrgUnitReadScopeFilter
	Query           string
	IncludeDisabled bool
	Limit           int
	Caller          string
}

type OrgUnitResolveRequest struct {
	TenantID        string
	AsOf            string
	ScopeFilter     OrgUnitReadScopeFilter
	OrgCodes        []string
	OrgNodeKeys     []string
	IncludeDisabled bool
	Caller          string
}

type OrgUnitReadNode struct {
	OrgCode            string
	OrgNodeKey         string
	Name               string
	Status             string
	IsBusinessUnit     *bool
	HasVisibleChildren bool
	PathOrgCodes       []string
	PathOrgNodeKeys    []string
}

type orgUnitReadService struct {
	store OrgUnitReadStore
}

func NewOrgUnitReadService(store OrgUnitReadStore) OrgUnitReadService {
	return orgUnitReadService{store: store}
}

func (s orgUnitReadService) VisibleRoots(ctx context.Context, req OrgUnitReadRequest) ([]OrgUnitReadNode, error) {
	if err := validateOrgUnitReadBase(req.TenantID, req.AsOf); err != nil {
		return nil, err
	}
	if req.ScopeFilter.AllTenant {
		roots, err := s.store.ListRoots(ctx, req.TenantID, req.AsOf, req.IncludeDisabled)
		if err != nil {
			return nil, err
		}
		return s.decorateVisibleChildren(ctx, req.TenantID, req.AsOf, req.IncludeDisabled, req.ScopeFilter, roots)
	}

	roots, err := s.visibleRootsFromScope(ctx, req.TenantID, req.AsOf, req.IncludeDisabled, req.ScopeFilter)
	if err != nil {
		return nil, err
	}
	return s.decorateVisibleChildren(ctx, req.TenantID, req.AsOf, req.IncludeDisabled, req.ScopeFilter, roots)
}

func (s orgUnitReadService) Children(ctx context.Context, req OrgUnitChildrenRequest) ([]OrgUnitReadNode, error) {
	if err := validateOrgUnitReadBase(req.TenantID, req.AsOf); err != nil {
		return nil, err
	}
	parent, err := s.resolveParent(ctx, req)
	if err != nil {
		return nil, err
	}
	if !scopeAllowsReadNode(req.ScopeFilter, parent) {
		return nil, ErrOrgUnitReadScopeForbidden
	}
	children, err := s.store.ListChildren(ctx, req.TenantID, parent.OrgNodeKey, req.AsOf, req.IncludeDisabled)
	if err != nil {
		return nil, err
	}
	visible := filterReadNodesByScope(req.ScopeFilter, children)
	return s.decorateVisibleChildren(ctx, req.TenantID, req.AsOf, req.IncludeDisabled, req.ScopeFilter, visible)
}

func (s orgUnitReadService) Search(ctx context.Context, req OrgUnitSearchRequest) ([]OrgUnitReadNode, error) {
	if err := validateOrgUnitReadBase(req.TenantID, req.AsOf); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, ErrOrgUnitReadInvalidArgument
	}
	roots, err := s.VisibleRoots(ctx, OrgUnitReadRequest{
		TenantID:        req.TenantID,
		AsOf:            req.AsOf,
		ScopeFilter:     req.ScopeFilter,
		IncludeDisabled: req.IncludeDisabled,
		Caller:          req.Caller,
	})
	if err != nil {
		return nil, err
	}
	searchLimit := req.Limit
	if !req.ScopeFilter.AllTenant {
		searchLimit = -1
	}
	candidates, err := s.store.Search(ctx, req.TenantID, query, req.AsOf, req.IncludeDisabled, searchLimit)
	if err != nil {
		return nil, err
	}
	out, err := s.withSafePaths(filterReadNodesByScope(req.ScopeFilter, candidates), roots)
	if err != nil {
		return nil, err
	}
	return limitReadNodes(out, req.Limit), nil
}

func (s orgUnitReadService) Resolve(ctx context.Context, req OrgUnitResolveRequest) ([]OrgUnitReadNode, error) {
	if err := validateOrgUnitReadBase(req.TenantID, req.AsOf); err != nil {
		return nil, err
	}
	roots, err := s.VisibleRoots(ctx, OrgUnitReadRequest{
		TenantID:        req.TenantID,
		AsOf:            req.AsOf,
		ScopeFilter:     req.ScopeFilter,
		IncludeDisabled: req.IncludeDisabled,
		Caller:          req.Caller,
	})
	if err != nil {
		return nil, err
	}

	nodes := make([]OrgUnitReadNode, 0, len(req.OrgCodes)+len(req.OrgNodeKeys))
	for _, code := range req.OrgCodes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		node, err := s.store.ResolveByOrgCode(ctx, req.TenantID, code, req.AsOf, req.IncludeDisabled)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	for _, key := range req.OrgNodeKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		node, err := s.store.ResolveByOrgNodeKey(ctx, req.TenantID, key, req.AsOf, req.IncludeDisabled)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, ErrOrgUnitReadInvalidArgument
	}
	return s.withSafePaths(filterReadNodesByScope(req.ScopeFilter, nodes), roots)
}

func (s orgUnitReadService) visibleRootsFromScope(ctx context.Context, tenantID string, asOf string, includeDisabled bool, filter OrgUnitReadScopeFilter) ([]OrgUnitReadNode, error) {
	scopes := normalizeReadScopes(filter.Scopes)
	if len(scopes) == 0 {
		return nil, ErrOrgUnitReadScopeRequired
	}

	type scopedReadNode struct {
		scope OrgUnitScope
		node  OrgUnitReadNode
	}

	nodes := make([]scopedReadNode, 0, len(scopes))
	for _, scope := range scopes {
		node, err := s.store.ResolveByOrgNodeKey(ctx, tenantID, scope.OrgNodeKey, asOf, includeDisabled)
		if err != nil {
			if errors.Is(err, ErrOrgUnitReadNotFound) {
				continue
			}
			return nil, err
		}
		node.OrgNodeKey = scope.OrgNodeKey
		node.PathOrgNodeKeys = normalizePathOrgNodeKeys(node.PathOrgNodeKeys, node.OrgNodeKey)
		nodes = append(nodes, scopedReadNode{scope: scope, node: node})
	}
	if len(nodes) == 0 {
		return nil, ErrOrgUnitReadScopeForbidden
	}

	roots := make([]OrgUnitReadNode, 0, len(nodes))
	for i, candidate := range nodes {
		covered := false
		for j, ancestor := range nodes {
			if i == j {
				continue
			}
			if ancestor.scope.IncludeDescendants && pathContainsOrgNodeKey(candidate.node.PathOrgNodeKeys, ancestor.scope.OrgNodeKey) {
				covered = true
				break
			}
		}
		if !covered {
			roots = append(roots, candidate.node)
		}
	}
	if len(roots) == 0 {
		return nil, ErrOrgUnitReadScopeForbidden
	}
	return roots, nil
}

func (s orgUnitReadService) resolveParent(ctx context.Context, req OrgUnitChildrenRequest) (OrgUnitReadNode, error) {
	nodeKey := strings.TrimSpace(req.ParentOrgNodeKey)
	code := strings.TrimSpace(req.ParentOrgCode)
	if nodeKey == "" && code == "" {
		return OrgUnitReadNode{}, ErrOrgUnitReadInvalidArgument
	}
	var byKey *OrgUnitReadNode
	if nodeKey != "" {
		node, err := s.store.ResolveByOrgNodeKey(ctx, req.TenantID, nodeKey, req.AsOf, req.IncludeDisabled)
		if err != nil {
			return OrgUnitReadNode{}, err
		}
		byKey = &node
	}
	if code == "" {
		return *byKey, nil
	}
	byCode, err := s.store.ResolveByOrgCode(ctx, req.TenantID, code, req.AsOf, req.IncludeDisabled)
	if err != nil {
		return OrgUnitReadNode{}, err
	}
	if byKey != nil && strings.TrimSpace(byKey.OrgNodeKey) != strings.TrimSpace(byCode.OrgNodeKey) {
		return OrgUnitReadNode{}, ErrOrgUnitReadInvalidArgument
	}
	return byCode, nil
}

func (s orgUnitReadService) decorateVisibleChildren(ctx context.Context, tenantID string, asOf string, includeDisabled bool, filter OrgUnitReadScopeFilter, nodes []OrgUnitReadNode) ([]OrgUnitReadNode, error) {
	out := append([]OrgUnitReadNode(nil), nodes...)
	for i := range out {
		children, err := s.store.ListChildren(ctx, tenantID, out[i].OrgNodeKey, asOf, includeDisabled)
		if err != nil {
			return nil, err
		}
		out[i].PathOrgNodeKeys = normalizePathOrgNodeKeys(out[i].PathOrgNodeKeys, out[i].OrgNodeKey)
		out[i].HasVisibleChildren = len(filterReadNodesByScope(filter, children)) > 0
	}
	return out, nil
}

func (s orgUnitReadService) withSafePaths(nodes []OrgUnitReadNode, roots []OrgUnitReadNode) ([]OrgUnitReadNode, error) {
	out := make([]OrgUnitReadNode, 0, len(nodes))
	for _, node := range nodes {
		if strings.TrimSpace(node.OrgNodeKey) == "" {
			return nil, ErrOrgUnitReadSafePathUnavailable
		}
		pathCodes, err := safePathOrgCodes(node, roots)
		if err != nil {
			return nil, err
		}
		node.PathOrgCodes = pathCodes
		out = append(out, node)
	}
	return out, nil
}

func validateOrgUnitReadBase(tenantID string, asOf string) error {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(asOf) == "" {
		return ErrOrgUnitReadInvalidArgument
	}
	if _, err := time.Parse("2006-01-02", strings.TrimSpace(asOf)); err != nil {
		return ErrOrgUnitReadInvalidArgument
	}
	return nil
}

func normalizeReadScopes(scopes []OrgUnitScope) []OrgUnitScope {
	out := make([]OrgUnitScope, 0, len(scopes))
	seen := map[string]int{}
	for _, scope := range scopes {
		orgNodeKey, err := orgunitpkg.NormalizeOrgNodeKey(scope.OrgNodeKey)
		if err != nil {
			continue
		}
		if idx, ok := seen[orgNodeKey]; ok {
			out[idx].IncludeDescendants = out[idx].IncludeDescendants || scope.IncludeDescendants
			continue
		}
		seen[orgNodeKey] = len(out)
		out = append(out, OrgUnitScope{
			OrgNodeKey:         orgNodeKey,
			IncludeDescendants: scope.IncludeDescendants,
		})
	}
	return out
}

func filterReadNodesByScope(filter OrgUnitReadScopeFilter, nodes []OrgUnitReadNode) []OrgUnitReadNode {
	if filter.AllTenant {
		return append([]OrgUnitReadNode(nil), nodes...)
	}
	out := make([]OrgUnitReadNode, 0, len(nodes))
	for _, node := range nodes {
		if scopeAllowsReadNode(filter, node) {
			out = append(out, node)
		}
	}
	return out
}

func scopeAllowsReadNode(filter OrgUnitReadScopeFilter, node OrgUnitReadNode) bool {
	if filter.AllTenant {
		return true
	}
	node.OrgNodeKey = strings.TrimSpace(node.OrgNodeKey)
	if node.OrgNodeKey == "" {
		return false
	}
	path := normalizePathOrgNodeKeys(node.PathOrgNodeKeys, node.OrgNodeKey)
	for _, scope := range normalizeReadScopes(filter.Scopes) {
		if node.OrgNodeKey == scope.OrgNodeKey {
			return true
		}
		if scope.IncludeDescendants && pathContainsOrgNodeKey(path, scope.OrgNodeKey) {
			return true
		}
	}
	return false
}

func normalizePathOrgNodeKeys(path []string, orgNodeKey string) []string {
	out := make([]string, 0, len(path)+1)
	for _, item := range path {
		normalized, err := orgunitpkg.NormalizeOrgNodeKey(item)
		if err == nil {
			out = append(out, normalized)
		}
	}
	key, err := orgunitpkg.NormalizeOrgNodeKey(orgNodeKey)
	if err != nil {
		return out
	}
	if len(out) == 0 || out[len(out)-1] != key {
		out = append(out, key)
	}
	return out
}

func pathContainsOrgNodeKey(path []string, orgNodeKey string) bool {
	for _, item := range path {
		if strings.TrimSpace(item) == strings.TrimSpace(orgNodeKey) {
			return true
		}
	}
	return false
}

func safePathOrgCodes(node OrgUnitReadNode, roots []OrgUnitReadNode) ([]string, error) {
	nodePath := normalizePathOrgNodeKeys(node.PathOrgNodeKeys, node.OrgNodeKey)
	if len(nodePath) == 0 || len(node.PathOrgCodes) != len(nodePath) {
		return nil, ErrOrgUnitReadSafePathUnavailable
	}
	for _, root := range roots {
		rootKey := strings.TrimSpace(root.OrgNodeKey)
		if rootKey == "" {
			continue
		}
		for idx, pathKey := range nodePath {
			if pathKey == rootKey {
				return append([]string(nil), node.PathOrgCodes[idx:]...), nil
			}
		}
	}
	return nil, ErrOrgUnitReadScopeForbidden
}

func limitReadNodes(nodes []OrgUnitReadNode, limit int) []OrgUnitReadNode {
	if limit <= 0 || len(nodes) <= limit {
		return nodes
	}
	return append([]OrgUnitReadNode(nil), nodes[:limit]...)
}
