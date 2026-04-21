package server

import (
	"context"
	"errors"
	"testing"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func mustOrgNodeKeyForTest(t *testing.T, orgID int) string {
	t.Helper()
	key, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		t.Fatalf("encodeOrgNodeKeyFromID(%d): %v", orgID, err)
	}
	return key
}

type setIDExplainOrgResolverStub struct {
	orgUnitCodeResolverStub
	byCode map[string]string
	err    error
}

func (s setIDExplainOrgResolverStub) ResolveOrgNodeKeyByCode(_ context.Context, _ string, orgCode string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	normalized, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return "", err
	}
	if s.byCode == nil {
		return "", errors.New("org_code_resolver_missing")
	}
	orgNodeKey, ok := s.byCode[normalized]
	if !ok {
		return "", orgunitpkg.ErrOrgCodeNotFound
	}
	return orgNodeKey, nil
}
