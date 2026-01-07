package server

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

type stubQueryRower struct {
	row pgx.Row
}

func (s stubQueryRower) QueryRow(context.Context, string, ...any) pgx.Row { return s.row }

func TestTenancyDBResolver_ResolveTenant(t *testing.T) {
	r := &tenancyDBResolver{
		q: stubQueryRower{row: &stubRow{vals: []any{"tid", "Tenant"}}},
	}
	got, ok, err := r.ResolveTenant(context.Background(), "HOST.local")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok")
	}
	if got.ID != "tid" || got.Name != "Tenant" || got.Domain != "host.local" {
		t.Fatalf("got=%+v", got)
	}
}

func TestTenancyDBResolver_ResolveTenant_NotFound(t *testing.T) {
	r := &tenancyDBResolver{
		q: stubQueryRower{row: &stubRow{err: pgx.ErrNoRows}},
	}
	_, ok, err := r.ResolveTenant(context.Background(), "missing.local")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected not found")
	}
}

func TestTenancyDBResolver_ResolveTenant_Error(t *testing.T) {
	r := &tenancyDBResolver{
		q: stubQueryRower{row: &stubRow{err: errors.New("boom")}},
	}
	_, _, err := r.ResolveTenant(context.Background(), "x.local")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTenancyDBResolver_ResolveTenant_EmptyHostname(t *testing.T) {
	r := &tenancyDBResolver{
		q: stubQueryRower{row: &stubRow{err: errors.New("should not query")}},
	}
	_, ok, err := r.ResolveTenant(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected not found")
	}
}
