package dict

import (
	"context"
	"testing"
)

type benchmarkResolver struct{}

func (benchmarkResolver) ResolveValueLabel(_ context.Context, _ string, _ string, dictCode string, code string) (string, bool, error) {
	if dictCode == "org_type" && code == "10" {
		return "Department", true, nil
	}
	return "", false, nil
}

func (benchmarkResolver) ListOptions(_ context.Context, _ string, _ string, _ string, _ string, limit int) ([]Option, error) {
	if limit <= 0 {
		return []Option{}, nil
	}
	return []Option{
		{Code: "10", Label: "Department", Status: "active", EnabledOn: "1970-01-01"},
		{Code: "20", Label: "Division", Status: "active", EnabledOn: "1970-01-01"},
	}, nil
}

var (
	benchmarkLabel string
	benchmarkOK    bool
	benchmarkOpts  []Option
)

func BenchmarkResolveValueLabel(b *testing.B) {
	if err := RegisterResolver(benchmarkResolver{}); err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	for b.Loop() {
		label, ok, err := ResolveValueLabel(ctx, "tenant-a", "2026-02-20", "org_type", "10")
		if err != nil {
			b.Fatal(err)
		}
		benchmarkLabel = label
		benchmarkOK = ok
	}
}

func BenchmarkListOptions(b *testing.B) {
	if err := RegisterResolver(benchmarkResolver{}); err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	for b.Loop() {
		opts, err := ListOptions(ctx, "tenant-a", "2026-02-20", "org_type", "dep", 20)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkOpts = opts
	}
}
