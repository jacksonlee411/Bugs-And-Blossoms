package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type ptrScanRow struct {
	vals []any
	err  error
}

func (r ptrScanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		var v any
		if i < len(r.vals) {
			v = r.vals[i]
		}
		if v == nil {
			switch d := dest[i].(type) {
			case **int:
				*d = nil
			case **string:
				*d = nil
			case *bool:
				*d = false
			case *int:
				*d = 0
			case *string:
				*d = ""
			default:
				// Best-effort for test stubs; treat as unsupported when needed.
			}
			continue
		}
		switch d := dest[i].(type) {
		case **int:
			tmp := v.(int)
			*d = &tmp
		case **string:
			tmp := v.(string)
			*d = &tmp
		case *bool:
			*d = v.(bool)
		case *int:
			*d = v.(int)
		case *string:
			*d = v.(string)
		default:
			return errors.New("unsupported scan dest")
		}
	}
	return nil
}

func TestOrgUnitPGStore_GetOrgUnitVersionExtSnapshot(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.GetOrgUnitVersionExtSnapshot(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetOrgUnitVersionExtSnapshot(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("version not found", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: pgx.ErrNoRows}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetOrgUnitVersionExtSnapshot(ctx, "t1", 10000001, "2026-01-01"); err == nil || !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("version query error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: errors.New("boom")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetOrgUnitVersionExtSnapshot(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unmarshal error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{[]byte("{bad"), []byte(`{}`), int64(0)}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetOrgUnitVersionExtSnapshot(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("labels decode error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{[]byte(`{"ext_str_01":"DEPARTMENT"}`), []byte("{bad"), int64(0)}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetOrgUnitVersionExtSnapshot(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("payload query no rows is ignored", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{[]byte(`{"ext_str_01":"DEPARTMENT"}`), []byte(`{"org_type":"Department"}`), int64(1)}},
			row2: metadataScanRow{err: pgx.ErrNoRows},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		snap, err := store.GetOrgUnitVersionExtSnapshot(ctx, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !snap.HasVersionData {
			t.Fatalf("expected version data")
		}
		if got := snap.VersionLabels["org_type"]; got != "Department" {
			t.Fatalf("labels=%v", snap.VersionLabels)
		}
		if len(snap.EventLabels) != 0 {
			t.Fatalf("expected empty event labels, got=%v", snap.EventLabels)
		}
	})

	t.Run("payload query error", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{[]byte(`{"ext_str_01":"DEPARTMENT"}`), []byte(`{}`), int64(1)}},
			row2: metadataScanRow{err: errors.New("payload")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetOrgUnitVersionExtSnapshot(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success with payload labels", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{[]byte(`{"ext_str_01":"DEPARTMENT"}`), []byte(`{}`), int64(1)}},
			row2: metadataScanRow{vals: []any{[]byte(`{"ext_labels_snapshot":{"org_type":"Department (e)"}}`)}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		snap, err := store.GetOrgUnitVersionExtSnapshot(ctx, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if snap.EventLabels["org_type"] != "Department (e)" {
			t.Fatalf("event labels=%v", snap.EventLabels)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row:       metadataScanRow{vals: []any{[]byte(`{}`), []byte(`{}`), int64(0)}},
			commitErr: errors.New("commit"),
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetOrgUnitVersionExtSnapshot(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_ResolveMutationTargetEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.ResolveMutationTargetEvent(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ResolveMutationTargetEvent(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("effective query error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: errors.New("boom")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ResolveMutationTargetEvent(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no effective and no raw", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{err: pgx.ErrNoRows},
			row2: metadataScanRow{err: pgx.ErrNoRows},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		got, err := store.ResolveMutationTargetEvent(ctx, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got.HasEffective || got.HasRaw {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("raw query by uuid error", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{"00000000-0000-0000-0000-000000000001", "CREATE"}},
			row2: metadataScanRow{err: errors.New("boom")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ResolveMutationTargetEvent(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fallback raw query error", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{"00000000-0000-0000-0000-000000000001", "CREATE"}},
			row2: metadataScanRow{err: pgx.ErrNoRows},
			row3: metadataScanRow{err: errors.New("boom")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ResolveMutationTargetEvent(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fallback raw query success", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{"00000000-0000-0000-0000-000000000001", "CREATE"}},
			row2: metadataScanRow{err: pgx.ErrNoRows},
			row3: metadataScanRow{vals: []any{"RENAME"}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		got, err := store.ResolveMutationTargetEvent(ctx, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !got.HasEffective || !got.HasRaw || got.RawEventType != "RENAME" {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("raw query by uuid success", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{"00000000-0000-0000-0000-000000000001", "CREATE"}},
			row2: metadataScanRow{vals: []any{"CREATE"}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		got, err := store.ResolveMutationTargetEvent(ctx, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !got.HasEffective || !got.HasRaw || got.RawEventType != "CREATE" {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("trim empty values clears flags", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{"00000000-0000-0000-0000-000000000001", "   "}},
			row2: metadataScanRow{vals: []any{"   "}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		got, err := store.ResolveMutationTargetEvent(ctx, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got.HasEffective || got.HasRaw {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row:       metadataScanRow{err: pgx.ErrNoRows},
			row2:      metadataScanRow{err: pgx.ErrNoRows},
			commitErr: errors.New("commit"),
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ResolveMutationTargetEvent(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_EvaluateRescindOrgDenyReasons(t *testing.T) {
	ctx := context.Background()
	nowOrgID := 10000001

	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.EvaluateRescindOrgDenyReasons(ctx, "t1", nowOrgID); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.EvaluateRescindOrgDenyReasons(ctx, "t1", nowOrgID); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("root query error", func(t *testing.T) {
		tx := &stubTx{row: ptrScanRow{err: errors.New("boom")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.EvaluateRescindOrgDenyReasons(ctx, "t1", nowOrgID); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("node path query error", func(t *testing.T) {
		tx := &stubTx{
			row:  ptrScanRow{err: pgx.ErrNoRows},
			row2: ptrScanRow{err: errors.New("boom")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.EvaluateRescindOrgDenyReasons(ctx, "t1", nowOrgID); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("children query error", func(t *testing.T) {
		tx := &stubTx{
			row:  ptrScanRow{err: pgx.ErrNoRows},
			row2: ptrScanRow{vals: []any{"1.2"}},
			row3: ptrScanRow{err: errors.New("boom")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.EvaluateRescindOrgDenyReasons(ctx, "t1", nowOrgID); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("dependencies query error", func(t *testing.T) {
		tx := &stubTx{
			row:  ptrScanRow{err: pgx.ErrNoRows},
			row2: ptrScanRow{err: pgx.ErrNoRows},
			row3: ptrScanRow{err: errors.New("boom")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.EvaluateRescindOrgDenyReasons(ctx, "t1", nowOrgID); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row:       ptrScanRow{err: pgx.ErrNoRows},
			row2:      ptrScanRow{err: pgx.ErrNoRows},
			row3:      ptrScanRow{vals: []any{false}},
			commitErr: errors.New("commit"),
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.EvaluateRescindOrgDenyReasons(ctx, "t1", nowOrgID); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("empty results", func(t *testing.T) {
		tx := &stubTx{
			row:  ptrScanRow{err: pgx.ErrNoRows},
			row2: ptrScanRow{err: pgx.ErrNoRows},
			row3: ptrScanRow{vals: []any{false}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		deny, err := store.EvaluateRescindOrgDenyReasons(ctx, "t1", nowOrgID)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(deny) != 0 {
			t.Fatalf("deny=%v", deny)
		}
	})

	t.Run("node path empty string skips children query", func(t *testing.T) {
		tx := &stubTx{
			row:  ptrScanRow{err: pgx.ErrNoRows},
			row2: ptrScanRow{vals: []any{""}},
			row3: ptrScanRow{vals: []any{false}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		deny, err := store.EvaluateRescindOrgDenyReasons(ctx, "t1", nowOrgID)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(deny) != 0 {
			t.Fatalf("deny=%v", deny)
		}
	})

	t.Run("root/children/dependencies denies", func(t *testing.T) {
		tx := &stubTx{
			row:  ptrScanRow{vals: []any{nowOrgID}},
			row2: ptrScanRow{vals: []any{"1.2"}},
			row3: ptrScanRow{vals: []any{true}},
			row4: ptrScanRow{vals: []any{true}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		deny, err := store.EvaluateRescindOrgDenyReasons(ctx, "t1", nowOrgID)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(deny) != 3 {
			t.Fatalf("deny=%v", deny)
		}
		if deny[0] != orgUnitErrRootDeleteForbidden || deny[1] != orgUnitErrHasChildrenCannotDelete || deny[2] != orgUnitErrHasDependenciesCannotDelete {
			t.Fatalf("deny=%v", deny)
		}
	})
}

func TestOrgUnitPGStore_IsOrgTreeInitialized(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.IsOrgTreeInitialized(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.IsOrgTreeInitialized(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{row: ptrScanRow{err: errors.New("boom")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.IsOrgTreeInitialized(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no rows means not initialized", func(t *testing.T) {
		tx := &stubTx{row: ptrScanRow{err: pgx.ErrNoRows}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		initialized, err := store.IsOrgTreeInitialized(ctx, "t1")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if initialized {
			t.Fatalf("expected false")
		}
	})

	t.Run("root exists means initialized", func(t *testing.T) {
		tx := &stubTx{row: ptrScanRow{vals: []any{10000001}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		initialized, err := store.IsOrgTreeInitialized(ctx, "t1")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !initialized {
			t.Fatalf("expected true")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row:       ptrScanRow{err: pgx.ErrNoRows},
			commitErr: errors.New("commit"),
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.IsOrgTreeInitialized(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_ResolveAppendFacts(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.ResolveAppendFacts(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ResolveAppendFacts(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("root query error", func(t *testing.T) {
		tx := &stubTx{row: ptrScanRow{err: errors.New("boom")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ResolveAppendFacts(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("status query error", func(t *testing.T) {
		tx := &stubTx{
			row:  ptrScanRow{vals: []any{10000001}},
			row2: metadataScanRow{err: errors.New("boom")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ResolveAppendFacts(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("target missing as of", func(t *testing.T) {
		tx := &stubTx{
			row:  ptrScanRow{vals: []any{10000001}},
			row2: metadataScanRow{err: pgx.ErrNoRows},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		facts, err := store.ResolveAppendFacts(ctx, "t1", 10000002, "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !facts.TreeInitialized || facts.IsRoot || facts.TargetExistsAsOf {
			t.Fatalf("facts=%+v", facts)
		}
	})

	t.Run("target exists and root", func(t *testing.T) {
		tx := &stubTx{
			row:  ptrScanRow{vals: []any{10000001}},
			row2: metadataScanRow{vals: []any{" disabled "}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		facts, err := store.ResolveAppendFacts(ctx, "t1", 10000001, "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !facts.TreeInitialized || !facts.IsRoot || !facts.TargetExistsAsOf {
			t.Fatalf("facts=%+v", facts)
		}
		if facts.TargetStatusAsOf != "disabled" {
			t.Fatalf("status=%q", facts.TargetStatusAsOf)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row:       ptrScanRow{err: pgx.ErrNoRows},
			row2:      metadataScanRow{err: pgx.ErrNoRows},
			commitErr: errors.New("commit"),
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ResolveAppendFacts(ctx, "t1", 10000001, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_ListOrgUnitsPage(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(123, 0).UTC()

	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ext filter not enabled", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: pgx.ErrNoRows}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ExtFilterFieldKey: "org_type", ExtFilterValue: "DEPARTMENT"}); err == nil || !errors.Is(err, errOrgUnitExtQueryFieldNotAllowed) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ext filter definition missing", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{"missing", "text", "PLAIN", []byte(`{}`), "ext_str_01", "2026-01-01", nil, now}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ExtFilterFieldKey: "missing", ExtFilterValue: "x"}); err == nil || !errors.Is(err, errOrgUnitExtQueryFieldNotAllowed) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ext filter allowlist rejects", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{"short_name", "text", "PLAIN", []byte(`{}`), "ext_str_01", "2026-01-01", nil, now}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ExtFilterFieldKey: "short_name", ExtFilterValue: "x"}); err == nil || !errors.Is(err, errOrgUnitExtQueryFieldNotAllowed) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ext filter physical col invalid", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{"org_type", "text", "DICT", []byte(`{}`), "bad_col", "2026-01-01", nil, now}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ExtFilterFieldKey: "org_type", ExtFilterValue: "x"}); err == nil || !errors.Is(err, errOrgUnitExtQueryFieldNotAllowed) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ext filter value parse error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{"org_type", "bool", "DICT", []byte(`{}`), "ext_bool_01", "2026-01-01", nil, now}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ExtFilterFieldKey: "org_type", ExtFilterValue: "bad"}); err == nil || !errors.Is(err, errOrgUnitExtQueryFieldNotAllowed) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ext filter query error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: errors.New("cfg")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ExtFilterFieldKey: "org_type", ExtFilterValue: "DEPARTMENT"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ext sort not enabled", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: pgx.ErrNoRows}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ExtSortFieldKey: "org_type"}); err == nil || !errors.Is(err, errOrgUnitExtQueryFieldNotAllowed) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ext sort allowlist rejects", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{"short_name", "text", "PLAIN", []byte(`{}`), "ext_str_01", "2026-01-01", nil, now}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ExtSortFieldKey: "short_name"}); err == nil || !errors.Is(err, errOrgUnitExtQueryFieldNotAllowed) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ext sort physical col invalid", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{"org_type", "text", "DICT", []byte(`{}`), "bad_col", "2026-01-01", nil, now}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ExtSortFieldKey: "org_type"}); err == nil || !errors.Is(err, errOrgUnitExtQueryFieldNotAllowed) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ext sort query error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: errors.New("cfg")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ExtSortFieldKey: "org_type"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("count query error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: errors.New("count")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("list query error", func(t *testing.T) {
		tx := &stubTx{
			row:      metadataScanRow{vals: []any{int(1)}},
			queryErr: errors.New("query"),
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows scan error", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{int(1)}},
			rows: &metadataScanRows{records: [][]any{{"A001", "Root", "active", true}}, scanErr: errors.New("scan")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows scan error with has_children", func(t *testing.T) {
		parentID := 123
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{int(1)}},
			rows: &metadataScanRows{records: [][]any{{"A001", "Child", "active", true, true}}, scanErr: errors.New("scan")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ParentID: &parentID}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows error", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{int(0)}},
			rows: &metadataScanRows{records: [][]any{}, err: errors.New("rows")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row:       metadataScanRow{vals: []any{int(1)}},
			rows:      &metadataScanRows{records: [][]any{{"A001", "Root", "active", true}}},
			commitErr: errors.New("commit"),
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{AsOf: "2026-01-01"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success root list with pagination and keyword/status filter", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{int(1)}},
			rows: &metadataScanRows{records: [][]any{{"A001", "Root", "", true}}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		items, total, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{
			AsOf:            "2026-01-01",
			IncludeDisabled: false,
			Keyword:         "A",
			Status:          orgUnitListStatusActive,
			SortField:       orgUnitListSortName,
			SortOrder:       "asc",
			Limit:           10,
			Offset:          0,
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if total != 1 || len(items) != 1 || items[0].Status != orgUnitListStatusActive {
			t.Fatalf("total=%d items=%v", total, items)
		}
	})

	t.Run("success children list with has_children", func(t *testing.T) {
		parentID := 123
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{int(1)}},
			rows: &metadataScanRows{records: [][]any{{"A001", "Child", "active", true, true}}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		items, total, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{
			AsOf:            "2026-01-01",
			IncludeDisabled: true,
			ParentID:        &parentID,
			Status:          orgUnitListStatusDisabled,
			SortField:       orgUnitListSortCode,
			SortOrder:       "DESC",
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if total != 1 || len(items) != 1 || items[0].HasChildren == nil || !*items[0].HasChildren {
			t.Fatalf("total=%d items=%v", total, items)
		}
	})

	t.Run("success with ext sort", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{"org_type", "text", "DICT", []byte(`{}`), "ext_str_01", "2026-01-01", nil, now}},
			row2: metadataScanRow{vals: []any{int(1)}},
			rows: &metadataScanRows{records: [][]any{{"A001", "Root", "active", true}}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		items, total, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{
			AsOf:            "2026-01-01",
			IncludeDisabled: false,
			ExtSortFieldKey: "org_type",
			SortOrder:       "bad",
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if total != 1 || len(items) != 1 {
			t.Fatalf("total=%d items=%v", total, items)
		}
	})

	t.Run("success with ext filter", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{"org_type", "text", "DICT", []byte(`{}`), "ext_str_01", "2026-01-01", nil, now}},
			row2: metadataScanRow{vals: []any{int(1)}},
			rows: &metadataScanRows{records: [][]any{{"A001", "Root", "active", true}}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		items, total, err := store.ListOrgUnitsPage(ctx, "t1", orgUnitListPageRequest{
			AsOf:              "2026-01-01",
			IncludeDisabled:   true,
			ExtFilterFieldKey: "org_type",
			ExtFilterValue:    "DEPARTMENT",
			SortField:         orgUnitListSortStatus,
			SortOrder:         "DESC",
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if total != 1 || len(items) != 1 {
			t.Fatalf("total=%d items=%v", total, items)
		}
	})
}
