package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

func TestAssistantConversationFlow_AmbiguousCandidateConfirmAndCommit(t *testing.T) {
	wd := mustGetwd(t)
	allowlistPath := mustAllowlistPathFromWd(t, wd)
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	orgStore := newOrgUnitMemoryStore()
	tenantID := "00000000-0000-0000-0000-000000000001"
	if _, err := orgStore.CreateNodeCurrent(context.Background(), tenantID, "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := orgStore.CreateNodeCurrent(context.Background(), tenantID, "2026-01-01", "FLOWER-B", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}

	svc := newAssistantConversationService(orgStore, assistantWriteServiceStub{store: orgStore})
	principal := Principal{ID: "00000000-0000-0000-0000-0000000000aa", RoleSlug: "tenant-admin"}
	conversation := svc.createConversation(tenantID, principal)
	created, err := svc.createTurn(context.Background(), tenantID, principal, conversation.ConversationID, `在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。`)
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Turns) != 1 {
		t.Fatalf("turn count=%d", len(created.Turns))
	}
	turn := created.Turns[0]
	if turn.Intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("intent=%s", turn.Intent.Action)
	}
	if turn.AmbiguityCount < 2 {
		t.Fatalf("ambiguity_count=%d", turn.AmbiguityCount)
	}
	if turn.Phase != assistantPhaseAwaitCandidatePick {
		t.Fatalf("expected await_candidate_pick, got=%s", turn.Phase)
	}
	candidateID := turn.Candidates[1].CandidateID
	picked, err := svc.createTurn(context.Background(), tenantID, principal, conversation.ConversationID, candidateID)
	if err != nil {
		t.Fatalf("candidate pick err=%v", err)
	}
	turn = picked.Turns[len(picked.Turns)-1]
	if turn.Phase == assistantPhaseAwaitCandidateConfirm {
		resolved, resolveErr := svc.createTurn(context.Background(), tenantID, principal, conversation.ConversationID, "确认")
		if resolveErr != nil {
			t.Fatalf("candidate confirm err=%v", resolveErr)
		}
		turn = resolved.Turns[len(resolved.Turns)-1]
	}
	if turn.Phase != assistantPhaseAwaitCommitConfirm {
		t.Fatalf("expected await_commit_confirm, got=%s", turn.Phase)
	}
	confirmed, err := svc.confirmTurn(tenantID, principal, conversation.ConversationID, turn.TurnID, candidateID)
	if err != nil {
		t.Fatalf("confirm err=%v", err)
	}
	turn = confirmed.Turns[len(confirmed.Turns)-1]
	if turn.State != assistantStateConfirmed {
		t.Fatalf("turn state=%s", turn.State)
	}
	if turn.ResolvedCandidateID != candidateID {
		t.Fatalf("resolved_candidate_id=%s", turn.ResolvedCandidateID)
	}
	committed, err := assistantCommitTurnSyncForTest(svc, context.Background(), tenantID, principal, conversation.ConversationID, turn.TurnID)
	if err != nil {
		t.Fatalf("commit err=%v", err)
	}
	committedTurn := assistantLookupTurn(committed, turn.TurnID)
	if committedTurn == nil {
		t.Fatalf("committed turn %s not found", turn.TurnID)
	}
	turn = committedTurn
	if turn.State != assistantStateCommitted {
		t.Fatalf("turn state=%s", turn.State)
	}
	if turn.CommitResult == nil {
		t.Fatal("commit result missing")
	}
	if turn.CommitResult.ParentOrgCode != turn.Candidates[1].CandidateCode {
		t.Fatalf("parent_org_code=%s want=%s", turn.CommitResult.ParentOrgCode, turn.Candidates[1].CandidateCode)
	}
	if turn.CommitResult.EffectiveDate != "2026-01-01" {
		t.Fatalf("effective_date=%s", turn.CommitResult.EffectiveDate)
	}
}

func TestAssistantConversationFlow_CommitResultVisibleInOrgList(t *testing.T) {
	wd := mustGetwd(t)
	allowlistPath := mustAllowlistPathFromWd(t, wd)
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	orgStore := newOrgUnitMemoryStore()
	tenantID := "00000000-0000-0000-0000-000000000001"
	if _, err := orgStore.CreateNodeCurrent(context.Background(), tenantID, "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}
	svc := newAssistantConversationService(orgStore, assistantWriteServiceStub{store: orgStore})
	principal := Principal{ID: "00000000-0000-0000-0000-0000000000ab", RoleSlug: "tenant-admin"}
	conversation := svc.createConversation(tenantID, principal)
	created, err := svc.createTurn(context.Background(), tenantID, principal, conversation.ConversationID, "在鲜花组织之下，新建一个名为人力资源部239A的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。")
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Turns) != 1 {
		t.Fatalf("turn count=%d", len(created.Turns))
	}
	turn := created.Turns[0]
	if turn.State != assistantStateValidated {
		t.Fatalf("turn state=%s", turn.State)
	}
	confirmed, err := svc.confirmTurn(tenantID, principal, conversation.ConversationID, turn.TurnID, "")
	if err != nil {
		t.Fatalf("confirm err=%v", err)
	}
	turn = confirmed.Turns[0]
	committed, err := assistantCommitTurnSyncForTest(svc, context.Background(), tenantID, principal, conversation.ConversationID, turn.TurnID)
	if err != nil {
		t.Fatalf("commit err=%v", err)
	}
	turn = committed.Turns[0]
	if turn.State != assistantStateCommitted {
		t.Fatalf("turn state=%s", turn.State)
	}
	if turn.CommitResult == nil {
		t.Fatal("commit_result missing")
	}
	if turn.CommitResult.EffectiveDate != "2026-01-01" {
		t.Fatalf("effective_date=%s", turn.CommitResult.EffectiveDate)
	}
	createdOrgCode := strings.TrimSpace(turn.CommitResult.OrgCode)
	if createdOrgCode == "" {
		t.Fatal("commit_result.org_code empty")
	}
	nodes, err := orgStore.ListNodesCurrent(context.Background(), tenantID, "2026-01-01")
	if err != nil {
		t.Fatalf("list nodes err=%v", err)
	}
	found := false
	for _, row := range nodes {
		if strings.TrimSpace(row.OrgCode) == createdOrgCode {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created org_code=%s not found in current nodes", createdOrgCode)
	}
}

func asString(value any) string {
	text, _ := value.(string)
	return text
}

func TestAssistantConversationAPI_InvalidTurnInput(t *testing.T) {
	wd := mustGetwd(t)
	allowlistPath := mustAllowlistPathFromWd(t, wd)
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000bb",
			Email:            "tenant-admin@example.invalid",
			RoleSlug:         "tenant-admin",
		}},
		OrgUnitStore:        newOrgUnitMemoryStore(),
		OrgUnitWriteService: assistantWriteServiceStub{store: newOrgUnitMemoryStore()},
	})
	if err != nil {
		t.Fatal(err)
	}

	sidCookie := loginAsTenantAdminForAssistantTests(t, h)
	conversation := createAssistantConversationForTest(t, h, sidCookie)

	path := "/internal/assistant/conversations/" + conversation.ConversationID + "/turns"
	req := httptest.NewRequest(http.MethodPost, "http://localhost"+path, bytes.NewBufferString(`{"user_input":""}`))
	req.Host = "localhost"
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sidCookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func loginAsTenantAdminForAssistantTests(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	login := httptest.NewRequest(http.MethodPost, "http://localhost/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	login.Host = "localhost"
	login.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("sid cookie missing")
	}
	return cookies[0]
}

func createAssistantConversationForTest(t *testing.T, h http.Handler, sidCookie *http.Cookie) *assistantConversation {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://localhost/internal/assistant/conversations", bytes.NewBufferString(`{}`))
	req.Host = "localhost"
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sidCookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create conversation status=%d body=%s", rec.Code, rec.Body.String())
	}
	var conversation assistantConversation
	if err := json.Unmarshal(rec.Body.Bytes(), &conversation); err != nil {
		t.Fatalf("unmarshal create conversation=%v", err)
	}
	return &conversation
}

func createAssistantTurnForTest(t *testing.T, h http.Handler, sidCookie *http.Cookie, conversationID string, input string) *assistantConversation {
	t.Helper()
	payload := map[string]string{"user_input": input}
	body, _ := json.Marshal(payload)
	path := filepath.ToSlash("/internal/assistant/conversations/" + conversationID + "/turns")
	req := httptest.NewRequest(http.MethodPost, "http://localhost"+path, bytes.NewBuffer(body))
	req.Host = "localhost"
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sidCookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create turn status=%d body=%s", rec.Code, rec.Body.String())
	}
	var conversation assistantConversation
	if err := json.Unmarshal(rec.Body.Bytes(), &conversation); err != nil {
		t.Fatalf("unmarshal create turn=%v", err)
	}
	return &conversation
}

type assistantWriteServiceStub struct {
	store *orgUnitMemoryStore
}

func (s assistantWriteServiceStub) Write(ctx context.Context, tenantID string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
	if s.store == nil {
		return orgunitservices.OrgUnitWriteResult{}, errors.New("write store missing")
	}
	parentID, err := s.store.ResolveOrgID(ctx, tenantID, strings.TrimSpace(*req.Patch.ParentOrgCode))
	if err != nil {
		return orgunitservices.OrgUnitWriteResult{}, err
	}
	name := strings.TrimSpace(*req.Patch.Name)
	if name == "" {
		name = "新建组织"
	}
	orgCode := strings.TrimSpace(req.OrgCode)
	if orgCode == "" {
		orgCode = assistantGeneratedOrgCode(req.RequestID)
	}
	_, err = s.store.CreateNodeCurrent(ctx, tenantID, req.EffectiveDate, orgCode, name, strconv.Itoa(parentID), false)
	if err != nil {
		return orgunitservices.OrgUnitWriteResult{}, err
	}
	return orgunitservices.OrgUnitWriteResult{
		OrgCode:       orgCode,
		EffectiveDate: req.EffectiveDate,
		EventType:     "CREATE",
		EventUUID:     "evt_" + req.RequestID,
	}, nil
}

func (assistantWriteServiceStub) Create(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceStub) Rename(context.Context, string, orgunitservices.RenameOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceStub) Move(context.Context, string, orgunitservices.MoveOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceStub) Disable(context.Context, string, orgunitservices.DisableOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceStub) Enable(context.Context, string, orgunitservices.EnableOrgUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceStub) SetBusinessUnit(context.Context, string, orgunitservices.SetBusinessUnitRequest) error {
	return errors.New("not implemented")
}

func (assistantWriteServiceStub) Correct(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceStub) CorrectStatus(context.Context, string, orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceStub) RescindRecord(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}

func (assistantWriteServiceStub) RescindOrg(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, errors.New("not implemented")
}
