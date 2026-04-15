package cubeboxsqlc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestConversationQueries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenantUUID := mustUUIDValue(t, 1)
	changedAt := timestamptzValue(1700000001)
	createdAt := timestamptzValue(1700000002)
	updatedAt := timestamptzValue(1700000003)
	turnID := "turn-1"
	turnAction := "submit"
	reasonCode := "reason"
	resolvedCandidateID := "candidate-1"
	selectedCandidateID := "candidate-2"
	resolutionSource := "router"
	pendingDraftSummary := "summary"
	errorCode := "route_failed"

	q := New(fakeDB{
		row: fakeRow{vals: []any{int64(3)}},
	})
	count, err := q.CountBlockingTasksForConversation(ctx, CountBlockingTasksForConversationParams{
		TenantUuid:     tenantUUID,
		ConversationID: "conversation-1",
	})
	if err != nil {
		t.Fatalf("count blocking tasks: %v", err)
	}
	if count != 3 {
		t.Fatalf("unexpected count: %d", count)
	}

	q = New(fakeDB{
		execTag: pgconn.NewCommandTag("DELETE 2"),
	})
	rows, err := q.DeleteConversationByID(ctx, DeleteConversationByIDParams{
		TenantUuid:     tenantUUID,
		ConversationID: "conversation-1",
	})
	if err != nil {
		t.Fatalf("delete conversation: %v", err)
	}
	if rows != 2 {
		t.Fatalf("unexpected rows affected: %d", rows)
	}

	q = New(fakeDB{
		row: fakeRow{vals: []any{
			tenantUUID,
			"conversation-1",
			"actor-1",
			"user",
			"active",
			"clarify",
			createdAt,
			updatedAt,
		}},
	})
	conversation, err := q.GetConversationByID(ctx, GetConversationByIDParams{
		TenantUuid:     tenantUUID,
		ConversationID: "conversation-1",
	})
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if conversation.ConversationID != "conversation-1" || conversation.ActorID != "actor-1" {
		t.Fatalf("unexpected conversation: %+v", conversation)
	}

	q = New(fakeDB{
		rows: &fakeRows{records: [][]any{{
			int64(10),
			tenantUUID,
			"conversation-1",
			turnID,
			turnAction,
			"request-1",
			"trace-1",
			"draft",
			"active",
			"resolve",
			"commit",
			reasonCode,
			"actor-1",
			changedAt,
		}}},
	})
	transitions, err := q.ListConversationStateTransitions(ctx, ListConversationStateTransitionsParams{
		TenantUuid:     tenantUUID,
		ConversationID: "conversation-1",
	})
	if err != nil {
		t.Fatalf("list state transitions: %v", err)
	}
	if len(transitions) != 1 || transitions[0].ReasonCode == nil || *transitions[0].ReasonCode != reasonCode {
		t.Fatalf("unexpected transitions: %+v", transitions)
	}

	q = New(fakeDB{
		rows: &fakeRows{records: [][]any{{
			tenantUUID,
			"conversation-1",
			turnID,
			"hello",
			"clarifying",
			"resolve",
			"low",
			"request-1",
			"trace-1",
			"policy-v1",
			"composition-v1",
			"mapping-v1",
			[]byte(`{"intent":"search"}`),
			[]byte(`{"plan":"do"}`),
			[]byte(`[]`),
			[]byte(`[]`),
			resolvedCandidateID,
			selectedCandidateID,
			int32(2),
			float64(0.85),
			resolutionSource,
			[]byte(`{"route":"clarify"}`),
			[]byte(`{"ask":"who"}`),
			[]byte(`{"dry_run":true}`),
			pendingDraftSummary,
			[]byte(`["field_a"]`),
			[]byte(`{"ok":true}`),
			[]byte(`reply`),
			errorCode,
			createdAt,
			updatedAt,
		}}},
	})
	turns, err := q.ListConversationTurns(ctx, ListConversationTurnsParams{
		TenantUuid:     tenantUUID,
		ConversationID: "conversation-1",
	})
	if err != nil {
		t.Fatalf("list turns: %v", err)
	}
	if len(turns) != 1 || turns[0].ResolvedCandidateID == nil || *turns[0].ResolvedCandidateID != resolvedCandidateID {
		t.Fatalf("unexpected turns: %+v", turns)
	}

	q = New(fakeDB{
		rows: &fakeRows{records: [][]any{{
			tenantUUID,
			"conversation-1",
			"actor-1",
			"user",
			"active",
			"resolve",
			createdAt,
			updatedAt,
		}}},
	})
	conversations, err := q.ListConversationsByActor(ctx, ListConversationsByActorParams{
		TenantUuid: tenantUUID,
		ActorID:    "actor-1",
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if len(conversations) != 1 || conversations[0].ConversationID != "conversation-1" {
		t.Fatalf("unexpected conversations: %+v", conversations)
	}
}

func TestTaskQueries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenantUUID := mustUUIDValue(t, 2)
	taskUUID := mustUUIDValue(t, 3)
	createdAt := timestamptzValue(1700000101)
	updatedAt := timestamptzValue(1700000102)
	deadlineAt := timestamptzValue(1700000103)
	submittedAt := timestamptzValue(1700000104)
	cancelRequestedAt := timestamptzValue(1700000105)
	completedAt := timestamptzValue(1700000106)
	lastErrorCode := "dispatch_failed"
	traceID := "trace-1"
	knowledgeSnapshotDigest := "knowledge-v1"
	routeCatalogVersion := "route-v1"
	resolverContractVersion := "resolver-v1"
	contextTemplateVersion := "template-v1"
	replyGuidanceVersion := "guidance-v1"
	policyContextDigest := "policy-digest"
	effectivePolicyVersion := "policy-v1"
	resolvedSetid := "S2601"
	setidSource := "registry"
	precheckProjectionDigest := "projection-v1"
	mutationPolicyVersion := "mutation-v1"
	fromStatus := "queued"
	eventErrorCode := "event_failed"

	q := New(fakeDB{
		row: fakeRow{vals: []any{
			tenantUUID,
			taskUUID,
			"conversation-1",
			"turn-1",
			"reply",
			"request-1",
			"request-hash",
			"workflow-1",
			"running",
			"queued",
			int32(1),
			deadlineAt,
			int32(2),
			int32(5),
			lastErrorCode,
			traceID,
			"intent-v1",
			"compiler-v1",
			"capability-v1",
			"skill-digest",
			"context-hash",
			"intent-hash",
			"plan-hash",
			knowledgeSnapshotDigest,
			routeCatalogVersion,
			resolverContractVersion,
			contextTemplateVersion,
			replyGuidanceVersion,
			policyContextDigest,
			effectivePolicyVersion,
			resolvedSetid,
			setidSource,
			precheckProjectionDigest,
			mutationPolicyVersion,
			submittedAt,
			cancelRequestedAt,
			completedAt,
			createdAt,
			updatedAt,
		}},
	})
	task, err := q.GetTaskByID(ctx, GetTaskByIDParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
	})
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.TaskType != "reply" || task.KnowledgeSnapshotDigest == nil || *task.KnowledgeSnapshotDigest != knowledgeSnapshotDigest {
		t.Fatalf("unexpected task: %+v", task)
	}

	q = New(fakeDB{
		rows: &fakeRows{records: [][]any{{
			int64(11),
			tenantUUID,
			taskUUID,
			"workflow-1",
			"queued",
			int32(2),
			deadlineAt,
			createdAt,
			updatedAt,
		}}},
	})
	outbox, err := q.ListDispatchOutboxByStatus(ctx, ListDispatchOutboxByStatusParams{
		TenantUuid: tenantUUID,
		Status:     "queued",
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("list dispatch outbox: %v", err)
	}
	if len(outbox) != 1 || outbox[0].WorkflowID != "workflow-1" {
		t.Fatalf("unexpected outbox: %+v", outbox)
	}

	q = New(fakeDB{
		rows: &fakeRows{records: [][]any{{
			int64(12),
			tenantUUID,
			taskUUID,
			fromStatus,
			"running",
			"dispatched",
			eventErrorCode,
			[]byte(`{"retry":1}`),
			createdAt,
		}}},
	})
	events, err := q.ListTaskEventsByTask(ctx, ListTaskEventsByTaskParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
	})
	if err != nil {
		t.Fatalf("list task events: %v", err)
	}
	if len(events) != 1 || events[0].FromStatus == nil || *events[0].FromStatus != fromStatus {
		t.Fatalf("unexpected task events: %+v", events)
	}
}

func TestFileQueries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenantUUID := mustUUIDValue(t, 4)
	uploadedAt := timestamptzValue(1700000201)
	updatedAt := timestamptzValue(1700000202)
	scanErrorCode := "scan_failed"
	turnID := "turn-2"

	q := New(fakeDB{
		row: fakeRow{vals: []any{
			tenantUUID,
			"file-1",
			"local",
			"tenant-a/file-1/doc.txt",
			"doc.txt",
			"text/plain",
			int64(42),
			"sha256",
			"clean",
			scanErrorCode,
			"actor-1",
			uploadedAt,
			updatedAt,
		}},
	})
	file, err := q.GetFileByID(ctx, GetFileByIDParams{
		TenantUuid: tenantUUID,
		FileID:     "file-1",
	})
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	if file.FileName != "doc.txt" || file.ScanErrorCode == nil || *file.ScanErrorCode != scanErrorCode {
		t.Fatalf("unexpected file: %+v", file)
	}

	q = New(fakeDB{
		rows: &fakeRows{records: [][]any{{
			int64(13),
			tenantUUID,
			"file-1",
			"conversation-1",
			turnID,
			"conversation_attachment",
			"actor-1",
			uploadedAt,
		}}},
	})
	links, err := q.ListFileLinksByConversation(ctx, ListFileLinksByConversationParams{
		TenantUuid:     tenantUUID,
		ConversationID: "conversation-1",
	})
	if err != nil {
		t.Fatalf("list file links: %v", err)
	}
	if len(links) != 1 || links[0].TurnID == nil || *links[0].TurnID != turnID {
		t.Fatalf("unexpected links: %+v", links)
	}

	fileRow := []any{
		tenantUUID,
		"file-1",
		"local",
		"tenant-a/file-1/doc.txt",
		"doc.txt",
		"text/plain",
		int64(42),
		"sha256",
		"clean",
		scanErrorCode,
		"actor-1",
		uploadedAt,
		updatedAt,
	}
	q = New(fakeDB{
		rows: &fakeRows{records: [][]any{fileRow}},
	})
	filesByConversation, err := q.ListFilesByConversation(ctx, ListFilesByConversationParams{
		TenantUuid:     tenantUUID,
		ConversationID: "conversation-1",
	})
	if err != nil {
		t.Fatalf("list files by conversation: %v", err)
	}
	if len(filesByConversation) != 1 || filesByConversation[0].FileID != "file-1" {
		t.Fatalf("unexpected files by conversation: %+v", filesByConversation)
	}

	q = New(fakeDB{
		rows: &fakeRows{records: [][]any{fileRow}},
	})
	filesByTenant, err := q.ListFilesByTenant(ctx, ListFilesByTenantParams{
		TenantUuid: tenantUUID,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("list files by tenant: %v", err)
	}
	if len(filesByTenant) != 1 || filesByTenant[0].StorageProvider != "local" {
		t.Fatalf("unexpected files by tenant: %+v", filesByTenant)
	}
}

func TestGeneratedQueryErrorPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenantUUID := mustUUIDValue(t, 5)
	taskUUID := mustUUIDValue(t, 6)

	q := New(fakeDB{execErr: errors.New("exec failed")})
	if _, err := q.DeleteConversationByID(ctx, DeleteConversationByIDParams{TenantUuid: tenantUUID, ConversationID: "conversation-1"}); err == nil {
		t.Fatal("expected exec error")
	}

	q = New(fakeDB{row: fakeRow{err: errors.New("row failed")}})
	if _, err := q.GetConversationByID(ctx, GetConversationByIDParams{TenantUuid: tenantUUID, ConversationID: "conversation-1"}); err == nil {
		t.Fatal("expected row error")
	}

	q = New(fakeDB{queryErr: errors.New("query failed")})
	if _, err := q.ListFilesByTenant(ctx, ListFilesByTenantParams{TenantUuid: tenantUUID, Limit: 1}); err == nil {
		t.Fatal("expected query error")
	}

	q = New(fakeDB{rows: &fakeRows{records: [][]any{{}}, scanErr: errors.New("scan failed")}})
	if _, err := q.ListTaskEventsByTask(ctx, ListTaskEventsByTaskParams{TenantUuid: tenantUUID, TaskID: taskUUID}); err == nil {
		t.Fatal("expected scan error")
	}

	q = New(fakeDB{rows: &fakeRows{records: [][]any{{}}, err: errors.New("rows failed")}})
	if _, err := q.ListConversationTurns(ctx, ListConversationTurnsParams{TenantUuid: tenantUUID, ConversationID: "conversation-1"}); err == nil {
		t.Fatal("expected rows error")
	}
}

func TestGeneratedListQueryAdditionalErrorPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenantUUID := mustUUIDValue(t, 7)
	taskUUID := mustUUIDValue(t, 8)

	t.Run("conversation state transitions query and rows errors", func(t *testing.T) {
		q := New(fakeDB{queryErr: errors.New("query failed")})
		if _, err := q.ListConversationStateTransitions(ctx, ListConversationStateTransitionsParams{
			TenantUuid:     tenantUUID,
			ConversationID: "conversation-1",
		}); err == nil {
			t.Fatal("expected query error")
		}

		q = New(fakeDB{rows: &fakeRows{records: [][]any{}, err: errors.New("rows failed")}})
		if _, err := q.ListConversationStateTransitions(ctx, ListConversationStateTransitionsParams{
			TenantUuid:     tenantUUID,
			ConversationID: "conversation-1",
		}); err == nil {
			t.Fatal("expected rows error")
		}
	})

	t.Run("conversations by actor query and scan errors", func(t *testing.T) {
		q := New(fakeDB{queryErr: errors.New("query failed")})
		if _, err := q.ListConversationsByActor(ctx, ListConversationsByActorParams{
			TenantUuid: tenantUUID,
			ActorID:    "actor-1",
			Limit:      1,
		}); err == nil {
			t.Fatal("expected query error")
		}

		q = New(fakeDB{rows: &fakeRows{records: [][]any{{}}, scanErr: errors.New("scan failed")}})
		if _, err := q.ListConversationsByActor(ctx, ListConversationsByActorParams{
			TenantUuid: tenantUUID,
			ActorID:    "actor-1",
			Limit:      1,
		}); err == nil {
			t.Fatal("expected scan error")
		}
	})

	t.Run("dispatch outbox query and rows errors", func(t *testing.T) {
		q := New(fakeDB{queryErr: errors.New("query failed")})
		if _, err := q.ListDispatchOutboxByStatus(ctx, ListDispatchOutboxByStatusParams{
			TenantUuid: tenantUUID,
			Status:     "queued",
			Limit:      1,
		}); err == nil {
			t.Fatal("expected query error")
		}

		q = New(fakeDB{rows: &fakeRows{records: [][]any{}, err: errors.New("rows failed")}})
		if _, err := q.ListDispatchOutboxByStatus(ctx, ListDispatchOutboxByStatusParams{
			TenantUuid: tenantUUID,
			Status:     "queued",
			Limit:      1,
		}); err == nil {
			t.Fatal("expected rows error")
		}
	})

	t.Run("file listing branches", func(t *testing.T) {
		q := New(fakeDB{queryErr: errors.New("query failed")})
		if _, err := q.ListFilesByConversation(ctx, ListFilesByConversationParams{
			TenantUuid:     tenantUUID,
			ConversationID: "conversation-1",
		}); err == nil {
			t.Fatal("expected conversation files query error")
		}

		q = New(fakeDB{rows: &fakeRows{records: [][]any{{}}, err: errors.New("rows failed")}})
		if _, err := q.ListFilesByConversation(ctx, ListFilesByConversationParams{
			TenantUuid:     tenantUUID,
			ConversationID: "conversation-1",
		}); err == nil {
			t.Fatal("expected conversation files rows error")
		}

		q = New(fakeDB{rows: &fakeRows{records: [][]any{{}}, scanErr: errors.New("scan failed")}})
		if _, err := q.ListFilesByTenant(ctx, ListFilesByTenantParams{
			TenantUuid: tenantUUID,
			Limit:      1,
		}); err == nil {
			t.Fatal("expected tenant files scan error")
		}

		q = New(fakeDB{rows: &fakeRows{records: [][]any{}, err: errors.New("rows failed")}})
		if _, err := q.ListFilesByTenant(ctx, ListFilesByTenantParams{
			TenantUuid: tenantUUID,
			Limit:      1,
		}); err == nil {
			t.Fatal("expected tenant files rows error")
		}
	})

	t.Run("remaining list helpers rows or scan errors", func(t *testing.T) {
		q := New(fakeDB{rows: &fakeRows{records: [][]any{{}}, err: errors.New("rows failed")}})
		if _, err := q.ListFileLinksByConversation(ctx, ListFileLinksByConversationParams{
			TenantUuid:     tenantUUID,
			ConversationID: "conversation-1",
		}); err == nil {
			t.Fatal("expected file links rows error")
		}

		q = New(fakeDB{rows: &fakeRows{records: [][]any{{}}, scanErr: errors.New("scan failed")}})
		if _, err := q.ListTaskEventsByTask(ctx, ListTaskEventsByTaskParams{
			TenantUuid: tenantUUID,
			TaskID:     taskUUID,
		}); err == nil {
			t.Fatal("expected task events scan error")
		}
	})
}

type fakeDB struct {
	execTag  pgconn.CommandTag
	execErr  error
	queryErr error
	rows     pgx.Rows
	row      pgx.Row
}

func (db fakeDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	if db.execErr != nil {
		return pgconn.CommandTag{}, db.execErr
	}
	if db.execTag != (pgconn.CommandTag{}) {
		return db.execTag, nil
	}
	return pgconn.NewCommandTag("SELECT 0"), nil
}

func (db fakeDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	if db.queryErr != nil {
		return nil, db.queryErr
	}
	if db.rows != nil {
		return db.rows, nil
	}
	return &fakeRows{}, nil
}

func (db fakeDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	if db.row != nil {
		return db.row
	}
	return fakeRow{}
}

type fakeRows struct {
	records [][]any
	index   int
	scanErr error
	err     error
}

func (r *fakeRows) Close()                        {}
func (r *fakeRows) Err() error                    { return r.err }
func (r *fakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeRows) Next() bool {
	if r.index >= len(r.records) {
		return false
	}
	r.index++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	var vals []any
	if r.index > 0 && r.index <= len(r.records) {
		vals = r.records[r.index-1]
	}
	return scanInto(dest, vals)
}

func (r *fakeRows) Values() ([]any, error) { return nil, nil }
func (r *fakeRows) RawValues() [][]byte    { return nil }
func (r *fakeRows) Conn() *pgx.Conn        { return nil }

type fakeRow struct {
	vals []any
	err  error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanInto(dest, r.vals)
}

func scanInto(dest []any, vals []any) error {
	for i := range dest {
		var val any
		if i < len(vals) {
			val = vals[i]
		}
		if err := assignValue(dest[i], val); err != nil {
			return err
		}
	}
	return nil
}

func assignValue(dest any, val any) error {
	switch d := dest.(type) {
	case *string:
		if val == nil {
			*d = ""
			return nil
		}
		*d = val.(string)
	case **string:
		if val == nil {
			*d = nil
			return nil
		}
		v := val.(string)
		*d = &v
	case *int32:
		if val == nil {
			*d = 0
			return nil
		}
		*d = val.(int32)
	case *int64:
		if val == nil {
			*d = 0
			return nil
		}
		*d = val.(int64)
	case *float64:
		if val == nil {
			*d = 0
			return nil
		}
		*d = val.(float64)
	case *[]byte:
		if val == nil {
			*d = nil
			return nil
		}
		*d = append([]byte(nil), val.([]byte)...)
	case *pgtype.UUID:
		if val == nil {
			*d = pgtype.UUID{}
			return nil
		}
		*d = val.(pgtype.UUID)
	case *pgtype.Timestamptz:
		if val == nil {
			*d = pgtype.Timestamptz{}
			return nil
		}
		*d = val.(pgtype.Timestamptz)
	default:
		return errors.New("unsupported scan destination")
	}
	return nil
}

func mustUUIDValue(t *testing.T, seed byte) pgtype.UUID {
	t.Helper()
	var bytes [16]byte
	bytes[15] = seed
	return pgtype.UUID{Bytes: bytes, Valid: true}
}

func timestamptzValue(unix int64) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  time.Unix(unix, 0).UTC(),
		Valid: true,
	}
}
