package persistence

import (
	"context"
	"errors"
	"strconv"
	"strings"

	jobcatalogtypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/domain/types"
)

type MemoryStore struct {
	groups   map[string]map[string][]jobcatalogtypes.JobFamilyGroup
	families map[string]map[string][]jobcatalogtypes.JobFamily
	levels   map[string]map[string][]jobcatalogtypes.JobLevel
	profiles map[string]map[string][]jobcatalogtypes.JobProfile
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		groups:   make(map[string]map[string][]jobcatalogtypes.JobFamilyGroup),
		families: make(map[string]map[string][]jobcatalogtypes.JobFamily),
		levels:   make(map[string]map[string][]jobcatalogtypes.JobLevel),
		profiles: make(map[string]map[string][]jobcatalogtypes.JobProfile),
	}
}

func (s *MemoryStore) ensure(tenantID string) {
	if s.groups[tenantID] == nil {
		s.groups[tenantID] = make(map[string][]jobcatalogtypes.JobFamilyGroup)
	}
	if s.families[tenantID] == nil {
		s.families[tenantID] = make(map[string][]jobcatalogtypes.JobFamily)
	}
	if s.levels[tenantID] == nil {
		s.levels[tenantID] = make(map[string][]jobcatalogtypes.JobLevel)
	}
	if s.profiles[tenantID] == nil {
		s.profiles[tenantID] = make(map[string][]jobcatalogtypes.JobProfile)
	}
}

func (s *MemoryStore) ResolveJobCatalogPackageByCode(_ context.Context, _ string, packageCode string, _ string) (jobcatalogtypes.JobCatalogPackage, error) {
	packageCode = strings.ToUpper(strings.TrimSpace(packageCode))
	if packageCode == "" {
		return jobcatalogtypes.JobCatalogPackage{}, errors.New("PACKAGE_CODE_INVALID")
	}
	return jobcatalogtypes.JobCatalogPackage{
		PackageUUID: packageCode,
		PackageCode: packageCode,
		OwnerSetID:  packageCode,
	}, nil
}

func (s *MemoryStore) ResolveJobCatalogPackageBySetID(_ context.Context, _ string, setID string, _ string) (string, error) {
	setID = normalizeSetID(setID)
	if setID == "" {
		return "", errors.New("setid is required")
	}
	return setID, nil
}

func (s *MemoryStore) CreateJobFamilyGroup(_ context.Context, tenantID string, setID string, effectiveDate string, code string, name string, _ string) error {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return errors.New("setid is required")
	}
	if s.groups[tenantID][setID] == nil {
		s.groups[tenantID][setID] = []jobcatalogtypes.JobFamilyGroup{}
	}
	s.groups[tenantID][setID] = append(s.groups[tenantID][setID], jobcatalogtypes.JobFamilyGroup{
		JobFamilyGroupUUID: strconv.Itoa(len(s.groups[tenantID][setID]) + 1),
		JobFamilyGroupCode: code,
		Name:               name,
		IsActive:           true,
		EffectiveDay:       effectiveDate,
	})
	return nil
}

func (s *MemoryStore) ListJobFamilyGroups(_ context.Context, tenantID string, setID string, _ string) ([]jobcatalogtypes.JobFamilyGroup, error) {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return nil, errors.New("setid is required")
	}
	return append([]jobcatalogtypes.JobFamilyGroup(nil), s.groups[tenantID][setID]...), nil
}

func (s *MemoryStore) CreateJobFamily(_ context.Context, tenantID string, setID string, effectiveDate string, code string, name string, _ string, groupCode string) error {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return errors.New("setid is required")
	}
	if s.families[tenantID][setID] == nil {
		s.families[tenantID][setID] = []jobcatalogtypes.JobFamily{}
	}
	s.families[tenantID][setID] = append(s.families[tenantID][setID], jobcatalogtypes.JobFamily{
		JobFamilyUUID:      strconv.Itoa(len(s.families[tenantID][setID]) + 1),
		JobFamilyCode:      code,
		JobFamilyGroupCode: groupCode,
		Name:               name,
		IsActive:           true,
		EffectiveDay:       effectiveDate,
	})
	return nil
}

func (s *MemoryStore) UpdateJobFamilyGroup(_ context.Context, tenantID string, setID string, effectiveDate string, familyCode string, groupCode string) error {
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

func (s *MemoryStore) ListJobFamilies(_ context.Context, tenantID string, setID string, _ string) ([]jobcatalogtypes.JobFamily, error) {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return nil, errors.New("setid is required")
	}
	return append([]jobcatalogtypes.JobFamily(nil), s.families[tenantID][setID]...), nil
}

func (s *MemoryStore) CreateJobLevel(_ context.Context, tenantID string, setID string, effectiveDate string, code string, name string, _ string) error {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return errors.New("setid is required")
	}
	if s.levels[tenantID][setID] == nil {
		s.levels[tenantID][setID] = []jobcatalogtypes.JobLevel{}
	}
	s.levels[tenantID][setID] = append(s.levels[tenantID][setID], jobcatalogtypes.JobLevel{
		JobLevelUUID: strconv.Itoa(len(s.levels[tenantID][setID]) + 1),
		JobLevelCode: code,
		Name:         name,
		IsActive:     true,
		EffectiveDay: effectiveDate,
	})
	return nil
}

func (s *MemoryStore) ListJobLevels(_ context.Context, tenantID string, setID string, _ string) ([]jobcatalogtypes.JobLevel, error) {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return nil, errors.New("setid is required")
	}
	return append([]jobcatalogtypes.JobLevel(nil), s.levels[tenantID][setID]...), nil
}

func (s *MemoryStore) CreateJobProfile(_ context.Context, tenantID string, setID string, effectiveDate string, code string, name string, _ string, familyCodes []string, primaryFamilyCode string) error {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return errors.New("setid is required")
	}
	if s.profiles[tenantID][setID] == nil {
		s.profiles[tenantID][setID] = []jobcatalogtypes.JobProfile{}
	}
	s.profiles[tenantID][setID] = append(s.profiles[tenantID][setID], jobcatalogtypes.JobProfile{
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

func (s *MemoryStore) ListJobProfiles(_ context.Context, tenantID string, setID string, _ string) ([]jobcatalogtypes.JobProfile, error) {
	s.ensure(tenantID)
	setID = normalizeSetID(setID)
	if setID == "" {
		return nil, errors.New("setid is required")
	}
	return append([]jobcatalogtypes.JobProfile(nil), s.profiles[tenantID][setID]...), nil
}

func normalizeSetID(input string) string {
	return strings.ToUpper(strings.TrimSpace(input))
}
