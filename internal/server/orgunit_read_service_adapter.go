package server

import (
	"context"
	"errors"
	"strings"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitReadStoreAdapter struct {
	store OrgUnitStore
}

func (a orgUnitReadStoreAdapter) ListRoots(ctx context.Context, tenantID string, asOf string, includeDisabled bool) ([]orgunitservices.OrgUnitReadNode, error) {
	nodes, err := listNodesCurrentByVisibility(ctx, a.store, tenantID, asOf, includeDisabled)
	if err != nil {
		return nil, err
	}
	out := make([]orgunitservices.OrgUnitReadNode, 0, len(nodes))
	for _, node := range nodes {
		readNode, err := orgUnitReadNodeFromRootNode(node)
		if err != nil {
			return nil, err
		}
		out = append(out, readNode)
	}
	return out, nil
}

func (a orgUnitReadStoreAdapter) ListChildren(ctx context.Context, tenantID string, parentOrgNodeKey string, asOf string, includeDisabled bool) ([]orgunitservices.OrgUnitReadNode, error) {
	children, err := listChildrenByVisibilityByNodeKey(ctx, a.store, tenantID, parentOrgNodeKey, asOf, includeDisabled)
	if err != nil {
		return nil, err
	}
	out := make([]orgunitservices.OrgUnitReadNode, 0, len(children))
	for _, child := range children {
		orgNodeKey, err := orgNodeKeyFromCompat(child.OrgNodeKey, child.OrgID)
		if err != nil {
			return nil, err
		}
		readNode, err := a.ResolveByOrgNodeKey(ctx, tenantID, orgNodeKey, asOf, includeDisabled)
		if err != nil {
			return nil, err
		}
		readNode.HasVisibleChildren = child.HasChildren
		if strings.TrimSpace(readNode.Name) == "" {
			readNode.Name = child.Name
		}
		if strings.TrimSpace(readNode.OrgCode) == "" {
			readNode.OrgCode = child.OrgCode
		}
		if strings.TrimSpace(readNode.Status) == "" {
			readNode.Status = orgUnitReadStatusOrActive(child.Status)
		}
		if readNode.IsBusinessUnit == nil {
			isBU := child.IsBusinessUnit
			readNode.IsBusinessUnit = &isBU
		}
		out = append(out, readNode)
	}
	return out, nil
}

func (a orgUnitReadStoreAdapter) ListTree(ctx context.Context, tenantID string, asOf string, includeDisabled bool) ([]orgunitservices.OrgUnitReadNode, error) {
	items, _, err := listOrgUnitListPage(ctx, a.store, tenantID, orgUnitListPageRequest{
		AsOf:            asOf,
		IncludeDisabled: includeDisabled,
		AllOrgUnits:     true,
	})
	if err != nil {
		return nil, err
	}
	return orgUnitReadNodesFromListItems(items), nil
}

func (a orgUnitReadStoreAdapter) ResolveByOrgNodeKey(ctx context.Context, tenantID string, orgNodeKey string, asOf string, includeDisabled bool) (orgunitservices.OrgUnitReadNode, error) {
	details, err := getNodeDetailsByVisibilityByNodeKey(ctx, a.store, tenantID, orgNodeKey, asOf, includeDisabled)
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) || errors.Is(err, orgunitpkg.ErrOrgNodeKeyNotFound) {
			return orgunitservices.OrgUnitReadNode{}, orgunitservices.ErrOrgUnitReadNotFound
		}
		return orgunitservices.OrgUnitReadNode{}, err
	}
	return orgUnitReadNodeFromDetails(ctx, a.store, tenantID, details)
}

func (a orgUnitReadStoreAdapter) ResolveByOrgCode(ctx context.Context, tenantID string, orgCode string, asOf string, includeDisabled bool) (orgunitservices.OrgUnitReadNode, error) {
	normalized, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return orgunitservices.OrgUnitReadNode{}, err
	}
	orgNodeKey, err := a.store.ResolveOrgNodeKeyByCode(ctx, tenantID, normalized)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
			return orgunitservices.OrgUnitReadNode{}, orgunitservices.ErrOrgUnitReadNotFound
		}
		return orgunitservices.OrgUnitReadNode{}, err
	}
	if strings.TrimSpace(orgNodeKey) == "" {
		return orgunitservices.OrgUnitReadNode{}, orgunitservices.ErrOrgUnitReadNotFound
	}
	return a.ResolveByOrgNodeKey(ctx, tenantID, orgNodeKey, asOf, includeDisabled)
}

func (a orgUnitReadStoreAdapter) Search(ctx context.Context, tenantID string, query string, asOf string, includeDisabled bool, limit int) ([]orgunitservices.OrgUnitReadNode, error) {
	candidates, err := searchNodeCandidatesByVisibility(ctx, a.store, tenantID, query, asOf, limit, includeDisabled)
	if err != nil {
		return nil, err
	}
	out := make([]orgunitservices.OrgUnitReadNode, 0, len(candidates))
	for _, candidate := range candidates {
		orgNodeKey, err := orgNodeKeyFromCompat(candidate.OrgNodeKey, candidate.OrgID)
		if err != nil {
			return nil, err
		}
		readNode, err := a.ResolveByOrgNodeKey(ctx, tenantID, orgNodeKey, asOf, includeDisabled)
		if err != nil {
			return nil, err
		}
		out = append(out, readNode)
	}
	return out, nil
}

func orgUnitReadNodeFromRootNode(node OrgUnitNode) (orgunitservices.OrgUnitReadNode, error) {
	orgNodeKey, err := orgNodeKeyFromCompat(node.ID, node.OrgID)
	if err != nil {
		return orgunitservices.OrgUnitReadNode{}, err
	}
	isBU := node.IsBusinessUnit
	return orgunitservices.OrgUnitReadNode{
		OrgCode:            node.OrgCode,
		OrgNodeKey:         orgNodeKey,
		Name:               node.Name,
		Status:             orgUnitReadStatusOrActive(node.Status),
		IsBusinessUnit:     &isBU,
		HasVisibleChildren: node.HasChildren,
		PathOrgNodeKeys:    []string{orgNodeKey},
		PathOrgCodes:       []string{node.OrgCode},
	}, nil
}

func orgUnitReadNodeFromDetails(ctx context.Context, store OrgUnitStore, tenantID string, details OrgUnitNodeDetails) (orgunitservices.OrgUnitReadNode, error) {
	orgNodeKey := strings.TrimSpace(details.OrgNodeKey)
	orgNodeKey, err := orgNodeKeyFromCompat(orgNodeKey, details.OrgID)
	if err != nil {
		return orgunitservices.OrgUnitReadNode{}, err
	}

	pathOrgNodeKeys := append([]string(nil), details.PathOrgNodeKeys...)
	if len(pathOrgNodeKeys) == 0 {
		pathOrgNodeKeys = []string{orgNodeKey}
	}
	pathOrgCodes, err := resolveOrgUnitPathCodes(ctx, store, tenantID, pathOrgNodeKeys)
	if err != nil {
		return orgunitservices.OrgUnitReadNode{}, err
	}
	isBU := details.IsBusinessUnit
	return orgunitservices.OrgUnitReadNode{
		OrgCode:         details.OrgCode,
		OrgNodeKey:      orgNodeKey,
		Name:            details.Name,
		Status:          orgUnitReadStatusOrActive(details.Status),
		IsBusinessUnit:  &isBU,
		PathOrgNodeKeys: pathOrgNodeKeys,
		PathOrgCodes:    pathOrgCodes,
	}, nil
}

func resolveOrgUnitPathCodes(ctx context.Context, store OrgUnitStore, tenantID string, pathOrgNodeKeys []string) ([]string, error) {
	if len(pathOrgNodeKeys) == 0 {
		return nil, nil
	}
	resolved, err := store.ResolveOrgCodesByNodeKeys(ctx, tenantID, pathOrgNodeKeys)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(pathOrgNodeKeys))
	for _, orgNodeKey := range pathOrgNodeKeys {
		code, ok := resolved[orgNodeKey]
		if !ok || strings.TrimSpace(code) == "" {
			return nil, orgunitservices.ErrOrgUnitReadSafePathUnavailable
		}
		out = append(out, code)
	}
	return out, nil
}

func orgUnitReadStatusOrActive(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return orgUnitListStatusActive
	}
	return status
}

func orgUnitReadScopeFilterFromPrincipalScopes(scopes []principalOrgScope) orgunitservices.OrgUnitReadScopeFilter {
	out := orgunitservices.OrgUnitReadScopeFilter{
		Scopes: make([]orgunitservices.OrgUnitScope, 0, len(scopes)),
	}
	for _, scope := range scopes {
		out.Scopes = append(out.Scopes, orgunitservices.OrgUnitScope{
			OrgNodeKey:         scope.OrgNodeKey,
			IncludeDescendants: scope.IncludeDescendants,
		})
	}
	return out
}

func orgUnitReadScopeFilterFromRuntime(ctx context.Context, runtime authzRuntimeStore, tenantID string) (orgunitservices.OrgUnitReadScopeFilter, error) {
	if runtime == nil {
		return orgunitservices.OrgUnitReadScopeFilter{AllTenant: true}, nil
	}
	scopes, err := currentPrincipalOrgScopes(ctx, runtime, tenantID)
	if err != nil {
		return orgunitservices.OrgUnitReadScopeFilter{}, err
	}
	return orgUnitReadScopeFilterFromPrincipalScopes(scopes), nil
}

func orgUnitListItemsFromReadNodes(nodes []orgunitservices.OrgUnitReadNode) []orgUnitListItem {
	out := make([]orgUnitListItem, 0, len(nodes))
	for _, node := range nodes {
		hasVisibleChildren := node.HasVisibleChildren
		hasChildren := node.HasVisibleChildren
		out = append(out, orgUnitListItem{
			OrgCode:            node.OrgCode,
			Name:               node.Name,
			Status:             orgUnitReadStatusOrActive(node.Status),
			IsBusinessUnit:     node.IsBusinessUnit,
			HasChildren:        &hasChildren,
			HasVisibleChildren: &hasVisibleChildren,
			OrgNodeKey:         node.OrgNodeKey,
			PathOrgNodeKeys:    append([]string(nil), node.PathOrgNodeKeys...),
		})
	}
	return out
}

func orgUnitReadNodesFromListItems(items []orgUnitListItem) []orgunitservices.OrgUnitReadNode {
	out := make([]orgunitservices.OrgUnitReadNode, 0, len(items))
	for _, item := range items {
		out = append(out, orgunitservices.OrgUnitReadNode{
			OrgCode:            item.OrgCode,
			OrgNodeKey:         item.OrgNodeKey,
			Name:               item.Name,
			Status:             orgUnitReadStatusOrActive(item.Status),
			IsBusinessUnit:     item.IsBusinessUnit,
			HasVisibleChildren: boolPtrValue(item.HasVisibleChildren, boolPtrValue(item.HasChildren, false)),
			PathOrgNodeKeys:    append([]string(nil), item.PathOrgNodeKeys...),
		})
	}
	return out
}

func boolPtrValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func orgUnitSearchResultFromReadNode(node orgunitservices.OrgUnitReadNode, asOf string) OrgUnitSearchResult {
	result := OrgUnitSearchResult{
		TargetOrgNodeKey: strings.TrimSpace(node.OrgNodeKey),
		TargetOrgCode:    strings.TrimSpace(node.OrgCode),
		TargetName:       strings.TrimSpace(node.Name),
		PathOrgNodeKeys:  append([]string(nil), node.PathOrgNodeKeys...),
		PathOrgCodes:     append([]string(nil), node.PathOrgCodes...),
		TreeAsOf:         strings.TrimSpace(asOf),
	}
	_ = hydrateOrgUnitSearchResultCompat(&result)
	return result
}
