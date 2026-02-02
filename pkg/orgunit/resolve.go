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

var orgCodePattern = regexp.MustCompile(`^[A-Z0-9_-]{1,16}$`)

func NormalizeOrgCode(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || trimmed != input {
		return "", ErrOrgCodeInvalid
	}
	normalized := strings.ToUpper(trimmed)
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
