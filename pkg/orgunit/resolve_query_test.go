package orgunit

import (
	"strings"
	"testing"
)

func TestResolveQueriesStayCompatibleAcrossOrgUnitCodesLayouts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		query     string
		fragments []string
	}{
		{
			name:  "resolve node key by code",
			query: resolveOrgNodeKeyByCodeQuery,
			fragments: []string{
				"to_jsonb(c) ? 'org_node_key'",
				"to_jsonb(c)->>'org_id'",
				"orgunit.encode_org_node_key",
			},
		},
		{
			name:  "resolve code by node key",
			query: resolveOrgCodeByNodeKeyQuery,
			fragments: []string{
				"to_jsonb(c) ? 'org_node_key'",
				"to_jsonb(c)->>'org_id'",
				"orgunit.decode_org_node_key",
			},
		},
		{
			name:  "resolve codes by node keys",
			query: resolveOrgCodesByNodeKeysQuery,
			fragments: []string{
				"to_jsonb(c) ? 'org_node_key'",
				"to_jsonb(c)->>'org_id'",
				"orgunit.decode_org_node_key",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, fragment := range tc.fragments {
				if !strings.Contains(tc.query, fragment) {
					t.Fatalf("query missing fragment %q\n%s", fragment, tc.query)
				}
			}
		})
	}
}
