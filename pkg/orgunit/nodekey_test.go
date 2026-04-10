package orgunit_test

import (
	"errors"
	"strconv"
	"testing"

	orgunit "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestNormalizeOrgNodeKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "trim and uppercase", input: " aaaaaaab ", want: "AAAAAAAB"},
		{name: "empty rejected", input: "", wantErr: orgunit.ErrOrgNodeKeyInvalid},
		{name: "too short rejected", input: "AAAAAAA", wantErr: orgunit.ErrOrgNodeKeyInvalid},
		{name: "digit first rejected", input: "2AAAAAAA", wantErr: orgunit.ErrOrgNodeKeyInvalid},
		{name: "invalid char rejected", input: "AAAAAAAI", wantErr: orgunit.ErrOrgNodeKeyInvalid},
		{name: "whitespace rejected", input: "AAAA AAAB", wantErr: orgunit.ErrOrgNodeKeyInvalid},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := orgunit.NormalizeOrgNodeKey(tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("NormalizeOrgNodeKey(%q) error = %v, want %v", tc.input, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("NormalizeOrgNodeKey(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestEncodeOrgNodeKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		seq  int64
		want string
	}{
		{seq: 1, want: "AAAAAAAB"},
		{seq: 2, want: "AAAAAAAC"},
		{seq: 3, want: "AAAAAAAD"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()

			got, err := orgunit.EncodeOrgNodeKey(tc.seq)
			if err != nil {
				t.Fatalf("EncodeOrgNodeKey(%d) error = %v", tc.seq, err)
			}
			if got != tc.want {
				t.Fatalf("EncodeOrgNodeKey(%d) = %q, want %q", tc.seq, got, tc.want)
			}
		})
	}
}

func TestEncodeDecodeOrgNodeKeyRoundTrip(t *testing.T) {
	t.Parallel()

	for _, seq := range []int64{1, 2, 31, 32, 1024, 1 << 20, (1 << 35) - 1} {
		seq := seq
		t.Run(strconv.FormatInt(seq, 10), func(t *testing.T) {
			t.Parallel()

			key, err := orgunit.EncodeOrgNodeKey(seq)
			if err != nil {
				t.Fatalf("EncodeOrgNodeKey(%d) error = %v", seq, err)
			}
			got, err := orgunit.DecodeOrgNodeKey(key)
			if err != nil {
				t.Fatalf("DecodeOrgNodeKey(%q) error = %v", key, err)
			}
			if got != seq {
				t.Fatalf("DecodeOrgNodeKey(%q) = %d, want %d", key, got, seq)
			}
		})
	}
}

func TestDecodeOrgNodeKeyRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "1AAAAAAA", "AAAAAAAI", "AAAA AAA"} {
		if _, err := orgunit.DecodeOrgNodeKey(input); !errors.Is(err, orgunit.ErrOrgNodeKeyInvalid) {
			t.Fatalf("DecodeOrgNodeKey(%q) error = %v, want %v", input, err, orgunit.ErrOrgNodeKeyInvalid)
		}
	}
}
