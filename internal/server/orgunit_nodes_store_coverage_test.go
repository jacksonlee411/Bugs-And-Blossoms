package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestNewOrgNodeRequestID_Prefix(t *testing.T) {
	got := newOrgNodeRequestID("p")
	if !strings.HasPrefix(got, "p:") {
		t.Fatalf("id=%q", got)
	}
}

func TestParseOrgID8(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty", input: "", wantErr: true},
		{name: "len_mismatch", input: "123", wantErr: true},
		{name: "non_digit", input: "12ab5678", wantErr: true},
		{name: "out_of_range", input: "00000000", wantErr: true},
		{name: "ok", input: "10000001", wantErr: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseOrgID8(tc.input)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseOptionalOrgID8(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, ok, err := parseOptionalOrgID8("")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		if _, _, err := parseOptionalOrgID8("bad"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("ok", func(t *testing.T) {
		got, ok, err := parseOptionalOrgID8("10000001")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if got != 10000001 {
			t.Fatalf("expected 10000001, got %d", got)
		}
	})
}

func TestCanEditOrgNodes(t *testing.T) {
	if canEditOrgNodes(context.Background()) {
		t.Fatal("expected false without principal")
	}
	if canEditOrgNodes(withPrincipal(context.Background(), Principal{RoleSlug: " "})) {
		t.Fatal("expected false for empty role")
	}
	if canEditOrgNodes(withPrincipal(context.Background(), Principal{RoleSlug: "tenant-viewer"})) {
		t.Fatal("expected false for viewer")
	}
	if !canEditOrgNodes(withPrincipal(context.Background(), Principal{RoleSlug: authz.RoleTenantAdmin})) {
		t.Fatal("expected true for tenant admin")
	}
	if !canEditOrgNodes(withPrincipal(context.Background(), Principal{RoleSlug: " SUPERADMIN "})) {
		t.Fatal("expected true for superadmin")
	}
}

func TestOrgUnitInitiatorUUID_PrefersValidPrincipalID(t *testing.T) {
	ctx := withPrincipal(context.Background(), Principal{ID: "00000000-0000-0000-0000-000000000000", RoleSlug: authz.RoleTenantAdmin})
	if got := orgUnitInitiatorUUID(ctx, "t1"); got != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("got=%q", got)
	}
}

func TestOrgUnitInitiatorUUID_FallsBackToTenantID(t *testing.T) {
	ctx := withPrincipal(context.Background(), Principal{ID: "not-a-uuid", RoleSlug: authz.RoleTenantAdmin})
	if got := orgUnitInitiatorUUID(ctx, "t1"); got != "t1" {
		t.Fatalf("got=%q", got)
	}
}

func TestOrgNodeWriteErrorMessage(t *testing.T) {
	if got := orgNodeWriteErrorMessage(errors.New("EVENT_DATE_CONFLICT")); got != "生效日期冲突：该生效日已存在记录。请修改“生效日期”（新增/插入记录）或使用“修正”修改该生效日记录后重试。" {
		t.Fatalf("got=%q", got)
	}
	if got := orgNodeWriteErrorMessage(errors.New("ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET")); got != "该记录不支持同日状态纠错" {
		t.Fatalf("got=%q", got)
	}
	if got := orgNodeWriteErrorMessage(errors.New("ORG_HIGH_RISK_REORDER_FORBIDDEN")); got != "该变更会触发高风险全量重放，请改用新增/插入记录" {
		t.Fatalf("got=%q", got)
	}
	if got := orgNodeWriteErrorMessage(errors.New("ORGUNIT_CODES_WRITE_FORBIDDEN")); got != "系统写入权限异常（ORGUNIT_CODES_WRITE_FORBIDDEN），请联系管理员" {
		t.Fatalf("got=%q", got)
	}
	if got := orgNodeWriteErrorMessage(errors.New("boom")); got != "boom" {
		t.Fatalf("got=%q", got)
	}
}

func TestIncludeDisabledHelpers(t *testing.T) {
	reqURL := httptest.NewRequest(http.MethodGet, "/org/api/org-units?include_disabled=1", nil)
	if !includeDisabledFromURL(reqURL) {
		t.Fatal("expected include_disabled from url")
	}

	reqForm := httptest.NewRequest(http.MethodPost, "/org/api/org-units", strings.NewReader("include_disabled=true"))
	reqForm.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = reqForm.ParseForm()
	if !includeDisabledFromFormOrURL(reqForm) {
		t.Fatal("expected include_disabled from form")
	}

	reqURLOnly := httptest.NewRequest(http.MethodPost, "/org/api/org-units?include_disabled=1", nil)
	reqURLOnly.Form = make(url.Values) // avoid ParseForm merging query into form
	if !includeDisabledFromFormOrURL(reqURLOnly) {
		t.Fatal("expected include_disabled fallback to url")
	}
	reqNone := httptest.NewRequest(http.MethodPost, "/org/api/org-units", nil)
	reqNone.Form = make(url.Values)
	if includeDisabledFromFormOrURL(reqNone) {
		t.Fatal("expected include_disabled false")
	}

	if includeDisabledQuerySuffix(false) != "" {
		t.Fatalf("unexpected suffix=%q", includeDisabledQuerySuffix(false))
	}
	if includeDisabledQuerySuffix(true) != "&include_disabled=1" {
		t.Fatalf("unexpected suffix=%q", includeDisabledQuerySuffix(true))
	}
}

func TestOrgNodeAuditLimitAndTabFromURL(t *testing.T) {
	if got := orgNodeAuditLimitFromURL(httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit", nil)); got != orgNodeAuditPageSize {
		t.Fatalf("limit=%d", got)
	}
	if got := orgNodeAuditLimitFromURL(httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?limit=x", nil)); got != orgNodeAuditPageSize {
		t.Fatalf("limit=%d", got)
	}
	if got := orgNodeAuditLimitFromURL(httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?limit=0", nil)); got != orgNodeAuditPageSize {
		t.Fatalf("limit=%d", got)
	}
	if got := orgNodeAuditLimitFromURL(httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?limit=5", nil)); got != 5 {
		t.Fatalf("limit=%d", got)
	}

	if got := orgNodeActiveTabFromURL(httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?tab=change", nil)); got != "change" {
		t.Fatalf("tab=%q", got)
	}
	if got := orgNodeActiveTabFromURL(httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?tab=%20CHANGE%20", nil)); got != "change" {
		t.Fatalf("tab=%q", got)
	}
	if got := orgNodeActiveTabFromURL(httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?tab=basic", nil)); got != "basic" {
		t.Fatalf("tab=%q", got)
	}
}

func TestOrgUnitLabelsAndTargetStatus(t *testing.T) {
	if orgUnitStatusLabel("disabled") != "无效" {
		t.Fatalf("unexpected status label")
	}
	if orgUnitStatusLabel("ACTIVE") != "有效" {
		t.Fatalf("unexpected status label")
	}

	if orgUnitBusinessUnitText(true) != "是" {
		t.Fatalf("unexpected bu text")
	}
	if orgUnitBusinessUnitText(false) != "否" {
		t.Fatalf("unexpected bu text")
	}

	if got, err := normalizeOrgUnitTargetStatus("有效"); err != nil || got != "active" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if got, err := normalizeOrgUnitTargetStatus("inactive"); err != nil || got != "disabled" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if _, err := normalizeOrgUnitTargetStatus("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOrgUnitMemoryStore_BasicCRUDAndResolve(t *testing.T) {
	s := newOrgUnitMemoryStore()
	s.now = func() time.Time { return time.Unix(123, 0).UTC() }

	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "Hello World", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, "Hello"); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "Hello" || nodes[0].OrgCode != "A001" {
		t.Fatalf("nodes=%v", nodes)
	}
	if nodes[0].CreatedAt != time.Unix(123, 0).UTC() {
		t.Fatalf("created_at=%s", nodes[0].CreatedAt)
	}

	orgID, err := s.ResolveOrgID(context.Background(), "t1", "A001")
	if err != nil || orgID != 10000000 {
		t.Fatalf("orgID=%d err=%v", orgID, err)
	}
	if code, err := s.ResolveOrgCode(context.Background(), "t1", 10000000); err != nil || code != "A001" {
		t.Fatalf("code=%q err=%v", code, err)
	}
}

func TestOrgUnitMemoryStore_ResolveErrors(t *testing.T) {
	s := newOrgUnitMemoryStore()
	if _, err := s.ResolveOrgID(context.Background(), "t1", "bad\x7f"); !errors.Is(err, orgunitpkg.ErrOrgCodeInvalid) {
		t.Fatalf("err=%v", err)
	}
	if _, err := s.ResolveOrgID(context.Background(), "t1", "A001"); !errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
		t.Fatalf("err=%v", err)
	}
	if _, err := s.ResolveOrgCode(context.Background(), "t1", 10000001); !errors.Is(err, orgunitpkg.ErrOrgIDNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitMemoryStore_VisibilityMethodsAndVersions(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatal(err)
	}
	orgID, _ := parseOrgID8(created.ID)

	// Visibility methods should delegate to their base variants.
	if _, err := store.ListNodesCurrentWithVisibility(ctx, "t1", "2026-01-01", true); err != nil {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.ListChildrenWithVisibility(ctx, "t1", orgID, "2026-01-01", true); err != nil {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.GetNodeDetailsWithVisibility(ctx, "t1", orgID, "2026-01-01", true); err != nil {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.SearchNodeWithVisibility(ctx, "t1", "A001", "2026-01-01", true); err != nil {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "root", "2026-01-01", 8, true); err != nil {
		t.Fatalf("err=%v", err)
	}

	versions, err := store.ListNodeVersions(ctx, "t1", orgID)
	if err != nil || len(versions) != 1 {
		t.Fatalf("versions=%v err=%v", versions, err)
	}
	if _, err := store.ListNodeVersions(ctx, "t1", orgID+1); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitMemoryStore_SearchCandidatesAndRenameErrors(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatal(err)
	}

	// SearchNodeCandidates: empty query, limit default, not found.
	if _, err := store.SearchNodeCandidates(ctx, "t1", "", "2026-01-01", 0); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.SearchNodeCandidates(ctx, "t1", "missing", "2026-01-01", 0); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("err=%v", err)
	}
	if got, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 0); err != nil || len(got) != 1 {
		t.Fatalf("got=%v err=%v", got, err)
	}

	// Rename errors: invalid org id, missing name, org not found.
	if err := store.RenameNodeCurrent(ctx, "t1", "2026-01-01", "bad", "X"); err == nil {
		t.Fatal("expected error")
	}
	if err := store.RenameNodeCurrent(ctx, "t1", "2026-01-01", created.ID, " "); err == nil {
		t.Fatal("expected error")
	}
	if err := store.RenameNodeCurrent(ctx, "t1", "2026-01-01", "10000002", "X"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOrgUnitMemoryStore_SearchCandidates_LimitBreak(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "A001", "Root A", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "A002", "Root B", "", false); err != nil {
		t.Fatal(err)
	}
	out, err := store.SearchNodeCandidates(ctx, "t1", "root", "2026-01-01", 1)
	if err != nil || len(out) != 1 {
		t.Fatalf("out=%v err=%v", out, err)
	}
}

func TestOrgUnitMemoryStore_CreateAndSearch_IDConversionErrors(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()

	if _, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "bad\x7f", "X", "", false); err == nil {
		t.Fatal("expected error")
	}

	// Force strconv.Atoi() failures in search paths.
	store.nodes["t1"] = []OrgUnitNode{{ID: "bad", OrgCode: "A001", Name: "Root", Status: "active"}}
	if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-01"); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.SearchNode(ctx, "t1", "root", "2026-01-01"); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 1); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.SearchNodeCandidates(ctx, "t1", "root", "2026-01-01", 1); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitMemoryStore_ResolveOrgCodes(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()

	got, err := store.ResolveOrgCodes(ctx, "t1", nil)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}

	n1, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "A1", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}
	n2, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "B1", "B", "", false)
	if err != nil {
		t.Fatal(err)
	}
	id1, _ := parseOrgID8(n1.ID)
	id2, _ := parseOrgID8(n2.ID)

	codes, err := store.ResolveOrgCodes(ctx, "t1", []int{id1, id2})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if codes[id1] != "A1" || codes[id2] != "B1" {
		t.Fatalf("unexpected codes: %v", codes)
	}

	if _, err := store.ResolveOrgCodes(ctx, "t1", []int{id1, 99999999}); !errors.Is(err, orgunitpkg.ErrOrgIDNotFound) {
		t.Fatalf("err=%v", err)
	}

	badStore := newOrgUnitMemoryStore()
	badStore.nodes["t1"] = []OrgUnitNode{{ID: "bad", OrgCode: "A1"}}
	if _, err := badStore.ResolveOrgCodes(ctx, "t1", []int{10000001}); !errors.Is(err, orgunitpkg.ErrOrgIDNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitMemoryStore_ResolveSetID(t *testing.T) {
	store := newOrgUnitMemoryStore()

	if _, err := store.ResolveSetID(context.Background(), "t1", "", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	got, err := store.ResolveSetID(context.Background(), "t1", "10000001", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != "S2601" {
		t.Fatalf("expected S2601, got %q", got)
	}
}

func TestOrgUnitMemoryStore_MoveDisableSetBusinessUnitErrors(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A003", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000002", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000002"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID); err != nil {
		t.Fatalf("err=%v", err)
	}

	if err := s.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-06", "", true, "r1"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-06", "10000002", true, "r1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOrgUnitMemoryStore_ListChildrenAndDetailsAndSearch(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()
	n, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatal(err)
	}
	orgID, _ := parseOrgID8(n.ID)

	children, err := store.ListChildren(ctx, "t1", orgID, "2026-01-01")
	if err != nil || len(children) != 0 {
		t.Fatalf("children=%v err=%v", children, err)
	}
	if _, err := store.ListChildren(ctx, "t1", orgID+1, "2026-01-01"); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("err=%v", err)
	}

	details, err := store.GetNodeDetails(ctx, "t1", orgID, "2026-01-01")
	if err != nil || details.OrgCode != "A001" {
		t.Fatalf("details=%+v err=%v", details, err)
	}
	if _, err := store.GetNodeDetails(ctx, "t1", orgID+1, "2026-01-01"); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("err=%v", err)
	}

	if _, err := store.SearchNode(ctx, "t1", "", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-01"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if _, err := store.SearchNode(ctx, "t1", "root", "2026-01-01"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if _, err := store.SearchNode(ctx, "t1", "missing", "2026-01-01"); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestVisibilityWrappers(t *testing.T) {
	ctx := context.Background()

	t.Run("listNodes includeDisabled=false uses base method", func(t *testing.T) {
		baseCalled := false
		s := &struct{ OrgUnitStore }{}
		s.OrgUnitStore = orgUnitStoreStub{
			listNodesFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				baseCalled = true
				return []OrgUnitNode{{OrgCode: "A001"}}, nil
			},
		}
		_, _ = listNodesCurrentByVisibility(ctx, s, "t1", "2026-01-01", false)
		if !baseCalled {
			t.Fatal("expected base called")
		}
	})

	t.Run("listNodes includeDisabled=true uses visibility method when available", func(t *testing.T) {
		visCalled := false
		s := orgUnitVisibilityStub{
			listNodesVisFn: func(context.Context, string, string, bool) ([]OrgUnitNode, error) {
				visCalled = true
				return nil, nil
			},
		}
		_, _ = listNodesCurrentByVisibility(ctx, s, "t1", "2026-01-01", true)
		if !visCalled {
			t.Fatal("expected visibility called")
		}
	})

	t.Run("details includeDisabled=true uses visibility method when available", func(t *testing.T) {
		visCalled := false
		s := orgUnitVisibilityStub{
			detailsVisFn: func(context.Context, string, int, string, bool) (OrgUnitNodeDetails, error) {
				visCalled = true
				return OrgUnitNodeDetails{}, nil
			},
		}
		_, _ = getNodeDetailsByVisibility(ctx, s, "t1", 1, "2026-01-01", true)
		if !visCalled {
			t.Fatal("expected visibility called")
		}
	})

	t.Run("children includeDisabled=true uses visibility method when available", func(t *testing.T) {
		visCalled := false
		s := orgUnitVisibilityStub{
			childrenVisFn: func(context.Context, string, int, string, bool) ([]OrgUnitChild, error) {
				visCalled = true
				return nil, nil
			},
		}
		_, _ = listChildrenByVisibility(ctx, s, "t1", 1, "2026-01-01", true)
		if !visCalled {
			t.Fatal("expected visibility called")
		}
	})

	t.Run("search includeDisabled=true uses visibility method when available", func(t *testing.T) {
		visCalled := false
		s := orgUnitVisibilityStub{
			searchVisFn: func(context.Context, string, string, string, bool) (OrgUnitSearchResult, error) {
				visCalled = true
				return OrgUnitSearchResult{}, nil
			},
		}
		_, _ = searchNodeByVisibility(ctx, s, "t1", "q", "2026-01-01", true)
		if !visCalled {
			t.Fatal("expected visibility called")
		}
	})

	t.Run("candidates includeDisabled=true uses visibility method when available", func(t *testing.T) {
		visCalled := false
		s := orgUnitVisibilityStub{
			searchCandidatesVisFn: func(context.Context, string, string, string, int, bool) ([]OrgUnitSearchCandidate, error) {
				visCalled = true
				return nil, nil
			},
		}
		_, _ = searchNodeCandidatesByVisibility(ctx, s, "t1", "q", "2026-01-01", 8, true)
		if !visCalled {
			t.Fatal("expected visibility called")
		}
	})

	t.Run("candidates includeDisabled=false uses base method", func(t *testing.T) {
		called := false
		_, _ = searchNodeCandidatesByVisibility(ctx, storeStubCandidates{called: &called}, "t1", "q", "2026-01-01", 8, false)
		if !called {
			t.Fatal("expected base called")
		}
	})

	t.Run("candidates includeDisabled=true falls back to base when visibility method missing", func(t *testing.T) {
		called := false
		_, _ = searchNodeCandidatesByVisibility(ctx, storeStubCandidates{called: &called}, "t1", "q", "2026-01-01", 8, true)
		if !called {
			t.Fatal("expected base called")
		}
	})
}

type storeStubCandidates struct {
	orgUnitStoreStub
	called *bool
}

func (s storeStubCandidates) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	if s.called != nil {
		*s.called = true
	}
	return nil, nil
}

type orgUnitStoreStub struct {
	listNodesFn func(context.Context, string, string) ([]OrgUnitNode, error)
}

func (s orgUnitStoreStub) ListNodesCurrent(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error) {
	if s.listNodesFn != nil {
		return s.listNodesFn(ctx, tenantID, asOfDate)
	}
	return nil, nil
}

func (orgUnitStoreStub) CreateNodeCurrent(context.Context, string, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}
func (orgUnitStoreStub) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (orgUnitStoreStub) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (orgUnitStoreStub) DisableNodeCurrent(context.Context, string, string, string) error { return nil }
func (orgUnitStoreStub) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return nil
}
func (orgUnitStoreStub) ResolveOrgID(context.Context, string, string) (int, error) { return 0, nil }
func (orgUnitStoreStub) ResolveOrgCode(context.Context, string, int) (string, error) {
	return "", nil
}
func (orgUnitStoreStub) ResolveOrgCodes(context.Context, string, []int) (map[int]string, error) {
	return nil, nil
}
func (orgUnitStoreStub) ListChildren(context.Context, string, int, string) ([]OrgUnitChild, error) {
	return nil, nil
}
func (orgUnitStoreStub) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	return OrgUnitNodeDetails{}, nil
}
func (orgUnitStoreStub) SearchNode(context.Context, string, string, string) (OrgUnitSearchResult, error) {
	return OrgUnitSearchResult{}, nil
}
func (orgUnitStoreStub) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return nil, nil
}
func (orgUnitStoreStub) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	return nil, nil
}
func (orgUnitStoreStub) MaxEffectiveDateOnOrBefore(context.Context, string, string) (string, bool, error) {
	return "", false, nil
}
func (orgUnitStoreStub) MinEffectiveDate(context.Context, string) (string, bool, error) {
	return "", false, nil
}

type orgUnitVisibilityStub struct {
	orgUnitStoreStub
	listNodesVisFn        func(context.Context, string, string, bool) ([]OrgUnitNode, error)
	childrenVisFn         func(context.Context, string, int, string, bool) ([]OrgUnitChild, error)
	detailsVisFn          func(context.Context, string, int, string, bool) (OrgUnitNodeDetails, error)
	searchVisFn           func(context.Context, string, string, string, bool) (OrgUnitSearchResult, error)
	searchCandidatesVisFn func(context.Context, string, string, string, int, bool) ([]OrgUnitSearchCandidate, error)
}

func (s orgUnitVisibilityStub) ListNodesCurrentWithVisibility(ctx context.Context, tenantID string, asOfDate string, includeDisabled bool) ([]OrgUnitNode, error) {
	if s.listNodesVisFn != nil {
		return s.listNodesVisFn(ctx, tenantID, asOfDate, includeDisabled)
	}
	return nil, nil
}
func (s orgUnitVisibilityStub) ListChildrenWithVisibility(ctx context.Context, tenantID string, parentID int, asOfDate string, includeDisabled bool) ([]OrgUnitChild, error) {
	if s.childrenVisFn != nil {
		return s.childrenVisFn(ctx, tenantID, parentID, asOfDate, includeDisabled)
	}
	return nil, nil
}
func (s orgUnitVisibilityStub) GetNodeDetailsWithVisibility(ctx context.Context, tenantID string, orgID int, asOfDate string, includeDisabled bool) (OrgUnitNodeDetails, error) {
	if s.detailsVisFn != nil {
		return s.detailsVisFn(ctx, tenantID, orgID, asOfDate, includeDisabled)
	}
	return OrgUnitNodeDetails{}, nil
}
func (s orgUnitVisibilityStub) SearchNodeWithVisibility(ctx context.Context, tenantID string, query string, asOfDate string, includeDisabled bool) (OrgUnitSearchResult, error) {
	if s.searchVisFn != nil {
		return s.searchVisFn(ctx, tenantID, query, asOfDate, includeDisabled)
	}
	return OrgUnitSearchResult{}, nil
}
func (s orgUnitVisibilityStub) SearchNodeCandidatesWithVisibility(ctx context.Context, tenantID string, query string, asOfDate string, limit int, includeDisabled bool) ([]OrgUnitSearchCandidate, error) {
	if s.searchCandidatesVisFn != nil {
		return s.searchCandidatesVisFn(ctx, tenantID, query, asOfDate, limit, includeDisabled)
	}
	return nil, nil
}

func TestListNodeAuditEventsHelper(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "A001", "Org", "", true)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := parseOrgID8(created.ID)

	// Non-audit reader is allowed: should return empty without error.
	events, err := listNodeAuditEvents(ctx, orgUnitStoreStub{}, "t1", id, 1)
	if err != nil || len(events) != 0 {
		t.Fatalf("events=%v err=%v", events, err)
	}

	events, err = store.ListNodeAuditEvents(ctx, "t1", id, 0)
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%v err=%v", events, err)
	}
	if _, err := store.ListNodeAuditEvents(ctx, "t1", id+1, 1); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitPGStore_ListNodeAuditEvents(t *testing.T) {
	ctx := context.Background()
	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 1); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("set tenant error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 1); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("query error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		})}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 1); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{scanErr: errors.New("scan")}}, nil
		})}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 1); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("rows err", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{err: errors.New("rows")}}, nil
		})}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 1); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("commit error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{}, commitErr: errors.New("commit")}, nil
		})}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 1); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("ok", func(t *testing.T) {
		when := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
		rows := &auditRows{records: [][]any{{
			int64(1),                  // id
			"e1",                      // event_uuid
			10000001,                  // org_id
			"RENAME",                  // event_type
			when,                      // effective_date
			when,                      // tx_time
			"initiator",               // initiator_name
			"emp",                     // initiator_employee_id
			"req",                     // request_code
			"reason",                  // reason
			[]byte(`{"op":"RENAME"}`), // payload
			[]byte(`{"before":1}`),    // before_snapshot
			[]byte(`{"after":2}`),     // after_snapshot
			"",                        // rescind_outcome
			false,                     // is_rescinded
			"",                        // rescinded_by_event_uuid
			when,                      // rescinded_by_tx_time
			"",                        // rescinded_by_request_code
		}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: rows}, nil
		})}
		events, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 0)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(events) == 0 {
			t.Fatal("expected some events")
		}
	})
}

type auditRows struct {
	records [][]any
	idx     int
	scanErr error
	err     error
}

func (r *auditRows) Close()                        {}
func (r *auditRows) Err() error                    { return r.err }
func (r *auditRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *auditRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *auditRows) Next() bool {
	if r.idx >= len(r.records) {
		return false
	}
	r.idx++
	return true
}
func (r *auditRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	rec := r.records[r.idx-1]
	for i := range dest {
		if i >= len(rec) || rec[i] == nil {
			continue
		}
		switch d := dest[i].(type) {
		case *int64:
			*d = rec[i].(int64)
		case *int:
			*d = rec[i].(int)
		case *string:
			*d = rec[i].(string)
		case *time.Time:
			*d = rec[i].(time.Time)
		case *[]byte:
			*d = append([]byte(nil), rec[i].([]byte)...)
		case *bool:
			*d = rec[i].(bool)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}
func (r *auditRows) Values() ([]any, error) { return nil, nil }
func (r *auditRows) RawValues() [][]byte    { return nil }
func (r *auditRows) Conn() *pgx.Conn        { return nil }

func TestOrgUnitMemoryStore_AppendFactsHelpers(t *testing.T) {
	store := newOrgUnitMemoryStore()
	initialized, err := store.IsOrgTreeInitialized(context.Background(), "t1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if initialized {
		t.Fatalf("expected empty tree not initialized")
	}

	node, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "ROOT", "Root", "", true)
	if err != nil {
		t.Fatalf("create err=%v", err)
	}
	store.nodes["t1"][0].Status = ""

	orgID, err := store.ResolveOrgID(context.Background(), "t1", node.OrgCode)
	if err != nil {
		t.Fatalf("resolve id err=%v", err)
	}
	facts, err := store.ResolveAppendFacts(context.Background(), "t1", orgID, "2026-01-01")
	if err != nil {
		t.Fatalf("facts err=%v", err)
	}
	if !facts.TreeInitialized || !facts.TargetExistsAsOf || !facts.IsRoot {
		t.Fatalf("facts=%+v", facts)
	}
	if facts.TargetStatusAsOf != "active" {
		t.Fatalf("status=%q", facts.TargetStatusAsOf)
	}

	missing, err := store.ResolveAppendFacts(context.Background(), "t1", orgID+999, "2026-01-01")
	if err != nil {
		t.Fatalf("missing err=%v", err)
	}
	if !missing.TreeInitialized || missing.TargetExistsAsOf {
		t.Fatalf("missing facts=%+v", missing)
	}
}
