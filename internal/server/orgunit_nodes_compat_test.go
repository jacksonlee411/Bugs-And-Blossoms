package server

import (
	"context"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func (s *orgUnitPGStore) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	orgNodeKey, err := s.ResolveOrgNodeKeyByCode(ctx, tenantID, orgCode)
	if err != nil {
		return 0, err
	}
	return decodeOrgNodeKeyToID(orgNodeKey)
}

func (s *orgUnitPGStore) ResolveOrgCode(ctx context.Context, tenantID string, orgID int) (string, error) {
	orgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		return "", err
	}
	return s.ResolveOrgCodeByNodeKey(ctx, tenantID, orgNodeKey)
}

func (s *orgUnitPGStore) ResolveOrgCodes(ctx context.Context, tenantID string, orgIDs []int) (map[int]string, error) {
	orgNodeKeys := make([]string, 0, len(orgIDs))
	keyByID := make(map[int]string, len(orgIDs))
	for _, orgID := range orgIDs {
		orgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
		if err != nil {
			return nil, err
		}
		orgNodeKeys = append(orgNodeKeys, orgNodeKey)
		keyByID[orgID] = orgNodeKey
	}
	resolved, err := s.ResolveOrgCodesByNodeKeys(ctx, tenantID, orgNodeKeys)
	if err != nil {
		return nil, err
	}
	out := make(map[int]string, len(orgIDs))
	for _, orgID := range orgIDs {
		code, ok := resolved[keyByID[orgID]]
		if !ok {
			return nil, orgunitpkg.ErrOrgNodeKeyNotFound
		}
		out[orgID] = code
	}
	return out, nil
}

func (s *orgUnitMemoryStore) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	orgNodeKey, err := s.ResolveOrgNodeKeyByCode(ctx, tenantID, orgCode)
	if err != nil {
		return 0, err
	}
	return decodeOrgNodeKeyToID(orgNodeKey)
}

func (s *orgUnitMemoryStore) ResolveOrgCode(ctx context.Context, tenantID string, orgID int) (string, error) {
	orgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		return "", err
	}
	return s.ResolveOrgCodeByNodeKey(ctx, tenantID, orgNodeKey)
}

func (s *orgUnitMemoryStore) ResolveOrgCodes(ctx context.Context, tenantID string, orgIDs []int) (map[int]string, error) {
	orgNodeKeys := make([]string, 0, len(orgIDs))
	keyByID := make(map[int]string, len(orgIDs))
	for _, orgID := range orgIDs {
		orgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
		if err != nil {
			return nil, err
		}
		orgNodeKeys = append(orgNodeKeys, orgNodeKey)
		keyByID[orgID] = orgNodeKey
	}
	resolved, err := s.ResolveOrgCodesByNodeKeys(ctx, tenantID, orgNodeKeys)
	if err != nil {
		return nil, err
	}
	out := make(map[int]string, len(orgIDs))
	for _, orgID := range orgIDs {
		code, ok := resolved[keyByID[orgID]]
		if !ok {
			return nil, orgunitpkg.ErrOrgNodeKeyNotFound
		}
		out[orgID] = code
	}
	return out, nil
}
