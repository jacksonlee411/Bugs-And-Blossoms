package server

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"

	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
)

type latencyBaselineResult struct {
	total     int
	errCount  int
	p95MS     float64
	p99MS     float64
	errorRate float64
}

func Test083BLatencyBaselineServerHandlers(t *testing.T) {
	if os.Getenv("RUN_083B_LATENCY") != "1" {
		t.Skip("set RUN_083B_LATENCY=1 to run 083B latency baseline")
	}

	const rounds = 3
	const samplesPerRound = 50

	mutationStore := mutationCapabilitiesStoreStub{
		resolveOrgCodeStore: &resolveOrgCodeStore{resolveID: 10000001},
		resolveTargetFn: func(_ context.Context, _ string, _ int, _ string) (orgUnitMutationTargetEvent, error) {
			return orgUnitMutationTargetEvent{
				EffectiveEventType: string(orgunittypes.OrgUnitEventRename),
				HasEffective:       true,
			}, nil
		},
		listEnabledFn: func(_ context.Context, _ string, _ string) ([]orgUnitTenantFieldConfig, error) {
			return []orgUnitTenantFieldConfig{{FieldKey: "org_type"}}, nil
		},
		evalRescindFn: func(_ context.Context, _ string, _ int) ([]string, error) {
			return []string{}, nil
		},
	}
	mutationResult := measureLatency(rounds, samplesPerRound, func() error {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, mutationStore)
		if rec.Code != http.StatusOK {
			return fmt.Errorf("unexpected mutation capabilities status=%d", rec.Code)
		}
		return nil
	})
	assertLatencyThreshold(t, "mutation-capabilities", mutationResult, 300, 600, 0.5)

	appendStore := orgUnitStoreWithAppendCapabilities{
		OrgUnitStore: newOrgUnitMemoryStore(),
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		listExtConfigsFn: func(_ context.Context, _ string, _ string) ([]orgUnitTenantFieldConfig, error) {
			return []orgUnitTenantFieldConfig{{FieldKey: "org_type"}}, nil
		},
		isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) {
			return true, nil
		},
		resolveFactsFn: func(_ context.Context, _ string, _ int, _ string) (orgUnitAppendFacts, error) {
			return orgUnitAppendFacts{
				TreeInitialized:  true,
				TargetExistsAsOf: true,
				IsRoot:           false,
			}, nil
		},
	}
	appendResult := measureLatency(rounds, samplesPerRound, func() error {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, appendStore)
		if rec.Code != http.StatusOK {
			return fmt.Errorf("unexpected append capabilities status=%d", rec.Code)
		}
		return nil
	})
	assertLatencyThreshold(t, "append-capabilities", appendResult, 300, 600, 0.5)

	listStore := &extListStoreStub{
		resolveOrgCodeStore: &resolveOrgCodeStore{},
		listFn: func(_ context.Context, _ string, _ orgUnitListPageRequest) ([]orgUnitListItem, int, error) {
			return []orgUnitListItem{{OrgCode: "A001", Name: "Root", Status: "active"}}, 1, nil
		},
	}
	listResult := measureLatency(rounds, samplesPerRound, func() error {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&mode=grid&sort=ext:org_type&order=desc&ext_filter_field_key=org_type&ext_filter_value=DEPARTMENT&page=0&size=10", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAPI(rec, req, listStore, nil)
		if rec.Code != http.StatusOK {
			return fmt.Errorf("unexpected list ext query status=%d", rec.Code)
		}
		return nil
	})
	assertLatencyThreshold(t, "list-ext-filter-sort", listResult, 900, 1500, 1.0)
}

func measureLatency(rounds int, samplesPerRound int, run func() error) latencyBaselineResult {
	durations := make([]time.Duration, 0, rounds*samplesPerRound)
	errCount := 0
	for i := 0; i < rounds*samplesPerRound; i++ {
		start := time.Now()
		if err := run(); err != nil {
			errCount++
		}
		durations = append(durations, time.Since(start))
	}
	p95, p99 := percentileMS(durations, 0.95), percentileMS(durations, 0.99)
	total := len(durations)
	return latencyBaselineResult{
		total:     total,
		errCount:  errCount,
		p95MS:     p95,
		p99MS:     p99,
		errorRate: float64(errCount) * 100 / float64(total),
	}
}

func percentileMS(items []time.Duration, q float64) float64 {
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

func assertLatencyThreshold(t *testing.T, scenario string, result latencyBaselineResult, p95Limit float64, p99Limit float64, errorRateLimit float64) {
	t.Helper()
	t.Logf("083B latency baseline scenario=%s samples=%d errors=%d error_rate=%.3f%% p95=%.3fms p99=%.3fms", scenario, result.total, result.errCount, result.errorRate, result.p95MS, result.p99MS)
	if result.p95MS > p95Limit {
		t.Fatalf("%s p95 %.3fms > limit %.3fms", scenario, result.p95MS, p95Limit)
	}
	if result.p99MS > p99Limit {
		t.Fatalf("%s p99 %.3fms > limit %.3fms", scenario, result.p99MS, p99Limit)
	}
	if result.errorRate > errorRateLimit {
		t.Fatalf("%s error rate %.3f%% > limit %.3f%%", scenario, result.errorRate, errorRateLimit)
	}
}
