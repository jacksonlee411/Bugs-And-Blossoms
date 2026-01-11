package server

import (
	"context"
	"errors"
	"html"
	"net/http"
	"net/url"
	"sort"
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
	ListBusinessUnits(ctx context.Context, tenantID string) ([]BusinessUnit, error)
	ResolveSetID(ctx context.Context, tenantID string, businessUnitID string, recordGroup string) (string, error)
	CreateJobFamilyGroup(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, description string) error
	ListJobFamilyGroups(ctx context.Context, tenantID string, businessUnitID string, asOfDate string) ([]JobFamilyGroup, string, error)
	CreateJobFamily(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, description string, groupCode string) error
	UpdateJobFamilyGroup(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, familyCode string, groupCode string) error
	ListJobFamilies(ctx context.Context, tenantID string, businessUnitID string, asOfDate string) ([]JobFamily, string, error)
	CreateJobLevel(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, description string) error
	ListJobLevels(ctx context.Context, tenantID string, businessUnitID string, asOfDate string) ([]JobLevel, string, error)
	CreateJobProfile(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, description string, familyCodes []string, primaryFamilyCode string) error
	ListJobProfiles(ctx context.Context, tenantID string, businessUnitID string, asOfDate string) ([]JobProfile, string, error)
}

type jobcatalogPGStore struct {
	pool pgBeginner
}

func newJobCatalogPGStore(pool pgBeginner) JobCatalogStore {
	return &jobcatalogPGStore{pool: pool}
}

type jobcatalogMemoryStore struct {
	businessUnits map[string][]BusinessUnit
	groups        map[string]map[string][]JobFamilyGroup // tenant -> bu -> groups
	families      map[string]map[string][]JobFamily      // tenant -> bu -> families
	levels        map[string]map[string][]JobLevel       // tenant -> bu -> levels
	profiles      map[string]map[string][]JobProfile     // tenant -> bu -> profiles
}

func newJobCatalogMemoryStore() JobCatalogStore {
	return &jobcatalogMemoryStore{
		businessUnits: make(map[string][]BusinessUnit),
		groups:        make(map[string]map[string][]JobFamilyGroup),
		families:      make(map[string]map[string][]JobFamily),
		levels:        make(map[string]map[string][]JobLevel),
		profiles:      make(map[string]map[string][]JobProfile),
	}
}

func (s *jobcatalogMemoryStore) ensure(tenantID string) {
	if _, ok := s.businessUnits[tenantID]; !ok {
		s.businessUnits[tenantID] = []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}
	}
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

func (s *jobcatalogMemoryStore) ListBusinessUnits(_ context.Context, tenantID string) ([]BusinessUnit, error) {
	s.ensure(tenantID)
	return append([]BusinessUnit(nil), s.businessUnits[tenantID]...), nil
}

func (s *jobcatalogMemoryStore) ResolveSetID(_ context.Context, tenantID string, _ string, _ string) (string, error) {
	s.ensure(tenantID)
	return "SHARE", nil
}

func (s *jobcatalogMemoryStore) CreateJobFamilyGroup(_ context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, _ string) error {
	s.ensure(tenantID)
	if businessUnitID == "" {
		businessUnitID = "BU000"
	}
	if s.groups[tenantID][businessUnitID] == nil {
		s.groups[tenantID][businessUnitID] = []JobFamilyGroup{}
	}
	s.groups[tenantID][businessUnitID] = append(s.groups[tenantID][businessUnitID], JobFamilyGroup{
		ID:           strconv.Itoa(len(s.groups[tenantID][businessUnitID]) + 1),
		Code:         code,
		Name:         name,
		IsActive:     true,
		EffectiveDay: effectiveDate,
	})
	return nil
}

func (s *jobcatalogMemoryStore) ListJobFamilyGroups(_ context.Context, tenantID string, businessUnitID string, _ string) ([]JobFamilyGroup, string, error) {
	s.ensure(tenantID)
	if businessUnitID == "" {
		businessUnitID = "BU000"
	}
	return append([]JobFamilyGroup(nil), s.groups[tenantID][businessUnitID]...), "SHARE", nil
}

func (s *jobcatalogMemoryStore) CreateJobFamily(_ context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, _ string, groupCode string) error {
	s.ensure(tenantID)
	if businessUnitID == "" {
		businessUnitID = "BU000"
	}
	if s.families[tenantID][businessUnitID] == nil {
		s.families[tenantID][businessUnitID] = []JobFamily{}
	}
	s.families[tenantID][businessUnitID] = append(s.families[tenantID][businessUnitID], JobFamily{
		ID:           strconv.Itoa(len(s.families[tenantID][businessUnitID]) + 1),
		Code:         code,
		GroupCode:    groupCode,
		Name:         name,
		IsActive:     true,
		EffectiveDay: effectiveDate,
	})
	return nil
}

func (s *jobcatalogMemoryStore) UpdateJobFamilyGroup(_ context.Context, tenantID string, businessUnitID string, effectiveDate string, familyCode string, groupCode string) error {
	s.ensure(tenantID)
	if businessUnitID == "" {
		businessUnitID = "BU000"
	}
	for i := range s.families[tenantID][businessUnitID] {
		if s.families[tenantID][businessUnitID][i].Code == familyCode {
			s.families[tenantID][businessUnitID][i].GroupCode = groupCode
			s.families[tenantID][businessUnitID][i].EffectiveDay = effectiveDate
			return nil
		}
	}
	return nil
}

func (s *jobcatalogMemoryStore) ListJobFamilies(_ context.Context, tenantID string, businessUnitID string, _ string) ([]JobFamily, string, error) {
	s.ensure(tenantID)
	if businessUnitID == "" {
		businessUnitID = "BU000"
	}
	return append([]JobFamily(nil), s.families[tenantID][businessUnitID]...), "SHARE", nil
}

func (s *jobcatalogMemoryStore) CreateJobLevel(_ context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, _ string) error {
	s.ensure(tenantID)
	if businessUnitID == "" {
		businessUnitID = "BU000"
	}
	if s.levels[tenantID][businessUnitID] == nil {
		s.levels[tenantID][businessUnitID] = []JobLevel{}
	}
	s.levels[tenantID][businessUnitID] = append(s.levels[tenantID][businessUnitID], JobLevel{
		ID:           strconv.Itoa(len(s.levels[tenantID][businessUnitID]) + 1),
		Code:         code,
		Name:         name,
		IsActive:     true,
		EffectiveDay: effectiveDate,
	})
	return nil
}

func (s *jobcatalogMemoryStore) ListJobLevels(_ context.Context, tenantID string, businessUnitID string, _ string) ([]JobLevel, string, error) {
	s.ensure(tenantID)
	if businessUnitID == "" {
		businessUnitID = "BU000"
	}
	return append([]JobLevel(nil), s.levels[tenantID][businessUnitID]...), "SHARE", nil
}

func (s *jobcatalogMemoryStore) CreateJobProfile(_ context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, _ string, familyCodes []string, primaryFamilyCode string) error {
	s.ensure(tenantID)
	if businessUnitID == "" {
		businessUnitID = "BU000"
	}
	if s.profiles[tenantID][businessUnitID] == nil {
		s.profiles[tenantID][businessUnitID] = []JobProfile{}
	}
	s.profiles[tenantID][businessUnitID] = append(s.profiles[tenantID][businessUnitID], JobProfile{
		ID:                strconv.Itoa(len(s.profiles[tenantID][businessUnitID]) + 1),
		Code:              code,
		Name:              name,
		IsActive:          true,
		EffectiveDay:      effectiveDate,
		FamilyCodesCSV:    strings.Join(familyCodes, ","),
		PrimaryFamilyCode: primaryFamilyCode,
	})
	return nil
}

func (s *jobcatalogMemoryStore) ListJobProfiles(_ context.Context, tenantID string, businessUnitID string, _ string) ([]JobProfile, string, error) {
	s.ensure(tenantID)
	if businessUnitID == "" {
		businessUnitID = "BU000"
	}
	return append([]JobProfile(nil), s.profiles[tenantID][businessUnitID]...), "SHARE", nil
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

func (s *jobcatalogPGStore) ListBusinessUnits(ctx context.Context, tenantID string) ([]BusinessUnit, error) {
	var out []BusinessUnit
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID)
		if err != nil {
			return err
		}

		rows, err := tx.Query(ctx, `
SELECT business_unit_id, name, status
FROM orgunit.business_units
WHERE tenant_id = $1::uuid
ORDER BY business_unit_id ASC
`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var r BusinessUnit
			if err := rows.Scan(&r.BusinessUnitID, &r.Name, &r.Status); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (s *jobcatalogPGStore) ResolveSetID(ctx context.Context, tenantID string, businessUnitID string, recordGroup string) (string, error) {
	var out string
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID)
		if err != nil {
			return err
		}

		v, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, recordGroup)
		if err != nil {
			return err
		}
		out = v
		return nil
	})
	return out, err
}

func (s *jobcatalogPGStore) CreateJobFamilyGroup(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, description string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID); err != nil {
			return err
		}

		resolved, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, setid.RecordGroupJobCatalog)
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

func (s *jobcatalogPGStore) ListJobFamilyGroups(ctx context.Context, tenantID string, businessUnitID string, asOfDate string) ([]JobFamilyGroup, string, error) {
	var out []JobFamilyGroup
	var resolved string
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID); err != nil {
			return err
		}
		v, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, setid.RecordGroupJobCatalog)
		if err != nil {
			return err
		}
		resolved = v

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
 AND v.setid = $2::text
 AND v.job_family_group_id = g.id
WHERE g.tenant_id = $1::uuid
  AND g.setid = $2::text
  AND v.validity @> $3::date
ORDER BY g.code ASC
`, tenantID, resolved, asOfDate)
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
	return out, resolved, err
}

func (s *jobcatalogPGStore) CreateJobFamily(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, description string, groupCode string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID); err != nil {
			return err
		}

		resolved, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, setid.RecordGroupJobCatalog)
		if err != nil {
			return err
		}

		var groupID string
		if err := tx.QueryRow(ctx, `
SELECT g.id::text
FROM jobcatalog.job_family_groups g
WHERE g.tenant_id = $1::uuid
  AND g.setid = $2::text
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

func (s *jobcatalogPGStore) UpdateJobFamilyGroup(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, familyCode string, groupCode string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID); err != nil {
			return err
		}

		resolved, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, setid.RecordGroupJobCatalog)
		if err != nil {
			return err
		}

		var groupID string
		if err := tx.QueryRow(ctx, `
SELECT g.id::text
FROM jobcatalog.job_family_groups g
WHERE g.tenant_id = $1::uuid
  AND g.setid = $2::text
  AND g.code = $3::text
`, tenantID, resolved, groupCode).Scan(&groupID); err != nil {
			return err
		}

		var familyID string
		if err := tx.QueryRow(ctx, `
SELECT f.id::text
FROM jobcatalog.job_families f
WHERE f.tenant_id = $1::uuid
  AND f.setid = $2::text
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

func (s *jobcatalogPGStore) ListJobFamilies(ctx context.Context, tenantID string, businessUnitID string, asOfDate string) ([]JobFamily, string, error) {
	var out []JobFamily
	var resolved string
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID); err != nil {
			return err
		}
		v, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, setid.RecordGroupJobCatalog)
		if err != nil {
			return err
		}
		resolved = v

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
 AND v.setid = $2::text
 AND v.job_family_id = f.id
JOIN jobcatalog.job_family_groups g
  ON g.tenant_id = $1::uuid
 AND g.setid = $2::text
 AND g.id = v.job_family_group_id
WHERE f.tenant_id = $1::uuid
  AND f.setid = $2::text
  AND v.validity @> $3::date
ORDER BY f.code ASC
`, tenantID, resolved, asOfDate)
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
	return out, resolved, err
}

func (s *jobcatalogPGStore) CreateJobLevel(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, description string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID); err != nil {
			return err
		}

		resolved, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, setid.RecordGroupJobCatalog)
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

func (s *jobcatalogPGStore) ListJobLevels(ctx context.Context, tenantID string, businessUnitID string, asOfDate string) ([]JobLevel, string, error) {
	var out []JobLevel
	var resolved string
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID); err != nil {
			return err
		}
		v, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, setid.RecordGroupJobCatalog)
		if err != nil {
			return err
		}
		resolved = v

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
	 AND v.setid = $2::text
	 AND v.job_level_id = l.id
	WHERE l.tenant_id = $1::uuid
	  AND l.setid = $2::text
	  AND v.validity @> $3::date
	ORDER BY l.code ASC
	`, tenantID, resolved, asOfDate)
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
	return out, resolved, err
}

func (s *jobcatalogPGStore) CreateJobProfile(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, description string, familyCodes []string, primaryFamilyCode string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID); err != nil {
			return err
		}

		resolved, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, setid.RecordGroupJobCatalog)
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
  AND f.setid = $2::text
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

func (s *jobcatalogPGStore) ListJobProfiles(ctx context.Context, tenantID string, businessUnitID string, asOfDate string) ([]JobProfile, string, error) {
	var out []JobProfile
	var resolved string
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID); err != nil {
			return err
		}
		v, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, setid.RecordGroupJobCatalog)
		if err != nil {
			return err
		}
		resolved = v

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
     AND f2.setid = $2::text
     AND f2.id = pf2.job_family_id
    WHERE pf2.tenant_id = $1::uuid
      AND pf2.setid = $2::text
      AND pf2.job_profile_version_id = v.id
      AND pf2.is_primary = true
    LIMIT 1
  ), '') AS primary_family_code
FROM jobcatalog.job_profiles p
JOIN jobcatalog.job_profile_versions v
  ON v.tenant_id = $1::uuid
 AND v.setid = $2::text
 AND v.job_profile_id = p.id
 AND v.validity @> $3::date
LEFT JOIN jobcatalog.job_profile_version_job_families pf
  ON pf.tenant_id = $1::uuid
 AND pf.setid = $2::text
 AND pf.job_profile_version_id = v.id
LEFT JOIN jobcatalog.job_families f
  ON f.tenant_id = $1::uuid
 AND f.setid = $2::text
 AND f.id = pf.job_family_id
WHERE p.tenant_id = $1::uuid
  AND p.setid = $2::text
GROUP BY p.id, p.code, v.id, v.name, v.is_active, v.validity
ORDER BY p.code ASC
`, tenantID, resolved, asOfDate)
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
	return out, resolved, err
}

func handleJobCatalog(w http.ResponseWriter, r *http.Request, store JobCatalogStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}

	bus, err := store.ListBusinessUnits(r.Context(), tenant.ID)
	if err != nil {
		writePage(w, r, renderJobCatalog(nil, nil, nil, nil, nil, tenant, "", err.Error(), asOf, ""))
		return
	}

	activeBUs := make([]BusinessUnit, 0, len(bus))
	for _, bu := range bus {
		if bu.Status == "active" {
			activeBUs = append(activeBUs, bu)
		}
	}
	sort.Slice(activeBUs, func(i, j int) bool { return activeBUs[i].BusinessUnitID < activeBUs[j].BusinessUnitID })

	buID := strings.TrimSpace(r.URL.Query().Get("business_unit_id"))
	if buID == "" {
		buID = "BU000"
	}

	list := func(errHint string) (groups []JobFamilyGroup, families []JobFamily, levels []JobLevel, profiles []JobProfile, resolved string, errMsg string) {
		mergeMsg := func(hint string, msg string) string {
			if hint == "" {
				return msg
			}
			if msg == "" {
				return hint
			}
			return hint + "；" + msg
		}

		groups, resolvedGroups, err := store.ListJobFamilyGroups(r.Context(), tenant.ID, buID, asOf)
		if err != nil {
			return nil, nil, nil, nil, "", mergeMsg(errHint, err.Error())
		}

		families, resolvedFamilies, err := store.ListJobFamilies(r.Context(), tenant.ID, buID, asOf)
		if err != nil {
			return groups, nil, nil, nil, resolvedGroups, mergeMsg(errHint, err.Error())
		}

		levels, resolvedLevels, err := store.ListJobLevels(r.Context(), tenant.ID, buID, asOf)
		if err != nil {
			return groups, families, nil, nil, resolvedGroups, mergeMsg(errHint, err.Error())
		}

		profiles, resolvedProfiles, err := store.ListJobProfiles(r.Context(), tenant.ID, buID, asOf)
		if err != nil {
			return groups, families, levels, nil, resolvedGroups, mergeMsg(errHint, err.Error())
		}

		resolved = resolvedGroups
		if resolved == "" {
			resolved = resolvedFamilies
		}
		if resolved == "" {
			resolved = resolvedLevels
		}
		if resolved == "" {
			resolved = resolvedProfiles
		}
		return groups, families, levels, profiles, resolved, errHint
	}

	switch r.Method {
	case http.MethodGet:
		groups, families, levels, profiles, resolved, errMsg := list("")
		writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			groups, families, levels, profiles, resolved, errMsg := list("bad form")
			writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
			return
		}

		action := strings.TrimSpace(strings.ToLower(r.Form.Get("action")))
		if action == "" {
			action = "create_job_family_group"
		}
		switch action {
		case "create_job_family_group", "create_job_family", "update_job_family_group", "create_job_level", "create_job_profile":
		default:
			groups, families, levels, profiles, resolved, errMsg := list("unknown action")
			writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
			return
		}

		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		if effectiveDate == "" {
			effectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			groups, families, levels, profiles, resolved, errMsg := list("effective_date 无效: " + err.Error())
			writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
			return
		}

		formBU := strings.TrimSpace(r.Form.Get("business_unit_id"))
		if formBU != "" {
			buID = formBU
		}

		switch action {
		case "create_job_family_group":
			code := strings.TrimSpace(r.Form.Get("job_family_group_code"))
			name := strings.TrimSpace(r.Form.Get("job_family_group_name"))
			desc := strings.TrimSpace(r.Form.Get("job_family_group_description"))
			if code == "" || name == "" {
				groups, families, levels, profiles, resolved, errMsg := list("code/name is required")
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
				return
			}
			if err := store.CreateJobFamilyGroup(r.Context(), tenant.ID, buID, effectiveDate, code, name, desc); err != nil {
				groups, families, levels, profiles, resolved, errMsg := list(err.Error())
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
				return
			}
		case "create_job_family":
			code := strings.TrimSpace(r.Form.Get("job_family_code"))
			name := strings.TrimSpace(r.Form.Get("job_family_name"))
			desc := strings.TrimSpace(r.Form.Get("job_family_description"))
			groupCode := strings.TrimSpace(r.Form.Get("job_family_group_code"))
			if code == "" || name == "" || groupCode == "" {
				groups, families, levels, profiles, resolved, errMsg := list("code/name/group is required")
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
				return
			}
			if err := store.CreateJobFamily(r.Context(), tenant.ID, buID, effectiveDate, code, name, desc, groupCode); err != nil {
				groups, families, levels, profiles, resolved, errMsg := list(err.Error())
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
				return
			}
		case "update_job_family_group":
			familyCode := strings.TrimSpace(r.Form.Get("job_family_code"))
			groupCode := strings.TrimSpace(r.Form.Get("job_family_group_code"))
			if familyCode == "" || groupCode == "" {
				groups, families, levels, profiles, resolved, errMsg := list("family/group is required")
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
				return
			}
			if err := store.UpdateJobFamilyGroup(r.Context(), tenant.ID, buID, effectiveDate, familyCode, groupCode); err != nil {
				groups, families, levels, profiles, resolved, errMsg := list(err.Error())
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
				return
			}
		case "create_job_level":
			code := strings.TrimSpace(r.Form.Get("job_level_code"))
			name := strings.TrimSpace(r.Form.Get("job_level_name"))
			desc := strings.TrimSpace(r.Form.Get("job_level_description"))
			if code == "" || name == "" {
				groups, families, levels, profiles, resolved, errMsg := list("code/name is required")
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
				return
			}
			if err := store.CreateJobLevel(r.Context(), tenant.ID, buID, effectiveDate, code, name, desc); err != nil {
				groups, families, levels, profiles, resolved, errMsg := list(err.Error())
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
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
				groups, families, levels, profiles, resolved, errMsg := list("code/name/families/primary is required")
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
				return
			}
			if err := store.CreateJobProfile(r.Context(), tenant.ID, buID, effectiveDate, code, name, desc, familyCodes, primary); err != nil {
				groups, families, levels, profiles, resolved, errMsg := list(err.Error())
				writePage(w, r, renderJobCatalog(groups, families, levels, profiles, activeBUs, tenant, buID, errMsg, asOf, resolved))
				return
			}
		}

		http.Redirect(w, r, "/org/job-catalog?business_unit_id="+url.QueryEscape(buID)+"&as_of="+url.QueryEscape(effectiveDate), http.StatusSeeOther)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func renderJobCatalog(groups []JobFamilyGroup, families []JobFamily, levels []JobLevel, profiles []JobProfile, businessUnits []BusinessUnit, tenant Tenant, businessUnitID string, errMsg string, asOf string, resolvedSetID string) string {
	var b strings.Builder
	b.WriteString("<h1>Job Catalog</h1>")
	b.WriteString("<p>Tenant: " + html.EscapeString(tenant.Name) + "</p>")
	b.WriteString(`<p><a href="/org/setid" hx-get="/org/setid" hx-target="#content" hx-push-url="true">SetID Governance</a></p>`)

	b.WriteString(`<form method="GET" action="/org/job-catalog">`)
	b.WriteString(`<label>As-of <input type="date" name="as_of" value="` + html.EscapeString(asOf) + `" /></label> `)
	b.WriteString(`<label>Business Unit <select name="business_unit_id">`)
	for _, bu := range businessUnits {
		selected := ""
		if bu.BusinessUnitID == businessUnitID {
			selected = " selected"
		}
		b.WriteString(`<option value="` + html.EscapeString(bu.BusinessUnitID) + `"` + selected + `>` + html.EscapeString(bu.BusinessUnitID) + ` - ` + html.EscapeString(bu.Name) + `</option>`)
	}
	b.WriteString(`</select></label> `)
	b.WriteString(`<button type="submit">Apply</button>`)
	b.WriteString(`</form>`)

	if resolvedSetID != "" {
		b.WriteString(`<p>Resolved SetID: <code>` + html.EscapeString(resolvedSetID) + `</code></p>`)
	}

	if errMsg != "" {
		b.WriteString(`<div style="padding:8px;border:1px solid #c00;color:#c00">` + html.EscapeString(errMsg) + `</div>`)
	}

	postAction := "/org/job-catalog?business_unit_id=" + url.QueryEscape(businessUnitID) + "&as_of=" + url.QueryEscape(asOf)
	b.WriteString(`<h2>Create Job Family Group</h2>`)
	b.WriteString(`<form method="POST" action="` + postAction + `">`)
	b.WriteString(`<input type="hidden" name="action" value="create_job_family_group" />`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>Business Unit <input name="business_unit_id" value="` + html.EscapeString(businessUnitID) + `" maxlength="5" /></label><br/>`)
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
	b.WriteString(`<label>Business Unit <input name="business_unit_id" value="` + html.EscapeString(businessUnitID) + `" maxlength="5" /></label><br/>`)
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
	b.WriteString(`<label>Business Unit <input name="business_unit_id" value="` + html.EscapeString(businessUnitID) + `" maxlength="5" /></label><br/>`)
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
	b.WriteString(`<label>Business Unit <input name="business_unit_id" value="` + html.EscapeString(businessUnitID) + `" maxlength="5" /></label><br/>`)
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
	b.WriteString(`<label>Business Unit <input name="business_unit_id" value="` + html.EscapeString(businessUnitID) + `" maxlength="5" /></label><br/>`)
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
