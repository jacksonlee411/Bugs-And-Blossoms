package server

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Tenant struct {
	ID     string
	Domain string
	Name   string
}

type TenancyResolver interface {
	ResolveTenant(ctx context.Context, hostname string) (Tenant, bool, error)
}

type staticTenancyResolver struct {
	tenants map[string]Tenant
}

func newStaticTenancyResolver(tenants map[string]Tenant) TenancyResolver {
	m := make(map[string]Tenant, len(tenants))
	for k, v := range tenants {
		m[strings.ToLower(strings.TrimSpace(k))] = v
	}
	return &staticTenancyResolver{tenants: m}
}

func (r *staticTenancyResolver) ResolveTenant(_ context.Context, hostname string) (Tenant, bool, error) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "" {
		return Tenant{}, false, nil
	}
	t, ok := r.tenants[hostname]
	return t, ok, nil
}

type tenancyDBResolver struct {
	q queryRower
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func newTenancyDBResolver(pool *pgxpool.Pool) TenancyResolver {
	return &tenancyDBResolver{q: pool}
}

func (r *tenancyDBResolver) ResolveTenant(ctx context.Context, hostname string) (Tenant, bool, error) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "" {
		return Tenant{}, false, nil
	}

	var tenantID string
	var tenantName string

	err := r.q.QueryRow(ctx, `
SELECT t.id::text, t.name
FROM iam.tenant_domains d
JOIN iam.tenants t ON t.id = d.tenant_id
WHERE d.hostname = $1
  AND t.is_active = true
LIMIT 1
`, hostname).Scan(&tenantID, &tenantName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tenant{}, false, nil
		}
		return Tenant{}, false, err
	}
	return Tenant{ID: tenantID, Domain: hostname, Name: tenantName}, true, nil
}
