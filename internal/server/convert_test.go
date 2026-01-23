package server

import (
	"encoding/json"
	"testing"
)

type testStringer struct{}

func (testStringer) String() string { return "stringer" }

func TestToString(t *testing.T) {
	if got := toString("ok"); got != "ok" {
		t.Fatalf("string=%q", got)
	}
	if got := toString(json.Number("42")); got != "42" {
		t.Fatalf("json.Number=%q", got)
	}
	if got := toString(testStringer{}); got != "stringer" {
		t.Fatalf("stringer=%q", got)
	}
	if got := toString(123); got != "123" {
		t.Fatalf("default=%q", got)
	}
}
