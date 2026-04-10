package orgunit

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	ErrOrgCodeInvalid  = errors.New("org_code_invalid")
	ErrOrgCodeNotFound = errors.New("org_code_not_found")
)

var (
	orgCodePattern      = regexp.MustCompile(`^[\t\x20-\x7E\x{3000}-\x{303F}\x{FF01}-\x{FF60}\x{FFE0}-\x{FFEE}]{1,64}$`)
	orgCodeWhitespaceRE = regexp.MustCompile(`^[\t\x20\x{3000}]+$`)
)

func NormalizeOrgCode(input string) (string, error) {
	if input == "" {
		return "", ErrOrgCodeInvalid
	}
	if !orgCodePattern.MatchString(input) {
		return "", ErrOrgCodeInvalid
	}
	if orgCodeWhitespaceRE.MatchString(input) {
		return "", ErrOrgCodeInvalid
	}
	normalized := strings.ToUpper(input)
	return normalized, nil
}

func ResolveOrgNodeKeyByCode(ctx context.Context, tx pgx.Tx, tenantUUID string, orgCode string) (string, error) {
	normalized, err := NormalizeOrgCode(orgCode)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `SELECT orgunit.assert_current_tenant($1::uuid);`, tenantUUID); err != nil {
		return "", err
	}
	var orgNodeKey string
	if err := tx.QueryRow(ctx, `
SELECT orgunit.encode_org_node_key(org_id::bigint)::text
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid AND org_code = $2::text
	`, tenantUUID, normalized).Scan(&orgNodeKey); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrOrgCodeNotFound
		}
		return "", err
	}
	return NormalizeOrgNodeKey(orgNodeKey)
}

func ResolveOrgCodeByNodeKey(ctx context.Context, tx pgx.Tx, tenantUUID string, orgNodeKey string) (string, error) {
	normalized, err := NormalizeOrgNodeKey(orgNodeKey)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `SELECT orgunit.assert_current_tenant($1::uuid);`, tenantUUID); err != nil {
		return "", err
	}
	var orgCode string
	if err := tx.QueryRow(ctx, `
SELECT org_code
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid
  AND org_id = orgunit.decode_org_node_key($2::char(8))::int
	`, tenantUUID, normalized).Scan(&orgCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrOrgNodeKeyNotFound
		}
		return "", err
	}
	return orgCode, nil
}

func ResolveOrgCodesByNodeKeys(ctx context.Context, tx pgx.Tx, tenantUUID string, orgNodeKeys []string) (map[string]string, error) {
	out := make(map[string]string)
	if len(orgNodeKeys) == 0 {
		return out, nil
	}
	normalized := make([]string, 0, len(orgNodeKeys))
	for _, orgNodeKey := range orgNodeKeys {
		key, err := NormalizeOrgNodeKey(orgNodeKey)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, key)
	}
	if _, err := tx.Exec(ctx, `SELECT orgunit.assert_current_tenant($1::uuid);`, tenantUUID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
WITH wanted AS (
  SELECT DISTINCT
    k AS org_node_key,
    orgunit.decode_org_node_key(k::char(8))::int AS org_id
  FROM unnest($2::text[]) AS t(k)
)
SELECT w.org_node_key, c.org_code
FROM wanted w
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = $1::uuid
 AND c.org_id = w.org_id
	`, tenantUUID, normalized)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var orgNodeKey string
		var orgCode string
		if err := rows.Scan(&orgNodeKey, &orgCode); err != nil {
			return nil, err
		}
		normalizedKey, err := NormalizeOrgNodeKey(orgNodeKey)
		if err != nil {
			return nil, err
		}
		out[normalizedKey] = orgCode
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
