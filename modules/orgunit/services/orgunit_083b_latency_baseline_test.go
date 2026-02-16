package services

import (
	"context"
	"math"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
)

type serviceLatencyResult struct {
	total            int
	errCount         int
	stableCodeCount  int
	p95MS            float64
	p99MS            float64
	errorRatePercent float64
	stableCodeRate   float64
}

func Test083BLatencyBaselineWriteFailClosed(t *testing.T) {
	if os.Getenv("RUN_083B_LATENCY") != "1" {
		t.Skip("set RUN_083B_LATENCY=1 to run 083B latency baseline")
	}

	const rounds = 3
	const samplesPerRound = 50

	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{
				EventType: types.OrgUnitEventMove,
			}, nil
		},
	}
	svc := newWriteService(store)

	durations := make([]time.Duration, 0, rounds*samplesPerRound)
	errCount := 0
	stableCodeCount := 0

	for i := 0; i < rounds*samplesPerRound; i++ {
		start := time.Now()
		_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
			OrgCode:             "ROOT",
			TargetEffectiveDate: "2026-01-01",
			RequestID:           "083b-latency",
			Patch: OrgUnitCorrectionPatch{
				Name: stringPtr("Rename-on-move-must-fail"),
			},
		})
		durations = append(durations, time.Since(start))
		if err == nil {
			errCount++
			continue
		}
		if !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			errCount++
			continue
		}
		stableCodeCount++
	}

	result := serviceLatencyResult{
		total:            len(durations),
		errCount:         errCount,
		stableCodeCount:  stableCodeCount,
		p95MS:            servicePercentileMS(durations, 0.95),
		p99MS:            servicePercentileMS(durations, 0.99),
		errorRatePercent: float64(errCount) * 100 / float64(len(durations)),
		stableCodeRate:   float64(stableCodeCount) * 100 / float64(len(durations)),
	}

	t.Logf("083B latency baseline scenario=write-fail-closed samples=%d errors=%d stable_code_rate=%.3f%% error_rate=%.3f%% p95=%.3fms p99=%.3fms", result.total, result.errCount, result.stableCodeRate, result.errorRatePercent, result.p95MS, result.p99MS)

	if result.p95MS > 500 {
		t.Fatalf("write-fail-closed p95 %.3fms > limit 500ms", result.p95MS)
	}
	if result.p99MS > 1000 {
		t.Fatalf("write-fail-closed p99 %.3fms > limit 1000ms", result.p99MS)
	}
	if result.errorRatePercent > 0.0 {
		t.Fatalf("write-fail-closed error rate %.3f%% > limit 0%%", result.errorRatePercent)
	}
	if result.stableCodeRate < 100.0 {
		t.Fatalf("write-fail-closed stable code rate %.3f%% < required 100%%", result.stableCodeRate)
	}
}

func servicePercentileMS(items []time.Duration, q float64) float64 {
	if len(items) == 0 {
		return 0
	}
	clone := append([]time.Duration(nil), items...)
	sort.Slice(clone, func(i, j int) bool { return clone[i] < clone[j] })
	idx := int(math.Ceil(float64(len(clone))*q)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(clone) {
		idx = len(clone) - 1
	}
	return float64(clone[idx].Microseconds()) / 1000.0
}
