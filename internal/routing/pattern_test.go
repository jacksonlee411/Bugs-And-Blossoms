package routing

import "testing"

func TestParsePathPattern(t *testing.T) {
	t.Parallel()

	if _, ok := parsePathPattern("/health"); ok {
		t.Fatal("expected non-pattern")
	}
	if _, ok := parsePathPattern("no-leading-slash"); ok {
		t.Fatal("expected invalid")
	}
	if _, ok := parsePathPattern("{no-leading-slash-but-has-brace}"); ok {
		t.Fatal("expected invalid")
	}
	if _, ok := parsePathPattern("/a/{id"); ok {
		t.Fatal("expected invalid")
	}
	if _, ok := parsePathPattern("/a/{}/b"); ok {
		t.Fatal("expected invalid")
	}
	if _, ok := parsePathPattern("/a/{id}x/b"); ok {
		t.Fatal("expected invalid")
	}
	if _, ok := parsePathPattern("/a/id}/b"); ok {
		t.Fatal("expected invalid")
	}
	if _, ok := parsePathPattern("/a//{id}/b"); ok {
		t.Fatal("expected invalid (empty segment)")
	}

	p, ok := parsePathPattern("/a/{id}/b")
	if !ok {
		t.Fatal("expected ok")
	}
	if (PathPattern{}).Match("/a/x/b") {
		t.Fatal("expected zero-value to not match")
	}
	if !p.Match("/a/x/b") {
		t.Fatal("expected match")
	}
	if p.Match("/a/x/c") {
		t.Fatal("expected no match")
	}
	if p.Match("/a/x") {
		t.Fatal("expected no match")
	}
	if p.Match("/a//b") {
		t.Fatal("expected no match for empty segment")
	}
}

func TestSplitPathSegments(t *testing.T) {
	t.Parallel()

	if got := splitPathSegments("/"); got != nil {
		t.Fatalf("got=%v", got)
	}
	got := splitPathSegments("/a/b")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got=%v", got)
	}
}
