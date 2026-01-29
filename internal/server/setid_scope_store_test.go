package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type seqTx struct {
	*stubTx
	rows []pgx.Row
}

func (t *seqTx) QueryRow(context.Context, string, ...any) pgx.Row {
	if len(t.rows) == 0 {
		return fakeRow{}
	}
	r := t.rows[0]
	t.rows = t.rows[1:]
	return r
}

func scopePackageRow(id string, scopeCode string, packageCode string, name string, status string) *stubRow {
	return &stubRow{vals: []any{id, scopeCode, packageCode, name, status}}
}

func scopeSubscriptionRow(setid string, scopeCode string, packageID string, ownerTenantID string, start string, end string) *stubRow {
	return &stubRow{vals: []any{setid, scopeCode, packageID, ownerTenantID, start, end}}
}

func TestSetIDPGStore_ListScopeCodes(t *testing.T) {
	tx := &stubTx{
		rows: &tableRows{rows: [][]any{
			{"jobcatalog", "jobcatalog", "tenant-only", true},
		}},
	}
	store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	codes, err := store.ListScopeCodes(context.Background(), "t1")
	if err != nil || len(codes) != 1 {
		t.Fatalf("len=%d err=%v", len(codes), err)
	}

	txQueryErr := &stubTx{queryErr: errors.New("query fail")}
	storeQueryErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txQueryErr, nil })}
	if _, err := storeQueryErr.ListScopeCodes(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txScanErr := &stubTx{rows: &scanErrRows{}}
	storeScanErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txScanErr, nil })}
	if _, err := storeScanErr.ListScopeCodes(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txRowsErr := &stubTx{rows: &tableRows{rows: [][]any{}, err: errors.New("rows err")}}
	storeRowsErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txRowsErr, nil })}
	if _, err := storeRowsErr.ListScopeCodes(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDPGStore_CreateScopePackage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tx := &stubTx{
			row:  &stubRow{vals: []any{"p1"}},
			row2: &stubRow{vals: []any{"e1"}},
			row3: scopePackageRow("p1", "jobcatalog", "PKG1", "Pkg", "active"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		pkg, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1")
		if err != nil || pkg.PackageID != "p1" {
			t.Fatalf("pkg=%+v err=%v", pkg, err)
		}
	})

	t.Run("bootstrap error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec fail"), execErrAt: 2}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("package id error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("row fail")}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event id error", func(t *testing.T) {
		tx := &stubTx{
			row:     &stubRow{vals: []any{"p1"}},
			row2Err: errors.New("row fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{"p1"}},
			row2:      &stubRow{vals: []any{"e1"}},
			execErr:   errors.New("exec fail"),
			execErrAt: 3,
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fetch error", func(t *testing.T) {
		tx := &stubTx{
			row:     &stubRow{vals: []any{"p1"}},
			row2:    &stubRow{vals: []any{"e1"}},
			row3Err: errors.New("fetch fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fetch missing uses request id", func(t *testing.T) {
		tx := &stubTx{
			row:  &stubRow{vals: []any{"p1"}},
			row2: &stubRow{vals: []any{"e1"}},
			row3: &stubRow{err: pgx.ErrNoRows},
			row4: &stubRow{vals: []any{"p2"}},
			row5: scopePackageRow("p2", "jobcatalog", "PKG2", "Pkg2", "active"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		pkg, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1")
		if err != nil || pkg.PackageID != "p2" {
			t.Fatalf("pkg=%+v err=%v", pkg, err)
		}
	})

	t.Run("fetch missing existing id error", func(t *testing.T) {
		tx := &stubTx{
			row:     &stubRow{vals: []any{"p1"}},
			row2:    &stubRow{vals: []any{"e1"}},
			row3:    &stubRow{err: pgx.ErrNoRows},
			row4Err: errors.New("row fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fetch missing second fetch error", func(t *testing.T) {
		tx := &stubTx{
			row:     &stubRow{vals: []any{"p1"}},
			row2:    &stubRow{vals: []any{"e1"}},
			row3:    &stubRow{err: pgx.ErrNoRows},
			row4:    &stubRow{vals: []any{"p2"}},
			row5Err: errors.New("fetch fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSetIDPGStore_DisableScopePackage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tx := &stubTx{
			row:  &stubRow{vals: []any{"e1"}},
			row2: scopePackageRow("p1", "jobcatalog", "PKG1", "Pkg", "disabled"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		pkg, err := store.DisableScopePackage(context.Background(), "t1", "p1", "r1", "p1")
		if err != nil || pkg.Status != "disabled" {
			t.Fatalf("pkg=%+v err=%v", pkg, err)
		}
	})

	t.Run("event id error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("row fail")}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.DisableScopePackage(context.Background(), "t1", "p1", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{"e1"}},
			execErr:   errors.New("exec fail"),
			execErrAt: 2,
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.DisableScopePackage(context.Background(), "t1", "p1", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fetch error", func(t *testing.T) {
		tx := &stubTx{
			row:     &stubRow{vals: []any{"e1"}},
			row2Err: errors.New("row fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.DisableScopePackage(context.Background(), "t1", "p1", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSetIDPGStore_ListScopePackages(t *testing.T) {
	tx := &stubTx{
		rows: &tableRows{rows: [][]any{
			{"p1", "jobcatalog", "PKG1", "Pkg", "active"},
		}},
	}
	store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	pkgs, err := store.ListScopePackages(context.Background(), "t1", "jobcatalog")
	if err != nil || len(pkgs) != 1 {
		t.Fatalf("len=%d err=%v", len(pkgs), err)
	}

	txQueryErr := &stubTx{queryErr: errors.New("query fail")}
	storeQueryErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txQueryErr, nil })}
	if _, err := storeQueryErr.ListScopePackages(context.Background(), "t1", "jobcatalog"); err == nil {
		t.Fatal("expected error")
	}

	txScanErr := &stubTx{rows: &scanErrRows{}}
	storeScanErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txScanErr, nil })}
	if _, err := storeScanErr.ListScopePackages(context.Background(), "t1", "jobcatalog"); err == nil {
		t.Fatal("expected error")
	}

	txRowsErr := &stubTx{rows: &tableRows{rows: [][]any{}, err: errors.New("rows err")}}
	storeRowsErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txRowsErr, nil })}
	if _, err := storeRowsErr.ListScopePackages(context.Background(), "t1", "jobcatalog"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDPGStore_CreateScopeSubscription(t *testing.T) {
	t.Run("tenant success", func(t *testing.T) {
		tx := &stubTx{
			row:  &stubRow{vals: []any{"e1"}},
			row2: scopeSubscriptionRow("S2601", "jobcatalog", "p1", "t1", "2026-01-01", ""),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		sub, err := store.CreateScopeSubscription(context.Background(), "t1", "S2601", "jobcatalog", "p1", "tenant", "2026-01-01", "r1", "p1")
		if err != nil || sub.PackageOwner != "tenant" {
			t.Fatalf("sub=%+v err=%v", sub, err)
		}
	})

	t.Run("global success", func(t *testing.T) {
		tx := &stubTx{
			row:  &stubRow{vals: []any{"e1"}},
			row2: &stubRow{vals: []any{"gt1"}},
			row3: scopeSubscriptionRow("S2601", "jobcatalog", "p1", "gt1", "2026-01-01", ""),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		sub, err := store.CreateScopeSubscription(context.Background(), "t1", "S2601", "jobcatalog", "p1", "global", "2026-01-01", "r1", "p1")
		if err != nil || sub.PackageOwner != "global" {
			t.Fatalf("sub=%+v err=%v", sub, err)
		}
	})

	t.Run("event id error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("row fail")}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateScopeSubscription(context.Background(), "t1", "S2601", "jobcatalog", "p1", "tenant", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global tenant id error", func(t *testing.T) {
		tx := &stubTx{
			row:     &stubRow{vals: []any{"e1"}},
			row2Err: errors.New("row fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateScopeSubscription(context.Background(), "t1", "S2601", "jobcatalog", "p1", "global", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{"e1"}},
			execErr:   errors.New("exec fail"),
			execErrAt: 2,
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateScopeSubscription(context.Background(), "t1", "S2601", "jobcatalog", "p1", "tenant", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fetch error", func(t *testing.T) {
		tx := &stubTx{
			row:     &stubRow{vals: []any{"e1"}},
			row2Err: errors.New("row fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateScopeSubscription(context.Background(), "t1", "S2601", "jobcatalog", "p1", "tenant", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSetIDPGStore_GetScopeSubscription(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		tx := &stubTx{rowErr: pgx.ErrNoRows}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetScopeSubscription(context.Background(), "t1", "S2601", "jobcatalog", "2026-01-01"); err == nil || !strings.Contains(err.Error(), "SCOPE_SUBSCRIPTION_MISSING") {
			t.Fatalf("expected missing err, got %v", err)
		}
	})

	t.Run("fetch error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("boom")}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetScopeSubscription(context.Background(), "t1", "S2601", "jobcatalog", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &stubTx{
			row: scopeSubscriptionRow("S2601", "jobcatalog", "p1", "t1", "2026-01-01", ""),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		sub, err := store.GetScopeSubscription(context.Background(), "t1", "S2601", "jobcatalog", "2026-01-01")
		if err != nil || sub.PackageOwner != "tenant" {
			t.Fatalf("sub=%+v err=%v", sub, err)
		}
	})
}

func TestSetIDPGStore_CreateGlobalScopePackage(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin fail")
		})}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("tenant id error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("row fail")}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config current_tenant error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{"gt1"}},
			execErr:   errors.New("exec fail"),
			execErrAt: 1,
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config allow_share_read error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{"gt1"}},
			execErr:   errors.New("exec fail"),
			execErrAt: 2,
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config actor_scope error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{"gt1"}},
			execErr:   errors.New("exec fail"),
			execErrAt: 3,
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("package id error", func(t *testing.T) {
		tx := &stubTx{
			row:     &stubRow{vals: []any{"gt1"}},
			row2Err: errors.New("row fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event id error", func(t *testing.T) {
		tx := &stubTx{
			row:     &stubRow{vals: []any{"gt1"}},
			row2:    &stubRow{vals: []any{"p1"}},
			row3Err: errors.New("row fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{"gt1"}},
			row2:      &stubRow{vals: []any{"p1"}},
			row3:      &stubRow{vals: []any{"e1"}},
			execErr:   errors.New("exec fail"),
			execErrAt: 4,
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fetch error", func(t *testing.T) {
		tx := &stubTx{
			row:     &stubRow{vals: []any{"gt1"}},
			row2:    &stubRow{vals: []any{"p1"}},
			row3:    &stubRow{vals: []any{"e1"}},
			row4Err: errors.New("fetch fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fetch missing uses request id", func(t *testing.T) {
		tx := &seqTx{
			stubTx: &stubTx{},
			rows: []pgx.Row{
				&stubRow{vals: []any{"gt1"}},
				&stubRow{vals: []any{"p1"}},
				&stubRow{vals: []any{"e1"}},
				&stubRow{err: pgx.ErrNoRows},
				&stubRow{vals: []any{"p2"}},
				scopePackageRow("p2", "jobcatalog", "PKG2", "Pkg2", "active"),
			},
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		pkg, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas")
		if err != nil || pkg.PackageID != "p2" {
			t.Fatalf("pkg=%+v err=%v", pkg, err)
		}
	})

	t.Run("fetch missing existing id error", func(t *testing.T) {
		tx := &seqTx{
			stubTx: &stubTx{},
			rows: []pgx.Row{
				&stubRow{vals: []any{"gt1"}},
				&stubRow{vals: []any{"p1"}},
				&stubRow{vals: []any{"e1"}},
				&stubRow{err: pgx.ErrNoRows},
				&stubRow{err: errors.New("row fail")},
			},
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fetch missing second fetch error", func(t *testing.T) {
		tx := &seqTx{
			stubTx: &stubTx{},
			rows: []pgx.Row{
				&stubRow{vals: []any{"gt1"}},
				&stubRow{vals: []any{"p1"}},
				&stubRow{vals: []any{"e1"}},
				&stubRow{err: pgx.ErrNoRows},
				&stubRow{vals: []any{"p2"}},
				&stubRow{err: errors.New("fetch fail")},
			},
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{"gt1"}},
			row2:      &stubRow{vals: []any{"p1"}},
			row3:      &stubRow{vals: []any{"e1"}},
			row4:      scopePackageRow("p1", "jobcatalog", "PKG1", "Pkg", "active"),
			commitErr: errors.New("commit fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &stubTx{
			row:  &stubRow{vals: []any{"gt1"}},
			row2: &stubRow{vals: []any{"p1"}},
			row3: &stubRow{vals: []any{"e1"}},
			row4: scopePackageRow("p1", "jobcatalog", "PKG1", "Pkg", "active"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		pkg, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "saas")
		if err != nil || pkg.PackageID != "p1" {
			t.Fatalf("pkg=%+v err=%v", pkg, err)
		}
	})
}

func TestSetIDPGStore_ListGlobalScopePackages(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin fail")
		})}
		if _, err := store.ListGlobalScopePackages(context.Background(), "jobcatalog"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("tenant id error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("row fail")}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListGlobalScopePackages(context.Background(), "jobcatalog"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{"gt1"}},
			execErr:   errors.New("exec fail"),
			execErrAt: 1,
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListGlobalScopePackages(context.Background(), "jobcatalog"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config allow_share_read error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{"gt1"}},
			execErr:   errors.New("exec fail"),
			execErrAt: 2,
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListGlobalScopePackages(context.Background(), "jobcatalog"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{
			row:      &stubRow{vals: []any{"gt1"}},
			queryErr: errors.New("query fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListGlobalScopePackages(context.Background(), "jobcatalog"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		tx := &stubTx{
			row:  &stubRow{vals: []any{"gt1"}},
			rows: &scanErrRows{},
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListGlobalScopePackages(context.Background(), "jobcatalog"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows error", func(t *testing.T) {
		tx := &stubTx{
			row:  &stubRow{vals: []any{"gt1"}},
			rows: &tableRows{rows: [][]any{}, err: errors.New("rows err")},
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListGlobalScopePackages(context.Background(), "jobcatalog"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{"gt1"}},
			rows:      &tableRows{rows: [][]any{}},
			commitErr: errors.New("commit fail"),
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListGlobalScopePackages(context.Background(), "jobcatalog"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &stubTx{
			row: &stubRow{vals: []any{"gt1"}},
			rows: &tableRows{rows: [][]any{
				{"p1", "jobcatalog", "PKG1", "Pkg", "active"},
			}},
		}
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		pkgs, err := store.ListGlobalScopePackages(context.Background(), "jobcatalog")
		if err != nil || len(pkgs) != 1 {
			t.Fatalf("len=%d err=%v", len(pkgs), err)
		}
	})
}

func TestSetIDMemoryStore_ScopePackages(t *testing.T) {
	store := newSetIDMemoryStore()

	codes, err := store.ListScopeCodes(context.Background(), "t1")
	if err != nil || len(codes) == 0 {
		t.Fatalf("len=%d err=%v", len(codes), err)
	}

	p1, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG2", "Pkg2", "2026-01-01", "r1", "p1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	_, _ = store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "Pkg1", "2026-01-01", "r2", "p1")
	pkgs, err := store.ListScopePackages(context.Background(), "t1", "jobcatalog")
	if err != nil || len(pkgs) != 2 || pkgs[0].PackageCode != "PKG1" {
		t.Fatalf("pkgs=%+v err=%v", pkgs, err)
	}

	if _, err := store.DisableScopePackage(context.Background(), "t1", p1.PackageID, "r3", "p1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.DisableScopePackage(context.Background(), "t1", "missing", "r3", "p1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := store.CreateScopeSubscription(context.Background(), "t1", "S2601", "jobcatalog", "p1", "tenant", "2026-01-01", "r1", "p1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.GetScopeSubscription(context.Background(), "t1", "S2601", "jobcatalog", "2026-01-01"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.GetScopeSubscription(context.Background(), "t1", "MISSING", "jobcatalog", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg", "2026-01-01", "r1", "p1", "nope"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG2", "Pkg2", "2026-01-01", "r1", "p1", "saas"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.CreateGlobalScopePackage(context.Background(), "jobcatalog", "PKG1", "Pkg1", "2026-01-01", "r2", "p1", "saas"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if pkgs, err := store.ListGlobalScopePackages(context.Background(), "jobcatalog"); err != nil || len(pkgs) != 2 || pkgs[0].PackageCode != "PKG1" {
		t.Fatalf("pkgs=%+v err=%v", pkgs, err)
	}
}
