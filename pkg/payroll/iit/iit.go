package iit

import "fmt"

const (
	standardDeductionMonthlyCents int64 = 5000 * 100
)

type CumulativeInput struct {
	IncomeCents                     int64
	TaxExemptIncomeCents            int64
	StandardDeductionCents          int64
	SpecialDeductionCents           int64
	SpecialAdditionalDeductionCents int64
	WithheldCents                   int64
}

type CumulativeResult struct {
	TaxableIncomeCents     int64
	TaxLiabilityCents      int64
	DeltaCents             int64
	WithholdThisMonthCents int64
	CreditCents            int64
	RatePercent            int64
	QuickDeductionCents    int64
}

func StandardDeductionCents(firstTaxMonth, taxMonth int) (int64, error) {
	if firstTaxMonth < 1 || firstTaxMonth > 12 {
		return 0, fmt.Errorf("firstTaxMonth out of range: %d", firstTaxMonth)
	}
	if taxMonth < 1 || taxMonth > 12 {
		return 0, fmt.Errorf("taxMonth out of range: %d", taxMonth)
	}
	if taxMonth < firstTaxMonth {
		return 0, fmt.Errorf("taxMonth must be >= firstTaxMonth: taxMonth=%d firstTaxMonth=%d", taxMonth, firstTaxMonth)
	}

	monthCount := int64(taxMonth - firstTaxMonth + 1)
	return standardDeductionMonthlyCents * monthCount, nil
}

func ComputeCumulativeWithholding(in CumulativeInput) (CumulativeResult, error) {
	if in.IncomeCents < 0 ||
		in.TaxExemptIncomeCents < 0 ||
		in.StandardDeductionCents < 0 ||
		in.SpecialDeductionCents < 0 ||
		in.SpecialAdditionalDeductionCents < 0 ||
		in.WithheldCents < 0 {
		return CumulativeResult{}, fmt.Errorf("all inputs must be non-negative")
	}

	taxableIncomeCents := max0(
		in.IncomeCents -
			in.TaxExemptIncomeCents -
			in.StandardDeductionCents -
			in.SpecialDeductionCents -
			in.SpecialAdditionalDeductionCents,
	)

	ratePercent, quickDeductionCents := bracketForTaxableIncome(taxableIncomeCents)

	taxLiabilityCents := max0(mulPercentRoundHalfUpCents(taxableIncomeCents, ratePercent) - quickDeductionCents)

	deltaCents := taxLiabilityCents - in.WithheldCents
	if deltaCents > 0 {
		return CumulativeResult{
			TaxableIncomeCents:     taxableIncomeCents,
			TaxLiabilityCents:      taxLiabilityCents,
			DeltaCents:             deltaCents,
			WithholdThisMonthCents: deltaCents,
			CreditCents:            0,
			RatePercent:            ratePercent,
			QuickDeductionCents:    quickDeductionCents,
		}, nil
	}

	return CumulativeResult{
		TaxableIncomeCents:     taxableIncomeCents,
		TaxLiabilityCents:      taxLiabilityCents,
		DeltaCents:             deltaCents,
		WithholdThisMonthCents: 0,
		CreditCents:            -deltaCents,
		RatePercent:            ratePercent,
		QuickDeductionCents:    quickDeductionCents,
	}, nil
}

func bracketForTaxableIncome(taxableIncomeCents int64) (ratePercent int64, quickDeductionCents int64) {
	switch {
	case taxableIncomeCents <= 36_000*100:
		return 3, 0
	case taxableIncomeCents <= 144_000*100:
		return 10, 2_520 * 100
	case taxableIncomeCents <= 300_000*100:
		return 20, 16_920 * 100
	case taxableIncomeCents <= 420_000*100:
		return 25, 31_920 * 100
	case taxableIncomeCents <= 660_000*100:
		return 30, 52_920 * 100
	case taxableIncomeCents <= 960_000*100:
		return 35, 85_920 * 100
	default:
		return 45, 181_920 * 100
	}
}

func mulPercentRoundHalfUpCents(amountCents int64, percent int64) int64 {
	if amountCents < 0 || percent < 0 {
		panic("mulPercentRoundHalfUpCents expects non-negative inputs")
	}
	if amountCents == 0 || percent == 0 {
		return 0
	}

	n := amountCents * percent
	q := n / 100
	r := n % 100
	if r >= 50 {
		return q + 1
	}
	return q
}

func max0(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
