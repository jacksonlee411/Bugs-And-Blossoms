package services

import (
	"context"
	"strings"

	jobcatalogtypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/jobcatalog/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type Principal struct {
	RoleSlug string
	Status   string
}

type SetIDRecord struct {
	SetID  string
	Status string
}

type OwnedScopePackage struct {
	PackageID     string
	ScopeCode     string
	PackageCode   string
	OwnerSetID    string
	Name          string
	Status        string
	EffectiveDate string
}

type JobCatalogPackage = jobcatalogtypes.JobCatalogPackage

type JobCatalogView struct {
	PackageCode  string
	OwnerSetID   string
	SetID        string
	ReadOnly     bool
	HasSelection bool
}

type JobCatalogStore interface {
	ResolveJobCatalogPackageByCode(ctx context.Context, tenantID string, packageCode string, asOfDate string) (JobCatalogPackage, error)
	ResolveJobCatalogPackageBySetID(ctx context.Context, tenantID string, setID string, asOfDate string) (string, error)
}

type SetIDStore interface {
	ListSetIDs(ctx context.Context, tenantID string) ([]SetIDRecord, error)
	ListOwnedScopePackages(ctx context.Context, tenantID string, scopeCode string, asOfDate string) ([]OwnedScopePackage, error)
}

func (v JobCatalogView) ListSetID() string {
	if !v.HasSelection {
		return ""
	}
	if v.ReadOnly {
		return v.SetID
	}
	return v.OwnerSetID
}

func NormalizeSetID(input string) string {
	return strings.ToUpper(strings.TrimSpace(input))
}

func NormalizePackageCode(input string) string {
	return strings.ToUpper(strings.TrimSpace(input))
}

func CanEditOwnedScopePackages(principal Principal) bool {
	role := strings.ToLower(strings.TrimSpace(principal.RoleSlug))
	if role == "" {
		return false
	}
	return role == authz.RoleTenantAdmin
}

func CanEditDefltPackage(principal Principal) bool {
	if !CanEditOwnedScopePackages(principal) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(principal.Status), "active")
}

func OwnerSetIDEditable(ctx context.Context, principal Principal, setidStore SetIDStore, tenantID string, ownerSetID string) bool {
	if !CanEditOwnedScopePackages(principal) {
		return false
	}
	if setidStore == nil {
		return false
	}
	ownerSetID = NormalizeSetID(ownerSetID)
	if ownerSetID == "" {
		return false
	}
	rows, err := setidStore.ListSetIDs(ctx, tenantID)
	if err != nil {
		return false
	}
	for _, row := range rows {
		if NormalizeSetID(row.SetID) == ownerSetID && strings.EqualFold(strings.TrimSpace(row.Status), "active") {
			return true
		}
	}
	return false
}

func LoadOwnedJobCatalogPackages(ctx context.Context, principal Principal, setidStore SetIDStore, tenantID string, asOf string) ([]OwnedScopePackage, error) {
	if setidStore == nil {
		return []OwnedScopePackage{}, nil
	}
	if !CanEditOwnedScopePackages(principal) {
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

func ResolveJobCatalogView(ctx context.Context, principal Principal, store JobCatalogStore, setidStore SetIDStore, tenantID string, asOf string, packageCode string, setID string) (JobCatalogView, string) {
	view := JobCatalogView{PackageCode: packageCode}
	if packageCode == "" && setID == "" {
		return view, ""
	}
	if setID != "" {
		view.SetID = NormalizeSetID(setID)
		view.OwnerSetID = view.SetID
		view.HasSelection = true
		if _, err := store.ResolveJobCatalogPackageBySetID(ctx, tenantID, view.SetID, asOf); err != nil {
			return view, err.Error()
		}
		view.ReadOnly = !OwnerSetIDEditable(ctx, principal, setidStore, tenantID, view.SetID)
		return view, ""
	}

	view.HasSelection = true
	if !CanEditOwnedScopePackages(principal) {
		return view, "OWNER_SETID_FORBIDDEN"
	}
	pkg, err := store.ResolveJobCatalogPackageByCode(ctx, tenantID, packageCode, asOf)
	if err != nil {
		return view, err.Error()
	}
	view.PackageCode = pkg.PackageCode
	view.OwnerSetID = pkg.OwnerSetID
	if !OwnerSetIDEditable(ctx, principal, setidStore, tenantID, pkg.OwnerSetID) {
		return view, "OWNER_SETID_FORBIDDEN"
	}
	if strings.EqualFold(pkg.PackageCode, "DEFLT") && !CanEditDefltPackage(principal) {
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
