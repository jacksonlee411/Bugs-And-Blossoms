package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type assistantSearchErrStore struct {
	*orgUnitMemoryStore
}

func (s assistantSearchErrStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return nil, errors.New("search failed")
}

type assistantDetailsErrStore struct {
	*orgUnitMemoryStore
}

func (s assistantDetailsErrStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	return OrgUnitNodeDetails{}, errors.New("details failed")
}

type assistantBlankCodeStore struct {
	*orgUnitMemoryStore
}

func (s assistantBlankCodeStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return []OrgUnitSearchCandidate{{OrgID: 42, Name: "鲜花组织", Status: "active"}}, nil
}

func (s assistantBlankCodeStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	return OrgUnitNodeDetails{FullNamePath: "总公司/鲜花组织"}, nil
}

type assistantNoCandidateStore struct {
	*orgUnitMemoryStore
}

func (s assistantNoCandidateStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return []OrgUnitSearchCandidate{}, nil
}

type assistantEmptyPathDetailsStore struct {
	*orgUnitMemoryStore
}

func (s assistantEmptyPathDetailsStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return []OrgUnitSearchCandidate{{OrgID: 7, OrgCode: "FLOWER-A", Name: "鲜花组织", Status: "active"}}, nil
}

func (s assistantEmptyPathDetailsStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	return OrgUnitNodeDetails{FullNamePath: "   "}, nil
}

type assistantWriteServiceErrorStub struct{}

func (assistantWriteServiceErrorStub) Write(context.Context, string, orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
	return orgunitservices.OrgUnitWriteResult{}, errors.New("write failed")
}

type assistantWriteServiceFieldPolicyMissingStub struct{}

func (assistantWriteServiceFieldPolicyMissingStub) Write(context.Context, string, orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
	return orgunitservices.OrgUnitWriteResult{}, errors.New(orgUnitErrFieldPolicyMissing)
}

func (assistantWriteServiceFieldPolicyMissingStub) Create(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceFieldPolicyMissingStub) Rename(context.Context, string, orgunitservices.RenameOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceFieldPolicyMissingStub) Move(context.Context, string, orgunitservices.MoveOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceFieldPolicyMissingStub) Disable(context.Context, string, orgunitservices.DisableOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceFieldPolicyMissingStub) Enable(context.Context, string, orgunitservices.EnableOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceFieldPolicyMissingStub) SetBusinessUnit(context.Context, string, orgunitservices.SetBusinessUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceFieldPolicyMissingStub) Correct(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceFieldPolicyMissingStub) CorrectStatus(context.Context, string, orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceFieldPolicyMissingStub) RescindRecord(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceFieldPolicyMissingStub) RescindOrg(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceErrorStub) Create(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceErrorStub) Rename(context.Context, string, orgunitservices.RenameOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceErrorStub) Move(context.Context, string, orgunitservices.MoveOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceErrorStub) Disable(context.Context, string, orgunitservices.DisableOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceErrorStub) Enable(context.Context, string, orgunitservices.EnableOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceErrorStub) SetBusinessUnit(context.Context, string, orgunitservices.SetBusinessUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceErrorStub) Correct(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceErrorStub) CorrectStatus(context.Context, string, orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceErrorStub) RescindRecord(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceErrorStub) RescindOrg(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func assistantReqWithContext(method string, path string, body string, withTenantCtx bool, withPrincipalCtx bool) *http.Request {
	req := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
	ctx := req.Context()
	if withTenantCtx {
		ctx = withTenant(ctx, Tenant{ID: "tenant-1"})
	}
	if withPrincipalCtx {
		ctx = withPrincipal(ctx, Principal{ID: "actor-1", RoleSlug: "tenant-admin"})
	}
	return req.WithContext(ctx)
}

func assistantDecodeErrCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var out routing.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal error envelope: %v body=%s", err, rec.Body.String())
	}
	return out.Code
}

func TestAssistantConversationHandlers_CoverageMatrix(t *testing.T) {
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})

	t.Run("create conversation handler branches", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodDelete, "/internal/assistant/conversations", "", true, true), svc)
		if rec.Code != http.StatusMethodNotAllowed || assistantDecodeErrCode(t, rec) != "method_not_allowed" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations?page_size=2", "", true, true), svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var listResp assistantConversationListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
			t.Fatalf("unmarshal list response: %v", err)
		}
		if listResp.Items == nil {
			t.Fatal("items should not be nil")
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations?page_size=x", "", true, true), svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "invalid_request" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations?cursor=not-valid", "", true, true), svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "assistant_conversation_cursor_invalid" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations", "", true, true), nil)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_service_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations", "", false, true), svc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "tenant_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations", "", true, false), svc)
		if rec.Code != http.StatusUnauthorized || assistantDecodeErrCode(t, rec) != "unauthorized" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		req := assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations", "{", true, true)
		req.Header.Set("Transfer-Encoding", "chunked")
		rec = httptest.NewRecorder()
		handleAssistantConversationsAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "bad_json" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations", "", true, true), svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var conv assistantConversation
		if err := json.Unmarshal(rec.Body.Bytes(), &conv); err != nil {
			t.Fatalf("unmarshal conversation: %v", err)
		}
		if conv.ConversationID == "" {
			t.Fatal("conversation id is empty")
		}
	})

	t.Run("conversation detail handler branches", func(t *testing.T) {
		principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
		conv := svc.createConversation("tenant-1", principal)

		rec := httptest.NewRecorder()
		handleAssistantConversationDetailAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID, "", true, true), svc)
		if rec.Code != http.StatusMethodNotAllowed || assistantDecodeErrCode(t, rec) != "method_not_allowed" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations/"+conv.ConversationID, "", true, true), nil)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_service_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations/"+conv.ConversationID, "", false, true), svc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "tenant_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations/"+conv.ConversationID, "", true, false), svc)
		if rec.Code != http.StatusUnauthorized || assistantDecodeErrCode(t, rec) != "unauthorized" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations", "", true, true), svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "invalid_request" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations/missing", "", true, true), svc)
		if rec.Code != http.StatusNotFound || assistantDecodeErrCode(t, rec) != "conversation_not_found" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		forbiddenReq := assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations/"+conv.ConversationID, "", true, true)
		forbiddenReq = forbiddenReq.WithContext(withPrincipal(forbiddenReq.Context(), Principal{ID: "actor-x", RoleSlug: "tenant-admin"}))
		rec = httptest.NewRecorder()
		handleAssistantConversationDetailAPI(rec, forbiddenReq, svc)
		if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != "forbidden" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		svc.mu.Lock()
		svc.byID["conv_corrupted"] = nil
		svc.mu.Unlock()
		rec = httptest.NewRecorder()
		handleAssistantConversationDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations/conv_corrupted", "", true, true), svc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_conversation_load_failed" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations/"+conv.ConversationID, "", true, true), svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("turn create handler branches", func(t *testing.T) {
		principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
		conv := svc.createConversation("tenant-1", principal)
		path := "/internal/assistant/conversations/" + conv.ConversationID + "/turns"

		rec := httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodGet, path, "", true, true), svc)
		if rec.Code != http.StatusMethodNotAllowed || assistantDecodeErrCode(t, rec) != "method_not_allowed" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, path, "{}", true, true), nil)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_service_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, path, "{}", false, true), svc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "tenant_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, path, "{}", true, false), svc)
		if rec.Code != http.StatusUnauthorized || assistantDecodeErrCode(t, rec) != "unauthorized" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/x/turn", "{}", true, true), svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "invalid_request" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, path, "{", true, true), svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "bad_json" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, path, `{"user_input":"   "}`, true, true), svc)
		if rec.Code != http.StatusUnprocessableEntity || assistantDecodeErrCode(t, rec) != "invalid_request" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/missing/turns", `{"user_input":"计划"}`, true, true), svc)
		if rec.Code != http.StatusNotFound || assistantDecodeErrCode(t, rec) != "conversation_not_found" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		t.Setenv("OPENAI_API_KEY", "dummy")
		schemaErrSvc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		schemaErrSvc.modelGateway = &assistantModelGateway{
			config: assistantModelConfig{
				ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
				Providers: []assistantModelProviderConfig{
					{Name: "openai", Enabled: true, Model: "gpt-5-codex", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1000, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
				},
			},
			adapters: map[string]assistantProviderAdapter{
				"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
					return []byte(`{"action":"create_orgunit","extra":1}`), nil
				}),
			},
		}
		schemaErrConv := schemaErrSvc.createConversation("tenant-1", principal)
		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+schemaErrConv.ConversationID+"/turns", `{"user_input":"在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"}`, true, true), schemaErrSvc)
		code := assistantDecodeErrCode(t, rec)
		if !((rec.Code == http.StatusUnprocessableEntity && code == "ai_plan_schema_constrained_decode_failed") || (rec.Code == http.StatusInternalServerError && code == "assistant_turn_create_failed")) {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, code, rec.Body.String())
		}

		forbiddenReq := assistantReqWithContext(http.MethodPost, path, `{"user_input":"计划"}`, true, true)
		forbiddenReq = forbiddenReq.WithContext(withPrincipal(forbiddenReq.Context(), Principal{ID: "actor-x", RoleSlug: "tenant-admin"}))
		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, forbiddenReq, svc)
		if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != "forbidden" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, path, `{"user_input":"在鲜花组织之下，新建一个名为运营部的部门"}`, true, true), svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var conversation assistantConversation
		if err := json.Unmarshal(rec.Body.Bytes(), &conversation); err != nil {
			t.Fatalf("decode conversation: %v", err)
		}
		last := latestTurn(&conversation)
		if last == nil {
			t.Fatal("expected turn created")
		}
		if got := strings.Join(last.DryRun.ValidationErrors, ","); !strings.Contains(got, "missing_effective_date") {
			t.Fatalf("expected missing_effective_date, got=%v", last.DryRun.ValidationErrors)
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, path, `{"user_input":"请执行 SELECT * FROM org_units"}`, true, true), svc)
		if rec.Code != http.StatusUnprocessableEntity || assistantDecodeErrCode(t, rec) != "ai_plan_boundary_violation" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		errSvc := newAssistantConversationService(assistantSearchErrStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}, assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		errConv := errSvc.createConversation("tenant-1", principal)
		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+errConv.ConversationID+"/turns", `{"user_input":"在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。"}`, true, true), errSvc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_turn_create_failed" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		routeErrSvc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		routeErrSvc.knowledgeRuntime = &assistantKnowledgeRuntime{
			RouteCatalogVersion:     "2026-03-11.v1",
			SnapshotDigest:          "sha256:test",
			ResolverContractVersion: "resolver_contract_v1",
			routeCatalog: assistantIntentRouteCatalog{Entries: []assistantIntentRouteEntry{{
				IntentID:  "route.bad",
				RouteKind: "bad_kind",
				Keywords:  []string{"坏"},
			}}},
		}
		routeErrConv := routeErrSvc.createConversation("tenant-1", principal)
		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+routeErrConv.ConversationID+"/turns", `{"user_input":"坏"}`, true, true), routeErrSvc)
		if rec.Code != http.StatusUnprocessableEntity || assistantDecodeErrCode(t, rec) != errAssistantRouteRuntimeInvalid.Error() {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}

		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, path, `{"user_input":"仅生成计划"}`, true, true), svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestAssistantTurnActionHandler_CoverageMatrix(t *testing.T) {
	store := newOrgUnitMemoryStore()
	tenantID := "tenant-1"
	if _, err := store.CreateNodeCurrent(context.Background(), tenantID, "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateNodeCurrent(context.Background(), tenantID, "2026-01-01", "FLOWER-B", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	conv := svc.createConversation(tenantID, principal)
	conversation, err := svc.createTurn(context.Background(), tenantID, principal, conv.ConversationID, "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。")
	if err != nil {
		t.Fatal(err)
	}
	turnID := conversation.Turns[0].TurnID

	baseConfirmPath := "/internal/assistant/conversations/" + conv.ConversationID + "/turns/" + turnID + ":confirm"
	baseCommitPath := "/internal/assistant/conversations/" + conv.ConversationID + "/turns/" + turnID + ":commit"

	t.Run("global guard branches", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodGet, baseConfirmPath, "", true, true), svc)
		if rec.Code != http.StatusMethodNotAllowed || assistantDecodeErrCode(t, rec) != "method_not_allowed" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, baseConfirmPath, "", true, true), nil)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_service_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, baseConfirmPath, "", false, true), svc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "tenant_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, baseConfirmPath, "", true, false), svc)
		if rec.Code != http.StatusUnauthorized || assistantDecodeErrCode(t, rec) != "unauthorized" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/x/turns/y", "", true, true), svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "invalid_request" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}
	})

	t.Run("confirm branches", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, baseConfirmPath, "{", true, true), svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "bad_json" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/missing/turns/turn-1:confirm", `{}`, true, true), svc)
		if rec.Code != http.StatusNotFound || assistantDecodeErrCode(t, rec) != "conversation_not_found" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		corruptedSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		corruptedSvc.mu.Lock()
		corruptedSvc.byID["conv-corrupted"] = nil
		corruptedSvc.mu.Unlock()
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/conv-corrupted/turns/turn-1:confirm", `{}`, true, true), corruptedSvc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_turn_confirm_failed" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		forbiddenReq := assistantReqWithContext(http.MethodPost, baseConfirmPath, `{}`, true, true)
		forbiddenReq = forbiddenReq.WithContext(withPrincipal(forbiddenReq.Context(), Principal{ID: "actor-x", RoleSlug: "tenant-admin"}))
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, forbiddenReq, svc)
		if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != "forbidden" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns/missing:confirm", `{}`, true, true), svc)
		if rec.Code != http.StatusNotFound || assistantDecodeErrCode(t, rec) != "conversation_turn_not_found" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, baseConfirmPath, `{}`, true, true), svc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "conversation_confirmation_required" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, baseConfirmPath, `{"candidate_id":"bad"}`, true, true), svc)
		if rec.Code != http.StatusUnprocessableEntity || assistantDecodeErrCode(t, rec) != "assistant_candidate_not_found" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		nonBusinessSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		nonBusinessConv := nonBusinessSvc.createConversation(tenantID, principal)
		nonBusinessConversation, nonBusinessErr := nonBusinessSvc.createTurn(context.Background(), tenantID, principal, nonBusinessConv.ConversationID, "系统有哪些功能")
		if nonBusinessErr != nil {
			t.Fatalf("create non business turn: %v", nonBusinessErr)
		}
		nonBusinessTurn := nonBusinessConversation.Turns[0]
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+nonBusinessConv.ConversationID+"/turns/"+nonBusinessTurn.TurnID+":confirm", `{}`, true, true), nonBusinessSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != errAssistantRouteNonBusinessBlocked.Error() {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}

		candidateID := conversation.Turns[0].Candidates[0].CandidateID
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, baseConfirmPath, `{"candidate_id":"`+candidateID+`"}`, true, true), svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, baseConfirmPath, `{"candidate_id":"`+candidateID+`"}`, true, true), svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		candidateID2 := conversation.Turns[0].Candidates[1].CandidateID
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, baseConfirmPath, `{"candidate_id":"`+candidateID2+`"}`, true, true), svc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "conversation_state_invalid" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		expiredSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		expiredConv := expiredSvc.createConversation(tenantID, principal)
		expiredConversation, createErr := expiredSvc.createTurn(context.Background(), tenantID, principal, expiredConv.ConversationID, "在鲜花组织之下，新建一个名为风控部的部门，成立日期是2026-01-01")
		if createErr != nil {
			t.Fatal(createErr)
		}
		expiredTurn := expiredConversation.Turns[0]
		expiredSvc.mu.Lock()
		expiredSvc.byID[expiredConv.ConversationID].Turns[0].Plan.ExpiresAt = time.Now().UTC().Add(-1 * time.Minute).Format(time.RFC3339)
		expiredSvc.mu.Unlock()
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+expiredConv.ConversationID+"/turns/"+expiredTurn.TurnID+":confirm", `{}`, true, true), expiredSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "conversation_confirmation_expired" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}
	})

	t.Run("commit branches", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, baseCommitPath, `{}`, true, true), svc)
		if rec.Code != http.StatusServiceUnavailable || assistantDecodeErrCode(t, rec) != "assistant_task_workflow_unavailable" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns/"+turnID+":noop", `{}`, true, true), svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "invalid_request" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}
	})

	t.Run("gate mapping keeps http reason and turn error aligned", func(t *testing.T) {
		originalAuthorizer := assistantLoadAuthorizerFn
		assistantLoadAuthorizerFn = func() (authorizer, error) {
			return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
		}
		defer func() { assistantLoadAuthorizerFn = originalAuthorizer }()

		originalDefinitions := capabilityDefinitionByKey
		defer func() { capabilityDefinitionByKey = originalDefinitions }()
		createSpec, ok := assistantLookupDefaultActionSpec(assistantIntentCreateOrgUnit)
		if !ok {
			t.Fatal("missing create action spec")
		}
		capabilityDefinitionByKey = map[string]capabilityDefinition{
			createSpec.CapabilityKey: {
				CapabilityKey:   createSpec.CapabilityKey,
				Status:          routeCapabilityStatusActive,
				ActivationState: routeCapabilityStatusActive,
			},
		}

		riskStore := newOrgUnitMemoryStore()
		if _, err := riskStore.CreateNodeCurrent(context.Background(), tenantID, "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatal(err)
		}
		riskSvc := newAssistantConversationService(riskStore, assistantWriteServiceStub{store: riskStore})
		riskSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{
			assistantIntentCreateOrgUnit: {
				ID:            assistantIntentCreateOrgUnit,
				Version:       "v1",
				CapabilityKey: createSpec.CapabilityKey,
				Security: assistantActionSecuritySpec{
					AuthObject:     createSpec.Security.AuthObject,
					AuthAction:     createSpec.Security.AuthAction,
					RiskTier:       "extreme",
					RequiredChecks: createSpec.Security.RequiredChecks,
				},
				Handler: assistantActionHandlerSpec{CommitAdapterKey: createSpec.Handler.CommitAdapterKey},
			},
		}}
		riskPrincipal := Principal{ID: "actor-risk", RoleSlug: "tenant-admin"}
		riskConversation := riskSvc.createConversation(tenantID, riskPrincipal)
		riskTurn := &assistantTurn{
			TurnID:              "turn-risk-gate",
			UserInput:           "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01",
			State:               assistantStateValidated,
			RequestID:           "req-risk-gate",
			TraceID:             "trace-risk-gate",
			PolicyVersion:       capabilityPolicyVersionBaseline,
			CompositionVersion:  capabilityPolicyVersionBaseline,
			MappingVersion:      capabilityPolicyVersionBaseline,
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates:          []assistantCandidate{{CandidateID: "FLOWER-A", CandidateCode: "FLOWER-A", Name: "鲜花组织"}},
			ResolvedCandidateID: "FLOWER-A",
			DryRun:              assistantDryRunResult{Explain: "ok"},
			CreatedAt:           time.Now().UTC(),
			UpdatedAt:           time.Now().UTC(),
		}
		assistantRefreshTurnDerivedFields(riskTurn)
		riskSvc.mu.Lock()
		riskSvc.byID[riskConversation.ConversationID].Turns = append(riskSvc.byID[riskConversation.ConversationID].Turns, riskTurn)
		riskSvc.mu.Unlock()

		req := assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+riskConversation.ConversationID+"/turns/"+riskTurn.TurnID+":confirm", `{}`, true, true)
		req = req.WithContext(withPrincipal(req.Context(), riskPrincipal))
		rec := httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, req, riskSvc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != errAssistantActionRiskGateDenied.Error() {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}

		riskSvc.mu.RLock()
		storedConversation := riskSvc.byID[riskConversation.ConversationID]
		if storedConversation == nil || len(storedConversation.Turns) == 0 || len(storedConversation.Transitions) == 0 {
			riskSvc.mu.RUnlock()
			t.Fatalf("missing stored conversation after gate reject: %+v", storedConversation)
		}
		storedTurn := storedConversation.Turns[len(storedConversation.Turns)-1]
		lastTransition := storedConversation.Transitions[len(storedConversation.Transitions)-1]
		riskSvc.mu.RUnlock()
		if storedTurn.ErrorCode != errAssistantActionRiskGateDenied.Error() {
			t.Fatalf("turn.error_code=%s", storedTurn.ErrorCode)
		}
		if lastTransition.ReasonCode != "risk_tier_invalid" || lastTransition.TurnAction != "confirm" {
			t.Fatalf("transition=%+v", lastTransition)
		}
	})
}

func TestAssistantServiceHelpersAndUtilities(t *testing.T) {
	t.Run("conversation methods and lookup", func(t *testing.T) {
		svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
		conv := svc.createConversation("tenant-1", principal)

		if _, err := svc.getConversation("tenant-1", "actor-1", "missing"); !errors.Is(err, errAssistantConversationNotFound) {
			t.Fatalf("want conversation not found, got %v", err)
		}
		if _, err := svc.getConversation("tenant-x", "actor-1", conv.ConversationID); !errors.Is(err, errAssistantTenantMismatch) {
			t.Fatalf("want tenant mismatch, got %v", err)
		}
		svc.mu.Lock()
		svc.byID["conv-corrupted"] = nil
		svc.mu.Unlock()
		if _, err := svc.getConversation("tenant-1", "actor-1", "conv-corrupted"); !errors.Is(err, errAssistantConversationCorrupted) {
			t.Fatalf("want conversation corrupted, got %v", err)
		}

		created, err := svc.createTurn(context.Background(), "tenant-1", principal, conv.ConversationID, "仅生成计划")
		if err != nil {
			t.Fatalf("create turn failed: %v", err)
		}
		turnID := created.Turns[0].TurnID

		if _, _, err := svc.lookupMutableTurn("tenant-1", "actor-1", "missing", turnID); !errors.Is(err, errAssistantConversationNotFound) {
			t.Fatalf("want not found, got %v", err)
		}
		if _, _, err := svc.lookupMutableTurn("tenant-x", "actor-1", conv.ConversationID, turnID); !errors.Is(err, errAssistantTenantMismatch) {
			t.Fatalf("want tenant mismatch, got %v", err)
		}
		if _, _, err := svc.lookupMutableTurn("tenant-1", "actor-1", conv.ConversationID, "missing"); !errors.Is(err, errAssistantTurnNotFound) {
			t.Fatalf("want turn not found, got %v", err)
		}
	})

	t.Run("resolve candidates variants", func(t *testing.T) {
		svc := newAssistantConversationService(nil, nil)
		out, err := svc.resolveCandidates(context.Background(), "tenant-1", "鲜花组织", "2026-01-01")
		if err != nil || out != nil {
			t.Fatalf("want nil,nil got %+v err=%v", out, err)
		}

		errSvc := newAssistantConversationService(assistantSearchErrStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}, nil)
		if _, err := errSvc.resolveCandidates(context.Background(), "tenant-1", "鲜花组织", "2026-01-01"); err == nil {
			t.Fatal("expected search error")
		}

		detailErrStore := assistantDetailsErrStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}
		if _, err := detailErrStore.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatal(err)
		}
		detailSvc := newAssistantConversationService(detailErrStore, nil)
		candidates, err := detailSvc.resolveCandidates(context.Background(), "tenant-1", "鲜花组织", "2026-01-01")
		if err != nil {
			t.Fatalf("resolve candidates failed: %v", err)
		}
		if len(candidates) == 0 || candidates[0].Path == "" {
			t.Fatalf("unexpected candidates: %+v", candidates)
		}

		blankSvc := newAssistantConversationService(assistantBlankCodeStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}, nil)
		candidates, err = blankSvc.resolveCandidates(context.Background(), "tenant-1", "鲜花组织", "2026-01-01")
		if err != nil {
			t.Fatalf("resolve candidates failed: %v", err)
		}
		if len(candidates) != 1 || candidates[0].CandidateID != "42" {
			t.Fatalf("unexpected candidate: %+v", candidates)
		}

		emptyPathSvc := newAssistantConversationService(assistantEmptyPathDetailsStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}, nil)
		candidates, err = emptyPathSvc.resolveCandidates(context.Background(), "tenant-1", "鲜花组织", "2026-01-01")
		if err != nil {
			t.Fatalf("resolve candidates failed: %v", err)
		}
		if len(candidates) != 1 || candidates[0].Path != "鲜花组织" {
			t.Fatalf("unexpected empty-path fallback candidate: %+v", candidates)
		}
	})

	t.Run("createTurn candidate confidence branches", func(t *testing.T) {
		principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}

		zeroStore := assistantNoCandidateStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}
		zeroSvc := newAssistantConversationService(zeroStore, nil)
		zeroConv := zeroSvc.createConversation("tenant-1", principal)
		zeroConversation, err := zeroSvc.createTurn(context.Background(), "tenant-1", principal, zeroConv.ConversationID, "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01")
		if err != nil {
			t.Fatalf("createTurn zero candidate failed: %v", err)
		}
		if got := zeroConversation.Turns[0].Confidence; got != 0.3 {
			t.Fatalf("zero candidate confidence=%v", got)
		}

		oneStore := newOrgUnitMemoryStore()
		if _, err := oneStore.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatal(err)
		}
		oneSvc := newAssistantConversationService(oneStore, nil)
		oneConv := oneSvc.createConversation("tenant-1", principal)
		oneConversation, err := oneSvc.createTurn(context.Background(), "tenant-1", principal, oneConv.ConversationID, "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01")
		if err != nil {
			t.Fatalf("createTurn one candidate failed: %v", err)
		}
		if got := oneConversation.Turns[0].Confidence; got != 0.95 {
			t.Fatalf("one candidate confidence=%v", got)
		}
		if oneConversation.Turns[0].ResolutionSource != assistantResolutionAuto {
			t.Fatalf("resolution source=%s", oneConversation.Turns[0].ResolutionSource)
		}
	})

	t.Run("intent and helper functions", func(t *testing.T) {
		if got := assistantRiskTierForIntent(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}); got != "high" {
			t.Fatalf("risk tier=%s", got)
		}
		if got := assistantRiskTierForIntent(assistantIntentSpec{Action: "plan_only"}); got != "low" {
			t.Fatalf("risk tier=%s", got)
		}

		intent := assistantExtractIntent("在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日")
		if intent.ParentRefText != "鲜花组织" || intent.EntityName != "运营部" || intent.EffectiveDate != "2026-01-01" {
			t.Fatalf("unexpected intent: %+v", intent)
		}
		intentISO := assistantExtractIntent("新建一个名为财务部的部门，成立日期是2026-01-02")
		if intentISO.EffectiveDate != "2026-01-02" {
			t.Fatalf("unexpected iso date: %+v", intentISO)
		}
		if _, err := assistantDecodeIntent("在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"); err != nil {
			t.Fatalf("decode intent failed: %v", err)
		}
		if _, err := assistantDecodeIntent("在鲜花组织之下，新建一个名为运营部的部门"); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
			t.Fatalf("want schema decode failed, got %v", err)
		}
		if _, err := assistantDecodeIntent("请执行 SELECT * FROM org_units"); !errors.Is(err, errAssistantPlanBoundaryViolation) {
			t.Fatalf("want boundary violation, got %v", err)
		}
		originalDefs := capabilityDefinitionByKey
		capabilityDefinitionByKey = map[string]capabilityDefinition{}
		if _, err := assistantDecodeIntent("仅生成计划"); !errors.Is(err, errAssistantPlanBoundaryViolation) {
			t.Fatalf("want boundary violation for unknown capability, got %v", err)
		}
		capabilityDefinitionByKey = originalDefs
		if !assistantBoundaryViolationDetected("drop table org_unit_nodes") {
			t.Fatal("expected boundary violation detection")
		}
		if assistantIntentSchemaInvalid(assistantIntentSpec{Action: "plan_only"}) {
			t.Fatal("plan_only should not trigger schema validation")
		}
		if !assistantIntentSchemaInvalid(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "", EntityName: "运营部", EffectiveDate: "2026-01-01"}) {
			t.Fatal("missing parent should be invalid")
		}
		if !assistantIntentSchemaInvalid(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "", EffectiveDate: "2026-01-01"}) {
			t.Fatal("missing entity should be invalid")
		}
		if !assistantIntentSchemaInvalid(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: ""}) {
			t.Fatal("missing effective date should be invalid")
		}
		if !assistantIntentSchemaInvalid(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026/01/01"}) {
			t.Fatal("invalid date format should be invalid")
		}
		if assistantIntentSchemaInvalid(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}) {
			t.Fatal("valid create intent should pass schema")
		}
		if assistantTurnVersionDrifted(nil) {
			t.Fatal("nil turn should not drift")
		}
		if assistantTurnVersionDrifted(&assistantTurn{
			Plan:               assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			PolicyVersion:      capabilityPolicyVersionBaseline,
			CompositionVersion: capabilityPolicyVersionBaseline,
			MappingVersion:     capabilityPolicyVersionBaseline,
		}) {
			t.Fatal("baseline turn should not drift")
		}
		if !assistantTurnVersionDrifted(&assistantTurn{
			Plan:               assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			PolicyVersion:      capabilityPolicyVersionBaseline,
			CompositionVersion: "1999-01-01",
			MappingVersion:     capabilityPolicyVersionBaseline,
		}) {
			t.Fatal("composition drift should be detected")
		}
		if !assistantTurnVersionDrifted(&assistantTurn{
			Plan:               assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			PolicyVersion:      capabilityPolicyVersionBaseline,
			CompositionVersion: capabilityPolicyVersionBaseline,
			MappingVersion:     "1999-01-01",
		}) {
			t.Fatal("mapping drift should be detected")
		}
		planOnly := assistantExtractIntent("hello")
		if planOnly.Action != "plan_only" {
			t.Fatalf("unexpected plan only intent: %+v", planOnly)
		}

		candidates := []assistantCandidate{{CandidateID: "A", CandidateCode: "FLOWER-A"}, {CandidateID: "B", CandidateCode: "FLOWER-B"}}
		if !assistantCandidateExists(candidates, "B") || assistantCandidateExists(candidates, "X") {
			t.Fatal("candidate exists mismatch")
		}
		if found, ok := assistantFindCandidate(candidates, "A"); !ok || found.CandidateCode != "FLOWER-A" {
			t.Fatalf("unexpected candidate find result: %+v %v", found, ok)
		}
		if _, ok := assistantFindCandidate(candidates, "X"); ok {
			t.Fatal("candidate should not be found")
		}

		if code := assistantGeneratedOrgCode("turn_abc-def"); !strings.HasPrefix(code, "AI") {
			t.Fatalf("org code=%s", code)
		}
		if code := assistantGeneratedOrgCode(""); code != "AIAIDEFAULT" {
			t.Fatalf("empty org code result=%s", code)
		}

		if convID, ok := extractConversationIDFromPath("/internal/assistant/conversations/conv-1"); !ok || convID != "conv-1" {
			t.Fatalf("extract conversation id failed: %s %v", convID, ok)
		}
		if _, ok := extractConversationIDFromPath("/internal/assistant/conversations"); ok {
			t.Fatal("expected invalid conversation path")
		}
		if _, ok := extractConversationIDFromPath("/wrong/assistant/conversations/conv-1"); ok {
			t.Fatal("expected invalid namespace conversation path")
		}
		if _, ok := extractConversationIDFromPath("/internal/assistant/conversations/ "); ok {
			t.Fatal("expected invalid empty conversation id")
		}
		if convID, ok := extractConversationTurnsPathConversationID("/internal/assistant/conversations/conv-1/turns"); !ok || convID != "conv-1" {
			t.Fatalf("extract turns conversation id failed: %s %v", convID, ok)
		}
		if _, ok := extractConversationTurnsPathConversationID("/internal/assistant/conversations/conv-1/turn"); ok {
			t.Fatal("expected invalid turns path")
		}
		if _, ok := extractConversationTurnsPathConversationID("/wrong/assistant/conversations/conv-1/turns"); ok {
			t.Fatal("expected invalid turns namespace")
		}
		if _, ok := extractConversationTurnsPathConversationID("/internal/assistant/conversations/ /turns"); ok {
			t.Fatal("expected invalid empty turns conversation id")
		}

		conversationID, turnID, action, ok := extractAssistantTurnActionPath("/internal/assistant/conversations/conv-1/turns/turn-1:confirm")
		if !ok || conversationID != "conv-1" || turnID != "turn-1" || action != "confirm" {
			t.Fatalf("extract turn action failed: %s %s %s %v", conversationID, turnID, action, ok)
		}
		_, _, _, ok = extractAssistantTurnActionPath("/internal/assistant/conversations/conv-1/turns/turn-1")
		if ok {
			t.Fatal("expected invalid turn action path")
		}
		_, _, _, ok = extractAssistantTurnActionPath("/internal/assistant/conversations/conv-1/turns/:confirm")
		if ok {
			t.Fatal("expected invalid empty turn id")
		}
		_, _, _, ok = extractAssistantTurnActionPath("/wrong/assistant/conversations/conv-1/turns/turn-1:confirm")
		if ok {
			t.Fatal("expected invalid namespace")
		}
		_, _, _, ok = extractAssistantTurnActionPath("/internal/assistant/conversations//turns/turn-1:confirm")
		if ok {
			t.Fatal("expected invalid empty conversation")
		}
		_, _, _, ok = extractAssistantTurnActionPath("/internal/assistant/conversations/conv-1/turns/ :confirm")
		if ok {
			t.Fatal("expected invalid blank turn id")
		}
		_, _, _, ok = extractAssistantTurnActionPath("/internal/assistant/conversations/conv-1/turns/turn-1:")
		if ok {
			t.Fatal("expected invalid empty action")
		}
		_, _, _, ok = extractAssistantTurnActionPath("/internal/assistant/conversations/conv-1/turns/turn-1")
		if ok {
			t.Fatal("expected missing action separator")
		}

		dryRun := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, candidates, "")
		if !strings.Contains(dryRun.Explain, "候选") {
			t.Fatalf("unexpected dryrun explain=%s", dryRun.Explain)
		}
		resolvedDryRun := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, candidates[:1], "A")
		if len(resolvedDryRun.Diff) < 3 {
			t.Fatalf("unexpected dryrun diff=%+v", resolvedDryRun.Diff)
		}
		formatDryRun := assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026/01/01"}, nil, "")
		if !strings.Contains(formatDryRun.Explain, "YYYY-MM-DD") {
			t.Fatalf("unexpected format explain=%s", formatDryRun.Explain)
		}

		if got := assistantDryRunValidationExplain(nil); !strings.Contains(got, "等待确认") {
			t.Fatalf("unexpected default explain=%s", got)
		}
		if got := assistantDryRunValidationExplain([]string{"candidate_confirmation_required"}); !strings.Contains(got, "候选主键") {
			t.Fatalf("unexpected candidate explain=%s", got)
		}
		if got := assistantDryRunValidationExplain([]string{"missing_entity_name"}); !strings.Contains(got, "部门名称") {
			t.Fatalf("unexpected missing entity explain=%s", got)
		}
		if got := assistantDryRunValidationExplain([]string{"unknown_code"}); !strings.Contains(got, "继续补充") {
			t.Fatalf("unexpected unknown explain=%s", got)
		}

		if got := assistantNormalizeValidationErrors([]string{"", "missing_effective_date", "missing_effective_date"}); len(got) != 1 || got[0] != "missing_effective_date" {
			t.Fatalf("unexpected normalized errors=%v", got)
		}
		if got := assistantNormalizeValidationErrors(nil); got != nil {
			t.Fatalf("expected nil normalized errors, got=%v", got)
		}

		if assistantTurnRequiresIntentClarification(nil) {
			t.Fatal("nil turn should not require clarification")
		}
		if assistantTurnRequiresIntentClarification(&assistantTurn{DryRun: assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required"}}}) {
			t.Fatal("candidate confirmation only should not require clarification")
		}
		if !assistantTurnRequiresIntentClarification(&assistantTurn{DryRun: assistantDryRunResult{ValidationErrors: []string{"invalid_effective_date_format"}}}) {
			t.Fatal("invalid date format should require clarification")
		}
	})

	t.Run("confirm and commit direct branches", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-B", "鲜花组织", "", true); err != nil {
			t.Fatal(err)
		}
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
		conv := svc.createConversation("tenant-1", principal)
		created, err := svc.createTurn(context.Background(), "tenant-1", principal, conv.ConversationID, "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		turnID := created.Turns[0].TurnID

		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, turnID, ""); !errors.Is(err, errAssistantConfirmationRequired) {
			t.Fatalf("want confirmation required, got %v", err)
		}
		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, turnID, "bad"); !errors.Is(err, errAssistantCandidateNotFound) {
			t.Fatalf("want candidate not found, got %v", err)
		}
		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, turnID, created.Turns[0].Candidates[0].CandidateID); err != nil {
			t.Fatalf("confirm failed: %v", err)
		}
		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, turnID, created.Turns[0].Candidates[0].CandidateID); err != nil {
			t.Fatalf("confirmed same candidate should be idempotent: %v", err)
		}
		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, turnID, "bad"); !errors.Is(err, errAssistantCandidateNotFound) {
			t.Fatalf("want candidate not found in confirmed state, got %v", err)
		}
		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, turnID, created.Turns[0].Candidates[1].CandidateID); !errors.Is(err, errAssistantConversationStateInvalid) {
			t.Fatalf("want state invalid for candidate rewrite, got %v", err)
		}
		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns[0].State = assistantStateCommitted
		svc.mu.Unlock()
		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, turnID, created.Turns[0].Candidates[0].CandidateID); err != nil {
			t.Fatalf("committed confirm should be idempotent: %v", err)
		}
		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns[0].State = assistantStateCanceled
		svc.mu.Unlock()
		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, turnID, created.Turns[0].Candidates[0].CandidateID); !errors.Is(err, errAssistantConversationStateInvalid) {
			t.Fatalf("want state invalid for canceled turn, got %v", err)
		}
		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns[0].State = assistantStateConfirmed
		svc.mu.Unlock()

		invalidStateTurn := &assistantTurn{TurnID: "turn-draft", State: assistantStateDraft, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}}
		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns = append(svc.byID[conv.ConversationID].Turns, invalidStateTurn)
		svc.mu.Unlock()
		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, invalidStateTurn.TurnID, created.Turns[0].Candidates[0].CandidateID); !errors.Is(err, errAssistantConfirmationRequired) {
			t.Fatalf("want confirmation required for draft, got %v", err)
		}

		expiredConfirmTurn := &assistantTurn{
			TurnID:    "turn-expired-confirm",
			State:     assistantStateValidated,
			RequestID: "req-expired-confirm",
			TraceID:   "trace-expired-confirm",
			Intent:    assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			Plan:      assistantFreezeConfirmWindow(assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), time.Now().UTC().Add(-1*time.Hour)),
			CreatedAt: time.Now().UTC().Add(-1 * time.Hour),
			UpdatedAt: time.Now().UTC().Add(-1 * time.Hour),
		}
		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns = append(svc.byID[conv.ConversationID].Turns, expiredConfirmTurn)
		svc.mu.Unlock()
		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, expiredConfirmTurn.TurnID, ""); !errors.Is(err, errAssistantConfirmationExpired) {
			t.Fatalf("want confirmation expired, got %v", err)
		}
		svc.mu.RLock()
		if got := svc.byID[conv.ConversationID].Turns[len(svc.byID[conv.ConversationID].Turns)-1].State; got != assistantStateExpired {
			svc.mu.RUnlock()
			t.Fatalf("want expired state after confirm timeout, got %s", got)
		}
		svc.mu.RUnlock()

		expiredCommitTurn := &assistantTurn{
			TurnID:    "turn-expired-commit",
			State:     assistantStateValidated,
			RequestID: "req-expired-commit",
			TraceID:   "trace-expired-commit",
			Intent:    assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			Plan:      assistantFreezeConfirmWindow(assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), time.Now().UTC().Add(-1*time.Hour)),
			CreatedAt: time.Now().UTC().Add(-1 * time.Hour),
			UpdatedAt: time.Now().UTC().Add(-1 * time.Hour),
		}
		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns = append(svc.byID[conv.ConversationID].Turns, expiredCommitTurn)
		svc.mu.Unlock()
		if _, err := assistantCommitTurnSyncForTest(svc, context.Background(), "tenant-1", principal, conv.ConversationID, expiredCommitTurn.TurnID); !errors.Is(err, errAssistantConfirmationExpired) {
			t.Fatalf("want confirmation expired on commit, got %v", err)
		}
		svc.mu.RLock()
		if got := svc.byID[conv.ConversationID].Turns[len(svc.byID[conv.ConversationID].Turns)-1].State; got != assistantStateExpired {
			svc.mu.RUnlock()
			t.Fatalf("want expired state after commit timeout, got %s", got)
		}
		svc.mu.RUnlock()

		singleChoiceTurn := &assistantTurn{
			TurnID:              "turn-single-choice-confirmed",
			State:               assistantStateConfirmed,
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			AmbiguityCount:      1,
			ResolvedCandidateID: "FLOWER-A",
			PolicyVersion:       capabilityPolicyVersionBaseline,
			CompositionVersion:  capabilityPolicyVersionBaseline,
			MappingVersion:      capabilityPolicyVersionBaseline,
		}
		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns = append(svc.byID[conv.ConversationID].Turns, singleChoiceTurn)
		svc.mu.Unlock()
		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, singleChoiceTurn.TurnID, ""); err != nil {
			t.Fatalf("confirmed turn with single candidate should be idempotent: %v", err)
		}

		unresolvedTurn := &assistantTurn{TurnID: "turn-unresolved", State: assistantStateValidated, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}}
		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns = append(svc.byID[conv.ConversationID].Turns, unresolvedTurn)
		svc.mu.Unlock()
		if _, err := svc.confirmTurn("tenant-1", principal, conv.ConversationID, unresolvedTurn.TurnID, ""); !errors.Is(err, errAssistantConfirmationRequired) {
			t.Fatalf("want confirmation required for unresolved candidate, got %v", err)
		}

		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns[0].State = assistantStateConfirmed
		svc.mu.Unlock()
		if _, err := assistantCommitTurnSyncForTest(svc, context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "viewer"}, conv.ConversationID, turnID); !errors.Is(err, errAssistantRoleDriftDetected) {
			t.Fatalf("want role drift, got %v", err)
		}
		if _, err := assistantCommitTurnSyncForTest(svc, context.Background(), "tenant-1", Principal{ID: "actor-x", RoleSlug: "tenant-admin"}, conv.ConversationID, turnID); !errors.Is(err, errAssistantAuthSnapshotExpired) {
			t.Fatalf("want auth snapshot expired, got %v", err)
		}
		if _, err := assistantCommitTurnSyncForTest(svc, context.Background(), "tenant-x", principal, conv.ConversationID, turnID); !errors.Is(err, errAssistantTenantMismatch) {
			t.Fatalf("want tenant mismatch, got %v", err)
		}
		if _, err := assistantCommitTurnSyncForTest(svc, context.Background(), "tenant-1", principal, conv.ConversationID, "missing"); !errors.Is(err, errAssistantTurnNotFound) {
			t.Fatalf("want turn not found, got %v", err)
		}
		if _, err := assistantCommitTurnSyncForTest(svc, context.Background(), "tenant-1", principal, conv.ConversationID, turnID); err != nil {
			t.Fatalf("commit failed: %v", err)
		}
		if _, err := assistantCommitTurnSyncForTest(svc, context.Background(), "tenant-1", principal, conv.ConversationID, turnID); err != nil {
			t.Fatalf("idempotent commit failed: %v", err)
		}
		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns[0].State = assistantStateCanceled
		svc.mu.Unlock()
		if _, err := assistantCommitTurnSyncForTest(svc, context.Background(), "tenant-1", principal, conv.ConversationID, turnID); !errors.Is(err, errAssistantConversationStateInvalid) {
			t.Fatalf("want state invalid for canceled commit, got %v", err)
		}
		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns[0].State = assistantStateExpired
		svc.mu.Unlock()
		if _, err := assistantCommitTurnSyncForTest(svc, context.Background(), "tenant-1", principal, conv.ConversationID, turnID); !errors.Is(err, errAssistantConversationStateInvalid) {
			t.Fatalf("want state invalid for expired commit, got %v", err)
		}

		unsupportedSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		unsupportedConv := unsupportedSvc.createConversation("tenant-1", principal)
		unsupportedTurn := &assistantTurn{
			TurnID:             "turn-unsupported",
			State:              assistantStateConfirmed,
			Intent:             assistantIntentSpec{Action: "plan_only"},
			Plan:               assistantBuildPlan(assistantIntentSpec{Action: "plan_only"}),
			PolicyVersion:      capabilityPolicyVersionBaseline,
			CompositionVersion: capabilityPolicyVersionBaseline,
			MappingVersion:     capabilityPolicyVersionBaseline,
		}
		unsupportedSvc.mu.Lock()
		unsupportedSvc.byID[unsupportedConv.ConversationID].Turns = append(unsupportedSvc.byID[unsupportedConv.ConversationID].Turns, unsupportedTurn)
		unsupportedSvc.mu.Unlock()
		if _, err := assistantCommitTurnSyncForTest(unsupportedSvc, context.Background(), "tenant-1", principal, unsupportedConv.ConversationID, unsupportedTurn.TurnID); !errors.Is(err, errAssistantUnsupportedIntent) {
			t.Fatalf("want unsupported intent, got %v", err)
		}

		missingCandidateSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		missingCandidateConv := missingCandidateSvc.createConversation("tenant-1", principal)
		missingCandidateTurn := &assistantTurn{
			TurnID:             "turn-missing-candidate",
			State:              assistantStateConfirmed,
			Intent:             assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
			Plan:               assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			PolicyVersion:      capabilityPolicyVersionBaseline,
			CompositionVersion: capabilityPolicyVersionBaseline,
			MappingVersion:     capabilityPolicyVersionBaseline,
		}
		missingCandidateSvc.mu.Lock()
		missingCandidateSvc.byID[missingCandidateConv.ConversationID].Turns = append(missingCandidateSvc.byID[missingCandidateConv.ConversationID].Turns, missingCandidateTurn)
		missingCandidateSvc.mu.Unlock()
		if _, err := assistantCommitTurnSyncForTest(missingCandidateSvc, context.Background(), "tenant-1", principal, missingCandidateConv.ConversationID, missingCandidateTurn.TurnID); !errors.Is(err, errAssistantCandidateNotFound) {
			t.Fatalf("want candidate not found, got %v", err)
		}

		driftSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		driftConv := driftSvc.createConversation("tenant-1", principal)
		driftTurn := &assistantTurn{
			TurnID:              "turn-drift",
			State:               assistantStateConfirmed,
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			ResolvedCandidateID: "FLOWER-A",
			Candidates:          []assistantCandidate{{CandidateID: "FLOWER-A", CandidateCode: "FLOWER-A", Name: "鲜花组织", IsActive: true}},
			PolicyVersion:       "1999-01-01",
			CompositionVersion:  capabilityPolicyVersionBaseline,
			MappingVersion:      capabilityPolicyVersionBaseline,
		}
		driftSvc.mu.Lock()
		driftSvc.byID[driftConv.ConversationID].Turns = append(driftSvc.byID[driftConv.ConversationID].Turns, driftTurn)
		driftSvc.mu.Unlock()
		if _, err := assistantCommitTurnSyncForTest(driftSvc, context.Background(), "tenant-1", principal, driftConv.ConversationID, driftTurn.TurnID); !errors.Is(err, errAssistantConfirmationRequired) {
			t.Fatalf("want confirmation required on drift, got %v", err)
		}
		driftSvc.mu.RLock()
		driftState := driftSvc.byID[driftConv.ConversationID].Turns[0].State
		driftSvc.mu.RUnlock()
		if driftState != assistantStateValidated {
			t.Fatalf("expected drift rollback to validated, got %s", driftState)
		}

		corruptedSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		corruptedSvc.mu.Lock()
		corruptedSvc.byID["conv-corrupted"] = nil
		corruptedSvc.mu.Unlock()
		if _, err := assistantCommitTurnSyncForTest(corruptedSvc, context.Background(), "tenant-1", principal, "conv-corrupted", "turn-1"); !errors.Is(err, errAssistantConversationCorrupted) {
			t.Fatalf("want conversation corrupted, got %v", err)
		}

		fallbackNameSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		fallbackConv := fallbackNameSvc.createConversation("tenant-1", principal)
		fallbackTurn := &assistantTurn{
			TurnID:              "turn-empty-name",
			State:               assistantStateConfirmed,
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			ResolvedCandidateID: "FLOWER-A",
			Candidates:          []assistantCandidate{{CandidateID: "FLOWER-A", CandidateCode: "FLOWER-A", Name: "鲜花组织", OrgID: 10000000, IsActive: true}},
			RequestID:           "assistant_req_empty_name",
			PolicyVersion:       capabilityPolicyVersionBaseline,
			CompositionVersion:  capabilityPolicyVersionBaseline,
			MappingVersion:      capabilityPolicyVersionBaseline,
		}
		if err := fallbackNameSvc.refreshTurnVersionTuple(context.Background(), "tenant-1", fallbackTurn); err != nil {
			t.Fatalf("refresh fallback turn err=%v", err)
		}
		fallbackNameSvc.mu.Lock()
		fallbackNameSvc.byID[fallbackConv.ConversationID].Turns = append(fallbackNameSvc.byID[fallbackConv.ConversationID].Turns, fallbackTurn)
		fallbackNameSvc.mu.Unlock()
		if _, err := assistantCommitTurnSyncForTest(fallbackNameSvc, context.Background(), "tenant-1", principal, fallbackConv.ConversationID, fallbackTurn.TurnID); err != nil {
			t.Fatalf("commit with fallback name failed: %v", err)
		}
	})

	t.Run("clone and request body helpers", func(t *testing.T) {
		now := time.Now().UTC()
		origin := &assistantConversation{
			ConversationID: "conv-1",
			Turns: []*assistantTurn{
				nil,
				{
					TurnID:     "turn-1",
					Candidates: []assistantCandidate{{CandidateID: "A"}},
					DryRun: assistantDryRunResult{
						Diff: []map[string]any{{"field": "name", "after": "运营部"}},
					},
					CommitResult: &assistantCommitResult{OrgCode: "AI0001"},
					CreatedAt:    now,
					UpdatedAt:    now,
				},
			},
		}
		cloned := cloneConversation(origin)
		if cloned == origin || len(cloned.Turns) != 1 {
			t.Fatalf("clone failed: %+v", cloned)
		}
		cloned.Turns[0].Candidates[0].CandidateID = "B"
		if origin.Turns[1].Candidates[0].CandidateID != "A" {
			t.Fatal("clone should deep copy candidates")
		}
		if cloneConversation(nil) != nil {
			t.Fatal("clone nil should return nil")
		}

		if hasRequestBody(nil) {
			t.Fatal("nil request should have no body")
		}
		reqNoBody := httptest.NewRequest(http.MethodPost, "http://localhost", http.NoBody)
		if hasRequestBody(reqNoBody) {
			t.Fatal("no body request should return false")
		}
		reqWithContent := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewBufferString(`{"a":1}`))
		if !hasRequestBody(reqWithContent) {
			t.Fatal("content-length body should return true")
		}
		reqChunked := httptest.NewRequest(http.MethodPost, "http://localhost", http.NoBody)
		reqChunked.Header.Set("Transfer-Encoding", "chunked")
		if !hasRequestBody(reqChunked) {
			t.Fatal("chunked body should return true")
		}

		rec := httptest.NewRecorder()
		writeJSON(rec, http.StatusAccepted, map[string]string{"status": "ok"})
		if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), "ok") {
			t.Fatalf("write json unexpected: status=%d body=%s", rec.Code, rec.Body.String())
		}
		if got := *ptrString("value"); got != "value" {
			t.Fatalf("ptrString result=%s", got)
		}

		if segs := assistantSplitPathSegments(" /internal/assistant "); len(segs) != 2 {
			t.Fatalf("segments=%v", segs)
		}
		if segs := assistantSplitPathSegments(" "); segs != nil {
			t.Fatalf("segments should be nil, got=%v", segs)
		}
	})
}

func TestAssistantResolveCommitError_CoverageMatrix(t *testing.T) {
	t.Run("known stable db code uses mapped message", func(t *testing.T) {
		status, code, message, ok := assistantResolveCommitError(errors.New(orgUnitErrFieldPolicyMissing))
		if !ok {
			t.Fatal("expected resolver hit")
		}
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", status)
		}
		if code != orgUnitErrFieldPolicyMissing {
			t.Fatalf("code=%q", code)
		}
		if message == code {
			t.Fatalf("expected mapped message, got=%q", message)
		}
	})

	t.Run("unknown but stable code returns 422", func(t *testing.T) {
		status, code, message, ok := assistantResolveCommitError(errors.New("SOME_STABLE_CODE"))
		if !ok {
			t.Fatal("expected resolver hit")
		}
		if status != http.StatusUnprocessableEntity || code != "SOME_STABLE_CODE" || message != "SOME_STABLE_CODE" {
			t.Fatalf("status=%d code=%q message=%q", status, code, message)
		}
	})

	t.Run("bad request error maps to invalid_request", func(t *testing.T) {
		err := newBadRequestError("invalid input from ui")
		status, code, message, ok := assistantResolveCommitError(err)
		if !ok {
			t.Fatal("expected resolver hit")
		}
		if status != http.StatusBadRequest || code != "invalid_request" || message != err.Error() {
			t.Fatalf("status=%d code=%q message=%q", status, code, message)
		}
	})

	t.Run("pg invalid input maps to invalid_request", func(t *testing.T) {
		err := &pgconn.PgError{Code: "22P02", Message: "invalid_text_representation"}
		status, code, message, ok := assistantResolveCommitError(err)
		if !ok {
			t.Fatal("expected resolver hit")
		}
		if status != http.StatusBadRequest || code != "invalid_request" || message != err.Error() {
			t.Fatalf("status=%d code=%q message=%q", status, code, message)
		}
	})

	t.Run("unknown and unstable error falls through", func(t *testing.T) {
		status, code, message, ok := assistantResolveCommitError(errors.New("write failed"))
		if ok {
			t.Fatalf("unexpected resolver hit status=%d code=%q message=%q", status, code, message)
		}
	})

	t.Run("blank error message falls through", func(t *testing.T) {
		status, code, message, ok := assistantResolveCommitError(errors.New("   "))
		if ok {
			t.Fatalf("unexpected resolver hit status=%d code=%q message=%q", status, code, message)
		}
	})
}

func TestAssistantConfirmWindowHelpers_Coverage(t *testing.T) {
	now := time.Now().UTC()

	plan := assistantFreezeConfirmWindow(assistantPlanSummary{}, time.Time{})
	if plan.ConfirmTTLSeconds != assistantConfirmTTLSecondsDefault || strings.TrimSpace(plan.ExpiresAt) == "" {
		t.Fatalf("unexpected default plan=%+v", plan)
	}

	preserved := assistantFreezeConfirmWindow(assistantPlanSummary{ConfirmTTLSeconds: 30, ExpiresAt: "2026-01-01T00:00:00Z"}, now)
	if preserved.ConfirmTTLSeconds != 30 || preserved.ExpiresAt != "2026-01-01T00:00:00Z" {
		t.Fatalf("unexpected preserved plan=%+v", preserved)
	}

	if deadline, ok := assistantTurnConfirmDeadline(nil); ok || !deadline.IsZero() {
		t.Fatalf("nil turn deadline=%v ok=%v", deadline, ok)
	}
	if deadline, ok := assistantTurnConfirmDeadline(&assistantTurn{}); ok || !deadline.IsZero() {
		t.Fatalf("empty turn deadline=%v ok=%v", deadline, ok)
	}

	fallbackTurn := &assistantTurn{
		State:     assistantStateValidated,
		Plan:      assistantPlanSummary{ExpiresAt: "bad-timestamp"},
		UpdatedAt: now,
	}
	deadline, ok := assistantTurnConfirmDeadline(fallbackTurn)
	if !ok || deadline.IsZero() {
		t.Fatalf("want fallback deadline, got %v ok=%v", deadline, ok)
	}

	if assistantTurnConfirmExpired(&assistantTurn{State: assistantStateConfirmed}, time.Time{}) {
		t.Fatal("confirmed turn should not use confirm expiry")
	}
	if assistantTurnConfirmExpired(&assistantTurn{State: assistantStateValidated}, time.Time{}) {
		t.Fatal("validated turn without deadline should not expire")
	}
	if !assistantTurnConfirmExpired(&assistantTurn{State: assistantStateValidated, Plan: assistantPlanSummary{ExpiresAt: now.Add(-1 * time.Second).Format(time.RFC3339)}}, time.Time{}) {
		t.Fatal("past deadline should expire")
	}
}
