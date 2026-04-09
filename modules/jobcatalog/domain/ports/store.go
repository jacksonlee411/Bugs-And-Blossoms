package ports

import (
	"context"

	jobcatalogtypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/domain/types"
)

type JobCatalogStore interface {
	ResolveJobCatalogPackageByCode(ctx context.Context, tenantID string, packageCode string, asOfDate string) (jobcatalogtypes.JobCatalogPackage, error)
	ResolveJobCatalogPackageBySetID(ctx context.Context, tenantID string, setID string, asOfDate string) (string, error)
	CreateJobFamilyGroup(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string) error
	ListJobFamilyGroups(ctx context.Context, tenantID string, setID string, asOfDate string) ([]jobcatalogtypes.JobFamilyGroup, error)
	CreateJobFamily(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string, groupCode string) error
	UpdateJobFamilyGroup(ctx context.Context, tenantID string, setID string, effectiveDate string, familyCode string, groupCode string) error
	ListJobFamilies(ctx context.Context, tenantID string, setID string, asOfDate string) ([]jobcatalogtypes.JobFamily, error)
	CreateJobLevel(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string) error
	ListJobLevels(ctx context.Context, tenantID string, setID string, asOfDate string) ([]jobcatalogtypes.JobLevel, error)
	CreateJobProfile(ctx context.Context, tenantID string, setID string, effectiveDate string, code string, name string, description string, familyCodes []string, primaryFamilyCode string) error
	ListJobProfiles(ctx context.Context, tenantID string, setID string, asOfDate string) ([]jobcatalogtypes.JobProfile, error)
}
