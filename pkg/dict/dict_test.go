package dict

import (
	"context"
	"errors"
	"testing"
)

type resolverStub struct{}

func (resolverStub) ResolveValueLabel(context.Context, string, string, string, string) (string, bool, error) {
	return "部门", true, nil
}

func (resolverStub) ListOptions(context.Context, string, string, string, string, int) ([]Option, error) {
	return []Option{{Code: "10", Label: "部门"}}, nil
}

type nilResolver struct{}

func (*nilResolver) ResolveValueLabel(context.Context, string, string, string, string) (string, bool, error) {
	return "", false, nil
}

func (*nilResolver) ListOptions(context.Context, string, string, string, string, int) ([]Option, error) {
	return nil, nil
}

func TestResolverRegistry(t *testing.T) {
	registry.mu.Lock()
	registry.r = nil
	registry.mu.Unlock()

	if err := RegisterResolver(nil); err == nil {
		t.Fatal("expected error")
	}
	var typedNil *nilResolver
	if err := RegisterResolver(typedNil); err == nil {
		t.Fatal("expected typed nil error")
	}
	if _, _, err := ResolveValueLabel(context.Background(), "t1", "2026-01-01", "org_type", "10"); !errors.Is(err, errResolverNotConfigured) {
		t.Fatalf("err=%v", err)
	}
	if _, err := ListOptions(context.Background(), "t1", "2026-01-01", "org_type", "", 10); !errors.Is(err, errResolverNotConfigured) {
		t.Fatalf("err=%v", err)
	}

	if err := RegisterResolver(resolverStub{}); err != nil {
		t.Fatalf("register err=%v", err)
	}
	label, ok, err := ResolveValueLabel(context.Background(), " t1 ", " 2026-01-01 ", " org_type ", " 10 ")
	if err != nil || !ok || label != "部门" {
		t.Fatalf("label=%q ok=%v err=%v", label, ok, err)
	}
	options, err := ListOptions(context.Background(), " t1 ", " 2026-01-01 ", " org_type ", " ", 10)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(options) != 1 || options[0].Code != "10" {
		t.Fatalf("options=%+v", options)
	}
}
