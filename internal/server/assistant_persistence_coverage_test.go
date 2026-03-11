package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type assistFakeTxBeginner struct {
	tx  pgx.Tx
	err error
}

func (f assistFakeTxBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tx, nil
}

type assistFakeBatchResults struct{}

func (assistFakeBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
func (assistFakeBatchResults) Query() (pgx.Rows, error)         { return nil, nil }
func (assistFakeBatchResults) QueryRow() pgx.Row                { return &assistFakeRow{err: pgx.ErrNoRows} }
func (assistFakeBatchResults) Close() error                     { return nil }

type assistFakeTx struct {
	execFn     func(sql string, args ...any) (pgconn.CommandTag, error)
	queryFn    func(sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(sql string, args ...any) pgx.Row
	commitErr  error
	rolledBack bool
	committed  bool
}

func (f *assistFakeTx) Begin(context.Context) (pgx.Tx, error) { return f, nil }
func (f *assistFakeTx) Commit(context.Context) error {
	f.committed = true
	return f.commitErr
}
func (f *assistFakeTx) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}
func (f *assistFakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (f *assistFakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return assistFakeBatchResults{}
}
func (f *assistFakeTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (f *assistFakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (f *assistFakeTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFn == nil {
		return pgconn.NewCommandTag(""), nil
	}
	return f.execFn(sql, args...)
}
func (f *assistFakeTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.queryFn == nil {
		return &assistFakeRows{}, nil
	}
	return f.queryFn(sql, args...)
}
func (f *assistFakeTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if f.queryRowFn == nil {
		return &assistFakeRow{err: pgx.ErrNoRows}
	}
	return f.queryRowFn(sql, args...)
}
func (f *assistFakeTx) Conn() *pgx.Conn { return nil }

type assistFakeRow struct {
	vals []any
	err  error
}

func (r *assistFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignScan(dest, r.vals)
}

type assistFakeRows struct {
	rows [][]any
	err  error
	idx  int
}

func (r *assistFakeRows) Close() {}
func (r *assistFakeRows) Err() error {
	return r.err
}
func (r *assistFakeRows) CommandTag() pgconn.CommandTag { return pgconn.NewCommandTag("") }
func (r *assistFakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *assistFakeRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *assistFakeRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("scan called without next")
	}
	return assignScan(dest, r.rows[r.idx-1])
}
func (r *assistFakeRows) Values() ([]any, error) { return nil, nil }
func (r *assistFakeRows) RawValues() [][]byte    { return nil }
func (r *assistFakeRows) Conn() *pgx.Conn        { return nil }

func assignScan(dest []any, vals []any) error {
	if len(dest) != len(vals) {
		return fmt.Errorf("dest len %d != vals len %d", len(dest), len(vals))
	}
	for i := range dest {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr || dv.IsNil() {
			return fmt.Errorf("dest[%d] not pointer", i)
		}
		t := dv.Elem().Type()
		if vals[i] == nil {
			dv.Elem().Set(reflect.Zero(t))
			continue
		}
		sv := reflect.ValueOf(vals[i])
		if t.Kind() == reflect.Ptr {
			pv := reflect.New(t.Elem())
			if sv.Type().AssignableTo(t.Elem()) {
				pv.Elem().Set(sv)
			} else if sv.Type().ConvertibleTo(t.Elem()) {
				pv.Elem().Set(sv.Convert(t.Elem()))
			} else {
				return fmt.Errorf("value %d type mismatch", i)
			}
			dv.Elem().Set(pv)
			continue
		}
		if sv.Type().AssignableTo(t) {
			dv.Elem().Set(sv)
		} else if sv.Type().ConvertibleTo(t) {
			dv.Elem().Set(sv.Convert(t))
		} else {
			return fmt.Errorf("value %d type mismatch", i)
		}
	}
	return nil
}

type assistWriteErrStub struct{}

func (assistWriteErrStub) Write(context.Context, string, orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
	return orgunitservices.OrgUnitWriteResult{}, errors.New("write failed")
}
func (assistWriteErrStub) Create(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}
func (assistWriteErrStub) Rename(context.Context, string, orgunitservices.RenameOrgUnitRequest) error {
	return errors.New("not implemented")
}
func (assistWriteErrStub) Move(context.Context, string, orgunitservices.MoveOrgUnitRequest) error {
	return errors.New("not implemented")
}
func (assistWriteErrStub) Disable(context.Context, string, orgunitservices.DisableOrgUnitRequest) error {
	return errors.New("not implemented")
}
func (assistWriteErrStub) Enable(context.Context, string, orgunitservices.EnableOrgUnitRequest) error {
	return errors.New("not implemented")
}
func (assistWriteErrStub) SetBusinessUnit(context.Context, string, orgunitservices.SetBusinessUnitRequest) error {
	return errors.New("not implemented")
}
func (assistWriteErrStub) Correct(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}
func (assistWriteErrStub) CorrectStatus(context.Context, string, orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}
func (assistWriteErrStub) RescindRecord(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}
func (assistWriteErrStub) RescindOrg(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func TestAssistantPersistence_UtilityFunctions(t *testing.T) {
	if got := nilIfEmptyJSON(nil); got != nil {
		t.Fatalf("want nil, got=%v", got)
	}
	if got := nilIfEmptyJSON([]byte(`{"k":1}`)); got != `{"k":1}` {
		t.Fatalf("unexpected json: %v", got)
	}

	if assistantHashText("a") == assistantHashText("b") {
		t.Fatal("hash text should differ")
	}
	if assistantHashBytes([]byte("a")) == assistantHashBytes([]byte("b")) {
		t.Fatal("hash bytes should differ")
	}
	if strings.TrimSpace(newUUIDString()) == "" {
		t.Fatal("uuid should not be empty")
	}

	if err := assistantErrorFromIdempotencyCode("assistant_idempotency_key_conflict"); !errors.Is(err, errAssistantIdempotencyKeyConflict) {
		t.Fatalf("unexpected err=%v", err)
	}
	if err := assistantErrorFromIdempotencyCode("assistant_request_in_progress"); !errors.Is(err, errAssistantRequestInProgress) {
		t.Fatalf("unexpected err=%v", err)
	}
	if err := assistantErrorFromIdempotencyCode("something_else"); err == nil || err.Error() != "something_else" {
		t.Fatalf("unexpected err=%v", err)
	}
	idemCodes := []struct {
		code string
		err  error
	}{
		{errAssistantConfirmationRequired.Error(), errAssistantConfirmationRequired},
		{errAssistantConfirmationExpired.Error(), errAssistantConfirmationExpired},
		{errAssistantConversationStateInvalid.Error(), errAssistantConversationStateInvalid},
		{errAssistantPlanContractVersionMismatch.Error(), errAssistantPlanContractVersionMismatch},
		{errAssistantCandidateNotFound.Error(), errAssistantCandidateNotFound},
		{errAssistantAuthSnapshotExpired.Error(), errAssistantAuthSnapshotExpired},
		{errAssistantRoleDriftDetected.Error(), errAssistantRoleDriftDetected},
		{errAssistantRouteRuntimeInvalid.Error(), errAssistantRouteRuntimeInvalid},
		{errAssistantRouteCatalogMissing.Error(), errAssistantRouteCatalogMissing},
		{errAssistantRouteActionConflict.Error(), errAssistantRouteActionConflict},
		{errAssistantRouteDecisionMissing.Error(), errAssistantRouteDecisionMissing},
		{errAssistantRouteNonBusinessBlocked.Error(), errAssistantRouteNonBusinessBlocked},
		{errAssistantRouteClarificationRequired.Error(), errAssistantRouteClarificationRequired},
		{errAssistantUnsupportedIntent.Error(), errAssistantUnsupportedIntent},
		{errAssistantServiceMissing.Error(), errAssistantServiceMissing},
	}
	for _, item := range idemCodes {
		if got := assistantErrorFromIdempotencyCode(item.code); !errors.Is(got, item.err) {
			t.Fatalf("code %s mapped err=%v", item.code, got)
		}
	}

	if status, code, ok := assistantIdempotencyErrorPayload(errAssistantPlanContractVersionMismatch); !ok || status != 409 || code == "" {
		t.Fatalf("unexpected payload status=%d code=%s ok=%v", status, code, ok)
	}
	idemCases := []error{
		errAssistantConfirmationRequired,
		errAssistantConfirmationExpired,
		errAssistantConversationStateInvalid,
		errAssistantPlanContractVersionMismatch,
		errAssistantCandidateNotFound,
		errAssistantAuthSnapshotExpired,
		errAssistantRoleDriftDetected,
		errAssistantRouteRuntimeInvalid,
		errAssistantRouteCatalogMissing,
		errAssistantRouteActionConflict,
		errAssistantRouteDecisionMissing,
		errAssistantRouteNonBusinessBlocked,
		errAssistantRouteClarificationRequired,
		errAssistantUnsupportedIntent,
		errAssistantServiceMissing,
	}
	for _, e := range idemCases {
		if _, _, ok := assistantIdempotencyErrorPayload(e); !ok {
			t.Fatalf("expected mapped idempotency error for %v", e)
		}
	}
	if status, code, ok := assistantIdempotencyErrorPayload(errors.New(orgUnitErrFieldPolicyMissing)); !ok || status != http.StatusUnprocessableEntity || code != orgUnitErrFieldPolicyMissing {
		t.Fatalf("unexpected mapped payload status=%d code=%s ok=%v", status, code, ok)
	}
	if _, _, ok := assistantIdempotencyErrorPayload(errors.New("unknown")); ok {
		t.Fatal("unknown error should not be persisted as idempotency payload")
	}

	if _, err := (&assistantConversationService{}).restoreIdempotentResult(assistantIdempotencyClaim{ErrorCode: errAssistantCandidateNotFound.Error()}); !errors.Is(err, errAssistantCandidateNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}
	if _, err := (&assistantConversationService{}).restoreIdempotentResult(assistantIdempotencyClaim{ErrorCode: errAssistantConfirmationExpired.Error()}); !errors.Is(err, errAssistantConfirmationExpired) {
		t.Fatalf("unexpected err=%v", err)
	}
	if _, err := (&assistantConversationService{}).restoreIdempotentResult(assistantIdempotencyClaim{Body: nil}); !errors.Is(err, errAssistantRequestInProgress) {
		t.Fatalf("unexpected err=%v", err)
	}
	if _, err := (&assistantConversationService{}).restoreIdempotentResult(assistantIdempotencyClaim{Body: []byte("{")}); err == nil {
		t.Fatal("invalid json should fail")
	}
	conv := assistantConversation{ConversationID: "conv_a", TenantID: "tenant_a"}
	body, _ := json.Marshal(conv)
	restored, err := (&assistantConversationService{}).restoreIdempotentResult(assistantIdempotencyClaim{Body: body})
	if err != nil || restored.ConversationID != "conv_a" {
		t.Fatalf("restore err=%v restored=%+v", err, restored)
	}
	if restored.CurrentPhase != assistantPhaseIdle {
		t.Fatalf("expected derived current phase idle, got=%q", restored.CurrentPhase)
	}

	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	svc.cacheConversation(nil)
	if _, ok := svc.getCachedConversation("missing"); ok {
		t.Fatal("cache should miss")
	}
	cached := &assistantConversation{ConversationID: "conv_cache", ActorID: "actor_1", TenantID: "tenant_1"}
	svc.cacheConversation(cached)
	svc.cacheConversation(cached)
	got, ok := svc.getCachedConversation("conv_cache")
	if !ok || got.ConversationID != "conv_cache" {
		t.Fatalf("unexpected cache lookup ok=%v got=%+v", ok, got)
	}
	if turns := assistantLookupTurn(nil, "t"); turns != nil {
		t.Fatal("lookup on nil conversation should return nil")
	}
	if turn := assistantLookupTurn(&assistantConversation{Turns: []*assistantTurn{{TurnID: "t1"}}}, "missing"); turn != nil {
		t.Fatal("missing turn should be nil")
	}
	if turn := assistantLookupTurn(&assistantConversation{Turns: []*assistantTurn{{TurnID: "t1"}}}, "t1"); turn == nil {
		t.Fatal("expected lookup hit")
	}
	if got := assistantExpireTurn(nil, nil, Principal{ID: "actor_1"}, "confirm"); got.PersistTurn || got.Transition != nil {
		t.Fatalf("nil expire turn result=%+v", got)
	}
}

func TestAssistantPersistence_ApplyConfirmTurnBranches(t *testing.T) {
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	mkTurn := func() *assistantTurn {
		return &assistantTurn{
			TurnID:         "turn_1",
			State:          assistantStateValidated,
			TraceID:        "trace_1",
			RequestID:      "req_1",
			AmbiguityCount: 2,
			Intent: assistantIntentSpec{
				Action: assistantIntentCreateOrgUnit,
			},
			Plan:       assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}, {CandidateID: "c2", CandidateCode: "FLOWER-B"}},
		}
	}
	conversation := &assistantConversation{ConversationID: "conv_1"}

	turn := mkTurn()
	turn.State = assistantStateCommitted
	if _, err := svc.applyConfirmTurn(conversation, turn, principal, ""); err != nil {
		t.Fatalf("committed should noop err=%v", err)
	}

	turn = mkTurn()
	turn.State = assistantStateCanceled
	if _, err := svc.applyConfirmTurn(conversation, turn, principal, ""); !errors.Is(err, errAssistantConversationStateInvalid) {
		t.Fatalf("unexpected err=%v", err)
	}

	turn = mkTurn()
	turn.State = assistantStateConfirmed
	turn.ResolvedCandidateID = "c1"
	if _, err := svc.applyConfirmTurn(conversation, turn, principal, "c1"); err != nil {
		t.Fatalf("same candidate should noop err=%v", err)
	}
	if _, err := svc.applyConfirmTurn(conversation, turn, principal, "cx"); !errors.Is(err, errAssistantCandidateNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}
	if _, err := svc.applyConfirmTurn(conversation, turn, principal, "c2"); !errors.Is(err, errAssistantConversationStateInvalid) {
		t.Fatalf("unexpected err=%v", err)
	}

	turn = mkTurn()
	turn.State = assistantStateDraft
	if _, err := svc.applyConfirmTurn(conversation, turn, principal, ""); !errors.Is(err, errAssistantConfirmationRequired) {
		t.Fatalf("unexpected err=%v", err)
	}

	turn = mkTurn()
	if _, err := svc.applyConfirmTurn(conversation, turn, principal, ""); !errors.Is(err, errAssistantConfirmationRequired) {
		t.Fatalf("unexpected err=%v", err)
	}
	if _, err := svc.applyConfirmTurn(conversation, turn, principal, "cx"); !errors.Is(err, errAssistantCandidateNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}

	turn = mkTurn()
	if _, err := svc.applyConfirmTurn(conversation, turn, principal, "c2"); err != nil {
		t.Fatalf("confirm should succeed err=%v", err)
	}
	if turn.State != assistantStateConfirmed || turn.ResolutionSource != assistantResolutionUserConfirmed {
		t.Fatalf("unexpected turn=%+v", turn)
	}
}

func TestAssistantPersistence_ApplyCommitTurnBranches(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true)
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	basePlan := assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit})
	basePlan.SkillManifestDigest = "digest"
	baseIntent := assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EntityName: "运营部", EffectiveDate: "2026-01-01", IntentSchemaVersion: assistantIntentSchemaVersionV1}

	mkTurn := func() *assistantTurn {
		turn := &assistantTurn{
			TurnID:              "turn_1",
			State:               assistantStateConfirmed,
			TraceID:             "trace_1",
			RequestID:           "req_1",
			PolicyVersion:       "2026-02-23",
			CompositionVersion:  "2026-02-23",
			MappingVersion:      "2026-02-23",
			Intent:              baseIntent,
			Plan:                basePlan,
			ResolvedCandidateID: "c1",
			Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}},
		}
		if err := svc.refreshTurnVersionTuple(context.Background(), "tenant_1", turn); err != nil {
			t.Fatalf("refresh turn version tuple err=%v", err)
		}
		return turn
	}
	conversation := &assistantConversation{ConversationID: "conv_1"}

	turn := mkTurn()
	turn.State = assistantStateCommitted
	if _, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); err != nil {
		t.Fatalf("committed should noop err=%v", err)
	}

	turn = mkTurn()
	turn.State = assistantStateExpired
	if _, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); !errors.Is(err, errAssistantConversationStateInvalid) {
		t.Fatalf("unexpected err=%v", err)
	}

	turn = mkTurn()
	turn.State = assistantStateValidated
	if _, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); !errors.Is(err, errAssistantConfirmationRequired) {
		t.Fatalf("unexpected err=%v", err)
	}

	turn = mkTurn()
	turn.Plan.CompilerContractVersion = "broken"
	if result, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); !errors.Is(err, errAssistantPlanContractVersionMismatch) || !result.PersistTurn {
		t.Fatalf("unexpected result=%+v err=%v", result, err)
	}

	turn = mkTurn()
	turn.PolicyVersion = "stale"
	if result, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); !errors.Is(err, errAssistantConfirmationRequired) || !result.PersistTurn {
		t.Fatalf("unexpected result=%+v err=%v", result, err)
	}

	turn = mkTurn()
	turn.Intent.Action = "plan_only"
	if _, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); !errors.Is(err, errAssistantUnsupportedIntent) {
		t.Fatalf("unexpected err=%v", err)
	}

	turn = mkTurn()
	turn.ResolvedCandidateID = ""
	if _, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); !errors.Is(err, errAssistantCandidateNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}

	turn = mkTurn()
	svc.writeSvc = nil
	if _, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); !errors.Is(err, errAssistantServiceMissing) {
		t.Fatalf("unexpected err=%v", err)
	}
	svc.writeSvc = assistantWriteServiceStub{store: store}

	turn = mkTurn()
	svc.commitAdapterRegistry = assistantCommitAdapterRegistryMap{adapters: map[string]assistantCommitAdapter{}}
	if _, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); !errors.Is(err, errAssistantServiceMissing) {
		t.Fatalf("want missing adapter, got err=%v", err)
	}
	svc.commitAdapterRegistry = nil

	turn = mkTurn()
	turn.Plan.VersionTuple = []byte(`{"parent_candidate_id":"c1","parent_org_code":"FLOWER-A","parent_updated_at":"2000-01-01T00:00:00Z","effective_date":"2026-01-01"}`)
	if result, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); !errors.Is(err, errAssistantVersionTupleStale) || !result.PersistTurn {
		t.Fatalf("want version tuple stale, got result=%+v err=%v", result, err)
	}

	turn = mkTurn()
	turn.ResolvedCandidateID = "missing"
	if _, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); !errors.Is(err, errAssistantCandidateNotFound) {
		t.Fatalf("unexpected err=%v", err)
	}

	turn = mkTurn()
	svc.writeSvc = assistWriteErrStub{}
	if _, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); err == nil {
		t.Fatal("expected write error")
	}

	turn = mkTurn()
	svc.writeSvc = assistantWriteServiceStub{store: store}
	if result, err := svc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant_1"); err != nil || result.Transition == nil {
		t.Fatalf("unexpected result=%+v err=%v", result, err)
	}
	if turn.State != assistantStateCommitted || turn.CommitResult == nil || turn.CommitResult.ParentOrgCode != "FLOWER-A" {
		t.Fatalf("unexpected turn=%+v", turn)
	}
}

func TestAssistantPersistence_DBHelpersAndTxPaths(t *testing.T) {
	ctx := context.Background()
	tx := &assistFakeTx{}
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})

	if _, err := (*assistantConversationService)(nil).beginAssistantTx(ctx, "tenant_1"); !errors.Is(err, errAssistantServiceMissing) {
		t.Fatalf("unexpected err=%v", err)
	}
	if _, err := svc.beginAssistantTx(ctx, "tenant_1"); !errors.Is(err, errAssistantServiceMissing) {
		t.Fatalf("unexpected err=%v", err)
	}

	svc.pool = assistFakeTxBeginner{err: errors.New("begin failed")}
	if _, err := svc.beginAssistantTx(ctx, "tenant_1"); err == nil {
		t.Fatal("expected begin error")
	}

	tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
		if strings.Contains(sql, "set_config") {
			return pgconn.NewCommandTag(""), errors.New("set_config failed")
		}
		return pgconn.NewCommandTag(""), nil
	}
	svc.pool = assistFakeTxBeginner{tx: tx}
	if _, err := svc.beginAssistantTx(ctx, "tenant_1"); err == nil {
		t.Fatal("expected set_config error")
	}

	tx.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
	begun, err := svc.beginAssistantTx(ctx, "tenant_1")
	if err != nil || begun == nil {
		t.Fatalf("unexpected begin err=%v tx=%v", err, begun)
	}

	conv := &assistantConversation{ConversationID: "conv_1", TenantID: "tenant_1", ActorID: "actor_1", ActorRole: "tenant-admin", State: assistantStateValidated, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Turns: []*assistantTurn{}, Transitions: []assistantStateTransition{}}
	badConv := &assistantConversation{ConversationID: "conv_bad", TenantID: "tenant_1", ActorID: "actor_1", ActorRole: "tenant-admin", State: assistantStateValidated, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Turns: []*assistantTurn{{Plan: assistantPlanSummary{ConfigDeltaPlan: assistantConfigDeltaPlan{Changes: []assistantConfigChange{{After: func() {}}}}}}}}

	tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		switch {
		case strings.Contains(sql, "FROM iam.assistant_conversations"):
			return &assistFakeRow{vals: []any{"conv_1", "tenant_1", "actor_1", "tenant-admin", assistantStateValidated, assistantConversationPhaseFromLegacyState(assistantStateValidated), conv.CreatedAt, conv.UpdatedAt}}
		case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
			return &assistFakeRow{vals: []any{int64(9)}}
		case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
			return &assistFakeRow{vals: []any{1}}
		case strings.Contains(sql, "SELECT request_hash"):
			return &assistFakeRow{vals: []any{"h1", "done", 200, "", bodyOrNull(conv)}}
		default:
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
	}
	tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
		switch {
		case strings.Contains(sql, "FROM iam.assistant_turns"):
			loadedTurn := &assistantTurn{
				TurnID:              "turn_1",
				UserInput:           "输入",
				State:               assistantStateConfirmed,
				RiskTier:            "high",
				RequestID:           "req_1",
				TraceID:             "trace_1",
				PolicyVersion:       "2026-02-23",
				CompositionVersion:  "2026-02-23",
				MappingVersion:      "2026-02-23",
				Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"},
				Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
				Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}},
				ResolvedCandidateID: "c1",
				AmbiguityCount:      1,
				Confidence:          0.9,
				ResolutionSource:    "auto",
				DryRun:              assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, nil, ""),
				CommitResult:        &assistantCommitResult{OrgCode: "ORG-1", ParentOrgCode: "FLOWER-A", EffectiveDate: "2026-01-01", EventType: "CREATE", EventUUID: "evt-1"},
				CreatedAt:           time.Now().UTC(),
				UpdatedAt:           time.Now().UTC(),
			}
			return &assistFakeRows{rows: [][]any{assistantTurnRowValues(loadedTurn)}}, nil
		case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
			return &assistFakeRows{rows: [][]any{{int64(1), "turn_1", "confirm", "req_1", "trace_1", assistantStateValidated, assistantStateConfirmed, assistantPhaseAwaitCommitConfirm, assistantPhaseAwaitCommitConfirm, "confirmed", "actor_1", time.Now().UTC()}}}, nil
		default:
			return &assistFakeRows{}, nil
		}
	}

	loaded, err := svc.loadConversationTx(ctx, tx, "tenant_1", "conv_1", true)
	if err != nil || loaded.ConversationID != "conv_1" || len(loaded.Turns) != 1 || len(loaded.Transitions) != 1 {
		t.Fatalf("unexpected load err=%v loaded=%+v", err, loaded)
	}
	if loaded.CurrentPhase != assistantPhaseAwaitCommitConfirm {
		t.Fatalf("expected current phase await_commit_confirm, got=%q", loaded.CurrentPhase)
	}
	if loaded.Turns[0].CommitReply == nil || loaded.Turns[0].CommitReply.Outcome != "success" {
		t.Fatalf("expected commit reply restored, got=%+v", loaded.Turns[0].CommitReply)
	}
	if loaded.Transitions[0].ToPhase != assistantPhaseAwaitCommitConfirm {
		t.Fatalf("expected transition to_phase await_commit_confirm, got=%q", loaded.Transitions[0].ToPhase)
	}

	if _, err := svc.loadConversationByTenant(ctx, "tenant_1", "conv_1", false); err != nil {
		t.Fatalf("load by tenant err=%v", err)
	}

	if err := svc.finalizeIdempotencySuccessTx(ctx, tx, assistantIdempotencyKey{TenantID: "tenant_1", ConversationID: "conv_1", TurnID: "turn_1", TurnAction: "commit", RequestID: "req_1"}, badConv); err == nil {
		t.Fatal("marshal failure should return error")
	}
	if err := svc.finalizeIdempotencySuccessTx(ctx, tx, assistantIdempotencyKey{TenantID: "tenant_1", ConversationID: "conv_1", TurnID: "turn_1", TurnAction: "commit", RequestID: "req_1"}, conv); err != nil {
		t.Fatalf("finalize success err=%v", err)
	}

	if claim, err := svc.claimIdempotencyTx(ctx, tx, assistantIdempotencyKey{TenantID: "tenant_1", ConversationID: "conv_1", TurnID: "turn_1", TurnAction: "commit", RequestID: "req_1"}, "h1"); err != nil || claim.State != assistantIdempotencyClaimInserted {
		t.Fatalf("unexpected claim=%+v err=%v", claim, err)
	}

	tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		switch {
		case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
			return &assistFakeRow{err: pgx.ErrNoRows}
		case strings.Contains(sql, "SELECT request_hash"):
			return &assistFakeRow{vals: []any{"h1", "done", 409, errAssistantConfirmationRequired.Error(), []byte("null")}}
		default:
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
	}
	if claim, err := svc.claimIdempotencyTx(ctx, tx, assistantIdempotencyKey{TenantID: "tenant_1", ConversationID: "conv_1", TurnID: "turn_1", TurnAction: "commit", RequestID: "req_1"}, "h1"); err != nil || claim.State != assistantIdempotencyClaimDone {
		t.Fatalf("unexpected claim=%+v err=%v", claim, err)
	}
	if claim, err := svc.claimIdempotencyTx(ctx, tx, assistantIdempotencyKey{TenantID: "tenant_1", ConversationID: "conv_1", TurnID: "turn_1", TurnAction: "commit", RequestID: "req_1"}, "h2"); err != nil || claim.State != assistantIdempotencyClaimConflict {
		t.Fatalf("unexpected claim=%+v err=%v", claim, err)
	}

	tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		switch {
		case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
			return &assistFakeRow{err: pgx.ErrNoRows}
		case strings.Contains(sql, "SELECT request_hash"):
			return &assistFakeRow{vals: []any{"h1", "pending", nil, nil, []byte(nil)}}
		default:
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
	}
	if claim, err := svc.claimIdempotencyTx(ctx, tx, assistantIdempotencyKey{TenantID: "tenant_1", ConversationID: "conv_1", TurnID: "turn_1", TurnAction: "commit", RequestID: "req_1"}, "h1"); err != nil || claim.State != assistantIdempotencyClaimInProgress {
		t.Fatalf("unexpected claim=%+v err=%v", claim, err)
	}

	if err := svc.finalizeIdempotencyErrorTx(ctx, tx, assistantIdempotencyKey{TenantID: "tenant_1", ConversationID: "conv_1", TurnID: "turn_1", TurnAction: "commit", RequestID: "req_1"}, errAssistantCandidateNotFound); err != nil {
		t.Fatalf("finalize known error err=%v", err)
	}
	if err := svc.finalizeIdempotencyErrorTx(ctx, tx, assistantIdempotencyKey{TenantID: "tenant_1", ConversationID: "conv_1", TurnID: "turn_1", TurnAction: "commit", RequestID: "req_1"}, errors.New("unknown")); err != nil {
		t.Fatalf("finalize unknown error err=%v", err)
	}
	tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		if strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions") {
			return &assistFakeRow{vals: []any{int64(10)}}
		}
		return &assistFakeRow{err: pgx.ErrNoRows}
	}

	if err := svc.insertTransitionTx(ctx, tx, "tenant_1", "conv_1", nil); err != nil {
		t.Fatalf("nil transition err=%v", err)
	}
	transition := &assistantStateTransition{FromState: assistantStateValidated, ToState: assistantStateConfirmed}
	if err := svc.insertTransitionTx(ctx, tx, "tenant_1", "conv_1", transition); err != nil {
		t.Fatalf("insert transition err=%v", err)
	}
	if transition.ID == 0 || transition.ActorID == "" || transition.RequestID == "" || transition.TraceID == "" {
		t.Fatalf("transition defaults not populated: %+v", transition)
	}

	turn := &assistantTurn{TurnID: "turn_1", UserInput: "输入", State: assistantStateValidated, RiskTier: "high", RequestID: "req_1", TraceID: "trace_1", PolicyVersion: "2026-02-23", CompositionVersion: "2026-02-23", MappingVersion: "2026-02-23", Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}}, DryRun: assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, nil, ""), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := svc.upsertTurnTx(ctx, tx, "tenant_1", "conv_1", turn); err != nil {
		t.Fatalf("upsert err=%v", err)
	}

	badTurn := *turn
	badTurn.Plan.ConfigDeltaPlan.Changes = []assistantConfigChange{{Field: "x", After: func() {}}}
	if err := svc.upsertTurnTx(ctx, tx, "tenant_1", "conv_1", &badTurn); err == nil {
		t.Fatal("bad plan should fail marshal")
	}
	badTurn = *turn
	badTurn.DryRun.Diff = []map[string]any{{"bad": func() {}}}
	if err := svc.upsertTurnTx(ctx, tx, "tenant_1", "conv_1", &badTurn); err == nil {
		t.Fatal("bad dryrun should fail marshal")
	}

	if err := svc.persistConversationCreate(ctx, conv); err != nil {
		t.Fatalf("persist conversation err=%v", err)
	}
	if err := svc.updateConversationStateTx(ctx, tx, "tenant_1", "conv_1", assistantStateConfirmed, assistantPhaseAwaitCommitConfirm, time.Now().UTC()); err != nil {
		t.Fatalf("update state err=%v", err)
	}

	svc.cacheConversation(conv)
	if got, err := svc.getConversationPG(ctx, "tenant_1", "actor_1", "conv_1"); err != nil || got.ConversationID != "conv_1" {
		t.Fatalf("unexpected get cached err=%v got=%+v", err, got)
	}
	if _, err := svc.getConversationPG(ctx, "tenant_2", "actor_1", "conv_1"); !errors.Is(err, errAssistantTenantMismatch) {
		t.Fatalf("unexpected err=%v", err)
	}
	if _, err := svc.getConversationPG(ctx, "tenant_1", "actor_x", "conv_1"); !errors.Is(err, errAssistantConversationForbidden) {
		t.Fatalf("unexpected err=%v", err)
	}
}

func bodyOrNull(conv *assistantConversation) []byte {
	if conv == nil {
		return []byte("null")
	}
	body, _ := json.Marshal(conv)
	return body
}

func assistantTurnRowValues(turn *assistantTurn) []any {
	assistantRefreshTurnDerivedFields(turn)
	intentJSON, _ := json.Marshal(turn.Intent)
	planJSON, _ := json.Marshal(turn.Plan)
	candidatesJSON, _ := json.Marshal(turn.Candidates)
	candidateOptionsJSON := []byte(assistantCandidateOptionsJSON(turn))
	var routeDecisionJSON []byte
	if assistantIntentRouteDecisionPresent(turn.RouteDecision) {
		routeDecisionJSON, _ = json.Marshal(turn.RouteDecision)
	}
	dryRunJSON, _ := json.Marshal(turn.DryRun)
	missingFieldsJSON := []byte(assistantMissingFieldsJSON(turn))
	var commitJSON []byte
	if turn.CommitResult != nil {
		commitJSON, _ = json.Marshal(turn.CommitResult)
	}
	var commitReplyJSON []byte
	if reply := assistantTurnCommitReply(turn); reply != nil {
		commitReplyJSON, _ = json.Marshal(reply)
	}
	var phase any
	if strings.TrimSpace(turn.Phase) != "" {
		phase = turn.Phase
	}
	var resolved any
	if strings.TrimSpace(turn.ResolvedCandidateID) != "" {
		resolved = turn.ResolvedCandidateID
	}
	var selected any
	if strings.TrimSpace(turn.SelectedCandidateID) != "" {
		selected = turn.SelectedCandidateID
	}
	var source any
	if strings.TrimSpace(turn.ResolutionSource) != "" {
		source = turn.ResolutionSource
	}
	var pendingDraft any
	if strings.TrimSpace(turn.PendingDraftSummary) != "" {
		pendingDraft = turn.PendingDraftSummary
	}
	var errorCode any
	if strings.TrimSpace(turn.ErrorCode) != "" {
		errorCode = turn.ErrorCode
	}
	return []any{
		turn.TurnID,
		turn.UserInput,
		turn.State,
		phase,
		turn.RiskTier,
		turn.RequestID,
		turn.TraceID,
		turn.PolicyVersion,
		turn.CompositionVersion,
		turn.MappingVersion,
		intentJSON,
		planJSON,
		candidatesJSON,
		candidateOptionsJSON,
		resolved,
		selected,
		turn.AmbiguityCount,
		turn.Confidence,
		source,
		routeDecisionJSON,
		dryRunJSON,
		pendingDraft,
		missingFieldsJSON,
		commitJSON,
		commitReplyJSON,
		errorCode,
		turn.CreatedAt,
		turn.UpdatedAt,
	}
}

func TestAssistantPersistence_PGFlowCreateConversation(t *testing.T) {
	now := time.Now().UTC()
	tx := &assistFakeTx{}
	tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		if strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions") {
			return &assistFakeRow{vals: []any{int64(1)}}
		}
		return &assistFakeRow{err: pgx.ErrNoRows}
	}
	tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
		if strings.Contains(sql, "FROM iam.assistant_turns") || strings.Contains(sql, "FROM iam.assistant_state_transitions") {
			return &assistFakeRows{}, nil
		}
		return &assistFakeRows{}, nil
	}
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	svc.pool = assistFakeTxBeginner{tx: tx}
	conv, err := svc.createConversationPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"})
	if err != nil {
		t.Fatalf("createConversationPG err=%v", err)
	}
	if conv.ConversationID == "" || conv.ActorID != "actor_1" || tx.committed == false {
		t.Fatalf("unexpected conversation=%+v committed=%v", conv, tx.committed)
	}

	svc.mu.Lock()
	svc.byID = map[string]*assistantConversation{}
	svc.mu.Unlock()
	tx2 := &assistFakeTx{}
	tx2.queryRowFn = func(sql string, _ ...any) pgx.Row {
		if strings.Contains(sql, "FROM iam.assistant_conversations") {
			return &assistFakeRow{vals: []any{conv.ConversationID, "tenant_1", "actor_1", "tenant-admin", assistantStateValidated, assistantConversationPhaseFromLegacyState(assistantStateValidated), now, now}}
		}
		return &assistFakeRow{err: pgx.ErrNoRows}
	}
	tx2.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
		if strings.Contains(sql, "FROM iam.assistant_turns") || strings.Contains(sql, "FROM iam.assistant_state_transitions") {
			return &assistFakeRows{}, nil
		}
		return &assistFakeRows{}, nil
	}
	svc.pool = assistFakeTxBeginner{tx: tx2}
	got, err := svc.getConversationPG(context.Background(), "tenant_1", "actor_1", conv.ConversationID)
	if err != nil || got.ConversationID != conv.ConversationID {
		t.Fatalf("getConversationPG err=%v got=%+v", err, got)
	}
}

func TestAssistantPersistence_PGFlowCreateConfirmCommitTurn(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true)
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})

	conversationID := "conv_pg_flow"
	now := time.Now().UTC()
	conv := &assistantConversation{
		ConversationID: conversationID,
		TenantID:       "tenant_1",
		ActorID:        "actor_1",
		ActorRole:      "tenant-admin",
		State:          assistantStateValidated,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	turnForConfirm := &assistantTurn{
		TurnID:             "turn_confirm_1",
		UserInput:          "请创建部门",
		State:              assistantStateValidated,
		RiskTier:           "high",
		RequestID:          "req_confirm_1",
		TraceID:            "trace_confirm_1",
		PolicyVersion:      "2026-02-23",
		CompositionVersion: "2026-02-23",
		MappingVersion:     "2026-02-23",
		Intent: assistantIntentSpec{
			Action:              assistantIntentCreateOrgUnit,
			ParentRefText:       "鲜花组织",
			EntityName:          "运营部",
			EffectiveDate:       "2026-01-01",
			IntentSchemaVersion: assistantIntentSchemaVersionV1,
		},
		Plan: assistantPlanSummary{
			Title:                   "创建组织",
			CapabilityKey:           "org.orgunit_create.field_policy",
			CapabilityMapVersion:    assistantCapabilityMapVersionV1,
			CompilerContractVersion: assistantCompilerContractVersionV1,
			SkillManifestDigest:     "digest",
		},
		Candidates: []assistantCandidate{
			{CandidateID: "c1", CandidateCode: "FLOWER-A"},
			{CandidateID: "c2", CandidateCode: "FLOWER-B"},
		},
		AmbiguityCount: 2,
		Confidence:     0.55,
		DryRun:         assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}, {CandidateID: "c2", CandidateCode: "FLOWER-B"}}, ""),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	turnForCommit := *turnForConfirm
	turnForCommit.TurnID = "turn_commit_1"
	turnForCommit.State = assistantStateConfirmed
	turnForCommit.ResolvedCandidateID = "c1"
	turnForCommit.ResolutionSource = assistantResolutionUserConfirmed
	turnForCommit.RequestID = "req_commit_1"
	turnForCommit.TraceID = "trace_commit_1"
	if err := svc.refreshTurnVersionTuple(context.Background(), "tenant_1", &turnForCommit); err != nil {
		t.Fatalf("refresh commit turn version tuple err=%v", err)
	}

	stage := "create"
	tx := &assistFakeTx{}
	tx.queryRowFn = func(sql string, args ...any) pgx.Row {
		switch {
		case strings.Contains(sql, "FROM iam.assistant_conversations"):
			return &assistFakeRow{vals: []any{conv.ConversationID, conv.TenantID, conv.ActorID, conv.ActorRole, conv.State, assistantConversationPhaseFromLegacyState(conv.State), conv.CreatedAt, conv.UpdatedAt}}
		case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
			return &assistFakeRow{vals: []any{int64(100)}}
		case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
			return &assistFakeRow{vals: []any{1}}
		case strings.Contains(sql, "SELECT request_hash"):
			return &assistFakeRow{vals: []any{"h", "pending", nil, nil, []byte(nil)}}
		default:
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
	}
	tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
		switch {
		case strings.Contains(sql, "FROM iam.assistant_turns"):
			if stage == "confirm" {
				return &assistFakeRows{rows: [][]any{assistantTurnRowValues(turnForConfirm)}}, nil
			}
			if stage == "commit" {
				return &assistFakeRows{rows: [][]any{assistantTurnRowValues(&turnForCommit)}}, nil
			}
			return &assistFakeRows{}, nil
		case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
			return &assistFakeRows{}, nil
		default:
			return &assistFakeRows{}, nil
		}
	}
	svc.pool = assistFakeTxBeginner{tx: tx}

	created, err := svc.createTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, conversationID, "测试普通对话，不执行写入")
	if err != nil {
		t.Fatalf("createTurnPG err=%v", err)
	}
	if len(created.Turns) != 1 {
		t.Fatalf("created turns=%d", len(created.Turns))
	}

	stage = "confirm"
	confirmed, err := svc.confirmTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, conversationID, turnForConfirm.TurnID, "c1")
	if err != nil {
		t.Fatalf("confirmTurnPG err=%v", err)
	}
	if len(confirmed.Turns) != 1 || confirmed.Turns[0].State != assistantStateConfirmed {
		t.Fatalf("unexpected confirmed conversation=%+v", confirmed)
	}

	stage = "commit"
	committed, err := svc.commitTurnPG(context.Background(), "tenant_1", Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, conversationID, turnForCommit.TurnID)
	if err != nil {
		t.Fatalf("commitTurnPG err=%v", err)
	}
	if len(committed.Turns) != 1 || committed.Turns[0].State != assistantStateCommitted || committed.Turns[0].CommitResult == nil {
		t.Fatalf("unexpected committed conversation=%+v", committed)
	}
}
