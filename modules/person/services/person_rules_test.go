package services

import (
	"context"
	"testing"
)

type storeStub struct {
	createFn func(ctx context.Context, tenantID string, pernr string, displayName string) (Person, error)
	findFn   func(ctx context.Context, tenantID string, pernr string) (Person, error)
}

func (s storeStub) CreatePerson(ctx context.Context, tenantID string, pernr string, displayName string) (Person, error) {
	return s.createFn(ctx, tenantID, pernr, displayName)
}

func (s storeStub) FindPersonByPernr(ctx context.Context, tenantID string, pernr string) (Person, error) {
	return s.findFn(ctx, tenantID, pernr)
}

func TestNormalizePernr(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if _, err := NormalizePernr(""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		if _, err := NormalizePernr("A"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("too long", func(t *testing.T) {
		if _, err := NormalizePernr("123456789"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("trim leading zeros", func(t *testing.T) {
		got, err := NormalizePernr("00012")
		if err != nil {
			t.Fatal(err)
		}
		if got != "12" {
			t.Fatalf("expected 12, got %q", got)
		}
	})

	t.Run("zero", func(t *testing.T) {
		got, err := NormalizePernr("00000000")
		if err != nil {
			t.Fatal(err)
		}
		if got != "0" {
			t.Fatalf("expected 0, got %q", got)
		}
	})
}

func TestFacade_CreatePerson(t *testing.T) {
	t.Run("invalid pernr", func(t *testing.T) {
		called := false
		facade := NewFacade(storeStub{
			createFn: func(context.Context, string, string, string) (Person, error) {
				called = true
				return Person{}, nil
			},
		})
		if _, err := facade.CreatePerson(context.Background(), "t1", "BAD", "A"); err == nil {
			t.Fatal("expected error")
		}
		if called {
			t.Fatal("unexpected store call")
		}
	})

	t.Run("missing display name", func(t *testing.T) {
		called := false
		facade := NewFacade(storeStub{
			createFn: func(context.Context, string, string, string) (Person, error) {
				called = true
				return Person{}, nil
			},
		})
		if _, err := facade.CreatePerson(context.Background(), "t1", "1", " "); err == nil {
			t.Fatal("expected error")
		}
		if called {
			t.Fatal("unexpected store call")
		}
	})

	t.Run("normalizes before store", func(t *testing.T) {
		facade := NewFacade(storeStub{
			createFn: func(_ context.Context, tenantID string, pernr string, displayName string) (Person, error) {
				if tenantID != "t1" {
					t.Fatalf("tenantID=%q", tenantID)
				}
				if pernr != "1" {
					t.Fatalf("pernr=%q", pernr)
				}
				if displayName != "A" {
					t.Fatalf("displayName=%q", displayName)
				}
				return Person{UUID: "p1", Pernr: pernr, DisplayName: displayName, Status: "active"}, nil
			},
		})
		p, err := facade.CreatePerson(context.Background(), "t1", "0001", " A ")
		if err != nil {
			t.Fatal(err)
		}
		if p.Pernr != "1" || p.DisplayName != "A" {
			t.Fatalf("person=%+v", p)
		}
	})
}

func TestFacade_FindPersonByPernr(t *testing.T) {
	t.Run("invalid pernr", func(t *testing.T) {
		called := false
		facade := NewFacade(storeStub{
			findFn: func(context.Context, string, string) (Person, error) {
				called = true
				return Person{}, nil
			},
		})
		if _, err := facade.FindPersonByPernr(context.Background(), "t1", "BAD"); err == nil {
			t.Fatal("expected error")
		}
		if called {
			t.Fatal("unexpected store call")
		}
	})

	t.Run("normalizes before store", func(t *testing.T) {
		facade := NewFacade(storeStub{
			findFn: func(_ context.Context, tenantID string, pernr string) (Person, error) {
				if tenantID != "t1" {
					t.Fatalf("tenantID=%q", tenantID)
				}
				if pernr != "1" {
					t.Fatalf("pernr=%q", pernr)
				}
				return Person{UUID: "p1", Pernr: pernr, DisplayName: "A", Status: "active"}, nil
			},
		})
		p, err := facade.FindPersonByPernr(context.Background(), "t1", "0001")
		if err != nil {
			t.Fatal(err)
		}
		if p.UUID != "p1" {
			t.Fatalf("person=%+v", p)
		}
	})
}
