package persistence

import (
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestNormalizePositionOrgNodeKey(t *testing.T) {
	t.Run("accepts org node key", func(t *testing.T) {
		orgNodeKey, err := orgunitpkg.EncodeOrgNodeKey(10000001)
		if err != nil {
			t.Fatal(err)
		}
		got, err := normalizePositionOrgNodeKey(orgNodeKey)
		if err != nil {
			t.Fatal(err)
		}
		if got != orgNodeKey {
			t.Fatalf("got=%q want=%q", got, orgNodeKey)
		}
	})

	t.Run("rejects org id digits", func(t *testing.T) {
		_, err := normalizePositionOrgNodeKey("10000001")
		if err == nil {
			t.Fatal("expected error")
		}
		if !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})

	t.Run("rejects invalid input", func(t *testing.T) {
		_, err := normalizePositionOrgNodeKey("bad")
		if err == nil {
			t.Fatal("expected error")
		}
		if !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})
}
