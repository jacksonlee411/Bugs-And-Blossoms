package dict

import (
	"context"
	"errors"
	"testing"
)

type nilResolver struct{}

func (*nilResolver) ResolveValueLabel(context.Context, string, string, string, string) (string, bool, error) {
	return "", false, nil
}

func (*nilResolver) ListOptions(context.Context, string, string, string, string, int) ([]Option, error) {
	return nil, nil
}

func TestRegisterResolver_RejectsNil_BlackBoxBoundary(t *testing.T) {
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
}

func TestCurrentResolver_NotConfigured(t *testing.T) {
	registry.mu.Lock()
	registry.r = nil
	registry.mu.Unlock()

	if _, _, err := ResolveValueLabel(context.Background(), "t1", "2026-01-01", "org_type", "10"); !errors.Is(err, errResolverNotConfigured) {
		t.Fatalf("err=%v", err)
	}
	if _, err := ListOptions(context.Background(), "t1", "2026-01-01", "org_type", "", 10); !errors.Is(err, errResolverNotConfigured) {
		t.Fatalf("err=%v", err)
	}
}
