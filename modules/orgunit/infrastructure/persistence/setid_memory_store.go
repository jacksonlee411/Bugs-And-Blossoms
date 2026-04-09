package persistence

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
)

type SetIDMemoryStore struct {
	SetIDs              map[string]map[string]ports.SetID
	Bindings            map[string]map[string]ports.SetIDBindingRow
	ScopePackages       map[string]map[string]map[string]ports.ScopePackage
	ScopeSubscriptions  map[string]map[string]map[string]ports.ScopeSubscription
	GlobalScopePackages map[string]map[string]ports.ScopePackage
	GlobalSetIDName     string
	Seq                 int
}

func NewSetIDMemoryStore() *SetIDMemoryStore {
	return &SetIDMemoryStore{
		SetIDs:              make(map[string]map[string]ports.SetID),
		Bindings:            make(map[string]map[string]ports.SetIDBindingRow),
		ScopePackages:       make(map[string]map[string]map[string]ports.ScopePackage),
		ScopeSubscriptions:  make(map[string]map[string]map[string]ports.ScopeSubscription),
		GlobalScopePackages: make(map[string]map[string]ports.ScopePackage),
	}
}

func (s *SetIDMemoryStore) EnsureBootstrap(_ context.Context, tenantID string, _ string) error {
	if s.SetIDs[tenantID] == nil {
		s.SetIDs[tenantID] = make(map[string]ports.SetID)
	}
	if s.Bindings[tenantID] == nil {
		s.Bindings[tenantID] = make(map[string]ports.SetIDBindingRow)
	}
	if _, ok := s.SetIDs[tenantID]["DEFLT"]; !ok {
		s.SetIDs[tenantID]["DEFLT"] = ports.SetID{SetID: "DEFLT", Name: "Default", Status: "active"}
	}
	if s.GlobalSetIDName == "" {
		s.GlobalSetIDName = "Shared"
	}
	return nil
}

func (s *SetIDMemoryStore) ListSetIDs(ctx context.Context, tenantID string) ([]ports.SetID, error) {
	var out []ports.SetID
	globalSetids, _ := s.ListGlobalSetIDs(ctx)
	out = append(out, globalSetids...)
	for _, v := range s.SetIDs[tenantID] {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SetID < out[j].SetID })
	return out, nil
}

func (s *SetIDMemoryStore) ListGlobalSetIDs(_ context.Context) ([]ports.SetID, error) {
	if s.GlobalSetIDName == "" {
		return nil, nil
	}
	return []ports.SetID{{SetID: "SHARE", Name: s.GlobalSetIDName, Status: "active", IsShared: true}}, nil
}

func (s *SetIDMemoryStore) CreateSetID(_ context.Context, tenantID string, setID string, name string, _ string, _ string, _ string) error {
	setID = strings.ToUpper(strings.TrimSpace(setID))
	if setID == "" {
		return errors.New("setid is required")
	}
	if setID == "SHARE" {
		return errors.New("SETID_RESERVED: SHARE is reserved")
	}
	if s.SetIDs[tenantID] == nil {
		s.SetIDs[tenantID] = make(map[string]ports.SetID)
	}
	if _, ok := s.SetIDs[tenantID][setID]; ok {
		return errors.New("SETID_ALREADY_EXISTS")
	}
	s.SetIDs[tenantID][setID] = ports.SetID{SetID: setID, Name: name, Status: "active"}
	return nil
}

func (s *SetIDMemoryStore) ListSetIDBindings(_ context.Context, tenantID string, _ string) ([]ports.SetIDBindingRow, error) {
	var out []ports.SetIDBindingRow
	for _, v := range s.Bindings[tenantID] {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OrgUnitID < out[j].OrgUnitID })
	return out, nil
}

func (s *SetIDMemoryStore) BindSetID(_ context.Context, tenantID string, orgUnitID string, effectiveDate string, setID string, _ string, _ string) error {
	orgUnitID = strings.TrimSpace(orgUnitID)
	if orgUnitID == "" {
		return errors.New("org_unit_id is required")
	}
	setID = strings.ToUpper(strings.TrimSpace(setID))
	if setID == "" {
		return errors.New("setid is required")
	}
	if _, ok := s.SetIDs[tenantID][setID]; !ok {
		return errors.New("SETID_NOT_FOUND")
	}
	if s.Bindings[tenantID] == nil {
		s.Bindings[tenantID] = make(map[string]ports.SetIDBindingRow)
	}
	s.Bindings[tenantID][orgUnitID] = ports.SetIDBindingRow{
		OrgUnitID: orgUnitID,
		SetID:     setID,
		ValidFrom: effectiveDate,
	}
	return nil
}

func (s *SetIDMemoryStore) ResolveSetID(_ context.Context, tenantID string, orgUnitID string, _ string) (string, error) {
	binding, ok := s.Bindings[tenantID][strings.TrimSpace(orgUnitID)]
	if !ok {
		return "", errors.New("SETID_NOT_FOUND")
	}
	return strings.ToUpper(strings.TrimSpace(binding.SetID)), nil
}

func (s *SetIDMemoryStore) CreateGlobalSetID(_ context.Context, name string, _ string, _ string, actorScope string) error {
	if strings.TrimSpace(actorScope) != "saas" {
		return errors.New("ACTOR_SCOPE_FORBIDDEN")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	s.GlobalSetIDName = name
	return nil
}

func (s *SetIDMemoryStore) ListScopeCodes(_ context.Context, _ string) ([]ports.ScopeCode, error) {
	return []ports.ScopeCode{
		{ScopeCode: "jobcatalog", OwnerModule: "jobcatalog", ShareMode: "tenant-only", IsStable: true},
		{ScopeCode: "orgunit_geo_admin", OwnerModule: "orgunit", ShareMode: "shared-only", IsStable: true},
		{ScopeCode: "orgunit_location", OwnerModule: "orgunit", ShareMode: "shared-only", IsStable: true},
		{ScopeCode: "person_school", OwnerModule: "person", ShareMode: "shared-only", IsStable: true},
		{ScopeCode: "person_education_type", OwnerModule: "person", ShareMode: "shared-only", IsStable: true},
		{ScopeCode: "person_credential_type", OwnerModule: "person", ShareMode: "shared-only", IsStable: true},
	}, nil
}

func (s *SetIDMemoryStore) CreateScopePackage(_ context.Context, tenantID string, scopeCode string, packageCode string, ownerSetID string, name string, effectiveDate string, _ string, _ string) (ports.ScopePackage, error) {
	if s.ScopePackages[tenantID] == nil {
		s.ScopePackages[tenantID] = make(map[string]map[string]ports.ScopePackage)
	}
	if s.ScopePackages[tenantID][scopeCode] == nil {
		s.ScopePackages[tenantID][scopeCode] = make(map[string]ports.ScopePackage)
	}
	s.Seq++
	packageID := "pkg-" + strconv.Itoa(s.Seq)
	pkg := ports.ScopePackage{
		PackageID:     packageID,
		ScopeCode:     scopeCode,
		PackageCode:   packageCode,
		OwnerSetID:    ownerSetID,
		Name:          name,
		Status:        "active",
		EffectiveDate: effectiveDate,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	s.ScopePackages[tenantID][scopeCode][packageID] = pkg
	return pkg, nil
}

func (s *SetIDMemoryStore) DisableScopePackage(_ context.Context, tenantID string, packageID string, effectiveDate string, _ string, _ string) (ports.ScopePackage, error) {
	for scopeCode, pkgs := range s.ScopePackages[tenantID] {
		if pkg, ok := pkgs[packageID]; ok {
			pkg.Status = "disabled"
			pkg.EffectiveDate = effectiveDate
			s.ScopePackages[tenantID][scopeCode][packageID] = pkg
			return pkg, nil
		}
	}
	return ports.ScopePackage{}, errors.New("SCOPE_PACKAGE_NOT_FOUND")
}

func (s *SetIDMemoryStore) ListScopePackages(_ context.Context, tenantID string, scopeCode string) ([]ports.ScopePackage, error) {
	var out []ports.ScopePackage
	for _, pkg := range s.ScopePackages[tenantID][scopeCode] {
		out = append(out, pkg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PackageCode < out[j].PackageCode })
	return out, nil
}

func (s *SetIDMemoryStore) ListOwnedScopePackages(_ context.Context, tenantID string, scopeCode string, asOfDate string) ([]ports.OwnedScopePackage, error) {
	var out []ports.OwnedScopePackage
	for _, pkg := range s.ScopePackages[tenantID][scopeCode] {
		setid, ok := s.SetIDs[tenantID][pkg.OwnerSetID]
		if !ok || setid.Status != "active" || pkg.Status != "active" {
			continue
		}
		out = append(out, ports.OwnedScopePackage{
			PackageID:     pkg.PackageID,
			ScopeCode:     pkg.ScopeCode,
			PackageCode:   pkg.PackageCode,
			OwnerSetID:    pkg.OwnerSetID,
			Name:          pkg.Name,
			Status:        pkg.Status,
			EffectiveDate: asOfDate,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PackageCode < out[j].PackageCode })
	return out, nil
}

func (s *SetIDMemoryStore) CreateScopeSubscription(_ context.Context, tenantID string, setID string, scopeCode string, packageID string, packageOwner string, effectiveDate string, _ string, _ string) (ports.ScopeSubscription, error) {
	if s.ScopeSubscriptions[tenantID] == nil {
		s.ScopeSubscriptions[tenantID] = make(map[string]map[string]ports.ScopeSubscription)
	}
	if s.ScopeSubscriptions[tenantID][setID] == nil {
		s.ScopeSubscriptions[tenantID][setID] = make(map[string]ports.ScopeSubscription)
	}
	sub := ports.ScopeSubscription{
		SetID:         setID,
		ScopeCode:     scopeCode,
		PackageID:     packageID,
		PackageOwner:  packageOwner,
		EffectiveDate: effectiveDate,
	}
	s.ScopeSubscriptions[tenantID][setID][scopeCode] = sub
	return sub, nil
}

func (s *SetIDMemoryStore) GetScopeSubscription(_ context.Context, tenantID string, setID string, scopeCode string, _ string) (ports.ScopeSubscription, error) {
	sub, ok := s.ScopeSubscriptions[tenantID][setID][scopeCode]
	if !ok {
		return ports.ScopeSubscription{}, errors.New("SCOPE_SUBSCRIPTION_MISSING")
	}
	return sub, nil
}

func (s *SetIDMemoryStore) CreateGlobalScopePackage(_ context.Context, scopeCode string, packageCode string, name string, _ string, _ string, _ string, actorScope string) (ports.ScopePackage, error) {
	if strings.TrimSpace(actorScope) != "saas" {
		return ports.ScopePackage{}, errors.New("ACTOR_SCOPE_FORBIDDEN")
	}
	if s.GlobalScopePackages[scopeCode] == nil {
		s.GlobalScopePackages[scopeCode] = make(map[string]ports.ScopePackage)
	}
	s.Seq++
	packageID := "global-pkg-" + strconv.Itoa(s.Seq)
	pkg := ports.ScopePackage{
		PackageID:   packageID,
		ScopeCode:   scopeCode,
		PackageCode: packageCode,
		Name:        name,
		Status:      "active",
	}
	s.GlobalScopePackages[scopeCode][packageID] = pkg
	return pkg, nil
}

func (s *SetIDMemoryStore) ListGlobalScopePackages(_ context.Context, scopeCode string) ([]ports.ScopePackage, error) {
	var out []ports.ScopePackage
	for _, pkg := range s.GlobalScopePackages[scopeCode] {
		out = append(out, pkg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PackageCode < out[j].PackageCode })
	return out, nil
}
