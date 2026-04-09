package persistence

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	jobcatalogtypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/setid"
)

type pgBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type PGStore struct {
	pool pgBeginner
}

func NewPGStore(pool pgBeginner) *PGStore {
	return &PGStore{pool: pool}
}

func (s *PGStore) withTx(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func NormalizeSetID(input string) string {
	return strings.ToUpper(strings.TrimSpace(input))
}

func StampJobProfileSetID(ctx context.Context, tx pgx.Tx, tenantID string, packageUUID string, profileUUID string, setID string) error {
	setID = NormalizeSetID(setID)
	if setID == "" {
		return errors.New("setid is required")
	}
	if _, err := tx.Exec(ctx, `
UPDATE jobcatalog.job_profiles
SET setid = $4::text
WHERE tenant_uuid = $1::uuid
  AND package_uuid = $2::uuid
  AND job_profile_uuid = $3::uuid
  AND COALESCE(setid, '') <> $4::text
`, tenantID, packageUUID, profileUUID, setID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE jobcatalog.job_profile_events
SET setid = $4::text
WHERE tenant_uuid = $1::uuid
  AND package_uuid = $2::uuid
  AND job_profile_uuid = $3::uuid
  AND COALESCE(setid, '') <> $4::text
`, tenantID, packageUUID, profileUUID, setID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE jobcatalog.job_profile_versions
SET setid = $4::text
WHERE tenant_uuid = $1::uuid
  AND package_uuid = $2::uuid
  AND job_profile_uuid = $3::uuid
  AND COALESCE(setid, '') <> $4::text
`, tenantID, packageUUID, profileUUID, setID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE jobcatalog.job_profile_version_job_families pf
SET setid = $4::text
FROM jobcatalog.job_profile_versions v
WHERE pf.tenant_uuid = $1::uuid
  AND pf.package_uuid = $2::uuid
  AND v.tenant_uuid = $1::uuid
  AND v.package_uuid = $2::uuid
  AND v.job_profile_uuid = $3::uuid
  AND pf.job_profile_version_id = v.id
  AND COALESCE(pf.setid, '') <> $4::text
`, tenantID, packageUUID, profileUUID, setID); err != nil {
		return err
	}
	return nil
}

func ensureSetIDActive(ctx context.Context, tx pgx.Tx, tenantID string, setID string) (string, error) {
	setID = NormalizeSetID(setID)
	if setID == "" {
		return "", errors.New("setid is required")
	}
	var status string
	if err := tx.QueryRow(ctx, `
SELECT status
FROM orgunit.setids
WHERE tenant_uuid = $1::uuid
  AND setid = $2::text
`, tenantID, setID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("JOBCATALOG_SETID_INVALID")
		}
		return "", err
	}
	if status != "active" {
		return "", errors.New("JOBCATALOG_SETID_INVALID")
	}
	return setID, nil
}

func resolveJobCatalogPackage(ctx context.Context, tx pgx.Tx, tenantID string, setID string, asOfDate string) (string, error) {
	resolvedSetID, err := ensureSetIDActive(ctx, tx, tenantID, setID)
	if err != nil {
		return "", err
	}

	var packageID string
	var ownerTenantID string
	if err := tx.QueryRow(ctx, `
SELECT package_id::text, package_owner_tenant_uuid::text
FROM orgunit.resolve_scope_package($1::uuid, $2::text, 'jobcatalog', $3::date)
	`, tenantID, resolvedSetID, asOfDate).Scan(&packageID, &ownerTenantID); err != nil {
		return "", err
	}
	if ownerTenantID != tenantID {
		return "", errors.New("JOBCATALOG_PACKAGE_OWNER_INVALID")
	}
	return packageID, nil
}

func ResolveJobCatalogPackageByCode(ctx context.Context, tx pgx.Tx, tenantID string, packageCode string, asOfDate string) (jobcatalogtypes.JobCatalogPackage, error) {
	packageCode = strings.ToUpper(strings.TrimSpace(packageCode))
	if packageCode == "" {
		return jobcatalogtypes.JobCatalogPackage{}, errors.New("PACKAGE_CODE_INVALID")
	}

	var out jobcatalogtypes.JobCatalogPackage
	if err := tx.QueryRow(ctx, `
SELECT package_id::text, owner_setid
FROM orgunit.setid_scope_packages
WHERE tenant_uuid = $1::uuid
  AND scope_code = 'jobcatalog'
  AND package_code = $2::text
`, tenantID, packageCode).Scan(&out.PackageUUID, &out.OwnerSetID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return jobcatalogtypes.JobCatalogPackage{}, errors.New("PACKAGE_NOT_FOUND")
		}
		return jobcatalogtypes.JobCatalogPackage{}, err
	}
	if _, err := tx.Exec(ctx, `
SELECT orgunit.assert_scope_package_active_as_of(
  $1::uuid,
  'jobcatalog',
  $2::uuid,
  $1::uuid,
  $3::date
);
`, tenantID, out.PackageUUID, asOfDate); err != nil {
		return jobcatalogtypes.JobCatalogPackage{}, err
	}
	out.PackageCode = packageCode
	return out, nil
}

func (s *PGStore) ResolveJobCatalogPackageByCode(ctx context.Context, tenantID string, packageCode string, asOfDate string) (jobcatalogtypes.JobCatalogPackage, error) {
	var out jobcatalogtypes.JobCatalogPackage
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		resolved, err := ResolveJobCatalogPackageByCode(ctx, tx, tenantID, packageCode, asOfDate)
		if err != nil {
			return err
		}
		out = resolved
		return nil
	})
	return out, err
}

func (s *PGStore) ResolveJobCatalogPackageBySetID(ctx context.Context, tenantID string, setID string, asOfDate string) (string, error) {
	var out string
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		resolved, err := resolveJobCatalogPackage(ctx, tx, tenantID, setID, asOfDate)
		if err != nil {
			return err
		}
		out = resolved
		return nil
	})
	return out, err
}

func (s *PGStore) CreateJobFamilyGroup(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := setid.EnsureBootstrap(ctx, tx, tenantID, tenantID); err != nil {
			return err
		}

		resolved, err := resolveJobCatalogPackage(ctx, tx, tenantID, setID, effectiveDate)
		if err != nil {
			return err
		}

		var groupID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&groupID); err != nil {
			return err
		}
		var eventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
			return err
		}

		payload := `{"job_family_group_code":` + strconv.Quote(code) +
			`,"name":` + strconv.Quote(name)
		if strings.TrimSpace(description) != "" {
			payload += `,"description":` + strconv.Quote(description)
		} else {
			payload += `,"description":null`
		}
		payload += `}`

		_, err = tx.Exec(ctx, `
SELECT jobcatalog.submit_job_family_group_event(
  $1::uuid,
  $2::uuid,
  $3::text,
  $4::uuid,
  'CREATE',
  $5::date,
  $6::jsonb,
  $7::text,
  $8::uuid
);
`, eventID, tenantID, resolved, groupID, effectiveDate, []byte(payload), "ui:"+eventID, tenantID)
		return err
	})
}

func (s *PGStore) ListJobFamilyGroups(ctx context.Context, tenantID string, setID string, asOfDate string) ([]jobcatalogtypes.JobFamilyGroup, error) {
	var out []jobcatalogtypes.JobFamilyGroup
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := setid.EnsureBootstrap(ctx, tx, tenantID, tenantID); err != nil {
			return err
		}
		v, err := resolveJobCatalogPackage(ctx, tx, tenantID, setID, asOfDate)
		if err != nil {
			return err
		}

		rows, err := tx.Query(ctx, `
SELECT
  g.job_family_group_uuid::text,
  g.job_family_group_code,
  v.name,
  v.is_active,
  lower(v.validity)::text
FROM jobcatalog.job_family_groups g
JOIN jobcatalog.job_family_group_versions v
  ON v.tenant_uuid = $1::uuid
 AND v.package_uuid = $2::uuid
 AND v.job_family_group_uuid = g.job_family_group_uuid
WHERE g.tenant_uuid = $1::uuid
  AND g.package_uuid = $2::uuid
  AND v.validity @> $3::date
ORDER BY g.job_family_group_code ASC
`, tenantID, v, asOfDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var g jobcatalogtypes.JobFamilyGroup
			if err := rows.Scan(&g.JobFamilyGroupUUID, &g.JobFamilyGroupCode, &g.Name, &g.IsActive, &g.EffectiveDay); err != nil {
				return err
			}
			out = append(out, g)
		}
		return rows.Err()
	})
	return out, err
}

func (s *PGStore) CreateJobFamily(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string, groupCode string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := setid.EnsureBootstrap(ctx, tx, tenantID, tenantID); err != nil {
			return err
		}

		resolved, err := resolveJobCatalogPackage(ctx, tx, tenantID, setID, effectiveDate)
		if err != nil {
			return err
		}

		var groupID string
		if err := tx.QueryRow(ctx, `
SELECT g.job_family_group_uuid::text
FROM jobcatalog.job_family_groups g
WHERE g.tenant_uuid = $1::uuid
  AND g.package_uuid = $2::uuid
  AND g.job_family_group_code = $3::text
`, tenantID, resolved, groupCode).Scan(&groupID); err != nil {
			return err
		}

		var familyID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&familyID); err != nil {
			return err
		}
		var eventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
			return err
		}

		payload := `{"job_family_code":` + strconv.Quote(code) +
			`,"name":` + strconv.Quote(name) +
			`,"job_family_group_uuid":` + strconv.Quote(groupID)
		if strings.TrimSpace(description) != "" {
			payload += `,"description":` + strconv.Quote(description)
		} else {
			payload += `,"description":null`
		}
		payload += `}`

		_, err = tx.Exec(ctx, `
SELECT jobcatalog.submit_job_family_event(
  $1::uuid,
  $2::uuid,
  $3::text,
  $4::uuid,
  'CREATE',
  $5::date,
  $6::jsonb,
  $7::text,
  $8::uuid
);
`, eventID, tenantID, resolved, familyID, effectiveDate, []byte(payload), "ui:"+eventID, tenantID)
		return err
	})
}

func (s *PGStore) UpdateJobFamilyGroup(ctx context.Context, tenantID string, setID string, effectiveDate string, familyCode string, groupCode string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := setid.EnsureBootstrap(ctx, tx, tenantID, tenantID); err != nil {
			return err
		}

		resolved, err := resolveJobCatalogPackage(ctx, tx, tenantID, setID, effectiveDate)
		if err != nil {
			return err
		}

		var groupID string
		if err := tx.QueryRow(ctx, `
SELECT g.job_family_group_uuid::text
FROM jobcatalog.job_family_groups g
WHERE g.tenant_uuid = $1::uuid
  AND g.package_uuid = $2::uuid
  AND g.job_family_group_code = $3::text
`, tenantID, resolved, groupCode).Scan(&groupID); err != nil {
			return err
		}

		var familyID string
		if err := tx.QueryRow(ctx, `
SELECT f.job_family_uuid::text
FROM jobcatalog.job_families f
WHERE f.tenant_uuid = $1::uuid
  AND f.package_uuid = $2::uuid
  AND f.job_family_code = $3::text
`, tenantID, resolved, familyCode).Scan(&familyID); err != nil {
			return err
		}

		var eventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
			return err
		}

		payload := `{"job_family_group_uuid":` + strconv.Quote(groupID) + `}`

		_, err = tx.Exec(ctx, `
SELECT jobcatalog.submit_job_family_event(
  $1::uuid,
  $2::uuid,
  $3::text,
  $4::uuid,
  'UPDATE',
  $5::date,
  $6::jsonb,
  $7::text,
  $8::uuid
);
`, eventID, tenantID, resolved, familyID, effectiveDate, []byte(payload), "ui:"+eventID, tenantID)
		return err
	})
}

func (s *PGStore) ListJobFamilies(ctx context.Context, tenantID string, setID string, asOfDate string) ([]jobcatalogtypes.JobFamily, error) {
	var out []jobcatalogtypes.JobFamily
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := setid.EnsureBootstrap(ctx, tx, tenantID, tenantID); err != nil {
			return err
		}
		v, err := resolveJobCatalogPackage(ctx, tx, tenantID, setID, asOfDate)
		if err != nil {
			return err
		}

		rows, err := tx.Query(ctx, `
SELECT
  f.job_family_uuid::text,
  f.job_family_code,
  g.job_family_group_code AS job_family_group_code,
  v.name,
  v.is_active,
  lower(v.validity)::text
FROM jobcatalog.job_families f
JOIN jobcatalog.job_family_versions v
  ON v.tenant_uuid = $1::uuid
 AND v.package_uuid = $2::uuid
 AND v.job_family_uuid = f.job_family_uuid
JOIN jobcatalog.job_family_groups g
  ON g.tenant_uuid = $1::uuid
 AND g.package_uuid = $2::uuid
 AND g.job_family_group_uuid = v.job_family_group_uuid
WHERE f.tenant_uuid = $1::uuid
  AND f.package_uuid = $2::uuid
  AND v.validity @> $3::date
ORDER BY f.job_family_code ASC
`, tenantID, v, asOfDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var f jobcatalogtypes.JobFamily
			if err := rows.Scan(&f.JobFamilyUUID, &f.JobFamilyCode, &f.JobFamilyGroupCode, &f.Name, &f.IsActive, &f.EffectiveDay); err != nil {
				return err
			}
			out = append(out, f)
		}
		return rows.Err()
	})
	return out, err
}

func (s *PGStore) CreateJobLevel(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := setid.EnsureBootstrap(ctx, tx, tenantID, tenantID); err != nil {
			return err
		}

		resolved, err := resolveJobCatalogPackage(ctx, tx, tenantID, setID, effectiveDate)
		if err != nil {
			return err
		}

		var levelID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&levelID); err != nil {
			return err
		}
		var eventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
			return err
		}

		payload := `{"job_level_code":` + strconv.Quote(code) +
			`,"name":` + strconv.Quote(name)
		if strings.TrimSpace(description) != "" {
			payload += `,"description":` + strconv.Quote(description)
		} else {
			payload += `,"description":null`
		}
		payload += `}`

		_, err = tx.Exec(ctx, `
	SELECT jobcatalog.submit_job_level_event(
	  $1::uuid,
	  $2::uuid,
	  $3::text,
	  $4::uuid,
	  'CREATE',
	  $5::date,
	  $6::jsonb,
	  $7::text,
	  $8::uuid
	);
	`, eventID, tenantID, resolved, levelID, effectiveDate, []byte(payload), "ui:"+eventID, tenantID)
		return err
	})
}

func (s *PGStore) ListJobLevels(ctx context.Context, tenantID string, setID string, asOfDate string) ([]jobcatalogtypes.JobLevel, error) {
	var out []jobcatalogtypes.JobLevel
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := setid.EnsureBootstrap(ctx, tx, tenantID, tenantID); err != nil {
			return err
		}
		v, err := resolveJobCatalogPackage(ctx, tx, tenantID, setID, asOfDate)
		if err != nil {
			return err
		}

		rows, err := tx.Query(ctx, `
	SELECT
	  l.job_level_uuid::text,
	  l.job_level_code,
	  v.name,
	  v.is_active,
	  lower(v.validity)::text
	FROM jobcatalog.job_levels l
	JOIN jobcatalog.job_level_versions v
	  ON v.tenant_uuid = $1::uuid
	 AND v.package_uuid = $2::uuid
	 AND v.job_level_uuid = l.job_level_uuid
	WHERE l.tenant_uuid = $1::uuid
	  AND l.package_uuid = $2::uuid
	  AND v.validity @> $3::date
	ORDER BY l.job_level_code ASC
	`, tenantID, v, asOfDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var l jobcatalogtypes.JobLevel
			if err := rows.Scan(&l.JobLevelUUID, &l.JobLevelCode, &l.Name, &l.IsActive, &l.EffectiveDay); err != nil {
				return err
			}
			out = append(out, l)
		}
		return rows.Err()
	})
	return out, err
}

func (s *PGStore) CreateJobProfile(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string, familyCodes []string, primaryFamilyCode string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := setid.EnsureBootstrap(ctx, tx, tenantID, tenantID); err != nil {
			return err
		}

		resolved, err := resolveJobCatalogPackage(ctx, tx, tenantID, setID, effectiveDate)
		if err != nil {
			return err
		}

		type familyRow struct {
			Code string
			ID   string
		}

		lookupCodes := append([]string(nil), familyCodes...)
		if primaryFamilyCode != "" {
			found := slices.Contains(lookupCodes, primaryFamilyCode)
			if !found {
				lookupCodes = append(lookupCodes, primaryFamilyCode)
			}
		}

		familyByCode := make(map[string]string, len(lookupCodes))
		if len(lookupCodes) > 0 {
			rows, err := tx.Query(ctx, `
SELECT f.job_family_code, f.job_family_uuid::text
FROM jobcatalog.job_families f
WHERE f.tenant_uuid = $1::uuid
  AND f.package_uuid = $2::uuid
  AND f.job_family_code = ANY($3::text[])
`, tenantID, resolved, lookupCodes)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var r familyRow
				if err := rows.Scan(&r.Code, &r.ID); err != nil {
					return err
				}
				familyByCode[r.Code] = r.ID
			}
			if err := rows.Err(); err != nil {
				return err
			}
		}

		familyIDs := make([]string, 0, len(familyCodes))
		for _, c := range familyCodes {
			id, ok := familyByCode[c]
			if !ok {
				return errors.New("job family not found: " + c)
			}
			familyIDs = append(familyIDs, id)
		}
		primaryID, ok := familyByCode[primaryFamilyCode]
		if !ok {
			return errors.New("job family not found: " + primaryFamilyCode)
		}

		var profileID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&profileID); err != nil {
			return err
		}
		var eventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
			return err
		}

		payload := `{"job_profile_code":` + strconv.Quote(code) +
			`,"name":` + strconv.Quote(name) +
			`,"job_family_uuids":[` + strings.Join(quoteAll(familyIDs), ",") + `]` +
			`,"primary_job_family_uuid":` + strconv.Quote(primaryID)
		if strings.TrimSpace(description) != "" {
			payload += `,"description":` + strconv.Quote(description)
		} else {
			payload += `,"description":null`
		}
		payload += `}`

		_, err = tx.Exec(ctx, `
SELECT jobcatalog.submit_job_profile_event(
  $1::uuid,
  $2::uuid,
  $3::text,
  $4::uuid,
  'CREATE',
  $5::date,
  $6::jsonb,
  $7::text,
  $8::uuid
);
`, eventID, tenantID, resolved, profileID, effectiveDate, []byte(payload), "ui:"+eventID, tenantID)
		if err != nil {
			return err
		}
		return StampJobProfileSetID(ctx, tx, tenantID, resolved, profileID, setID)
	})
}

func (s *PGStore) ListJobProfiles(ctx context.Context, tenantID string, setID string, asOfDate string) ([]jobcatalogtypes.JobProfile, error) {
	var out []jobcatalogtypes.JobProfile
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := setid.EnsureBootstrap(ctx, tx, tenantID, tenantID); err != nil {
			return err
		}
		v, err := resolveJobCatalogPackage(ctx, tx, tenantID, setID, asOfDate)
		if err != nil {
			return err
		}

		rows, err := tx.Query(ctx, `
SELECT
  p.job_profile_uuid::text,
  p.job_profile_code,
  v.name,
  v.is_active,
  lower(v.validity)::text,
  COALESCE(string_agg(f.job_family_code, ',' ORDER BY f.job_family_code) FILTER (WHERE f.job_family_code IS NOT NULL), '') AS family_codes_csv,
  COALESCE((
    SELECT f2.job_family_code
    FROM jobcatalog.job_profile_version_job_families pf2
    JOIN jobcatalog.job_families f2
      ON f2.tenant_uuid = $1::uuid
     AND f2.package_uuid = $2::uuid
     AND f2.job_family_uuid = pf2.job_family_uuid
    WHERE pf2.tenant_uuid = $1::uuid
      AND pf2.package_uuid = $2::uuid
      AND pf2.job_profile_version_id = v.id
      AND pf2.is_primary = true
    LIMIT 1
  ), '') AS primary_family_code
FROM jobcatalog.job_profiles p
JOIN jobcatalog.job_profile_versions v
  ON v.tenant_uuid = $1::uuid
 AND v.package_uuid = $2::uuid
 AND v.job_profile_uuid = p.job_profile_uuid
 AND v.validity @> $3::date
LEFT JOIN jobcatalog.job_profile_version_job_families pf
  ON pf.tenant_uuid = $1::uuid
 AND pf.package_uuid = $2::uuid
 AND pf.job_profile_version_id = v.id
LEFT JOIN jobcatalog.job_families f
  ON f.tenant_uuid = $1::uuid
 AND f.package_uuid = $2::uuid
 AND f.job_family_uuid = pf.job_family_uuid
WHERE p.tenant_uuid = $1::uuid
  AND p.package_uuid = $2::uuid
GROUP BY p.job_profile_uuid, p.job_profile_code, v.id, v.name, v.is_active, v.validity
ORDER BY p.job_profile_code ASC
`, tenantID, v, asOfDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p jobcatalogtypes.JobProfile
			if err := rows.Scan(&p.JobProfileUUID, &p.JobProfileCode, &p.Name, &p.IsActive, &p.EffectiveDay, &p.FamilyCodesCSV, &p.PrimaryFamilyCode); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

func quoteAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, strconv.Quote(v))
	}
	return out
}
