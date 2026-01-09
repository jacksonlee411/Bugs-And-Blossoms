package iit

import "testing"

func TestStandardDeductionCents(t *testing.T) {
	t.Run("month count", func(t *testing.T) {
		got, err := StandardDeductionCents(3, 3)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got != 5000*100 {
			t.Fatalf("got=%d", got)
		}

		got, err = StandardDeductionCents(3, 5)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got != 5000*100*3 {
			t.Fatalf("got=%d", got)
		}
	})

	t.Run("reject invalid month", func(t *testing.T) {
		if _, err := StandardDeductionCents(0, 1); err == nil {
			t.Fatalf("expected error")
		}
		if _, err := StandardDeductionCents(1, 13); err == nil {
			t.Fatalf("expected error")
		}
		if _, err := StandardDeductionCents(5, 4); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestBracketBoundaries(t *testing.T) {
	cases := []struct {
		name               string
		taxableIncomeCents int64
		wantRate           int64
		wantQuickCents     int64
	}{
		{"<=36000", 36_000 * 100, 3, 0},
		{">36000", 36_000*100 + 1, 10, 2_520 * 100},
		{"<=144000", 144_000 * 100, 10, 2_520 * 100},
		{">144000", 144_000*100 + 1, 20, 16_920 * 100},
		{"<=300000", 300_000 * 100, 20, 16_920 * 100},
		{">300000", 300_000*100 + 1, 25, 31_920 * 100},
		{"<=420000", 420_000 * 100, 25, 31_920 * 100},
		{">420000", 420_000*100 + 1, 30, 52_920 * 100},
		{"<=660000", 660_000 * 100, 30, 52_920 * 100},
		{">660000", 660_000*100 + 1, 35, 85_920 * 100},
		{"<=960000", 960_000 * 100, 35, 85_920 * 100},
		{">960000", 960_000*100 + 1, 45, 181_920 * 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRate, gotQuick := bracketForTaxableIncome(tc.taxableIncomeCents)
			if gotRate != tc.wantRate || gotQuick != tc.wantQuickCents {
				t.Fatalf("rate=%d quick=%d", gotRate, gotQuick)
			}
		})
	}
}

func TestComputeCumulativeWithholding_Credit(t *testing.T) {
	in := CumulativeInput{
		IncomeCents:                     10_000 * 100,
		TaxExemptIncomeCents:            0,
		StandardDeductionCents:          5_000 * 100,
		SpecialDeductionCents:           0,
		SpecialAdditionalDeductionCents: 10_000 * 100,
		WithheldCents:                   0,
	}

	out, err := ComputeCumulativeWithholding(in)
	if err != nil {
		t.Fatalf("err=%v", err)
	}

	if out.TaxableIncomeCents != 0 {
		t.Fatalf("taxable=%d", out.TaxableIncomeCents)
	}
	if out.TaxLiabilityCents != 0 {
		t.Fatalf("liability=%d", out.TaxLiabilityCents)
	}
	if out.WithholdThisMonthCents != 0 {
		t.Fatalf("withhold=%d", out.WithholdThisMonthCents)
	}
	if out.CreditCents != 0 {
		t.Fatalf("credit=%d (expected 0 because withheld=0 and liability=0)", out.CreditCents)
	}

	in.WithheldCents = 123
	out, err = ComputeCumulativeWithholding(in)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if out.CreditCents != 123 || out.WithholdThisMonthCents != 0 {
		t.Fatalf("credit=%d withhold=%d", out.CreditCents, out.WithholdThisMonthCents)
	}
}

func TestComputeCumulativeWithholding_Simple(t *testing.T) {
	in := CumulativeInput{
		IncomeCents:                     20_000 * 100,
		TaxExemptIncomeCents:            0,
		StandardDeductionCents:          10_000 * 100,
		SpecialDeductionCents:           2_000 * 100,
		SpecialAdditionalDeductionCents: 0,
		WithheldCents:                   0,
	}

	out, err := ComputeCumulativeWithholding(in)
	if err != nil {
		t.Fatalf("err=%v", err)
	}

	if out.TaxableIncomeCents != 8_000*100 {
		t.Fatalf("taxable=%d", out.TaxableIncomeCents)
	}
	if out.RatePercent != 3 || out.QuickDeductionCents != 0 {
		t.Fatalf("rate=%d quick=%d", out.RatePercent, out.QuickDeductionCents)
	}
	if out.TaxLiabilityCents != 240*100 {
		t.Fatalf("liability=%d", out.TaxLiabilityCents)
	}
	if out.WithholdThisMonthCents != 240*100 || out.CreditCents != 0 {
		t.Fatalf("withhold=%d credit=%d", out.WithholdThisMonthCents, out.CreditCents)
	}
}

func TestMulPercentRoundHalfUpCents(t *testing.T) {
	t.Run("no rounding needed", func(t *testing.T) {
		if got := mulPercentRoundHalfUpCents(8000*100, 3); got != 240*100 {
			t.Fatalf("got=%d", got)
		}
	})

	t.Run("half up", func(t *testing.T) {
		if got := mulPercentRoundHalfUpCents(17, 3); got != 1 {
			t.Fatalf("got=%d", got)
		}
		if got := mulPercentRoundHalfUpCents(16, 3); got != 0 {
			t.Fatalf("got=%d", got)
		}
	})
}
