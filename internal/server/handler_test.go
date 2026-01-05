package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewHandler_Health(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandler()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d", rec2.Code)
	}
}

func TestMustNewHandler_PanicsOnBadPath(t *testing.T) {
	if err := os.Setenv("ALLOWLIST_PATH", "no-such-file.yaml"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = MustNewHandler()
}

func TestMustNewHandler_Success(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h := MustNewHandler()
	if h == nil {
		t.Fatal("nil handler")
	}
}

func TestDefaultAllowlistPath_Errors(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	_, err = defaultAllowlistPath()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandler_DefaultPath(t *testing.T) {
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandler()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestNewHandler_ClassifierError(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "allowlist.yaml")
	if err := os.WriteFile(p, []byte("version: 1\nentrypoints:\n  server:\n    routes: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("ALLOWLIST_PATH", p); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	_, err := NewHandler()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandler_DefaultAllowlistNotFound(t *testing.T) {
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	_, err = NewHandler()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUI_ShellAndPartials(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandler()
	if err != nil {
		t.Fatal(err)
	}

	reqAsset := httptest.NewRequest(http.MethodGet, "/assets/app.css", nil)
	recAsset := httptest.NewRecorder()
	h.ServeHTTP(recAsset, reqAsset)
	if recAsset.Code != http.StatusOK {
		t.Fatalf("asset status=%d", recAsset.Code)
	}

	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	recRoot := httptest.NewRecorder()
	h.ServeHTTP(recRoot, reqRoot)
	if recRoot.Code != http.StatusFound {
		t.Fatalf("root status=%d", recRoot.Code)
	}

	reqApp := httptest.NewRequest(http.MethodGet, "/app", nil)
	recApp := httptest.NewRecorder()
	h.ServeHTTP(recApp, reqApp)
	if recApp.Code != http.StatusOK {
		t.Fatalf("app status=%d", recApp.Code)
	}

	reqHome := httptest.NewRequest(http.MethodGet, "/app/home", nil)
	recHome := httptest.NewRecorder()
	h.ServeHTTP(recHome, reqHome)
	if recHome.Code != http.StatusOK {
		t.Fatalf("home status=%d", recHome.Code)
	}

	reqFlash := httptest.NewRequest(http.MethodGet, "/ui/flash", nil)
	recFlash := httptest.NewRecorder()
	h.ServeHTTP(recFlash, reqFlash)
	if recFlash.Code != http.StatusOK {
		t.Fatalf("flash status=%d", recFlash.Code)
	}

	reqNavZH := httptest.NewRequest(http.MethodGet, "/ui/nav", nil)
	reqNavZH.AddCookie(&http.Cookie{Name: "lang", Value: "zh"})
	recNavZH := httptest.NewRecorder()
	h.ServeHTTP(recNavZH, reqNavZH)
	if recNavZH.Code != http.StatusOK {
		t.Fatalf("nav zh status=%d", recNavZH.Code)
	}

	reqTopbarEN := httptest.NewRequest(http.MethodGet, "/ui/topbar", nil)
	recTopbarEN := httptest.NewRecorder()
	h.ServeHTTP(recTopbarEN, reqTopbarEN)
	if recTopbarEN.Code != http.StatusOK {
		t.Fatalf("topbar en status=%d", recTopbarEN.Code)
	}

	reqTopbarZH := httptest.NewRequest(http.MethodGet, "/ui/topbar", nil)
	reqTopbarZH.AddCookie(&http.Cookie{Name: "lang", Value: "zh"})
	recTopbarZH := httptest.NewRecorder()
	h.ServeHTTP(recTopbarZH, reqTopbarZH)
	if recTopbarZH.Code != http.StatusOK {
		t.Fatalf("topbar zh status=%d", recTopbarZH.Code)
	}

	reqNavEN := httptest.NewRequest(http.MethodGet, "/ui/nav", nil)
	recNavEN := httptest.NewRecorder()
	h.ServeHTTP(recNavEN, reqNavEN)
	if recNavEN.Code != http.StatusOK {
		t.Fatalf("nav en status=%d", recNavEN.Code)
	}

	reqLangNoRef := httptest.NewRequest(http.MethodGet, "/lang/en", nil)
	recLangNoRef := httptest.NewRecorder()
	h.ServeHTTP(recLangNoRef, reqLangNoRef)
	if recLangNoRef.Code != http.StatusFound {
		t.Fatalf("lang status=%d", recLangNoRef.Code)
	}

	reqLangWithRef := httptest.NewRequest(http.MethodGet, "/lang/zh", nil)
	reqLangWithRef.Header.Set("Referer", "/app")
	recLangWithRef := httptest.NewRecorder()
	h.ServeHTTP(recLangWithRef, reqLangWithRef)
	if recLangWithRef.Code != http.StatusFound {
		t.Fatalf("lang status=%d", recLangWithRef.Code)
	}

	reqLogin := httptest.NewRequest(http.MethodGet, "/login", nil)
	recLogin := httptest.NewRecorder()
	h.ServeHTTP(recLogin, reqLogin)
	if recLogin.Code != http.StatusOK {
		t.Fatalf("login status=%d", recLogin.Code)
	}

	reqLoginPost := httptest.NewRequest(http.MethodPost, "/login", nil)
	recLoginPost := httptest.NewRecorder()
	h.ServeHTTP(recLoginPost, reqLoginPost)
	if recLoginPost.Code != http.StatusFound {
		t.Fatalf("login post status=%d", recLoginPost.Code)
	}

	reqLogout := httptest.NewRequest(http.MethodPost, "/logout", nil)
	recLogout := httptest.NewRecorder()
	h.ServeHTTP(recLogout, reqLogout)
	if recLogout.Code != http.StatusFound {
		t.Fatalf("logout status=%d", recLogout.Code)
	}

	reqOrg := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
	recOrg := httptest.NewRecorder()
	h.ServeHTTP(recOrg, reqOrg)
	if recOrg.Code != http.StatusOK {
		t.Fatalf("org status=%d", recOrg.Code)
	}

	reqJC := httptest.NewRequest(http.MethodGet, "/org/job-catalog", nil)
	recJC := httptest.NewRecorder()
	h.ServeHTTP(recJC, reqJC)
	if recJC.Code != http.StatusOK {
		t.Fatalf("jobcatalog status=%d", recJC.Code)
	}

	reqPos := httptest.NewRequest(http.MethodGet, "/org/positions", nil)
	recPos := httptest.NewRecorder()
	h.ServeHTTP(recPos, reqPos)
	if recPos.Code != http.StatusOK {
		t.Fatalf("positions status=%d", recPos.Code)
	}

	reqAsn := httptest.NewRequest(http.MethodGet, "/org/assignments", nil)
	recAsn := httptest.NewRecorder()
	h.ServeHTTP(recAsn, reqAsn)
	if recAsn.Code != http.StatusOK {
		t.Fatalf("assignments status=%d", recAsn.Code)
	}

	reqPer := httptest.NewRequest(http.MethodGet, "/person/persons", nil)
	recPer := httptest.NewRecorder()
	h.ServeHTTP(recPer, reqPer)
	if recPer.Code != http.StatusOK {
		t.Fatalf("persons status=%d", recPer.Code)
	}

	_ = tr("en", "unknown")
	_ = tr("zh", "unknown")

	rNoCookie := &http.Request{Header: http.Header{}}
	_ = lang(rNoCookie)
	rOther := &http.Request{Header: http.Header{}}
	rOther.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	_ = lang(rOther)
}
