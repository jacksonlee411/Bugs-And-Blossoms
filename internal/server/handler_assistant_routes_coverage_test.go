package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHandlerWithOptions_AssistantRoutes_AreWired(t *testing.T) {
	wd := mustGetwd(t)
	allowlistPath := mustAllowlistPathFromWd(t, wd)
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
			RoleSlug:         "tenant-admin",
		}},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	login := httptest.NewRequest(http.MethodPost, "http://localhost/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	login.Host = "localhost"
	login.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	sidCookie := loginRec.Result().Cookies()[0]

	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
		req.Host = "localhost"
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(sidCookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := call(http.MethodPost, "/internal/assistant/tasks", `{"conversation_id":"conv_1","turn_id":"turn_1","task_type":"assistant_async_plan","request_id":"req_1","contract_snapshot":{"intent_schema_version":"v1","compiler_contract_version":"v1","capability_map_version":"v1","skill_manifest_digest":"d","context_hash":"c","intent_hash":"i","plan_hash":"p"}}`); rec.Code == http.StatusNotFound {
		t.Fatalf("assistant tasks route not wired")
	}
	if rec := call(http.MethodGet, "/internal/assistant/tasks/task_1", ""); rec.Code == http.StatusNotFound {
		t.Fatalf("assistant task detail route not wired")
	} else if rec.Code != http.StatusServiceUnavailable || assistantDecodeErrCode(t, rec) != "assistant_task_workflow_unavailable" {
		t.Fatalf("assistant task detail status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := call(http.MethodGet, "/internal/assistant/conversations", ""); rec.Code == http.StatusNotFound {
		t.Fatalf("assistant conversation list route not wired")
	}
	if rec := call(http.MethodPost, "/internal/assistant/tasks/task_1:cancel", ""); rec.Code == http.StatusNotFound {
		t.Fatalf("assistant task action route not wired")
	} else if rec.Code != http.StatusServiceUnavailable || assistantDecodeErrCode(t, rec) != "assistant_task_workflow_unavailable" {
		t.Fatalf("assistant task action status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := call(http.MethodGet, "/internal/assistant/model-providers", ""); rec.Code == http.StatusNotFound {
		t.Fatalf("assistant model providers route not wired")
	}
	if rec := call(http.MethodPost, "/internal/assistant/model-providers:validate", `{"providers":[]}`); rec.Code == http.StatusNotFound {
		t.Fatalf("assistant model providers validate route not wired")
	}
	if rec := call(http.MethodPost, "/internal/assistant/model-providers:apply", `{"providers":[]}`); rec.Code == http.StatusNotFound {
		t.Fatalf("assistant model providers apply route not wired")
	}
	if rec := call(http.MethodGet, "/internal/assistant/models", ""); rec.Code == http.StatusNotFound {
		t.Fatalf("assistant models route not wired")
	}
	if rec := call(http.MethodGet, "/internal/assistant/runtime-status", ""); rec.Code == http.StatusNotFound {
		t.Fatalf("assistant runtime status route not wired")
	}
}
