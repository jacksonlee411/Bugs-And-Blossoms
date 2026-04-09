package persistence

import "testing"

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
