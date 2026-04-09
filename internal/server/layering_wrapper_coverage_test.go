package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestDictCompatibilityWrappers(t *testing.T) {
	ctx := context.Background()

	store := newDictPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{}, nil
	}))
	if store == nil {
		t.Fatal("expected store")
	}

	sourceTx := &stubTx{row: &stubRow{vals: []any{"t1"}}}
	sourceTenant, err := resolveDictSourceTenantAsOfTx(ctx, sourceTx, "t1", "org_type", "2026-01-01")
	if err != nil || sourceTenant != "t1" {
		t.Fatalf("sourceTenant=%q err=%v", sourceTenant, err)
	}

	now := time.Unix(5, 0).UTC()
	valueSnapshot, _ := json.Marshal(map[string]any{
		"dict_code":   "org_type",
		"code":        "10",
		"label":       "部门",
		"status":      "active",
		"enabled_on":  "1970-01-01",
		"disabled_on": nil,
	})
	valueTx := &stubTx{row: &stubRow{vals: []any{valueSnapshot, now}}}
	item, err := getDictValueFromEventTx(ctx, valueTx, "t1", 1)
	if err != nil || item.Code != "10" || item.UpdatedAt != now {
		t.Fatalf("item=%+v err=%v", item, err)
	}

	readyTx := &stubTx{row: &stubRow{vals: []any{true}}}
	if err := assertTenantBaselineReadyTx(ctx, readyTx, "t1"); err != nil {
		t.Fatalf("err=%v", err)
	}

	disabledOn := "2026-02-01"
	cloned := cloneDictItem(DictItem{DictCode: "org_type", DisabledOn: &disabledOn})
	if cloned.DisabledOn == nil || *cloned.DisabledOn != disabledOn {
		t.Fatalf("cloned=%+v", cloned)
	}
}

func TestSetIDCompatibilityWrappers(t *testing.T) {
	ctx := context.Background()

	store := newSetIDPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{}, nil
	}))
	if store == nil {
		t.Fatal("expected store")
	}

	globalRows := &tableRows{rows: [][]any{{"SHARE", "Shared", "active"}}}
	tx := &stubTx{
		row:  &stubRow{vals: []any{"gt1"}},
		rows: globalRows,
	}
	pgStore := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	setids, err := pgStore.ListGlobalSetIDs(ctx)
	if err != nil || len(setids) != 1 || !setids[0].IsShared {
		t.Fatalf("setids=%+v err=%v", setids, err)
	}

	globalRows2 := &tableRows{rows: [][]any{{"SHARE", "Shared", "active"}}}
	tx2 := &stubTx{
		row:  &stubRow{vals: []any{"gt1"}},
		rows: globalRows2,
	}
	pgStore2 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx2, nil })}
	setids2, err := pgStore2.listGlobalSetIDs(ctx)
	if err != nil || len(setids2) != 1 {
		t.Fatalf("setids=%+v err=%v", setids2, err)
	}
}
