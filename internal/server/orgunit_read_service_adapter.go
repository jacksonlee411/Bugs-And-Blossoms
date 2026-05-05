package server

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitListPageRequest struct {
	AsOf              string
	IncludeDisabled   bool
	ScopeFilter       orgunitservices.OrgUnitReadScopeFilter
	ParentOrgNodeKey  *string
	AllOrgUnits       bool
	Keyword           string
	Status            string // "", "active", "disabled"
	IsBusinessUnit    *bool
	SortField         string
	ExtSortFieldKey   string
	SortOrder         string
	ExtFilterFieldKey string
	ExtFilterValue    string
	Limit             int
	Offset            int
}

func (r orgUnitListPageRequest) ShouldSearchAllOrgUnits() bool {
	return r.ParentOrgNodeKey == nil && (r.AllOrgUnits || strings.TrimSpace(r.Keyword) != "" || r.IsBusinessUnit != nil)
}

type orgUnitListPageReader interface {
	ListOrgUnitsPage(ctx context.Context, tenantID string, req orgUnitListPageRequest) ([]orgUnitListItem, int, error)
}

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

func (a orgUnitReadStoreAdapter) ListPage(ctx context.Context, req orgunitservices.OrgUnitReadListPageRequest) ([]orgunitservices.OrgUnitReadNode, int, error) {
	if strings.TrimSpace(req.ExtFilterFieldKey) != "" || strings.TrimSpace(req.ExtSortFieldKey) != "" {
		if _, ok := a.store.(orgUnitListPageReader); !ok {
			return nil, 0, orgunitservices.ErrOrgUnitReadExtQueryNotAllowed
		}
	}

	var parentOrgNodeKey *string
	if strings.TrimSpace(req.ParentOrgNodeKey) != "" {
		normalized, err := normalizeOrgNodeKeyInput(req.ParentOrgNodeKey)
		if err != nil {
			return nil, 0, err
		}
		parentOrgNodeKey = &normalized
	} else if strings.TrimSpace(req.ParentOrgCode) != "" {
		normalized, err := orgunitpkg.NormalizeOrgCode(req.ParentOrgCode)
		if err != nil {
			return nil, 0, err
		}
		resolved, err := a.store.ResolveOrgNodeKeyByCode(ctx, req.TenantID, normalized)
		if err != nil {
			if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
				return nil, 0, orgunitservices.ErrOrgUnitReadNotFound
			}
			return nil, 0, err
		}
		parentOrgNodeKey = &resolved
	}

	items, total, err := listOrgUnitListPage(ctx, a.store, req.TenantID, orgUnitListPageRequest{
		AsOf:              req.AsOf,
		IncludeDisabled:   req.IncludeDisabled,
		ScopeFilter:       req.ScopeFilter,
		ParentOrgNodeKey:  parentOrgNodeKey,
		AllOrgUnits:       req.AllOrgUnits,
		Keyword:           req.Keyword,
		Status:            req.Status,
		IsBusinessUnit:    req.IsBusinessUnit,
		SortField:         req.SortField,
		ExtSortFieldKey:   req.ExtSortFieldKey,
		SortOrder:         req.SortOrder,
		ExtFilterFieldKey: req.ExtFilterFieldKey,
		ExtFilterValue:    req.ExtFilterValue,
		Limit:             req.Limit,
		Offset:            req.Offset,
	})
	if err != nil {
		if errors.Is(err, errOrgUnitExtQueryFieldNotAllowed) {
			return nil, 0, orgunitservices.ErrOrgUnitReadExtQueryNotAllowed
		}
		if errors.Is(err, errOrgUnitNotFound) || errors.Is(err, orgunitpkg.ErrOrgNodeKeyNotFound) {
			return nil, 0, orgunitservices.ErrOrgUnitReadNotFound
		}
		return nil, 0, err
	}
	if err := hydrateOrgUnitListItemScopePaths(ctx, a.store, req.TenantID, req.AsOf, items); err != nil {
		if errors.Is(err, errOrgUnitNotFound) || errors.Is(err, orgunitpkg.ErrOrgNodeKeyNotFound) {
			return nil, 0, orgunitservices.ErrOrgUnitReadNotFound
		}
		return nil, 0, err
	}
	return orgUnitReadNodesFromListItems(items), total, nil
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

func listOrgUnitListPage(ctx context.Context, store OrgUnitStore, tenantID string, req orgUnitListPageRequest) ([]orgUnitListItem, int, error) {
	if pager, ok := store.(orgUnitListPageReader); ok {
		return pager.ListOrgUnitsPage(ctx, tenantID, req)
	}

	var items []orgUnitListItem
	if req.ParentOrgNodeKey != nil {
		children, err := listChildrenByVisibilityByNodeKey(ctx, store, tenantID, *req.ParentOrgNodeKey, req.AsOf, req.IncludeDisabled)
		if err != nil {
			return nil, 0, err
		}
		items = make([]orgUnitListItem, 0, len(children))
		for _, child := range children {
			isBU := child.IsBusinessUnit
			hasChildren := child.HasChildren
			status := strings.TrimSpace(child.Status)
			if status == "" {
				status = orgUnitListStatusActive
			}
			orgNodeKey, err := orgNodeKeyFromCompat(child.OrgNodeKey, child.OrgID)
			if err != nil {
				return nil, 0, err
			}
			items = append(items, orgUnitListItem{
				OrgCode:        child.OrgCode,
				Name:           child.Name,
				Status:         status,
				IsBusinessUnit: &isBU,
				HasChildren:    &hasChildren,
				OrgNodeKey:     orgNodeKey,
			})
		}
	} else {
		nodes, err := listNodesCurrentByVisibility(ctx, store, tenantID, req.AsOf, req.IncludeDisabled)
		if err != nil {
			return nil, 0, err
		}
		items = make([]orgUnitListItem, 0, len(nodes))
		for _, node := range nodes {
			isBU := node.IsBusinessUnit
			hasChildren := node.HasChildren
			status := strings.TrimSpace(node.Status)
			if status == "" {
				status = orgUnitListStatusActive
			}
			orgNodeKey, err := orgNodeKeyFromCompat(node.ID, node.OrgID)
			if err != nil {
				return nil, 0, err
			}
			items = append(items, orgUnitListItem{
				OrgCode:        node.OrgCode,
				Name:           node.Name,
				Status:         status,
				IsBusinessUnit: &isBU,
				HasChildren:    &hasChildren,
				OrgNodeKey:     orgNodeKey,
			})
		}
	}
	items = filterOrgUnitListItems(items, req.Keyword, req.Status, req.IsBusinessUnit)
	if req.SortField != "" {
		sortOrgUnitListItems(items, req.SortField, req.SortOrder)
	}
	if err := hydrateOrgUnitListItemScopePaths(ctx, store, tenantID, req.AsOf, items); err != nil {
		return nil, 0, err
	}
	items = filterOrgUnitListItemsByReadScope(items, req.ScopeFilter)

	total := len(items)
	if req.Limit <= 0 {
		return items, total, nil
	}

	start := max(req.Offset, 0)
	if start >= total {
		return []orgUnitListItem{}, total, nil
	}

	end := min(start+req.Limit, total)

	return items[start:end], total, nil
}

func filterOrgUnitListItems(items []orgUnitListItem, keyword string, status string, isBusinessUnit *bool) []orgUnitListItem {
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))

	if normalizedKeyword == "" && normalizedStatus == "" && isBusinessUnit == nil {
		return items
	}

	out := make([]orgUnitListItem, 0, len(items))
	for _, item := range items {
		itemStatus := strings.ToLower(strings.TrimSpace(item.Status))
		if itemStatus == "" {
			itemStatus = orgUnitListStatusActive
		}

		if normalizedStatus != "" && itemStatus != normalizedStatus {
			continue
		}

		if normalizedKeyword != "" {
			code := strings.ToLower(item.OrgCode)
			name := strings.ToLower(item.Name)
			if !strings.Contains(code, normalizedKeyword) && !strings.Contains(name, normalizedKeyword) {
				continue
			}
		}

		if isBusinessUnit != nil {
			if item.IsBusinessUnit == nil || *item.IsBusinessUnit != *isBusinessUnit {
				continue
			}
		}

		out = append(out, item)
	}

	return out
}

func filterOrgUnitListItemsByReadScope(items []orgUnitListItem, filter orgunitservices.OrgUnitReadScopeFilter) []orgUnitListItem {
	// DEV-PLAN-492 adapter bridge: listOrgUnitListPage still returns API list DTOs for
	// orgUnitReadStoreAdapter.ListPage; handler scope checks must use OrgUnitReadService.
	if filter.AllTenant || len(filter.Scopes) == 0 {
		return items
	}
	out := make([]orgUnitListItem, 0, len(items))
	for _, item := range items {
		if orgUnitReadScopeAllowsListItem(filter, item) {
			out = append(out, item)
		}
	}
	return out
}

func orgUnitReadScopeAllowsListItem(filter orgunitservices.OrgUnitReadScopeFilter, item orgUnitListItem) bool {
	// DEV-PLAN-492 adapter bridge: path-based matching is retained only for
	// list DTOs that have already been hydrated before conversion to read nodes.
	orgNodeKey := strings.TrimSpace(item.OrgNodeKey)
	if orgNodeKey == "" {
		return false
	}
	path := append([]string(nil), item.PathOrgNodeKeys...)
	if len(path) == 0 || strings.TrimSpace(path[len(path)-1]) != orgNodeKey {
		path = append(path, orgNodeKey)
	}
	for _, scope := range filter.Scopes {
		boundOrgNodeKey := strings.TrimSpace(scope.OrgNodeKey)
		if boundOrgNodeKey == "" {
			continue
		}
		if orgNodeKey == boundOrgNodeKey {
			return true
		}
		if !scope.IncludeDescendants {
			continue
		}
		for _, pathOrgNodeKey := range path {
			if strings.TrimSpace(pathOrgNodeKey) == boundOrgNodeKey {
				return true
			}
		}
	}
	return false
}

func sortOrgUnitListItems(items []orgUnitListItem, sortField string, sortOrder string) {
	normalizedField := strings.ToLower(strings.TrimSpace(sortField))
	desc := strings.EqualFold(strings.TrimSpace(sortOrder), orgUnitListSortOrderDesc)

	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]

		var cmp int
		switch normalizedField {
		case orgUnitListSortName:
			cmp = strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
		case orgUnitListSortStatus:
			cmp = strings.Compare(strings.ToLower(left.Status), strings.ToLower(right.Status))
		case orgUnitListSortCode:
			fallthrough
		default:
			cmp = strings.Compare(strings.ToLower(left.OrgCode), strings.ToLower(right.OrgCode))
		}

		// Stable tie-breaker.
		if cmp == 0 {
			cmp = strings.Compare(strings.ToLower(left.OrgCode), strings.ToLower(right.OrgCode))
		}

		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func hydrateOrgUnitListItemScopePaths(ctx context.Context, store OrgUnitStore, tenantID string, asOf string, items []orgUnitListItem) error {
	// DEV-PLAN-492 adapter bridge: hydrate path data for list DTOs before the
	// orgUnitReadStoreAdapter converts them into OrgUnitReadNode values.
	for i := range items {
		if len(items[i].PathOrgNodeKeys) > 0 {
			continue
		}
		if strings.TrimSpace(items[i].OrgNodeKey) == "" {
			continue
		}
		if store == nil {
			items[i].PathOrgNodeKeys = []string{items[i].OrgNodeKey}
			continue
		}
		targetOrgNodeKey, pathOrgNodeKeys, err := orgUnitScopePathOrgNodeKeys(ctx, store, tenantID, items[i].OrgNodeKey, asOf)
		if err != nil {
			return err
		}
		items[i].OrgNodeKey = targetOrgNodeKey
		items[i].PathOrgNodeKeys = pathOrgNodeKeys
	}
	return nil
}

func orgUnitScopePathOrgNodeKeys(ctx context.Context, store OrgUnitStore, tenantID string, orgNodeKey string, asOf string) (string, []string, error) {
	// DEV-PLAN-492 adapter bridge: this exists only to hydrate list DTO paths for
	// orgUnitReadStoreAdapter. Handler/authz visibility checks must resolve
	// through OrgUnitReadService instead of using this path data directly.
	orgNodeKey = strings.TrimSpace(orgNodeKey)
	if orgNodeKey == "" {
		return "", nil, errAuthzScopeForbidden
	}
	if store == nil {
		return "", nil, errAuthzRuntimeUnavailable
	}
	targetAsOf := strings.TrimSpace(asOf)
	if targetAsOf == "" {
		maxAsOf, ok, err := store.MaxEffectiveDateOnOrBefore(ctx, tenantID, time.Now().UTC().Format(asOfLayout))
		if err != nil {
			return "", nil, err
		}
		if ok {
			targetAsOf = maxAsOf
		}
	}
	if targetAsOf == "" {
		return orgNodeKey, []string{orgNodeKey}, nil
	}
	details, err := getNodeDetailsByVisibilityByNodeKey(ctx, store, tenantID, orgNodeKey, targetAsOf, true)
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			return orgNodeKey, []string{orgNodeKey}, nil
		}
		return "", nil, err
	}
	targetOrgNodeKey := strings.TrimSpace(details.OrgNodeKey)
	if targetOrgNodeKey == "" {
		targetOrgNodeKey = orgNodeKey
	}
	pathOrgNodeKeys := append([]string(nil), details.PathOrgNodeKeys...)
	if len(pathOrgNodeKeys) == 0 {
		pathOrgNodeKeys = []string{targetOrgNodeKey}
	}
	return targetOrgNodeKey, pathOrgNodeKeys, nil
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
		if errors.Is(err, errAuthzOrgScopeRequired) {
			return orgunitservices.OrgUnitReadScopeFilter{}, nil
		}
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
