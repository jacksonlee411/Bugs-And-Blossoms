package dict_test

import (
	"context"
	"testing"

	dict "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
)

type resolverStub struct{}

func (resolverStub) ResolveValueLabel(context.Context, string, string, string, string) (string, bool, error) {
	return "部门", true, nil
}

func (resolverStub) ListOptions(context.Context, string, string, string, string, int) ([]dict.Option, error) {
	return []dict.Option{{Code: "10", Label: "部门"}}, nil
}

func registerResolverStub(t testing.TB) {
	t.Helper()
	if err := dict.RegisterResolver(resolverStub{}); err != nil {
		t.Fatalf("register resolver err=%v", err)
	}
}

func TestRegisterResolverAndResolveValueLabel_BlackBox(t *testing.T) {
	registerResolverStub(t)

	label, ok, err := dict.ResolveValueLabel(context.Background(), " t1 ", " 2026-01-01 ", " org_type ", " 10 ")
	if err != nil || !ok || label != "部门" {
		t.Fatalf("label=%q ok=%v err=%v", label, ok, err)
	}
}

func TestListOptions_BlackBox(t *testing.T) {
	registerResolverStub(t)

	options, err := dict.ListOptions(context.Background(), " t1 ", " 2026-01-01 ", " org_type ", " ", 10)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(options) != 1 || options[0].Code != "10" {
		t.Fatalf("options=%+v", options)
	}
}

func BenchmarkResolveValueLabel_BlackBox(b *testing.B) {
	registerResolverStub(b)
	for i := 0; i < b.N; i++ {
		if _, _, err := dict.ResolveValueLabel(context.Background(), "t1", "2026-01-01", "org_type", "10"); err != nil {
			b.Fatalf("err=%v", err)
		}
	}
}

func BenchmarkListOptions_BlackBox(b *testing.B) {
	registerResolverStub(b)
	for i := 0; i < b.N; i++ {
		if _, err := dict.ListOptions(context.Background(), "t1", "2026-01-01", "org_type", "", 10); err != nil {
			b.Fatalf("err=%v", err)
		}
	}
}
