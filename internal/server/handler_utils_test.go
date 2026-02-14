package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMustNewHandler_Success(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/bugs_and_blossoms?sslmode=disable")

	if MustNewHandler() == nil {
		t.Fatal("expected handler")
	}
}

func TestMustNewHandler_Panic(t *testing.T) {
	t.Setenv("ALLOWLIST_PATH", filepath.Join(t.TempDir(), "missing.yaml"))

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = MustNewHandler()
}

func TestWriteContentWithStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/content", nil)

	writeContentWithStatus(rec, req, http.StatusCreated, "hello")

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("content-type=%q", ct)
	}
	if body := rec.Body.String(); body != "hello" {
		t.Fatalf("body=%q", body)
	}
}

func TestWritePageWithStatus_HX(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	req.Header.Set("HX-Request", "true")

	writePageWithStatus(rec, req, http.StatusAccepted, "hx-body")

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); body != "hx-body" {
		t.Fatalf("body=%q", body)
	}
}

func TestWritePageWithStatus_NonHX(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/page", nil)

	writePageWithStatus(rec, req, http.StatusTeapot, "page-body")

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "page-body") {
		t.Fatal("expected body in shell")
	}
}

func TestRedirectBack_UsesRefererOrDefault(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Referer", "/from")

	redirectBack(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Result().Header.Get("Location"); loc != "/from" {
		t.Fatalf("location=%q", loc)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	redirectBack(rec2, req2)
	if rec2.Code != http.StatusFound {
		t.Fatalf("status=%d", rec2.Code)
	}
	if loc := rec2.Result().Header.Get("Location"); loc != "/app" {
		t.Fatalf("location=%q", loc)
	}
}

func TestSetLangCookie_SetsCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	setLangCookie(rec, "zh")

	resp := rec.Result()
	defer resp.Body.Close()

	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "lang" && c.Value == "zh" && c.HttpOnly {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected lang cookie set; got=%v", resp.Cookies())
	}
}

func TestLang_Tr_RenderHelpers(t *testing.T) {
	if got := lang(httptest.NewRequest(http.MethodGet, "/", nil)); got != "en" {
		t.Fatalf("expected en, got %q", got)
	}

	reqZH := httptest.NewRequest(http.MethodGet, "/", nil)
	reqZH.AddCookie(&http.Cookie{Name: "lang", Value: "zh"})
	if got := lang(reqZH); got != "zh" {
		t.Fatalf("expected zh, got %q", got)
	}

	reqOther := httptest.NewRequest(http.MethodGet, "/", nil)
	reqOther.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	if got := lang(reqOther); got != "en" {
		t.Fatalf("expected en, got %q", got)
	}

	keys := []string{
		"nav_orgunit",
		"nav_orgunit_snapshot",
		"nav_setid",
		"nav_jobcatalog",
		"nav_staffing",
		"nav_person",
		"as_of",
		"shared_readonly",
		"tenant_owned",
	}
	for _, key := range keys {
		if tr("zh", key) == "" {
			t.Fatalf("expected zh translation for %q", key)
		}
		if tr("en", key) == "" {
			t.Fatalf("expected en translation for %q", key)
		}
	}
	if tr("en", "unknown_key") != "" {
		t.Fatal("expected empty translation for unknown key")
	}
	if tr("zh", "unknown_key") != "" {
		t.Fatal("expected empty translation for unknown key")
	}

	if nav := renderNav(httptest.NewRequest(http.MethodGet, "/", nil)); !strings.Contains(nav, "hx-get") {
		t.Fatalf("unexpected nav: %q", nav)
	}
	if topbar := renderTopbar(httptest.NewRequest(http.MethodGet, "/", nil)); !strings.Contains(topbar, "/lang/en") {
		t.Fatalf("unexpected topbar: %q", topbar)
	}

	if got := renderLoginForm(""); !strings.Contains(got, `action="/login"`) {
		t.Fatalf("unexpected login form: %q", got)
	}
	if got := renderLoginForm("bad"); !strings.Contains(got, "bad") {
		t.Fatalf("expected error in login form: %q", got)
	}
}
