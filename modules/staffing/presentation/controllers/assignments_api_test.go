package controllers

import "testing"

func TestIsStableDBCode(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{in: "", want: false},
		{in: "UNKNOWN", want: false},
		{in: "  UNKNOWN  ", want: false},
		{in: "STAFFING_IDEMPOTENCY_REUSED", want: true},
		{in: "A", want: true},
		{in: "1ABC", want: false},
		{in: "AbC", want: false},
		{in: "ABC-def", want: false},
		{in: "A BC", want: false},
	}
	for _, tc := range cases {
		if got := isStableDBCode(tc.in); got != tc.want {
			t.Fatalf("in=%q got=%v want=%v", tc.in, got, tc.want)
		}
	}
}
