package server

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type jobcatalogRows struct {
	idx     int
	rows    [][]any
	scanErr error
	err     error
}

func (r *jobcatalogRows) Close()                        {}
func (r *jobcatalogRows) Err() error                    { return r.err }
func (r *jobcatalogRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *jobcatalogRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *jobcatalogRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *jobcatalogRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.rows[r.idx-1]
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *bool:
			*d = row[i].(bool)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}
func (r *jobcatalogRows) Values() ([]any, error) { return nil, nil }
func (r *jobcatalogRows) RawValues() [][]byte    { return nil }
func (r *jobcatalogRows) Conn() *pgx.Conn        { return nil }

type jobCatalogResolveStub struct{}

func (jobCatalogResolveStub) ResolveJobCatalogPackageByCode(_ context.Context, _ string, packageCode string, _ string) (JobCatalogPackage, error) {
	packageCode = normalizePackageCode(packageCode)
	if packageCode == "" {
		return JobCatalogPackage{}, errors.New("PACKAGE_CODE_INVALID")
	}
	return JobCatalogPackage{PackageUUID: packageCode, PackageCode: packageCode, OwnerSetID: packageCode}, nil
}

func (jobCatalogResolveStub) ResolveJobCatalogPackageBySetID(_ context.Context, _ string, setID string, _ string) (string, error) {
	setID = normalizeSetID(setID)
	if setID == "" {
		return "", errors.New("setid is required")
	}
	return setID, nil
}

type jobCatalogSetIDStoreStub struct {
	setids []SetID
	owned  []OwnedScopePackage
	err    error
}

func (s jobCatalogSetIDStoreStub) ListSetIDs(_ context.Context, _ string) ([]SetID, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]SetID(nil), s.setids...), nil
}

func (s jobCatalogSetIDStoreStub) ListOwnedScopePackages(_ context.Context, _ string, _ string, _ string) ([]OwnedScopePackage, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]OwnedScopePackage(nil), s.owned...), nil
}

type resolveJobCatalogStoreStub struct {
	pkg              JobCatalogPackage
	pkgErr           error
	setidPackageUUID string
	setidErr         error
}

func (s resolveJobCatalogStoreStub) ResolveJobCatalogPackageByCode(_ context.Context, _ string, _ string, _ string) (JobCatalogPackage, error) {
	if s.pkgErr != nil {
		return JobCatalogPackage{}, s.pkgErr
	}
	return s.pkg, nil
}

func (s resolveJobCatalogStoreStub) ResolveJobCatalogPackageBySetID(_ context.Context, _ string, _ string, _ string) (string, error) {
	if s.setidErr != nil {
		return "", s.setidErr
	}
	if s.setidPackageUUID != "" {
		return s.setidPackageUUID, nil
	}
	return "", nil
}

func (s resolveJobCatalogStoreStub) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s resolveJobCatalogStoreStub) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, error) {
	return nil, nil
}
func (s resolveJobCatalogStoreStub) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s resolveJobCatalogStoreStub) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s resolveJobCatalogStoreStub) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, error) {
	return nil, nil
}
func (s resolveJobCatalogStoreStub) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s resolveJobCatalogStoreStub) ListJobLevels(context.Context, string, string, string) ([]JobLevel, error) {
	return nil, nil
}
func (s resolveJobCatalogStoreStub) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s resolveJobCatalogStoreStub) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, nil
}

type orgStoreStub struct {
	nodes []OrgUnitNode
	err   error
}

func (s orgStoreStub) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]OrgUnitNode(nil), s.nodes...), nil
}
func (s orgStoreStub) CreateNodeCurrent(context.Context, string, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, s.err
}
func (s orgStoreStub) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return s.err
}
func (s orgStoreStub) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return s.err
}
func (s orgStoreStub) DisableNodeCurrent(context.Context, string, string, string) error {
	return s.err
}
func (s orgStoreStub) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return s.err
}
func (s orgStoreStub) ResolveOrgID(context.Context, string, string) (int, error) {
	return 0, s.err
}
func (s orgStoreStub) ResolveOrgCode(context.Context, string, int) (string, error) {
	return "", s.err
}
func (s orgStoreStub) ResolveOrgCodes(context.Context, string, []int) (map[int]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return map[int]string{}, nil
}
func (s orgStoreStub) ListChildren(context.Context, string, int, string) ([]OrgUnitChild, error) {
	return nil, s.err
}
func (s orgStoreStub) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	return OrgUnitNodeDetails{}, s.err
}
func (s orgStoreStub) SearchNode(context.Context, string, string, string) (OrgUnitSearchResult, error) {
	return OrgUnitSearchResult{}, s.err
}
func (s orgStoreStub) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return nil, s.err
}
func (s orgStoreStub) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	return nil, s.err
}
func (s orgStoreStub) MaxEffectiveDateOnOrBefore(context.Context, string, string) (string, bool, error) {
	if s.err != nil {
		return "", false, s.err
	}
	return "", false, nil
}
func (s orgStoreStub) MinEffectiveDate(context.Context, string) (string, bool, error) {
	if s.err != nil {
		return "", false, s.err
	}
	return "", false, nil
}

func defaultJobCatalogOrgStore() OrgUnitStore {
	return orgStoreStub{nodes: []OrgUnitNode{{ID: "10000001", Name: "Root Org", IsBusinessUnit: true}}}
}

func defaultJobCatalogSetIDStore() jobCatalogSetIDStoreStub {
	return jobCatalogSetIDStoreStub{
		setids: []SetID{
			{SetID: "PKG1", Status: "active"},
			{SetID: "S2601", Status: "active"},
			{SetID: "DEFLT", Status: "active"},
		},
		owned: []OwnedScopePackage{
			{PackageCode: "PKG1", OwnerSetID: "PKG1", Name: "Package 1", Status: "active"},
			{PackageCode: "DEFLT", OwnerSetID: "DEFLT", Name: "Default", Status: "active"},
		},
	}
}

func withTenantAdminRequest(r *http.Request) *http.Request {
	r = r.WithContext(withTenant(r.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	r = r.WithContext(withPrincipal(r.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
	return r
}

type errJobCatalogStore struct {
	jobCatalogResolveStub
	err error
}

func (s errJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s errJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, error) {
	return nil, s.err
}
func (s errJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return s.err
}
func (s errJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return s.err
}
func (s errJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, error) {
	return nil, s.err
}
func (s errJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s errJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, error) {
	return nil, s.err
}
func (s errJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return s.err
}
func (s errJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, s.err
}

type partialJobCatalogStore struct {
	jobCatalogResolveStub
	listErr error
}

func (s partialJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s partialJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, error) {
	return nil, s.listErr
}
func (s partialJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s partialJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s partialJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, error) {
	return nil, nil
}
func (s partialJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s partialJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, error) {
	return nil, nil
}
func (s partialJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s partialJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, nil
}

type createErrJobCatalogStore struct {
	jobCatalogResolveStub
	err error
}

func (s createErrJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s createErrJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, error) {
	return nil, nil
}
func (s createErrJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s createErrJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s createErrJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, error) {
	return nil, nil
}
func (s createErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, error) {
	return nil, nil
}
func (s createErrJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s createErrJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, nil
}

func TestJobCatalogMemoryStore_Branches(t *testing.T) {
	ctx := context.Background()
	store := newJobCatalogMemoryStore().(*jobcatalogMemoryStore)
	tenantID := "t1"

	if err := store.CreateJobFamilyGroup(ctx, tenantID, "", "2026-01-01", "C1", "Group", ""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.ListJobFamilyGroups(ctx, tenantID, "", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	if err := store.CreateJobFamily(ctx, tenantID, "", "2026-01-01", "F1", "Family", "", "C1"); err == nil {
		t.Fatal("expected error")
	}
	if err := store.UpdateJobFamilyGroup(ctx, tenantID, "", "2026-01-01", "F1", "C2"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.ListJobFamilies(ctx, tenantID, "", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	if err := store.CreateJobLevel(ctx, tenantID, "", "2026-01-01", "L1", "Level", ""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.ListJobLevels(ctx, tenantID, "", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	if err := store.CreateJobProfile(ctx, tenantID, "", "2026-01-01", "P1", "Profile", "", []string{"F1"}, "F1"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.ListJobProfiles(ctx, tenantID, "", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	setID := "S2601"
	if err := store.CreateJobFamilyGroup(ctx, tenantID, setID, "2026-01-01", "C1", "Group", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if groups, err := store.ListJobFamilyGroups(ctx, tenantID, setID, "2026-01-01"); err != nil || len(groups) != 1 {
		t.Fatalf("groups=%d err=%v", len(groups), err)
	}
	if err := store.CreateJobFamily(ctx, tenantID, setID, "2026-01-01", "F1", "Family", "", "C1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if families, err := store.ListJobFamilies(ctx, tenantID, setID, "2026-01-01"); err != nil || len(families) != 1 {
		t.Fatalf("families=%d err=%v", len(families), err)
	}
	if err := store.UpdateJobFamilyGroup(ctx, tenantID, setID, "2026-01-02", "F1", "C2"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := store.UpdateJobFamilyGroup(ctx, tenantID, setID, "2026-01-02", "F2", "C2"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := store.CreateJobLevel(ctx, tenantID, setID, "2026-01-01", "L1", "Level", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if levels, err := store.ListJobLevels(ctx, tenantID, setID, "2026-01-01"); err != nil || len(levels) != 1 {
		t.Fatalf("levels=%d err=%v", len(levels), err)
	}
	if err := store.CreateJobProfile(ctx, tenantID, setID, "2026-01-01", "P1", "Profile", "", []string{"F1"}, "F1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if profiles, err := store.ListJobProfiles(ctx, tenantID, setID, "2026-01-01"); err != nil || len(profiles) != 1 {
		t.Fatalf("profiles=%d err=%v", len(profiles), err)
	}
}

func TestJobCatalogMemoryStore_ResolvePackage(t *testing.T) {
	ctx := context.Background()
	store := newJobCatalogMemoryStore()

	if _, err := store.ResolveJobCatalogPackageByCode(ctx, "t1", "", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	pkg, err := store.ResolveJobCatalogPackageByCode(ctx, "t1", "pkg1", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if pkg.PackageUUID == "" || pkg.PackageCode != "PKG1" || pkg.OwnerSetID != "PKG1" {
		t.Fatalf("unexpected pkg=%+v", pkg)
	}
	if _, err := store.ResolveJobCatalogPackageBySetID(ctx, "t1", "", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	resolved, err := store.ResolveJobCatalogPackageBySetID(ctx, "t1", "S2601", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if resolved != "S2601" {
		t.Fatalf("resolved=%s", resolved)
	}
}

func TestJobCatalogView_ListSetID(t *testing.T) {
	if got := (jobCatalogView{}).listSetID(); got != "" {
		t.Fatalf("got=%q", got)
	}
	view := jobCatalogView{HasSelection: true, ReadOnly: true, SetID: "S1"}
	if got := view.listSetID(); got != "S1" {
		t.Fatalf("got=%q", got)
	}
	view = jobCatalogView{HasSelection: true, OwnerSetID: "S2"}
	if got := view.listSetID(); got != "S2" {
		t.Fatalf("got=%q", got)
	}
}

func TestOwnerSetIDEditableAndLoadOwnedPackages(t *testing.T) {
	ctx := context.Background()
	if ownerSetIDEditable(ctx, nil, "t1", "S1") {
		t.Fatal("expected false")
	}
	viewerCtx := withPrincipal(context.Background(), Principal{RoleSlug: "tenant-viewer"})
	if ownerSetIDEditable(viewerCtx, defaultJobCatalogSetIDStore(), "t1", "S1") {
		t.Fatal("expected false")
	}
	adminCtx := withPrincipal(context.Background(), Principal{RoleSlug: "tenant-admin"})
	if ownerSetIDEditable(adminCtx, nil, "t1", "S1") {
		t.Fatal("expected false")
	}
	if ownerSetIDEditable(adminCtx, defaultJobCatalogSetIDStore(), "t1", "") {
		t.Fatal("expected false")
	}
	if ownerSetIDEditable(adminCtx, jobCatalogSetIDStoreStub{err: errors.New("boom")}, "t1", "S1") {
		t.Fatal("expected false")
	}
	if ownerSetIDEditable(adminCtx, jobCatalogSetIDStoreStub{setids: []SetID{{SetID: "S1", Status: "disabled"}}}, "t1", "S1") {
		t.Fatal("expected false")
	}
	if !ownerSetIDEditable(adminCtx, jobCatalogSetIDStoreStub{setids: []SetID{{SetID: "S1", Status: "active"}}}, "t1", "S1") {
		t.Fatal("expected true")
	}

	owned, err := loadOwnedJobCatalogPackages(ctx, nil, "t1", "2026-01-01")
	if err != nil || len(owned) != 0 {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
	owned, err = loadOwnedJobCatalogPackages(viewerCtx, defaultJobCatalogSetIDStore(), "t1", "2026-01-01")
	if err != nil || len(owned) != 0 {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
	if _, err := loadOwnedJobCatalogPackages(adminCtx, jobCatalogSetIDStoreStub{err: errors.New("boom")}, "t1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	owned, err = loadOwnedJobCatalogPackages(adminCtx, jobCatalogSetIDStoreStub{}, "t1", "2026-01-01")
	if err != nil || len(owned) != 0 {
		t.Fatalf("owned=%v err=%v", owned, err)
	}

	owned, err = loadOwnedJobCatalogPackages(adminCtx, jobCatalogSetIDStoreStub{
		owned: []OwnedScopePackage{{PackageID: "pkg-1", PackageCode: "PKG1", OwnerSetID: "S1"}},
	}, "t1", "2026-01-01")
	if err != nil || len(owned) != 1 {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
}

func TestCanEditDefltPackage(t *testing.T) {
	if canEditDefltPackage(context.Background()) {
		t.Fatal("expected false")
	}
	inactiveCtx := withPrincipal(context.Background(), Principal{RoleSlug: "tenant-admin", Status: "disabled"})
	if canEditDefltPackage(inactiveCtx) {
		t.Fatal("expected false")
	}
	activeCtx := withPrincipal(context.Background(), Principal{RoleSlug: "tenant-admin", Status: "active"})
	if !canEditDefltPackage(activeCtx) {
		t.Fatal("expected true")
	}
}

func TestJobCatalogPGStore_SetIDValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("missing setid", func(t *testing.T) {
		store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})).(*jobcatalogPGStore)
		if err := store.CreateJobFamilyGroup(ctx, "t1", "", "2026-01-01", "C1", "Group", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("setid not found", func(t *testing.T) {
		store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: pgx.ErrNoRows}, nil
		})).(*jobcatalogPGStore)
		if err := store.CreateJobFamilyGroup(ctx, "t1", "S2601", "2026-01-01", "C1", "Group", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("setid inactive", func(t *testing.T) {
		store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: &stubRow{vals: []any{"disabled"}}}, nil
		})).(*jobcatalogPGStore)
		if err := store.CreateJobFamilyGroup(ctx, "t1", "S2601", "2026-01-01", "C1", "Group", ""); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestResolveJobCatalogPackageByCode_PG(t *testing.T) {
	ctx := context.Background()
	if _, err := resolveJobCatalogPackageByCode(ctx, &stubTx{}, "t1", "", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := resolveJobCatalogPackageByCode(ctx, &stubTx{rowErr: errors.New("boom")}, "t1", "PKG1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := resolveJobCatalogPackageByCode(ctx, &stubTx{rowErr: pgx.ErrNoRows}, "t1", "PKG1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	txExecErr := &stubTx{row: &stubRow{vals: []any{"pkg-1", "S2601"}}, execErr: errors.New("boom")}
	if _, err := resolveJobCatalogPackageByCode(ctx, txExecErr, "t1", "PKG1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	txOK := &stubTx{row: &stubRow{vals: []any{"pkg-1", "S2601"}}}
	pkg, err := resolveJobCatalogPackageByCode(ctx, txOK, "t1", "PKG1", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if pkg.PackageUUID != "pkg-1" || pkg.OwnerSetID != "S2601" || pkg.PackageCode != "PKG1" {
		t.Fatalf("unexpected pkg=%+v", pkg)
	}
}

func TestJobCatalogPGStore_ResolvePackages(t *testing.T) {
	ctx := context.Background()
	storeErr := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin fail")
	})).(*jobcatalogPGStore)
	if _, err := storeErr.ResolveJobCatalogPackageByCode(ctx, "t1", "PKG1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := storeErr.ResolveJobCatalogPackageBySetID(ctx, "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	storeResolveErr := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{rowErr: errors.New("boom")}, nil
	})).(*jobcatalogPGStore)
	if _, err := storeResolveErr.ResolveJobCatalogPackageByCode(ctx, "t1", "PKG1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := storeResolveErr.ResolveJobCatalogPackageBySetID(ctx, "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{row: &stubRow{vals: []any{"pkg-1", "S2601"}}}, nil
	})).(*jobcatalogPGStore)
	pkg, err := store.ResolveJobCatalogPackageByCode(ctx, "t1", "PKG1", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if pkg.PackageUUID != "pkg-1" || pkg.OwnerSetID != "S2601" || pkg.PackageCode != "PKG1" {
		t.Fatalf("unexpected pkg=%+v", pkg)
	}

	store2 := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-2", "t1"}}}, nil
	})).(*jobcatalogPGStore)
	resolved, err := store2.ResolveJobCatalogPackageBySetID(ctx, "t1", "S2601", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if resolved != "pkg-2" {
		t.Fatalf("resolved=%s", resolved)
	}
}

func TestResolveJobCatalogView_Branches(t *testing.T) {
	adminCtx := withPrincipal(context.Background(), Principal{RoleSlug: "tenant-admin", Status: "active"})
	inactiveAdminCtx := withPrincipal(context.Background(), Principal{RoleSlug: "tenant-admin", Status: "inactive"})
	setidStore := jobCatalogSetIDStoreStub{setids: []SetID{{SetID: "S1", Status: "active"}}}
	store := resolveJobCatalogStoreStub{
		pkg:              JobCatalogPackage{PackageUUID: "pkg-1", PackageCode: "PKG1", OwnerSetID: "S1"},
		setidPackageUUID: "pkg-1",
	}

	view, errMsg := resolveJobCatalogView(context.Background(), store, setidStore, "t1", "2026-01-01", "", "")
	if view.HasSelection || errMsg != "" {
		t.Fatalf("unexpected view=%+v err=%s", view, errMsg)
	}

	view, errMsg = resolveJobCatalogView(context.Background(), store, setidStore, "t1", "2026-01-01", "", "S1")
	if !view.ReadOnly || view.SetID != "S1" || errMsg != "" {
		t.Fatalf("unexpected view=%+v err=%s", view, errMsg)
	}

	_, errMsg = resolveJobCatalogView(context.Background(), store, setidStore, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "OWNER_SETID_FORBIDDEN" {
		t.Fatalf("err=%s", errMsg)
	}

	_, errMsg = resolveJobCatalogView(adminCtx, store, jobCatalogSetIDStoreStub{}, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "OWNER_SETID_FORBIDDEN" {
		t.Fatalf("err=%s", errMsg)
	}

	_, errMsg = resolveJobCatalogView(adminCtx, resolveJobCatalogStoreStub{pkgErr: errors.New("PACKAGE_NOT_FOUND")}, setidStore, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "PACKAGE_NOT_FOUND" {
		t.Fatalf("err=%s", errMsg)
	}

	_, errMsg = resolveJobCatalogView(adminCtx, resolveJobCatalogStoreStub{
		pkg:              JobCatalogPackage{PackageUUID: "pkg-1", PackageCode: "PKG1", OwnerSetID: "S1"},
		setidPackageUUID: "pkg-2",
	}, setidStore, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "PACKAGE_CODE_MISMATCH" {
		t.Fatalf("err=%s", errMsg)
	}

	_, errMsg = resolveJobCatalogView(adminCtx, resolveJobCatalogStoreStub{
		pkg:      JobCatalogPackage{PackageUUID: "pkg-1", PackageCode: "PKG1", OwnerSetID: "S1"},
		setidErr: errors.New("resolve failed"),
	}, setidStore, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "resolve failed" {
		t.Fatalf("err=%s", errMsg)
	}

	_, errMsg = resolveJobCatalogView(inactiveAdminCtx, resolveJobCatalogStoreStub{
		pkg:              JobCatalogPackage{PackageUUID: "deflt-id", PackageCode: "DEFLT", OwnerSetID: "DEFLT"},
		setidPackageUUID: "deflt-id",
	}, jobCatalogSetIDStoreStub{setids: []SetID{{SetID: "DEFLT", Status: "active"}}}, "t1", "2026-01-01", "DEFLT", "")
	if errMsg != "DEFLT_EDIT_FORBIDDEN" {
		t.Fatalf("err=%s", errMsg)
	}

	view, errMsg = resolveJobCatalogView(adminCtx, store, setidStore, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "" || view.OwnerSetID != "S1" || !view.HasSelection {
		t.Fatalf("unexpected view=%+v err=%s", view, errMsg)
	}
}

func TestJobCatalogStatusForError(t *testing.T) {
	if jobCatalogStatusForError("OWNER_SETID_FORBIDDEN") != http.StatusForbidden {
		t.Fatal("expected forbidden")
	}
	if jobCatalogStatusForError("DEFLT_EDIT_FORBIDDEN") != http.StatusForbidden {
		t.Fatal("expected forbidden")
	}
	if jobCatalogStatusForError("PACKAGE_CODE_MISMATCH") != http.StatusUnprocessableEntity {
		t.Fatal("expected unprocessable")
	}
	if jobCatalogStatusForError("other") != http.StatusOK {
		t.Fatal("expected ok")
	}
}

type createLevelErrJobCatalogStore struct {
	jobCatalogResolveStub
	err error
}

func (s createLevelErrJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createLevelErrJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, error) {
	return nil, nil
}
func (s createLevelErrJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s createLevelErrJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s createLevelErrJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, error) {
	return nil, nil
}
func (s createLevelErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s createLevelErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, error) {
	return nil, nil
}
func (s createLevelErrJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s createLevelErrJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, nil
}

type levelsListErrJobCatalogStore struct {
	jobCatalogResolveStub
	err error
}

func (s levelsListErrJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s levelsListErrJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, error) {
	return nil, nil
}
func (s levelsListErrJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s levelsListErrJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s levelsListErrJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, error) {
	return nil, nil
}
func (s levelsListErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s levelsListErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, error) {
	return nil, s.err
}
func (s levelsListErrJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s levelsListErrJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, nil
}

type createJobFamilyErrStore struct {
	jobCatalogResolveStub
	err error
}

func (s createJobFamilyErrStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createJobFamilyErrStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, error) {
	return nil, nil
}
func (s createJobFamilyErrStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return s.err
}
func (s createJobFamilyErrStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s createJobFamilyErrStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, error) {
	return nil, nil
}
func (s createJobFamilyErrStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createJobFamilyErrStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, error) {
	return nil, nil
}
func (s createJobFamilyErrStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s createJobFamilyErrStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, nil
}

type updateJobFamilyGroupErrStore struct {
	jobCatalogResolveStub
	err error
}

func (s updateJobFamilyGroupErrStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s updateJobFamilyGroupErrStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, error) {
	return nil, nil
}
func (s updateJobFamilyGroupErrStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s updateJobFamilyGroupErrStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return s.err
}
func (s updateJobFamilyGroupErrStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, error) {
	return nil, nil
}
func (s updateJobFamilyGroupErrStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s updateJobFamilyGroupErrStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, error) {
	return nil, nil
}
func (s updateJobFamilyGroupErrStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s updateJobFamilyGroupErrStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, nil
}

type createJobProfileErrStore struct {
	jobCatalogResolveStub
	err error
}

func (s createJobProfileErrStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createJobProfileErrStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, error) {
	return nil, nil
}
func (s createJobProfileErrStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s createJobProfileErrStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s createJobProfileErrStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, error) {
	return nil, nil
}
func (s createJobProfileErrStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s createJobProfileErrStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, error) {
	return nil, nil
}
func (s createJobProfileErrStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return s.err
}
func (s createJobProfileErrStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, nil
}

type listFamiliesErrJobCatalogStore struct {
	jobCatalogResolveStub
	err error
}

func (s listFamiliesErrJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s listFamiliesErrJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, error) {
	return nil, nil
}
func (s listFamiliesErrJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s listFamiliesErrJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s listFamiliesErrJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, error) {
	return nil, s.err
}
func (s listFamiliesErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s listFamiliesErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, error) {
	return nil, nil
}
func (s listFamiliesErrJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s listFamiliesErrJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, nil
}

type listProfilesErrJobCatalogStore struct {
	jobCatalogResolveStub
	err error
}

func (s listProfilesErrJobCatalogStore) CreateJobFamilyGroup(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s listProfilesErrJobCatalogStore) ListJobFamilyGroups(context.Context, string, string, string) ([]JobFamilyGroup, error) {
	return nil, nil
}
func (s listProfilesErrJobCatalogStore) CreateJobFamily(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (s listProfilesErrJobCatalogStore) UpdateJobFamilyGroup(context.Context, string, string, string, string, string) error {
	return nil
}
func (s listProfilesErrJobCatalogStore) ListJobFamilies(context.Context, string, string, string) ([]JobFamily, error) {
	return nil, nil
}
func (s listProfilesErrJobCatalogStore) CreateJobLevel(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s listProfilesErrJobCatalogStore) ListJobLevels(context.Context, string, string, string) ([]JobLevel, error) {
	return nil, nil
}
func (s listProfilesErrJobCatalogStore) CreateJobProfile(context.Context, string, string, string, string, string, string, []string, string) error {
	return nil
}
func (s listProfilesErrJobCatalogStore) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, s.err
}

func TestJobCatalogPGStore_CreateAndListJobLevels(t *testing.T) {
	makeTx := func() *stubTx {
		return &stubTx{
			row:  &stubRow{vals: []any{"active"}},
			row2: &stubRow{vals: []any{"pkg-1", "t1"}},
			row3: &stubRow{vals: []any{"level-id"}},
			row4: &stubRow{vals: []any{"event-id"}},
		}
	}

	store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return makeTx(), nil
	})).(*jobcatalogPGStore)

	if err := store.CreateJobLevel(context.Background(), "t1", "S2601", "2026-01-01", "JL1", "Level 1", "desc"); err != nil {
		t.Fatalf("err=%v", err)
	}

	store2 := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return makeTx(), nil
	})).(*jobcatalogPGStore)
	if err := store2.CreateJobLevel(context.Background(), "t1", "S2601", "2026-01-01", "JL1", "Level 1", ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	rows := &jobcatalogRows{rows: [][]any{{"id1", "JL1", "Level 1", true, "2026-01-01"}}}
	store3 := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{
			row:  &stubRow{vals: []any{"active"}},
			row2: &stubRow{vals: []any{"pkg-1", "t1"}},
			rows: rows,
		}, nil
	})).(*jobcatalogPGStore)
	levels, err := store3.ListJobLevels(context.Background(), "t1", "S2601", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(levels) != 1 {
		t.Fatalf("levels=%d", len(levels))
	}
}

func TestJobCatalogPGStore_CreateAndListJobFamilies(t *testing.T) {
	makeTx := func() *stubTx {
		return &stubTx{
			row:  &stubRow{vals: []any{"active"}},
			row2: &stubRow{vals: []any{"pkg-1", "t1"}},
			row3: &stubRow{vals: []any{"group-id"}},
			row4: &stubRow{vals: []any{"family-id"}},
			row5: &stubRow{vals: []any{"event-id"}},
		}
	}

	store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return makeTx(), nil
	})).(*jobcatalogPGStore)

	if err := store.CreateJobFamily(context.Background(), "t1", "S2601", "2026-01-01", "JF-BE", "Backend", "desc", "JFG-ENG"); err != nil {
		t.Fatalf("err=%v", err)
	}
	storeNullDesc := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return makeTx(), nil
	})).(*jobcatalogPGStore)
	if err := storeNullDesc.CreateJobFamily(context.Background(), "t1", "S2601", "2026-01-01", "JF-FE", "Frontend", "", "JFG-ENG"); err != nil {
		t.Fatalf("err=%v", err)
	}

	rows := &jobcatalogRows{rows: [][]any{{"id1", "JF-BE", "JFG-ENG", "Backend", true, "2026-01-01"}}}
	store2 := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{
			row:  &stubRow{vals: []any{"active"}},
			row2: &stubRow{vals: []any{"pkg-1", "t1"}},
			rows: rows,
		}, nil
	})).(*jobcatalogPGStore)
	families, err := store2.ListJobFamilies(context.Background(), "t1", "S2601", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(families) != 1 {
		t.Fatalf("families=%d", len(families))
	}
}

func TestJobCatalogPGStore_CreateJobFamily_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "group_lookup_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, row3Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "family_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, row3: &stubRow{vals: []any{"group-id"}}, row4Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "event_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, row3: &stubRow{vals: []any{"group-id"}}, row4: &stubRow{vals: []any{"family-id"}}, row5Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "submit_exec_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, row3: &stubRow{vals: []any{"group-id"}}, row4: &stubRow{vals: []any{"family-id"}}, row5: &stubRow{vals: []any{"event-id"}}, execErr: errors.New("boom"), execErrAt: 3},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			err := store.CreateJobFamily(context.Background(), "t1", "S2601", "2026-01-01", "JF-BE", "Backend", "desc", "JFG-ENG")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_ListJobFamilies_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "query_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, queryErr: errors.New("boom"), queryErrAt: 1},
			wantError: true,
		},
		{
			name:      "scan_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: [][]any{{"id1", "JF-BE", "JFG-ENG", "Backend", true, "2026-01-01"}}, scanErr: errors.New("boom")}},
			wantError: true,
		},
		{
			name:      "rows_err",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: nil, err: errors.New("boom")}},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			_, err := store.ListJobFamilies(context.Background(), "t1", "S2601", "2026-01-01")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_UpdateJobFamilyGroup_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "group_lookup_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, row3Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "family_lookup_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, row3: &stubRow{vals: []any{"group-id"}}, row4Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "event_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, row3: &stubRow{vals: []any{"group-id"}}, row4: &stubRow{vals: []any{"family-id"}}, row5Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "submit_exec_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, row3: &stubRow{vals: []any{"group-id"}}, row4: &stubRow{vals: []any{"family-id"}}, row5: &stubRow{vals: []any{"event-id"}}, execErr: errors.New("boom"), execErrAt: 3},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			err := store.UpdateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-02-01", "JF-BE", "JFG-SALES")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_CreateAndListJobProfiles(t *testing.T) {
	rows := &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}, {"JF-FE", "family2"}}}
	store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{
			row:  &stubRow{vals: []any{"active"}},
			row2: &stubRow{vals: []any{"pkg-1", "t1"}},
			rows: rows,
			row3: &stubRow{vals: []any{"profile-id"}},
			row4: &stubRow{vals: []any{"event-id"}},
		}, nil
	})).(*jobcatalogPGStore)
	if err := store.CreateJobProfile(context.Background(), "t1", "S2601", "2026-01-01", "JP-SWE", "Software Engineer", "desc", []string{"JF-BE", "JF-FE"}, "JF-BE"); err != nil {
		t.Fatalf("err=%v", err)
	}
	rows2 := &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}, {"JF-FE", "family2"}}}
	storePrimaryNotInFamilies := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{
			row:  &stubRow{vals: []any{"active"}},
			row2: &stubRow{vals: []any{"pkg-1", "t1"}},
			rows: rows2,
			row3: &stubRow{vals: []any{"profile-id"}},
			row4: &stubRow{vals: []any{"event-id"}},
		}, nil
	})).(*jobcatalogPGStore)
	if err := storePrimaryNotInFamilies.CreateJobProfile(context.Background(), "t1", "S2601", "2026-01-01", "JP-BAD", "Bad", "", []string{"JF-BE"}, "JF-FE"); err != nil {
		t.Fatalf("err=%v", err)
	}

	rowsProfiles := &jobcatalogRows{rows: [][]any{{"id1", "JP-SWE", "Software Engineer", true, "2026-01-01", "JF-BE,JF-FE", "JF-BE"}}}
	store2 := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{
			row:  &stubRow{vals: []any{"active"}},
			row2: &stubRow{vals: []any{"pkg-1", "t1"}},
			rows: rowsProfiles,
		}, nil
	})).(*jobcatalogPGStore)
	profiles, err := store2.ListJobProfiles(context.Background(), "t1", "S2601", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles=%d", len(profiles))
	}
}

func TestJobCatalogPGStore_CreateJobProfile_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		families  []string
		primary   string
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "family_query_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, queryErr: errors.New("boom"), queryErrAt: 1},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "family_scan_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}, scanErr: errors.New("boom")}},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "family_rows_err",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: nil, err: errors.New("boom")}},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "family_missing",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}}},
			families:  []string{"JF-BE", "JF-FE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "primary_missing",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}}},
			families:  []string{"JF-BE"},
			primary:   "JF-FE",
			wantError: true,
		},
		{
			name:      "profile_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}}, row3Err: errors.New("boom")},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "event_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}}, row3: &stubRow{vals: []any{"profile-id"}}, row4Err: errors.New("boom")},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
		{
			name:      "submit_exec_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: [][]any{{"JF-BE", "family1"}}}, row3: &stubRow{vals: []any{"profile-id"}}, row4: &stubRow{vals: []any{"event-id"}}, execErr: errors.New("boom"), execErrAt: 3},
			families:  []string{"JF-BE"},
			primary:   "JF-BE",
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			err := store.CreateJobProfile(context.Background(), "t1", "S2601", "2026-01-01", "JP-SWE", "Software Engineer", "desc", tc.families, tc.primary)
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_ListJobProfiles_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "query_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, queryErr: errors.New("boom"), queryErrAt: 1},
			wantError: true,
		},
		{
			name:      "scan_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: [][]any{{"id1", "JP-SWE", "Software Engineer", true, "2026-01-01", "JF-BE", "JF-BE"}}, scanErr: errors.New("boom")}},
			wantError: true,
		},
		{
			name:      "rows_err",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: nil, err: errors.New("boom")}},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			_, err := store.ListJobProfiles(context.Background(), "t1", "S2601", "2026-01-01")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_CreateJobLevel_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "level_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, row3Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "event_uuid_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, row3: &stubRow{vals: []any{"level-id"}}, row4Err: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "submit_exec_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, row3: &stubRow{vals: []any{"level-id"}}, row4: &stubRow{vals: []any{"event-id"}}, execErr: errors.New("boom"), execErrAt: 3},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			err := store.CreateJobLevel(context.Background(), "t1", "S2601", "2026-01-01", "JL1", "Level 1", "desc")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogPGStore_ListJobLevels_Errors(t *testing.T) {
	cases := []struct {
		name      string
		tx        *stubTx
		wantError bool
	}{
		{
			name:      "ensure_bootstrap_exec_error",
			tx:        &stubTx{execErr: errors.New("boom"), execErrAt: 2},
			wantError: true,
		},
		{
			name:      "resolve_setid_error",
			tx:        &stubTx{rowErr: errors.New("boom")},
			wantError: true,
		},
		{
			name:      "query_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, queryErr: errors.New("boom"), queryErrAt: 1},
			wantError: true,
		},
		{
			name:      "scan_error",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: [][]any{{"id1", "JL1", "Level 1", true, "2026-01-01"}}, scanErr: errors.New("boom")}},
			wantError: true,
		},
		{
			name:      "rows_err",
			tx:        &stubTx{row: &stubRow{vals: []any{"active"}}, row2: &stubRow{vals: []any{"pkg-1", "t1"}}, rows: &jobcatalogRows{rows: nil, err: errors.New("boom")}},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newJobCatalogPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return tc.tx, nil
			})).(*jobcatalogPGStore)
			_, err := store.ListJobLevels(context.Background(), "t1", "S2601", "2026-01-01")
			if tc.wantError && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestJobCatalogMemoryStore_CreateAndListJobLevels_DefaultBU(t *testing.T) {
	store := newJobCatalogMemoryStore().(*jobcatalogMemoryStore)
	if err := store.CreateJobLevel(context.Background(), "t1", "S2601", "2026-01-01", "JL1", "Level 1", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := store.CreateJobLevel(context.Background(), "t1", "S2601", "2026-01-01", "JL2", "Level 2", ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	levels, err := store.ListJobLevels(context.Background(), "t1", "S2601", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(levels) != 2 {
		t.Fatalf("levels=%d", len(levels))
	}
}

func TestJobCatalogMemoryStore_CreateAndListJobFamilies_DefaultBU(t *testing.T) {
	store := newJobCatalogMemoryStore().(*jobcatalogMemoryStore)
	if err := store.CreateJobFamily(context.Background(), "t1", "S2601", "2026-01-01", "JF-BE", "Backend", "", "JFG-ENG"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := store.UpdateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-02-01", "JF-BE", "JFG-SALES"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := store.UpdateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-02-01", "NOPE", "JFG-SALES"); err != nil {
		t.Fatalf("err=%v", err)
	}

	families, err := store.ListJobFamilies(context.Background(), "t1", "S2601", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(families) != 1 {
		t.Fatalf("families=%d", len(families))
	}
}

func TestJobCatalogMemoryStore_CreateAndListJobProfiles_DefaultBU(t *testing.T) {
	store := newJobCatalogMemoryStore().(*jobcatalogMemoryStore)
	if err := store.CreateJobProfile(context.Background(), "t1", "S2601", "2026-01-01", "JP-SWE", "Software Engineer", "", []string{"JF-BE", "JF-FE"}, "JF-BE"); err != nil {
		t.Fatalf("err=%v", err)
	}
	profiles, err := store.ListJobProfiles(context.Background(), "t1", "S2601", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles=%d", len(profiles))
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" a, ,b,  , c ")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("got=%v", got)
	}
}

func TestQuoteAll(t *testing.T) {
	got := quoteAll([]string{"x", "y"})
	if len(got) != 2 || got[0] != `"x"` || got[1] != `"y"` {
		t.Fatalf("got=%v", got)
	}
}

func TestJobCatalogPGStore_WithTxAndMethods(t *testing.T) {
	beginErrStore := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin fail")
	})}
	if _, err := beginErrStore.ListJobFamilyGroups(context.Background(), "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx3 := &stubTx{
		row:  fakeRow{vals: []any{"active"}},
		row2: fakeRow{vals: []any{"pkg-1", "t1"}},
		row3: fakeRow{vals: []any{"g1"}},
		row4: fakeRow{vals: []any{"e1"}},
	}
	s3 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3, nil })}
	if err := s3.CreateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-01-01", "JC1", "Group1", ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	tx3OwnerMismatch := &stubTx{
		row:  fakeRow{vals: []any{"active"}},
		row2: fakeRow{vals: []any{"pkg-1", "t2"}},
	}
	s3OwnerMismatch := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3OwnerMismatch, nil })}
	if err := s3OwnerMismatch.CreateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-01-01", "JC1", "Group1", ""); err == nil || err.Error() != "JOBCATALOG_PACKAGE_OWNER_INVALID" {
		t.Fatalf("expected JOBCATALOG_PACKAGE_OWNER_INVALID, got %v", err)
	}

	tx3a := &stubTx{execErr: errors.New("bootstrap fail"), execErrAt: 2}
	s3a := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3a, nil })}
	if err := s3a.CreateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-01-01", "JC1", "Group1", "desc"); err == nil {
		t.Fatal("expected error")
	}

	tx3ResolveErr := &stubTx{row: fakeRow{vals: []any{"active"}}, row2Err: errors.New("resolve fail")}
	s3ResolveErr := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3ResolveErr, nil })}
	if err := s3ResolveErr.CreateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-01-01", "JC1", "Group1", "desc"); err == nil {
		t.Fatal("expected error")
	}

	tx3b := &stubTx{
		row:     fakeRow{vals: []any{"active"}},
		row2:    fakeRow{vals: []any{"pkg-1", "t1"}},
		row3Err: errors.New("uuid fail"),
	}
	s3b := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3b, nil })}
	if err := s3b.CreateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-01-01", "JC1", "Group1", "desc"); err == nil {
		t.Fatal("expected error")
	}

	tx3EventIDErr := &stubTx{
		row:     fakeRow{vals: []any{"active"}},
		row2:    fakeRow{vals: []any{"pkg-1", "t1"}},
		row3:    fakeRow{vals: []any{"g1"}},
		row4Err: errors.New("event id fail"),
	}
	s3EventIDErr := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3EventIDErr, nil })}
	if err := s3EventIDErr.CreateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-01-01", "JC1", "Group1", "desc"); err == nil {
		t.Fatal("expected error")
	}

	tx3c := &stubTx{
		execErr:   errors.New("exec fail"),
		execErrAt: 3,
		row:       fakeRow{vals: []any{"active"}},
		row2:      fakeRow{vals: []any{"pkg-1", "t1"}},
		row3:      fakeRow{vals: []any{"g1"}},
		row4:      fakeRow{vals: []any{"e1"}},
	}
	s3c := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3c, nil })}
	if err := s3c.CreateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-01-01", "JC1", "Group1", "desc"); err == nil {
		t.Fatal("expected error")
	}

	tx4 := &stubTx{
		row:  fakeRow{vals: []any{"active"}},
		row2: fakeRow{vals: []any{"pkg-1", "t1"}},
		rows: &jobcatalogRows{rows: [][]any{
			{"g1", "JC1", "Group1", true, "2026-01-01"},
		}},
	}
	s4 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4, nil })}
	groups, err := s4.ListJobFamilyGroups(context.Background(), "t1", "S2601", "2026-01-01")
	if err != nil || len(groups) != 1 {
		t.Fatalf("len=%d err=%v", len(groups), err)
	}

	tx4a := &stubTx{execErr: errors.New("bootstrap fail"), execErrAt: 2}
	s4a := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4a, nil })}
	if _, err := s4a.ListJobFamilyGroups(context.Background(), "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx4ResolveErr := &stubTx{row: fakeRow{vals: []any{"active"}}, row2Err: errors.New("resolve fail")}
	s4ResolveErr := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4ResolveErr, nil })}
	if _, err := s4ResolveErr.ListJobFamilyGroups(context.Background(), "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx4b := &stubTx{
		row:      fakeRow{vals: []any{"active"}},
		row2:     fakeRow{vals: []any{"pkg-1", "t1"}},
		queryErr: errors.New("query fail"),
	}
	s4b := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4b, nil })}
	if _, err := s4b.ListJobFamilyGroups(context.Background(), "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx4c := &stubTx{
		row:  fakeRow{vals: []any{"active"}},
		row2: fakeRow{vals: []any{"pkg-1", "t1"}},
		rows: &scanErrRows{},
	}
	s4c := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4c, nil })}
	if _, err := s4c.ListJobFamilyGroups(context.Background(), "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
}

func TestJobCatalogPGStore_Errors(t *testing.T) {
	tx := &stubTx{row: fakeRow{vals: []any{"active"}}, row2: fakeRow{vals: []any{"pkg-1", "t1"}}, queryErr: errors.New("query fail")}
	s := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	if _, err := s.ListJobFamilyGroups(context.Background(), "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx2 := &stubTx{rowErr: errors.New("row fail")}
	s2 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx2, nil })}
	if _, err := s2.ListJobFamilyGroups(context.Background(), "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx3 := &stubTx{
		row:     fakeRow{vals: []any{"active"}},
		row2:    fakeRow{vals: []any{"pkg-1", "t1"}},
		row3Err: errors.New("uuid fail"),
	}
	s3 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3, nil })}
	if err := s3.CreateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-01-01", "JC1", "Group1", ""); err == nil {
		t.Fatal("expected error")
	}

	tx4 := &stubTx{
		row:  fakeRow{vals: []any{"active"}},
		row2: fakeRow{vals: []any{"pkg-1", "t1"}},
		rows: &jobcatalogRows{err: errors.New("rows err")},
	}
	s4 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4, nil })}
	if _, err := s4.ListJobFamilyGroups(context.Background(), "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx5 := &stubTx{execErr: errors.New("set_config fail"), execErrAt: 1}
	s5 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx5, nil })}
	if _, err := s5.ListJobFamilyGroups(context.Background(), "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	tx6 := &stubTx{execErr: errors.New("bootstrap fail"), execErrAt: 2}
	s6 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx6, nil })}
	if err := s6.CreateJobFamilyGroup(context.Background(), "t1", "S2601", "2026-01-01", "JC1", "Group1", ""); err == nil {
		t.Fatal("expected error")
	}

	tx7 := &stubTx{
		commitErr: errors.New("commit fail"),
		row:       fakeRow{vals: []any{"active"}},
		row2:      fakeRow{vals: []any{"pkg-1", "t1"}},
		rows:      &jobcatalogRows{rows: [][]any{{"g1", "JC1", "Group1", true, "2026-01-01"}}},
	}
	s7 := &jobcatalogPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx7, nil })}
	if _, err := s7.ListJobFamilyGroups(context.Background(), "t1", "S2601", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
}
