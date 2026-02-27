package dict

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"time"
)

var errResolverNotConfigured = errors.New("dict: resolver not configured")

type Option struct {
	Code        string
	Label       string
	SetID       string
	SetIDSource string
	Status      string
	EnabledOn   string
	DisabledOn  *string
	UpdatedAt   time.Time
}

type Resolver interface {
	ResolveValueLabel(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error)
	ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]Option, error)
}

var registry = struct {
	mu sync.RWMutex
	r  Resolver
}{}

func RegisterResolver(r Resolver) error {
	if r == nil {
		return errors.New("dict: resolver is nil")
	}
	v := reflect.ValueOf(r)
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface || v.Kind() == reflect.Slice || v.Kind() == reflect.Map || v.Kind() == reflect.Func || v.Kind() == reflect.Chan) && v.IsNil() {
		return errors.New("dict: resolver is nil")
	}
	registry.mu.Lock()
	registry.r = r
	registry.mu.Unlock()
	return nil
}

func ResolveValueLabel(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	resolver, err := currentResolver()
	if err != nil {
		return "", false, err
	}
	return resolver.ResolveValueLabel(
		ctx,
		strings.TrimSpace(tenantID),
		strings.TrimSpace(asOf),
		strings.TrimSpace(dictCode),
		strings.TrimSpace(code),
	)
}

func ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]Option, error) {
	resolver, err := currentResolver()
	if err != nil {
		return nil, err
	}
	return resolver.ListOptions(
		ctx,
		strings.TrimSpace(tenantID),
		strings.TrimSpace(asOf),
		strings.TrimSpace(dictCode),
		strings.TrimSpace(keyword),
		limit,
	)
}

func currentResolver() (Resolver, error) {
	registry.mu.RLock()
	r := registry.r
	registry.mu.RUnlock()
	if r == nil {
		return nil, errResolverNotConfigured
	}
	return r, nil
}
