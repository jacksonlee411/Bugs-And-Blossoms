package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestOrgUnitPGStore_ListTenantFieldPolicies(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(123, 0).UTC()

	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.ListTenantFieldPolicies(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListTenantFieldPolicies(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{queryErr: errors.New("query")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListTenantFieldPolicies(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		tx := &stubTx{
			rows: &metadataScanRows{
				records: [][]any{{"org_code", "FORM", "orgunit.create_dialog", true, "NONE", nil, "2026-01-01", nil, now}},
				scanErr: errors.New("scan"),
			},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListTenantFieldPolicies(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows error", func(t *testing.T) {
		tx := &stubTx{
			rows: &metadataScanRows{
				records: [][]any{},
				err:     errors.New("rows"),
			},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListTenantFieldPolicies(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			rows:      &metadataScanRows{records: [][]any{}},
			commitErr: errors.New("commit"),
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListTenantFieldPolicies(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		rule := "next_org_code(\"O\", 6)"
		disabled := "2026-02-01"
		tx := &stubTx{
			rows: &metadataScanRows{
				records: [][]any{
					{"org_code", "FORM", "orgunit.create_dialog", false, "cel", rule, "2026-01-01", disabled, now},
				},
			},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		items, err := store.ListTenantFieldPolicies(ctx, "t1")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(items) != 1 {
			t.Fatalf("items=%v", items)
		}
		if items[0].DefaultMode != "CEL" || items[0].DefaultRuleExpr == nil || *items[0].DefaultRuleExpr != rule {
			t.Fatalf("item=%+v", items[0])
		}
	})
}

func TestResolveTenantFieldPolicyTx_Branches(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(123, 0).UTC()

	t.Run("blank field key", func(t *testing.T) {
		item, found, err := resolveTenantFieldPolicyTx(ctx, &stubTx{}, "t1", " ", "FORM", "orgunit.create_dialog", "2026-01-01")
		if err != nil || found || item.FieldKey != "" {
			t.Fatalf("item=%+v found=%v err=%v", item, found, err)
		}
	})

	t.Run("invalid scope type", func(t *testing.T) {
		_, found, err := resolveTenantFieldPolicyTx(ctx, &stubTx{}, "t1", "org_code", "BAD", "x", "2026-01-01")
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("empty scope type defaults to form", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: pgx.ErrNoRows}}
		_, found, err := resolveTenantFieldPolicyTx(ctx, tx, "t1", "org_code", "", "orgunit.create_dialog", "2026-01-01")
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("no rows returns not found", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: pgx.ErrNoRows}}
		_, found, err := resolveTenantFieldPolicyTx(ctx, tx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01")
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: errors.New("boom")}}
		if _, _, err := resolveTenantFieldPolicyTx(ctx, tx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global success", func(t *testing.T) {
		rule := "next_org_code(\"O\", 6)"
		row := metadataScanRow{vals: []any{
			"org_code",
			"GLOBAL",
			"global",
			false,
			"CEL",
			rule,
			"2026-01-01",
			nil,
			now,
		}}
		tx := &stubTx{
			row: row,
			row2: metadataScanRow{vals: []any{
				"org_code",
				"GLOBAL",
				"global",
				false,
				"CEL",
				rule,
				"2026-01-01",
				nil,
				now,
			}},
		}
		item, found, err := resolveTenantFieldPolicyTx(ctx, tx, "t1", "org_code", "GLOBAL", "ignored", "2026-01-01")
		if err != nil || !found {
			t.Fatalf("item=%+v found=%v err=%v", item, found, err)
		}
		if item.ScopeType != "GLOBAL" || item.ScopeKey != "global" {
			t.Fatalf("item=%+v", item)
		}
	})
}

func TestOrgUnitPGStore_ResolveTenantFieldPolicy(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(123, 0).UTC()

	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, _, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid scope returns not found", func(t *testing.T) {
		tx := &stubTx{}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		_, found, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "BAD", "x", "2026-01-01")
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: errors.New("boom")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row: metadataScanRow{vals: []any{
				"org_code",
				"FORM",
				"orgunit.create_dialog",
				true,
				"NONE",
				nil,
				"2026-01-01",
				nil,
				now,
			}},
			commitErr: errors.New("commit"),
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success not found", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: pgx.ErrNoRows}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		_, found, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01")
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("success found", func(t *testing.T) {
		rule := "next_org_code(\"O\", 6)"
		tx := &stubTx{
			row: metadataScanRow{vals: []any{
				"org_code",
				"FORM",
				"orgunit.create_dialog",
				false,
				"CEL",
				rule,
				"2026-01-01",
				nil,
				now,
			}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		item, found, err := store.ResolveTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-01-01")
		if err != nil || !found {
			t.Fatalf("item=%+v found=%v err=%v", item, found, err)
		}
		if item.DefaultMode != "CEL" {
			t.Fatalf("item=%+v", item)
		}
	})
}

func TestOrgUnitPGStore_UpsertTenantFieldPolicy(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(123, 0).UTC()

	call := func(tx pgx.Tx) *orgUnitPGStore {
		return &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	}

	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, _, err := store.UpsertTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", true, "NONE", nil, "2026-01-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config error", func(t *testing.T) {
		if _, _, err := call(&stubTx{execErr: errors.New("exec")}).UpsertTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", true, "NONE", nil, "2026-01-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("request exists error", func(t *testing.T) {
		if _, _, err := call(&stubTx{row: metadataScanRow{err: errors.New("req")}}).UpsertTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", true, "NONE", nil, "2026-01-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("upsert query error", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{err: pgx.ErrNoRows},
			row2: metadataScanRow{err: errors.New("upsert")},
		}
		if _, _, err := call(tx).UpsertTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", true, "NONE", nil, "2026-01-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("get by id error", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{err: pgx.ErrNoRows},
			row2: metadataScanRow{vals: []any{int64(9)}},
			row3: metadataScanRow{err: errors.New("get")},
		}
		if _, _, err := call(tx).UpsertTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", true, "NONE", nil, "2026-01-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{err: pgx.ErrNoRows},
			row2: metadataScanRow{vals: []any{int64(9)}},
			row3: metadataScanRow{vals: []any{
				"org_code",
				"FORM",
				"orgunit.create_dialog",
				true,
				"NONE",
				nil,
				"2026-01-01",
				nil,
				now,
			}},
			commitErr: errors.New("commit"),
		}
		if _, _, err := call(tx).UpsertTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", true, "NONE", nil, "2026-01-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success and was retry", func(t *testing.T) {
		rule := "next_org_code(\"O\", 6)"
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{"UPSERT"}},
			row2: metadataScanRow{vals: []any{int64(9)}},
			row3: metadataScanRow{vals: []any{
				"org_code",
				"FORM",
				"orgunit.create_dialog",
				false,
				"CEL",
				rule,
				"2026-01-01",
				nil,
				now,
			}},
		}
		item, wasRetry, err := call(tx).UpsertTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", false, "CEL", &rule, "2026-01-01", "r1", "u1")
		if err != nil || !wasRetry {
			t.Fatalf("item=%+v wasRetry=%v err=%v", item, wasRetry, err)
		}
	})
}

func TestOrgUnitPGStore_DisableTenantFieldPolicy(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(123, 0).UTC()

	call := func(tx pgx.Tx) *orgUnitPGStore {
		return &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	}

	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, _, err := store.DisableTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-02-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config error", func(t *testing.T) {
		if _, _, err := call(&stubTx{execErr: errors.New("exec")}).DisableTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-02-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("request exists error", func(t *testing.T) {
		if _, _, err := call(&stubTx{row: metadataScanRow{err: errors.New("req")}}).DisableTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-02-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("disable query error", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{err: pgx.ErrNoRows},
			row2: metadataScanRow{err: errors.New("disable")},
		}
		if _, _, err := call(tx).DisableTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-02-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("get by id error", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{err: pgx.ErrNoRows},
			row2: metadataScanRow{vals: []any{int64(9)}},
			row3: metadataScanRow{err: errors.New("get")},
		}
		if _, _, err := call(tx).DisableTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-02-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{err: pgx.ErrNoRows},
			row2: metadataScanRow{vals: []any{int64(9)}},
			row3: metadataScanRow{vals: []any{
				"org_code",
				"FORM",
				"orgunit.create_dialog",
				true,
				"NONE",
				nil,
				"2026-01-01",
				nil,
				now,
			}},
			commitErr: errors.New("commit"),
		}
		if _, _, err := call(tx).DisableTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-02-01", "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success and was retry", func(t *testing.T) {
		tx := &stubTx{
			row:  metadataScanRow{vals: []any{"DISABLE"}},
			row2: metadataScanRow{vals: []any{int64(9)}},
			row3: metadataScanRow{vals: []any{
				"org_code",
				"FORM",
				"orgunit.create_dialog",
				true,
				"NONE",
				nil,
				"2026-01-01",
				nil,
				now,
			}},
		}
		_, wasRetry, err := call(tx).DisableTenantFieldPolicy(ctx, "t1", "org_code", "FORM", "orgunit.create_dialog", "2026-02-01", "r1", "u1")
		if err != nil || !wasRetry {
			t.Fatalf("wasRetry=%v err=%v", wasRetry, err)
		}
	})
}

func TestTenantFieldPolicyRequestExistsTx(t *testing.T) {
	ctx := context.Background()

	t.Run("no rows", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: pgx.ErrNoRows}}
		exists, err := tenantFieldPolicyRequestExistsTx(ctx, tx, "t1", "r1", "UPSERT")
		if err != nil || exists {
			t.Fatalf("exists=%v err=%v", exists, err)
		}
	})

	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: errors.New("boom")}}
		if _, err := tenantFieldPolicyRequestExistsTx(ctx, tx, "t1", "r1", "UPSERT"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event mismatch", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{"DISABLE"}}}
		exists, err := tenantFieldPolicyRequestExistsTx(ctx, tx, "t1", "r1", "UPSERT")
		if err != nil || exists {
			t.Fatalf("exists=%v err=%v", exists, err)
		}
	})

	t.Run("event matches", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{"UPSERT"}}}
		exists, err := tenantFieldPolicyRequestExistsTx(ctx, tx, "t1", "r1", "UPSERT")
		if err != nil || !exists {
			t.Fatalf("exists=%v err=%v", exists, err)
		}
	})
}

func TestGetTenantFieldPolicyByIDTx(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(123, 0).UTC()
	rule := "next_org_code(\"O\", 6)"

	t.Run("scan error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: errors.New("boom")}}
		if _, err := getTenantFieldPolicyByIDTx(ctx, tx, "t1", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{
			"org_code",
			"FORM",
			"orgunit.create_dialog",
			false,
			"CEL",
			rule,
			"2026-01-01",
			nil,
			now,
		}}}
		item, err := getTenantFieldPolicyByIDTx(ctx, tx, "t1", 1)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if item.DefaultMode != "CEL" || item.DefaultRuleExpr == nil {
			t.Fatalf("item=%+v", item)
		}
	})
}
