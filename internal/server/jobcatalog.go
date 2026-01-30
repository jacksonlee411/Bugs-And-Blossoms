package server

import (
	"context"
	"errors"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/setid"
)

type JobFamilyGroup struct {
	ID           string
	Code         string
	Name         string
	IsActive     bool
	EffectiveDay string
}

type JobLevel struct {
	ID           string
	Code         string
	Name         string
	IsActive     bool
	EffectiveDay string
}

type JobFamily struct {
	ID           string
	Code         string
	GroupCode    string
	Name         string
	IsActive     bool
	EffectiveDay string
}

type JobProfile struct {
	ID                string
	Code              string
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
	PackageID   string
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
		PackageID:   packageCode,
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
		ID:           strconv.Itoa(len(s.groups[tenantID][setID]) + 1),
		Code:         code,
		Name:         name,
		IsActive:     true,
		EffectiveDay: effectiveDate,
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
		ID:           strconv.Itoa(len(s.families[tenantID][setID]) + 1),
		Code:         code,
		GroupCode:    groupCode,
		Name:         name,
		IsActive:     true,
		EffectiveDay: effectiveDate,
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
		if s.families[tenantID][setID][i].Code == familyCode {
			s.families[tenantID][setID][i].GroupCode = groupCode
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
		ID:           strconv.Itoa(len(s.levels[tenantID][setID]) + 1),
		Code:         code,
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
		ID:                strconv.Itoa(len(s.profiles[tenantID][setID]) + 1),
		Code:              code,
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
WHERE tenant_id = $1::uuid
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
SELECT package_id::text, package_owner_tenant_id::text
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
WHERE tenant_id = $1::uuid
  AND scope_code = 'jobcatalog'
  AND package_code = $2::text
`, tenantID, packageCode).Scan(&out.PackageID, &out.OwnerSetID); err != nil {
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
`, tenantID, out.PackageID, asOfDate); err != nil {
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

		payload := `{"code":` + strconv.Quote(code) +
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
  g.id::text,
  g.code,
  v.name,
  v.is_active,
  lower(v.validity)::text
FROM jobcatalog.job_family_groups g
JOIN jobcatalog.job_family_group_versions v
  ON v.tenant_id = $1::uuid
 AND v.package_id = $2::uuid
 AND v.job_family_group_id = g.id
WHERE g.tenant_id = $1::uuid
  AND g.package_id = $2::uuid
  AND v.validity @> $3::date
ORDER BY g.code ASC
`, tenantID, v, asOfDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var g JobFamilyGroup
			if err := rows.Scan(&g.ID, &g.Code, &g.Name, &g.IsActive, &g.EffectiveDay); err != nil {
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
SELECT g.id::text
FROM jobcatalog.job_family_groups g
WHERE g.tenant_id = $1::uuid
  AND g.package_id = $2::uuid
  AND g.code = $3::text
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

		payload := `{"code":` + strconv.Quote(code) +
			`,"name":` + strconv.Quote(name) +
			`,"job_family_group_id":` + strconv.Quote(groupID)
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
SELECT g.id::text
FROM jobcatalog.job_family_groups g
WHERE g.tenant_id = $1::uuid
  AND g.package_id = $2::uuid
  AND g.code = $3::text
`, tenantID, resolved, groupCode).Scan(&groupID); err != nil {
			return err
		}

		var familyID string
		if err := tx.QueryRow(ctx, `
SELECT f.id::text
FROM jobcatalog.job_families f
WHERE f.tenant_id = $1::uuid
  AND f.package_id = $2::uuid
  AND f.code = $3::text
`, tenantID, resolved, familyCode).Scan(&familyID); err != nil {
			return err
		}

		var eventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
			return err
		}

		payload := `{"job_family_group_id":` + strconv.Quote(groupID) + `}`

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
  f.id::text,
  f.code,
  g.code AS group_code,
  v.name,
  v.is_active,
  lower(v.validity)::text
FROM jobcatalog.job_families f
JOIN jobcatalog.job_family_versions v
  ON v.tenant_id = $1::uuid
 AND v.package_id = $2::uuid
 AND v.job_family_id = f.id
JOIN jobcatalog.job_family_groups g
  ON g.tenant_id = $1::uuid
 AND g.package_id = $2::uuid
 AND g.id = v.job_family_group_id
WHERE f.tenant_id = $1::uuid
  AND f.package_id = $2::uuid
  AND v.validity @> $3::date
ORDER BY f.code ASC
`, tenantID, v, asOfDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var f JobFamily
			if err := rows.Scan(&f.ID, &f.Code, &f.GroupCode, &f.Name, &f.IsActive, &f.EffectiveDay); err != nil {
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

		payload := `{"code":` + strconv.Quote(code) +
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
	  l.id::text,
	  l.code,
	  v.name,
	  v.is_active,
	  lower(v.validity)::text
	FROM jobcatalog.job_levels l
	JOIN jobcatalog.job_level_versions v
	  ON v.tenant_id = $1::uuid
	 AND v.package_id = $2::uuid
	 AND v.job_level_id = l.id
	WHERE l.tenant_id = $1::uuid
	  AND l.package_id = $2::uuid
	  AND v.validity @> $3::date
	ORDER BY l.code ASC
	`, tenantID, v, asOfDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var l JobLevel
			if err := rows.Scan(&l.ID, &l.Code, &l.Name, &l.IsActive, &l.EffectiveDay); err != nil {
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
SELECT f.code, f.id::text
FROM jobcatalog.job_families f
WHERE f.tenant_id = $1::uuid
  AND f.package_id = $2::uuid
  AND f.code = ANY($3::text[])
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

		payload := `{"code":` + strconv.Quote(code) +
			`,"name":` + strconv.Quote(name) +
			`,"job_family_ids":[` + strings.Join(quoteAll(familyIDs), ",") + `]` +
			`,"primary_job_family_id":` + strconv.Quote(primaryID)
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
  p.id::text,
  p.code,
  v.name,
  v.is_active,
  lower(v.validity)::text,
  COALESCE(string_agg(f.code, ',' ORDER BY f.code) FILTER (WHERE f.code IS NOT NULL), '') AS family_codes_csv,
  COALESCE((
    SELECT f2.code
    FROM jobcatalog.job_profile_version_job_families pf2
    JOIN jobcatalog.job_families f2
      ON f2.tenant_id = $1::uuid
     AND f2.package_id = $2::uuid
     AND f2.id = pf2.job_family_id
    WHERE pf2.tenant_id = $1::uuid
      AND pf2.package_id = $2::uuid
      AND pf2.job_profile_version_id = v.id
      AND pf2.is_primary = true
    LIMIT 1
  ), '') AS primary_family_code
FROM jobcatalog.job_profiles p
JOIN jobcatalog.job_profile_versions v
  ON v.tenant_id = $1::uuid
 AND v.package_id = $2::uuid
 AND v.job_profile_id = p.id
 AND v.validity @> $3::date
LEFT JOIN jobcatalog.job_profile_version_job_families pf
  ON pf.tenant_id = $1::uuid
 AND pf.package_id = $2::uuid
 AND pf.job_profile_version_id = v.id
LEFT JOIN jobcatalog.job_families f
  ON f.tenant_id = $1::uuid
 AND f.package_id = $2::uuid
 AND f.id = pf.job_family_id
WHERE p.tenant_id = $1::uuid
  AND p.package_id = $2::uuid
GROUP BY p.id, p.code, v.id, v.name, v.is_active, v.validity
ORDER BY p.code ASC
`, tenantID, v, asOfDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p JobProfile
			if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.IsActive, &p.EffectiveDay, &p.FamilyCodesCSV, &p.PrimaryFamilyCode); err != nil {
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
	if resolvedID != pkg.PackageID {
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

func handleJobCatalog(w http.ResponseWriter, r *http.Request, orgStore OrgUnitStore, setidStore jobCatalogSetIDStore, store JobCatalogStore) {
	_ = orgStore
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}

	mergeMsg := func(hint string, msg string) string {
		if hint == "" {
			return msg
		}
		if msg == "" {
			return hint
		}
		return hint + "；" + msg
	}

	queryPackageCode := normalizePackageCode(r.URL.Query().Get("package_code"))
	querySetID := normalizeSetID(r.URL.Query().Get("setid"))
	if queryPackageCode != "" && querySetID != "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_request", "package_code and setid are mutually exclusive")
		return
	}

	ownedPackages, ownedErr := loadOwnedJobCatalogPackages(r.Context(), setidStore, tenant.ID, asOf)
	if ownedErr != nil {
		ownedPackages = []OwnedScopePackage{}
	}
	mergeOwned := func(errMsg string) string {
		if ownedErr != nil {
			return mergeMsg(errMsg, ownedErr.Error())
		}
		return errMsg
	}

	list := func(errHint string, packageCode string, setID string) (groups []JobFamilyGroup, families []JobFamily, levels []JobLevel, profiles []JobProfile, view jobCatalogView, errMsg string) {
		var err error
		view, errMsg = resolveJobCatalogView(r.Context(), store, setidStore, tenant.ID, asOf, packageCode, setID)
		if errMsg != "" {
			return nil, nil, nil, nil, view, mergeMsg(errHint, errMsg)
		}

		listSetID := view.listSetID()
		if listSetID == "" {
			return nil, nil, nil, nil, view, errHint
		}

		groups, err = store.ListJobFamilyGroups(r.Context(), tenant.ID, listSetID, asOf)
		if err != nil {
			return nil, nil, nil, nil, view, mergeMsg(errHint, err.Error())
		}

		families, err = store.ListJobFamilies(r.Context(), tenant.ID, listSetID, asOf)
		if err != nil {
			return groups, nil, nil, nil, view, mergeMsg(errHint, err.Error())
		}

		levels, err = store.ListJobLevels(r.Context(), tenant.ID, listSetID, asOf)
		if err != nil {
			return groups, families, nil, nil, view, mergeMsg(errHint, err.Error())
		}

		profiles, err = store.ListJobProfiles(r.Context(), tenant.ID, listSetID, asOf)
		if err != nil {
			return groups, families, levels, nil, view, mergeMsg(errHint, err.Error())
		}

		return groups, families, levels, profiles, view, errHint
	}

	switch r.Method {
	case http.MethodGet:
		groups, families, levels, profiles, view, errMsg := list("", queryPackageCode, querySetID)
		errMsg = mergeOwned(errMsg)
		writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			groups, families, levels, profiles, view, errMsg := list("bad form", queryPackageCode, querySetID)
			errMsg = mergeOwned(errMsg)
			writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
			return
		}

		formPackageCode := normalizePackageCode(r.Form.Get("package_code"))
		packageCode := queryPackageCode
		if formPackageCode != "" {
			packageCode = formPackageCode
		}

		formSetID := normalizeSetID(r.Form.Get("setid"))
		setID := querySetID
		if formSetID != "" {
			setID = formSetID
		}

		if packageCode != "" && setID != "" {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_request", "package_code and setid are mutually exclusive")
			return
		}

		action := strings.TrimSpace(strings.ToLower(r.Form.Get("action")))
		if action == "" {
			action = "create_job_family_group"
		}
		switch action {
		case "create_job_family_group", "create_job_family", "update_job_family_group", "create_job_level", "create_job_profile":
		default:
			groups, families, levels, profiles, view, errMsg := list("unknown action", packageCode, setID)
			errMsg = mergeOwned(errMsg)
			writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
			return
		}

		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		if effectiveDate == "" {
			effectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			groups, families, levels, profiles, view, errMsg := list("effective_date 无效: "+err.Error(), packageCode, setID)
			errMsg = mergeOwned(errMsg)
			writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
			return
		}

		if setID != "" {
			groups, families, levels, profiles, view, errMsg := list("setid is read-only; use package_code", packageCode, setID)
			errMsg = mergeOwned(errMsg)
			status := jobCatalogStatusForError(errMsg)
			writePageWithStatus(w, r, status, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
			return
		}

		if packageCode == "" {
			groups, families, levels, profiles, view, errMsg := list("package_code is required", packageCode, setID)
			errMsg = mergeOwned(errMsg)
			writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
			return
		}

		writeView, writeErr := resolveJobCatalogView(r.Context(), store, setidStore, tenant.ID, effectiveDate, packageCode, "")
		if writeErr != "" {
			groups, families, levels, profiles, view, errMsg := list(writeErr, packageCode, setID)
			errMsg = mergeOwned(errMsg)
			status := jobCatalogStatusForError(writeErr)
			writePageWithStatus(w, r, status, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
			return
		}

		ownerSetID := writeView.OwnerSetID

		switch action {
		case "create_job_family_group":
			code := strings.TrimSpace(r.Form.Get("job_family_group_code"))
			name := strings.TrimSpace(r.Form.Get("job_family_group_name"))
			desc := strings.TrimSpace(r.Form.Get("job_family_group_description"))
			if code == "" || name == "" {
				groups, families, levels, profiles, view, errMsg := list("code/name is required", packageCode, setID)
				errMsg = mergeOwned(errMsg)
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
				return
			}
			if err := store.CreateJobFamilyGroup(r.Context(), tenant.ID, ownerSetID, effectiveDate, code, name, desc); err != nil {
				groups, families, levels, profiles, view, errMsg := list(err.Error(), packageCode, "")
				errMsg = mergeOwned(errMsg)
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
				return
			}
		case "create_job_family":
			code := strings.TrimSpace(r.Form.Get("job_family_code"))
			name := strings.TrimSpace(r.Form.Get("job_family_name"))
			desc := strings.TrimSpace(r.Form.Get("job_family_description"))
			groupCode := strings.TrimSpace(r.Form.Get("job_family_group_code"))
			if code == "" || name == "" || groupCode == "" {
				groups, families, levels, profiles, view, errMsg := list("code/name/group is required", packageCode, setID)
				errMsg = mergeOwned(errMsg)
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
				return
			}
			if err := store.CreateJobFamily(r.Context(), tenant.ID, ownerSetID, effectiveDate, code, name, desc, groupCode); err != nil {
				groups, families, levels, profiles, view, errMsg := list(err.Error(), packageCode, "")
				errMsg = mergeOwned(errMsg)
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
				return
			}
		case "update_job_family_group":
			familyCode := strings.TrimSpace(r.Form.Get("job_family_code"))
			groupCode := strings.TrimSpace(r.Form.Get("job_family_group_code"))
			if familyCode == "" || groupCode == "" {
				groups, families, levels, profiles, view, errMsg := list("family/group is required", packageCode, setID)
				errMsg = mergeOwned(errMsg)
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
				return
			}
			if err := store.UpdateJobFamilyGroup(r.Context(), tenant.ID, ownerSetID, effectiveDate, familyCode, groupCode); err != nil {
				groups, families, levels, profiles, view, errMsg := list(err.Error(), packageCode, "")
				errMsg = mergeOwned(errMsg)
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
				return
			}
		case "create_job_level":
			code := strings.TrimSpace(r.Form.Get("job_level_code"))
			name := strings.TrimSpace(r.Form.Get("job_level_name"))
			desc := strings.TrimSpace(r.Form.Get("job_level_description"))
			if code == "" || name == "" {
				groups, families, levels, profiles, view, errMsg := list("code/name is required", packageCode, setID)
				errMsg = mergeOwned(errMsg)
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
				return
			}
			if err := store.CreateJobLevel(r.Context(), tenant.ID, ownerSetID, effectiveDate, code, name, desc); err != nil {
				groups, families, levels, profiles, view, errMsg := list(err.Error(), packageCode, "")
				errMsg = mergeOwned(errMsg)
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
				return
			}
		case "create_job_profile":
			code := strings.TrimSpace(r.Form.Get("job_profile_code"))
			name := strings.TrimSpace(r.Form.Get("job_profile_name"))
			desc := strings.TrimSpace(r.Form.Get("job_profile_description"))
			familiesCSV := strings.TrimSpace(r.Form.Get("job_profile_family_codes"))
			primary := strings.TrimSpace(r.Form.Get("job_profile_primary_family_code"))
			familyCodes := splitCSV(familiesCSV)
			if code == "" || name == "" || len(familyCodes) == 0 || primary == "" {
				groups, families, levels, profiles, view, errMsg := list("code/name/families/primary is required", packageCode, setID)
				errMsg = mergeOwned(errMsg)
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
				return
			}
			if err := store.CreateJobProfile(r.Context(), tenant.ID, ownerSetID, effectiveDate, code, name, desc, familyCodes, primary); err != nil {
				groups, families, levels, profiles, view, errMsg := list(err.Error(), packageCode, "")
				errMsg = mergeOwned(errMsg)
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, view, errMsg, asOf, ownedPackages))
				return
			}
		}

		http.Redirect(w, r, "/org/job-catalog?package_code="+url.QueryEscape(packageCode)+"&as_of="+url.QueryEscape(effectiveDate), http.StatusSeeOther)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func renderJobCatalog(groups []JobFamilyGroup, families []JobFamily, levels []JobLevel, profiles []JobProfile, tenant Tenant, view jobCatalogView, errMsg string, asOf string, ownedPackages []OwnedScopePackage) string {
	var b strings.Builder
	b.WriteString("<h1>Job Catalog</h1>")
	b.WriteString("<p>Tenant: " + html.EscapeString(tenant.Name) + "</p>")
	b.WriteString(`<p><a href="/org/setid" hx-get="/org/setid" hx-target="#content" hx-push-url="true">SetID Governance</a></p>`)

	b.WriteString(`<form method="GET" action="/org/job-catalog">`)
	b.WriteString(`<label>As-of <input type="date" name="as_of" value="` + html.EscapeString(asOf) + `" /></label> `)
	b.WriteString(`<label>Package <select name="package_code">`)
	b.WriteString(`<option value="">(select)</option>`)
	for _, pkg := range ownedPackages {
		label := pkg.PackageCode
		if strings.TrimSpace(pkg.Name) != "" {
			label += " - " + pkg.Name
		}
		if strings.TrimSpace(pkg.OwnerSetID) != "" {
			label += " (" + pkg.OwnerSetID + ")"
		}
		selected := ""
		if strings.EqualFold(pkg.PackageCode, view.PackageCode) {
			selected = ` selected="selected"`
		}
		b.WriteString(`<option value="` + html.EscapeString(pkg.PackageCode) + `"` + selected + `>` + html.EscapeString(label) + `</option>`)
	}
	b.WriteString(`</select></label> `)
	b.WriteString(`<button type="submit">Apply</button>`)
	b.WriteString(`</form>`)

	if view.ReadOnly && view.SetID != "" {
		b.WriteString(`<p><strong>只读视图</strong>：通过 SetID 访问。如需编辑，请选择 package_code。</p>`)
		b.WriteString(`<p>SetID: <code>` + html.EscapeString(view.SetID) + `</code></p>`)
	}

	if !view.ReadOnly && view.PackageCode != "" {
		b.WriteString(`<p>Package: <code>` + html.EscapeString(view.PackageCode) + `</code></p>`)
		if view.OwnerSetID != "" {
			b.WriteString(`<p>Owner SetID: <code>` + html.EscapeString(view.OwnerSetID) + `</code></p>`)
		}
	}

	if !view.HasSelection {
		b.WriteString(`<p>请选择 package_code 以进入编辑。</p>`)
	}

	if errMsg != "" {
		b.WriteString(`<div style="padding:8px;border:1px solid #c00;color:#c00">` + html.EscapeString(errMsg) + `</div>`)
	}

	editBlocked := strings.Contains(errMsg, "OWNER_SETID_FORBIDDEN") ||
		strings.Contains(errMsg, "PACKAGE_CODE_MISMATCH") ||
		strings.Contains(errMsg, "DEFLT_EDIT_FORBIDDEN") ||
		strings.Contains(errMsg, "PACKAGE_CODE_INVALID") ||
		strings.Contains(errMsg, "PACKAGE_NOT_FOUND") ||
		strings.Contains(errMsg, "PACKAGE_INACTIVE_AS_OF")
	showForms := view.HasSelection && !view.ReadOnly && view.PackageCode != "" && !editBlocked
	postAction := ""
	if showForms {
		postAction = "/org/job-catalog?package_code=" + url.QueryEscape(view.PackageCode) + "&as_of=" + url.QueryEscape(asOf)
	}

	if showForms {
		b.WriteString(`<h2>Create Job Family Group</h2>`)
		b.WriteString(`<form method="POST" action="` + postAction + `">`)
		b.WriteString(`<input type="hidden" name="action" value="create_job_family_group" />`)
		b.WriteString(`<input type="hidden" name="package_code" value="` + html.EscapeString(view.PackageCode) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
		b.WriteString(`<label>Code <input name="job_family_group_code" /></label><br/>`)
		b.WriteString(`<label>Name <input name="job_family_group_name" /></label><br/>`)
		b.WriteString(`<label>Description <input name="job_family_group_description" /></label><br/>`)
		b.WriteString(`<button type="submit">Create</button>`)
		b.WriteString(`</form>`)
	}

	if view.HasSelection {
		b.WriteString(`<h2>Job Family Groups</h2>`)
		b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr><th>code</th><th>name</th><th>active</th><th>effective_date</th><th>id</th></tr></thead><tbody>`)
		for _, g := range groups {
			active := "false"
			if g.IsActive {
				active = "true"
			}
			b.WriteString("<tr>")
			b.WriteString("<td>" + html.EscapeString(g.Code) + "</td>")
			b.WriteString("<td>" + html.EscapeString(g.Name) + "</td>")
			b.WriteString("<td>" + html.EscapeString(active) + "</td>")
			b.WriteString("<td>" + html.EscapeString(g.EffectiveDay) + "</td>")
			b.WriteString("<td><code>" + html.EscapeString(g.ID) + "</code></td>")
			b.WriteString("</tr>")
		}
		b.WriteString("</tbody></table>")
	}

	if showForms {
		b.WriteString(`<h2>Create Job Family</h2>`)
		b.WriteString(`<form method="POST" action="` + postAction + `">`)
		b.WriteString(`<input type="hidden" name="action" value="create_job_family" />`)
		b.WriteString(`<input type="hidden" name="package_code" value="` + html.EscapeString(view.PackageCode) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
		b.WriteString(`<label>Code <input name="job_family_code" /></label><br/>`)
		b.WriteString(`<label>Name <input name="job_family_name" /></label><br/>`)
		b.WriteString(`<label>Group Code <input name="job_family_group_code" /></label><br/>`)
		b.WriteString(`<label>Description <input name="job_family_description" /></label><br/>`)
		b.WriteString(`<button type="submit">Create</button>`)
		b.WriteString(`</form>`)

		b.WriteString(`<h2>Reparent Job Family (UPDATE group)</h2>`)
		b.WriteString(`<form method="POST" action="` + postAction + `">`)
		b.WriteString(`<input type="hidden" name="action" value="update_job_family_group" />`)
		b.WriteString(`<input type="hidden" name="package_code" value="` + html.EscapeString(view.PackageCode) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
		b.WriteString(`<label>Family Code <input name="job_family_code" /></label><br/>`)
		b.WriteString(`<label>New Group Code <input name="job_family_group_code" /></label><br/>`)
		b.WriteString(`<button type="submit">Update</button>`)
		b.WriteString(`</form>`)
	}

	if view.HasSelection {
		b.WriteString(`<h2>Job Families</h2>`)
		b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr><th>code</th><th>group</th><th>name</th><th>active</th><th>effective_date</th><th>id</th></tr></thead><tbody>`)
		for _, f := range families {
			active := "false"
			if f.IsActive {
				active = "true"
			}
			b.WriteString("<tr>")
			b.WriteString("<td>" + html.EscapeString(f.Code) + "</td>")
			b.WriteString("<td>" + html.EscapeString(f.GroupCode) + "</td>")
			b.WriteString("<td>" + html.EscapeString(f.Name) + "</td>")
			b.WriteString("<td>" + html.EscapeString(active) + "</td>")
			b.WriteString("<td>" + html.EscapeString(f.EffectiveDay) + "</td>")
			b.WriteString("<td><code>" + html.EscapeString(f.ID) + "</code></td>")
			b.WriteString("</tr>")
		}
		b.WriteString("</tbody></table>")
	}

	if showForms {
		b.WriteString(`<h2>Create Job Level</h2>`)
		b.WriteString(`<form method="POST" action="` + postAction + `">`)
		b.WriteString(`<input type="hidden" name="action" value="create_job_level" />`)
		b.WriteString(`<input type="hidden" name="package_code" value="` + html.EscapeString(view.PackageCode) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
		b.WriteString(`<label>Code <input name="job_level_code" /></label><br/>`)
		b.WriteString(`<label>Name <input name="job_level_name" /></label><br/>`)
		b.WriteString(`<label>Description <input name="job_level_description" /></label><br/>`)
		b.WriteString(`<button type="submit">Create</button>`)
		b.WriteString(`</form>`)
	}

	if view.HasSelection {
		b.WriteString(`<h2>Job Levels</h2>`)
		b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr><th>code</th><th>name</th><th>active</th><th>effective_date</th><th>id</th></tr></thead><tbody>`)
		for _, l := range levels {
			active := "false"
			if l.IsActive {
				active = "true"
			}
			b.WriteString("<tr>")
			b.WriteString("<td>" + html.EscapeString(l.Code) + "</td>")
			b.WriteString("<td>" + html.EscapeString(l.Name) + "</td>")
			b.WriteString("<td>" + html.EscapeString(active) + "</td>")
			b.WriteString("<td>" + html.EscapeString(l.EffectiveDay) + "</td>")
			b.WriteString("<td><code>" + html.EscapeString(l.ID) + "</code></td>")
			b.WriteString("</tr>")
		}
		b.WriteString("</tbody></table>")
	}

	if showForms {
		b.WriteString(`<h2>Create Job Profile</h2>`)
		b.WriteString(`<form method="POST" action="` + postAction + `">`)
		b.WriteString(`<input type="hidden" name="action" value="create_job_profile" />`)
		b.WriteString(`<input type="hidden" name="package_code" value="` + html.EscapeString(view.PackageCode) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
		b.WriteString(`<label>Code <input name="job_profile_code" /></label><br/>`)
		b.WriteString(`<label>Name <input name="job_profile_name" /></label><br/>`)
		b.WriteString(`<label>Family Codes (comma-separated) <input name="job_profile_family_codes" /></label><br/>`)
		b.WriteString(`<label>Primary Family Code <input name="job_profile_primary_family_code" /></label><br/>`)
		b.WriteString(`<label>Description <input name="job_profile_description" /></label><br/>`)
		b.WriteString(`<button type="submit">Create</button>`)
		b.WriteString(`</form>`)
	}

	if view.HasSelection {
		b.WriteString(`<h2>Job Profiles</h2>`)
		b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr><th>code</th><th>name</th><th>families</th><th>primary</th><th>active</th><th>effective_date</th><th>id</th></tr></thead><tbody>`)
		for _, p := range profiles {
			active := "false"
			if p.IsActive {
				active = "true"
			}
			b.WriteString("<tr>")
			b.WriteString("<td>" + html.EscapeString(p.Code) + "</td>")
			b.WriteString("<td>" + html.EscapeString(p.Name) + "</td>")
			b.WriteString("<td>" + html.EscapeString(p.FamilyCodesCSV) + "</td>")
			b.WriteString("<td>" + html.EscapeString(p.PrimaryFamilyCode) + "</td>")
			b.WriteString("<td>" + html.EscapeString(active) + "</td>")
			b.WriteString("<td>" + html.EscapeString(p.EffectiveDay) + "</td>")
			b.WriteString("<td><code>" + html.EscapeString(p.ID) + "</code></td>")
			b.WriteString("</tr>")
		}
		b.WriteString("</tbody></table>")
	}

	return b.String()
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
