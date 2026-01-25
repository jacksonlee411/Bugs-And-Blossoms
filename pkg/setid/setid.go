package setid

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type QueryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func EnsureBootstrap(ctx context.Context, e Execer, tenantID string, initiatorID string) error {
	_, err := e.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, initiatorID)
	return err
}

func Resolve(ctx context.Context, q QueryRower, tenantID string, orgUnitID string, asOfDate string) (string, error) {
	var out string
	if err := q.QueryRow(
		ctx,
		`SELECT orgunit.resolve_setid($1::uuid, $2::uuid, $3::date);`,
		tenantID,
		orgUnitID,
		asOfDate,
	).Scan(&out); err != nil {
		return "", err
	}
	return out, nil
}
