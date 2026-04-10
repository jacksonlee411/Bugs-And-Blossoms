package persistence

import (
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestIsOrgUnitID8(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "valid", input: "12345678", want: true},
		{name: "short", input: "1234567", want: false},
		{name: "long", input: "123456789", want: false},
		{name: "contains non digit", input: "1234ab78", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOrgUnitID8(tc.input); got != tc.want {
				t.Fatalf("got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestNormalizePositionOrgUnitID(t *testing.T) {
	t.Run("accepts org id", func(t *testing.T) {
		got, err := normalizePositionOrgUnitID("10000001")
		if err != nil {
			t.Fatal(err)
		}
		if got != "10000001" {
			t.Fatalf("got=%q want=%q", got, "10000001")
		}
	})

	t.Run("accepts org node key", func(t *testing.T) {
		orgNodeKey, err := orgunitpkg.EncodeOrgNodeKey(10000001)
		if err != nil {
			t.Fatal(err)
		}
		got, err := normalizePositionOrgUnitID(orgNodeKey)
		if err != nil {
			t.Fatal(err)
		}
		if got != "10000001" {
			t.Fatalf("got=%q want=%q", got, "10000001")
		}
	})

	t.Run("rejects invalid input", func(t *testing.T) {
		_, err := normalizePositionOrgUnitID("bad")
		if err == nil {
			t.Fatal("expected error")
		}
		if !httperr.IsBadRequest(err) {
			t.Fatalf("expected bad request, got %v", err)
		}
	})
}
