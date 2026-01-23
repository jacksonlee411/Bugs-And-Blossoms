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
