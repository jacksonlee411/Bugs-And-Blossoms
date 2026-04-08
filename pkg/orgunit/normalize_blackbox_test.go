package orgunit_test

import (
	"strings"
	"testing"

	orgunit "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestNormalizeOrgCode_BlackBox(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "uppercase ascii", input: "root", want: "ROOT"},
		{name: "preserve spaces inside", input: "root a", want: "ROOT A"},
		{name: "fullwidth letters uppercased", input: "ａｂｃ", want: "ＡＢＣ"},
		{name: "empty rejected", input: "", wantErr: orgunit.ErrOrgCodeInvalid},
		{name: "whitespace only rejected", input: " ", wantErr: orgunit.ErrOrgCodeInvalid},
		{name: "tab whitespace only rejected", input: "\t", wantErr: orgunit.ErrOrgCodeInvalid},
		{name: "newline rejected", input: "bad\ncode", wantErr: orgunit.ErrOrgCodeInvalid},
		{name: "control char rejected", input: "bad\x7f", wantErr: orgunit.ErrOrgCodeInvalid},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := orgunit.NormalizeOrgCode(tc.input)
			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Fatalf("want err %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err=%v", err)
			}
			if got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func FuzzNormalizeOrgCode_BlackBox(f *testing.F) {
	seeds := []string{
		"root",
		"root a",
		"ＡＢＣ",
		" ",
		"\t",
		"bad\ncode",
		"bad\x7f",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		got, err := orgunit.NormalizeOrgCode(input)
		if err != nil {
			return
		}
		if got == "" {
			t.Fatal("normalized org code should not be empty")
		}
		if got != strings.ToUpper(got) {
			t.Fatalf("normalized org code should be uppercase, got %q", got)
		}
		got2, err := orgunit.NormalizeOrgCode(got)
		if err != nil {
			t.Fatalf("normalized output should stay valid, err=%v", err)
		}
		if got2 != got {
			t.Fatalf("normalize should be idempotent, got %q then %q", got, got2)
		}
	})
}

func BenchmarkNormalizeOrgCode_BlackBox(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := orgunit.NormalizeOrgCode("root a"); err != nil {
			b.Fatalf("unexpected err=%v", err)
		}
	}
}
