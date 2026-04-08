package services

import (
	"context"
	"errors"
	"testing"
)

type setIDStoreStub struct {
	setids []SetIDRecord
	owned  []OwnedScopePackage
	err    error
}

func (s setIDStoreStub) ListSetIDs(_ context.Context, _ string) ([]SetIDRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]SetIDRecord(nil), s.setids...), nil
}

func (s setIDStoreStub) ListOwnedScopePackages(_ context.Context, _ string, _ string, _ string) ([]OwnedScopePackage, error) {
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

func TestJobCatalogView_ListSetID(t *testing.T) {
	if got := (JobCatalogView{}).ListSetID(); got != "" {
		t.Fatalf("got=%q", got)
	}
	view := JobCatalogView{HasSelection: true, ReadOnly: true, SetID: "S1"}
	if got := view.ListSetID(); got != "S1" {
		t.Fatalf("got=%q", got)
	}
	view = JobCatalogView{HasSelection: true, OwnerSetID: "S2"}
	if got := view.ListSetID(); got != "S2" {
		t.Fatalf("got=%q", got)
	}
}

func TestOwnerSetIDEditableAndLoadOwnedPackages(t *testing.T) {
	ctx := context.Background()
	viewer := Principal{RoleSlug: "tenant-viewer"}
	admin := Principal{RoleSlug: "tenant-admin"}

	if OwnerSetIDEditable(ctx, Principal{}, nil, "t1", "S1") {
		t.Fatal("expected false")
	}
	if OwnerSetIDEditable(ctx, viewer, setIDStoreStub{}, "t1", "S1") {
		t.Fatal("expected false")
	}
	if OwnerSetIDEditable(ctx, admin, nil, "t1", "S1") {
		t.Fatal("expected false")
	}
	if OwnerSetIDEditable(ctx, admin, setIDStoreStub{}, "t1", "") {
		t.Fatal("expected false")
	}
	if OwnerSetIDEditable(ctx, admin, setIDStoreStub{err: errors.New("boom")}, "t1", "S1") {
		t.Fatal("expected false")
	}
	if OwnerSetIDEditable(ctx, admin, setIDStoreStub{setids: []SetIDRecord{{SetID: "S1", Status: "disabled"}}}, "t1", "S1") {
		t.Fatal("expected false")
	}
	if !OwnerSetIDEditable(ctx, admin, setIDStoreStub{setids: []SetIDRecord{{SetID: "S1", Status: "active"}}}, "t1", "S1") {
		t.Fatal("expected true")
	}

	owned, err := LoadOwnedJobCatalogPackages(ctx, Principal{}, nil, "t1", "2026-01-01")
	if err != nil || len(owned) != 0 {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
	owned, err = LoadOwnedJobCatalogPackages(ctx, viewer, setIDStoreStub{}, "t1", "2026-01-01")
	if err != nil || len(owned) != 0 {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
	if _, err := LoadOwnedJobCatalogPackages(ctx, admin, setIDStoreStub{err: errors.New("boom")}, "t1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	owned, err = LoadOwnedJobCatalogPackages(ctx, admin, setIDStoreStub{}, "t1", "2026-01-01")
	if err != nil || len(owned) != 0 {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
	owned, err = LoadOwnedJobCatalogPackages(ctx, admin, setIDStoreStub{
		owned: []OwnedScopePackage{{PackageID: "pkg-1", PackageCode: "PKG1", OwnerSetID: "S1"}},
	}, "t1", "2026-01-01")
	if err != nil || len(owned) != 1 {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
}

func TestCanEditDefltPackage(t *testing.T) {
	if CanEditDefltPackage(Principal{}) {
		t.Fatal("expected false")
	}
	if CanEditDefltPackage(Principal{RoleSlug: "tenant-admin", Status: "disabled"}) {
		t.Fatal("expected false")
	}
	if !CanEditDefltPackage(Principal{RoleSlug: "tenant-admin", Status: "active"}) {
		t.Fatal("expected true")
	}
}

func TestResolveJobCatalogView_Branches(t *testing.T) {
	admin := Principal{RoleSlug: "tenant-admin", Status: "active"}
	inactiveAdmin := Principal{RoleSlug: "tenant-admin", Status: "inactive"}
	setidStore := setIDStoreStub{setids: []SetIDRecord{{SetID: "S1", Status: "active"}}}
	store := resolveJobCatalogStoreStub{
		pkg:              JobCatalogPackage{PackageUUID: "pkg-1", PackageCode: "PKG1", OwnerSetID: "S1"},
		setidPackageUUID: "pkg-1",
	}

	view, errMsg := ResolveJobCatalogView(context.Background(), Principal{}, store, setidStore, "t1", "2026-01-01", "", "")
	if view.HasSelection || errMsg != "" {
		t.Fatalf("unexpected view=%+v err=%s", view, errMsg)
	}

	view, errMsg = ResolveJobCatalogView(context.Background(), Principal{}, store, setidStore, "t1", "2026-01-01", "", "S1")
	if !view.ReadOnly || view.SetID != "S1" || errMsg != "" {
		t.Fatalf("unexpected view=%+v err=%s", view, errMsg)
	}

	_, errMsg = ResolveJobCatalogView(context.Background(), Principal{}, store, setidStore, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "OWNER_SETID_FORBIDDEN" {
		t.Fatalf("err=%s", errMsg)
	}

	_, errMsg = ResolveJobCatalogView(context.Background(), admin, store, setIDStoreStub{}, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "OWNER_SETID_FORBIDDEN" {
		t.Fatalf("err=%s", errMsg)
	}

	_, errMsg = ResolveJobCatalogView(context.Background(), admin, resolveJobCatalogStoreStub{pkgErr: errors.New("PACKAGE_NOT_FOUND")}, setidStore, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "PACKAGE_NOT_FOUND" {
		t.Fatalf("err=%s", errMsg)
	}

	_, errMsg = ResolveJobCatalogView(context.Background(), admin, resolveJobCatalogStoreStub{
		pkg:              JobCatalogPackage{PackageUUID: "pkg-1", PackageCode: "PKG1", OwnerSetID: "S1"},
		setidPackageUUID: "pkg-2",
	}, setidStore, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "PACKAGE_CODE_MISMATCH" {
		t.Fatalf("err=%s", errMsg)
	}

	_, errMsg = ResolveJobCatalogView(context.Background(), admin, resolveJobCatalogStoreStub{
		pkg:      JobCatalogPackage{PackageUUID: "pkg-1", PackageCode: "PKG1", OwnerSetID: "S1"},
		setidErr: errors.New("resolve failed"),
	}, setidStore, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "resolve failed" {
		t.Fatalf("err=%s", errMsg)
	}

	_, errMsg = ResolveJobCatalogView(context.Background(), inactiveAdmin, resolveJobCatalogStoreStub{
		pkg:              JobCatalogPackage{PackageUUID: "deflt-id", PackageCode: "DEFLT", OwnerSetID: "DEFLT"},
		setidPackageUUID: "deflt-id",
	}, setIDStoreStub{setids: []SetIDRecord{{SetID: "DEFLT", Status: "active"}}}, "t1", "2026-01-01", "DEFLT", "")
	if errMsg != "DEFLT_EDIT_FORBIDDEN" {
		t.Fatalf("err=%s", errMsg)
	}

	view, errMsg = ResolveJobCatalogView(context.Background(), admin, store, setidStore, "t1", "2026-01-01", "PKG1", "")
	if errMsg != "" || view.OwnerSetID != "S1" || !view.HasSelection {
		t.Fatalf("unexpected view=%+v err=%s", view, errMsg)
	}
}

func TestNormalizePackageCode(t *testing.T) {
	if got := NormalizePackageCode(" pkg1 "); got != "PKG1" {
		t.Fatalf("NormalizePackageCode=%q", got)
	}
}
