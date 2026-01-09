package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"net/http"
	"net/http/httptest"
)

func TestAttendanceTimeBankRoute_AuthzAllowAndForbid(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "enforce")

	writePolicy := func(t *testing.T, lines []string) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "policy.csv")
		if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	mustLogin := func(t *testing.T, h http.Handler) *http.Cookie {
		t.Helper()

		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=pw"))
		req.Host = "localhost:8080"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("login status=%d body=%q", rec.Code, rec.Body.String())
		}
		c := rec.Result().Cookies()[0]
		if c == nil || c.Name != "sid" || c.Value == "" {
			t.Fatalf("unexpected sid cookie: %#v", c)
		}
		return c
	}

	basePolicy := []string{
		"p, role:anonymous, *, iam.session, read",
		"p, role:anonymous, *, iam.session, admin",
		"p, role:tenant-admin, *, iam.session, admin",
	}

	t.Run("allowed when policy includes time bank read", func(t *testing.T) {
		policy := append([]string(nil), basePolicy...)
		policy = append(policy, "p, role:tenant-admin, *, staffing.attendance-time-bank, read")

		t.Setenv("AUTHZ_POLICY_PATH", writePolicy(t, policy))

		h, err := NewHandlerWithOptions(HandlerOptions{
			TenancyResolver:  localTenancyResolver(),
			IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1"}},
			OrgUnitStore:     newOrgUnitMemoryStore(),
		})
		if err != nil {
			t.Fatal(err)
		}

		sid := mustLogin(t, h)
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-time-bank?as_of=2026-01-01", nil)
		req.Host = "localhost:8080"
		req.AddCookie(sid)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("forbidden when policy lacks time bank read", func(t *testing.T) {
		t.Setenv("AUTHZ_POLICY_PATH", writePolicy(t, basePolicy))

		h, err := NewHandlerWithOptions(HandlerOptions{
			TenancyResolver:  localTenancyResolver(),
			IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1"}},
			OrgUnitStore:     newOrgUnitMemoryStore(),
		})
		if err != nil {
			t.Fatal(err)
		}

		sid := mustLogin(t, h)
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-time-bank?as_of=2026-01-01", nil)
		req.Host = "localhost:8080"
		req.AddCookie(sid)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})
}
