package server

import (
	"context"
	"errors"
	"testing"
	"time"
)

type assistantOrgStoreStub struct {
	*orgUnitMemoryStore
	resolveOrgID int
	resolveErr   error
	search       []OrgUnitSearchCandidate
	searchErr    error
	details      OrgUnitNodeDetails
	detailsErr   error
}

func (s assistantOrgStoreStub) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	if s.resolveErr != nil || s.resolveOrgID != 0 {
		return s.resolveOrgID, s.resolveErr
	}
	return s.orgUnitMemoryStore.ResolveOrgID(ctx, tenantID, orgCode)
}

func (s assistantOrgStoreStub) SearchNodeCandidates(ctx context.Context, tenantID string, query string, asOfDate string, limit int) ([]OrgUnitSearchCandidate, error) {
	if s.searchErr != nil || s.search != nil {
		return s.search, s.searchErr
	}
	return s.orgUnitMemoryStore.SearchNodeCandidates(ctx, tenantID, query, asOfDate, limit)
}

func (s assistantOrgStoreStub) GetNodeDetails(ctx context.Context, tenantID string, orgID int, asOfDate string) (OrgUnitNodeDetails, error) {
	if s.detailsErr != nil || s.details.OrgID != 0 || s.details.OrgCode != "" {
		return s.details, s.detailsErr
	}
	return s.orgUnitMemoryStore.GetNodeDetails(ctx, tenantID, orgID, asOfDate)
}

type assistantCommitAdapterStub struct {
	result *assistantCommitResult
	err    error
}

func (s assistantCommitAdapterStub) Commit(context.Context, assistantCommitRequest) (*assistantCommitResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func TestAssistantActionRegistryAndVersionTupleHelpers(t *testing.T) {
	t.Run("lookup action spec branches", func(t *testing.T) {
		svc := &assistantConversationService{actionRegistry: assistantActionRegistryMap{specs: map[string]assistantActionSpec{
			"custom": {ID: "custom", Version: "v9"},
		}}}
		if spec, ok := svc.lookupActionSpec("custom"); !ok || spec.Version != "v9" {
			t.Fatalf("unexpected custom spec=%+v ok=%v", spec, ok)
		}
		fallbackSvc := &assistantConversationService{}
		if spec, ok := fallbackSvc.lookupActionSpec(assistantIntentCreateOrgUnit); !ok || spec.ID != assistantIntentCreateOrgUnit {
			t.Fatalf("unexpected fallback spec=%+v ok=%v", spec, ok)
		}
	})

	t.Run("lookup commit adapter branches", func(t *testing.T) {
		svc := &assistantConversationService{commitAdapterRegistry: assistantCommitAdapterRegistryMap{adapters: map[string]assistantCommitAdapter{
			"custom": assistantCommitAdapterStub{result: &assistantCommitResult{OrgCode: "A1"}},
		}}}
		if adapter, ok := svc.lookupCommitAdapter("custom"); !ok || adapter == nil {
			t.Fatalf("unexpected custom adapter=%v ok=%v", adapter, ok)
		}
		defaultSvc := &assistantConversationService{writeSvc: assistantWriteServiceStub{store: newOrgUnitMemoryStore()}}
		if adapter, ok := defaultSvc.lookupCommitAdapter("orgunit_create_v1"); !ok || adapter == nil {
			t.Fatalf("unexpected default adapter=%v ok=%v", adapter, ok)
		}
		var nilSvc *assistantConversationService
		if adapter, ok := nilSvc.lookupCommitAdapter("missing"); ok || adapter != nil {
			t.Fatalf("expected nil service miss, got adapter=%v ok=%v", adapter, ok)
		}
	})

	t.Run("commit adapter branches", func(t *testing.T) {
		adapter := assistantCreateOrgUnitCommitAdapter{}
		if _, err := adapter.Commit(context.Background(), assistantCommitRequest{}); !errors.Is(err, errAssistantServiceMissing) {
			t.Fatalf("expected service missing, got %v", err)
		}
		adapter = assistantCreateOrgUnitCommitAdapter{writeSvc: assistantWriteServiceStub{store: newOrgUnitMemoryStore()}}
		if _, err := adapter.Commit(context.Background(), assistantCommitRequest{}); !errors.Is(err, errAssistantConversationCorrupted) {
			t.Fatalf("expected conversation corrupted, got %v", err)
		}
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatal(err)
		}
		adapter = assistantCreateOrgUnitCommitAdapter{writeSvc: assistantWriteServiceStub{store: store}}
		result, err := adapter.Commit(context.Background(), assistantCommitRequest{
			TenantID:  "tenant_1",
			Principal: Principal{ID: "actor_1"},
			Turn: &assistantTurn{
				Intent:        assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
				PolicyVersion: capabilityPolicyVersionBaseline,
				RequestID:     "req_1",
			},
			ResolvedCandidate: assistantCandidate{CandidateCode: "FLOWER-A"},
		})
		if err != nil || result == nil || result.ParentOrgCode != "FLOWER-A" {
			t.Fatalf("unexpected result=%+v err=%v", result, err)
		}
	})

	t.Run("version tuple timestamp branches", func(t *testing.T) {
		if got := assistantVersionTupleTimestamp(time.Time{}); got != "" {
			t.Fatalf("expected empty timestamp, got %q", got)
		}
		if got := assistantVersionTupleTimestamp(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)); got == "" {
			t.Fatal("expected non-empty timestamp")
		}
	})

	t.Run("refresh turn version tuple branches", func(t *testing.T) {
		svc := &assistantConversationService{}
		if err := svc.refreshTurnVersionTuple(context.Background(), "tenant_1", nil); err != nil {
			t.Fatalf("nil turn err=%v", err)
		}
		turn := &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), DryRun: assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, nil, "")}
		if err := svc.refreshTurnVersionTuple(context.Background(), "tenant_1", turn); err != nil {
			t.Fatalf("empty candidate err=%v", err)
		}
		unsupported := &assistantTurn{Intent: assistantIntentSpec{Action: "unknown"}, Plan: assistantBuildPlan(assistantIntentSpec{Action: "unknown"}), DryRun: assistantBuildDryRun(assistantIntentSpec{Action: "unknown"}, nil, ""), ResolvedCandidateID: "c1", Candidates: []assistantCandidate{{CandidateID: "c1"}}}
		if err := svc.refreshTurnVersionTuple(context.Background(), "tenant_1", unsupported); !errors.Is(err, errAssistantUnsupportedIntent) {
			t.Fatalf("expected unsupported intent, got %v", err)
		}
		store := newOrgUnitMemoryStore()
		tolerantSvc := &assistantConversationService{orgStore: assistantOrgStoreStub{orgUnitMemoryStore: store, detailsErr: errOrgUnitNotFound}}
		turn = &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"}, Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), DryRun: assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, nil, "c1"), ResolvedCandidateID: "c1", Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 1}}}
		if err := tolerantSvc.refreshTurnVersionTuple(context.Background(), "tenant_1", turn); err != nil {
			t.Fatalf("expected tolerant refresh, got %v", err)
		}
	})

	t.Run("capture and validate version tuple branches", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatal(err)
		}
		svc := &assistantConversationService{orgStore: store}
		if raw, err := svc.captureSelectedCandidateVersionTuple(context.Background(), "tenant_1", assistantIntentSpec{Action: "unknown"}, nil, ""); err != nil || raw != nil {
			t.Fatalf("unexpected raw=%s err=%v", string(raw), err)
		}
		if _, err := svc.captureSelectedCandidateVersionTuple(context.Background(), "tenant_1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"}, nil, "c1"); !errors.Is(err, errAssistantCandidateNotFound) {
			t.Fatalf("expected candidate not found, got %v", err)
		}
		missingSvc := &assistantConversationService{}
		if _, err := missingSvc.captureSelectedCandidateVersionTuple(context.Background(), "tenant_1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"}, []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}}, "c1"); !errors.Is(err, errAssistantServiceMissing) {
			t.Fatalf("expected service missing, got %v", err)
		}
		raw, err := svc.captureSelectedCandidateVersionTuple(context.Background(), "tenant_1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"}, []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 10000000}}, "c1")
		if err != nil || len(raw) == 0 {
			t.Fatalf("unexpected raw=%s err=%v", string(raw), err)
		}
		turn := &assistantTurn{Intent: assistantIntentSpec{Action: "unknown"}}
		if err := svc.validateTurnVersionTuple(context.Background(), "tenant_1", turn); err != nil {
			t.Fatalf("unexpected non-create validate err=%v", err)
		}
		turn = &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}}
		if err := svc.validateTurnVersionTuple(context.Background(), "tenant_1", turn); !errors.Is(err, errAssistantVersionTupleStale) {
			t.Fatalf("expected stale on empty tuple, got %v", err)
		}
		turn.Plan.VersionTuple = []byte(`bad-json`)
		if err := svc.validateTurnVersionTuple(context.Background(), "tenant_1", turn); !errors.Is(err, errAssistantVersionTupleStale) {
			t.Fatalf("expected stale on bad tuple, got %v", err)
		}
		turn = &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"}, Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), ResolvedCandidateID: "c1", Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 10000000}}}
		turn.Plan.VersionTuple = []byte(`{"parent_candidate_id":"c1","parent_org_code":"OTHER","parent_updated_at":"2000-01-01T00:00:00Z"}`)
		if err := svc.validateTurnVersionTuple(context.Background(), "tenant_1", turn); !errors.Is(err, errAssistantVersionTupleStale) {
			t.Fatalf("expected stale on mismatch, got %v", err)
		}
		if err := svc.refreshTurnVersionTuple(context.Background(), "tenant_1", turn); err != nil {
			t.Fatalf("refresh tuple err=%v", err)
		}
		if err := svc.validateTurnVersionTuple(context.Background(), "tenant_1", turn); err != nil {
			t.Fatalf("expected valid tuple, got %v", err)
		}
	})

	t.Run("lookup candidate details and resolve candidate org id branches", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatal(err)
		}
		svc := &assistantConversationService{}
		if _, _, err := svc.lookupCandidateDetails(context.Background(), "tenant_1", "2026-01-01", nil, "c1"); !errors.Is(err, errAssistantCandidateNotFound) {
			t.Fatalf("expected candidate not found, got %v", err)
		}
		svc = &assistantConversationService{orgStore: store}
		candidate, details, err := svc.lookupCandidateDetails(context.Background(), "tenant_1", "2026-01-01", []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 10000000}}, "c1")
		if err != nil || candidate.OrgID == 0 || details.OrgCode != "FLOWER-A" {
			t.Fatalf("unexpected candidate=%+v details=%+v err=%v", candidate, details, err)
		}
		stubSvc := &assistantConversationService{orgStore: assistantOrgStoreStub{orgUnitMemoryStore: store, resolveOrgID: 10000000}}
		if orgID, err := stubSvc.resolveAssistantCandidateOrgID(context.Background(), "tenant_1", assistantCandidate{CandidateCode: "FLOWER-A"}); err != nil || orgID != 10000000 {
			t.Fatalf("unexpected resolve by code orgID=%d err=%v", orgID, err)
		}
		stubSvc = &assistantConversationService{orgStore: assistantOrgStoreStub{orgUnitMemoryStore: store, resolveErr: errOrgUnitNotFound, search: []OrgUnitSearchCandidate{{OrgID: 7, OrgCode: "FLOWER-A", Name: "鲜花组织"}}}}
		if orgID, err := stubSvc.resolveAssistantCandidateOrgID(context.Background(), "tenant_1", assistantCandidate{CandidateCode: "FLOWER-A", Name: "鲜花组织", AsOf: "2026-01-01"}); err != nil || orgID != 7 {
			t.Fatalf("unexpected resolve by search orgID=%d err=%v", orgID, err)
		}
		stubSvc = &assistantConversationService{orgStore: assistantOrgStoreStub{orgUnitMemoryStore: store, search: []OrgUnitSearchCandidate{{OrgID: 9, OrgCode: "FLOWER-Z"}}}}
		if orgID, err := stubSvc.resolveAssistantCandidateOrgID(context.Background(), "tenant_1", assistantCandidate{Name: "鲜花组织", AsOf: "2026-01-01"}); err != nil || orgID != 9 {
			t.Fatalf("unexpected resolve single result orgID=%d err=%v", orgID, err)
		}
		stubSvc = &assistantConversationService{}
		if _, err := stubSvc.resolveAssistantCandidateOrgID(context.Background(), "tenant_1", assistantCandidate{}); !errors.Is(err, errAssistantServiceMissing) {
			t.Fatalf("expected service missing, got %v", err)
		}
		stubSvc = &assistantConversationService{orgStore: assistantOrgStoreStub{orgUnitMemoryStore: store, resolveErr: errOrgUnitNotFound, search: []OrgUnitSearchCandidate{{OrgID: 11, OrgCode: "OTHER"}, {OrgID: 12, OrgCode: "DIFF"}}}}
		if _, err := stubSvc.resolveAssistantCandidateOrgID(context.Background(), "tenant_1", assistantCandidate{CandidateCode: "FLOWER-A", AsOf: "2026-01-01"}); !errors.Is(err, errAssistantCandidateNotFound) {
			t.Fatalf("expected candidate not found, got %v", err)
		}
	})
}
