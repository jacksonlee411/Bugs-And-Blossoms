package server

import (
	"context"
	"errors"
	"testing"
	"time"

	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
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

type assistantWriteServiceRecorder struct {
	writeReq   orgunitservices.WriteOrgUnitRequest
	writeErr   error
	renameReq  orgunitservices.RenameOrgUnitRequest
	renameErr  error
	moveReq    orgunitservices.MoveOrgUnitRequest
	moveErr    error
	disableReq orgunitservices.DisableOrgUnitRequest
	disableErr error
	enableReq  orgunitservices.EnableOrgUnitRequest
	enableErr  error
	correctReq orgunitservices.CorrectOrgUnitRequest
	correctErr error
}

func (s *assistantWriteServiceRecorder) Write(_ context.Context, _ string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
	s.writeReq = req
	if s.writeErr != nil {
		return orgunitservices.OrgUnitWriteResult{}, s.writeErr
	}
	eventType := "UPDATE"
	if req.Intent == string(orgunitservices.OrgUnitWriteIntentCreateOrg) {
		eventType = "CREATE"
	}
	orgCode := req.OrgCode
	if orgCode == "" {
		orgCode = "FLOWER-C"
	}
	return orgunitservices.OrgUnitWriteResult{OrgCode: orgCode, EffectiveDate: req.EffectiveDate, EventType: eventType, EventUUID: "evt_write"}, nil
}

func (s *assistantWriteServiceRecorder) Create(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (s *assistantWriteServiceRecorder) Rename(_ context.Context, _ string, req orgunitservices.RenameOrgUnitRequest) error {
	s.renameReq = req
	return s.renameErr
}

func (s *assistantWriteServiceRecorder) Move(_ context.Context, _ string, req orgunitservices.MoveOrgUnitRequest) error {
	s.moveReq = req
	return s.moveErr
}

func (s *assistantWriteServiceRecorder) Disable(_ context.Context, _ string, req orgunitservices.DisableOrgUnitRequest) error {
	s.disableReq = req
	return s.disableErr
}

func (s *assistantWriteServiceRecorder) Enable(_ context.Context, _ string, req orgunitservices.EnableOrgUnitRequest) error {
	s.enableReq = req
	return s.enableErr
}

func (s *assistantWriteServiceRecorder) SetBusinessUnit(context.Context, string, orgunitservices.SetBusinessUnitRequest) error {
	return errors.New("not implemented")
}

func (s *assistantWriteServiceRecorder) Correct(_ context.Context, _ string, req orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	s.correctReq = req
	if s.correctErr != nil {
		return orgunittypes.OrgUnitResult{}, s.correctErr
	}
	return orgunittypes.OrgUnitResult{OrgCode: req.OrgCode, EffectiveDate: req.TargetEffectiveDate}, nil
}

func (s *assistantWriteServiceRecorder) CorrectStatus(context.Context, string, orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (s *assistantWriteServiceRecorder) RescindRecord(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (s *assistantWriteServiceRecorder) RescindOrg(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
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
		for _, actionID := range []string{assistantIntentCreateOrgUnit, assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion, assistantIntentCorrectOrgUnit, assistantIntentDisableOrgUnit, assistantIntentEnableOrgUnit, assistantIntentMoveOrgUnit, assistantIntentRenameOrgUnit} {
			if spec, ok := fallbackSvc.lookupActionSpec(actionID); !ok || spec.ID != actionID || spec.Handler.CommitAdapterKey == "" {
				t.Fatalf("unexpected fallback spec action=%s spec=%+v ok=%v", actionID, spec, ok)
			}
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
		for _, key := range []string{"orgunit_create_v1", "orgunit_add_version_v1", "orgunit_insert_version_v1", "orgunit_correct_v1", "orgunit_disable_v1", "orgunit_enable_v1", "orgunit_move_v1", "orgunit_rename_v1"} {
			if adapter, ok := defaultSvc.lookupCommitAdapter(key); !ok || adapter == nil {
				t.Fatalf("unexpected default adapter key=%s adapter=%v ok=%v", key, adapter, ok)
			}
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

	t.Run("additional commit adapters succeed", func(t *testing.T) {
		writeSvc := &assistantWriteServiceRecorder{}
		cases := []struct {
			name    string
			adapter assistantCommitAdapter
			turn    *assistantTurn
			cand    assistantCandidate
			assert  func(*testing.T, *assistantCommitResult, *assistantWriteServiceRecorder)
		}{
			{name: "add_version", adapter: assistantAddOrgUnitVersionCommitAdapter{writeSvc: writeSvc}, turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentAddOrgUnitVersion, OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01", NewName: "运营一部"}, PolicyVersion: capabilityPolicyVersionBaseline, RequestID: "req_add"}, assert: func(t *testing.T, result *assistantCommitResult, recorder *assistantWriteServiceRecorder) {
				if result == nil || result.OrgCode != "FLOWER-C" || result.EventType != "UPDATE" || recorder.writeReq.Intent != string(orgunitservices.OrgUnitWriteIntentAddVersion) {
					t.Fatalf("unexpected add result=%+v req=%+v", result, recorder.writeReq)
				}
			}},
			{name: "correct", adapter: assistantCorrectOrgUnitCommitAdapter{writeSvc: writeSvc}, turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, OrgCode: "FLOWER-C", TargetEffectiveDate: "2026-01-01", NewName: "运营中心"}, PolicyVersion: capabilityPolicyVersionBaseline, RequestID: "req_correct"}, assert: func(t *testing.T, result *assistantCommitResult, recorder *assistantWriteServiceRecorder) {
				if result == nil || result.OrgCode != "FLOWER-C" || result.EventType != "UPDATE" || recorder.correctReq.OrgCode != "FLOWER-C" {
					t.Fatalf("unexpected correct result=%+v req=%+v", result, recorder.correctReq)
				}
			}},
			{name: "rename", adapter: assistantRenameOrgUnitCommitAdapter{writeSvc: writeSvc}, turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentRenameOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-03-01", NewName: "运营平台部"}, PolicyVersion: capabilityPolicyVersionBaseline, RequestID: "req_rename"}, assert: func(t *testing.T, result *assistantCommitResult, recorder *assistantWriteServiceRecorder) {
				if result == nil || result.EventType != "RENAME" || recorder.renameReq.NewName != "运营平台部" {
					t.Fatalf("unexpected rename result=%+v req=%+v", result, recorder.renameReq)
				}
			}},
			{name: "move", adapter: assistantMoveOrgUnitCommitAdapter{writeSvc: writeSvc}, turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentMoveOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01", NewParentRefText: "共享服务中心"}, PolicyVersion: capabilityPolicyVersionBaseline, RequestID: "req_move"}, cand: assistantCandidate{CandidateCode: "FLOWER-B"}, assert: func(t *testing.T, result *assistantCommitResult, recorder *assistantWriteServiceRecorder) {
				if result == nil || result.ParentOrgCode != "FLOWER-B" || result.EventType != "MOVE" || recorder.moveReq.NewParentOrgCode != "FLOWER-B" {
					t.Fatalf("unexpected move result=%+v req=%+v", result, recorder.moveReq)
				}
			}},
			{name: "disable", adapter: assistantDisableOrgUnitCommitAdapter{writeSvc: writeSvc}, turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentDisableOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-05-01"}, PolicyVersion: capabilityPolicyVersionBaseline, RequestID: "req_disable"}, assert: func(t *testing.T, result *assistantCommitResult, recorder *assistantWriteServiceRecorder) {
				if result == nil || result.EventType != "DISABLE" || recorder.disableReq.OrgCode != "FLOWER-C" {
					t.Fatalf("unexpected disable result=%+v req=%+v", result, recorder.disableReq)
				}
			}},
			{name: "enable", adapter: assistantEnableOrgUnitCommitAdapter{writeSvc: writeSvc}, turn: &assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentEnableOrgUnit, OrgCode: "FLOWER-C", EffectiveDate: "2026-06-01"}, PolicyVersion: capabilityPolicyVersionBaseline, RequestID: "req_enable"}, assert: func(t *testing.T, result *assistantCommitResult, recorder *assistantWriteServiceRecorder) {
				if result == nil || result.EventType != "ENABLE" || recorder.enableReq.OrgCode != "FLOWER-C" {
					t.Fatalf("unexpected enable result=%+v req=%+v", result, recorder.enableReq)
				}
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				*writeSvc = assistantWriteServiceRecorder{}
				result, err := tc.adapter.Commit(context.Background(), assistantCommitRequest{TenantID: "tenant_1", Principal: Principal{ID: "actor_1"}, Turn: tc.turn, ResolvedCandidate: tc.cand})
				if err != nil {
					t.Fatalf("commit err=%v", err)
				}
				tc.assert(t, result, writeSvc)
			})
		}
		if _, err := (assistantRenameOrgUnitCommitAdapter{}).Commit(context.Background(), assistantCommitRequest{}); !errors.Is(err, errAssistantServiceMissing) {
			t.Fatalf("expected rename adapter service missing, got %v", err)
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
