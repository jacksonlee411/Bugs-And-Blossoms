package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCanonicalizeJSONObjectRaw(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, err := canonicalizeJSONObjectRaw(nil)
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request error, got %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		_, err := canonicalizeJSONObjectRaw(json.RawMessage("{bad"))
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request error, got %v", err)
		}
	})

	t.Run("not an object", func(t *testing.T) {
		_, err := canonicalizeJSONObjectRaw(json.RawMessage(`[]`))
		if err == nil || !isBadRequestError(err) {
			t.Fatalf("expected bad request error, got %v", err)
		}
	})

	t.Run("ok canonical order", func(t *testing.T) {
		got, err := canonicalizeJSONObjectRaw(json.RawMessage(`{"b":1,"a":2}`))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != `{"a":2,"b":1}` {
			t.Fatalf("got=%s", got)
		}
	})
}

func TestCanonicalizeJSONObjectOrEmpty(t *testing.T) {
	t.Run("empty => {}", func(t *testing.T) {
		got, err := canonicalizeJSONObjectOrEmpty(nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != `{}` {
			t.Fatalf("got=%s", got)
		}
	})

	t.Run("null => {}", func(t *testing.T) {
		got, err := canonicalizeJSONObjectOrEmpty(json.RawMessage(`null`))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != `{}` {
			t.Fatalf("got=%s", got)
		}
	})
}

func TestCanonicalizeJSON(t *testing.T) {
	t.Run("covers primitives, arrays, numbers, default", func(t *testing.T) {
		var b strings.Builder
		v := map[string]any{
			"z": []any{json.Number("10"), true, nil, "s"},
			"a": map[string]any{
				"b": json.Number("2"),
				"a": 3,
			},
		}
		if err := canonicalizeJSON(&b, v); err != nil {
			t.Fatal(err)
		}
		if got := b.String(); got != `{"a":{"a":3,"b":2},"z":[10,true,null,"s"]}` {
			t.Fatalf("got=%s", got)
		}
	})

	t.Run("returns error on unsupported value", func(t *testing.T) {
		var b strings.Builder
		err := canonicalizeJSON(&b, map[string]any{"x": func() {}})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("propagates error from array element", func(t *testing.T) {
		var b strings.Builder
		err := canonicalizeJSON(&b, []any{func() {}})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
