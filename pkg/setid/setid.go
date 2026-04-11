package setid

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
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
	orgNodeKey, err := parseOrgUnitID(orgUnitID)
	if err != nil {
		return "", err
	}

	var out string
	if err := q.QueryRow(
		ctx,
		`SELECT orgunit.resolve_setid($1::uuid, $2::char(8), $3::date);`,
		tenantID,
		orgNodeKey,
		asOfDate,
	).Scan(&out); err != nil {
		return "", err
	}
	return out, nil
}

func parseOrgUnitID(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", errors.New("org_node_key is required")
	}
	if orgNodeKey, err := orgunitpkg.NormalizeOrgNodeKey(trimmed); err == nil {
		return orgNodeKey, nil
	}
	if len(trimmed) != 8 {
		return "", errors.New("org_node_key invalid")
	}
	value := int64(0)
	for _, r := range trimmed {
		if r < '0' || r > '9' {
			return "", errors.New("org_node_key invalid")
		}
		value = value*10 + int64(r-'0')
	}
	if value < 10000000 || value > 99999999 {
		return "", errors.New("org_node_key invalid")
	}
	return orgunitpkg.EncodeOrgNodeKey(value)
}
