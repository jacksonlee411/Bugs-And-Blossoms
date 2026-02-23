package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func newConcreteOrgUnitPGStore(begin beginFunc) *OrgUnitPGStore {
	return &OrgUnitPGStore{pool: begin}
}

type execAtTxStub struct {
	*txStub
	execErrAt int
	execN     int
}

func (t *execAtTxStub) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	t.execN++
	at := t.execErrAt
	if at <= 0 {
		at = 1
	}
	if t.execErr != nil && t.execN == at {
		return pgconn.CommandTag{}, t.execErr
	}
	return pgconn.CommandTag{}, nil
}

func TestOrgUnitPGStore_FindEventByRequestID(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		if _, found, err := store.FindEventByRequestID(ctx, "t1", "r1"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("set_config error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{execErr: errors.New("exec")}, nil
		}))
		if _, found, err := store.FindEventByRequestID(ctx, "t1", "r1"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("no rows", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{row: stubRow{err: pgx.ErrNoRows}}, nil
		}))
		_, found, err := store.FindEventByRequestID(ctx, "t1", "r1")
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("query row error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{row: stubRow{err: errors.New("row")}}, nil
		}))
		if _, found, err := store.FindEventByRequestID(ctx, "t1", "r1"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{
				row:       stubRow{vals: []any{int64(1), "e1", 10000001, "CREATE", "2026-01-01", []byte(`{"a":"b"}`), time.Unix(1, 0).UTC()}},
				commitErr: errors.New("commit"),
			}, nil
		}))
		if _, found, err := store.FindEventByRequestID(ctx, "t1", "r1"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{row: stubRow{vals: []any{int64(1), "e1", 10000001, "CREATE", "2026-01-01", []byte(`{"org_code":"ROOT"}`), time.Unix(1, 0).UTC()}}}, nil
		}))
		event, found, err := store.FindEventByRequestID(ctx, "t1", "r1")
		if err != nil || !found {
			t.Fatalf("event=%+v found=%v err=%v", event, found, err)
		}
		if string(event.Payload) != `{"org_code":"ROOT"}` {
			t.Fatalf("payload=%s", string(event.Payload))
		}
	})
}

func TestOrgUnitPGStore_ResolveTenantFieldPolicy(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		if _, found, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("set_config error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{execErr: errors.New("exec")}, nil
		}))
		if _, found, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{}, nil
		}))
		_, found, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "BAD", "x", "2026-01-01")
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("no rows", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{row: stubRow{err: pgx.ErrNoRows}}, nil
		}))
		_, found, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01")
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("query row error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{row: stubRow{err: errors.New("row")}}, nil
		}))
		if _, found, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{
				row:       stubRow{vals: []any{"org_code", "GLOBAL", "global", true, "none", nil, "2026-01-01", nil}},
				commitErr: errors.New("commit"),
			}, nil
		}))
		if _, found, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "GLOBAL", "ignored", "2026-01-01"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("success", func(t *testing.T) {
		rule := "next_org_code(\"O\", 6)"
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{row: stubRow{vals: []any{"org_code", "FORM", "orgunit.create_dialog", false, "cel", rule, "2026-01-01", nil}}}, nil
		}))
		policy, found, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01")
		if err != nil || !found {
			t.Fatalf("policy=%+v found=%v err=%v", policy, found, err)
		}
		if policy.DefaultMode != "CEL" {
			t.Fatalf("policy=%+v", policy)
		}
	})
}

func TestOrgUnitPGStore_SubmitCreateEventWithGeneratedCode(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		if _, _, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", nil, "r1", "u1", "O", 3); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &execAtTxStub{txStub: &txStub{execErr: errors.New("exec")}, execErrAt: 1}, nil
		}))
		if _, _, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", nil, "r1", "u1", "O", 3); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("advisory lock error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &execAtTxStub{txStub: &txStub{execErr: errors.New("lock")}, execErrAt: 2}, nil
		}))
		if _, _, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", nil, "r1", "u1", "O", 3); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query existing codes error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{queryErr: errors.New("query")}, nil
		}))
		if _, _, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", nil, "r1", "u1", "O", 3); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows scan error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{
				rows: &rowsWithData{
					stubRows: &stubRows{},
					data:     [][]any{{"O001"}},
					scanErr:  errors.New("scan"),
				},
			}, nil
		}))
		if _, _, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", nil, "r1", "u1", "O", 3); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{
				rows: &rowsWithData{
					stubRows: &stubRows{},
					err:      errors.New("rows"),
				},
			}, nil
		}))
		if _, _, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", nil, "r1", "u1", "O", 3); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("code exhausted", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{
				rows: &rowsWithData{
					stubRows: &stubRows{},
					data: [][]any{
						{"O1"}, {"O2"}, {"O3"}, {"O4"}, {"O5"}, {"O6"}, {"O7"}, {"O8"}, {"O9"},
					},
				},
			}, nil
		}))
		if _, _, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", nil, "r1", "u1", "O", 1); err == nil || err.Error() != "ORG_CODE_EXHAUSTED" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ignore malformed existing codes", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{
				rows: &rowsWithData{
					stubRows: &stubRows{},
					data: [][]any{
						{"X001"},
						{"OABC"},
						{"O000"},
						{"O002"},
					},
				},
				row: stubRow{vals: []any{int64(8)}},
			}, nil
		}))
		eventID, orgCode, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", nil, "r1", "u1", "O", 3)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if eventID != 8 || orgCode != "O001" {
			t.Fatalf("eventID=%d orgCode=%s", eventID, orgCode)
		}
	})

	t.Run("payload unmarshal error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{rows: &rowsWithData{stubRows: &stubRows{}}}, nil
		}))
		if _, _, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", json.RawMessage(`{`), "r1", "u1", "O", 3); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("payload marshal error", func(t *testing.T) {
		orig := marshalCreatePayloadJSON
		marshalCreatePayloadJSON = func(any) ([]byte, error) { return nil, errors.New("marshal") }
		t.Cleanup(func() { marshalCreatePayloadJSON = orig })

		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{rows: &rowsWithData{stubRows: &stubRows{}}}, nil
		}))
		if _, _, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", nil, "r1", "u1", "O", 3); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit event error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{
				rows: &rowsWithData{stubRows: &stubRows{}},
				row:  stubRow{err: errors.New("submit")},
			}, nil
		}))
		if _, _, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", nil, "r1", "u1", "O", 3); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{
				rows:      &rowsWithData{stubRows: &stubRows{}},
				row:       stubRow{vals: []any{int64(9)}},
				commitErr: errors.New("commit"),
			}, nil
		}))
		if _, _, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", nil, "r1", "u1", "O", 3); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success chooses first gap", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{
				rows: &rowsWithData{
					stubRows: &stubRows{},
					data: [][]any{
						{"O001"},
						{"O003"},
					},
				},
				row: stubRow{vals: []any{int64(11)}},
			}, nil
		}))
		eventID, orgCode, err := store.SubmitCreateEventWithGeneratedCode(ctx, "t1", "e1", "2026-01-01", json.RawMessage(`{"name":"Root"}`), "r1", "u1", "O", 3)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if eventID != 11 || orgCode != "O002" {
			t.Fatalf("eventID=%d orgCode=%q", eventID, orgCode)
		}
	})
}

func TestOrgUnitPGStore_ResolveSetIDStrategyFieldDecision(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		if _, found, err := store.ResolveSetIDStrategyFieldDecision(ctx, "t1", "org.orgunit_create.field_policy", "org_code", "", "2026-01-01"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("set_config error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{execErr: errors.New("exec")}, nil
		}))
		if _, found, err := store.ResolveSetIDStrategyFieldDecision(ctx, "t1", "org.orgunit_create.field_policy", "org_code", "", "2026-01-01"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{row: stubRow{err: pgx.ErrNoRows}}, nil
		}))
		_, found, err := store.ResolveSetIDStrategyFieldDecision(ctx, "t1", "org.orgunit_create.field_policy", "org_code", "", "2026-01-01")
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("query row error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{row: stubRow{err: errors.New("row")}}, nil
		}))
		if _, found, err := store.ResolveSetIDStrategyFieldDecision(ctx, "t1", "org.orgunit_create.field_policy", "org_code", "", "2026-01-01"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("allowed_value_codes json invalid", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{row: stubRow{vals: []any{"org.orgunit_create.field_policy", "org_code", true, true, true, "", "", "{"}}}, nil
		}))
		if _, found, err := store.ResolveSetIDStrategyFieldDecision(ctx, "t1", "org.orgunit_create.field_policy", "org_code", "", "2026-01-01"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{
				row:       stubRow{vals: []any{"org.orgunit_create.field_policy", "org_code", true, true, false, `next_org_code("F", 8)`, "", `["11"]`}},
				commitErr: errors.New("commit"),
			}, nil
		}))
		if _, found, err := store.ResolveSetIDStrategyFieldDecision(ctx, "t1", "org.orgunit_create.field_policy", "org_code", "", "2026-01-01"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newConcreteOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
			return &txStub{
				row: stubRow{vals: []any{"org.orgunit_create.field_policy", "d_org_type", true, true, true, "", "11", `[" 11 ", "11", "12"]`}},
			}, nil
		}))
		decision, found, err := store.ResolveSetIDStrategyFieldDecision(ctx, "t1", " Org.OrgUnit_Create.Field_Policy ", " D_Org_Type ", "", "2026-01-01")
		if err != nil || !found {
			t.Fatalf("decision=%+v found=%v err=%v", decision, found, err)
		}
		if decision.CapabilityKey != "org.orgunit_create.field_policy" || decision.FieldKey != "d_org_type" {
			t.Fatalf("decision=%+v", decision)
		}
		if len(decision.AllowedValueCodes) != 2 || decision.AllowedValueCodes[0] != "11" || decision.AllowedValueCodes[1] != "12" {
			t.Fatalf("allowed=%v", decision.AllowedValueCodes)
		}
	})
}

func TestNormalizeAllowedValueCodes(t *testing.T) {
	if got := normalizeAllowedValueCodes(nil); got != nil {
		t.Fatalf("got=%v", got)
	}
	if got := normalizeAllowedValueCodes([]string{" ", ""}); got != nil {
		t.Fatalf("got=%v", got)
	}
	got := normalizeAllowedValueCodes([]string{" 11 ", "11", "", "12"})
	if len(got) != 2 || got[0] != "11" || got[1] != "12" {
		t.Fatalf("got=%v", got)
	}
}

func TestCloneOptionalString(t *testing.T) {
	if got := cloneOptionalString(nil); got != nil {
		t.Fatalf("expected nil")
	}
	in := "  X001 "
	got := cloneOptionalString(&in)
	if got == nil || *got != "X001" {
		t.Fatalf("got=%v", got)
	}
}
