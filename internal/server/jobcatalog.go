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

func handleJobCatalog(w http.ResponseWriter, r *http.Request, orgStore OrgUnitStore, store JobCatalogStore) {
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

	setID := normalizeSetID(r.URL.Query().Get("setid"))

	list := func(errHint string) (groups []JobFamilyGroup, families []JobFamily, levels []JobLevel, profiles []JobProfile, errMsg string) {
		mergeMsg := func(hint string, msg string) string {
			if hint == "" {
				return msg
			}
			if msg == "" {
				return hint
			}
			return hint + "；" + msg
		}
		var err error

		if setID == "" {
			return nil, nil, nil, nil, mergeMsg(errHint, "setid is required")
		}

		groups, err = store.ListJobFamilyGroups(r.Context(), tenant.ID, setID, asOf)
		if err != nil {
			return nil, nil, nil, nil, mergeMsg(errHint, err.Error())
		}

		families, err = store.ListJobFamilies(r.Context(), tenant.ID, setID, asOf)
		if err != nil {
			return groups, nil, nil, nil, mergeMsg(errHint, err.Error())
		}

		levels, err = store.ListJobLevels(r.Context(), tenant.ID, setID, asOf)
		if err != nil {
			return groups, families, nil, nil, mergeMsg(errHint, err.Error())
		}

		profiles, err = store.ListJobProfiles(r.Context(), tenant.ID, setID, asOf)
		if err != nil {
			return groups, families, levels, nil, mergeMsg(errHint, err.Error())
		}

		return groups, families, levels, profiles, errHint
	}

	switch r.Method {
	case http.MethodGet:
		groups, families, levels, profiles, errMsg := list("")
		writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			groups, families, levels, profiles, errMsg := list("bad form")
			writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
			return
		}

		formSetID := normalizeSetID(r.Form.Get("setid"))
		if formSetID != "" {
			setID = formSetID
		}

		action := strings.TrimSpace(strings.ToLower(r.Form.Get("action")))
		if action == "" {
			action = "create_job_family_group"
		}
		switch action {
		case "create_job_family_group", "create_job_family", "update_job_family_group", "create_job_level", "create_job_profile":
		default:
			groups, families, levels, profiles, errMsg := list("unknown action")
			writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
			return
		}

		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		if effectiveDate == "" {
			effectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			groups, families, levels, profiles, errMsg := list("effective_date 无效: " + err.Error())
			writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
			return
		}

		if setID == "" {
			groups, families, levels, profiles, errMsg := list("setid is required")
			writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
			return
		}

		switch action {
		case "create_job_family_group":
			code := strings.TrimSpace(r.Form.Get("job_family_group_code"))
			name := strings.TrimSpace(r.Form.Get("job_family_group_name"))
			desc := strings.TrimSpace(r.Form.Get("job_family_group_description"))
			if code == "" || name == "" {
				groups, families, levels, profiles, errMsg := list("code/name is required")
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
				return
			}
			if err := store.CreateJobFamilyGroup(r.Context(), tenant.ID, setID, effectiveDate, code, name, desc); err != nil {
				groups, families, levels, profiles, errMsg := list(err.Error())
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
				return
			}
		case "create_job_family":
			code := strings.TrimSpace(r.Form.Get("job_family_code"))
			name := strings.TrimSpace(r.Form.Get("job_family_name"))
			desc := strings.TrimSpace(r.Form.Get("job_family_description"))
			groupCode := strings.TrimSpace(r.Form.Get("job_family_group_code"))
			if code == "" || name == "" || groupCode == "" {
				groups, families, levels, profiles, errMsg := list("code/name/group is required")
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
				return
			}
			if err := store.CreateJobFamily(r.Context(), tenant.ID, setID, effectiveDate, code, name, desc, groupCode); err != nil {
				groups, families, levels, profiles, errMsg := list(err.Error())
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
				return
			}
		case "update_job_family_group":
			familyCode := strings.TrimSpace(r.Form.Get("job_family_code"))
			groupCode := strings.TrimSpace(r.Form.Get("job_family_group_code"))
			if familyCode == "" || groupCode == "" {
				groups, families, levels, profiles, errMsg := list("family/group is required")
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
				return
			}
			if err := store.UpdateJobFamilyGroup(r.Context(), tenant.ID, setID, effectiveDate, familyCode, groupCode); err != nil {
				groups, families, levels, profiles, errMsg := list(err.Error())
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
				return
			}
		case "create_job_level":
			code := strings.TrimSpace(r.Form.Get("job_level_code"))
			name := strings.TrimSpace(r.Form.Get("job_level_name"))
			desc := strings.TrimSpace(r.Form.Get("job_level_description"))
			if code == "" || name == "" {
				groups, families, levels, profiles, errMsg := list("code/name is required")
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
				return
			}
			if err := store.CreateJobLevel(r.Context(), tenant.ID, setID, effectiveDate, code, name, desc); err != nil {
				groups, families, levels, profiles, errMsg := list(err.Error())
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
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
				groups, families, levels, profiles, errMsg := list("code/name/families/primary is required")
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
				return
			}
			if err := store.CreateJobProfile(r.Context(), tenant.ID, setID, effectiveDate, code, name, desc, familyCodes, primary); err != nil {
				groups, families, levels, profiles, errMsg := list(err.Error())
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, tenant, setID, errMsg, asOf))
				return
			}
		}

		http.Redirect(w, r, "/org/job-catalog?setid="+url.QueryEscape(setID)+"&as_of="+url.QueryEscape(effectiveDate), http.StatusSeeOther)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func renderJobCatalog(groups []JobFamilyGroup, families []JobFamily, levels []JobLevel, profiles []JobProfile, tenant Tenant, setID string, errMsg string, asOf string) string {
	var b strings.Builder
	b.WriteString("<h1>Job Catalog</h1>")
	b.WriteString("<p>Tenant: " + html.EscapeString(tenant.Name) + "</p>")
	b.WriteString(`<p><a href="/org/setid" hx-get="/org/setid" hx-target="#content" hx-push-url="true">SetID Governance</a></p>`)

	b.WriteString(`<form method="GET" action="/org/job-catalog">`)
	b.WriteString(`<label>As-of <input type="date" name="as_of" value="` + html.EscapeString(asOf) + `" /></label> `)
	b.WriteString(`<label>SetID <input name="setid" value="` + html.EscapeString(setID) + `" /></label> `)
	b.WriteString(`<button type="submit">Apply</button>`)
	b.WriteString(`</form>`)

	showSetID := setID != "" &&
		!strings.Contains(errMsg, "setid is required") &&
		!strings.Contains(errMsg, "JOBCATALOG_SETID_INVALID")
	if showSetID {
		b.WriteString(`<p>SetID: <code>` + html.EscapeString(setID) + `</code></p>`)
	}

	if errMsg != "" {
		b.WriteString(`<div style="padding:8px;border:1px solid #c00;color:#c00">` + html.EscapeString(errMsg) + `</div>`)
	}

	postAction := "/org/job-catalog?setid=" + url.QueryEscape(setID) + "&as_of=" + url.QueryEscape(asOf)
	b.WriteString(`<h2>Create Job Family Group</h2>`)
	b.WriteString(`<form method="POST" action="` + postAction + `">`)
	b.WriteString(`<input type="hidden" name="action" value="create_job_family_group" />`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>SetID <input name="setid" value="` + html.EscapeString(setID) + `" /></label><br/>`)
	b.WriteString(`<label>Code <input name="job_family_group_code" /></label><br/>`)
	b.WriteString(`<label>Name <input name="job_family_group_name" /></label><br/>`)
	b.WriteString(`<label>Description <input name="job_family_group_description" /></label><br/>`)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

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

	b.WriteString(`<h2>Create Job Family</h2>`)
	b.WriteString(`<form method="POST" action="` + postAction + `">`)
	b.WriteString(`<input type="hidden" name="action" value="create_job_family" />`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>SetID <input name="setid" value="` + html.EscapeString(setID) + `" /></label><br/>`)
	b.WriteString(`<label>Code <input name="job_family_code" /></label><br/>`)
	b.WriteString(`<label>Name <input name="job_family_name" /></label><br/>`)
	b.WriteString(`<label>Group Code <input name="job_family_group_code" /></label><br/>`)
	b.WriteString(`<label>Description <input name="job_family_description" /></label><br/>`)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Reparent Job Family (UPDATE group)</h2>`)
	b.WriteString(`<form method="POST" action="` + postAction + `">`)
	b.WriteString(`<input type="hidden" name="action" value="update_job_family_group" />`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>SetID <input name="setid" value="` + html.EscapeString(setID) + `" /></label><br/>`)
	b.WriteString(`<label>Family Code <input name="job_family_code" /></label><br/>`)
	b.WriteString(`<label>New Group Code <input name="job_family_group_code" /></label><br/>`)
	b.WriteString(`<button type="submit">Update</button>`)
	b.WriteString(`</form>`)

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

	b.WriteString(`<h2>Create Job Level</h2>`)
	b.WriteString(`<form method="POST" action="` + postAction + `">`)
	b.WriteString(`<input type="hidden" name="action" value="create_job_level" />`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>SetID <input name="setid" value="` + html.EscapeString(setID) + `" /></label><br/>`)
	b.WriteString(`<label>Code <input name="job_level_code" /></label><br/>`)
	b.WriteString(`<label>Name <input name="job_level_name" /></label><br/>`)
	b.WriteString(`<label>Description <input name="job_level_description" /></label><br/>`)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

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

	b.WriteString(`<h2>Create Job Profile</h2>`)
	b.WriteString(`<form method="POST" action="` + postAction + `">`)
	b.WriteString(`<input type="hidden" name="action" value="create_job_profile" />`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>SetID <input name="setid" value="` + html.EscapeString(setID) + `" /></label><br/>`)
	b.WriteString(`<label>Code <input name="job_profile_code" /></label><br/>`)
	b.WriteString(`<label>Name <input name="job_profile_name" /></label><br/>`)
	b.WriteString(`<label>Family Codes (comma-separated) <input name="job_profile_family_codes" /></label><br/>`)
	b.WriteString(`<label>Primary Family Code <input name="job_profile_primary_family_code" /></label><br/>`)
	b.WriteString(`<label>Description <input name="job_profile_description" /></label><br/>`)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

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
