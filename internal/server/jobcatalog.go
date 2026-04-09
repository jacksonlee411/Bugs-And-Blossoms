package server

import (
	"context"
	"net/http"
	"strings"

	jobcatalogmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog"
	jobcatalogports "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/domain/ports"
	jobcatalogtypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/domain/types"
	jobcatalogservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/services"
)

type JobFamilyGroup = jobcatalogtypes.JobFamilyGroup
type JobLevel = jobcatalogtypes.JobLevel
type JobFamily = jobcatalogtypes.JobFamily
type JobProfile = jobcatalogtypes.JobProfile
type JobCatalogStore = jobcatalogports.JobCatalogStore
type JobCatalogPackage = jobcatalogtypes.JobCatalogPackage

func newJobCatalogPGStore(pool pgBeginner) JobCatalogStore {
	return jobcatalogmodule.NewPGStore(pool)
}

func newJobCatalogMemoryStore() JobCatalogStore {
	return jobcatalogmodule.NewMemoryStore()
}

func normalizeSetID(input string) string {
	return jobcatalogservices.NormalizeSetID(input)
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
	return jobcatalogservices.JobCatalogView(v).ListSetID()
}

func normalizePackageCode(input string) string {
	return jobcatalogservices.NormalizePackageCode(input)
}

func canEditDefltPackage(ctx context.Context) bool {
	return jobcatalogservices.CanEditDefltPackage(jobCatalogPrincipalFromContext(ctx))
}

func ownerSetIDEditable(ctx context.Context, setidStore jobCatalogSetIDStore, tenantID string, ownerSetID string) bool {
	return jobcatalogservices.OwnerSetIDEditable(ctx, jobCatalogPrincipalFromContext(ctx), adaptJobCatalogSetIDStore(setidStore), tenantID, ownerSetID)
}

func loadOwnedJobCatalogPackages(ctx context.Context, setidStore jobCatalogSetIDStore, tenantID string, asOf string) ([]OwnedScopePackage, error) {
	rows, err := jobcatalogservices.LoadOwnedJobCatalogPackages(ctx, jobCatalogPrincipalFromContext(ctx), adaptJobCatalogSetIDStore(setidStore), tenantID, asOf)
	if err != nil {
		return nil, err
	}
	return toServerOwnedScopePackages(rows), nil
}

func resolveJobCatalogView(ctx context.Context, store JobCatalogStore, setidStore jobCatalogSetIDStore, tenantID string, asOf string, packageCode string, setID string) (jobCatalogView, string) {
	view, errMsg := jobcatalogservices.ResolveJobCatalogView(
		ctx,
		jobCatalogPrincipalFromContext(ctx),
		jobCatalogStoreAdapter{store: store},
		adaptJobCatalogSetIDStore(setidStore),
		tenantID,
		asOf,
		packageCode,
		setID,
	)
	return jobCatalogView(view), errMsg
}

type jobCatalogStoreAdapter struct {
	store JobCatalogStore
}

func (a jobCatalogStoreAdapter) ResolveJobCatalogPackageByCode(ctx context.Context, tenantID string, packageCode string, asOfDate string) (jobcatalogservices.JobCatalogPackage, error) {
	pkg, err := a.store.ResolveJobCatalogPackageByCode(ctx, tenantID, packageCode, asOfDate)
	if err != nil {
		return jobcatalogservices.JobCatalogPackage{}, err
	}
	return jobcatalogservices.JobCatalogPackage{
		PackageUUID: pkg.PackageUUID,
		PackageCode: pkg.PackageCode,
		OwnerSetID:  pkg.OwnerSetID,
	}, nil
}

func (a jobCatalogStoreAdapter) ResolveJobCatalogPackageBySetID(ctx context.Context, tenantID string, setID string, asOfDate string) (string, error) {
	return a.store.ResolveJobCatalogPackageBySetID(ctx, tenantID, setID, asOfDate)
}

type jobCatalogSetIDStoreAdapter struct {
	store jobCatalogSetIDStore
}

func (a jobCatalogSetIDStoreAdapter) ListSetIDs(ctx context.Context, tenantID string) ([]jobcatalogservices.SetIDRecord, error) {
	rows, err := a.store.ListSetIDs(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]jobcatalogservices.SetIDRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, jobcatalogservices.SetIDRecord{
			SetID:  row.SetID,
			Status: row.Status,
		})
	}
	return out, nil
}

func (a jobCatalogSetIDStoreAdapter) ListOwnedScopePackages(ctx context.Context, tenantID string, scopeCode string, asOfDate string) ([]jobcatalogservices.OwnedScopePackage, error) {
	rows, err := a.store.ListOwnedScopePackages(ctx, tenantID, scopeCode, asOfDate)
	if err != nil {
		return nil, err
	}
	out := make([]jobcatalogservices.OwnedScopePackage, 0, len(rows))
	for _, row := range rows {
		out = append(out, jobcatalogservices.OwnedScopePackage{
			PackageID:     row.PackageID,
			ScopeCode:     row.ScopeCode,
			PackageCode:   row.PackageCode,
			OwnerSetID:    row.OwnerSetID,
			Name:          row.Name,
			Status:        row.Status,
			EffectiveDate: row.EffectiveDate,
		})
	}
	return out, nil
}

func adaptJobCatalogSetIDStore(store jobCatalogSetIDStore) jobcatalogservices.SetIDStore {
	if store == nil {
		return nil
	}
	return jobCatalogSetIDStoreAdapter{store: store}
}

func jobCatalogPrincipalFromContext(ctx context.Context) jobcatalogservices.Principal {
	p, _ := currentPrincipal(ctx)
	return jobcatalogservices.Principal{
		RoleSlug: p.RoleSlug,
		Status:   p.Status,
	}
}

func toServerOwnedScopePackages(rows []jobcatalogservices.OwnedScopePackage) []OwnedScopePackage {
	if rows == nil {
		return nil
	}
	out := make([]OwnedScopePackage, 0, len(rows))
	for _, row := range rows {
		out = append(out, OwnedScopePackage{
			PackageID:     row.PackageID,
			ScopeCode:     row.ScopeCode,
			PackageCode:   row.PackageCode,
			OwnerSetID:    row.OwnerSetID,
			Name:          row.Name,
			Status:        row.Status,
			EffectiveDate: row.EffectiveDate,
		})
	}
	return out
}

func jobCatalogStatusForError(errMsg string) int {
	if strings.Contains(errMsg, "DEFLT_EDIT_FORBIDDEN") || strings.Contains(errMsg, "OWNER_SETID_FORBIDDEN") {
		return http.StatusForbidden
	}
	if strings.Contains(errMsg, "PACKAGE_CODE_MISMATCH") {
		return http.StatusUnprocessableEntity
	}
	if strings.Contains(errMsg, "JOBCATALOG_SETID_INVALID") || strings.Contains(errMsg, "PACKAGE_NOT_FOUND") || strings.Contains(errMsg, "PACKAGE_OWNER_INVALID") {
		return http.StatusUnprocessableEntity
	}
	return http.StatusUnprocessableEntity
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
