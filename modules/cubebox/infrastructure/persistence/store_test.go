package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
)

const (
	testTenantUUID = "11111111-1111-1111-1111-111111111111"
	testTaskUUID   = "22222222-2222-2222-2222-222222222222"
)

type beginnerFunc func(context.Context) (pgx.Tx, error)

func (f beginnerFunc) Begin(ctx context.Context) (pgx.Tx, error) { return f(ctx) }

type fakeTx struct {
	execErr    error
	execErrAt  int
	execN      int
	execTags   []pgconn.CommandTag
	execSQLs   []string
	queryErr   error
	querySQL   string
	queryArgs  []any
	rowSQL     string
	rowArgs    []any
	rowQueue   []pgx.Row
	rows       pgx.Rows
	row        pgx.Row
	commitErr  error
	committed  bool
	rolledBack bool
}

func (t *fakeTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *fakeTx) Commit(context.Context) error {
	t.committed = true
	return t.commitErr
}
func (t *fakeTx) Rollback(context.Context) error {
	t.rolledBack = true
	return nil
}
func (t *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return fakeBatchResults{} }
func (t *fakeTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *fakeTx) Conn() *pgx.Conn { return nil }

func (t *fakeTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.execN++
	t.execSQLs = append(t.execSQLs, sql)
	if t.execErr != nil {
		at := t.execErrAt
		if at == 0 {
			at = 1
		}
		if t.execN == at {
			return pgconn.CommandTag{}, t.execErr
		}
	}
	if t.execN <= len(t.execTags) {
		return t.execTags[t.execN-1], nil
	}
	return pgconn.NewCommandTag("SELECT 0"), nil
}

func (t *fakeTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.querySQL = sql
	t.queryArgs = append([]any(nil), args...)
	if t.queryErr != nil {
		return nil, t.queryErr
	}
	if t.rows != nil {
		return t.rows, nil
	}
	return &fakeRows{records: [][]any{{}}}, nil
}

func (t *fakeTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	t.rowSQL = sql
	t.rowArgs = append([]any(nil), args...)
	if len(t.rowQueue) > 0 {
		row := t.rowQueue[0]
		t.rowQueue = t.rowQueue[1:]
		return row
	}
	if t.row != nil {
		return t.row
	}
	return &fakeRow{}
}

type fakeBatchResults struct{}

func (fakeBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (fakeBatchResults) Query() (pgx.Rows, error)         { return &fakeRows{}, nil }
func (fakeBatchResults) QueryRow() pgx.Row                { return &fakeRow{} }
func (fakeBatchResults) Close() error                     { return nil }

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

func (r *fakeRow) Scan(dest ...any) error {
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
		if err := assignScanValue(dest[i], val); err != nil {
			return err
		}
	}
	return nil
}

func assignScanValue(dest any, val any) error {
	switch d := dest.(type) {
	case *string:
		if val == nil {
			*d = ""
			return nil
		}
		switch v := val.(type) {
		case string:
			*d = v
			return nil
		case *string:
			if v == nil {
				*d = ""
				return nil
			}
			*d = *v
			return nil
		default:
			return fmt.Errorf("unsupported string source %T", val)
		}
	case **string:
		if val == nil {
			*d = nil
			return nil
		}
		switch v := val.(type) {
		case string:
			s := v
			*d = &s
			return nil
		case *string:
			*d = v
			return nil
		default:
			return fmt.Errorf("unsupported string pointer source %T", val)
		}
	case *int64:
		if val == nil {
			*d = 0
			return nil
		}
		*d = val.(int64)
		return nil
	case *int32:
		if val == nil {
			*d = 0
			return nil
		}
		*d = val.(int32)
		return nil
	case *float64:
		if val == nil {
			*d = 0
			return nil
		}
		*d = val.(float64)
		return nil
	case *[]byte:
		if val == nil {
			*d = nil
			return nil
		}
		*d = append([]byte(nil), val.([]byte)...)
		return nil
	case *pgtype.UUID:
		if val == nil {
			*d = pgtype.UUID{}
			return nil
		}
		*d = val.(pgtype.UUID)
		return nil
	case *pgtype.Timestamptz:
		if val == nil {
			*d = pgtype.Timestamptz{}
			return nil
		}
		*d = val.(pgtype.Timestamptz)
		return nil
	default:
		return fmt.Errorf("unsupported scan destination %T", dest)
	}
}

func TestParseUUID(t *testing.T) {
	t.Parallel()

	parsed, err := parseUUID(testTenantUUID)
	if err != nil {
		t.Fatalf("parse uuid: %v", err)
	}
	if !parsed.Valid {
		t.Fatal("expected valid uuid")
	}
	if _, err := parseUUID("bad"); err == nil {
		t.Fatal("expected invalid uuid error")
	}
}

func TestBeginTenantTx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin failed")
		}))
		tx, queries, err := store.beginTenantTx(ctx, testTenantUUID)
		if err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got tx=%v queries=%v err=%v", tx, queries, err)
		}
	})

	t.Run("set config error rolls back", func(t *testing.T) {
		tx := &fakeTx{execErr: errors.New("set_config failed"), execErrAt: 1}
		store := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, _, err := store.beginTenantTx(ctx, testTenantUUID)
		if err == nil || !strings.Contains(err.Error(), "set_config failed") {
			t.Fatalf("expected set config error, got %v", err)
		}
		if !tx.rolledBack {
			t.Fatal("expected rollback on set_config error")
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &fakeTx{}
		store := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		gotTx, queries, err := store.beginTenantTx(ctx, testTenantUUID)
		if err != nil {
			t.Fatalf("begin tenant tx: %v", err)
		}
		if gotTx != tx || queries == nil {
			t.Fatalf("unexpected tx=%v queries=%v", gotTx, queries)
		}
		if len(tx.execSQLs) != 1 || !strings.Contains(tx.execSQLs[0], "set_config") {
			t.Fatalf("expected set_config call, got %v", tx.execSQLs)
		}
	})
}

func TestPGStoreSuccessPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	makeStore := func(tx *fakeTx) *PGStore {
		return NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
	}

	t.Run("list conversations uses default limit", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}}
		items, err := makeStore(tx).ListConversations(ctx, testTenantUUID, "actor-1", 0, time.Time{}, "")
		if err != nil {
			t.Fatalf("list conversations: %v", err)
		}
		if len(items) != 1 || !tx.committed {
			t.Fatalf("unexpected items=%d committed=%v", len(items), tx.committed)
		}
		if got, ok := tx.queryArgs[4].(int32); !ok || got != 20 {
			t.Fatalf("expected default limit 20, got %#v", tx.queryArgs)
		}
	})

	t.Run("get conversation", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{}}
		item, err := makeStore(tx).GetConversation(ctx, testTenantUUID, "conversation-1")
		if err != nil {
			t.Fatalf("get conversation: %v", err)
		}
		if item.ConversationID != "" || !tx.committed {
			t.Fatalf("unexpected item=%+v committed=%v", item, tx.committed)
		}
	})

	t.Run("list conversation turns", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}}
		items, err := makeStore(tx).ListConversationTurns(ctx, testTenantUUID, "conversation-1")
		if err != nil {
			t.Fatalf("list turns: %v", err)
		}
		if len(items) != 1 || !tx.committed {
			t.Fatalf("unexpected items=%d committed=%v", len(items), tx.committed)
		}
	})

	t.Run("list state transitions", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}}
		items, err := makeStore(tx).ListConversationStateTransitions(ctx, testTenantUUID, "conversation-1")
		if err != nil {
			t.Fatalf("list transitions: %v", err)
		}
		if len(items) != 1 || !tx.committed {
			t.Fatalf("unexpected items=%d committed=%v", len(items), tx.committed)
		}
	})

	t.Run("sync conversation snapshot", func(t *testing.T) {
		now := time.Date(2026, 4, 15, 16, 0, 0, 0, time.UTC)
		tx := &fakeTx{}
		err := makeStore(tx).SyncConversationSnapshot(ctx, testTenantUUID, cubeboxdomain.Conversation{
			ConversationID: "conv_1",
			ActorID:        "actor-1",
			ActorRole:      "tenant-admin",
			State:          "confirmed",
			CurrentPhase:   "await_commit_confirm",
			CreatedAt:      now,
			UpdatedAt:      now,
			Turns: []cubeboxdomain.ConversationTurn{{
				TurnID:              "turn_1",
				UserInput:           "create org",
				State:               "confirmed",
				Phase:               "await_commit_confirm",
				RiskTier:            "medium",
				RequestID:           "assistant_req",
				TraceID:             "trace",
				PolicyVersion:       "policy.v1",
				CompositionVersion:  "composition.v1",
				MappingVersion:      "mapping.v1",
				Intent:              map[string]any{"action": "create_orgunit"},
				Plan:                map[string]any{"summary": "confirm"},
				Candidates:          []map[string]any{{"candidate_id": "cand_1"}},
				ResolvedCandidateID: "cand_1",
				DryRun:              map[string]any{"plan_hash": "plan"},
				MissingFields:       []string{},
				CreatedAt:           now,
				UpdatedAt:           now,
			}},
			Transitions: []cubeboxdomain.StateTransition{{
				TurnID:     "turn_1",
				TurnAction: "confirm",
				RequestID:  "assistant_req",
				TraceID:    "trace",
				FromState:  "validated",
				ToState:    "confirmed",
				FromPhase:  "await_candidate_confirm",
				ToPhase:    "await_commit_confirm",
				ReasonCode: "confirmed",
				ActorID:    "actor-1",
				ChangedAt:  now,
			}},
		})
		if err != nil {
			t.Fatalf("sync conversation snapshot: %v", err)
		}
		if !tx.committed {
			t.Fatal("expected commit")
		}
		if len(tx.execSQLs) != 4 {
			t.Fatalf("unexpected exec count=%d sqls=%v", len(tx.execSQLs), tx.execSQLs)
		}
		if !strings.Contains(tx.execSQLs[1], "INSERT INTO iam.cubebox_conversations") {
			t.Fatalf("unexpected conversation sql=%q", tx.execSQLs[1])
		}
		if !strings.Contains(tx.execSQLs[2], "INSERT INTO iam.cubebox_turns") {
			t.Fatalf("unexpected turn sql=%q", tx.execSQLs[2])
		}
		if !strings.Contains(tx.execSQLs[3], "INSERT INTO iam.cubebox_state_transitions") {
			t.Fatalf("unexpected transition sql=%q", tx.execSQLs[3])
		}
	})

	t.Run("count blocking tasks", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{vals: []any{int64(7)}}}
		count, err := makeStore(tx).CountBlockingTasks(ctx, testTenantUUID, "conversation-1")
		if err != nil {
			t.Fatalf("count tasks: %v", err)
		}
		if count != 7 || !tx.committed {
			t.Fatalf("unexpected count=%d committed=%v", count, tx.committed)
		}
	})

	t.Run("delete conversation", func(t *testing.T) {
		tx := &fakeTx{execTags: []pgconn.CommandTag{pgconn.NewCommandTag("SELECT 1"), pgconn.NewCommandTag("DELETE 2")}}
		rows, err := makeStore(tx).DeleteConversation(ctx, testTenantUUID, "conversation-1")
		if err != nil {
			t.Fatalf("delete conversation: %v", err)
		}
		if rows != 2 || !tx.committed {
			t.Fatalf("unexpected rows=%d committed=%v", rows, tx.committed)
		}
		if len(tx.execSQLs) != 2 || !strings.Contains(tx.execSQLs[1], "DELETE FROM iam.cubebox_conversations") {
			t.Fatalf("unexpected exec sqls: %v", tx.execSQLs)
		}
	})

	t.Run("get task", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{}}
		item, err := makeStore(tx).GetTask(ctx, testTenantUUID, testTaskUUID)
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		if item.TaskType != "" || !tx.committed {
			t.Fatalf("unexpected item=%+v committed=%v", item, tx.committed)
		}
	})

	t.Run("list task events", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}}
		items, err := makeStore(tx).ListTaskEvents(ctx, testTenantUUID, testTaskUUID)
		if err != nil {
			t.Fatalf("list task events: %v", err)
		}
		if len(items) != 1 || !tx.committed {
			t.Fatalf("unexpected items=%d committed=%v", len(items), tx.committed)
		}
	})

	t.Run("list dispatch outbox uses default limit", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}}
		items, err := makeStore(tx).ListDispatchOutbox(ctx, testTenantUUID, "pending", 0)
		if err != nil {
			t.Fatalf("list dispatch outbox: %v", err)
		}
		if len(items) != 1 || !tx.committed {
			t.Fatalf("unexpected items=%d committed=%v", len(items), tx.committed)
		}
		if got, ok := tx.queryArgs[2].(int32); !ok || got != 20 {
			t.Fatalf("expected default limit 20, got %#v", tx.queryArgs)
		}
	})

	t.Run("list files by tenant uses default limit", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}}
		items, err := makeStore(tx).ListFiles(ctx, testTenantUUID, "", 0)
		if err != nil {
			t.Fatalf("list files by tenant: %v", err)
		}
		if len(items) != 1 || !tx.committed {
			t.Fatalf("unexpected items=%d committed=%v", len(items), tx.committed)
		}
		if got, ok := tx.queryArgs[1].(int32); !ok || got != 200 {
			t.Fatalf("expected default limit 200, got %#v", tx.queryArgs)
		}
	})

	t.Run("list files by conversation", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}}
		items, err := makeStore(tx).ListFiles(ctx, testTenantUUID, "conversation-1", 50)
		if err != nil {
			t.Fatalf("list files by conversation: %v", err)
		}
		if len(items) != 1 || !tx.committed {
			t.Fatalf("unexpected items=%d committed=%v", len(items), tx.committed)
		}
		if len(tx.queryArgs) != 2 {
			t.Fatalf("expected conversation query args, got %#v", tx.queryArgs)
		}
	})

	t.Run("get file", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{}}
		item, err := makeStore(tx).GetFile(ctx, testTenantUUID, "file-1")
		if err != nil {
			t.Fatalf("get file: %v", err)
		}
		if item.FileID != "" || !tx.committed {
			t.Fatalf("unexpected item=%+v committed=%v", item, tx.committed)
		}
	})

	t.Run("list conversation file links", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}}
		items, err := makeStore(tx).ListConversationFileLinks(ctx, testTenantUUID, "conversation-1")
		if err != nil {
			t.Fatalf("list file links: %v", err)
		}
		if len(items) != 1 || !tx.committed {
			t.Fatalf("unexpected items=%d committed=%v", len(items), tx.committed)
		}
	})
}

func TestPGStoreFailurePaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("invalid tenant uuid short circuits", func(t *testing.T) {
		store := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			t.Fatal("begin should not be called")
			return nil, nil
		}))
		if _, err := store.ListConversations(ctx, "bad", "actor-1", 1, time.Time{}, ""); err == nil {
			t.Fatal("expected tenant parse error")
		}
	})

	t.Run("invalid task uuid short circuits", func(t *testing.T) {
		store := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			t.Fatal("begin should not be called")
			return nil, nil
		}))
		if _, err := store.GetTask(ctx, testTenantUUID, "bad"); err == nil {
			t.Fatal("expected task parse error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		tx := &fakeTx{queryErr: errors.New("query failed")}
		store := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListConversationTurns(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("row error", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{err: errors.New("row failed")}}
		store := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.GetConversation(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "row failed") {
			t.Fatalf("expected row error, got %v", err)
		}
	})

	t.Run("rows scan error", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}, scanErr: errors.New("scan failed")}}
		store := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListTaskEvents(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "scan failed") {
			t.Fatalf("expected scan error, got %v", err)
		}
	})

	t.Run("rows err", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}, err: errors.New("rows failed")}}
		store := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListConversationFileLinks(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "rows failed") {
			t.Fatalf("expected rows error, got %v", err)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}, commitErr: errors.New("commit failed")}
		store := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListFiles(ctx, testTenantUUID, "", 1); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})
}

func TestPGStoreAdditionalFailurePaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	makeStore := func(tx *fakeTx) *PGStore {
		return NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
	}

	t.Run("list state transitions invalid tenant", func(t *testing.T) {
		if _, err := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			t.Fatal("begin should not be called")
			return nil, nil
		})).ListConversationStateTransitions(ctx, "bad", "conversation-1"); err == nil {
			t.Fatal("expected tenant parse error")
		}
	})

	t.Run("list state transitions query error", func(t *testing.T) {
		tx := &fakeTx{queryErr: errors.New("query failed")}
		if _, err := makeStore(tx).ListConversationStateTransitions(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("list state transitions commit error", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}, commitErr: errors.New("commit failed")}
		if _, err := makeStore(tx).ListConversationStateTransitions(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

	t.Run("count blocking tasks row error", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{err: errors.New("row failed")}}
		if _, err := makeStore(tx).CountBlockingTasks(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "row failed") {
			t.Fatalf("expected row error, got %v", err)
		}
	})

	t.Run("count blocking tasks commit error", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{vals: []any{int64(1)}}, commitErr: errors.New("commit failed")}
		if _, err := makeStore(tx).CountBlockingTasks(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

	t.Run("delete conversation exec error", func(t *testing.T) {
		tx := &fakeTx{execErr: errors.New("delete failed"), execErrAt: 2}
		if _, err := makeStore(tx).DeleteConversation(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "delete failed") {
			t.Fatalf("expected delete exec error, got %v", err)
		}
	})

	t.Run("delete conversation commit error", func(t *testing.T) {
		tx := &fakeTx{
			execTags:  []pgconn.CommandTag{pgconn.NewCommandTag("SELECT 1"), pgconn.NewCommandTag("DELETE 1")},
			commitErr: errors.New("commit failed"),
		}
		if _, err := makeStore(tx).DeleteConversation(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

	t.Run("list dispatch outbox invalid tenant", func(t *testing.T) {
		if _, err := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			t.Fatal("begin should not be called")
			return nil, nil
		})).ListDispatchOutbox(ctx, "bad", "pending", 1); err == nil {
			t.Fatal("expected tenant parse error")
		}
	})

	t.Run("list dispatch outbox query error", func(t *testing.T) {
		tx := &fakeTx{queryErr: errors.New("query failed")}
		if _, err := makeStore(tx).ListDispatchOutbox(ctx, testTenantUUID, "pending", 10); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("list dispatch outbox commit error", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}, commitErr: errors.New("commit failed")}
		if _, err := makeStore(tx).ListDispatchOutbox(ctx, testTenantUUID, "pending", 10); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

	t.Run("get file invalid tenant", func(t *testing.T) {
		if _, err := NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			t.Fatal("begin should not be called")
			return nil, nil
		})).GetFile(ctx, "bad", "file-1"); err == nil {
			t.Fatal("expected tenant parse error")
		}
	})

	t.Run("get file row error", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{err: errors.New("row failed")}}
		if _, err := makeStore(tx).GetFile(ctx, testTenantUUID, "file-1"); err == nil || !strings.Contains(err.Error(), "row failed") {
			t.Fatalf("expected row error, got %v", err)
		}
	})

	t.Run("get file commit error", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{}, commitErr: errors.New("commit failed")}
		if _, err := makeStore(tx).GetFile(ctx, testTenantUUID, "file-1"); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

	t.Run("list file links query error", func(t *testing.T) {
		tx := &fakeTx{queryErr: errors.New("query failed")}
		if _, err := makeStore(tx).ListConversationFileLinks(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("list file links commit error", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}, commitErr: errors.New("commit failed")}
		if _, err := makeStore(tx).ListConversationFileLinks(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})
}

func TestPGStoreRemainingBranchCoverage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	newStore := func(beginErr error, tx *fakeTx) *PGStore {
		return NewPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			if beginErr != nil {
				return nil, beginErr
			}
			return tx, nil
		}))
	}

	t.Run("list conversations begin error", func(t *testing.T) {
		if _, err := newStore(errors.New("begin failed"), nil).ListConversations(ctx, testTenantUUID, "actor-1", 1, time.Time{}, ""); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("list conversations query error", func(t *testing.T) {
		tx := &fakeTx{queryErr: errors.New("query failed")}
		if _, err := newStore(nil, tx).ListConversations(ctx, testTenantUUID, "actor-1", 1, time.Time{}, ""); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("list conversations commit error", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}, commitErr: errors.New("commit failed")}
		if _, err := newStore(nil, tx).ListConversations(ctx, testTenantUUID, "actor-1", 1, time.Time{}, ""); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

	t.Run("get conversation invalid tenant", func(t *testing.T) {
		if _, err := newStore(nil, nil).GetConversation(ctx, "bad", "conversation-1"); err == nil {
			t.Fatal("expected tenant parse error")
		}
	})

	t.Run("get conversation begin error", func(t *testing.T) {
		if _, err := newStore(errors.New("begin failed"), nil).GetConversation(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("get conversation commit error", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{}, commitErr: errors.New("commit failed")}
		if _, err := newStore(nil, tx).GetConversation(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

	t.Run("list conversation turns invalid tenant", func(t *testing.T) {
		if _, err := newStore(nil, nil).ListConversationTurns(ctx, "bad", "conversation-1"); err == nil {
			t.Fatal("expected tenant parse error")
		}
	})

	t.Run("list conversation turns begin error", func(t *testing.T) {
		if _, err := newStore(errors.New("begin failed"), nil).ListConversationTurns(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("list conversation turns commit error", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}, commitErr: errors.New("commit failed")}
		if _, err := newStore(nil, tx).ListConversationTurns(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

	t.Run("list state transitions begin error", func(t *testing.T) {
		if _, err := newStore(errors.New("begin failed"), nil).ListConversationStateTransitions(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("count blocking tasks invalid tenant", func(t *testing.T) {
		if _, err := newStore(nil, nil).CountBlockingTasks(ctx, "bad", "conversation-1"); err == nil {
			t.Fatal("expected tenant parse error")
		}
	})

	t.Run("count blocking tasks begin error", func(t *testing.T) {
		if _, err := newStore(errors.New("begin failed"), nil).CountBlockingTasks(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("delete conversation invalid tenant", func(t *testing.T) {
		if _, err := newStore(nil, nil).DeleteConversation(ctx, "bad", "conversation-1"); err == nil {
			t.Fatal("expected tenant parse error")
		}
	})

	t.Run("delete conversation begin error", func(t *testing.T) {
		if _, err := newStore(errors.New("begin failed"), nil).DeleteConversation(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("get task begin error", func(t *testing.T) {
		if _, err := newStore(errors.New("begin failed"), nil).GetTask(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("get task row error", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{err: errors.New("row failed")}}
		if _, err := newStore(nil, tx).GetTask(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "row failed") {
			t.Fatalf("expected row error, got %v", err)
		}
	})

	t.Run("get task commit error", func(t *testing.T) {
		tx := &fakeTx{row: &fakeRow{}, commitErr: errors.New("commit failed")}
		if _, err := newStore(nil, tx).GetTask(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

	t.Run("list task events invalid tenant", func(t *testing.T) {
		if _, err := newStore(nil, nil).ListTaskEvents(ctx, "bad", testTaskUUID); err == nil {
			t.Fatal("expected tenant parse error")
		}
	})

	t.Run("list task events invalid task", func(t *testing.T) {
		if _, err := newStore(nil, nil).ListTaskEvents(ctx, testTenantUUID, "bad"); err == nil {
			t.Fatal("expected task parse error")
		}
	})

	t.Run("list task events begin error", func(t *testing.T) {
		if _, err := newStore(errors.New("begin failed"), nil).ListTaskEvents(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("list task events query error", func(t *testing.T) {
		tx := &fakeTx{queryErr: errors.New("query failed")}
		if _, err := newStore(nil, tx).ListTaskEvents(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("list task events commit error", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}, commitErr: errors.New("commit failed")}
		if _, err := newStore(nil, tx).ListTaskEvents(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

	t.Run("list files invalid tenant", func(t *testing.T) {
		if _, err := newStore(nil, nil).ListFiles(ctx, "bad", "", 1); err == nil {
			t.Fatal("expected tenant parse error")
		}
	})

	t.Run("list files begin error", func(t *testing.T) {
		if _, err := newStore(errors.New("begin failed"), nil).ListFiles(ctx, testTenantUUID, "", 1); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("list files by conversation query error", func(t *testing.T) {
		tx := &fakeTx{queryErr: errors.New("query failed")}
		if _, err := newStore(nil, tx).ListFiles(ctx, testTenantUUID, "conversation-1", 1); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("list files by conversation commit error", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}, commitErr: errors.New("commit failed")}
		if _, err := newStore(nil, tx).ListFiles(ctx, testTenantUUID, "conversation-1", 1); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

	t.Run("list file links invalid tenant", func(t *testing.T) {
		if _, err := newStore(nil, nil).ListConversationFileLinks(ctx, "bad", "conversation-1"); err == nil {
			t.Fatal("expected tenant parse error")
		}
	})

	t.Run("list file links begin error", func(t *testing.T) {
		if _, err := newStore(errors.New("begin failed"), nil).ListConversationFileLinks(ctx, testTenantUUID, "conversation-1"); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("task write paths and helpers", func(t *testing.T) {
		now := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
		taskUUID, err := parseUUID(testTaskUUID)
		if err != nil {
			t.Fatalf("parse task uuid: %v", err)
		}
		tenantUUID, err := parseUUID(testTenantUUID)
		if err != nil {
			t.Fatalf("parse tenant uuid: %v", err)
		}

		t.Run("get task for dispatch", func(t *testing.T) {
			tx := &fakeTx{row: &fakeRow{vals: []any{tenantUUID, taskUUID}}}
			record, err := newStore(nil, tx).GetTaskForDispatch(ctx, testTenantUUID, testTaskUUID)
			if err != nil {
				t.Fatalf("get task for dispatch: %v", err)
			}
			if record.TaskID != testTaskUUID || !tx.committed {
				t.Fatalf("record=%+v committed=%v", record, tx.committed)
			}
		})

		t.Run("get task for dispatch invalid task id", func(t *testing.T) {
			if _, err := newStore(nil, nil).GetTaskForDispatch(ctx, testTenantUUID, "bad"); err == nil {
				t.Fatal("expected task parse error")
			}
		})

		t.Run("get task for dispatch begin error", func(t *testing.T) {
			if _, err := newStore(errors.New("begin failed"), nil).GetTaskForDispatch(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "begin failed") {
				t.Fatalf("expected begin error, got %v", err)
			}
		})

		t.Run("get task for dispatch row error", func(t *testing.T) {
			tx := &fakeTx{row: &fakeRow{err: errors.New("row failed")}}
			if _, err := newStore(nil, tx).GetTaskForDispatch(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "row failed") {
				t.Fatalf("expected row error, got %v", err)
			}
		})

		t.Run("get task for dispatch commit error", func(t *testing.T) {
			tx := &fakeTx{row: &fakeRow{vals: []any{tenantUUID, taskUUID}}, commitErr: errors.New("commit failed")}
			if _, err := newStore(nil, tx).GetTaskForDispatch(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "commit failed") {
				t.Fatalf("expected commit error, got %v", err)
			}
		})

		t.Run("get task actor id", func(t *testing.T) {
			tx := &fakeTx{row: &fakeRow{vals: []any{"actor-1"}}}
			actorID, err := newStore(nil, tx).GetTaskActorID(ctx, testTenantUUID, testTaskUUID)
			if err != nil {
				t.Fatalf("get task actor id: %v", err)
			}
			if actorID != "actor-1" || !tx.committed {
				t.Fatalf("actorID=%q committed=%v", actorID, tx.committed)
			}
		})

		t.Run("get task actor id invalid tenant", func(t *testing.T) {
			if _, err := newStore(nil, nil).GetTaskActorID(ctx, "bad", testTaskUUID); err == nil {
				t.Fatal("expected tenant parse error")
			}
		})

		t.Run("get task actor id invalid task", func(t *testing.T) {
			if _, err := newStore(nil, nil).GetTaskActorID(ctx, testTenantUUID, "bad"); err == nil {
				t.Fatal("expected task parse error")
			}
		})

		t.Run("get task actor id begin error", func(t *testing.T) {
			if _, err := newStore(errors.New("begin failed"), nil).GetTaskActorID(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "begin failed") {
				t.Fatalf("expected begin error, got %v", err)
			}
		})

		t.Run("get task actor id row error", func(t *testing.T) {
			tx := &fakeTx{row: &fakeRow{err: errors.New("row failed")}}
			if _, err := newStore(nil, tx).GetTaskActorID(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "row failed") {
				t.Fatalf("expected row error, got %v", err)
			}
		})

		t.Run("get task actor id commit error", func(t *testing.T) {
			tx := &fakeTx{row: &fakeRow{vals: []any{"actor-1"}}, commitErr: errors.New("commit failed")}
			if _, err := newStore(nil, tx).GetTaskActorID(ctx, testTenantUUID, testTaskUUID); err == nil || !strings.Contains(err.Error(), "commit failed") {
				t.Fatalf("expected commit error, got %v", err)
			}
		})

		t.Run("submit task inserts new record", func(t *testing.T) {
			var nilString *string
			tx := &fakeTx{
				rowQueue: []pgx.Row{
					&fakeRow{err: pgx.ErrNoRows},
					&fakeRow{vals: []any{
						tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
						"queued", "pending", int32(0), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
						int32(0), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
						"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
						nilString, nilString, nilString, nilString, nilString, nilString,
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
					}},
				},
			}
			record, existed, err := newStore(nil, tx).SubmitTask(ctx, testTenantUUID, cubeboxdomain.TaskRecord{
				TaskID:                  testTaskUUID,
				ConversationID:          "conv_1",
				TurnID:                  "turn_1",
				TaskType:                "assistant_async_plan",
				RequestID:               "req_1",
				RequestHash:             "hash",
				WorkflowID:              "wf_1",
				Status:                  "queued",
				DispatchStatus:          "pending",
				MaxAttempts:             3,
				IntentSchemaVersion:     "intent.v1",
				CompilerContractVersion: "compiler.v1",
				CapabilityMapVersion:    "cap.v1",
				SkillManifestDigest:     "skill",
				ContextHash:             "ctx",
				IntentHash:              "intent",
				PlanHash:                "plan",
				SubmittedAt:             now,
				CreatedAt:               now,
				UpdatedAt:               now,
				DispatchDeadlineAt:      timePtr(now.Add(time.Minute)),
			})
			if err != nil {
				t.Fatalf("submit task insert: %v", err)
			}
			if existed || record.TaskID != testTaskUUID || len(tx.execSQLs) < 3 || !tx.committed {
				t.Fatalf("record=%+v existed=%v execs=%v committed=%v", record, existed, tx.execSQLs, tx.committed)
			}
		})

		t.Run("submit task returns existing record", func(t *testing.T) {
			var nilString *string
			tx := &fakeTx{
				row: &fakeRow{vals: []any{
					tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
					"queued", "pending", int32(0), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
					int32(0), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
					"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
					nilString, nilString, nilString, nilString, nilString, nilString,
					pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
					pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
				}},
			}
			record, existed, err := newStore(nil, tx).SubmitTask(ctx, testTenantUUID, cubeboxdomain.TaskRecord{
				ConversationID: "conv_1",
				TurnID:         "turn_1",
				RequestID:      "req_1",
			})
			if err != nil {
				t.Fatalf("submit task existing: %v", err)
			}
			if !existed || record.TaskID != testTaskUUID || len(tx.execSQLs) != 1 || !tx.committed {
				t.Fatalf("record=%+v existed=%v execs=%v committed=%v", record, existed, tx.execSQLs, tx.committed)
			}
		})

		t.Run("submit task existing commit error", func(t *testing.T) {
			var nilString *string
			tx := &fakeTx{
				row: &fakeRow{vals: []any{
					tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
					"queued", "pending", int32(0), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
					int32(0), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
					"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
					nilString, nilString, nilString, nilString, nilString, nilString,
					pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
					pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
				}},
				commitErr: errors.New("commit failed"),
			}
			if _, existed, err := newStore(nil, tx).SubmitTask(ctx, testTenantUUID, cubeboxdomain.TaskRecord{
				ConversationID: "conv_1",
				TurnID:         "turn_1",
				RequestID:      "req_1",
			}); err == nil || !strings.Contains(err.Error(), "commit failed") || !existed {
				t.Fatalf("expected commit error on existing record, existed=%v err=%v", existed, err)
			}
		})

		t.Run("submit task insert task error", func(t *testing.T) {
			tx := &fakeTx{
				rowQueue:  []pgx.Row{&fakeRow{err: pgx.ErrNoRows}},
				execErr:   errors.New("insert task failed"),
				execErrAt: 2,
			}
			if _, _, err := newStore(nil, tx).SubmitTask(ctx, testTenantUUID, cubeboxdomain.TaskRecord{
				TaskID:                  testTaskUUID,
				ConversationID:          "conv_1",
				TurnID:                  "turn_1",
				TaskType:                "assistant_async_plan",
				RequestID:               "req_1",
				RequestHash:             "hash",
				WorkflowID:              "wf_1",
				Status:                  "queued",
				DispatchStatus:          "pending",
				MaxAttempts:             3,
				IntentSchemaVersion:     "intent.v1",
				CompilerContractVersion: "compiler.v1",
				CapabilityMapVersion:    "cap.v1",
				SkillManifestDigest:     "skill",
				ContextHash:             "ctx",
				IntentHash:              "intent",
				PlanHash:                "plan",
				SubmittedAt:             now,
				CreatedAt:               now,
				UpdatedAt:               now,
			}); err == nil || !strings.Contains(err.Error(), "insert task failed") {
				t.Fatalf("expected insert task error, got %v", err)
			}
		})

		t.Run("submit task event error", func(t *testing.T) {
			var nilString *string
			tx := &fakeTx{
				rowQueue: []pgx.Row{
					&fakeRow{err: pgx.ErrNoRows},
					&fakeRow{vals: []any{
						tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
						"queued", "pending", int32(0), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
						int32(0), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
						"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
						nilString, nilString, nilString, nilString, nilString, nilString,
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
					}},
				},
				execErr:   errors.New("insert event failed"),
				execErrAt: 3,
			}
			if _, _, err := newStore(nil, tx).SubmitTask(ctx, testTenantUUID, cubeboxdomain.TaskRecord{
				TaskID:                  testTaskUUID,
				ConversationID:          "conv_1",
				TurnID:                  "turn_1",
				TaskType:                "assistant_async_plan",
				RequestID:               "req_1",
				RequestHash:             "hash",
				WorkflowID:              "wf_1",
				Status:                  "queued",
				DispatchStatus:          "pending",
				MaxAttempts:             3,
				IntentSchemaVersion:     "intent.v1",
				CompilerContractVersion: "compiler.v1",
				CapabilityMapVersion:    "cap.v1",
				SkillManifestDigest:     "skill",
				ContextHash:             "ctx",
				IntentHash:              "intent",
				PlanHash:                "plan",
				SubmittedAt:             now,
				CreatedAt:               now,
				UpdatedAt:               now,
			}); err == nil || !strings.Contains(err.Error(), "insert event failed") {
				t.Fatalf("expected insert event error, got %v", err)
			}
		})

		t.Run("submit task outbox error", func(t *testing.T) {
			var nilString *string
			tx := &fakeTx{
				rowQueue: []pgx.Row{
					&fakeRow{err: pgx.ErrNoRows},
					&fakeRow{vals: []any{
						tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
						"queued", "pending", int32(0), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
						int32(0), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
						"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
						nilString, nilString, nilString, nilString, nilString, nilString,
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
					}},
				},
				execErr:   errors.New("upsert outbox failed"),
				execErrAt: 3,
			}
			if _, _, err := newStore(nil, tx).SubmitTask(ctx, testTenantUUID, cubeboxdomain.TaskRecord{
				TaskID:                  testTaskUUID,
				ConversationID:          "conv_1",
				TurnID:                  "turn_1",
				TaskType:                "assistant_async_plan",
				RequestID:               "req_1",
				RequestHash:             "hash",
				WorkflowID:              "wf_1",
				Status:                  "queued",
				DispatchStatus:          "pending",
				MaxAttempts:             3,
				IntentSchemaVersion:     "intent.v1",
				CompilerContractVersion: "compiler.v1",
				CapabilityMapVersion:    "cap.v1",
				SkillManifestDigest:     "skill",
				ContextHash:             "ctx",
				IntentHash:              "intent",
				PlanHash:                "plan",
				SubmittedAt:             now,
				CreatedAt:               now,
				UpdatedAt:               now,
			}); err == nil || !strings.Contains(err.Error(), "upsert outbox failed") {
				t.Fatalf("expected upsert outbox error, got %v", err)
			}
		})

		t.Run("cancel task transitions running task", func(t *testing.T) {
			var nilString *string
			tx := &fakeTx{
				rowQueue: []pgx.Row{
					&fakeRow{vals: []any{
						tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
						"running", "pending", int32(1), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
						int32(1), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
						"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
						nilString, nilString, nilString, nilString, nilString, nilString,
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
					}},
					&fakeRow{vals: []any{
						tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
						"canceled", "failed", int32(1), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
						int32(1), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
						"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
						nilString, nilString, nilString, nilString, nilString, nilString,
						pgtype.Timestamptz{Time: now, Valid: true},
						pgtype.Timestamptz{Time: now, Valid: true},
						pgtype.Timestamptz{Time: now, Valid: true},
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
					}},
				},
				execTags: []pgconn.CommandTag{pgconn.NewCommandTag("SELECT 0"), pgconn.NewCommandTag("UPDATE 1"), pgconn.NewCommandTag("INSERT 1"), pgconn.NewCommandTag("INSERT 1"), pgconn.NewCommandTag("UPDATE 1")},
			}
			record, accepted, err := newStore(nil, tx).CancelTask(ctx, testTenantUUID, testTaskUUID, now)
			if err != nil {
				t.Fatalf("cancel task: %v", err)
			}
			if !accepted || record.Status != "canceled" || len(tx.execSQLs) < 4 || !tx.committed {
				t.Fatalf("record=%+v accepted=%v execs=%v committed=%v", record, accepted, tx.execSQLs, tx.committed)
			}
		})

		t.Run("cancel task returns terminal record without update", func(t *testing.T) {
			var nilString *string
			tx := &fakeTx{
				row: &fakeRow{vals: []any{
					tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
					"succeeded", "started", int32(1), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
					int32(1), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
					"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
					nilString, nilString, nilString, nilString, nilString, nilString,
					pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
					pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
				}},
			}
			record, accepted, err := newStore(nil, tx).CancelTask(ctx, testTenantUUID, testTaskUUID, now)
			if err != nil {
				t.Fatalf("cancel terminal task: %v", err)
			}
			if accepted || record.Status != "succeeded" || len(tx.execSQLs) != 1 || !tx.committed {
				t.Fatalf("record=%+v accepted=%v execs=%v committed=%v", record, accepted, tx.execSQLs, tx.committed)
			}
		})

		t.Run("cancel task update error", func(t *testing.T) {
			var nilString *string
			tx := &fakeTx{
				rowQueue: []pgx.Row{
					&fakeRow{vals: []any{
						tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
						"running", "pending", int32(1), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
						int32(1), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
						"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
						nilString, nilString, nilString, nilString, nilString, nilString,
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
					}},
				},
				row: &fakeRow{err: errors.New("update failed")},
			}
			if _, _, err := newStore(nil, tx).CancelTask(ctx, testTenantUUID, testTaskUUID, now); err == nil || !strings.Contains(err.Error(), "update failed") {
				t.Fatalf("expected update error, got %v", err)
			}
		})

		t.Run("cancel task event error", func(t *testing.T) {
			var nilString *string
			tx := &fakeTx{
				rowQueue: []pgx.Row{
					&fakeRow{vals: []any{
						tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
						"running", "pending", int32(1), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
						int32(1), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
						"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
						nilString, nilString, nilString, nilString, nilString, nilString,
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
					}},
					&fakeRow{vals: []any{
						tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
						"canceled", "failed", int32(1), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
						int32(1), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
						"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
						nilString, nilString, nilString, nilString, nilString, nilString,
						pgtype.Timestamptz{Time: now, Valid: true},
						pgtype.Timestamptz{Time: now, Valid: true},
						pgtype.Timestamptz{Time: now, Valid: true},
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
					}},
				},
				execErr:   errors.New("insert event failed"),
				execErrAt: 3,
			}
			if _, _, err := newStore(nil, tx).CancelTask(ctx, testTenantUUID, testTaskUUID, now); err == nil || !strings.Contains(err.Error(), "insert event failed") {
				t.Fatalf("expected insert event error, got %v", err)
			}
		})

		t.Run("cancel task outbox error", func(t *testing.T) {
			var nilString *string
			tx := &fakeTx{
				rowQueue: []pgx.Row{
					&fakeRow{vals: []any{
						tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
						"running", "pending", int32(1), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
						int32(1), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
						"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
						nilString, nilString, nilString, nilString, nilString, nilString,
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
					}},
					&fakeRow{vals: []any{
						tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
						"canceled", "failed", int32(1), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
						int32(1), int32(3), nilString, nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
						"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
						nilString, nilString, nilString, nilString, nilString, nilString,
						pgtype.Timestamptz{Time: now, Valid: true},
						pgtype.Timestamptz{Time: now, Valid: true},
						pgtype.Timestamptz{Time: now, Valid: true},
						pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
					}},
				},
				execErr:   errors.New("mark outbox failed"),
				execErrAt: 4,
			}
			if _, _, err := newStore(nil, tx).CancelTask(ctx, testTenantUUID, testTaskUUID, now); err == nil || !strings.Contains(err.Error(), "mark outbox failed") {
				t.Fatalf("expected mark outbox error, got %v", err)
			}
		})

		t.Run("update task state, event, outbox, helpers", func(t *testing.T) {
			var nilString *string
			txState := &fakeTx{row: &fakeRow{vals: []any{
				tenantUUID, taskUUID, "conv_1", "turn_1", "assistant_async_plan", "req_1", "hash", "wf_1",
				"running", "started", int32(2), pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
				int32(2), int32(3), stringPtr("boom"), nilString, "intent.v1", "compiler.v1", "cap.v1", "skill",
				"ctx", "intent", "plan", nilString, nilString, nilString, nilString, nilString,
				nilString, nilString, nilString, nilString, nilString, nilString,
				pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
				pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
			}}}
			record, err := newStore(nil, txState).UpdateTaskState(ctx, testTenantUUID, cubeboxdomain.TaskStateUpdate{
				TaskID:          testTaskUUID,
				Status:          "running",
				DispatchStatus:  "started",
				DispatchAttempt: 2,
				Attempt:         2,
				LastErrorCode:   "boom",
				UpdatedAt:       now,
			})
			if err != nil || record.LastErrorCode != "boom" || !txState.committed {
				t.Fatalf("record=%+v err=%v committed=%v", record, err, txState.committed)
			}

			txEvent := &fakeTx{}
			if err := newStore(nil, txEvent).InsertTaskEvent(ctx, testTenantUUID, cubeboxdomain.TaskEventRecord{
				TaskID:     testTaskUUID,
				FromStatus: "queued",
				ToStatus:   "running",
				EventType:  "running",
				ErrorCode:  "boom",
				OccurredAt: now,
			}); err != nil || len(txEvent.execSQLs) != 2 || !txEvent.committed {
				t.Fatalf("insert task event err=%v execs=%v committed=%v", err, txEvent.execSQLs, txEvent.committed)
			}

			txOutbox := &fakeTx{execTags: []pgconn.CommandTag{pgconn.NewCommandTag("SELECT 0"), pgconn.NewCommandTag("UPDATE 1")}}
			if err := newStore(nil, txOutbox).UpdateTaskDispatchOutbox(ctx, testTenantUUID, cubeboxdomain.TaskDispatchOutboxUpdate{
				TaskID:      testTaskUUID,
				Status:      "started",
				Attempt:     2,
				NextRetryAt: now.Add(time.Minute),
				UpdatedAt:   now,
			}); err != nil || len(txOutbox.execSQLs) != 2 || !txOutbox.committed {
				t.Fatalf("update outbox err=%v execs=%v committed=%v", err, txOutbox.execSQLs, txOutbox.committed)
			}

			if got := timestamptzPtr(nil); got.Valid {
				t.Fatalf("expected invalid nil timestamptz, got %+v", got)
			}
			if got := timestamptzValue(now); !got.Valid || !got.Time.Equal(now) {
				t.Fatalf("unexpected timestamptz value %+v", got)
			}
			if mustParseUUID(testTaskUUID).Bytes != taskUUID.Bytes {
				t.Fatal("mustParseUUID mismatch")
			}
			if ptr := stringPtr("abc"); ptr == nil || *ptr != "abc" {
				t.Fatalf("unexpected string ptr %v", ptr)
			}
			if ptr := timePtr(now); ptr == nil || !ptr.Equal(now) {
				t.Fatalf("unexpected time ptr %v", ptr)
			}
			if !isUniqueViolation(&pgconn.PgError{Code: "23505"}) || isUniqueViolation(errors.New("nope")) {
				t.Fatal("unexpected unique violation detection")
			}
		})

		t.Run("update task state error paths", func(t *testing.T) {
			if _, err := newStore(nil, nil).UpdateTaskState(ctx, "bad", cubeboxdomain.TaskStateUpdate{}); err == nil {
				t.Fatal("expected tenant parse error")
			}
			if _, err := newStore(nil, nil).UpdateTaskState(ctx, testTenantUUID, cubeboxdomain.TaskStateUpdate{TaskID: "bad"}); err == nil {
				t.Fatal("expected task parse error")
			}
			if _, err := newStore(errors.New("begin failed"), nil).UpdateTaskState(ctx, testTenantUUID, cubeboxdomain.TaskStateUpdate{TaskID: testTaskUUID}); err == nil || !strings.Contains(err.Error(), "begin failed") {
				t.Fatalf("expected begin error, got %v", err)
			}
			tx := &fakeTx{row: &fakeRow{err: errors.New("update failed")}}
			if _, err := newStore(nil, tx).UpdateTaskState(ctx, testTenantUUID, cubeboxdomain.TaskStateUpdate{TaskID: testTaskUUID, UpdatedAt: now}); err == nil || !strings.Contains(err.Error(), "update failed") {
				t.Fatalf("expected update error, got %v", err)
			}
			tx = &fakeTx{row: &fakeRow{vals: []any{tenantUUID, taskUUID}}, commitErr: errors.New("commit failed")}
			if _, err := newStore(nil, tx).UpdateTaskState(ctx, testTenantUUID, cubeboxdomain.TaskStateUpdate{TaskID: testTaskUUID, UpdatedAt: now}); err == nil || !strings.Contains(err.Error(), "commit failed") {
				t.Fatalf("expected commit error, got %v", err)
			}
		})

		t.Run("insert task event error paths", func(t *testing.T) {
			if err := newStore(nil, nil).InsertTaskEvent(ctx, "bad", cubeboxdomain.TaskEventRecord{}); err == nil {
				t.Fatal("expected tenant parse error")
			}
			if err := newStore(nil, nil).InsertTaskEvent(ctx, testTenantUUID, cubeboxdomain.TaskEventRecord{TaskID: "bad"}); err == nil {
				t.Fatal("expected task parse error")
			}
			if err := newStore(errors.New("begin failed"), nil).InsertTaskEvent(ctx, testTenantUUID, cubeboxdomain.TaskEventRecord{TaskID: testTaskUUID}); err == nil || !strings.Contains(err.Error(), "begin failed") {
				t.Fatalf("expected begin error, got %v", err)
			}
			tx := &fakeTx{execErr: errors.New("exec failed"), execErrAt: 2}
			if err := newStore(nil, tx).InsertTaskEvent(ctx, testTenantUUID, cubeboxdomain.TaskEventRecord{TaskID: testTaskUUID, ToStatus: "running", EventType: "running", OccurredAt: now}); err == nil || !strings.Contains(err.Error(), "exec failed") {
				t.Fatalf("expected exec error, got %v", err)
			}
			tx = &fakeTx{commitErr: errors.New("commit failed")}
			if err := newStore(nil, tx).InsertTaskEvent(ctx, testTenantUUID, cubeboxdomain.TaskEventRecord{TaskID: testTaskUUID, ToStatus: "running", EventType: "running", OccurredAt: now}); err == nil || !strings.Contains(err.Error(), "commit failed") {
				t.Fatalf("expected commit error, got %v", err)
			}
		})

		t.Run("update task dispatch outbox error paths", func(t *testing.T) {
			update := cubeboxdomain.TaskDispatchOutboxUpdate{TaskID: testTaskUUID, Status: "started", NextRetryAt: now, UpdatedAt: now}
			if err := newStore(nil, nil).UpdateTaskDispatchOutbox(ctx, "bad", update); err == nil {
				t.Fatal("expected tenant parse error")
			}
			update.TaskID = "bad"
			if err := newStore(nil, nil).UpdateTaskDispatchOutbox(ctx, testTenantUUID, update); err == nil {
				t.Fatal("expected task parse error")
			}
			update.TaskID = testTaskUUID
			if err := newStore(errors.New("begin failed"), nil).UpdateTaskDispatchOutbox(ctx, testTenantUUID, update); err == nil || !strings.Contains(err.Error(), "begin failed") {
				t.Fatalf("expected begin error, got %v", err)
			}
			tx := &fakeTx{execErr: errors.New("exec failed"), execErrAt: 2}
			if err := newStore(nil, tx).UpdateTaskDispatchOutbox(ctx, testTenantUUID, update); err == nil || !strings.Contains(err.Error(), "exec failed") {
				t.Fatalf("expected exec error, got %v", err)
			}
			tx = &fakeTx{commitErr: errors.New("commit failed")}
			if err := newStore(nil, tx).UpdateTaskDispatchOutbox(ctx, testTenantUUID, update); err == nil || !strings.Contains(err.Error(), "commit failed") {
				t.Fatalf("expected commit error, got %v", err)
			}
		})

		t.Run("snapshot and json helpers", func(t *testing.T) {
			tx := &fakeTx{execErr: &pgconn.PgError{Code: "23505"}, execErrAt: 1}
			if err := syncTransitionSnapshot(ctx, tx, tenantUUID, "conv_1", cubeboxdomain.StateTransition{RequestID: "req", TraceID: "trace"}); err != nil {
				t.Fatalf("expected duplicate transition to be ignored, got %v", err)
			}
			tx = &fakeTx{execErr: errors.New("insert failed"), execErrAt: 1}
			if err := syncTransitionSnapshot(ctx, tx, tenantUUID, "conv_1", cubeboxdomain.StateTransition{RequestID: "req", TraceID: "trace"}); err == nil || !strings.Contains(err.Error(), "insert failed") {
				t.Fatalf("expected insert failed, got %v", err)
			}

			if raw, err := marshalJSON(nil, true); err != nil || string(raw) != "{}" {
				t.Fatalf("marshalJSON object default raw=%s err=%v", raw, err)
			}
			if raw, err := marshalJSON(nil, false); err != nil || string(raw) != "[]" {
				t.Fatalf("marshalJSON array default raw=%s err=%v", raw, err)
			}
			if raw, err := marshalJSON([]string{"a"}, false); err != nil || string(raw) != `["a"]` {
				t.Fatalf("marshalJSON raw=%s err=%v", raw, err)
			}
			if _, err := marshalJSON(func() {}, true); err == nil {
				t.Fatal("expected marshalJSON error")
			}

			if raw, err := marshalNullableJSON(nil); err != nil || raw != nil {
				t.Fatalf("marshalNullableJSON nil raw=%v err=%v", raw, err)
			}
			if raw, err := marshalNullableJSON(map[string]any{"a": 1}); err != nil || string(raw) != `{"a":1}` {
				t.Fatalf("marshalNullableJSON raw=%s err=%v", raw, err)
			}
			if _, err := marshalNullableJSON(func() {}); err == nil {
				t.Fatal("expected marshalNullableJSON error")
			}
		})
	})
}
