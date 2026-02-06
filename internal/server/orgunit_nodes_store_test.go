package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type timePtrRow struct {
	value *time.Time
	err   error
}

func (r timePtrRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) == 0 {
		return nil
	}
	if ptr, ok := dest[0].(**time.Time); ok {
		*ptr = r.value
		return nil
	}
	return errors.New("unsupported scan type")
}

func TestOrgUnitPGStore_MaxEffectiveDateOnOrBefore(t *testing.T) {
	ctx := context.Background()
	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		if _, _, err := store.MaxEffectiveDateOnOrBefore(ctx, "t1", "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, _, err := store.MaxEffectiveDateOnOrBefore(ctx, "t1", "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("row")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, _, err := store.MaxEffectiveDateOnOrBefore(ctx, "t1", "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("commit error", func(t *testing.T) {
		when := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
		tx := &stubTx{row: timePtrRow{value: &when}, commitErr: errors.New("commit")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, _, err := store.MaxEffectiveDateOnOrBefore(ctx, "t1", "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("nil value", func(t *testing.T) {
		tx := &stubTx{row: timePtrRow{value: nil}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		value, ok, err := store.MaxEffectiveDateOnOrBefore(ctx, "t1", "2026-01-06")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || value != "" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
	})
	t.Run("success", func(t *testing.T) {
		when := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
		tx := &stubTx{row: timePtrRow{value: &when}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		value, ok, err := store.MaxEffectiveDateOnOrBefore(ctx, "t1", "2026-01-06")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != "2026-01-05" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
	})
}

func TestOrgUnitPGStore_MinEffectiveDate(t *testing.T) {
	ctx := context.Background()
	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		if _, _, err := store.MinEffectiveDate(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, _, err := store.MinEffectiveDate(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("row")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, _, err := store.MinEffectiveDate(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("commit error", func(t *testing.T) {
		when := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
		tx := &stubTx{row: timePtrRow{value: &when}, commitErr: errors.New("commit")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, _, err := store.MinEffectiveDate(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("nil value", func(t *testing.T) {
		tx := &stubTx{row: timePtrRow{value: nil}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		value, ok, err := store.MinEffectiveDate(ctx, "t1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || value != "" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
	})
	t.Run("success", func(t *testing.T) {
		when := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
		tx := &stubTx{row: timePtrRow{value: &when}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		value, ok, err := store.MinEffectiveDate(ctx, "t1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != "2026-01-05" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
	})
}

func TestOrgUnitMemoryStore_MaxEffectiveDateOnOrBefore(t *testing.T) {
	ctx := context.Background()
	t.Run("empty", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		value, ok, err := store.MaxEffectiveDateOnOrBefore(ctx, "t1", "2026-01-06")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || value != "" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
	})
	t.Run("invalid date", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-06", "A001", "Root", "", false); err != nil {
			t.Fatal(err)
		}
		if _, _, err := store.MaxEffectiveDateOnOrBefore(ctx, "t1", "bad"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("success", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-06", "A001", "Root", "", false); err != nil {
			t.Fatal(err)
		}
		value, ok, err := store.MaxEffectiveDateOnOrBefore(ctx, "t1", "2026-01-06")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != "2026-01-06" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
	})
}

func TestOrgUnitMemoryStore_MinEffectiveDate(t *testing.T) {
	ctx := context.Background()
	t.Run("empty", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		value, ok, err := store.MinEffectiveDate(ctx, "t1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || value != "" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
	})
	t.Run("success", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		fixed := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)
		store.now = func() time.Time { return fixed }
		if _, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-06", "A001", "Root", "", false); err != nil {
			t.Fatal(err)
		}
		value, ok, err := store.MinEffectiveDate(ctx, "t1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || value != "2026-01-07" {
			t.Fatalf("value=%q ok=%v", value, ok)
		}
	})
}
