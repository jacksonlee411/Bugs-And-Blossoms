package cubebox

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type beginFunc func(ctx context.Context) (pgx.Tx, error)

func (f beginFunc) Begin(ctx context.Context) (pgx.Tx, error) { return f(ctx) }

type txStub struct {
	execSQLs  []string
	execArgs  [][]any
	execErr   error
	queryErr  error
	rowQueue  []pgx.Row
	rowsQueue []pgx.Rows
}

func (t *txStub) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *txStub) Commit(context.Context) error          { return nil }
func (t *txStub) Rollback(context.Context) error        { return nil }
func (t *txStub) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *txStub) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return fakeBatchResults{} }
func (t *txStub) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *txStub) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *txStub) Conn() *pgx.Conn { return nil }

func (t *txStub) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.execSQLs = append(t.execSQLs, sql)
	t.execArgs = append(t.execArgs, args)
	return pgconn.CommandTag{}, t.execErr
}

func (t *txStub) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if t.queryErr != nil {
		return nil, t.queryErr
	}
	if len(t.rowsQueue) == 0 {
		return &rowsStub{}, nil
	}
	next := t.rowsQueue[0]
	t.rowsQueue = t.rowsQueue[1:]
	return next, nil
}

func (t *txStub) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if len(t.rowQueue) == 0 {
		return rowStub{err: errors.New("unexpected QueryRow")}
	}
	next := t.rowQueue[0]
	t.rowQueue = t.rowQueue[1:]
	return next
}

type rowStub struct {
	vals []any
	err  error
}

func (r rowStub) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		if i >= len(r.vals) {
			break
		}
		switch d := dest[i].(type) {
		case *pgtype.UUID:
			*d = r.vals[i].(pgtype.UUID)
		case *string:
			*d = r.vals[i].(string)
		case *bool:
			*d = r.vals[i].(bool)
		case *pgtype.Timestamptz:
			*d = r.vals[i].(pgtype.Timestamptz)
		case **string:
			*d = r.vals[i].(*string)
		case *[]byte:
			*d = r.vals[i].([]byte)
		case *int32:
			*d = r.vals[i].(int32)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

type rowsStub struct {
	rows [][]any
	idx  int
	err  error
}

func (r *rowsStub) Close()                                       {}
func (r *rowsStub) Err() error                                   { return r.err }
func (r *rowsStub) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *rowsStub) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *rowsStub) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *rowsStub) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	row := r.rows[r.idx-1]
	return rowStub{vals: row}.Scan(dest...)
}
func (r *rowsStub) Values() ([]any, error) { return nil, nil }
func (r *rowsStub) RawValues() [][]byte    { return nil }
func (r *rowsStub) Conn() *pgx.Conn        { return nil }

type fakeBatchResults struct{}

func (fakeBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (fakeBatchResults) Query() (pgx.Rows, error)         { return nil, nil }
func (fakeBatchResults) QueryRow() pgx.Row                { return rowStub{} }
func (fakeBatchResults) Close() error                     { return nil }

func TestStoreGetConversationSetsTenantAndFailsClosedOnTenantIsolation(t *testing.T) {
	tenantID := uuid.NewString()
	principalID := uuid.NewString()
	tx := &txStub{
		rowQueue: []pgx.Row{
			rowStub{err: pgx.ErrNoRows},
		},
	}
	store := NewStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return tx, nil
	}))

	_, err := store.GetConversation(context.Background(), tenantID, principalID, "conv_1")
	if !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("expected ErrConversationNotFound, got %v", err)
	}
	if len(tx.execSQLs) == 0 || !strings.Contains(tx.execSQLs[0], "set_config('app.current_tenant', $1, true)") {
		t.Fatalf("expected tenant set_config before read, got %#v", tx.execSQLs)
	}
	if got := tx.execArgs[0][0]; got != tenantID {
		t.Fatalf("expected tenant arg %s, got %#v", tenantID, got)
	}
}

func TestStoreAppendEventFailsClosedWhenConversationLookupIsTenantScopedAway(t *testing.T) {
	tenantID := uuid.NewString()
	principalID := uuid.NewString()
	tx := &txStub{
		rowQueue: []pgx.Row{
			rowStub{err: pgx.ErrNoRows},
		},
	}
	store := NewStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return tx, nil
	}))

	err := store.AppendEvent(context.Background(), tenantID, principalID, "conv_1", CanonicalEvent{
		EventID:        "evt_1",
		ConversationID: "conv_1",
		Sequence:       1,
		Type:           "turn.started",
		Payload:        map[string]any{"user_message_id": "msg_1"},
	})
	if !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("expected ErrConversationNotFound, got %v", err)
	}
	if len(tx.execSQLs) == 0 || !strings.Contains(tx.execSQLs[0], "set_config('app.current_tenant', $1, true)") {
		t.Fatalf("expected tenant set_config before append, got %#v", tx.execSQLs)
	}
}

func TestStoreCompactConversationReusesTenantScopedReadAndWrite(t *testing.T) {
	tenantID := uuid.NewString()
	principalID := uuid.NewString()
	now := timestamptz(time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC))
	payload := []byte(`{"title":"新对话","status":"active","archived":false}`)
	tx := &txStub{
		rowQueue: []pgx.Row{
			rowStub{vals: []any{
				uuidToPGType(tenantID),
				"conv_1",
				uuidToPGType(principalID),
				"新对话",
				"active",
				false,
				now,
				now,
				nullTimestamptz(),
			}},
			rowStub{vals: []any{
				uuidToPGType(tenantID),
				"conv_1",
				"evt_compact",
				int32(9),
				(*string)(nil),
				"turn.context_compacted",
				[]byte(`{"summary_id":"summary_1","source_range":[1,4]}`),
				now,
			}},
		},
		rowsQueue: []pgx.Rows{
			&rowsStub{
				rows: [][]any{
					{
						uuidToPGType(tenantID),
						"conv_1",
						"evt_1",
						int32(1),
						(*string)(nil),
						"conversation.loaded",
						payload,
						now,
					},
					{
						uuidToPGType(tenantID),
						"conv_1",
						"evt_2",
						int32(2),
						(*string)(nil),
						"turn.user_message.accepted",
						[]byte(`{"message_id":"msg_1","text":"请总结最近进度"}`),
						now,
					},
					{
						uuidToPGType(tenantID),
						"conv_1",
						"evt_3",
						int32(3),
						ptr("turn_1"),
						"turn.agent_message.delta",
						[]byte(`{"message_id":"msg_agent_1","delta":"当前已完成 Phase B。"}`),
						now,
					},
					{
						uuidToPGType(tenantID),
						"conv_1",
						"evt_4",
						int32(4),
						ptr("turn_1"),
						"turn.agent_message.completed",
						[]byte(`{"message_id":"msg_agent_1"}`),
						now,
					},
					{
						uuidToPGType(tenantID),
						"conv_1",
						"evt_5",
						int32(5),
						ptr("turn_2"),
						"turn.user_message.accepted",
						[]byte(`{"message_id":"msg_2","text":"继续下一步"}`),
						now,
					},
					{
						uuidToPGType(tenantID),
						"conv_1",
						"evt_6",
						int32(6),
						ptr("turn_2"),
						"turn.agent_message.delta",
						[]byte(`{"message_id":"msg_agent_2","delta":"接下来处理 Phase C。"}`),
						now,
					},
					{
						uuidToPGType(tenantID),
						"conv_1",
						"evt_7",
						int32(7),
						ptr("turn_2"),
						"turn.agent_message.completed",
						[]byte(`{"message_id":"msg_agent_2"}`),
						now,
					},
					{
						uuidToPGType(tenantID),
						"conv_1",
						"evt_8",
						int32(8),
						ptr("turn_3"),
						"turn.user_message.accepted",
						[]byte(`{"message_id":"msg_3","text":"最后确认 compaction"}`),
						now,
					},
				},
			},
		},
	}
	store := NewStore(beginFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))

	response, err := store.CompactConversation(context.Background(), tenantID, principalID, "conv_1", CanonicalContext{
		TenantID:    tenantID,
		PrincipalID: principalID,
		Language:    "zh",
		Page:        "/app/cubebox",
	}, "manual")
	if err != nil {
		t.Fatalf("expected compact success, got %v", err)
	}
	if response.Event == nil || response.Event.Type != "turn.context_compacted" {
		t.Fatalf("unexpected compact event=%#v", response.Event)
	}
	if len(tx.execSQLs) == 0 {
		t.Fatalf("expected tenant-scoped tx execs, got %#v", tx.execSQLs)
	}
	if got := tx.execArgs[0][0]; got != tenantID {
		t.Fatalf("expected tenant arg %s, got %#v", tenantID, got)
	}
	if response.NextSequence != 10 {
		t.Fatalf("expected next sequence 10, got %d", response.NextSequence)
	}
}

func TestStoreCompactConversationSkipsNoOpCompactionEvent(t *testing.T) {
	tenantID := uuid.NewString()
	principalID := uuid.NewString()
	now := timestamptz(time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC))
	payload := []byte(`{"title":"新对话","status":"active","archived":false}`)
	tx := &txStub{
		rowQueue: []pgx.Row{
			rowStub{vals: []any{
				uuidToPGType(tenantID),
				"conv_short",
				uuidToPGType(principalID),
				"新对话",
				"active",
				false,
				now,
				now,
				nullTimestamptz(),
			}},
		},
		rowsQueue: []pgx.Rows{
			&rowsStub{
				rows: [][]any{
					{
						uuidToPGType(tenantID),
						"conv_short",
						"evt_1",
						int32(1),
						(*string)(nil),
						"conversation.loaded",
						payload,
						now,
					},
					{
						uuidToPGType(tenantID),
						"conv_short",
						"evt_2",
						int32(2),
						ptr("turn_1"),
						"turn.user_message.accepted",
						[]byte(`{"message_id":"msg_1","text":"最近一条消息"}`),
						now,
					},
				},
			},
		},
	}
	store := NewStore(beginFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))

	response, err := store.CompactConversation(context.Background(), tenantID, principalID, "conv_short", CanonicalContext{
		TenantID:    tenantID,
		PrincipalID: principalID,
		Language:    "zh",
		Page:        "/app/cubebox",
	}, "manual")
	if err != nil {
		t.Fatalf("expected compact success, got %v", err)
	}
	if response.Event != nil {
		t.Fatalf("expected no event for no-op compaction, got %#v", response.Event)
	}
	if response.NextSequence != 3 {
		t.Fatalf("expected next sequence to remain 3, got %d", response.NextSequence)
	}
	if len(tx.rowQueue) != 0 {
		t.Fatalf("expected no append query row consumption, remaining=%d", len(tx.rowQueue))
	}
	for _, sql := range tx.execSQLs {
		if strings.Contains(sql, "UPDATE iam.cubebox_conversations") {
			t.Fatalf("did not expect conversation updated_at write on no-op compaction, exec=%#v", tx.execSQLs)
		}
	}
}

func TestStoreGetConversationReturnsPhaseCLifecycleRoundtripGolden(t *testing.T) {
	tenantID := uuid.NewString()
	principalID := uuid.NewString()
	now := timestamptz(time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC))
	tx := &txStub{
		rowQueue: []pgx.Row{
			rowStub{vals: []any{
				uuidToPGType(tenantID),
				"conv_roundtrip",
				uuidToPGType(principalID),
				"恢复后的活跃会话",
				"active",
				false,
				now,
				now,
				nullTimestamptz(),
			}},
		},
		rowsQueue: []pgx.Rows{
			&rowsStub{
				rows: [][]any{
					{uuidToPGType(tenantID), "conv_roundtrip", "evt_1", int32(1), (*string)(nil), "conversation.loaded", []byte(`{"title":"新对话","status":"active","archived":false}`), now},
					{uuidToPGType(tenantID), "conv_roundtrip", "evt_2", int32(2), (*string)(nil), "conversation.renamed", []byte(`{"title":"需求澄清","status":"active","archived":false}`), now},
					{uuidToPGType(tenantID), "conv_roundtrip", "evt_3", int32(3), ptr("turn_1"), "turn.user_message.accepted", []byte(`{"message_id":"msg_user_1","text":"请总结当前状态"}`), now},
					{uuidToPGType(tenantID), "conv_roundtrip", "evt_4", int32(4), ptr("turn_1"), "turn.agent_message.delta", []byte(`{"message_id":"msg_agent_1","delta":"当前已完成持久化，"}`), now},
					{uuidToPGType(tenantID), "conv_roundtrip", "evt_5", int32(5), ptr("turn_1"), "turn.agent_message.delta", []byte(`{"message_id":"msg_agent_1","delta":"正在进入封板收口。"}`), now},
					{uuidToPGType(tenantID), "conv_roundtrip", "evt_6", int32(6), ptr("turn_1"), "turn.agent_message.completed", []byte(`{"message_id":"msg_agent_1"}`), now},
					{uuidToPGType(tenantID), "conv_roundtrip", "evt_7", int32(7), (*string)(nil), "conversation.unarchived", []byte(`{"title":"恢复后的活跃会话","status":"active","archived":false}`), now},
				},
			},
		},
	}
	store := NewStore(beginFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))

	response, err := store.GetConversation(context.Background(), tenantID, principalID, "conv_roundtrip")
	if err != nil {
		t.Fatalf("expected get success, got %v", err)
	}
	if response.Conversation.Title != "恢复后的活跃会话" || response.Conversation.Status != "active" || response.Conversation.Archived {
		t.Fatalf("unexpected conversation=%+v", response.Conversation)
	}
	if response.NextSequence != 8 {
		t.Fatalf("expected next sequence 8, got %d", response.NextSequence)
	}
	if len(response.Events) != 7 {
		t.Fatalf("expected 7 events, got %d", len(response.Events))
	}
	if response.Events[6].Type != "conversation.unarchived" {
		t.Fatalf("unexpected terminal event=%+v", response.Events[6])
	}
}

func TestStoreGetActiveModelRuntimeConfig(t *testing.T) {
	tenantID := uuid.NewString()
	principalID := uuid.NewString()
	now := timestamptz(time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC))
	tx := &txStub{
		rowQueue: []pgx.Row{
			rowStub{vals: []any{
				uuidToPGType(tenantID),
				"active",
				"provider_1",
				"gpt-4.1",
				[]byte(`{"streaming":true}`),
				uuidToPGType(principalID),
				now,
				now,
			}},
			rowStub{vals: []any{
				uuidToPGType(tenantID),
				"provider_1",
				"openai-compatible",
				"Primary",
				"https://example.invalid/v1",
				true,
				uuidToPGType(principalID),
				uuidToPGType(principalID),
				now,
				now,
				nullTimestamptz(),
			}},
		},
		rowsQueue: []pgx.Rows{
			&rowsStub{
				rows: [][]any{
					{
						uuidToPGType(tenantID),
						"cred_1",
						"provider_1",
						"env://OPENAI_API_KEY",
						"sk-****",
						int32(2),
						true,
						uuidToPGType(principalID),
						now,
						nullTimestamptz(),
					},
				},
			},
		},
	}
	store := NewStore(beginFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))

	config, err := store.GetActiveModelRuntimeConfig(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("get active model runtime config: %v", err)
	}
	if config.Selection.ProviderID != "provider_1" || config.Selection.ModelSlug != "gpt-4.1" {
		t.Fatalf("unexpected selection=%+v", config.Selection)
	}
	if config.Provider.BaseURL != "https://example.invalid/v1" || !config.Provider.Enabled {
		t.Fatalf("unexpected provider=%+v", config.Provider)
	}
	if config.Credential.SecretRef != "env://OPENAI_API_KEY" || !config.Credential.Active {
		t.Fatalf("unexpected credential=%+v", config.Credential)
	}
}

func TestStoreGetActiveModelRuntimeConfigFailsClosedWhenCredentialMissing(t *testing.T) {
	tenantID := uuid.NewString()
	principalID := uuid.NewString()
	now := timestamptz(time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC))
	tx := &txStub{
		rowQueue: []pgx.Row{
			rowStub{vals: []any{
				uuidToPGType(tenantID),
				"active",
				"provider_1",
				"gpt-4.1",
				[]byte(`{"streaming":true}`),
				uuidToPGType(principalID),
				now,
				now,
			}},
			rowStub{vals: []any{
				uuidToPGType(tenantID),
				"provider_1",
				"openai-compatible",
				"Primary",
				"https://example.invalid/v1",
				true,
				uuidToPGType(principalID),
				uuidToPGType(principalID),
				now,
				now,
				nullTimestamptz(),
			}},
		},
		rowsQueue: []pgx.Rows{
			&rowsStub{},
		},
	}
	store := NewStore(beginFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))

	_, err := store.GetActiveModelRuntimeConfig(context.Background(), tenantID)
	if !errors.Is(err, ErrModelCredentialNotFound) {
		t.Fatalf("expected ErrModelCredentialNotFound, got %v", err)
	}
}

func TestStoreGetActiveModelRuntimeConfigFailsClosedWhenCredentialInactive(t *testing.T) {
	tenantID := uuid.NewString()
	principalID := uuid.NewString()
	now := timestamptz(time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC))
	tx := &txStub{
		rowQueue: []pgx.Row{
			rowStub{vals: []any{
				uuidToPGType(tenantID),
				"active",
				"provider_1",
				"gpt-4.1",
				[]byte(`{"streaming":true}`),
				uuidToPGType(principalID),
				now,
				now,
			}},
			rowStub{vals: []any{
				uuidToPGType(tenantID),
				"provider_1",
				"openai-compatible",
				"Primary",
				"https://example.invalid/v1",
				true,
				uuidToPGType(principalID),
				uuidToPGType(principalID),
				now,
				now,
				nullTimestamptz(),
			}},
		},
		rowsQueue: []pgx.Rows{
			&rowsStub{
				rows: [][]any{
					{
						uuidToPGType(tenantID),
						"cred_1",
						"provider_1",
						"env://OPENAI_API_KEY",
						"sk-****",
						int32(2),
						false,
						uuidToPGType(principalID),
						now,
						timestamptz(time.Date(2026, 4, 22, 10, 1, 0, 0, time.UTC)),
					},
				},
			},
		},
	}
	store := NewStore(beginFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))

	_, err := store.GetActiveModelRuntimeConfig(context.Background(), tenantID)
	if !errors.Is(err, ErrModelCredentialNotFound) {
		t.Fatalf("expected ErrModelCredentialNotFound, got %v", err)
	}
}

func ptr(value string) *string {
	return &value
}
