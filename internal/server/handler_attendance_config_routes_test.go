package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewHandler_AttendanceConfigRoutesWired(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	personStore := newPersonMemoryStore()
	p, err := personStore.CreatePerson(context.Background(), "00000000-0000-0000-0000-000000000001", "1", "Alice")
	if err != nil {
		t.Fatal(err)
	}

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1"}},
		OrgUnitStore:     newOrgUnitMemoryStore(),
		PersonStore:      personStore,
	})
	if err != nil {
		t.Fatal(err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=pw"))
	loginReq.Host = "localhost:8080"
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusFound {
		t.Fatalf("login status=%d", loginRec.Code)
	}
	sidCookie := loginRec.Result().Cookies()[0]
	if sidCookie == nil || sidCookie.Name != "sid" || sidCookie.Value == "" {
		t.Fatalf("unexpected sid cookie: %#v", sidCookie)
	}

	req1 := httptest.NewRequest(http.MethodGet, "/org/attendance-time-profile?as_of=2026-01-01", nil)
	req1.Host = "localhost:8080"
	req1.AddCookie(sidCookie)
	req1.Header.Set("HX-Request", "true")
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec1.Code, rec1.Body.String())
	}

	req1Post := httptest.NewRequest(http.MethodPost, "/org/attendance-time-profile?as_of=2026-01-01", strings.NewReader("op=nope"))
	req1Post.Host = "localhost:8080"
	req1Post.AddCookie(sidCookie)
	req1Post.Header.Set("HX-Request", "true")
	req1Post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec1Post := httptest.NewRecorder()
	h.ServeHTTP(rec1Post, req1Post)
	if rec1Post.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec1Post.Code, rec1Post.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", nil)
	req2.Host = "localhost:8080"
	req2.AddCookie(sidCookie)
	req2.Header.Set("HX-Request", "true")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec2.Code, rec2.Body.String())
	}

	req2Post := httptest.NewRequest(http.MethodPost, "/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01", strings.NewReader("op=nope"))
	req2Post.Host = "localhost:8080"
	req2Post.AddCookie(sidCookie)
	req2Post.Header.Set("HX-Request", "true")
	req2Post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec2Post := httptest.NewRecorder()
	h.ServeHTTP(rec2Post, req2Post)
	if rec2Post.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec2Post.Code, rec2Post.Body.String())
	}

	req3 := httptest.NewRequest(http.MethodGet, "/org/attendance-integrations?as_of=2026-01-01", nil)
	req3.Host = "localhost:8080"
	req3.AddCookie(sidCookie)
	req3.Header.Set("HX-Request", "true")
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec3.Code, rec3.Body.String())
	}

	req4 := httptest.NewRequest(http.MethodPost, "/org/attendance-integrations?as_of=2026-01-01", strings.NewReader("op=link&provider=WECOM&external_user_id=wecom_u1&person_uuid="+p.UUID))
	req4.Host = "localhost:8080"
	req4.AddCookie(sidCookie)
	req4.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec4 := httptest.NewRecorder()
	h.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%q", rec4.Code, rec4.Body.String())
	}
}
