package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

func TestNewCubeBoxOrgUnitRegisteredExecutors(t *testing.T) {
	items, err := newCubeBoxOrgUnitRegisteredExecutors(newOrgUnitMemoryStore())
	if err != nil {
		t.Fatalf("newCubeBoxOrgUnitRegisteredExecutors err=%v", err)
	}
	if len(items) != 4 {
		t.Fatalf("items=%d", len(items))
	}
	registry, err := cubebox.NewExecutionRegistry(items...)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	for _, apiKey := range []string{"orgunit.details", "orgunit.list", "orgunit.search", "orgunit.audit"} {
		if _, ok := registry.Resolve(apiKey); !ok {
			t.Fatalf("api_key %q not registered", apiKey)
		}
	}
	list, ok := registry.Resolve("orgunit.list")
	if !ok {
		t.Fatal("orgunit.list not registered")
	}
	if !containsString(list.OptionalParams, "all_org_units") {
		t.Fatalf("orgunit.list optional params missing all_org_units: %#v", list.OptionalParams)
	}
}

func TestNewCubeBoxOrgUnitRegisteredExecutorsAllowsNonDetailsExecutorsWithoutExtStore(t *testing.T) {
	store := &resolveOrgCodeStore{}
	items, err := newCubeBoxOrgUnitRegisteredExecutors(store)
	if err != nil {
		t.Fatalf("newCubeBoxOrgUnitRegisteredExecutors err=%v", err)
	}
	registry, err := cubebox.NewExecutionRegistry(items...)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	if _, ok := registry.Resolve("orgunit.details"); ok {
		t.Fatal("orgunit.details should not be registered without ext store")
	}
	for _, apiKey := range []string{"orgunit.list", "orgunit.search", "orgunit.audit"} {
		if _, ok := registry.Resolve(apiKey); !ok {
			t.Fatalf("api_key %q not registered", apiKey)
		}
	}
}

func TestCubeBoxOrgUnitDetailsExecutor(t *testing.T) {
	store := &orgUnitDetailsExtStoreStub{
		resolveOrgCodeStore: &resolveOrgCodeStore{
			resolveID: 1001,
			getNodeDetails: OrgUnitNodeDetails{
				OrgNodeKey:     "10000001",
				OrgCode:        "1001",
				Name:           "华东销售中心",
				Status:         "active",
				ParentCode:     "0001",
				ParentName:     "总部",
				IsBusinessUnit: true,
				ManagerPernr:   "00010001",
				ManagerName:    "张三",
				FullNamePath:   "总部 / 华东销售中心",
				CreatedAt:      time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC),
				UpdatedAt:      time.Date(2026, 4, 2, 8, 0, 0, 0, time.UTC),
				EventUUID:      "evt-1",
			},
		},
		cfgs: []orgUnitTenantFieldConfig{
			{FieldKey: "short_name", PhysicalCol: "ext_str_01"},
		},
		snapshot: orgUnitVersionExtSnapshot{
			VersionValues: map[string]any{
				"ext_str_01": "华东",
			},
		},
	}

	items, err := newCubeBoxOrgUnitRegisteredExecutors(store)
	if err != nil {
		t.Fatalf("newCubeBoxOrgUnitRegisteredExecutors err=%v", err)
	}
	registry, err := cubebox.NewExecutionRegistry(items...)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	plan := cubebox.ReadPlan{
		Intent:        "orgunit.details",
		Confidence:    0.9,
		MissingParams: []string{},
		Steps: []cubebox.ReadPlanStep{
			{
				ID:          "step-1",
				APIKey:      "orgunit.details",
				Params:      map[string]any{"org_code": "1001", "as_of": "2026-04-23", "include_disabled": false},
				ResultFocus: []string{"org_unit.name", "ext_fields"},
				DependsOn:   []string{},
			},
		},
		ExplainFocus: []string{"详情"},
	}

	results, err := registry.ExecutePlan(context.Background(), cubebox.ExecuteRequest{
		TenantID:    "t1",
		PrincipalID: "p1",
	}, plan)
	if err != nil {
		t.Fatalf("ExecutePlan err=%v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results=%d", len(results))
	}
	orgUnit, ok := results[0].Payload["org_unit"].(map[string]any)
	if !ok {
		t.Fatalf("org_unit=%T", results[0].Payload["org_unit"])
	}
	if got := orgUnit["name"]; got != "华东销售中心" {
		t.Fatalf("org_unit.name=%v", got)
	}
	extFields, ok := results[0].Payload["ext_fields"].([]any)
	if !ok || len(extFields) != 1 {
		t.Fatalf("ext_fields=%T len=%d", results[0].Payload["ext_fields"], len(extFields))
	}
	if store.snapshotByNodeKeyArg != "10000001" {
		t.Fatalf("snapshotByNodeKeyArg=%q", store.snapshotByNodeKeyArg)
	}
}

func TestCubeBoxOrgUnitListExecutor(t *testing.T) {
	store := &orgUnitListPageReaderStore{
		resolveOrgCodeStore: &resolveOrgCodeStore{
			resolveID: 1001,
		},
		items: []orgUnitListItem{
			{OrgCode: "1002", Name: "上海销售组", Status: "active"},
		},
		total: 12,
	}
	executor := cubeBoxOrgUnitListExecutor{store: store}
	params, err := executor.ValidateParams(map[string]any{
		"as_of":            "2026-04-23",
		"parent_org_code":  "1001",
		"include_disabled": true,
		"keyword":          "销售",
		"status":           "disabled",
		"is_business_unit": true,
		"page":             float64(2),
		"size":             float64(5),
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	result, err := executor.Execute(context.Background(), cubebox.ExecuteRequest{TenantID: "t1"}, params)
	if err != nil {
		t.Fatalf("Execute err=%v", err)
	}
	if store.capturedReq.ParentOrgNodeKey == nil || *store.capturedReq.ParentOrgNodeKey == "" {
		t.Fatal("expected parent_org_node_key")
	}
	if store.capturedReq.Status != orgUnitListStatusDisabled {
		t.Fatalf("status=%q", store.capturedReq.Status)
	}
	if store.capturedReq.IsBusinessUnit == nil || !*store.capturedReq.IsBusinessUnit {
		t.Fatalf("isBusinessUnit=%v", store.capturedReq.IsBusinessUnit)
	}
	if store.capturedReq.Limit != 5 || store.capturedReq.Offset != 5 {
		t.Fatalf("limit=%d offset=%d", store.capturedReq.Limit, store.capturedReq.Offset)
	}
	if got := result.Payload["total"]; got != float64(12) {
		t.Fatalf("total=%v", got)
	}
	if result.ConfirmedEntity != nil {
		t.Fatalf("list result must not confirm a single entity, got %#v", result.ConfirmedEntity)
	}
}

func TestCubeBoxOrgUnitListExecutorRejectsNonBoolBusinessUnitFilter(t *testing.T) {
	executor := cubeBoxOrgUnitListExecutor{}
	_, err := executor.ValidateParams(map[string]any{
		"as_of":            "2026-04-23",
		"is_business_unit": "true",
	})
	if err == nil {
		t.Fatal("expected is_business_unit validation error")
	}
}

func TestCubeBoxOrgUnitListExecutorPaginatesWhenOnlyPageProvided(t *testing.T) {
	store := &orgUnitListPageReaderStore{
		resolveOrgCodeStore: &resolveOrgCodeStore{},
		items: []orgUnitListItem{
			{OrgCode: "1002", Name: "上海销售组", Status: "active"},
		},
		total: 12,
	}
	executor := cubeBoxOrgUnitListExecutor{store: store}
	params, err := executor.ValidateParams(map[string]any{
		"as_of": "2026-04-23",
		"page":  float64(2),
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	result, err := executor.Execute(context.Background(), cubebox.ExecuteRequest{TenantID: "t1"}, params)
	if err != nil {
		t.Fatalf("Execute err=%v", err)
	}
	if store.capturedReq.Limit != cubeBoxOrgUnitListDefaultPageSize || store.capturedReq.Offset != cubeBoxOrgUnitListDefaultPageSize {
		t.Fatalf("limit=%d offset=%d", store.capturedReq.Limit, store.capturedReq.Offset)
	}
	if got := result.Payload["page"]; got != float64(2) {
		t.Fatalf("page=%v", got)
	}
	if got := result.Payload["size"]; got != float64(cubeBoxOrgUnitListDefaultPageSize) {
		t.Fatalf("size=%v", got)
	}
	if got := result.Payload["total"]; got != float64(12) {
		t.Fatalf("total=%v", got)
	}
}

func TestCubeBoxOrgUnitListExecutorDefaultsPaginationWhenOmitted(t *testing.T) {
	store := &orgUnitListPageReaderStore{
		resolveOrgCodeStore: &resolveOrgCodeStore{},
		items: []orgUnitListItem{
			{OrgCode: "1002", Name: "上海销售组", Status: "active"},
		},
		total: 212,
	}
	executor := cubeBoxOrgUnitListExecutor{store: store}
	params, err := executor.ValidateParams(map[string]any{
		"as_of": "2026-04-23",
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	result, err := executor.Execute(context.Background(), cubebox.ExecuteRequest{TenantID: "t1"}, params)
	if err != nil {
		t.Fatalf("Execute err=%v", err)
	}
	if store.capturedReq.Limit != cubeBoxOrgUnitListDefaultPageSize || store.capturedReq.Offset != 0 {
		t.Fatalf("limit=%d offset=%d", store.capturedReq.Limit, store.capturedReq.Offset)
	}
	if got := result.Payload["page"]; got != float64(1) {
		t.Fatalf("page=%v", got)
	}
	if got := result.Payload["size"]; got != float64(cubeBoxOrgUnitListDefaultPageSize) {
		t.Fatalf("size=%v", got)
	}
	if got := result.Payload["total"]; got != float64(212) {
		t.Fatalf("total=%v", got)
	}
}

func TestCubeBoxOrgUnitListExecutorSupportsAllOrgUnitsScope(t *testing.T) {
	store := &orgUnitListPageReaderStore{
		resolveOrgCodeStore: &resolveOrgCodeStore{},
		items: []orgUnitListItem{
			{OrgCode: "100000", Name: "飞虫与鲜花", Status: "active"},
			{OrgCode: "200007", Name: "成本C组", Status: "active"},
		},
		total: 10,
	}
	executor := cubeBoxOrgUnitListExecutor{store: store}
	params, err := executor.ValidateParams(map[string]any{
		"as_of":            "2026-04-27",
		"all_org_units":    true,
		"page":             float64(1),
		"size":             float64(100),
		"include_disabled": false,
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	result, err := executor.Execute(context.Background(), cubebox.ExecuteRequest{TenantID: "t1"}, params)
	if err != nil {
		t.Fatalf("Execute err=%v", err)
	}
	if !store.capturedReq.AllOrgUnits {
		t.Fatalf("expected all org units scope, got %+v", store.capturedReq)
	}
	if store.capturedReq.ParentOrgNodeKey != nil {
		t.Fatalf("all_org_units must not require parent scope, got %+v", store.capturedReq)
	}
	if got := result.Payload["total"]; got != float64(10) {
		t.Fatalf("total=%v", got)
	}
}

func TestCubeBoxOrgUnitListExecutorDefaultsToFirstPageWhenOnlySizeProvided(t *testing.T) {
	store := &orgUnitListPageReaderStore{
		resolveOrgCodeStore: &resolveOrgCodeStore{},
		items: []orgUnitListItem{
			{OrgCode: "1002", Name: "上海销售组", Status: "active"},
		},
		total: 12,
	}
	executor := cubeBoxOrgUnitListExecutor{store: store}
	params, err := executor.ValidateParams(map[string]any{
		"as_of": "2026-04-23",
		"size":  float64(100),
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	result, err := executor.Execute(context.Background(), cubebox.ExecuteRequest{TenantID: "t1"}, params)
	if err != nil {
		t.Fatalf("Execute err=%v", err)
	}
	if store.capturedReq.Limit != 100 || store.capturedReq.Offset != 0 {
		t.Fatalf("limit=%d offset=%d", store.capturedReq.Limit, store.capturedReq.Offset)
	}
	if got := result.Payload["page"]; got != float64(1) {
		t.Fatalf("page=%v", got)
	}
	if got := result.Payload["size"]; got != float64(100) {
		t.Fatalf("size=%v", got)
	}
}

func TestCubeBoxOrgUnitListExecutorTreatsPageOneAsFirstPage(t *testing.T) {
	store := &orgUnitListPageReaderStore{
		resolveOrgCodeStore: &resolveOrgCodeStore{},
		items: []orgUnitListItem{
			{OrgCode: "1002", Name: "上海销售组", Status: "active"},
		},
		total: 12,
	}
	executor := cubeBoxOrgUnitListExecutor{store: store}
	params, err := executor.ValidateParams(map[string]any{
		"as_of": "2026-04-23",
		"page":  float64(1),
		"size":  float64(100),
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	result, err := executor.Execute(context.Background(), cubebox.ExecuteRequest{TenantID: "t1"}, params)
	if err != nil {
		t.Fatalf("Execute err=%v", err)
	}
	if store.capturedReq.Limit != 100 || store.capturedReq.Offset != 0 {
		t.Fatalf("limit=%d offset=%d", store.capturedReq.Limit, store.capturedReq.Offset)
	}
	if got := result.Payload["page"]; got != float64(1) {
		t.Fatalf("page=%v", got)
	}
}

func TestCubeBoxOrgUnitListExecutorIgnoresBlankParentOrgCode(t *testing.T) {
	executor := cubeBoxOrgUnitListExecutor{}
	params, err := executor.ValidateParams(map[string]any{
		"as_of":           "2026-04-23",
		"parent_org_code": "   ",
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	if _, ok := params["parent_org_code"]; ok {
		t.Fatal("expected blank parent_org_code to be ignored")
	}
}

func TestCubeBoxOrgUnitSearchExecutor(t *testing.T) {
	store := &resolveOrgCodeStore{
		searchNodeResult: OrgUnitSearchResult{
			TargetOrgCode:   "1001",
			TargetName:      "华东销售中心",
			PathOrgNodeKeys: []string{"10000000", "10000001"},
			TreeAsOf:        "2026-04-23",
		},
		searchCandidates: []OrgUnitSearchCandidate{
			{OrgNodeKey: "10000001", OrgCode: "1001", Name: "华东销售中心", Status: "active"},
		},
		resolveCodes: map[int]string{
			10000000: "0001",
			10000001: "1001",
		},
	}
	executor := cubeBoxOrgUnitSearchExecutor{store: store}
	params, err := executor.ValidateParams(map[string]any{
		"query": "华东",
		"as_of": "2026-04-23",
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	result, err := executor.Execute(context.Background(), cubebox.ExecuteRequest{TenantID: "t1"}, params)
	if err != nil {
		t.Fatalf("Execute err=%v", err)
	}
	pathCodes, ok := result.Payload["path_org_codes"].([]any)
	if !ok {
		t.Fatalf("path_org_codes=%T", result.Payload["path_org_codes"])
	}
	if len(pathCodes) != 2 || pathCodes[0] != "0001" || pathCodes[1] != "1001" {
		t.Fatalf("path_org_codes=%v", pathCodes)
	}
}

func TestCubeBoxOrgUnitSearchExecutorReturnsClarificationWhenSearchIsAmbiguous(t *testing.T) {
	store := &resolveOrgCodeStore{
		searchCandidates: []OrgUnitSearchCandidate{
			{OrgNodeKey: "10000001", OrgCode: "1001", Name: "华东销售中心", Status: "active"},
			{OrgNodeKey: "10000002", OrgCode: "1002", Name: "华东运营中心", Status: "disabled"},
		},
	}
	executor := cubeBoxOrgUnitSearchExecutor{store: store}
	params, err := executor.ValidateParams(map[string]any{
		"query": "华东",
		"as_of": "2026-04-23",
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	_, err = executor.Execute(context.Background(), cubebox.ExecuteRequest{TenantID: "t1"}, params)
	var ambiguous *orgUnitSearchAmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected ambiguous search error, got %v", err)
	}
	candidates := ambiguous.QueryCandidates()
	if len(candidates) != 2 {
		t.Fatalf("expected query candidates, got %#v", candidates)
	}
	if candidates[0].EntityKey != "1001" || candidates[1].EntityKey != "1002" {
		t.Fatalf("unexpected candidate entity keys=%#v", candidates)
	}
	if candidates[1].Status != "disabled" {
		t.Fatalf("expected disabled candidate hint in structured candidate, got %#v", candidates[1])
	}
}

func TestCubeBoxOrgUnitAuditExecutor(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID: 1001,
		auditEvents: []OrgUnitNodeAuditEvent{
			{
				EventID:       1,
				EventUUID:     "evt-1",
				EventType:     "rename",
				EffectiveDate: "2026-04-20",
				TxTime:        time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC),
				RequestID:     "req-1",
			},
			{
				EventID:       2,
				EventUUID:     "evt-2",
				EventType:     "move",
				EffectiveDate: "2026-04-21",
				TxTime:        time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC),
				RequestID:     "req-2",
			},
			{
				EventID:       3,
				EventUUID:     "evt-3",
				EventType:     "disable",
				EffectiveDate: "2026-04-22",
				TxTime:        time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
				RequestID:     "req-3",
			},
		},
	}
	executor := cubeBoxOrgUnitAuditExecutor{store: store}
	params, err := executor.ValidateParams(map[string]any{
		"org_code": "1001",
		"limit":    float64(2),
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	result, err := executor.Execute(context.Background(), cubebox.ExecuteRequest{TenantID: "t1"}, params)
	if err != nil {
		t.Fatalf("Execute err=%v", err)
	}
	if got := result.Payload["has_more"]; got != true {
		t.Fatalf("has_more=%v", got)
	}
	events, ok := result.Payload["events"].([]any)
	if !ok || len(events) != 2 {
		t.Fatalf("events=%T len=%d", result.Payload["events"], len(events))
	}
}

func TestCubeBoxOrgUnitAuditExecutorDefaultsLimit(t *testing.T) {
	executor := cubeBoxOrgUnitAuditExecutor{}
	params, err := executor.ValidateParams(map[string]any{"org_code": "1001"})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	if params["limit"] != orgNodeAuditPageSize {
		t.Fatalf("limit=%v", params["limit"])
	}
}

func TestCubeBoxOrgUnitExecutorsRejectInvalidParams(t *testing.T) {
	details := cubeBoxOrgUnitDetailsExecutor{}
	if _, err := details.ValidateParams(map[string]any{"org_code": "1001"}); err == nil {
		t.Fatal("expected as_of error")
	}

	list := cubeBoxOrgUnitListExecutor{}
	if _, err := list.ValidateParams(map[string]any{"as_of": "2026-04-23", "size": float64(0)}); err == nil {
		t.Fatal("expected size error")
	}
	if _, err := list.ValidateParams(map[string]any{"as_of": "2026-04-23", "status": "inactive"}); err == nil {
		t.Fatal("expected canonical status error")
	}

	search := cubeBoxOrgUnitSearchExecutor{}
	if _, err := search.ValidateParams(map[string]any{"query": "", "as_of": "2026-04-23"}); err == nil {
		t.Fatal("expected query error")
	}

	audit := cubeBoxOrgUnitAuditExecutor{}
	if _, err := audit.ValidateParams(map[string]any{"org_code": "1001", "limit": float64(1.5)}); err == nil {
		t.Fatal("expected limit error")
	}
}

func TestCubeBoxOrgUnitSearchExecutorPropagatesResolverError(t *testing.T) {
	store := &resolveOrgCodeStore{
		searchNodeResult: OrgUnitSearchResult{
			PathOrgNodeKeys: []string{"10000001"},
		},
		resolveCodesErr: errors.New("boom"),
	}
	executor := cubeBoxOrgUnitSearchExecutor{store: store}
	params, err := executor.ValidateParams(map[string]any{
		"query": "华东",
		"as_of": "2026-04-23",
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	_, err = executor.Execute(context.Background(), cubebox.ExecuteRequest{TenantID: "t1"}, params)
	if err == nil {
		t.Fatal("expected resolver error")
	}
}

func TestCubeBoxOrgUnitDetailsExecutorPreservesExtFieldPayloadShape(t *testing.T) {
	label := "简称"
	store := &orgUnitDetailsExtStoreStub{
		resolveOrgCodeStore: &resolveOrgCodeStore{
			resolveID: 1001,
			getNodeDetails: OrgUnitNodeDetails{
				OrgNodeKey: "10000001",
				OrgCode:    "1001",
				Name:       "华东销售中心",
				Status:     "active",
			},
		},
		cfgs: []orgUnitTenantFieldConfig{
			{FieldKey: "short_name", DisplayLabel: &label, PhysicalCol: "ext_str_01", ValueType: "text", DataSourceType: "PLAIN"},
		},
		snapshot: orgUnitVersionExtSnapshot{
			VersionValues: map[string]any{
				"ext_str_01": "华东",
			},
		},
	}
	executor := cubeBoxOrgUnitDetailsExecutor{store: store}
	params, err := executor.ValidateParams(map[string]any{
		"org_code": "1001",
		"as_of":    "2026-04-23",
	})
	if err != nil {
		t.Fatalf("ValidateParams err=%v", err)
	}
	result, err := executor.Execute(context.Background(), cubebox.ExecuteRequest{TenantID: "t1"}, params)
	if err != nil {
		t.Fatalf("Execute err=%v", err)
	}
	raw, err := json.Marshal(result.Payload["ext_fields"])
	if err != nil {
		t.Fatalf("Marshal err=%v", err)
	}
	if len(raw) == 0 {
		t.Fatal("empty ext_fields payload")
	}
}
