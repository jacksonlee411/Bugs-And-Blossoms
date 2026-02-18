package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrgunitEngine_StickyEffectiveDateIsAppliedInViewAndReplay(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	// DEV-PLAN-106B: sticky effective_date must be computed independently from the latest correction payload.
	for _, token := range []string{
		"latest_effective_date_corrections AS",
		"NULLIF(btrim(payload->>'effective_date'), '')::date AS sticky_effective_date",
		"COALESCE(lec.sticky_effective_date, e.effective_date) AS effective_date",
		"COALESCE(lec.sticky_effective_date, se.effective_date) AS effective_date",
		"LEFT JOIN latest_effective_date_corrections lec",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, p)
		}
	}
}
