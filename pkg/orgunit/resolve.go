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
	ErrOrgIDNotFound   = errors.New("org_id_not_found")
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
	if !orgCodePattern.MatchString(normalized) {
		return "", ErrOrgCodeInvalid
	}
	return normalized, nil
}

func ResolveOrgID(ctx context.Context, tx pgx.Tx, tenantUUID string, orgCode string) (int, error) {
	normalized, err := NormalizeOrgCode(orgCode)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `SELECT orgunit.assert_current_tenant($1::uuid);`, tenantUUID); err != nil {
		return 0, err
	}
	var orgID int
	if err := tx.QueryRow(ctx, `
SELECT org_id
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid AND org_code = $2::text
`, tenantUUID, normalized).Scan(&orgID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrOrgCodeNotFound
		}
		return 0, err
	}
	return orgID, nil
}

func ResolveOrgCode(ctx context.Context, tx pgx.Tx, tenantUUID string, orgID int) (string, error) {
	if _, err := tx.Exec(ctx, `SELECT orgunit.assert_current_tenant($1::uuid);`, tenantUUID); err != nil {
		return "", err
	}
	var orgCode string
	if err := tx.QueryRow(ctx, `
SELECT org_code
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid AND org_id = $2::int
`, tenantUUID, orgID).Scan(&orgCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrOrgIDNotFound
		}
		return "", err
	}
	return orgCode, nil
}

func ResolveOrgCodes(ctx context.Context, tx pgx.Tx, tenantUUID string, orgIDs []int) (map[int]string, error) {
	out := make(map[int]string)
	if len(orgIDs) == 0 {
		return out, nil
	}
	if _, err := tx.Exec(ctx, `SELECT orgunit.assert_current_tenant($1::uuid);`, tenantUUID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
SELECT org_id, org_code
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid AND org_id = ANY($2::int[])
`, tenantUUID, orgIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var orgID int
		var orgCode string
		if err := rows.Scan(&orgID, &orgCode); err != nil {
			return nil, err
		}
		out[orgID] = orgCode
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
