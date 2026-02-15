package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/setid"
)

type JobFamilyGroup struct {
	JobFamilyGroupUUID string
	JobFamilyGroupCode string
	Name               string
	IsActive           bool
	EffectiveDay       string
}

type JobLevel struct {
	JobLevelUUID string
	JobLevelCode string
	Name         string
	IsActive     bool
	EffectiveDay string
}

type JobFamily struct {
	JobFamilyUUID      string
	JobFamilyCode      string
	JobFamilyGroupCode string
	Name               string
	IsActive           bool
	EffectiveDay       string
}

type JobProfile struct {
	JobProfileUUID    string
	JobProfileCode    string
	Name              string
	IsActive          bool
	EffectiveDay      string
	FamilyCodesCSV    string
	PrimaryFamilyCode string
}

type JobCatalogStore interface {
	ResolveJobCatalogPackageByCode(ctx context.Context, tenantID string, packageCode string, asOfDate string) (JobCatalogPackage, error)
	ResolveJobCatalogPackageBySetID(ctx context.Context, tenantID string, setID string, asOfDate string) (string, error)
	CreateJobFamilyGroup(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string) error
	ListJobFamilyGroups(ctx context.Context, tenantID string, setID string, asOfDate string) ([]JobFamilyGroup, error)
	CreateJobFamily(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string, groupCode string) error
	UpdateJobFamilyGroup(ctx context.Context, tenantID string, setID string, effectiveDate string, familyCode string, groupCode string) error
	ListJobFamilies(ctx context.Context, tenantID string, setID string, asOfDate string) ([]JobFamily, error)
	CreateJobLevel(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string) error
	ListJobLevels(ctx context.Context, tenantID string, setID string, asOfDate string) ([]JobLevel, error)
	CreateJobProfile(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string, familyCodes []string, primaryFamilyCode string) error
	ListJobProfiles(ctx context.Context, tenantID string, setID string, asOfDate string) ([]JobProfile, error)
}

type JobCatalogPackage struct {
	PackageUUID string
	PackageCode string
	OwnerSetID  string
}

type jobcatalogPGStore struct {
	pool pgBeginner
}

func newJobCatalogPGStore(pool pgBeginner) JobCatalogStore {
	return &jobcatalogPGStore{pool: pool}
}

type jobcatalogMemoryStore struct {
	groups   map[string]map[string][]JobFamilyGroup // tenant -> setid -> groups
	families map[string]map[string][]JobFamily      // tenant -> setid -> families
	levels   map[string]map[string][]JobLevel       // tenant -> setid -> levels
	profiles map[string]map[string][]JobProfile     // tenant -> setid -> profiles
}

func newJobCatalogMemoryStore() JobCatalogStore {
	return &jobcatalogMemoryStore{
		groups:   make(map[string]map[string][]JobFamilyGroup),
		families: make(map[string]map[string][]JobFamily),
		levels:   make(map[string]map[string][]JobLevel),
		profiles: make(map[string]map[string][]JobProfile),
	}
}

func (s *jobcatalogMemoryStore) ensure(tenantID string) {
	if s.groups[tenantID] == nil {
		s.groups[tenantID] = make(map[string][]JobFamilyGroup)
	}
	if s.families[tenantID] == nil {
		s.families[tenantID] = make(map[string][]JobFamily)
	}
	if s.levels[tenantID] == nil {
		s.levels[tenantID] = make(map[string][]JobLevel)
	}
	if s.profiles[tenantID] == nil {
		s.profiles[tenantID] = make(map[string][]JobProfile)
	}
}

func (s *jobcatalogMemoryStore) ResolveJobCatalogPackageByCode(_ context.Context, _ string, packageCode string, _ string) (JobCatalogPackage, error) {
	packageCode = strings.ToUpper(strings.TrimSpace(packageCode))
	if packageCode == "" {
		return JobCatalogPackage{}, errors.New("PACKAGE_CODE_INVALID")
	}
	return JobCatalogPackage{
		PackageUUID: packageCode,
		PackageCode: packageCode,
		OwnerSetID:  packageCode,
	}, nil
}

func (s *jobcatalogMemoryStore) ResolveJobCatalogPackageBySetID(_ context.Context, _ string, setID string, _ string) (string, error) {
	setID = normalizeSetID(setID)
	if setID == "" {
		return "", errors.New("setid is required")
	}
	return setID, nil
}

func (s *jobcatalogMemoryStore) CreateJobFamilyGroup(_ context.Context, tenantID string, setID string, effectiveDate string, code string, name string, _ string) error {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return errors.New("setid is required")
	}
	if s.groups[tenantID][setID] == nil {
		s.groups[tenantID][setID] = []JobFamilyGroup{}
	}
	s.groups[tenantID][setID] = append(s.groups[tenantID][setID], JobFamilyGroup{
		JobFamilyGroupUUID: strconv.Itoa(len(s.groups[tenantID][setID]) + 1),
		JobFamilyGroupCode: code,
		Name:               name,
		IsActive:           true,
		EffectiveDay:       effectiveDate,
	})
	return nil
}

func (s *jobcatalogMemoryStore) ListJobFamilyGroups(_ context.Context, tenantID string, setID string, _ string) ([]JobFamilyGroup, error) {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return nil, errors.New("setid is required")
	}
	return append([]JobFamilyGroup(nil), s.groups[tenantID][setID]...), nil
}

func (s *jobcatalogMemoryStore) CreateJobFamily(_ context.Context, tenantID string, setID string, effectiveDate string, code string, name string, _ string, groupCode string) error {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return errors.New("setid is required")
	}
	if s.families[tenantID][setID] == nil {
		s.families[tenantID][setID] = []JobFamily{}
	}
	s.families[tenantID][setID] = append(s.families[tenantID][setID], JobFamily{
		JobFamilyUUID:      strconv.Itoa(len(s.families[tenantID][setID]) + 1),
		JobFamilyCode:      code,
		JobFamilyGroupCode: groupCode,
		Name:               name,
		IsActive:           true,
		EffectiveDay:       effectiveDate,
	})
	return nil
}

func (s *jobcatalogMemoryStore) UpdateJobFamilyGroup(_ context.Context, tenantID string, setID string, effectiveDate string, familyCode string, groupCode string) error {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return errors.New("setid is required")
	}
	for i := range s.families[tenantID][setID] {
		if s.families[tenantID][setID][i].JobFamilyCode == familyCode {
			s.families[tenantID][setID][i].JobFamilyGroupCode = groupCode
			s.families[tenantID][setID][i].EffectiveDay = effectiveDate
			return nil
		}
	}
	return nil
}

func (s *jobcatalogMemoryStore) ListJobFamilies(_ context.Context, tenantID string, setID string, _ string) ([]JobFamily, error) {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return nil, errors.New("setid is required")
	}
	return append([]JobFamily(nil), s.families[tenantID][setID]...), nil
}

func (s *jobcatalogMemoryStore) CreateJobLevel(_ context.Context, tenantID string, setID string, effectiveDate string, code string, name string, _ string) error {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return errors.New("setid is required")
	}
	if s.levels[tenantID][setID] == nil {
		s.levels[tenantID][setID] = []JobLevel{}
	}
	s.levels[tenantID][setID] = append(s.levels[tenantID][setID], JobLevel{
		JobLevelUUID: strconv.Itoa(len(s.levels[tenantID][setID]) + 1),
		JobLevelCode: code,
		Name:         name,
		IsActive:     true,
		EffectiveDay: effectiveDate,
	})
	return nil
}

func (s *jobcatalogMemoryStore) ListJobLevels(_ context.Context, tenantID string, setID string, _ string) ([]JobLevel, error) {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return nil, errors.New("setid is required")
	}
	return append([]JobLevel(nil), s.levels[tenantID][setID]...), nil
}

func (s *jobcatalogMemoryStore) CreateJobProfile(_ context.Context, tenantID string, setID string, effectiveDate string, code string, name string, _ string, familyCodes []string, primaryFamilyCode string) error {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return errors.New("setid is required")
	}
	if s.profiles[tenantID][setID] == nil {
		s.profiles[tenantID][setID] = []JobProfile{}
	}
	s.profiles[tenantID][setID] = append(s.profiles[tenantID][setID], JobProfile{
		JobProfileUUID:    strconv.Itoa(len(s.profiles[tenantID][setID]) + 1),
		JobProfileCode:    code,
		Name:              name,
		IsActive:          true,
		EffectiveDay:      effectiveDate,
		FamilyCodesCSV:    strings.Join(familyCodes, ","),
		PrimaryFamilyCode: primaryFamilyCode,
	})
	return nil
}

func (s *jobcatalogMemoryStore) ListJobProfiles(_ context.Context, tenantID string, setID string, _ string) ([]JobProfile, error) {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return nil, errors.New("setid is required")
	}
	return append([]JobProfile(nil), s.profiles[tenantID][setID]...), nil
}

func (s *jobcatalogPGStore) withTx(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
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

func normalizeSetID(input string) string {
	return strings.ToUpper(strings.TrimSpace(input))
}

func ensureSetIDActive(ctx context.Context, tx pgx.Tx, tenantID string, setID string) (string, error) {
	setID = normalizeSetID(setID)
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

func resolveJobCatalogPackageByCode(ctx context.Context, tx pgx.Tx, tenantID string, packageCode string, asOfDate string) (JobCatalogPackage, error) {
	packageCode = strings.ToUpper(strings.TrimSpace(packageCode))
	if packageCode == "" {
		return JobCatalogPackage{}, errors.New("PACKAGE_CODE_INVALID")
	}

	var out JobCatalogPackage
	if err := tx.QueryRow(ctx, `
SELECT package_id::text, owner_setid
FROM orgunit.setid_scope_packages
WHERE tenant_uuid = $1::uuid
  AND scope_code = 'jobcatalog'
  AND package_code = $2::text
`, tenantID, packageCode).Scan(&out.PackageUUID, &out.OwnerSetID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return JobCatalogPackage{}, errors.New("PACKAGE_NOT_FOUND")
		}
		return JobCatalogPackage{}, err
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
		return JobCatalogPackage{}, err
	}
	out.PackageCode = packageCode
	return out, nil
}

func (s *jobcatalogPGStore) ResolveJobCatalogPackageByCode(ctx context.Context, tenantID string, packageCode string, asOfDate string) (JobCatalogPackage, error) {
	var out JobCatalogPackage
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		resolved, err := resolveJobCatalogPackageByCode(ctx, tx, tenantID, packageCode, asOfDate)
		if err != nil {
			return err
		}
		out = resolved
		return nil
	})
	return out, err
}

func (s *jobcatalogPGStore) ResolveJobCatalogPackageBySetID(ctx context.Context, tenantID string, setID string, asOfDate string) (string, error) {
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

func (s *jobcatalogPGStore) CreateJobFamilyGroup(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string) error {
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

func (s *jobcatalogPGStore) ListJobFamilyGroups(ctx context.Context, tenantID string, setID string, asOfDate string) ([]JobFamilyGroup, error) {
	var out []JobFamilyGroup
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
			var g JobFamilyGroup
			if err := rows.Scan(&g.JobFamilyGroupUUID, &g.JobFamilyGroupCode, &g.Name, &g.IsActive, &g.EffectiveDay); err != nil {
				return err
			}
			out = append(out, g)
		}
		return rows.Err()
	})
	return out, err
}

func (s *jobcatalogPGStore) CreateJobFamily(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string, groupCode string) error {
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

func (s *jobcatalogPGStore) UpdateJobFamilyGroup(ctx context.Context, tenantID string, setID string, effectiveDate string, familyCode string, groupCode string) error {
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

func (s *jobcatalogPGStore) ListJobFamilies(ctx context.Context, tenantID string, setID string, asOfDate string) ([]JobFamily, error) {
	var out []JobFamily
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
			var f JobFamily
			if err := rows.Scan(&f.JobFamilyUUID, &f.JobFamilyCode, &f.JobFamilyGroupCode, &f.Name, &f.IsActive, &f.EffectiveDay); err != nil {
				return err
			}
			out = append(out, f)
		}
		return rows.Err()
	})
	return out, err
}

func (s *jobcatalogPGStore) CreateJobLevel(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string) error {
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

func (s *jobcatalogPGStore) ListJobLevels(ctx context.Context, tenantID string, setID string, asOfDate string) ([]JobLevel, error) {
	var out []JobLevel
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
			var l JobLevel
			if err := rows.Scan(&l.JobLevelUUID, &l.JobLevelCode, &l.Name, &l.IsActive, &l.EffectiveDay); err != nil {
				return err
			}
			out = append(out, l)
		}
		return rows.Err()
	})
	return out, err
}

func (s *jobcatalogPGStore) CreateJobProfile(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string, familyCodes []string, primaryFamilyCode string) error {
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
			found := false
			for _, c := range lookupCodes {
				if c == primaryFamilyCode {
					found = true
					break
				}
			}
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
		return err
	})
}

func (s *jobcatalogPGStore) ListJobProfiles(ctx context.Context, tenantID string, setID string, asOfDate string) ([]JobProfile, error) {
	var out []JobProfile
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
			var p JobProfile
			if err := rows.Scan(&p.JobProfileUUID, &p.JobProfileCode, &p.Name, &p.IsActive, &p.EffectiveDay, &p.FamilyCodesCSV, &p.PrimaryFamilyCode); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

type jobCatalogSetIDStore interface {
	ListSetIDs(ctx context.Context, tenantID string) ([]SetID, error)
	ListOwnedScopePackages(ctx context.Context, tenantID string, scopeCode string, asOfDate string) ([]OwnedScopePackage, error)
}

type jobCatalogView struct {
	PackageCode  string
	OwnerSetID   string
	SetID        string
	ReadOnly     bool
	HasSelection bool
}

func (v jobCatalogView) listSetID() string {
	if !v.HasSelection {
		return ""
	}
	if v.ReadOnly {
		return v.SetID
	}
	return v.OwnerSetID
}

func normalizePackageCode(input string) string {
	return strings.ToUpper(strings.TrimSpace(input))
}

func canEditDefltPackage(ctx context.Context) bool {
	if !canEditOwnedScopePackages(ctx) {
		return false
	}
	p, _ := currentPrincipal(ctx)
	return strings.EqualFold(strings.TrimSpace(p.Status), "active")
}

func ownerSetIDEditable(ctx context.Context, setidStore jobCatalogSetIDStore, tenantID string, ownerSetID string) bool {
	if !canEditOwnedScopePackages(ctx) {
		return false
	}
	if setidStore == nil {
		return false
	}
	ownerSetID = normalizeSetID(ownerSetID)
	if ownerSetID == "" {
		return false
	}
	rows, err := setidStore.ListSetIDs(ctx, tenantID)
	if err != nil {
		return false
	}
	for _, row := range rows {
		if normalizeSetID(row.SetID) == ownerSetID && strings.EqualFold(strings.TrimSpace(row.Status), "active") {
			return true
		}
	}
	return false
}

func loadOwnedJobCatalogPackages(ctx context.Context, setidStore jobCatalogSetIDStore, tenantID string, asOf string) ([]OwnedScopePackage, error) {
	if setidStore == nil {
		return []OwnedScopePackage{}, nil
	}
	if !canEditOwnedScopePackages(ctx) {
		return []OwnedScopePackage{}, nil
	}
	rows, err := setidStore.ListOwnedScopePackages(ctx, tenantID, "jobcatalog", asOf)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		return []OwnedScopePackage{}, nil
	}
	return rows, nil
}

func resolveJobCatalogView(ctx context.Context, store JobCatalogStore, setidStore jobCatalogSetIDStore, tenantID string, asOf string, packageCode string, setID string) (jobCatalogView, string) {
	view := jobCatalogView{PackageCode: packageCode}
	if packageCode == "" && setID == "" {
		return view, ""
	}
	if setID != "" {
		view.SetID = setID
		view.ReadOnly = true
		view.HasSelection = true
		return view, ""
	}

	view.HasSelection = true
	if !canEditOwnedScopePackages(ctx) {
		return view, "OWNER_SETID_FORBIDDEN"
	}
	pkg, err := store.ResolveJobCatalogPackageByCode(ctx, tenantID, packageCode, asOf)
	if err != nil {
		return view, err.Error()
	}
	view.PackageCode = pkg.PackageCode
	view.OwnerSetID = pkg.OwnerSetID
	if !ownerSetIDEditable(ctx, setidStore, tenantID, pkg.OwnerSetID) {
		return view, "OWNER_SETID_FORBIDDEN"
	}
	if strings.EqualFold(pkg.PackageCode, "DEFLT") && !canEditDefltPackage(ctx) {
		return view, "DEFLT_EDIT_FORBIDDEN"
	}
	resolvedID, err := store.ResolveJobCatalogPackageBySetID(ctx, tenantID, pkg.OwnerSetID, asOf)
	if err != nil {
		return view, err.Error()
	}
	if resolvedID != pkg.PackageUUID {
		return view, "PACKAGE_CODE_MISMATCH"
	}
	return view, ""
}

func jobCatalogStatusForError(errMsg string) int {
	if strings.Contains(errMsg, "DEFLT_EDIT_FORBIDDEN") || strings.Contains(errMsg, "OWNER_SETID_FORBIDDEN") {
		return http.StatusForbidden
	}
	if strings.Contains(errMsg, "PACKAGE_CODE_MISMATCH") {
		return http.StatusUnprocessableEntity
	}
	return http.StatusOK
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func quoteAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, strconv.Quote(v))
	}
	return out
}
