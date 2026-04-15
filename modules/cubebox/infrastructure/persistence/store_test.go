package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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
		*d = val.(string)
		return nil
	case **string:
		if val == nil {
			*d = nil
			return nil
		}
		s := val.(string)
		*d = &s
		return nil
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
		items, err := makeStore(tx).ListConversations(ctx, testTenantUUID, "actor-1", 0)
		if err != nil {
			t.Fatalf("list conversations: %v", err)
		}
		if len(items) != 1 || !tx.committed {
			t.Fatalf("unexpected items=%d committed=%v", len(items), tx.committed)
		}
		if got, ok := tx.queryArgs[2].(int32); !ok || got != 20 {
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
		if _, err := store.ListConversations(ctx, "bad", "actor-1", 1); err == nil {
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
		if _, err := newStore(errors.New("begin failed"), nil).ListConversations(ctx, testTenantUUID, "actor-1", 1); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("list conversations query error", func(t *testing.T) {
		tx := &fakeTx{queryErr: errors.New("query failed")}
		if _, err := newStore(nil, tx).ListConversations(ctx, testTenantUUID, "actor-1", 1); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("list conversations commit error", func(t *testing.T) {
		tx := &fakeTx{rows: &fakeRows{records: [][]any{{}}}, commitErr: errors.New("commit failed")}
		if _, err := newStore(nil, tx).ListConversations(ctx, testTenantUUID, "actor-1", 1); err == nil || !strings.Contains(err.Error(), "commit failed") {
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
}
