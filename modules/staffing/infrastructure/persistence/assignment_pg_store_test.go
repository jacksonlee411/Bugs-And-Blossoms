package persistence

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCanonicalizeJSONObjectRaw(t *testing.T) {
	t.Run("empty -> bad request", func(t *testing.T) {
		_, err := canonicalizeJSONObjectRaw(nil)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid json -> bad request", func(t *testing.T) {
		_, err := canonicalizeJSONObjectRaw(json.RawMessage("{bad"))
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("non-object -> bad request", func(t *testing.T) {
		_, err := canonicalizeJSONObjectRaw(json.RawMessage(`[1,2,3]`))
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("mixed types canonicalize", func(t *testing.T) {
		raw := json.RawMessage(`{"b":true,"a":null,"c":"x","d":1,"e":[2,3]}`)
		got, err := canonicalizeJSONObjectRaw(raw)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if string(got) != `{"a":null,"b":true,"c":"x","d":1,"e":[2,3]}` {
			t.Fatalf("got=%s", got)
		}
	})
}

func TestCanonicalizeJSON_DefaultBranches(t *testing.T) {
	t.Run("default marshal ok", func(t *testing.T) {
		var b strings.Builder
		if err := canonicalizeJSON(&b, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("err=%v", err)
		}
		if b.String() == "" {
			t.Fatalf("expected non-empty output")
		}
	})

	t.Run("default marshal error", func(t *testing.T) {
		var b strings.Builder
		ch := make(chan int)
		if err := canonicalizeJSON(&b, ch); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("map propagates nested error", func(t *testing.T) {
		var b strings.Builder
		ch := make(chan int)
		if err := canonicalizeJSON(&b, map[string]any{"x": ch}); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("array propagates nested error", func(t *testing.T) {
		var b strings.Builder
		ch := make(chan int)
		if err := canonicalizeJSON(&b, []any{ch}); err == nil {
			t.Fatalf("expected error")
		}
	})
}
