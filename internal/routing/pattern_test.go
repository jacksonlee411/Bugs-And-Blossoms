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
	if p, ok := parsePathPattern("/a/{id}x/b"); !ok {
		t.Fatal("expected suffix pattern ok")
	} else if !p.Match("/a/123x/b") {
		t.Fatal("expected suffix literal after param to match")
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

	actionPattern, ok := parsePathPattern("/a/{id}:confirm")
	if !ok {
		t.Fatal("expected suffix pattern ok")
	}
	if !actionPattern.Match("/a/turn-1:confirm") {
		t.Fatal("expected suffix pattern match")
	}
	if actionPattern.Match("/a/:confirm") {
		t.Fatal("expected missing param value to fail")
	}
	if actionPattern.Match("/a/turn-1:commit") {
		t.Fatal("expected suffix mismatch")
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

func TestSplitPatternSegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input      string
		wantPrefix string
		wantSuffix string
		wantOK     bool
	}{
		{input: "x{id}y", wantPrefix: "x", wantSuffix: "y", wantOK: true},
		{input: "x{id}y{z}", wantOK: false},
		{input: "x}y{id}z", wantOK: false},
		{input: "a{i{d}b", wantOK: false},
		{input: "a{id}b}", wantOK: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			gotPrefix, gotSuffix, gotOK := splitPatternSegment(tt.input)
			if gotPrefix != tt.wantPrefix || gotSuffix != tt.wantSuffix || gotOK != tt.wantOK {
				t.Fatalf("splitPatternSegment(%q)=(%q,%q,%v) want (%q,%q,%v)", tt.input, gotPrefix, gotSuffix, gotOK, tt.wantPrefix, tt.wantSuffix, tt.wantOK)
			}
		})
	}
}
