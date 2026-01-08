package superadmin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type stubDB struct {
	execFn     func(sql string, args ...any) error
	queryRowFn func(sql string, args ...any) pgx.Row
}

func (s stubDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if s.execFn == nil {
		return pgconn.CommandTag{}, nil
	}
	return pgconn.CommandTag{}, s.execFn(sql, args...)
}

func (s stubDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if s.queryRowFn == nil {
		return stubRow{err: errors.New("unexpected QueryRow")}
	}
	return s.queryRowFn(sql, args...)
}

func TestPGPrincipalStore_UpsertFromKratos(t *testing.T) {
	cases := []struct {
		name    string
		row     pgx.Row
		wantErr bool
	}{
		{name: "ok", row: stubRow{vals: []any{"p1", "active", "kid-1"}}},
		{name: "query_error", row: stubRow{err: errors.New("db")}, wantErr: true},
		{name: "disabled", row: stubRow{vals: []any{"p1", "disabled", "kid-1"}}, wantErr: true},
		{name: "missing_kid", row: stubRow{vals: []any{"p1", "active", ""}}, wantErr: true},
		{name: "kid_mismatch", row: stubRow{vals: []any{"p1", "active", "kid-2"}}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newPrincipalStoreFromDB(stubDB{
				queryRowFn: func(string, ...any) pgx.Row { return tc.row },
			})
			_, err := store.UpsertFromKratos(context.Background(), "admin@example.invalid", "kid-1")
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPGPrincipalStore_GetByID(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		store := newPrincipalStoreFromDB(stubDB{
			queryRowFn: func(string, ...any) pgx.Row {
				return stubRow{vals: []any{"p1", "admin@example.invalid", "active", "kid-1"}}
			},
		})
		p, ok, err := store.GetByID(context.Background(), "p1")
		if err != nil || !ok || p.ID != "p1" {
			t.Fatalf("ok=%v err=%v p=%+v", ok, err, p)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		store := newPrincipalStoreFromDB(stubDB{
			queryRowFn: func(string, ...any) pgx.Row { return stubRow{err: pgx.ErrNoRows} },
		})
		_, ok, err := store.GetByID(context.Background(), "p1")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("canceled", func(t *testing.T) {
		store := newPrincipalStoreFromDB(stubDB{
			queryRowFn: func(string, ...any) pgx.Row { return stubRow{err: context.Canceled} },
		})
		_, _, err := store.GetByID(context.Background(), "p1")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("other_error", func(t *testing.T) {
		store := newPrincipalStoreFromDB(stubDB{
			queryRowFn: func(string, ...any) pgx.Row { return stubRow{err: errors.New("db")} },
		})
		_, _, err := store.GetByID(context.Background(), "p1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPGSessionStore_CreateLookupRevoke(t *testing.T) {
	store := newSessionStoreFromDB(stubDB{
		execFn: func(string, ...any) error { return nil },
		queryRowFn: func(string, ...any) pgx.Row {
			return stubRow{err: errors.New("unexpected QueryRow")}
		},
	})

	saSid, err := store.Create(context.Background(), "p1", time.Now().Add(time.Hour), "ip", "ua")
	if err != nil {
		t.Fatal(err)
	}
	if saSid == "" {
		t.Fatal("expected sa_sid")
	}

	t.Run("lookup_not_found", func(t *testing.T) {
		store := newSessionStoreFromDB(stubDB{
			queryRowFn: func(string, ...any) pgx.Row { return stubRow{err: pgx.ErrNoRows} },
		})
		_, ok, err := store.Lookup(context.Background(), "x")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("lookup_revoked", func(t *testing.T) {
		now := time.Now()
		store := newSessionStoreFromDB(stubDB{
			queryRowFn: func(string, ...any) pgx.Row {
				return stubRow{vals: []any{"p1", now.Add(time.Hour), &now}}
			},
		})
		_, ok, err := store.Lookup(context.Background(), "x")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("lookup_expired", func(t *testing.T) {
		store := newSessionStoreFromDB(stubDB{
			queryRowFn: func(string, ...any) pgx.Row {
				return stubRow{vals: []any{"p1", time.Now().Add(-time.Second), nil}}
			},
		})
		_, ok, err := store.Lookup(context.Background(), "x")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("lookup_canceled", func(t *testing.T) {
		store := newSessionStoreFromDB(stubDB{
			queryRowFn: func(string, ...any) pgx.Row { return stubRow{err: context.Canceled} },
		})
		_, _, err := store.Lookup(context.Background(), "x")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("lookup_deadline", func(t *testing.T) {
		store := newSessionStoreFromDB(stubDB{
			queryRowFn: func(string, ...any) pgx.Row { return stubRow{err: context.DeadlineExceeded} },
		})
		_, _, err := store.Lookup(context.Background(), "x")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("lookup_ok", func(t *testing.T) {
		store := newSessionStoreFromDB(stubDB{
			queryRowFn: func(string, ...any) pgx.Row {
				return stubRow{vals: []any{"p1", time.Now().Add(time.Hour), nil}}
			},
		})
		_, ok, err := store.Lookup(context.Background(), "x")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("lookup_other_error", func(t *testing.T) {
		store := newSessionStoreFromDB(stubDB{
			queryRowFn: func(string, ...any) pgx.Row { return stubRow{err: errors.New("db")} },
		})
		_, _, err := store.Lookup(context.Background(), "x")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("revoke_empty_noop", func(t *testing.T) {
		if err := store.Revoke(context.Background(), ""); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("revoke_error", func(t *testing.T) {
		store := newSessionStoreFromDB(stubDB{
			execFn: func(string, ...any) error { return errors.New("db") },
		})
		if err := store.Revoke(context.Background(), "x"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPGSessionStore_Create_Errors(t *testing.T) {
	t.Run("rand_error", func(t *testing.T) {
		old := saSidRandReader
		t.Cleanup(func() { saSidRandReader = old })
		saSidRandReader = errReader{}

		store := newSessionStoreFromDB(stubDB{execFn: func(string, ...any) error { return nil }})
		if _, err := store.Create(context.Background(), "p1", time.Now().Add(time.Hour), "ip", "ua"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("exec_error", func(t *testing.T) {
		store := newSessionStoreFromDB(stubDB{execFn: func(string, ...any) error { return errors.New("db") }})
		if _, err := store.Create(context.Background(), "p1", time.Now().Add(time.Hour), "ip", "ua"); err == nil {
			t.Fatal("expected error")
		}
	})
}
