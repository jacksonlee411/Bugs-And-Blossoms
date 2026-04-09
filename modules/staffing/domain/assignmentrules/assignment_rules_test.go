package assignmentrules

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCanonicalizeJSONBranches(t *testing.T) {
	t.Run("map and array stay ordered", func(t *testing.T) {
		var b strings.Builder
		input := map[string]any{
			"b": []any{"x", true, nil},
			"a": map[string]any{
				"z": "tail",
				"m": json.Number("7"),
			},
		}
		if err := canonicalizeJSON(&b, input); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := b.String(); got != `{"a":{"m":7,"z":"tail"},"b":["x",true,null]}` {
			t.Fatalf("got=%q", got)
		}
	})

	t.Run("json number", func(t *testing.T) {
		var b strings.Builder
		if err := canonicalizeJSON(&b, json.Number("12.5")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := b.String(); got != "12.5" {
			t.Fatalf("got=%q", got)
		}
	})

	t.Run("default branch returns marshal error", func(t *testing.T) {
		var b strings.Builder
		if err := canonicalizeJSON(&b, make(chan int)); err == nil {
			t.Fatal("expected marshal error")
		}
	})
}
