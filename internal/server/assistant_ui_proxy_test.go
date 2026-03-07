package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssistantUIProxyHandlerRejectsInvalidPath(t *testing.T) {
	t.Setenv("LIBRECHAT_UPSTREAM", "http://assistant.local")
	h := newAssistantUIProxyHandler()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/not-assistant", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), assistantUIProxyPathInvalidCode) {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestAssistantUIProxyBridgePathIsNoLongerSpecialCased(t *testing.T) {
	h := newAssistantUIProxyHandler()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui/bridge.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Result().Header.Get("Location"); loc != libreChatFormalEntryPrefix {
		t.Fatalf("location=%q", loc)
	}
}

func TestModifyAssistantUIProxyResponseOnlyFiltersCookies(t *testing.T) {
	html := `<!doctype html><html><head><base href="/" /></head><body>x</body></html>`
	resp := &http.Response{
		Header:  make(http.Header),
		Body:    io.NopCloser(strings.NewReader(html)),
		Request: httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil),
	}
	resp.Header.Set("Content-Type", "text/html; charset=utf-8")
	resp.Header.Add("Set-Cookie", "refreshToken=rf; Path=/; HttpOnly")
	if err := modifyAssistantUIProxyResponse(resp); err != nil {
		t.Fatalf("modifyAssistantUIProxyResponse returned error: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != html {
		t.Fatalf("html should remain unchanged, got=%q", string(body))
	}
	cookies := resp.Cookies()
	if len(cookies) != 1 || cookies[0].Path != "/assistant-ui" {
		t.Fatalf("cookies=%+v", cookies)
	}
}

func TestServeAssistantUIFallbackShellShowsRetiredNoticeOnly(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	rec := httptest.NewRecorder()
	serveAssistantUIFallbackShell(rec, req, http.StatusBadGateway)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "bridge.js") {
		t.Fatalf("fallback shell should not load bridge.js, got=%q", body)
	}
	if strings.Contains(body, "data-assistant-dialog-stream") {
		t.Fatalf("fallback shell should not expose legacy dialog stream, got=%q", body)
	}
	if !strings.Contains(body, "旧 bridge/iframe 链路已退役") {
		t.Fatalf("fallback shell should show retired notice, got=%q", body)
	}
}
