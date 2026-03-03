package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func assistantPersistenceConversationRow(conversationID string, actorID string, state string, now time.Time) []any {
	return []any{conversationID, "tenant_1", actorID, "tenant-admin", state, now, now}
}

func TestAssistantPersistence_CreateAndGetPGErrorBranches(t *testing.T) {
	now := time.Now().UTC()
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	tx := &assistFakeTx{}
	tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
		if strings.Contains(sql, "INSERT INTO iam.assistant_conversations") {
			return pgconn.NewCommandTag(""), errors.New("insert failed")
		}
		return pgconn.NewCommandTag(""), nil
	}
	svc.pool = assistFakeTxBeginner{tx: tx}

	if _, err := svc.createConversationPG(nil, "tenant_1", principal); err == nil {
		t.Fatal("expected createConversationPG insert error")
	}

	svc.mu.Lock()
	svc.byID["conv_nil"] = nil
	svc.mu.Unlock()
	if _, err := svc.getConversationPG(context.Background(), "tenant_1", principal.ID, "conv_nil"); !errors.Is(err, errAssistantConversationCorrupted) {
		t.Fatalf("unexpected err=%v", err)
	}

	tx = &assistFakeTx{}
	tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		if strings.Contains(sql, "FROM iam.assistant_conversations") {
			return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_load", "actor_1", assistantStateValidated, now)}
		}
		return &assistFakeRow{err: pgx.ErrNoRows}
	}
	tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
		if strings.Contains(sql, "FROM iam.assistant_turns") || strings.Contains(sql, "FROM iam.assistant_state_transitions") {
			return &assistFakeRows{}, nil
		}
		return &assistFakeRows{}, nil
	}
	svc.pool = assistFakeTxBeginner{tx: tx}
	if _, err := svc.getConversationPG(nil, "tenant_1", "actor_x", "conv_load"); !errors.Is(err, errAssistantConversationForbidden) {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestAssistantPersistence_CreateTurnPGErrorMatrix(t *testing.T) {
	now := time.Now().UTC()
	makeTx := func(actorID string, execNeedle string, transitionErr error, commitErr error) *assistFakeTx {
		tx := &assistFakeTx{commitErr: commitErr}
		tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if execNeedle != "" && strings.Contains(sql, execNeedle) {
				return pgconn.NewCommandTag(""), errors.New("exec failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_pg", actorID, assistantStateValidated, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				if transitionErr != nil {
					return &assistFakeRow{err: transitionErr}
				}
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.assistant_turns") || strings.Contains(sql, "FROM iam.assistant_state_transitions") {
				return &assistFakeRows{}, nil
			}
			return &assistFakeRows{}, nil
		}
		return tx
	}

	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true)
	_, _ = store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-B", "鲜花组织", "", true)
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})

	svc.pool = assistFakeTxBeginner{tx: makeTx("actor_2", "", nil, nil)}
	if _, err := svc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "仅生成计划"); !errors.Is(err, errAssistantConversationForbidden) {
		t.Fatalf("unexpected err=%v", err)
	}

	svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", "INSERT INTO iam.assistant_turns", nil, nil)}
	if _, err := svc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "仅生成计划"); err == nil || !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", "UPDATE iam.assistant_conversations", nil, nil)}
	if _, err := svc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "仅生成计划"); err == nil || !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", "", errors.New("transition failed"), nil)}
	if _, err := svc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "仅生成计划"); err == nil || !strings.Contains(err.Error(), "transition failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", "", nil, errors.New("commit failed"))}
	if _, err := svc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "仅生成计划"); err == nil || !strings.Contains(err.Error(), "commit failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	errSvc := newAssistantConversationService(assistantSearchErrStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}, assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	if _, err := errSvc.createTurnPG(nil, "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"); err == nil {
		t.Fatal("expected resolve candidates error")
	}

	originalDefinitions := capabilityDefinitionByKey
	capabilityDefinitionByKey = map[string]capabilityDefinition{}
	if _, err := svc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "仅生成计划"); !errors.Is(err, errAssistantPlanBoundaryViolation) {
		t.Fatalf("unexpected err=%v", err)
	}
	capabilityDefinitionByKey = originalDefinitions

	zeroSvc := newAssistantConversationService(assistantNoCandidateStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}, assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	zeroSvc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", "", nil, nil)}
	zeroConversation, err := zeroSvc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01")
	if err != nil {
		t.Fatalf("unexpected err=%v", err)
	}
	if got := zeroConversation.Turns[0].Confidence; got != 0.3 {
		t.Fatalf("zero candidate confidence=%v", got)
	}

	oneStore := newOrgUnitMemoryStore()
	_, _ = oneStore.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true)
	oneSvc := newAssistantConversationService(oneStore, assistantWriteServiceStub{store: oneStore})
	oneSvc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", "", nil, nil)}
	oneConversation, err := oneSvc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01")
	if err != nil {
		t.Fatalf("unexpected err=%v", err)
	}
	if got := oneConversation.Turns[0].Confidence; got != 0.95 || oneConversation.Turns[0].ResolvedCandidateID == "" {
		t.Fatalf("one candidate turn=%+v", oneConversation.Turns[0])
	}

	svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", "", nil, nil)}
	multiConversation, err := svc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01")
	if err != nil {
		t.Fatalf("unexpected err=%v", err)
	}
	if got := multiConversation.Turns[0].Confidence; got != 0.55 {
		t.Fatalf("multi candidate confidence=%v", got)
	}

	if _, err := svc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "在鲜花组织之下，新建一个名为运营部的部门"); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("unexpected err=%v", err)
	}

	originalAnnotateFn := assistantAnnotateIntentPlanFn
	assistantAnnotateIntentPlanFn = func(string, string, string, *assistantIntentSpec, *assistantPlanSummary, *assistantDryRunResult) error {
		return errAssistantPlanDeterminismViolation
	}
	svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", "", nil, nil)}
	if _, err := svc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "仅生成计划"); !errors.Is(err, errAssistantPlanDeterminismViolation) {
		t.Fatalf("unexpected err=%v", err)
	}
	assistantAnnotateIntentPlanFn = originalAnnotateFn

	missingConversationTx := makeTx("actor_1", "", nil, nil)
	missingConversationTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		if strings.Contains(sql, "FROM iam.assistant_conversations") {
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		return &assistFakeRow{err: pgx.ErrNoRows}
	}
	svc.pool = assistFakeTxBeginner{tx: missingConversationTx}
	if _, err := svc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_pg", "仅生成计划"); !errors.Is(err, errAssistantConversationNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestAssistantPersistence_LoadConversationTxErrorMatrix(t *testing.T) {
	now := time.Now().UTC()
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	makeBaseTurnRow := func() []any {
		turn := &assistantTurn{
			TurnID:             "turn_1",
			UserInput:          "输入",
			State:              assistantStateValidated,
			RiskTier:           "high",
			RequestID:          "req_1",
			TraceID:            "trace_1",
			PolicyVersion:      capabilityPolicyVersionBaseline,
			CompositionVersion: capabilityPolicyVersionBaseline,
			MappingVersion:     capabilityPolicyVersionBaseline,
			Intent: assistantIntentSpec{
				Action:              assistantIntentCreateOrgUnit,
				ParentRefText:       "鲜花组织",
				EntityName:          "运营部",
				EffectiveDate:       "2026-01-01",
				IntentSchemaVersion: assistantIntentSchemaVersionV1,
			},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}},
			ResolvedCandidateID: "c1",
			AmbiguityCount:      1,
			Confidence:          0.9,
			ResolutionSource:    assistantResolutionAuto,
			DryRun:              assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EntityName: "运营部", EffectiveDate: "2026-01-01"}, nil, ""),
			CommitResult:        &assistantCommitResult{OrgCode: "ORG-1"},
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		return assistantTurnRowValues(turn)
	}

	makeTx := func(turnRows pgx.Rows, turnErr error, transitionRows pgx.Rows, transitionErr error) *assistFakeTx {
		tx := &assistFakeTx{}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "FROM iam.assistant_conversations") {
				return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_err", "actor_1", assistantStateValidated, now)}
			}
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				if turnErr != nil {
					return nil, turnErr
				}
				return turnRows, nil
			case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
				if transitionErr != nil {
					return nil, transitionErr
				}
				return transitionRows, nil
			default:
				return &assistFakeRows{}, nil
			}
		}
		return tx
	}

	noRowTx := &assistFakeTx{queryRowFn: func(string, ...any) pgx.Row { return &assistFakeRow{err: pgx.ErrNoRows} }}
	if _, err := svc.loadConversationTx(context.Background(), noRowTx, "tenant_1", "conv_err", false); !errors.Is(err, errAssistantConversationNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}
	scanErrTx := &assistFakeTx{queryRowFn: func(string, ...any) pgx.Row { return &assistFakeRow{err: errors.New("scan failed")} }}
	if _, err := svc.loadConversationTx(context.Background(), scanErrTx, "tenant_1", "conv_err", false); err == nil || !strings.Contains(err.Error(), "scan failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	if _, err := svc.loadConversationTx(context.Background(), makeTx(nil, errors.New("turn query failed"), nil, nil), "tenant_1", "conv_err", false); err == nil || !strings.Contains(err.Error(), "turn query failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	for _, tc := range []struct {
		name string
		idx  int
	}{
		{name: "intent", idx: 9},
		{name: "plan", idx: 10},
		{name: "candidates", idx: 11},
		{name: "dry_run", idx: 16},
		{name: "commit_result", idx: 17},
	} {
		row := makeBaseTurnRow()
		row[tc.idx] = []byte("{")
		turnRows := &assistFakeRows{rows: [][]any{row}}
		_, err := svc.loadConversationTx(context.Background(), makeTx(turnRows, nil, &assistFakeRows{}, nil), "tenant_1", "conv_err", false)
		if err == nil {
			t.Fatalf("%s: expected decode error", tc.name)
		}
	}
	turnScanErrRows := &assistFakeRows{rows: [][]any{{"bad"}}}
	if _, err := svc.loadConversationTx(context.Background(), makeTx(turnScanErrRows, nil, &assistFakeRows{}, nil), "tenant_1", "conv_err", false); err == nil {
		t.Fatal("expected turn scan error")
	}

	_, err := svc.loadConversationTx(context.Background(), makeTx(&assistFakeRows{err: errors.New("turn rows err")}, nil, &assistFakeRows{}, nil), "tenant_1", "conv_err", false)
	if err == nil || !strings.Contains(err.Error(), "turn rows err") {
		t.Fatalf("unexpected err=%v", err)
	}

	turnRows := &assistFakeRows{rows: [][]any{makeBaseTurnRow()}}
	_, err = svc.loadConversationTx(context.Background(), makeTx(turnRows, nil, nil, errors.New("transition query failed")), "tenant_1", "conv_err", false)
	if err == nil || !strings.Contains(err.Error(), "transition query failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	badTransitionRows := &assistFakeRows{rows: [][]any{{"bad"}}}
	_, err = svc.loadConversationTx(context.Background(), makeTx(turnRows, nil, badTransitionRows, nil), "tenant_1", "conv_err", false)
	if err == nil {
		t.Fatal("expected transition scan error")
	}

	_, err = svc.loadConversationTx(context.Background(), makeTx(turnRows, nil, &assistFakeRows{err: errors.New("transition rows err")}, nil), "tenant_1", "conv_err", false)
	if err == nil || !strings.Contains(err.Error(), "transition rows err") {
		t.Fatalf("unexpected err=%v", err)
	}

	commitErrTx := &assistFakeTx{commitErr: errors.New("commit failed")}
	commitErrTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		if strings.Contains(sql, "FROM iam.assistant_conversations") {
			return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_commit", "actor_1", assistantStateValidated, now)}
		}
		return &assistFakeRow{err: pgx.ErrNoRows}
	}
	commitErrTx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
		if strings.Contains(sql, "FROM iam.assistant_turns") || strings.Contains(sql, "FROM iam.assistant_state_transitions") {
			return &assistFakeRows{}, nil
		}
		return &assistFakeRows{}, nil
	}
	svc.pool = assistFakeTxBeginner{tx: commitErrTx}
	if _, err := svc.loadConversationByTenant(context.Background(), "tenant_1", "conv_commit", false); err == nil || !strings.Contains(err.Error(), "commit failed") {
		t.Fatalf("unexpected err=%v", err)
	}
	notFoundTx := &assistFakeTx{}
	notFoundTx.queryRowFn = func(string, ...any) pgx.Row { return &assistFakeRow{err: pgx.ErrNoRows} }
	svc.pool = assistFakeTxBeginner{tx: notFoundTx}
	if _, err := svc.loadConversationByTenant(context.Background(), "tenant_1", "conv_commit", false); !errors.Is(err, errAssistantConversationNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestAssistantPersistence_IdempotencyAndFinalizeErrorBranches(t *testing.T) {
	ctx := context.Background()
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	key := assistantIdempotencyKey{TenantID: "tenant_1", ConversationID: "conv_1", TurnID: "turn_1", TurnAction: "commit", RequestID: "req_1"}

	tx := &assistFakeTx{}
	tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		if strings.Contains(sql, "INSERT INTO iam.assistant_idempotency") {
			return &assistFakeRow{err: errors.New("insert failed")}
		}
		return &assistFakeRow{err: pgx.ErrNoRows}
	}
	if _, err := svc.claimIdempotencyTx(ctx, tx, key, "hash"); err == nil || !strings.Contains(err.Error(), "insert failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		switch {
		case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
			return &assistFakeRow{err: pgx.ErrNoRows}
		case strings.Contains(sql, "SELECT request_hash"):
			return &assistFakeRow{err: errors.New("select failed")}
		default:
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
	}
	if _, err := svc.claimIdempotencyTx(ctx, tx, key, "hash"); err == nil || !strings.Contains(err.Error(), "select failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		if strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions") {
			return &assistFakeRow{err: errors.New("transition insert failed")}
		}
		return &assistFakeRow{err: pgx.ErrNoRows}
	}
	if err := svc.insertTransitionTx(ctx, tx, "tenant_1", "conv_1", &assistantStateTransition{}); err == nil || !strings.Contains(err.Error(), "transition insert failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
		if strings.Contains(sql, "UPDATE iam.assistant_idempotency") {
			return pgconn.NewCommandTag(""), errors.New("finalize update failed")
		}
		return pgconn.NewCommandTag(""), nil
	}
	if err := svc.finalizeIdempotencyErrorTx(ctx, tx, key, errAssistantCandidateNotFound); err == nil || !strings.Contains(err.Error(), "finalize update failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	conv := &assistantConversation{
		ConversationID: "conv_1",
		TenantID:       "tenant_1",
		ActorID:        "actor_1",
		ActorRole:      "tenant-admin",
		State:          assistantStateValidated,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		Transitions:    []assistantStateTransition{{FromState: "init", ToState: assistantStateValidated}},
	}
	createTx := &assistFakeTx{commitErr: errors.New("commit failed")}
	createTx.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
	createTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		if strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions") {
			return &assistFakeRow{err: errors.New("transition create failed")}
		}
		return &assistFakeRow{err: pgx.ErrNoRows}
	}
	svc.pool = assistFakeTxBeginner{tx: createTx}
	if err := svc.persistConversationCreate(context.Background(), conv); err == nil || !strings.Contains(err.Error(), "transition create failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	createTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		if strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions") {
			return &assistFakeRow{vals: []any{int64(1)}}
		}
		return &assistFakeRow{err: pgx.ErrNoRows}
	}
	if err := svc.persistConversationCreate(context.Background(), conv); err == nil || !strings.Contains(err.Error(), "commit failed") {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestAssistantPersistence_UpsertAndMutationBranchCoverage(t *testing.T) {
	ctx := context.Background()
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	tx := &assistFakeTx{}
	turn := &assistantTurn{
		TurnID:             "turn_1",
		UserInput:          "输入",
		State:              assistantStateValidated,
		RequestID:          "req_1",
		TraceID:            "trace_1",
		PolicyVersion:      capabilityPolicyVersionBaseline,
		CompositionVersion: capabilityPolicyVersionBaseline,
		MappingVersion:     capabilityPolicyVersionBaseline,
		Intent:             assistantIntentSpec{Action: "plan_only"},
		Plan:               assistantBuildPlan(assistantIntentSpec{Action: "plan_only"}),
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
	badPlan := *turn
	badPlan.Plan.ConfigDeltaPlan.Changes = []assistantConfigChange{{Field: "x", After: func() {}}}
	if err := svc.upsertTurnTx(ctx, tx, "tenant_1", "conv_1", &badPlan); err == nil {
		t.Fatal("expected plan marshal error")
	}

	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	conversation := &assistantConversation{ConversationID: "conv_1"}
	confirmedSingle := &assistantTurn{
		TurnID:              "turn_single",
		State:               assistantStateConfirmed,
		Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
		ResolvedCandidateID: "c1",
	}
	if _, err := svc.applyConfirmTurn(conversation, confirmedSingle, principal, ""); err != nil {
		t.Fatalf("unexpected err=%v", err)
	}

	validatedNoCandidate := &assistantTurn{
		TurnID:     "turn_no_candidate",
		State:      assistantStateValidated,
		Intent:     assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
		Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}},
	}
	if _, err := svc.applyConfirmTurn(conversation, validatedNoCandidate, principal, ""); !errors.Is(err, errAssistantConfirmationRequired) {
		t.Fatalf("unexpected err=%v", err)
	}

	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true)
	commitSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	commitTurn := &assistantTurn{
		TurnID:              "turn_commit_name",
		State:               assistantStateConfirmed,
		RequestID:           "req_commit_name",
		TraceID:             "trace_commit_name",
		Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
		Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
		ResolvedCandidateID: "c1",
		Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}},
		PolicyVersion:       capabilityPolicyVersionBaseline,
		CompositionVersion:  capabilityPolicyVersionBaseline,
		MappingVersion:      capabilityPolicyVersionBaseline,
	}
	commitTurn.Plan.SkillManifestDigest = "digest"
	if _, err := commitSvc.applyCommitTurn(context.Background(), conversation, commitTurn, principal, "tenant_1"); err != nil {
		t.Fatalf("unexpected err=%v", err)
	}
	if commitTurn.CommitResult == nil || commitTurn.CommitResult.ParentOrgCode != "FLOWER-A" {
		t.Fatalf("unexpected commit result=%+v", commitTurn.CommitResult)
	}
}

func TestAssistantPersistence_ConfirmCommitPGIdempotencyBranches(t *testing.T) {
	now := time.Now().UTC()
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true)
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}

	baseTurn := &assistantTurn{
		TurnID:             "turn_1",
		UserInput:          "输入",
		State:              assistantStateValidated,
		RiskTier:           "high",
		RequestID:          "req_1",
		TraceID:            "trace_1",
		PolicyVersion:      capabilityPolicyVersionBaseline,
		CompositionVersion: capabilityPolicyVersionBaseline,
		MappingVersion:     capabilityPolicyVersionBaseline,
		Intent: assistantIntentSpec{
			Action:              assistantIntentCreateOrgUnit,
			ParentRefText:       "鲜花组织",
			EntityName:          "运营部",
			EffectiveDate:       "2026-01-01",
			IntentSchemaVersion: assistantIntentSchemaVersionV1,
		},
		Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
		Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}, {CandidateID: "c2", CandidateCode: "FLOWER-B"}},
		AmbiguityCount:      2,
		ResolvedCandidateID: "",
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	baseTurn.Plan.SkillManifestDigest = "digest"

	makeTx := func(turn *assistantTurn, idemInsertErr error, idemSelectRow pgx.Row) *assistFakeTx {
		tx := &assistFakeTx{}
		tx.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_1", "actor_1", assistantStateValidated, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				if idemInsertErr != nil {
					return &assistFakeRow{err: idemInsertErr}
				}
				return &assistFakeRow{vals: []any{1}}
			case strings.Contains(sql, "SELECT request_hash"):
				return idemSelectRow
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				return &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil
			case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
				return &assistFakeRows{}, nil
			default:
				return &assistFakeRows{}, nil
			}
		}
		return tx
	}

	conflictTx := makeTx(baseTurn, pgx.ErrNoRows, &assistFakeRow{vals: []any{"other-hash", "done", 409, "", bodyOrNull(nil)}})
	svc.pool = assistFakeTxBeginner{tx: conflictTx}
	if _, err := svc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", baseTurn.TurnID, "c1"); !errors.Is(err, errAssistantIdempotencyKeyConflict) {
		t.Fatalf("unexpected err=%v", err)
	}

	inProgressTx := makeTx(baseTurn, pgx.ErrNoRows, &assistFakeRow{vals: []any{assistantHashText("confirm\nc1"), "pending", nil, nil, []byte(nil)}})
	svc.pool = assistFakeTxBeginner{tx: inProgressTx}
	if _, err := svc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", baseTurn.TurnID, "c1"); !errors.Is(err, errAssistantRequestInProgress) {
		t.Fatalf("unexpected err=%v", err)
	}

	doneBody, _ := json.Marshal(&assistantConversation{ConversationID: "conv_done"})
	doneTx := makeTx(baseTurn, pgx.ErrNoRows, &assistFakeRow{vals: []any{assistantHashText("confirm\nc1"), "done", 200, "", doneBody}})
	svc.pool = assistFakeTxBeginner{tx: doneTx}
	if got, err := svc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", baseTurn.TurnID, "c1"); err != nil || got.ConversationID != "conv_done" {
		t.Fatalf("unexpected result=%+v err=%v", got, err)
	}

	applyErrTurn := *baseTurn
	applyErrTx := makeTx(&applyErrTurn, nil, &assistFakeRow{err: pgx.ErrNoRows})
	svc.pool = assistFakeTxBeginner{tx: applyErrTx}
	if _, err := svc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", applyErrTurn.TurnID, ""); !errors.Is(err, errAssistantConfirmationRequired) {
		t.Fatalf("unexpected err=%v", err)
	}

	commitTurn := *baseTurn
	commitTurn.State = assistantStateConfirmed
	commitTurn.ResolvedCandidateID = "c1"
	commitTurn.AmbiguityCount = 1
	commitTurn.Plan.CompilerContractVersion = "compiler_contract_v0"
	commitTx := makeTx(&commitTurn, nil, &assistFakeRow{err: pgx.ErrNoRows})
	svc.pool = assistFakeTxBeginner{tx: commitTx}
	if _, err := svc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", commitTurn.TurnID); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestAssistantPersistence_ConfirmTurnPG_ErrorPathMatrix(t *testing.T) {
	now := time.Now().UTC()
	baseTurn := &assistantTurn{
		TurnID:             "turn_1",
		UserInput:          "输入",
		State:              assistantStateValidated,
		RequestID:          "req_1",
		TraceID:            "trace_1",
		Intent:             assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
		Plan:               assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
		Candidates:         []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}, {CandidateID: "c2", CandidateCode: "FLOWER-B"}},
		AmbiguityCount:     2,
		PolicyVersion:      capabilityPolicyVersionBaseline,
		CompositionVersion: capabilityPolicyVersionBaseline,
		MappingVersion:     capabilityPolicyVersionBaseline,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	baseTurn.Plan.SkillManifestDigest = "digest"

	makeSvc := func(actorID string, turnRows [][]any, execFn func(string) error, queryRowFn func(string) pgx.Row, commitErr error) *assistantConversationService {
		tx := &assistFakeTx{commitErr: commitErr}
		tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if execFn != nil {
				if err := execFn(sql); err != nil {
					return pgconn.NewCommandTag(""), err
				}
			}
			return pgconn.NewCommandTag(""), nil
		}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if queryRowFn != nil {
				if row := queryRowFn(sql); row != nil {
					return row
				}
			}
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_1", actorID, assistantStateValidated, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{vals: []any{1}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				return &assistFakeRows{rows: turnRows}, nil
			case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
				return &assistFakeRows{}, nil
			default:
				return &assistFakeRows{}, nil
			}
		}
		svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		svc.pool = assistFakeTxBeginner{tx: tx}
		return svc
	}

	row := assistantTurnRowValues(baseTurn)
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}

	loadErrSvc := makeSvc("actor_1", nil, nil, func(sql string) pgx.Row {
		if strings.Contains(sql, "FROM iam.assistant_conversations") {
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		return nil
	}, nil)
	if _, err := loadErrSvc.confirmTurnPG(nil, "tenant_1", principal, "conv_1", "turn_1", "c1"); !errors.Is(err, errAssistantConversationNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}

	forbiddenSvc := makeSvc("actor_x", [][]any{row}, nil, nil, nil)
	if _, err := forbiddenSvc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1", "c1"); !errors.Is(err, errAssistantConversationForbidden) {
		t.Fatalf("unexpected err=%v", err)
	}

	turnMissingSvc := makeSvc("actor_1", nil, nil, nil, nil)
	if _, err := turnMissingSvc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1", "c1"); !errors.Is(err, errAssistantTurnNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}

	claimErrSvc := makeSvc("actor_1", [][]any{row}, nil, func(sql string) pgx.Row {
		if strings.Contains(sql, "INSERT INTO iam.assistant_idempotency") {
			return &assistFakeRow{err: errors.New("claim failed")}
		}
		return nil
	}, nil)
	if _, err := claimErrSvc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1", "c1"); err == nil || !strings.Contains(err.Error(), "claim failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	finalizeErrSvc := makeSvc("actor_1", [][]any{row}, func(sql string) error {
		if strings.Contains(sql, "UPDATE iam.assistant_idempotency") {
			return errors.New("finalize failed")
		}
		return nil
	}, nil, nil)
	if _, err := finalizeErrSvc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1", ""); err == nil || !strings.Contains(err.Error(), "finalize failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	applyCommitErrSvc := makeSvc("actor_1", [][]any{row}, nil, nil, errors.New("commit failed"))
	if _, err := applyCommitErrSvc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1", ""); err == nil || !strings.Contains(err.Error(), "commit failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	successUpsertErrSvc := makeSvc("actor_1", [][]any{row}, func(sql string) error {
		if strings.Contains(sql, "INSERT INTO iam.assistant_turns") {
			return errors.New("upsert failed")
		}
		return nil
	}, nil, nil)
	if _, err := successUpsertErrSvc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1", "c1"); err == nil || !strings.Contains(err.Error(), "upsert failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	successUpdateErrSvc := makeSvc("actor_1", [][]any{row}, func(sql string) error {
		if strings.Contains(sql, "UPDATE iam.assistant_conversations") {
			return errors.New("update failed")
		}
		return nil
	}, nil, nil)
	if _, err := successUpdateErrSvc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1", "c1"); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	successTransitionErrSvc := makeSvc("actor_1", [][]any{row}, nil, func(sql string) pgx.Row {
		if strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions") {
			return &assistFakeRow{err: errors.New("transition failed")}
		}
		return nil
	}, nil)
	if _, err := successTransitionErrSvc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1", "c1"); err == nil || !strings.Contains(err.Error(), "transition failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	successFinalizeErrSvc := makeSvc("actor_1", [][]any{row}, func(sql string) error {
		if strings.Contains(sql, "UPDATE iam.assistant_idempotency") {
			return errors.New("finalize success failed")
		}
		return nil
	}, nil, nil)
	if _, err := successFinalizeErrSvc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1", "c1"); err == nil || !strings.Contains(err.Error(), "finalize success failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	successCommitErrSvc := makeSvc("actor_1", [][]any{row}, nil, nil, errors.New("commit success failed"))
	if _, err := successCommitErrSvc.confirmTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1", "c1"); err == nil || !strings.Contains(err.Error(), "commit success failed") {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestAssistantPersistence_CommitTurnPG_ErrorPathMatrix(t *testing.T) {
	now := time.Now().UTC()
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true)
	baseTurn := &assistantTurn{
		TurnID:              "turn_1",
		UserInput:           "输入",
		State:               assistantStateConfirmed,
		RequestID:           "req_1",
		TraceID:             "trace_1",
		Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01", IntentSchemaVersion: assistantIntentSchemaVersionV1},
		Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
		Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}},
		ResolvedCandidateID: "c1",
		PolicyVersion:       capabilityPolicyVersionBaseline,
		CompositionVersion:  capabilityPolicyVersionBaseline,
		MappingVersion:      capabilityPolicyVersionBaseline,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	baseTurn.Plan.SkillManifestDigest = "digest"
	baseTurn.Plan.CompilerContractVersion = "compiler_contract_v0"

	makeSvc := func(actorID, actorRole string, turnRows [][]any, execFn func(string) error, queryRowFn func(string) pgx.Row, commitErr error) *assistantConversationService {
		tx := &assistFakeTx{commitErr: commitErr}
		tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if execFn != nil {
				if err := execFn(sql); err != nil {
					return pgconn.NewCommandTag(""), err
				}
			}
			return pgconn.NewCommandTag(""), nil
		}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if queryRowFn != nil {
				if row := queryRowFn(sql); row != nil {
					return row
				}
			}
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"conv_1", "tenant_1", actorID, actorRole, assistantStateValidated, now, now}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{vals: []any{1}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				return &assistFakeRows{rows: turnRows}, nil
			case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
				return &assistFakeRows{}, nil
			default:
				return &assistFakeRows{}, nil
			}
		}
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		svc.pool = assistFakeTxBeginner{tx: tx}
		return svc
	}

	row := assistantTurnRowValues(baseTurn)
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}

	loadErrSvc := makeSvc("actor_1", "tenant-admin", nil, nil, func(sql string) pgx.Row {
		if strings.Contains(sql, "FROM iam.assistant_conversations") {
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		return nil
	}, nil)
	if _, err := loadErrSvc.commitTurnPG(nil, "tenant_1", principal, "conv_1", "turn_1"); !errors.Is(err, errAssistantConversationNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}

	authErrSvc := makeSvc("actor_x", "tenant-admin", [][]any{row}, nil, nil, nil)
	if _, err := authErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); !errors.Is(err, errAssistantAuthSnapshotExpired) {
		t.Fatalf("unexpected err=%v", err)
	}

	roleErrSvc := makeSvc("actor_1", "viewer", [][]any{row}, nil, nil, nil)
	if _, err := roleErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); !errors.Is(err, errAssistantRoleDriftDetected) {
		t.Fatalf("unexpected err=%v", err)
	}

	turnMissingSvc := makeSvc("actor_1", "tenant-admin", nil, nil, nil, nil)
	if _, err := turnMissingSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); !errors.Is(err, errAssistantTurnNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}

	claimErrSvc := makeSvc("actor_1", "tenant-admin", [][]any{row}, nil, func(sql string) pgx.Row {
		if strings.Contains(sql, "INSERT INTO iam.assistant_idempotency") {
			return &assistFakeRow{err: errors.New("claim failed")}
		}
		return nil
	}, nil)
	if _, err := claimErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "claim failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	doneBody, _ := json.Marshal(&assistantConversation{ConversationID: "conv_done"})
	doneSvc := makeSvc("actor_1", "tenant-admin", [][]any{row}, nil, func(sql string) pgx.Row {
		switch {
		case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
			return &assistFakeRow{err: pgx.ErrNoRows}
		case strings.Contains(sql, "SELECT request_hash"):
			return &assistFakeRow{vals: []any{assistantHashText("commit\n"), "done", 200, "", doneBody}}
		default:
			return nil
		}
	}, nil)
	if got, err := doneSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err != nil || got.ConversationID != "conv_done" {
		t.Fatalf("unexpected result=%+v err=%v", got, err)
	}

	applyUpsertErrSvc := makeSvc("actor_1", "tenant-admin", [][]any{row}, func(sql string) error {
		if strings.Contains(sql, "INSERT INTO iam.assistant_turns") {
			return errors.New("upsert failed")
		}
		return nil
	}, nil, nil)
	if _, err := applyUpsertErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "upsert failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	applyUpdateErrSvc := makeSvc("actor_1", "tenant-admin", [][]any{row}, func(sql string) error {
		if strings.Contains(sql, "UPDATE iam.assistant_conversations") {
			return errors.New("update failed")
		}
		return nil
	}, nil, nil)
	if _, err := applyUpdateErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	applyTransitionErrSvc := makeSvc("actor_1", "tenant-admin", [][]any{row}, nil, func(sql string) pgx.Row {
		if strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions") {
			return &assistFakeRow{err: errors.New("transition failed")}
		}
		return nil
	}, nil)
	if _, err := applyTransitionErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "transition failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	applyFinalizeErrSvc := makeSvc("actor_1", "tenant-admin", [][]any{row}, func(sql string) error {
		if strings.Contains(sql, "UPDATE iam.assistant_idempotency") {
			return errors.New("finalize failed")
		}
		return nil
	}, nil, nil)
	if _, err := applyFinalizeErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "finalize failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	applyCommitErrSvc := makeSvc("actor_1", "tenant-admin", [][]any{row}, nil, nil, errors.New("commit failed"))
	if _, err := applyCommitErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "commit failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	successTurn := *baseTurn
	successTurn.Plan.CompilerContractVersion = assistantCompilerContractVersionV1
	successTurn.Plan.CapabilityMapVersion = assistantCapabilityMapVersionV1
	successTurn.Plan.SkillManifestDigest = "digest"
	successTurn.Intent.IntentSchemaVersion = assistantIntentSchemaVersionV1
	successRow := assistantTurnRowValues(&successTurn)

	successPathUpsertErrSvc := makeSvc("actor_1", "tenant-admin", [][]any{successRow}, func(sql string) error {
		if strings.Contains(sql, "INSERT INTO iam.assistant_turns") {
			return errors.New("upsert success-path failed")
		}
		return nil
	}, nil, nil)
	if _, err := successPathUpsertErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "upsert success-path failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	successPathUpdateErrSvc := makeSvc("actor_1", "tenant-admin", [][]any{successRow}, func(sql string) error {
		if strings.Contains(sql, "UPDATE iam.assistant_conversations") {
			return errors.New("update success-path failed")
		}
		return nil
	}, nil, nil)
	if _, err := successPathUpdateErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "update success-path failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	successPathTransitionErrSvc := makeSvc("actor_1", "tenant-admin", [][]any{successRow}, nil, func(sql string) pgx.Row {
		if strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions") {
			return &assistFakeRow{err: errors.New("transition success-path failed")}
		}
		return nil
	}, nil)
	if _, err := successPathTransitionErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "transition success-path failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	successPathFinalizeErrSvc := makeSvc("actor_1", "tenant-admin", [][]any{successRow}, func(sql string) error {
		if strings.Contains(sql, "UPDATE iam.assistant_idempotency") {
			return errors.New("finalize success-path failed")
		}
		return nil
	}, nil, nil)
	if _, err := successPathFinalizeErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "finalize success-path failed") {
		t.Fatalf("unexpected err=%v", err)
	}

	successPathCommitErrSvc := makeSvc("actor_1", "tenant-admin", [][]any{successRow}, nil, nil, errors.New("commit success-path failed"))
	if _, err := successPathCommitErrSvc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "commit success-path failed") {
		t.Fatalf("unexpected err=%v", err)
	}
}
