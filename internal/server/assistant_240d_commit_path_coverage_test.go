package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func assistantConversationRowWithRole(conversationID string, actorID string, actorRole string, state string, now time.Time) []any {
	return []any{conversationID, "tenant_1", actorID, actorRole, state, assistantConversationPhaseFromLegacyState(state), now, now}
}

func newAssistantCommitCoverageEnv(t *testing.T, turnState string) (*assistantConversationService, Principal, *assistantTurn, time.Time) {
	t.Helper()
	now := time.Now().UTC()
	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatalf("create parent org err=%v", err)
	}
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	turn := assistantTaskSampleTurn(now)
	turn.State = turnState
	turn.UserInput = "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"
	turn.Intent = assistantIntentSpec{
		Action:              assistantIntentCreateOrgUnit,
		IntentSchemaVersion: assistantIntentSchemaVersionV1,
		ContextHash:         "ctx_hash",
		IntentHash:          "intent_hash",
		ParentRefText:       "鲜花组织",
		EntityName:          "运营部",
		EffectiveDate:       "2026-01-01",
	}
	turn.Plan = assistantBuildPlan(turn.Intent)
	turn.Plan.SkillManifestDigest = "skill_digest"
	turn.Candidates = []assistantCandidate{{CandidateID: "FLOWER-A", CandidateCode: "FLOWER-A", Name: "鲜花组织", OrgID: 0, IsActive: true}}
	turn.ResolvedCandidateID = "FLOWER-A"
	turn.AmbiguityCount = 0
	turn.RequestID = "req_commit"
	turn.TraceID = "trace_commit"
	turn.PolicyVersion = capabilityPolicyVersionBaseline
	turn.CompositionVersion = capabilityPolicyVersionBaseline
	turn.MappingVersion = capabilityPolicyVersionBaseline
	turn.CreatedAt = now
	turn.UpdatedAt = now
	assistantTestAttachBusinessRoute(turn)
	if err := svc.refreshTurnVersionTuple(context.Background(), "tenant_1", turn); err != nil {
		t.Fatalf("refreshTurnVersionTuple err=%v", err)
	}
	return svc, principal, turn, now
}

func newAssistantCommitTx(now time.Time, actorID string, actorRole string, conversationState string, turnRows *assistFakeRows) *assistFakeTx {
	tx := &assistFakeTx{}
	tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		switch {
		case strings.Contains(sql, "FROM iam.assistant_conversations"):
			return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", actorID, actorRole, conversationState, now)}
		case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
			return &assistFakeRow{vals: []any{1}}
		case strings.Contains(sql, "SELECT request_hash, status, http_status, error_code, response_body"):
			return &assistFakeRow{err: pgx.ErrNoRows}
		case strings.Contains(sql, "FROM iam.assistant_tasks"):
			return &assistFakeRow{err: pgx.ErrNoRows}
		case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
			return &assistFakeRow{vals: []any{int64(1)}}
		case strings.Contains(sql, "SELECT intent_json"):
			return &assistFakeRow{err: pgx.ErrNoRows}
		default:
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
	}
	tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
		switch {
		case strings.Contains(sql, "FROM iam.assistant_turns"):
			if turnRows != nil {
				return turnRows, nil
			}
			return &assistFakeRows{}, nil
		case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
			return &assistFakeRows{}, nil
		default:
			return &assistFakeRows{}, nil
		}
	}
	tx.execFn = func(string, ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), nil
	}
	return tx
}

func TestAssistant240DRequestAndCommitPGCoverage(t *testing.T) {
	now := time.Now().UTC()
	turn := assistantTaskSampleTurn(now)
	turn.RequestID = ""
	if _, err := assistantBuildTaskSubmitRequestFromTurn("conv_1", turn); err == nil || !strings.Contains(err.Error(), "request_id required") {
		t.Fatalf("expected request validation err, got %v", err)
	}

	svc, principal, validatedTurn, validatedAt := newAssistantCommitCoverageEnv(t, assistantStateValidated)
	svc.pool = assistFakeTxBeginner{err: errors.New("begin failed")}
	if _, err := svc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", validatedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "begin failed") {
		t.Fatalf("unexpected begin err=%v", err)
	}

	tx := newAssistantCommitTx(validatedAt, principal.ID, principal.RoleSlug, assistantStateValidated, &assistFakeRows{rows: [][]any{assistantTurnRowValues(validatedTurn)}})
	svc.pool = assistFakeTxBeginner{tx: tx}
	if _, err := svc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", validatedTurn.TurnID); !errors.Is(err, errAssistantConfirmationRequired) {
		t.Fatalf("unexpected commit apply err=%v", err)
	}
	if !tx.committed {
		t.Fatal("expected commitTurnPG applyErr path to commit transaction")
	}

	finalizeErrTx := newAssistantCommitTx(validatedAt, principal.ID, principal.RoleSlug, assistantStateValidated, &assistFakeRows{rows: [][]any{assistantTurnRowValues(validatedTurn)}})
	finalizeErrTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
		if strings.Contains(sql, "UPDATE iam.assistant_idempotency") {
			return pgconn.NewCommandTag(""), errors.New("commit finalize failed")
		}
		return pgconn.NewCommandTag(""), nil
	}
	svc.pool = assistFakeTxBeginner{tx: finalizeErrTx}
	if _, err := svc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", validatedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "commit finalize failed") {
		t.Fatalf("unexpected commit finalize err=%v", err)
	}

	commitErrTx := newAssistantCommitTx(validatedAt, principal.ID, principal.RoleSlug, assistantStateValidated, &assistFakeRows{rows: [][]any{assistantTurnRowValues(validatedTurn)}})
	commitErrTx.commitErr = errors.New("commit apply failed")
	svc.pool = assistFakeTxBeginner{tx: commitErrTx}
	if _, err := svc.commitTurnPG(context.Background(), "tenant_1", principal, "conv_1", validatedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "commit apply failed") {
		t.Fatalf("unexpected commit apply commit err=%v", err)
	}
}

func TestAssistant240DSubmitCommitTaskPGCoverage(t *testing.T) {
	t.Run("direct branches", func(t *testing.T) {
		svc, principal, confirmedTurn, now := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		makeTurnRows := func(turn *assistantTurn) *assistFakeRows {
			return &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}
		}

		if _, err := svc.submitCommitTaskPG(nil, "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "beginAssistantTx") && !strings.Contains(err.Error(), "assistant_service_missing") {
			t.Fatalf("unexpected nil ctx service err=%v", err)
		}

		svc.pool = assistFakeTxBeginner{err: errors.New("begin failed")}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("unexpected begin err=%v", err)
		}

		loadErrTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		loadErrTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "FROM iam.assistant_conversations") {
				return &assistFakeRow{err: errors.New("load conversation failed")}
			}
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		svc.pool = assistFakeTxBeginner{tx: loadErrTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "load conversation failed") {
			t.Fatalf("unexpected load err=%v", err)
		}

		svc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, "actor_2", principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); !errors.Is(err, errAssistantAuthSnapshotExpired) {
			t.Fatalf("unexpected auth snapshot err=%v", err)
		}

		svc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, "viewer", assistantStateConfirmed, makeTurnRows(confirmedTurn))}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); !errors.Is(err, errAssistantRoleDriftDetected) {
			t.Fatalf("unexpected role drift err=%v", err)
		}

		svc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{})}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); !errors.Is(err, errAssistantTurnNotFound) {
			t.Fatalf("unexpected turn missing err=%v", err)
		}

		claimErrTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		claimErrTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", principal.ID, principal.RoleSlug, assistantStateConfirmed, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{err: errors.New("claim failed")}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		svc.pool = assistFakeTxBeginner{tx: claimErrTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "claim failed") {
			t.Fatalf("unexpected claim err=%v", err)
		}

		claimConflictTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		claimConflictTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", principal.ID, principal.RoleSlug, assistantStateConfirmed, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{err: pgx.ErrNoRows}
			case strings.Contains(sql, "SELECT request_hash, status"):
				return &assistFakeRow{vals: []any{"other_hash", "done", 202, nil, []byte("null")}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		svc.pool = assistFakeTxBeginner{tx: claimConflictTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); !errors.Is(err, errAssistantIdempotencyKeyConflict) {
			t.Fatalf("unexpected claim conflict err=%v", err)
		}

		claimInProgressTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		claimInProgressTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", principal.ID, principal.RoleSlug, assistantStateConfirmed, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{err: pgx.ErrNoRows}
			case strings.Contains(sql, "SELECT request_hash, status"):
				return &assistFakeRow{vals: []any{assistantHashText("commit\n"), "pending", nil, nil, []byte(nil)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		svc.pool = assistFakeTxBeginner{tx: claimInProgressTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); !errors.Is(err, errAssistantRequestInProgress) {
			t.Fatalf("unexpected in progress err=%v", err)
		}

		receipt := assistantTaskAsyncReceipt{TaskID: "task_done", TaskType: assistantTaskTypeAsyncPlan, Status: assistantTaskStatusQueued, WorkflowID: "wf_done", SubmittedAt: now, PollURI: "/internal/assistant/tasks/task_done"}
		receiptBody, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		claimDoneTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		claimDoneTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", principal.ID, principal.RoleSlug, assistantStateConfirmed, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{err: pgx.ErrNoRows}
			case strings.Contains(sql, "SELECT request_hash, status"):
				return &assistFakeRow{vals: []any{assistantHashText("commit\n"), "done", 202, nil, receiptBody}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		svc.pool = assistFakeTxBeginner{tx: claimDoneTx}
		if got, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err != nil || got.TaskID != receipt.TaskID {
			t.Fatalf("unexpected done receipt=%+v err=%v", got, err)
		}

		validatedSvc, validatedPrincipal, validatedTurn, validatedNow := newAssistantCommitCoverageEnv(t, assistantStateValidated)
		validatedTx := newAssistantCommitTx(validatedNow, validatedPrincipal.ID, validatedPrincipal.RoleSlug, assistantStateValidated, &assistFakeRows{rows: [][]any{assistantTurnRowValues(validatedTurn)}})
		validatedSvc.pool = assistFakeTxBeginner{tx: validatedTx}
		if _, err := validatedSvc.submitCommitTaskPG(context.Background(), "tenant_1", validatedPrincipal, "conv_1", validatedTurn.TurnID); !errors.Is(err, errAssistantConfirmationRequired) {
			t.Fatalf("unexpected preErr err=%v", err)
		}
		if !validatedTx.committed {
			t.Fatal("expected preErr path to commit transaction")
		}

		committedSvc, committedPrincipal, committedTurn, committedNow := newAssistantCommitCoverageEnv(t, assistantStateCommitted)
		committedTx := newAssistantCommitTx(committedNow, committedPrincipal.ID, committedPrincipal.RoleSlug, assistantStateCommitted, &assistFakeRows{rows: [][]any{assistantTurnRowValues(committedTurn)}})
		committedSvc.pool = assistFakeTxBeginner{tx: committedTx}
		if _, err := committedSvc.submitCommitTaskPG(context.Background(), "tenant_1", committedPrincipal, "conv_1", committedTurn.TurnID); !errors.Is(err, errAssistantTaskStateInvalid) {
			t.Fatalf("unexpected skip execution err=%v", err)
		}

		missingReqSvc, missingReqPrincipal, missingReqTurn, missingReqNow := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		missingReqTurn.RequestID = ""
		missingReqTx := newAssistantCommitTx(missingReqNow, missingReqPrincipal.ID, missingReqPrincipal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(missingReqTurn)}})
		missingReqSvc.pool = assistFakeTxBeginner{tx: missingReqTx}
		if _, err := missingReqSvc.submitCommitTaskPG(context.Background(), "tenant_1", missingReqPrincipal, "conv_1", missingReqTurn.TurnID); err == nil || !strings.Contains(err.Error(), "request_id required") {
			t.Fatalf("unexpected build request err=%v", err)
		}

		marshalErrSvc, marshalErrPrincipal, marshalErrTurn, marshalErrNow := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		marshalErrTx := newAssistantCommitTx(marshalErrNow, marshalErrPrincipal.ID, marshalErrPrincipal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(marshalErrTurn)}})
		marshalErrSvc.pool = assistFakeTxBeginner{tx: marshalErrTx}
		origMarshalFn := assistantTaskMarshalFn
		assistantTaskMarshalFn = func(any) ([]byte, error) { return nil, errors.New("hash marshal failed") }
		if _, err := marshalErrSvc.submitCommitTaskPG(context.Background(), "tenant_1", marshalErrPrincipal, "conv_1", marshalErrTurn.TurnID); err == nil || !strings.Contains(err.Error(), "hash marshal failed") {
			t.Fatalf("unexpected hash err=%v", err)
		}
		assistantTaskMarshalFn = origMarshalFn

		submitKeyErrTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		submitKeyErrTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", principal.ID, principal.RoleSlug, assistantStateConfirmed, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{vals: []any{1}}
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{err: errors.New("submit key query failed")}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		svc.pool = assistFakeTxBeginner{tx: submitKeyErrTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "submit key query failed") {
			t.Fatalf("unexpected submit key err=%v", err)
		}

		req, err := assistantBuildTaskSubmitRequestFromTurn("conv_1", confirmedTurn)
		if err != nil {
			t.Fatal(err)
		}
		hash, err := assistantTaskRequestHash(req)
		if err != nil {
			t.Fatal(err)
		}
		existing := assistantTaskRecord{
			TaskID:             "task_existing",
			TenantID:           "tenant_1",
			ConversationID:     "conv_1",
			TurnID:             confirmedTurn.TurnID,
			TaskType:           assistantTaskTypeAsyncPlan,
			RequestID:          confirmedTurn.RequestID,
			RequestHash:        "other_hash",
			WorkflowID:         "wf_existing",
			Status:             assistantTaskStatusQueued,
			DispatchStatus:     assistantTaskDispatchPending,
			DispatchDeadlineAt: now.Add(time.Minute),
			MaxAttempts:        2,
			ContractSnapshot:   assistantTaskSnapshotFromTurn(confirmedTurn),
			SubmittedAt:        now,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		conflictExistingTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		conflictExistingTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", principal.ID, principal.RoleSlug, assistantStateConfirmed, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{vals: []any{1}}
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(existing)}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		svc.pool = assistFakeTxBeginner{tx: conflictExistingTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); !errors.Is(err, errAssistantIdempotencyKeyConflict) {
			t.Fatalf("unexpected existing conflict err=%v", err)
		}

		existing.RequestHash = hash
		existingTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		existingTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", principal.ID, principal.RoleSlug, assistantStateConfirmed, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{vals: []any{1}}
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(existing)}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		svc.pool = assistFakeTxBeginner{tx: existingTx}
		if got, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err != nil || got.TaskID != existing.TaskID {
			t.Fatalf("unexpected existing receipt=%+v err=%v", got, err)
		}

		insertErrTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		insertErrTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO iam.assistant_tasks") {
				return pgconn.NewCommandTag(""), errors.New("insert task failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		svc.pool = assistFakeTxBeginner{tx: insertErrTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "insert task failed") {
			t.Fatalf("unexpected insert graph err=%v", err)
		}

		finalizeErrTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		finalizeErrTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "UPDATE iam.assistant_idempotency") {
				return pgconn.NewCommandTag(""), errors.New("finalize receipt failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		svc.pool = assistFakeTxBeginner{tx: finalizeErrTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "finalize receipt failed") {
			t.Fatalf("unexpected finalize receipt err=%v", err)
		}

		commitErrTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		commitErrTx.commitErr = errors.New("create commit failed")
		svc.pool = assistFakeTxBeginner{tx: commitErrTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "create commit failed") {
			t.Fatalf("unexpected commit err=%v", err)
		}

		preErrPersistSvc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		preErrPersistPrincipal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
		preErrPersistNow := time.Now().UTC().Add(-time.Hour)
		preErrPersistTurn := &assistantTurn{
			TurnID:    "turn_preerr_expired",
			State:     assistantStateValidated,
			RequestID: "req_preerr_expired",
			TraceID:   "trace_preerr_expired",
			Intent:    assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			Plan:      assistantFreezeConfirmWindow(assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), preErrPersistNow),
			CreatedAt: preErrPersistNow,
			UpdatedAt: preErrPersistNow,
			RiskTier:  "low",
		}
		preErrPersistTx := newAssistantCommitTx(preErrPersistNow, preErrPersistPrincipal.ID, preErrPersistPrincipal.RoleSlug, assistantStateValidated, &assistFakeRows{rows: [][]any{assistantTurnRowValues(preErrPersistTurn)}})
		preErrPersistTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO iam.assistant_turns") {
				return pgconn.NewCommandTag(""), errors.New("persist mutation failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		preErrPersistSvc.pool = assistFakeTxBeginner{tx: preErrPersistTx}
		if _, err := preErrPersistSvc.submitCommitTaskPG(context.Background(), "tenant_1", preErrPersistPrincipal, "conv_1", preErrPersistTurn.TurnID); err == nil || !strings.Contains(err.Error(), "persist mutation failed") {
			t.Fatalf("unexpected preErr persist err=%v", err)
		}

		preErrFinalizeTx := newAssistantCommitTx(validatedNow, validatedPrincipal.ID, validatedPrincipal.RoleSlug, assistantStateValidated, &assistFakeRows{rows: [][]any{assistantTurnRowValues(validatedTurn)}})
		preErrFinalizeTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "UPDATE iam.assistant_idempotency") {
				return pgconn.NewCommandTag(""), errors.New("preErr finalize failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		validatedSvc.pool = assistFakeTxBeginner{tx: preErrFinalizeTx}
		if _, err := validatedSvc.submitCommitTaskPG(context.Background(), "tenant_1", validatedPrincipal, "conv_1", validatedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "preErr finalize failed") {
			t.Fatalf("unexpected preErr finalize err=%v", err)
		}

		preErrCommitTx := newAssistantCommitTx(validatedNow, validatedPrincipal.ID, validatedPrincipal.RoleSlug, assistantStateValidated, &assistFakeRows{rows: [][]any{assistantTurnRowValues(validatedTurn)}})
		preErrCommitTx.commitErr = errors.New("preErr commit failed")
		validatedSvc.pool = assistFakeTxBeginner{tx: preErrCommitTx}
		if _, err := validatedSvc.submitCommitTaskPG(context.Background(), "tenant_1", validatedPrincipal, "conv_1", validatedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "preErr commit failed") {
			t.Fatalf("unexpected preErr commit err=%v", err)
		}

		skipFinalizeTx := newAssistantCommitTx(committedNow, committedPrincipal.ID, committedPrincipal.RoleSlug, assistantStateCommitted, &assistFakeRows{rows: [][]any{assistantTurnRowValues(committedTurn)}})
		skipFinalizeTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "DELETE FROM iam.assistant_idempotency") {
				return pgconn.NewCommandTag(""), errors.New("skip finalize failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		committedSvc.pool = assistFakeTxBeginner{tx: skipFinalizeTx}
		if _, err := committedSvc.submitCommitTaskPG(context.Background(), "tenant_1", committedPrincipal, "conv_1", committedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "skip finalize failed") {
			t.Fatalf("unexpected skip finalize err=%v", err)
		}

		skipCommitTx := newAssistantCommitTx(committedNow, committedPrincipal.ID, committedPrincipal.RoleSlug, assistantStateCommitted, &assistFakeRows{rows: [][]any{assistantTurnRowValues(committedTurn)}})
		skipCommitTx.commitErr = errors.New("skip commit failed")
		committedSvc.pool = assistFakeTxBeginner{tx: skipCommitTx}
		if _, err := committedSvc.submitCommitTaskPG(context.Background(), "tenant_1", committedPrincipal, "conv_1", committedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "skip commit failed") {
			t.Fatalf("unexpected skip commit err=%v", err)
		}

		missingReqFinalizeTx := newAssistantCommitTx(missingReqNow, missingReqPrincipal.ID, missingReqPrincipal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(missingReqTurn)}})
		missingReqFinalizeTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "DELETE FROM iam.assistant_idempotency") {
				return pgconn.NewCommandTag(""), errors.New("build request finalize failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		missingReqSvc.pool = assistFakeTxBeginner{tx: missingReqFinalizeTx}
		if _, err := missingReqSvc.submitCommitTaskPG(context.Background(), "tenant_1", missingReqPrincipal, "conv_1", missingReqTurn.TurnID); err == nil || !strings.Contains(err.Error(), "build request finalize failed") {
			t.Fatalf("unexpected build request finalize err=%v", err)
		}

		missingReqCommitTx := newAssistantCommitTx(missingReqNow, missingReqPrincipal.ID, missingReqPrincipal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(missingReqTurn)}})
		missingReqCommitTx.commitErr = errors.New("build request commit failed")
		missingReqSvc.pool = assistFakeTxBeginner{tx: missingReqCommitTx}
		if _, err := missingReqSvc.submitCommitTaskPG(context.Background(), "tenant_1", missingReqPrincipal, "conv_1", missingReqTurn.TurnID); err == nil || !strings.Contains(err.Error(), "build request commit failed") {
			t.Fatalf("unexpected build request commit err=%v", err)
		}

		marshalFinalizeTx := newAssistantCommitTx(marshalErrNow, marshalErrPrincipal.ID, marshalErrPrincipal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(marshalErrTurn)}})
		marshalFinalizeTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "DELETE FROM iam.assistant_idempotency") {
				return pgconn.NewCommandTag(""), errors.New("hash finalize failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		marshalErrSvc.pool = assistFakeTxBeginner{tx: marshalFinalizeTx}
		assistantTaskMarshalFn = func(any) ([]byte, error) { return nil, errors.New("hash marshal failed") }
		if _, err := marshalErrSvc.submitCommitTaskPG(context.Background(), "tenant_1", marshalErrPrincipal, "conv_1", marshalErrTurn.TurnID); err == nil || !strings.Contains(err.Error(), "hash finalize failed") {
			t.Fatalf("unexpected hash finalize err=%v", err)
		}
		assistantTaskMarshalFn = origMarshalFn

		marshalCommitTx := newAssistantCommitTx(marshalErrNow, marshalErrPrincipal.ID, marshalErrPrincipal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(marshalErrTurn)}})
		marshalCommitTx.commitErr = errors.New("hash commit failed")
		marshalErrSvc.pool = assistFakeTxBeginner{tx: marshalCommitTx}
		assistantTaskMarshalFn = func(any) ([]byte, error) { return nil, errors.New("hash marshal failed") }
		if _, err := marshalErrSvc.submitCommitTaskPG(context.Background(), "tenant_1", marshalErrPrincipal, "conv_1", marshalErrTurn.TurnID); err == nil || !strings.Contains(err.Error(), "hash commit failed") {
			t.Fatalf("unexpected hash commit err=%v", err)
		}
		assistantTaskMarshalFn = origMarshalFn

		existing.RequestHash = "mismatch_hash"
		conflictFinalizeTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		conflictFinalizeTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", principal.ID, principal.RoleSlug, assistantStateConfirmed, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{vals: []any{1}}
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(existing)}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		conflictFinalizeTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "UPDATE iam.assistant_idempotency") || strings.Contains(sql, "DELETE FROM iam.assistant_idempotency") {
				return pgconn.NewCommandTag(""), errors.New("existing conflict finalize failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		svc.pool = assistFakeTxBeginner{tx: conflictFinalizeTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "existing conflict finalize failed") {
			t.Fatalf("unexpected existing conflict finalize err=%v", err)
		}

		conflictCommitTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		conflictCommitTx.queryRowFn = conflictFinalizeTx.queryRowFn
		conflictCommitTx.commitErr = errors.New("existing conflict commit failed")
		svc.pool = assistFakeTxBeginner{tx: conflictCommitTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "existing conflict commit failed") {
			t.Fatalf("unexpected existing conflict commit err=%v", err)
		}

		existing.RequestHash = hash
		existingFinalizeTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		existingFinalizeTx.queryRowFn = existingTx.queryRowFn
		existingFinalizeTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "UPDATE iam.assistant_idempotency") {
				return pgconn.NewCommandTag(""), errors.New("existing receipt finalize failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		svc.pool = assistFakeTxBeginner{tx: existingFinalizeTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "existing receipt finalize failed") {
			t.Fatalf("unexpected existing receipt finalize err=%v", err)
		}

		existingCommitTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		existingCommitTx.queryRowFn = existingTx.queryRowFn
		existingCommitTx.commitErr = errors.New("existing receipt commit failed")
		svc.pool = assistFakeTxBeginner{tx: existingCommitTx}
		if _, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err == nil || !strings.Contains(err.Error(), "existing receipt commit failed") {
			t.Fatalf("unexpected existing receipt commit err=%v", err)
		}

		successTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, makeTurnRows(confirmedTurn))
		svc.pool = assistFakeTxBeginner{tx: successTx}
		if got, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", principal, "conv_1", confirmedTurn.TurnID); err != nil || strings.TrimSpace(got.TaskID) == "" {
			t.Fatalf("unexpected success receipt=%+v err=%v", got, err)
		}
	})
}

func TestAssistant240DExecuteWorkflowAndAPICommitCoverage(t *testing.T) {
	t.Run("workflow branches", func(t *testing.T) {
		svc, _, confirmedTurn, now := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		task := assistantTaskRecord{
			TaskID:             "task_1",
			TenantID:           "tenant_1",
			ConversationID:     "conv_1",
			TurnID:             confirmedTurn.TurnID,
			TaskType:           assistantTaskTypeAsyncPlan,
			RequestID:          confirmedTurn.RequestID,
			RequestHash:        "hash_1",
			WorkflowID:         "wf_1",
			Status:             assistantTaskStatusQueued,
			DispatchStatus:     assistantTaskDispatchPending,
			DispatchDeadlineAt: now.Add(time.Minute),
			MaxAttempts:        2,
			ContractSnapshot:   assistantTaskSnapshotFromTurn(confirmedTurn),
			SubmittedAt:        now,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		intentJSON, _ := json.Marshal(confirmedTurn.Intent)
		planJSON, _ := json.Marshal(confirmedTurn.Plan)
		dryRunJSON, _ := json.Marshal(confirmedTurn.DryRun)

		loadConversationErrTx := newAssistantCommitTx(now, "actor_1", "tenant-admin", assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(confirmedTurn)}})
		loadConversationErrTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "SELECT intent_json"):
				return &assistFakeRow{vals: []any{intentJSON, planJSON, dryRunJSON}}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{err: errors.New("load conversation failed")}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		if err := svc.executeAssistantTaskWorkflowTx(context.Background(), loadConversationErrTx, "tenant_1", &task, now); err == nil || !strings.Contains(err.Error(), "load conversation failed") {
			t.Fatalf("unexpected load conversation err=%v", err)
		}

		missingTurnTask := task
		missingTurnTx := newAssistantCommitTx(now, "actor_1", "tenant-admin", assistantStateConfirmed, &assistFakeRows{})
		missingTurnTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "SELECT intent_json"):
				return &assistFakeRow{vals: []any{intentJSON, planJSON, dryRunJSON}}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", "actor_1", "tenant-admin", assistantStateConfirmed, now)}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		if err := svc.executeAssistantTaskWorkflowTx(context.Background(), missingTurnTx, "tenant_1", &missingTurnTask, now); err != nil || missingTurnTask.Status != assistantTaskStatusManualTakeoverNeeded {
			t.Fatalf("unexpected missing turn err=%v task=%+v", err, missingTurnTask)
		}

		execErrTask := task
		execErrTx := newAssistantCommitTx(now, "actor_1", "tenant-admin", assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(confirmedTurn)}})
		execErrTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "SELECT intent_json"):
				return &assistFakeRow{vals: []any{intentJSON, planJSON, dryRunJSON}}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", "actor_1", "tenant-admin", assistantStateConfirmed, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		execErrTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "UPDATE iam.assistant_conversations") {
				return pgconn.NewCommandTag(""), errors.New("update execute failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		if err := svc.executeAssistantTaskWorkflowTx(context.Background(), execErrTx, "tenant_1", &execErrTask, now); err == nil || !strings.Contains(err.Error(), "update execute failed") {
			t.Fatalf("unexpected exec err=%v", err)
		}

		validatedSvc, _, validatedTurn, validatedNow := newAssistantCommitCoverageEnv(t, assistantStateValidated)
		validatedTask := task
		validatedTask.ContractSnapshot = assistantTaskSnapshotFromTurn(validatedTurn)
		validatedTx := newAssistantCommitTx(validatedNow, "actor_1", "tenant-admin", assistantStateValidated, &assistFakeRows{rows: [][]any{assistantTurnRowValues(validatedTurn)}})
		validatedIntentJSON, _ := json.Marshal(validatedTurn.Intent)
		validatedPlanJSON, _ := json.Marshal(validatedTurn.Plan)
		validatedDryRunJSON, _ := json.Marshal(validatedTurn.DryRun)
		validatedTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "SELECT intent_json"):
				return &assistFakeRow{vals: []any{validatedIntentJSON, validatedPlanJSON, validatedDryRunJSON}}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", "actor_1", "tenant-admin", assistantStateValidated, validatedNow)}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		if err := validatedSvc.executeAssistantTaskWorkflowTx(context.Background(), validatedTx, "tenant_1", &validatedTask, validatedNow); err != nil || validatedTask.Status != assistantTaskStatusManualTakeoverNeeded {
			t.Fatalf("unexpected apply err path=%v task=%+v", err, validatedTask)
		}

		successTask := task
		successTx := newAssistantCommitTx(now, "actor_1", "tenant-admin", assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(confirmedTurn)}})
		successTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "SELECT intent_json"):
				return &assistFakeRow{vals: []any{intentJSON, planJSON, dryRunJSON}}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", "actor_1", "tenant-admin", assistantStateConfirmed, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		if err := svc.executeAssistantTaskWorkflowTx(context.Background(), successTx, "tenant_1", &successTask, now); err != nil || successTask.Status != assistantTaskStatusSucceeded {
			t.Fatalf("unexpected success err=%v task=%+v", err, successTask)
		}

		updateErrTask := task
		updateErrTx := newAssistantCommitTx(now, "actor_1", "tenant-admin", assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(confirmedTurn)}})
		updateErrTx.queryRowFn = successTx.queryRowFn
		updateErrTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "UPDATE iam.assistant_tasks") {
				return pgconn.NewCommandTag(""), errors.New("workflow update failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		if err := svc.executeAssistantTaskWorkflowTx(context.Background(), updateErrTx, "tenant_1", &updateErrTask, now); err == nil || !strings.Contains(err.Error(), "workflow update failed") {
			t.Fatalf("unexpected workflow update err=%v", err)
		}

		eventErrTask := task
		eventErrTx := newAssistantCommitTx(now, "actor_1", "tenant-admin", assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(confirmedTurn)}})
		eventErrTx.queryRowFn = successTx.queryRowFn
		eventErrTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO iam.assistant_task_events") {
				return pgconn.NewCommandTag(""), errors.New("workflow event failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		if err := svc.executeAssistantTaskWorkflowTx(context.Background(), eventErrTx, "tenant_1", &eventErrTask, now); err == nil || !strings.Contains(err.Error(), "workflow event failed") {
			t.Fatalf("unexpected workflow event err=%v", err)
		}

		if err := svc.markAssistantTaskManualTakeoverTx(context.Background(), successTx, "tenant_1", nil, assistantTaskStatusRunning, "manual", now); !errors.Is(err, errAssistantTaskStateInvalid) {
			t.Fatalf("unexpected nil task err=%v", err)
		}
	})

	t.Run("api mappings", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		origLoadAuthorizerFn := assistantLoadAuthorizerFn
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil }
		defer func() { assistantLoadAuthorizerFn = origLoadAuthorizerFn }()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatal(err)
		}
		principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
		now := time.Now().UTC()
		baseReq := func(path string, body string) *http.Request {
			req := httptest.NewRequest(http.MethodPost, "http://localhost"+path, strings.NewReader(body))
			ctx := withTenant(req.Context(), Tenant{ID: "tenant_1"})
			ctx = withPrincipal(ctx, Principal{ID: "actor_1", RoleSlug: "tenant-admin"})
			return req.WithContext(ctx)
		}

		confirmRiskSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		confirmRiskSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{assistantIntentCreateOrgUnit: {ID: assistantIntentCreateOrgUnit, Version: "v1", CapabilityKey: "org.orgunit_create.field_policy", Security: assistantActionSecuritySpec{AuthObject: "org.setid_capability_config", AuthAction: "admin", RiskTier: "extreme"}, Handler: assistantActionHandlerSpec{CommitAdapterKey: "orgunit_create_v1"}}}}
		confirmConv := confirmRiskSvc.createConversation("tenant_1", principal)
		confirmTurn := assistantTaskSampleTurn(now)
		confirmTurn.State = assistantStateValidated
		confirmTurn.Intent = assistantIntentSpec{Action: assistantIntentCreateOrgUnit, IntentSchemaVersion: assistantIntentSchemaVersionV1, EffectiveDate: "2026-01-01", ParentRefText: "鲜花组织", EntityName: "运营部"}
		confirmTurn.Plan = assistantBuildPlan(confirmTurn.Intent)
		confirmTurn.Candidates = []assistantCandidate{{CandidateID: "FLOWER-A", CandidateCode: "FLOWER-A", Name: "鲜花组织"}}
		confirmTurn.ResolvedCandidateID = "FLOWER-A"
		confirmRiskSvc.mu.Lock()
		confirmRiskSvc.byID[confirmConv.ConversationID].Turns = append(confirmRiskSvc.byID[confirmConv.ConversationID].Turns, confirmTurn)
		confirmRiskSvc.mu.Unlock()
		rec := httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/"+confirmConv.ConversationID+"/turns/"+confirmTurn.TurnID+":confirm", `{}`), confirmRiskSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != errAssistantActionRiskGateDenied.Error() {
			t.Fatalf("confirm risk status=%d body=%s", rec.Code, rec.Body.String())
		}

		beginStableSvc, _, confirmedTurn, now := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		beginStableSvc.pool = assistFakeTxBeginner{err: errors.New(orgUnitErrFieldPolicyMissing)}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+confirmedTurn.TurnID+":commit", `{}`), beginStableSvc)
		if rec.Code != http.StatusUnprocessableEntity || assistantDecodeErrCode(t, rec) != orgUnitErrFieldPolicyMissing {
			t.Fatalf("stable commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		beginUnknownSvc, _, confirmedTurn2, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		beginUnknownSvc.pool = assistFakeTxBeginner{err: errors.New("begin failed")}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+confirmedTurn2.TurnID+":commit", `{}`), beginUnknownSvc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_commit_failed" {
			t.Fatalf("unknown commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		notFoundSvc, _, confirmedTurn3, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		notFoundTx := newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(confirmedTurn3)}})
		notFoundTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "FROM iam.assistant_conversations") {
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		notFoundSvc.pool = assistFakeTxBeginner{tx: notFoundTx}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+confirmedTurn3.TurnID+":commit", `{}`), notFoundSvc)
		if rec.Code != http.StatusNotFound || assistantDecodeErrCode(t, rec) != "conversation_not_found" {
			t.Fatalf("not found commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		turnMissingSvc, _, confirmedTurn4, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		turnMissingSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+confirmedTurn4.TurnID+":commit", `{}`), turnMissingSvc)
		if rec.Code != http.StatusNotFound || assistantDecodeErrCode(t, rec) != "conversation_turn_not_found" {
			t.Fatalf("turn missing commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		authSnapshotSvc, _, confirmedTurn5, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		authSnapshotSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, "actor_2", principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(confirmedTurn5)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+confirmedTurn5.TurnID+":commit", `{}`), authSnapshotSvc)
		if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != "ai_actor_auth_snapshot_expired" {
			t.Fatalf("auth snapshot commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		roleDriftSvc, _, confirmedTurn6, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		roleDriftSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, "viewer", assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(confirmedTurn6)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+confirmedTurn6.TurnID+":commit", `{}`), roleDriftSvc)
		if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != "ai_actor_role_drift_detected" {
			t.Fatalf("role drift commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		stateInvalidSvc, _, committedTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateCommitted)
		stateInvalidSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateCommitted, &assistFakeRows{rows: [][]any{assistantTurnRowValues(committedTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+committedTurn.TurnID+":commit", `{}`), stateInvalidSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "assistant_task_state_invalid" {
			t.Fatalf("state invalid commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		confirmRequiredSvc, _, validatedTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateValidated)
		confirmRequiredSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateValidated, &assistFakeRows{rows: [][]any{assistantTurnRowValues(validatedTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+validatedTurn.TurnID+":commit", `{}`), confirmRequiredSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "conversation_confirmation_required" {
			t.Fatalf("confirm required commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		requiredCheckCommitSvc, _, requiredCheckTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		requiredCheckCommitSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{
			assistantIntentCreateOrgUnit: {
				ID:            assistantIntentCreateOrgUnit,
				Version:       "v1",
				CapabilityKey: "org.orgunit_create.field_policy",
				Security: assistantActionSecuritySpec{
					AuthObject:     "org.setid_capability_config",
					AuthAction:     "admin",
					RiskTier:       "high",
					RequiredChecks: []string{"unknown_check"},
				},
				Handler: assistantActionHandlerSpec{CommitAdapterKey: "orgunit_create_v1"},
			},
		}}
		requiredCheckCommitSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(requiredCheckTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+requiredCheckTurn.TurnID+":commit", `{}`), requiredCheckCommitSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != errAssistantActionRequiredCheckFailed.Error() {
			t.Fatalf("required check commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		expiredSvc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		expiredTurnCreatedAt := time.Now().UTC().Add(-time.Hour)
		expiredTurn := &assistantTurn{
			TurnID:    "turn_commit_expired",
			State:     assistantStateValidated,
			RequestID: "req_commit_expired",
			TraceID:   "trace_commit_expired",
			Intent:    assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			Plan:      assistantFreezeConfirmWindow(assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), expiredTurnCreatedAt),
			CreatedAt: expiredTurnCreatedAt,
			UpdatedAt: expiredTurnCreatedAt,
			RiskTier:  "low",
		}
		expiredSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(expiredTurnCreatedAt, principal.ID, principal.RoleSlug, assistantStateValidated, &assistFakeRows{rows: [][]any{assistantTurnRowValues(expiredTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+expiredTurn.TurnID+":commit", `{}`), expiredSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "conversation_confirmation_expired" {
			t.Fatalf("expired commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		stateSvc, _, canceledTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateCanceled)
		stateSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateCanceled, &assistFakeRows{rows: [][]any{assistantTurnRowValues(canceledTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+canceledTurn.TurnID+":commit", `{}`), stateSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "conversation_state_invalid" {
			t.Fatalf("state invalid commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		contractSvc, _, contractTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		contractTurn.Plan.SkillManifestDigest = ""
		contractSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(contractTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+contractTurn.TurnID+":commit", `{}`), contractSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "ai_plan_contract_version_mismatch" {
			t.Fatalf("contract commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		staleSvc, _, staleTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		staleTurn.Plan.VersionTuple = nil
		staleSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(staleTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+staleTurn.TurnID+":commit", `{}`), staleSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "ai_version_tuple_stale" {
			t.Fatalf("stale commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		unsupportedSvc, _, unsupportedTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		unsupportedSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{}}
		unsupportedSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(unsupportedTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+unsupportedTurn.TurnID+":commit", `{}`), unsupportedSvc)
		if rec.Code != http.StatusUnprocessableEntity || assistantDecodeErrCode(t, rec) != "assistant_intent_unsupported" {
			t.Fatalf("unsupported commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		serviceMissingSvc, _, serviceTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		serviceMissingSvc.commitAdapterRegistry = assistantCommitAdapterRegistryMap{adapters: map[string]assistantCommitAdapter{}}
		serviceMissingSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(serviceTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+serviceTurn.TurnID+":commit", `{}`), serviceMissingSvc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "orgunit_service_missing" {
			t.Fatalf("service missing commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		candidateSvc, _, candidateTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		candidateTurn.ResolvedCandidateID = ""
		candidateTurn.Candidates = nil
		candidateSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(candidateTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+candidateTurn.TurnID+":commit", `{}`), candidateSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "conversation_confirmation_required" {
			t.Fatalf("candidate commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		authzSvc, _, authzTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: false, enforced: true}, nil }
		authzSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(authzTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+authzTurn.TurnID+":commit", `{}`), authzSvc)
		if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != errAssistantActionAuthzDenied.Error() {
			t.Fatalf("authz commit status=%d body=%s", rec.Code, rec.Body.String())
		}
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil }

		riskSvc, _, riskTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		riskSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{assistantIntentCreateOrgUnit: {ID: assistantIntentCreateOrgUnit, Version: "v1", CapabilityKey: "org.orgunit_create.field_policy", Security: assistantActionSecuritySpec{AuthObject: "org.setid_capability_config", AuthAction: "admin", RiskTier: "extreme"}, Handler: assistantActionHandlerSpec{CommitAdapterKey: "orgunit_create_v1"}}}}
		riskSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(riskTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+riskTurn.TurnID+":commit", `{}`), riskSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != errAssistantActionRiskGateDenied.Error() {
			t.Fatalf("risk commit status=%d body=%s", rec.Code, rec.Body.String())
		}

		successSvc, _, successTurn, _ := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		successSvc.pool = assistFakeTxBeginner{tx: newAssistantCommitTx(now, principal.ID, principal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(successTurn)}})}
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, baseReq("/internal/assistant/conversations/conv_1/turns/"+successTurn.TurnID+":commit", `{}`), successSvc)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("success commit status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}
