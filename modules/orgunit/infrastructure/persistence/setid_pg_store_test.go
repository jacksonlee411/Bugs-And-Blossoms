package persistence

import "testing"

func TestParseOrgID8SetID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "trimmed valid", input: " 12345678 ", want: 12345678},
		{name: "empty", input: "   ", wantErr: true},
		{name: "short", input: "123", wantErr: true},
		{name: "non digit", input: "1234ab78", wantErr: true},
		{name: "leading zero becomes out of range", input: "01234567", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOrgID8SetID(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got value=%d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got=%d want=%d", got, tc.want)
			}
		})
	}
}
